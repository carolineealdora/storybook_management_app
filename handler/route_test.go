package handler

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"codebase/config"
	helpers "codebase/internal/helper"
	"codebase/models"
	sql_mock "codebase/test/mock/sql"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSecret = "test-secret-test-secret-test-secret!!"

func newTestHolder() *models.SharedHolder {
	orm := new(sql_mock.ORMWrapperMock)
	orm.On("Ping").Return(nil)
	return &models.SharedHolder{
		Config: &config.SystemConfig{JWTSecret: testSecret},
		Echo:   echo.New(),
		Sql:    orm,
	}
}

func serve(h *models.SharedHolder, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.Echo.ServeHTTP(rec, req)
	return rec
}

func TestRoute_UnknownPath_Returns404Envelope(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.JSONEq(t, `{"error":{"code":"NOT_FOUND","message":"Resource not found."}}`, rec.Body.String())
}

func TestRoute_NoRequestIDHeader_GeneratesOne(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/v1/healthcheck", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(echo.HeaderXRequestID))
}

func TestRoute_RequestIDHeader_Echoed(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/healthcheck", nil)
	req.Header.Set(echo.HeaderXRequestID, "req-123")
	rec := serve(h, req)
	assert.Equal(t, "req-123", rec.Header().Get(echo.HeaderXRequestID))
}

func TestRoute_NoPartnerHeader_NotRequired(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	rec := serve(h, httptest.NewRequest(http.MethodGet, "/api/v1/healthcheck", nil))
	assert.JSONEq(t, `{"data":{"database":"UP"}}`, rec.Body.String())
}

// --- Security sweep over every registered route ---------------------------------------

var publicRoutes = map[string]bool{
	"GET /api/v1/healthcheck":       true,
	"POST /api/v1/auth/refresh":     true,
	"POST /api/v1/admin/auth/login": true,
}

// expectedRoutes guards against an endpoint silently going missing. This backend serves
// only fe_storybook_management_app; parent endpoints live in storybook_library_app (ADR-011).
var expectedRoutes = []string{
	"POST /api/v1/auth/refresh",
	"POST /api/v1/admin/auth/login",
	"POST /api/v1/admin/auth/logout",
	"GET /api/v1/admin/books",
	"POST /api/v1/admin/books",
	"GET /api/v1/admin/books/:id",
	"PUT /api/v1/admin/books/:id",
	"DELETE /api/v1/admin/books/:id",
	"GET /api/v1/admin/subscription-plan",
	"PUT /api/v1/admin/subscription-plan",
	"GET /api/v1/books/:id/cover",
}

// removedParentRoutes must not exist here: a parent could otherwise bypass the library
// backend's parent-mode token and Forgot PIN lockout (STORY-4, F1).
var removedParentRoutes = []string{
	"POST /api/v1/auth/login",
	"POST /api/v1/auth/logout",
	"GET /api/v1/me",
	"PATCH /api/v1/me",
	"PUT /api/v1/me/pin",
	"POST /api/v1/me/pin/verify",
	"POST /api/v1/me/pin/forgot",
	"GET /api/v1/me/books",
	"GET /api/v1/books",
	"GET /api/v1/books/:id",
	"GET /api/v1/books/:id/content",
	"POST /api/v1/book-orders",
	"GET /api/v1/book-orders",
	"POST /api/v1/book-orders/:id/pay",
	"GET /api/v1/subscription/plan",
	"POST /api/v1/subscription-orders",
	"POST /api/v1/subscription-orders/:id/pay",
	"POST /api/v1/subscriptions/current/cancel",
	"GET /api/v1/subscriptions",
}

var paramRe = regexp.MustCompile(`:[A-Za-z]+`)

func apiRoutes(h *models.SharedHolder) []*echo.Route {
	var out []*echo.Route
	for _, r := range h.Echo.Routes() {
		if strings.HasPrefix(r.Path, "/api/v1/") {
			out = append(out, r)
		}
	}
	return out
}

func bearer(t *testing.T, accountType string) string {
	t.Helper()
	c := helpers.Claims{AccountType: accountType, UserID: "11111111-1111-1111-1111-111111111111"}
	c.Subject = "cred"
	c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(time.Minute))
	tok, err := helpers.SignAccessToken([]byte(testSecret), c)
	require.NoError(t, err)
	return "Bearer " + tok
}

func call(h *models.SharedHolder, r *echo.Route, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(r.Method, paramRe.ReplaceAllString(r.Path, "11111111-1111-1111-1111-111111111111"), nil)
	if auth != "" {
		req.Header.Set(echo.HeaderAuthorization, auth)
	}
	return serve(h, req)
}

func TestRoutes_AllPRDEndpoints_Registered(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	have := map[string]bool{}
	for _, r := range apiRoutes(h) {
		have[r.Method+" "+r.Path] = true
	}
	for _, want := range expectedRoutes {
		assert.True(t, have[want], "missing route %s", want)
	}
}

func TestRoutes_ProtectedWithoutToken_Return401(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	checked := 0
	for _, r := range apiRoutes(h) {
		key := r.Method + " " + r.Path
		if publicRoutes[key] {
			continue
		}
		checked++
		rec := call(h, r, "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code, key)
		assert.Contains(t, rec.Body.String(), `"UNAUTHORIZED"`, key)
	}
	assert.GreaterOrEqual(t, checked, 2)
}

func TestRoutes_EveryProtectedRouteWithParentToken_Returns403(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	auth := bearer(t, models.AccountParent)
	checked := 0
	for _, r := range apiRoutes(h) {
		key := r.Method + " " + r.Path
		if publicRoutes[key] {
			continue
		}
		checked++
		rec := call(h, r, auth)
		assert.Equal(t, http.StatusForbidden, rec.Code, key)
	}
	assert.GreaterOrEqual(t, checked, len(expectedRoutes)-len(publicRoutes)+1)
}

func TestRoute_CoverWithParentToken_Returns403(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/books/11111111-1111-1111-1111-111111111111/cover", nil)
	req.Header.Set(echo.HeaderAuthorization, bearer(t, models.AccountParent))
	assert.Equal(t, http.StatusForbidden, serve(h, req).Code)
}

func TestRoute_ParentEndpoints_Return404(t *testing.T) {
	h := newTestHolder()
	InitRoute(h)
	auth := bearer(t, models.AccountParent)
	for _, key := range removedParentRoutes {
		method, path, _ := strings.Cut(key, " ")
		req := httptest.NewRequest(method, paramRe.ReplaceAllString(path, "11111111-1111-1111-1111-111111111111"), nil)
		req.Header.Set(echo.HeaderAuthorization, auth)
		rec := serve(h, req)
		assert.Equal(t, http.StatusNotFound, rec.Code, key)
		assert.Contains(t, rec.Body.String(), `"NOT_FOUND"`, key)
	}
}
