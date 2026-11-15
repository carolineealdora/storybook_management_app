package order

import (
	"net/http"
	"testing"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models/dto"
	"codebase/test/apitest"
	services_mocks "codebase/test/mock/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	bookID  = "33333333-3333-3333-3333-333333333333"
	orderID = "44444444-4444-4444-4444-444444444444"
)

var errWrap = helpers.NewError(nil)

func newCtrl(svc *services_mocks.OrderService) *Controller {
	return &Controller{Response: apitest.Response(), Binding: helpers.NewBinding(), Service: svc}
}

func TestCreate_Valid_Returns201(t *testing.T) {
	svc := new(services_mocks.OrderService)
	svc.On("Create", "user-1", bookID, "key-1").Return(&dto.BookOrderView{ID: orderID, Price: 15000, Status: "pending"}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"book_id":"` + bookID + `","idempotency_key":"key-1"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).Create(c))
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestCreate_InvalidBody_Returns400(t *testing.T) {
	for _, body := range []string{`{"idempotency_key":"k"}`, `{"book_id":"` + bookID + `"}`, `{"book_id":"x","idempotency_key":"k"}`} {
		c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: body, Claims: apitest.ParentClaims()})
		require.NoError(t, newCtrl(new(services_mocks.OrderService)).Create(c))
		assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec), body)
	}
}

func TestCreate_ErrorMapping(t *testing.T) {
	for code, status := range map[string]int{
		respConst.BOOK_ALREADY_OWNED:   409,
		respConst.SUBSCRIPTION_ACTIVE:  409,
		respConst.BOOK_NOT_PURCHASABLE: 409,
		respConst.DUPLICATE_REQUEST:    409,
		respConst.BOOK_NOT_FOUND:       404,
	} {
		svc := new(services_mocks.OrderService)
		svc.On("Create", "user-1", bookID, "k").Return(nil, errWrap.Wrap(code, ""))
		c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"book_id":"` + bookID + `","idempotency_key":"k"}`, Claims: apitest.ParentClaims()})
		require.NoError(t, newCtrl(svc).Create(c))
		assert.Equal(t, status, rec.Code, code)
		assert.Equal(t, code, apitest.ErrCode(t, rec))
	}
}

func TestPay_Success_Returns200(t *testing.T) {
	svc := new(services_mocks.OrderService)
	svc.On("Pay", "user-1", orderID, "success").Return(&dto.BookOrderView{ID: orderID, Status: "paid"}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"success"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": orderID}})
	require.NoError(t, newCtrl(svc).Pay(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPay_ErrorMapping(t *testing.T) {
	for code, status := range map[string]int{
		respConst.PAYMENT_FAILED:    402,
		respConst.DUPLICATE_REQUEST: 409,
		respConst.BOOK_UNAVAILABLE:  409,
		respConst.ORDER_NOT_FOUND:   404,
	} {
		svc := new(services_mocks.OrderService)
		svc.On("Pay", "user-1", orderID, "failed").Return(nil, errWrap.Wrap(code, ""))
		c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"failed"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": orderID}})
		require.NoError(t, newCtrl(svc).Pay(c))
		assert.Equal(t, status, rec.Code, code)
	}
}

func TestPay_InvalidResultOrID_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"maybe"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": orderID}})
	require.NoError(t, newCtrl(new(services_mocks.OrderService)).Pay(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))

	c, rec = apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"success"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": "x"}})
	require.NoError(t, newCtrl(new(services_mocks.OrderService)).Pay(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestList_Returns200WithTotal(t *testing.T) {
	svc := new(services_mocks.OrderService)
	svc.On("List", "user-1").Return([]dto.BookOrderView{}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).List(c))
	assert.JSONEq(t, `{"data":[],"meta":{"total":0}}`, rec.Body.String())
}
