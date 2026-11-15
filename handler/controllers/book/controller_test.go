package book

import (
	"net/http"
	"testing"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models"
	"codebase/models/dto"
	"codebase/test/apitest"
	services_mocks "codebase/test/mock/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bookID = "33333333-3333-3333-3333-333333333333"

var errWrap = helpers.NewError(nil)

func newCtrl(svc *services_mocks.BookService) *Controller {
	return &Controller{Response: apitest.Response(), Service: svc}
}

func idReq(method string, claims *helpers.Claims) apitest.Req {
	return apitest.Req{Method: method, Claims: claims, Params: map[string]string{"id": bookID}}
}

func TestList_Returns200WithTotal(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("List", "user-1").Return([]dto.BookCard{{ID: "a", Access: "free"}}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).List(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"meta":{"total":1}`)
}

func TestGet_Found_Returns200(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("Get", "user-1", bookID).Return(&dto.BookDetail{BookCard: dto.BookCard{ID: bookID}, Description: "d"}, nil)
	c, rec := apitest.Ctx(idReq(http.MethodGet, apitest.ParentClaims()))
	require.NoError(t, newCtrl(svc).Get(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"description":"d"`)
}

func TestGet_NotFound_Returns404(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("Get", "user-1", bookID).Return(nil, errWrap.Wrap(respConst.BOOK_NOT_FOUND, ""))
	c, rec := apitest.Ctx(idReq(http.MethodGet, apitest.ParentClaims()))
	require.NoError(t, newCtrl(svc).Get(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGet_BadID_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.ParentClaims(), Params: map[string]string{"id": "1"}})
	require.NoError(t, newCtrl(new(services_mocks.BookService)).Get(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestCover_ReturnsImageBytes(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("Cover", bookID).Return(&models.BookFile{MimeType: "image/png", Content: []byte("PNGDATA")}, nil)
	c, rec := apitest.Ctx(idReq(http.MethodGet, apitest.AdminClaims()))
	require.NoError(t, newCtrl(svc).Cover(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, "PNGDATA", rec.Body.String())
}

func TestCover_Missing_Returns404(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("Cover", bookID).Return(nil, errWrap.Wrap(respConst.BOOK_NOT_FOUND, ""))
	c, rec := apitest.Ctx(idReq(http.MethodGet, apitest.ParentClaims()))
	require.NoError(t, newCtrl(svc).Cover(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestContent_Unlocked_StreamsEpubWithoutCaching(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("Content", "user-1", bookID).Return(&models.Book{EpubFileName: "b.epub", EpubContent: []byte("EPUBDATA")}, nil)
	c, rec := apitest.Ctx(idReq(http.MethodGet, apitest.ParentClaims()))
	require.NoError(t, newCtrl(svc).Content(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/epub+zip", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "EPUBDATA", rec.Body.String())
}

func TestContent_Locked_Returns403BookLocked(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("Content", "user-1", bookID).Return(nil, errWrap.Wrap(respConst.BOOK_LOCKED, ""))
	c, rec := apitest.Ctx(idReq(http.MethodGet, apitest.ParentClaims()))
	require.NoError(t, newCtrl(svc).Content(c))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, respConst.BOOK_LOCKED, apitest.ErrCode(t, rec))
	assert.NotContains(t, rec.Body.String(), "EPUB")
}

func TestMyBooks_Returns200(t *testing.T) {
	svc := new(services_mocks.BookService)
	svc.On("MyBooks", "user-1").Return([]dto.MyBook{}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).MyBooks(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":[],"meta":{"total":0}}`, rec.Body.String())
}
