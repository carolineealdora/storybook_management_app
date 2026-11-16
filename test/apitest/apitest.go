// Package apitest builds echo contexts for controller tests.
package apitest

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	helpers "codebase/internal/helper"
	"codebase/models"

	"github.com/labstack/echo/v4"
)

type Req struct {
	Method      string
	Body        string            // JSON body
	ContentType string            // defaults to application/json when Body is set
	Claims      *helpers.Claims   // set as if RequireAccount ran
	Params      map[string]string // path params
	Query       string            // raw query, e.g. "search=cat"
}

func Ctx(r Req) (echo.Context, *httptest.ResponseRecorder) {
	target := "/"
	if r.Query != "" {
		target += "?" + r.Query
	}
	req := httptest.NewRequest(r.Method, target, strings.NewReader(r.Body))
	if r.Body != "" {
		ct := r.ContentType
		if ct == "" {
			ct = echo.MIMEApplicationJSON
		}
		req.Header.Set(echo.HeaderContentType, ct)
	}
	rec := httptest.NewRecorder()
	e := echo.New()
	// echo caps path params at the router's max; register a dummy route so params survive.
	e.GET("/_/:p1/:p2/:p3", func(echo.Context) error { return nil })
	c := e.NewContext(req, rec)
	var names, values []string
	for k, v := range r.Params {
		names, values = append(names, k), append(values, v)
	}
	c.SetParamNames(names...)
	c.SetParamValues(values...)
	if r.Claims != nil {
		helpers.SetClaims(c, *r.Claims)
	}
	return c, rec
}

func ParentClaims() *helpers.Claims {
	c := helpers.Claims{AccountType: models.AccountParent, UserID: "user-1"}
	c.Subject = "cred-1"
	return &c
}

func AdminClaims() *helpers.Claims {
	c := helpers.Claims{AccountType: models.AccountAdmin, AdminID: "admin-1"}
	c.Subject = "cred-2"
	return &c
}

func Response() helpers.Response { return helpers.NewResponse(&models.SharedHolder{}) }

// ErrCode returns error.code from an error envelope ("" on success bodies).
func ErrCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var b models.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return b.Error.Code
}

// Data decodes the "data" field of a success envelope into out.
func Data(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	var b struct{ Data json.RawMessage }
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	if err := json.Unmarshal(b.Data, out); err != nil {
		t.Fatalf("decode data %q: %v", b.Data, err)
	}
}
