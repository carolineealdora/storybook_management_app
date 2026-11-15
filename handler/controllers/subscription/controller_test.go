package subscription

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

var errWrap = helpers.NewError(nil)

func newCtrl(svc *services_mocks.SubscriptionService) *Controller {
	return &Controller{Response: apitest.Response(), Binding: helpers.NewBinding(), Service: svc}
}

func TestGetPlan_Returns200(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("GetPlan").Return(&dto.PlanView{Price: 49000, Benefits: "all"}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).GetPlan(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetPlan_NotConfigured_Returns404(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("GetPlan").Return(nil, errWrap.Wrap(respConst.PLAN_NOT_FOUND, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).GetPlan(c))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPutPlan_Valid_Returns200(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("UpdatePlan", 59000, "All books").Return(&dto.PlanView{Price: 59000}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: `{"price":59000,"benefits":"All books"}`, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).PutPlan(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPutPlan_InvalidPrice_Returns400(t *testing.T) {
	for _, body := range []string{`{"price":0,"benefits":"x"}`, `{"price":49000.5,"benefits":"x"}`, `{"price":"49000","benefits":"x"}`, `{"price":49000}`} {
		c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: body, Claims: apitest.AdminClaims()})
		require.NoError(t, newCtrl(new(services_mocks.SubscriptionService)).PutPlan(c))
		assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec), body)
	}
}

const orderID = "55555555-5555-5555-5555-555555555555"

func TestCreateOrder_Valid_Returns201(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("CreateOrder", "user-1", "k1").Return(&dto.SubscriptionOrderView{ID: orderID}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"idempotency_key":"k1"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).CreateOrder(c))
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestCreateOrder_MissingKey_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(new(services_mocks.SubscriptionService)).CreateOrder(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestCreateOrder_AlreadySubscribed_Returns409(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("CreateOrder", "user-1", "k1").Return(nil, errWrap.Wrap(respConst.SUBSCRIPTION_ACTIVE, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"idempotency_key":"k1"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).CreateOrder(c))
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestPayOrder_Success_Returns200(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("PayOrder", "user-1", orderID, "success").Return(&dto.SubscriptionView{Status: "active"}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"success"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": orderID}})
	require.NoError(t, newCtrl(svc).PayOrder(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPayOrder_Failed_Returns402(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("PayOrder", "user-1", orderID, "failed").Return(nil, errWrap.Wrap(respConst.PAYMENT_FAILED, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"failed"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": orderID}})
	require.NoError(t, newCtrl(svc).PayOrder(c))
	assert.Equal(t, http.StatusPaymentRequired, rec.Code)
}

func TestPayOrder_BadInput_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"success"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": "bad"}})
	require.NoError(t, newCtrl(new(services_mocks.SubscriptionService)).PayOrder(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
	c, rec = apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"result":"pending"}`, Claims: apitest.ParentClaims(), Params: map[string]string{"id": orderID}})
	require.NoError(t, newCtrl(new(services_mocks.SubscriptionService)).PayOrder(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestCancel_Active_Returns200(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("Cancel", "user-1").Return(&dto.SubscriptionView{Status: "cancelled"}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).Cancel(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCancel_NotActive_Returns409(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("Cancel", "user-1").Return(nil, errWrap.Wrap(respConst.SUBSCRIPTION_NOT_ACTIVE, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).Cancel(c))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, respConst.SUBSCRIPTION_NOT_ACTIVE, apitest.ErrCode(t, rec))
}

func TestHistory_Returns200WithTotal(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("History", "user-1").Return([]dto.SubscriptionView{{ID: "s1"}}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).History(c))
	assert.Contains(t, rec.Body.String(), `"meta":{"total":1}`)
}
