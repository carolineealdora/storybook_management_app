package adminbook

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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const bookID = "33333333-3333-3333-3333-333333333333"

var errWrap = helpers.NewError(nil)

func newCtrl(svc *services_mocks.AdminBookService) *Controller {
	return &Controller{Response: apitest.Response(), Binding: helpers.NewBinding(), Service: svc}
}

func bookForm(t *testing.T, withFiles bool) (string, string) {
	fields := map[string]string{
		"title": "T", "author": "A", "description": "D", "age_type": "range",
		"age_min": "1", "age_max": "3", "price": "15000", "status": "published",
	}
	if !withFiles {
		return apitest.Multipart(t, fields)
	}
	return apitest.Multipart(t, fields,
		apitest.File{Field: "cover", Name: "c.png", Content: apitest.PNG},
		apitest.File{Field: "epub", Name: "b.epub", Content: apitest.EPUB})
}

func TestCreate_Multipart_PassesFieldsAndFiles_Returns201(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Create", mock.MatchedBy(func(in dto.BookInput) bool {
		return in.Title == "T" && in.AgeMax == "3" && in.Price == "15000" && in.Status == "published" &&
			in.Cover != nil && in.Cover.FileName == "c.png" && len(in.Cover.Content) == len(apitest.PNG) &&
			in.Epub != nil && in.Epub.FileName == "b.epub"
	})).Return(&dto.AdminBook{ID: bookID}, nil)
	body, ct := bookForm(t, true)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: body, ContentType: ct, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).Create(c))
	assert.Equal(t, http.StatusCreated, rec.Code)
	svc.AssertExpectations(t)
}

func TestCreate_ServiceRejectsFormat_Returns400InvalidFileFormat(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Create", mock.Anything).Return(nil, errWrap.Wrap(respConst.INVALID_FILE_FORMAT, ""))
	body, ct := bookForm(t, true)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: body, ContentType: ct, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).Create(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, respConst.INVALID_FILE_FORMAT, apitest.ErrCode(t, rec))
}

func TestCreate_NotMultipart_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"title":"T"}`, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(new(services_mocks.AdminBookService)).Create(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestUpdate_NoFiles_PassesNilFiles_Returns200(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Update", bookID, mock.MatchedBy(func(in dto.BookInput) bool { return in.Cover == nil && in.Epub == nil })).
		Return(&dto.AdminBook{ID: bookID}, nil)
	body, ct := bookForm(t, false)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: body, ContentType: ct, Claims: apitest.AdminClaims(), Params: map[string]string{"id": bookID}})
	require.NoError(t, newCtrl(svc).Update(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestUpdate_BadID_Returns400(t *testing.T) {
	body, ct := bookForm(t, false)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: body, ContentType: ct, Claims: apitest.AdminClaims(), Params: map[string]string{"id": "nope"}})
	require.NoError(t, newCtrl(new(services_mocks.AdminBookService)).Update(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestGet_NotFound_Returns404(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Get", bookID).Return(nil, errWrap.Wrap(respConst.BOOK_NOT_FOUND, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims(), Params: map[string]string{"id": bookID}})
	require.NoError(t, newCtrl(svc).Get(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGet_Found_Returns200(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Get", bookID).Return(&dto.AdminBook{ID: bookID}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims(), Params: map[string]string{"id": bookID}})
	require.NoError(t, newCtrl(svc).Get(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestList_SearchAndStatus_Returns200WithTotal(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("List", "cat", "draft").Return([]dto.AdminBook{{ID: "a"}, {ID: "b"}}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims(), Query: "search=cat&status=draft"})
	require.NoError(t, newCtrl(svc).List(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"meta":{"total":2}`)
}

func TestList_BadStatus_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims(), Query: "status=archived"})
	require.NoError(t, newCtrl(new(services_mocks.AdminBookService)).List(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestDelete_Existing_Returns204(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Delete", bookID).Return(nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodDelete, Claims: apitest.AdminClaims(), Params: map[string]string{"id": bookID}})
	require.NoError(t, newCtrl(svc).Delete(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestDelete_Missing_Returns404(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Delete", bookID).Return(errWrap.Wrap(respConst.BOOK_NOT_FOUND, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodDelete, Claims: apitest.AdminClaims(), Params: map[string]string{"id": bookID}})
	require.NoError(t, newCtrl(svc).Delete(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func coverReq(id string) apitest.Req {
	return apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims(), Params: map[string]string{"id": id}}
}

func TestCover_ReturnsImageBytes(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Cover", bookID).Return(&models.BookFile{MimeType: "image/png", Content: []byte("PNGDATA")}, nil)
	c, rec := apitest.Ctx(coverReq(bookID))
	require.NoError(t, newCtrl(svc).Cover(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, "private, max-age=3600", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "PNGDATA", rec.Body.String())
}

func TestCover_Missing_Returns404(t *testing.T) {
	svc := new(services_mocks.AdminBookService)
	svc.On("Cover", bookID).Return(nil, errWrap.Wrap(respConst.BOOK_NOT_FOUND, ""))
	c, rec := apitest.Ctx(coverReq(bookID))
	require.NoError(t, newCtrl(svc).Cover(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCover_BadID_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(coverReq("nope"))
	require.NoError(t, newCtrl(new(services_mocks.AdminBookService)).Cover(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}
