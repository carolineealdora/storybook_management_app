package account

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

func newCtrl(svc *services_mocks.AccountService) *Controller {
	return &Controller{Response: apitest.Response(), Binding: helpers.NewBinding(), AccountService: svc}
}

func TestGetMe_Returns200Profile(t *testing.T) {
	svc := new(services_mocks.AccountService)
	svc.On("GetProfile", "user-1").Return(&dto.Profile{ID: "user-1", Name: "P", PinSet: true}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodGet, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).GetMe(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	var p dto.Profile
	apitest.Data(t, rec, &p)
	assert.True(t, p.PinSet)
}

func TestPatchMe_ValidName_Returns200(t *testing.T) {
	svc := new(services_mocks.AccountService)
	svc.On("UpdateName", "user-1", "New").Return(&dto.Profile{Name: "New"}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPatch, Body: `{"name":"New"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).PatchMe(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPatchMe_MissingName_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPatch, Body: `{}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(new(services_mocks.AccountService)).PatchMe(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestPutPin_Valid_Returns204(t *testing.T) {
	svc := new(services_mocks.AccountService)
	svc.On("SetPin", "user-1", dto.SetPinRequest{Pin: "1234", ConfirmPin: "1234", Password: "pw"}).Return(nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: `{"pin":"1234","confirm_pin":"1234","password":"pw"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).PutPin(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestPutPin_ConfirmMismatch_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: `{"pin":"1234","confirm_pin":"9999"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(new(services_mocks.AccountService)).PutPin(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestPutPin_WrongPassword_Returns401InvalidPassword(t *testing.T) {
	svc := new(services_mocks.AccountService)
	svc.On("SetPin", "user-1", dto.SetPinRequest{Pin: "1234", ConfirmPin: "1234", Password: "bad"}).Return(errWrap.Wrap(respConst.INVALID_PASSWORD, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPut, Body: `{"pin":"1234","confirm_pin":"1234","password":"bad"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).PutPin(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, respConst.INVALID_PASSWORD, apitest.ErrCode(t, rec))
}

func TestVerifyPin_Correct_Returns204(t *testing.T) {
	svc := new(services_mocks.AccountService)
	svc.On("VerifyPin", "user-1", "1234").Return(nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"pin":"1234"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).VerifyPin(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestVerifyPin_ErrorMapping(t *testing.T) {
	for code, status := range map[string]int{
		respConst.INVALID_PIN:       http.StatusUnauthorized,
		respConst.TOO_MANY_ATTEMPTS: http.StatusTooManyRequests,
		respConst.PIN_NOT_SET:       http.StatusConflict,
	} {
		svc := new(services_mocks.AccountService)
		svc.On("VerifyPin", "user-1", "0000").Return(errWrap.Wrap(code, ""))
		c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"pin":"0000"}`, Claims: apitest.ParentClaims()})
		require.NoError(t, newCtrl(svc).VerifyPin(c))
		assert.Equal(t, status, rec.Code, code)
		assert.Equal(t, code, apitest.ErrCode(t, rec))
	}
}

func TestVerifyPin_MissingPin_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(new(services_mocks.AccountService)).VerifyPin(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestForgotPin_Correct_Returns204(t *testing.T) {
	svc := new(services_mocks.AccountService)
	svc.On("ForgotPin", "user-1", "pw").Return(nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"password":"pw"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).ForgotPin(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestForgotPin_Wrong_Returns401(t *testing.T) {
	svc := new(services_mocks.AccountService)
	svc.On("ForgotPin", "user-1", "bad").Return(errWrap.Wrap(respConst.INVALID_PASSWORD, ""))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"password":"bad"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).ForgotPin(c))
	assert.Equal(t, respConst.INVALID_PASSWORD, apitest.ErrCode(t, rec))
}

func TestForgotPin_MissingPassword_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(new(services_mocks.AccountService)).ForgotPin(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}
