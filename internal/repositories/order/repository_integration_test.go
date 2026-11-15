//go:build integration

package order

import (
	"testing"
	"time"

	"codebase/internal/repositories/testdb"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
