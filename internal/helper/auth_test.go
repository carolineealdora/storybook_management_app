package helpers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"codebase/config"
	respConst "codebase/internal/constant/response"
	"codebase/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testSecret = []byte("test-secret-test-secret-test-secret!!")

func parentClaims(exp time.Time) Claims {
	return Claims{
		AccountType:      models.AccountParent,
		UserID:           "user-1",
		RegisteredClaims: jwt.RegisteredClaims{Subject: "cred-1", ExpiresAt: jwt.NewNumericDate(exp)},
	}
}

func TestParseAccessToken_ValidToken_ReturnsClaims(t *testing.T) {
	tok, err := SignAccessToken(testSecret, parentClaims(time.Now().Add(time.Minute)))
	require.NoError(t, err)
	c, err := ParseAccessToken(testSecret, tok)
	require.NoError(t, err)
	assert.Equal(t, "cred-1", c.Subject)
	assert.Equal(t, "user-1", c.UserID)
	assert.Equal(t, models.AccountParent, c.AccountType)
}

func TestParseAccessToken_Expired_ReturnsErrTokenExpired(t *testing.T) {
	tok, err := SignAccessToken(testSecret, parentClaims(time.Now().Add(-time.Minute)))
	require.NoError(t, err)
	_, err = ParseAccessToken(testSecret, tok)
	assert.ErrorIs(t, err, ErrAccessTokenExpired)
}

func TestParseAccessToken_WrongSecret_ReturnsInvalid(t *testing.T) {
	tok, err := SignAccessToken([]byte("another-secret-another-secret-0000"), parentClaims(time.Now().Add(time.Minute)))
	require.NoError(t, err)
	_, err = ParseAccessToken(testSecret, tok)
	assert.ErrorIs(t, err, ErrInvalidAccessToken)
}

func TestParseAccessToken_AlgNone_Rejected(t *testing.T) {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, parentClaims(time.Now().Add(time.Minute))).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = ParseAccessToken(testSecret, tok)
	assert.ErrorIs(t, err, ErrInvalidAccessToken)
}

func runMiddleware(t *testing.T, accountType, authHeader string) (*httptest.ResponseRecorder, *Claims) {
	t.Helper()
	holder := &models.SharedHolder{Config: &config.SystemConfig{JWTSecret: string(testSecret)}}
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if authHeader != "" {
		req.Header.Set(echo.HeaderAuthorization, authHeader)
	}
	rec := httptest.NewRecorder()
	var seen *Claims
	h := RequireAccount(holder, accountType)(func(c echo.Context) error {
		cl := ClaimsFrom(c)
		seen = &cl
		return c.NoContent(http.StatusNoContent)
	})
	require.NoError(t, h(e.NewContext(req, rec)))
	return rec, seen
}

func TestRequireAccount_NoToken_Returns401Unauthorized(t *testing.T) {
	rec, seen := runMiddleware(t, models.AccountParent, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, respConst.UNAUTHORIZED, decodeErr(t, rec).Error.Code)
	assert.Nil(t, seen)
}

func TestRequireAccount_MalformedHeader_Returns401Unauthorized(t *testing.T) {
	rec, _ := runMiddleware(t, models.AccountParent, "Token abc")
	assert.Equal(t, respConst.UNAUTHORIZED, decodeErr(t, rec).Error.Code)
	rec, _ = runMiddleware(t, models.AccountParent, "Bearer not-a-jwt")
	assert.Equal(t, respConst.UNAUTHORIZED, decodeErr(t, rec).Error.Code)
}

func TestRequireAccount_ExpiredToken_Returns401TokenExpired(t *testing.T) {
	tok, _ := SignAccessToken(testSecret, parentClaims(time.Now().Add(-time.Second)))
	rec, _ := runMiddleware(t, models.AccountParent, "Bearer "+tok)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, respConst.TOKEN_EXPIRED, decodeErr(t, rec).Error.Code)
}

func TestRequireAccount_ParentOnAdminRoute_Returns403Forbidden(t *testing.T) {
	tok, _ := SignAccessToken(testSecret, parentClaims(time.Now().Add(time.Minute)))
	rec, seen := runMiddleware(t, models.AccountAdmin, "Bearer "+tok)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, respConst.FORBIDDEN, decodeErr(t, rec).Error.Code)
	assert.Nil(t, seen)
}

func TestRequireAccount_ValidToken_SetsClaims(t *testing.T) {
	tok, _ := SignAccessToken(testSecret, parentClaims(time.Now().Add(time.Minute)))
	rec, seen := runMiddleware(t, models.AccountParent, "Bearer "+tok)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.NotNil(t, seen)
	assert.Equal(t, "user-1", seen.UserID)
	assert.Equal(t, "cred-1", seen.Subject)
}
