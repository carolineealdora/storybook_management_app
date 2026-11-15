package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"codebase/config"
	"codebase/models"
	sql_mock "codebase/test/mock/sql"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
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
