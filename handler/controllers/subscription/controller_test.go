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
