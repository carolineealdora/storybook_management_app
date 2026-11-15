package auth

import (
	"errors"
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

func newCtrl(svc *services_mocks.AuthService) *Controller {
	return &Controller{Response: apitest.Response(), Binding: helpers.NewBinding(), AuthService: svc}
}

var errWrap = helpers.NewError(nil)

func TestLogin_ValidBody_Returns200WithTokens(t *testing.T) {
	svc := new(services_mocks.AuthService)
	svc.On("Login", "parent@example.com", "pw-123456", models.AccountParent).
		Return(&dto.TokenResponse{AccessToken: "at", RefreshToken: "rt", TokenType: "Bearer", ExpiresIn: 1800, AccountType: "parent"}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"email":"parent@example.com","password":"pw-123456"}`})

	require.NoError(t, newCtrl(svc).Login(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	var got dto.TokenResponse
	apitest.Data(t, rec, &got)
	assert.Equal(t, "at", got.AccessToken)
	assert.Equal(t, "rt", got.RefreshToken)
}

func TestLogin_MissingPassword_Returns400Validation(t *testing.T) {
	svc := new(services_mocks.AuthService)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"email":"parent@example.com"}`})
	require.NoError(t, newCtrl(svc).Login(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
	svc.AssertNotCalled(t, "Login")
}

func TestLogin_InvalidEmail_Returns400Validation(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"email":"nope","password":"x"}`})
	require.NoError(t, newCtrl(new(services_mocks.AuthService)).Login(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestLogin_MalformedJSON_Returns400Validation(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"email":`})
	require.NoError(t, newCtrl(new(services_mocks.AuthService)).Login(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestLogin_BadCredentials_Returns401(t *testing.T) {
	svc := new(services_mocks.AuthService)
	svc.On("Login", "parent@example.com", "bad", models.AccountParent).Return(nil, errWrap.Wrap(respConst.INVALID_CREDENTIALS, "x"))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"email":"parent@example.com","password":"bad"}`})
	require.NoError(t, newCtrl(svc).Login(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, respConst.INVALID_CREDENTIALS, apitest.ErrCode(t, rec))
}

func TestAdminLogin_ParentAccount_Returns403(t *testing.T) {
	svc := new(services_mocks.AuthService)
	svc.On("Login", "parent@example.com", "pw", models.AccountAdmin).Return(nil, errWrap.Wrap(respConst.FORBIDDEN, "x"))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"email":"parent@example.com","password":"pw"}`})
	require.NoError(t, newCtrl(svc).AdminLogin(c))
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, respConst.FORBIDDEN, apitest.ErrCode(t, rec))
}

func TestRefresh_ValidToken_Returns200(t *testing.T) {
	svc := new(services_mocks.AuthService)
	svc.On("Refresh", "rt").Return(&dto.TokenResponse{AccessToken: "at2", ExpiresIn: 1800}, nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"refresh_token":"rt"}`})
	require.NoError(t, newCtrl(svc).Refresh(c))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRefresh_Expired_Returns401RefreshTokenExpired(t *testing.T) {
	svc := new(services_mocks.AuthService)
	svc.On("Refresh", "rt").Return(nil, errWrap.Wrap(respConst.REFRESH_TOKEN_EXPIRED, "x"))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"refresh_token":"rt"}`})
	require.NoError(t, newCtrl(svc).Refresh(c))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, respConst.REFRESH_TOKEN_EXPIRED, apitest.ErrCode(t, rec))
}

func TestRefresh_MissingToken_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{}`})
	require.NoError(t, newCtrl(new(services_mocks.AuthService)).Refresh(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}

func TestLogout_Valid_Returns204AndRevokesForCaller(t *testing.T) {
	svc := new(services_mocks.AuthService)
	svc.On("Logout", "cred-1", "rt").Return(nil)
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"refresh_token":"rt"}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(svc).Logout(c))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	svc.AssertExpectations(t)
}

func TestLogout_ServiceError_Returns500(t *testing.T) {
	svc := new(services_mocks.AuthService)
	svc.On("Logout", "cred-2", "rt").Return(errors.New("db down"))
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{"refresh_token":"rt"}`, Claims: apitest.AdminClaims()})
	require.NoError(t, newCtrl(svc).Logout(c))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestLogout_MissingToken_Returns400(t *testing.T) {
	c, rec := apitest.Ctx(apitest.Req{Method: http.MethodPost, Body: `{}`, Claims: apitest.ParentClaims()})
	require.NoError(t, newCtrl(new(services_mocks.AuthService)).Logout(c))
	assert.Equal(t, respConst.VALIDATION_ERROR, apitest.ErrCode(t, rec))
}
