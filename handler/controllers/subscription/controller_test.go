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

func TestGetAdminPlan_Returns200WithHistory(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("GetAdminPlan").Return(&dto.AdminPlanView{PlanView: dto.PlanView{Price: 59000},
		PriceHistory: []dto.PlanPriceChangeView{{OldPrice: 49000, NewPrice: 59000, ChangedBy: "admin-1"}}}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).GetAdminPlan(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"price_history":[{`)
}

func TestPutPlan_Valid_PassesAdminAndReturns200(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("UpdatePlan", "admin-1", 59000, "All books").Return(&dto.AdminPlanView{PlanView: dto.PlanView{Price: 59000}}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: `{"price":59000,"benefits":"All books"}`, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).PutPlan(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPutPlan_PriceNotPositive_Returns400InvalidPrice(t *testing.T) {
	svc := new(services_mocks.SubscriptionService)
	svc.On("UpdatePlan", "admin-1", 0, "x").Return(nil, errWrap.Wrap(respConst.INVALID_PRICE, "")).Once()
	svc.On("UpdatePlan", "admin-1", -1, "x").Return(nil, errWrap.Wrap(respConst.INVALID_PRICE, "")).Once()
	for _, body := range []string{`{"price":0,"benefits":"x"}`, `{"price":-1,"benefits":"x"}`} {
		c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: body, Claims: apitest.AdminClaims()})
		require.NoError(t, newCtrl(svc).PutPlan(c))
		assert.Equal(t, http.StatusBadRequest, rec.Code, body)
		assert.Equal(t, respConst.INVALID_PRICE, apitest.ErrCode(t, rec), body)
	}
}

func TestPutPlan_MalformedBody_Returns400Validation(t *testing.T) {
	for _, body := range []string{`{"price":49000.5,"benefits":"x"}`, `{"price":"49000","benefits":"x"}`, `{"price":49000}`} {
		c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: body, Claims: apitest.AdminClaims()})
		require.NoError(t, newCtrl(new(services_mocks.SubscriptionService)).PutPlan(c))
		assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec), body)
	}
}

const orderID = "55555555-5555-5555-5555-555555555555"
