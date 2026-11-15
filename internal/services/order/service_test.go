package order

import (
	"errors"
	"testing"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models"
	sql "codebase/pkg/orm"
	repository_mock "codebase/test/mock/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var now = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

type fixture struct {
	books  *repository_mock.BookRepository
	orders *repository_mock.OrderRepository
	subs   *repository_mock.SubscriptionRepository
	svc    *ServiceImpl
}

func newFixture() fixture {
	f := fixture{books: new(repository_mock.BookRepository), orders: new(repository_mock.OrderRepository), subs: new(repository_mock.SubscriptionRepository)}
	f.svc = &ServiceImpl{books: f.books, orders: f.orders, subs: f.subs, errors: helpers.NewError(nil), now: func() time.Time { return now }}
	return f
}

func code(err error) string { return helpers.ErrCode(err) }

func paidBook(price int) *models.Book {
	return &models.Book{ID: "book-1", Title: "Cat", Price: price, Status: models.BookPublished, AgeType: models.AgePlus}
}

// state sets up the book, ownership and subscription the checks look at.
func (f fixture) state(b *models.Book, owned, subscribed bool) {
	f.books.On("GetByID", "book-1").Return(b, nil)
	f.orders.On("HasPurchase", "user-1", "book-1").Return(owned, nil)
	if subscribed {
		f.subs.On("GetActive", "user-1").Return(&models.Subscription{Status: models.SubActive}, nil)
	} else {
		f.subs.On("GetActive", "user-1").Return(nil, gorm.ErrRecordNotFound)
	}
}

func pending(price int) *models.BookOrder {
	return &models.BookOrder{ID: "order-1", UserID: "user-1", BookID: "book-1", Price: price, Status: models.OrderPending, IdempotencyKey: "k1", CreatedAt: now}
}

// --- Create (US-09) ---

func TestCreate_PaidBookNotOwned_CreatesPendingOrderWithPriceSnapshot(t *testing.T) {
	f := newFixture()
	f.state(paidBook(15000), false, false)
	f.orders.On("CreateBookOrder", mock.MatchedBy(func(o *models.BookOrder) bool {
		return o.UserID == "user-1" && o.BookID == "book-1" && o.Price == 15000 && o.Status == models.OrderPending && o.IdempotencyKey == "k1"
	})).Return(nil)

	got, err := f.svc.Create("user-1", "book-1", "k1")
	require.NoError(t, err)
	assert.Equal(t, 15000, got.Price)
	assert.Equal(t, "Cat", got.BookTitle)
	assert.Equal(t, models.OrderPending, got.Status)
}

func TestCreate_Rejections(t *testing.T) {
	deleted := now
	draft := paidBook(15000)
	draft.Status = models.BookDraft
	gone := paidBook(15000)
	gone.DeletedAt = &deleted
	cases := map[string]struct {
		book       *models.Book
		owned, sub bool
		want       string
	}{
		"E10 free book":          {paidBook(0), false, false, respConst.BOOK_NOT_PURCHASABLE},
		"E1 already owned":       {paidBook(15000), true, false, respConst.BOOK_ALREADY_OWNED},
		"E2 subscribed":          {paidBook(15000), false, true, respConst.SUBSCRIPTION_ACTIVE},
		"E22 unpublished":        {draft, false, false, respConst.BOOK_NOT_FOUND},
		"E22 soft-deleted":       {gone, false, false, respConst.BOOK_NOT_FOUND},
		"owned beats subscribed": {paidBook(15000), true, true, respConst.BOOK_ALREADY_OWNED},
	}
	for name, c := range cases {
		f := newFixture()
		f.state(c.book, c.owned, c.sub)
		_, err := f.svc.Create("user-1", "book-1", "k1")
		assert.Equal(t, c.want, code(err), name)
		f.orders.AssertNotCalled(t, "CreateBookOrder", mock.Anything)
	}
}

func TestCreate_MissingBook_ReturnsBookNotFound(t *testing.T) {
	f := newFixture()
	f.books.On("GetByID", "book-1").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.Create("user-1", "book-1", "k1")
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
}

func TestCreate_SameIdempotencyKey_ReturnsDuplicateRequest(t *testing.T) {
	f := newFixture()
	f.state(paidBook(15000), false, false)
	f.orders.On("CreateBookOrder", mock.Anything).Return(sql.ErrDuplicate)
	_, err := f.svc.Create("user-1", "book-1", "k1")
	assert.Equal(t, respConst.DUPLICATE_REQUEST, code(err))
}

func TestCreate_DBDown_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.state(paidBook(15000), false, false)
	f.orders.On("CreateBookOrder", mock.Anything).Return(errors.New("db down"))
	_, err := f.svc.Create("user-1", "book-1", "k1")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- Pay (US-09, E6–E9) ---

func TestPay_Success_MarksPaidAtSnapshotPrice(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
	f.state(paidBook(25000), false, false) // admin raised the price after checkout (E7)
	f.orders.On("MarkBookOrderPaid", mock.MatchedBy(func(o *models.BookOrder) bool { return o.Price == 15000 }), now).Return(nil)

	got, err := f.svc.Pay("user-1", "order-1", "success")
	require.NoError(t, err)
	assert.Equal(t, models.OrderPaid, got.Status)
	assert.Equal(t, 15000, got.Price, "charged the price shown at checkout")
	require.NotNil(t, got.PaidAt)
	f.orders.AssertExpectations(t)
}

func TestPay_Failed_MarksFailedAndReturnsPaymentFailed(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
	f.state(paidBook(15000), false, false)
	f.orders.On("MarkBookOrderFailed", "order-1", now).Return(nil)
	_, err := f.svc.Pay("user-1", "order-1", "failed")
	assert.Equal(t, respConst.PAYMENT_FAILED, code(err))
	f.orders.AssertNotCalled(t, "MarkBookOrderPaid", mock.Anything, mock.Anything)
}

func TestPay_AlreadyPaidOrFailed_ReturnsDuplicateRequest(t *testing.T) {
	for _, status := range []string{models.OrderPaid, models.OrderFailed} {
		f := newFixture()
		o := pending(15000)
		o.Status = status
		f.orders.On("GetBookOrder", "order-1", "user-1").Return(o, nil)
		_, err := f.svc.Pay("user-1", "order-1", "success")
		assert.Equal(t, respConst.DUPLICATE_REQUEST, code(err), status)
	}
}

func TestPay_ConcurrentDoubleTap_SecondGetsDuplicateRequest(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
	f.state(paidBook(15000), false, false)
	f.orders.On("MarkBookOrderPaid", mock.Anything, now).Return(sql.ErrStateChanged)
	_, err := f.svc.Pay("user-1", "order-1", "success")
	assert.Equal(t, respConst.DUPLICATE_REQUEST, code(err))
}

func TestPay_ConcurrentFailVsPay_ReturnsDuplicateRequest(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
	f.state(paidBook(15000), false, false)
	f.orders.On("MarkBookOrderFailed", "order-1", now).Return(sql.ErrStateChanged)
	_, err := f.svc.Pay("user-1", "order-1", "failed")
	assert.Equal(t, respConst.DUPLICATE_REQUEST, code(err))
}

func TestPay_OtherOrderBoughtSameBookConcurrently_ReturnsAlreadyOwned(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
	f.state(paidBook(15000), false, false)
	f.orders.On("MarkBookOrderPaid", mock.Anything, now).Return(sql.ErrDuplicate)
	_, err := f.svc.Pay("user-1", "order-1", "success")
	assert.Equal(t, respConst.BOOK_ALREADY_OWNED, code(err))
}

func TestPay_BookUnpublishedOrDeletedDuringCheckout_ReturnsBookUnavailable(t *testing.T) {
	deleted := now
	draft := paidBook(15000)
	draft.Status = models.BookDraft
	gone := paidBook(15000)
	gone.DeletedAt = &deleted
	for name, b := range map[string]*models.Book{"unpublished": draft, "deleted": gone} {
		f := newFixture()
		f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
		f.state(b, false, false)
		_, err := f.svc.Pay("user-1", "order-1", "success")
		assert.Equal(t, respConst.BOOK_UNAVAILABLE, code(err), name)
		f.orders.AssertNotCalled(t, "MarkBookOrderPaid", mock.Anything, mock.Anything)
	}
}

func TestPay_SubscribedSinceCheckout_ReturnsSubscriptionActive(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
	f.state(paidBook(15000), false, true)
	_, err := f.svc.Pay("user-1", "order-1", "success")
	assert.Equal(t, respConst.SUBSCRIPTION_ACTIVE, code(err))
}

func TestPay_OtherUsersOrder_ReturnsOrderNotFound(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.Pay("user-1", "order-1", "success")
	assert.Equal(t, respConst.ORDER_NOT_FOUND, code(err))
}

func TestPay_MarkPaidFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.orders.On("GetBookOrder", "order-1", "user-1").Return(pending(15000), nil)
	f.state(paidBook(15000), false, false)
	f.orders.On("MarkBookOrderPaid", mock.Anything, now).Return(errors.New("db down"))
	_, err := f.svc.Pay("user-1", "order-1", "success")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- Order History (US-12) ---

func TestList_ReturnsRowsWithTitleAndPricePaid(t *testing.T) {
	f := newFixture()
	paidAt := now
	f.orders.On("ListBookOrders", "user-1").Return([]models.BookOrderRow{
		{BookOrder: models.BookOrder{ID: "o2", BookID: "b", Price: 15000, Status: models.OrderPaid, PaidAt: &paidAt}, BookTitle: "Cat"},
		{BookOrder: models.BookOrder{ID: "o1", BookID: "b", Price: 15000, Status: models.OrderFailed}, BookTitle: "Cat"},
	}, nil)
	got, err := f.svc.List("user-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Cat", got[0].BookTitle)
	assert.Equal(t, models.OrderFailed, got[1].Status)
}

func TestList_None_ReturnsEmptySlice(t *testing.T) {
	f := newFixture()
	f.orders.On("ListBookOrders", "user-1").Return(nil, nil)
	got, err := f.svc.List("user-1")
	require.NoError(t, err)
	assert.NotNil(t, got)
}
