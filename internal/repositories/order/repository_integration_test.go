//go:build integration

package order

import (
	"sync"
	"testing"
	"time"

	"codebase/internal/repositories/testdb"
	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type env struct {
	repo   *Repo
	db     sql.ORM
	userID string
}

func setup(t *testing.T) env {
	db := testdb.Open(t)
	uid := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO users (id, name) VALUES (?, 'P')`, uid))
	return env{repo: &Repo{db: db}, db: db, userID: uid}
}

func (e env) book(t *testing.T, title string, price int) string {
	id := uuid.NewString()
	require.NoError(t, e.db.Exec(`INSERT INTO books (id, title, author, description, age_type, age_min, price, status, epub_file_name, epub_size_bytes, epub_content)
		VALUES (?, ?, 'A', 'D', 'plus', 3, ?, 'published', 'b.epub', 1, '\x00')`, id, title, price))
	return id
}

func (e env) purchase(t *testing.T, bookID string, pricePaid int, at time.Time) {
	orderID := uuid.NewString()
	require.NoError(t, e.db.Exec(`INSERT INTO book_orders (id, user_id, book_id, price, status, idempotency_key, paid_at) VALUES (?, ?, ?, ?, 'paid', ?, ?)`,
		orderID, e.userID, bookID, pricePaid, orderID, at))
	require.NoError(t, e.db.Exec(`INSERT INTO purchases (user_id, book_id, book_order_id, price_paid, purchased_at) VALUES (?, ?, ?, ?, ?)`,
		e.userID, bookID, orderID, pricePaid, at))
}

func TestPurchases_LookupsAndMyBooks_NewestFirstIncludingDeleted(t *testing.T) {
	e := setup(t)
	older, newer, notOwned := e.book(t, "Older", 10000), e.book(t, "Newer", 20000), e.book(t, "Other", 5000)
	now := time.Now().UTC()
	e.purchase(t, older, 9000, now.Add(-time.Hour))
	e.purchase(t, newer, 20000, now)
	require.NoError(t, e.db.Exec(`UPDATE books SET deleted_at = NOW(), price = 99999 WHERE id = ?`, older))

	ids, err := e.repo.PurchasedBookIDs(e.userID)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{older: true, newer: true}, ids)

	has, err := e.repo.HasPurchase(e.userID, newer)
	require.NoError(t, err)
	assert.True(t, has)
	has, err = e.repo.HasPurchase(e.userID, notOwned)
	require.NoError(t, err)
	assert.False(t, has)

	mine, err := e.repo.ListMyBooks(e.userID)
	require.NoError(t, err)
	require.Len(t, mine, 2)
	assert.Equal(t, "Newer", mine[0].Title)
	assert.Equal(t, "Older", mine[1].Title, "soft-deleted purchase stays in My Book")
	assert.Equal(t, 9000, mine[1].PricePaid, "price paid, not the current price")
	assert.NotNil(t, mine[1].DeletedAt)
	assert.Nil(t, mine[0].EpubContent)
}

func TestMyBooks_NoPurchases_Empty(t *testing.T) {
	e := setup(t)
	mine, err := e.repo.ListMyBooks(e.userID)
	require.NoError(t, err)
	assert.Empty(t, mine)
}

func (e env) pendingOrder(t *testing.T, bookID, key string, price int) *models.BookOrder {
	o := &models.BookOrder{UserID: e.userID, BookID: bookID, Price: price, Status: models.OrderPending, IdempotencyKey: key, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, e.repo.CreateBookOrder(o))
	return o
}

func TestCreateBookOrder_SameKeySameUser_ReturnsErrDuplicate(t *testing.T) {
	e := setup(t)
	b := e.book(t, "Cat", 15000)
	e.pendingOrder(t, b, "k1", 15000)
	err := e.repo.CreateBookOrder(&models.BookOrder{UserID: e.userID, BookID: b, Price: 15000, Status: models.OrderPending, IdempotencyKey: "k1"})
	assert.ErrorIs(t, err, sql.ErrDuplicate)
}

func TestGetBookOrder_OtherUser_NotFound(t *testing.T) {
	e := setup(t)
	o := e.pendingOrder(t, e.book(t, "Cat", 15000), "k1", 15000)
	_, err := e.repo.GetBookOrder(o.ID, uuid.NewString())
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	got, err := e.repo.GetBookOrder(o.ID, e.userID)
	require.NoError(t, err)
	assert.Equal(t, models.OrderPending, got.Status)
}

func TestMarkBookOrderPaid_CreatesPurchase_SecondCallStateChanged(t *testing.T) {
	e := setup(t)
	b := e.book(t, "Cat", 15000)
	o := e.pendingOrder(t, b, "k1", 12000)
	require.NoError(t, e.repo.MarkBookOrderPaid(o, time.Now()))
	assert.ErrorIs(t, e.repo.MarkBookOrderPaid(o, time.Now()), sql.ErrStateChanged)

	mine, err := e.repo.ListMyBooks(e.userID)
	require.NoError(t, err)
	require.Len(t, mine, 1)
	assert.Equal(t, 12000, mine[0].PricePaid, "purchase stores the order price")
}

func TestMarkBookOrderPaid_SecondOrderSameBook_ErrDuplicateAndRolledBack(t *testing.T) {
	e := setup(t)
	b := e.book(t, "Cat", 15000)
	first, second := e.pendingOrder(t, b, "k1", 15000), e.pendingOrder(t, b, "k2", 15000)
	require.NoError(t, e.repo.MarkBookOrderPaid(first, time.Now()))
	assert.ErrorIs(t, e.repo.MarkBookOrderPaid(second, time.Now()), sql.ErrDuplicate)

	got, err := e.repo.GetBookOrder(second.ID, e.userID)
	require.NoError(t, err)
	assert.Equal(t, models.OrderPending, got.Status, "paid flag rolled back with the failed purchase insert")
}

func TestMarkBookOrderPaid_ConcurrentDoubleTap_ExactlyOneWins(t *testing.T) {
	e := setup(t)
	o := e.pendingOrder(t, e.book(t, "Cat", 15000), "k1", 15000)
	const n = 10
	errs := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- e.repo.MarkBookOrderPaid(o, time.Now()) }()
	}
	wg.Wait()
	close(errs)
	ok := 0
	for err := range errs {
		if err == nil {
			ok++
		} else {
			assert.ErrorIs(t, err, sql.ErrStateChanged)
		}
	}
	assert.Equal(t, 1, ok)
	ids, err := e.repo.PurchasedBookIDs(e.userID)
	require.NoError(t, err)
	assert.Len(t, ids, 1)
}

func TestMarkBookOrderFailed_PendingOnly(t *testing.T) {
	e := setup(t)
	o := e.pendingOrder(t, e.book(t, "Cat", 15000), "k1", 15000)
	require.NoError(t, e.repo.MarkBookOrderFailed(o.ID, time.Now()))
	assert.ErrorIs(t, e.repo.MarkBookOrderFailed(o.ID, time.Now()), sql.ErrStateChanged)
	assert.ErrorIs(t, e.repo.MarkBookOrderPaid(o, time.Now()), sql.ErrStateChanged, "failed order cannot be paid; retry is a new order")
}

func TestListBookOrders_NewestFirstWithTitle(t *testing.T) {
	e := setup(t)
	b := e.book(t, "Cat", 15000)
	first := e.pendingOrder(t, b, "k1", 15000)
	require.NoError(t, e.repo.MarkBookOrderFailed(first.ID, time.Now()))
	second := e.pendingOrder(t, b, "k2", 15000)
	require.NoError(t, e.db.Exec(`UPDATE book_orders SET created_at = NOW() + INTERVAL '1 second' WHERE id = ?`, second.ID))
	require.NoError(t, e.repo.MarkBookOrderPaid(second, time.Now()))

	rows, err := e.repo.ListBookOrders(e.userID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, second.ID, rows[0].ID)
	assert.Equal(t, models.OrderPaid, rows[0].Status)
	assert.Equal(t, models.OrderFailed, rows[1].Status)
	assert.Equal(t, "Cat", rows[0].BookTitle)
}
