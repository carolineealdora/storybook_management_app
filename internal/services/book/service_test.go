package book

import (
	"errors"
	"testing"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models"
	repository_mock "codebase/test/mock/repository"

	"github.com/stretchr/testify/assert"
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
	f.svc = &ServiceImpl{books: f.books, orders: f.orders, subs: f.subs, errors: helpers.NewError(nil)}
	return f
}

func (f fixture) subscribed(yes bool) {
	if yes {
		f.subs.On("GetActive", "user-1").Return(&models.Subscription{ID: "sub-1", Status: models.SubActive}, nil)
	} else {
		f.subs.On("GetActive", "user-1").Return(nil, gorm.ErrRecordNotFound)
	}
}

func book(id string, price int) models.Book {
	return models.Book{ID: id, Title: id, Price: price, Status: models.BookPublished, AgeType: models.AgePlus, AgeMin: 3}
}

func code(err error) string { return helpers.ErrCode(err) }

// --- Showcase (US-04, US-05) ---

func TestList_NonSubscriber_FreeOwnedAndLocked(t *testing.T) {
	f := newFixture()
	f.books.On("ListPublished").Return([]models.Book{book("free", 0), book("owned", 15000), book("paid", 20000)}, nil)
	f.orders.On("PurchasedBookIDs", "user-1").Return(map[string]bool{"owned": true}, nil)
	f.subscribed(false)

	got, err := f.svc.List("user-1")
	require.NoError(t, err)
	access := map[string]string{}
	for _, c := range got {
		access[c.ID] = c.Access
	}
	assert.Equal(t, map[string]string{"free": models.AccessFree, "owned": models.AccessOwned, "paid": models.AccessLocked}, access)
	assert.True(t, got[0].IsFree)
	assert.Equal(t, "3+", got[0].AgeLabel)
	assert.Equal(t, "/api/v1/books/free/cover", got[0].CoverURL)
}

func TestList_Subscriber_NoPaidBookLocked(t *testing.T) {
	f := newFixture()
	f.books.On("ListPublished").Return([]models.Book{book("free", 0), book("owned", 15000), book("paid", 20000)}, nil)
	f.orders.On("PurchasedBookIDs", "user-1").Return(map[string]bool{"owned": true}, nil)
	f.subscribed(true)

	got, err := f.svc.List("user-1")
	require.NoError(t, err)
	assert.Equal(t, models.AccessFree, got[0].Access, "free stays free")
	assert.Equal(t, models.AccessOwned, got[1].Access, "purchased stays owned")
	assert.Equal(t, models.AccessSubscribed, got[2].Access)
}

func TestList_Empty_ReturnsEmptySlice(t *testing.T) {
	f := newFixture()
	f.books.On("ListPublished").Return(nil, nil)
	f.orders.On("PurchasedBookIDs", "user-1").Return(map[string]bool{}, nil)
	f.subscribed(false)
	got, err := f.svc.List("user-1")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestList_RepoFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.books.On("ListPublished").Return(nil, errors.New("db down"))
	_, err := f.svc.List("user-1")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- Detail (US-06, E22) ---

func TestGet_Published_ReturnsDetailWithAccess(t *testing.T) {
	f := newFixture()
	b := book("b1", 15000)
	b.Description, b.Author = "A story", "Ana"
	f.books.On("GetByID", "b1").Return(&b, nil)
	f.orders.On("HasPurchase", "user-1", "b1").Return(false, nil)
	f.subscribed(false)

	got, err := f.svc.Get("user-1", "b1")
	require.NoError(t, err)
	assert.Equal(t, "A story", got.Description)
	assert.Equal(t, "Ana", got.Author)
	assert.Equal(t, models.AccessLocked, got.Access)
}

func TestGet_MissingOrUnpublishedNotOwned_ReturnsBookNotFound(t *testing.T) {
	f := newFixture()
	draft := book("draft", 15000)
	draft.Status = models.BookDraft
	f.books.On("GetByID", "missing").Return(nil, gorm.ErrRecordNotFound)
	f.books.On("GetByID", "draft").Return(&draft, nil)
	f.orders.On("HasPurchase", "user-1", "draft").Return(false, nil)
	f.subscribed(true)

	_, err := f.svc.Get("user-1", "missing")
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
	_, err = f.svc.Get("user-1", "draft")
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err), "subscription does not reveal unpublished books")
}

// --- Content (US-07, E4, E5, E18) ---

func TestContent_Locked_ReturnsBookLocked(t *testing.T) {
	f := newFixture()
	b := book("b1", 15000)
	f.books.On("GetWithContent", "b1").Return(&b, nil)
	f.orders.On("HasPurchase", "user-1", "b1").Return(false, nil)
	f.subscribed(false)
	_, err := f.svc.Content("user-1", "b1")
	assert.Equal(t, respConst.BOOK_LOCKED, code(err))
}

func TestContent_FreeOwnedOrSubscribed_ReturnsEpub(t *testing.T) {
	cases := map[string]struct {
		price      int
		owned, sub bool
	}{
		"free":       {0, false, false},
		"owned":      {15000, true, false},
		"subscribed": {15000, false, true},
	}
	for name, c := range cases {
		f := newFixture()
		b := book("b1", c.price)
		b.EpubContent = []byte("EPUB")
		f.books.On("GetWithContent", "b1").Return(&b, nil)
		f.orders.On("HasPurchase", "user-1", "b1").Return(c.owned, nil)
		f.subscribed(c.sub)
		got, err := f.svc.Content("user-1", "b1")
		require.NoError(t, err, name)
		assert.Equal(t, []byte("EPUB"), got.EpubContent, name)
	}
}

func TestContent_SoftDeletedButOwned_StillReadable(t *testing.T) {
	f := newFixture()
	b := book("b1", 15000)
	deleted := now
	b.DeletedAt, b.EpubContent = &deleted, []byte("EPUB")
	f.books.On("GetWithContent", "b1").Return(&b, nil)
	f.orders.On("HasPurchase", "user-1", "b1").Return(true, nil)
	f.subscribed(false)
	_, err := f.svc.Content("user-1", "b1")
	assert.NoError(t, err)
}

func TestContent_SoftDeletedSubscriberNotOwner_ReturnsBookNotFound(t *testing.T) {
	f := newFixture()
	b := book("b1", 15000)
	deleted := now
	b.DeletedAt = &deleted
	f.books.On("GetWithContent", "b1").Return(&b, nil)
	f.orders.On("HasPurchase", "user-1", "b1").Return(false, nil)
	f.subscribed(true)
	_, err := f.svc.Content("user-1", "b1")
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
}

func TestContent_PurchaseLookupFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	b := book("b1", 15000)
	f.books.On("GetWithContent", "b1").Return(&b, nil)
	f.orders.On("HasPurchase", "user-1", "b1").Return(false, errors.New("db down"))
	_, err := f.svc.Content("user-1", "b1")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- Cover / My Book (US-08) ---

func TestCover_Missing_ReturnsBookNotFound(t *testing.T) {
	f := newFixture()
	f.books.On("GetCover", "b1").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.Cover("b1")
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
}

func TestCover_Found_ReturnsFile(t *testing.T) {
	f := newFixture()
	f.books.On("GetCover", "b1").Return(&models.BookFile{MimeType: "image/png", Content: []byte("PNG")}, nil)
	got, err := f.svc.Cover("b1")
	require.NoError(t, err)
	assert.Equal(t, "image/png", got.MimeType)
}

func TestMyBooks_ListsPurchasesWithOwnedAccess(t *testing.T) {
	f := newFixture()
	deleted := now
	gone := book("gone", 15000)
	gone.DeletedAt = &deleted
	f.orders.On("ListMyBooks", "user-1").Return([]models.OwnedBook{
		{Book: book("b2", 20000), PricePaid: 18000, PurchasedAt: now},
		{Book: gone, PricePaid: 15000, PurchasedAt: now.Add(-time.Hour)},
	}, nil)
	got, err := f.svc.MyBooks("user-1")
	require.NoError(t, err)
	require.Len(t, got, 2, "soft-deleted purchased books stay in My Book (E5)")
	assert.Equal(t, models.AccessOwned, got[0].Access)
	assert.Equal(t, 18000, got[0].PricePaid, "price paid, not current price")
}

func TestMyBooks_None_ReturnsEmptySlice(t *testing.T) {
	f := newFixture()
	f.orders.On("ListMyBooks", "user-1").Return(nil, nil)
	got, err := f.svc.MyBooks("user-1")
	require.NoError(t, err)
	assert.NotNil(t, got)
}
