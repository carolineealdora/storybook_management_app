package helpers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	respConst "codebase/internal/constant/response"
	"codebase/models"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sampleReq struct {
	ID     string `param:"id" validate:"required,uuid"`
	Search string `query:"search" validate:"max=5"`
	Email  string `json:"email" validate:"required,email"`
	Pin    string `json:"pin" validate:"omitempty,len=4,numeric"`
	Again  string `json:"again" validate:"omitempty,eqfield=Pin"`
	Kind   string `json:"kind" validate:"omitempty,oneof=a b"`
	Count  int    `json:"count" validate:"omitempty,gt=0"`
	Name   string `json:"name" validate:"omitempty,min=2"`
}

func (r sampleReq) Validate() interface{} { return r }

const uuidV = "33333333-3333-3333-3333-333333333333"

func bindCtx(body, query, id string) echo.Context {
	e := echo.New()
	e.GET("/_/:id", func(echo.Context) error { return nil })
	req := httptest.NewRequest(http.MethodPost, "/?"+query, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, httptest.NewRecorder())
	c.SetParamNames("id")
	c.SetParamValues(id)
	return c
}

func TestBind_ParamQueryAndBody_AllFilled(t *testing.T) {
	var r sampleReq
	err := NewBinding().Bind(CustomBinder{Request: &r, Context: bindCtx(`{"email":"a@b.co","pin":"1234","again":"1234","kind":"a","count":2}`, "search=cat", uuidV)})
	require.NoError(t, err)
	assert.Equal(t, uuidV, r.ID)
	assert.Equal(t, "cat", r.Search)
	assert.Equal(t, "a@b.co", r.Email)
	assert.Equal(t, 2, r.Count)
}

func TestBind_ValidationMessages_UseClientFieldNames(t *testing.T) {
	cases := map[string]string{
		`{"pin":"12"}`:                                "email is a required field.",
		`{"email":"nope"}`:                            "email must be a valid email address.",
		`{"email":"a@b.co","pin":"12"}`:               "pin must be exactly 4 characters long.",
		`{"email":"a@b.co","pin":"12ab"}`:             "pin must contain digits only.",
		`{"email":"a@b.co","pin":"1234","again":"1"}`: "again does not match.",
		`{"email":"a@b.co","kind":"z"}`:               "kind must be one of: a b.",
		`{"email":"a@b.co","count":-1}`:               "count is out of range.",
		`{"email":"a@b.co","name":"x"}`:               "name must be at least 2 characters long.",
	}
	for body, want := range cases {
		var r sampleReq
		err := NewBinding().Bind(CustomBinder{Request: &r, Context: bindCtx(body, "", uuidV)})
		require.Error(t, err, body)
		assert.Contains(t, err.Error(), want, body)
	}
}

func TestBind_BadParamAndQuery_Rejected(t *testing.T) {
	var r sampleReq
	err := NewBinding().Bind(CustomBinder{Request: &r, Context: bindCtx(`{"email":"a@b.co"}`, "", "not-a-uuid")})
	assert.ErrorContains(t, err, "id must be a valid id.")
	err = NewBinding().Bind(CustomBinder{Request: &r, Context: bindCtx(`{"email":"a@b.co"}`, "search=toolong", uuidV)})
	assert.ErrorContains(t, err, "search must be at most 5 characters long.")
}

func TestBind_WrongJSONType_ExplainsField(t *testing.T) {
	var r sampleReq
	err := NewBinding().Bind(CustomBinder{Request: &r, Context: bindCtx(`{"email":"a@b.co","count":"two"}`, "", uuidV)})
	assert.ErrorContains(t, err, "must be int")
}

func TestBind_NilRequest_NoOp(t *testing.T) {
	assert.NoError(t, NewBinding().Bind(CustomBinder{Context: bindCtx("", "", uuidV)}))
}

func TestBindValid_WrapsAsValidationError(t *testing.T) {
	var r sampleReq
	err := BindValid(NewBinding(), bindCtx(`{}`, "", uuidV), &r)
	assert.Equal(t, respConst.VALIDATION_ERROR, ErrCode(err))
	assert.NoError(t, BindValid(NewBinding(), bindCtx(`{"email":"a@b.co"}`, "", uuidV), &r))
}

func TestPathUUID_ValidAndInvalid(t *testing.T) {
	id, err := PathUUID(bindCtx("", "", uuidV), "id")
	require.NoError(t, err)
	assert.Equal(t, uuidV, id)
	_, err = PathUUID(bindCtx("", "", "1; DROP TABLE books"), "id")
	assert.Equal(t, respConst.VALIDATION_ERROR, ErrCode(err))
}

func TestHTTPErrorHandler_EchoErrors_MappedToEnvelope(t *testing.T) {
	cases := []struct {
		err  error
		code string
		http int
	}{
		{echo.ErrNotFound, respConst.NOT_FOUND, 404},
		{echo.ErrMethodNotAllowed, respConst.METHOD_NOT_ALLOWED, 405},
		{echo.ErrStatusRequestEntityTooLarge, respConst.PAYLOAD_TOO_LARGE, 413},
		{echo.NewHTTPError(http.StatusBadRequest, "bad multipart"), respConst.VALIDATION_ERROR, 400},
		{echo.ErrUnauthorized, respConst.UNAUTHORIZED, 401},
		{errors.New("panic: nil map"), respConst.GENERAL_ERROR, 500},
	}
	h := NewHTTPErrorHandler(&models.SharedHolder{})
	for _, c := range cases {
		ctx, rec := newCtx()
		h(c.err, ctx)
		assert.Equal(t, c.http, rec.Code, c.code)
		b := decodeErr(t, rec)
		assert.Equal(t, c.code, b.Error.Code)
		assert.NotContains(t, b.Error.Message, "nil map")
	}
}

func TestHTTPErrorHandler_ResponseAlreadyCommitted_Untouched(t *testing.T) {
	ctx, rec := newCtx()
	require.NoError(t, ctx.String(http.StatusOK, "done"))
	NewHTTPErrorHandler(&models.SharedHolder{})(echo.ErrNotFound, ctx)
	assert.Equal(t, "done", rec.Body.String())
}

func TestClaimsFrom_Unset_ReturnsZero(t *testing.T) {
	ctx, _ := newCtx()
	assert.Equal(t, Claims{}, ClaimsFrom(ctx))
}
