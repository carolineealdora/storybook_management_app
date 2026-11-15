package helpers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	respConst "codebase/internal/constant/response"
	"codebase/models"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCtx() (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

type errBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeErr(t *testing.T, rec *httptest.ResponseRecorder) errBody {
	t.Helper()
	var b errBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	return b
}

func TestCommonResponse_Success_WrapsDataEnvelope(t *testing.T) {
	c, rec := newCtx()
	err := NewResponse(&models.SharedHolder{}).CommonResponse(models.ResponseArgs{Ctx: c, RespData: map[string]string{"a": "b"}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"data":{"a":"b"}}`, rec.Body.String())
}

func TestCommonResponse_CustomStatusAndMeta_Returned(t *testing.T) {
	c, rec := newCtx()
	err := NewResponse(&models.SharedHolder{}).CommonResponse(models.ResponseArgs{
		Ctx: c, Status: http.StatusCreated, RespData: []int{1, 2}, Meta: map[string]int{"total": 2},
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.JSONEq(t, `{"data":[1,2],"meta":{"total":2}}`, rec.Body.String())
}

func TestCommonResponse_CodedError_MapsToPRDStatus(t *testing.T) {
	cases := map[string]int{
		respConst.VALIDATION_ERROR:        400,
		respConst.INVALID_FILE_FORMAT:     400,
		respConst.INVALID_AGE_RANGE:       400,
		respConst.UNAUTHORIZED:            401,
		respConst.TOKEN_EXPIRED:           401,
		respConst.REFRESH_TOKEN_EXPIRED:   401,
		respConst.INVALID_CREDENTIALS:     401,
		respConst.INVALID_PIN:             401,
		respConst.INVALID_PASSWORD:        401,
		respConst.PAYMENT_FAILED:          402,
		respConst.FORBIDDEN:               403,
		respConst.BOOK_LOCKED:             403,
		respConst.BOOK_NOT_FOUND:          404,
		respConst.ORDER_NOT_FOUND:         404,
		respConst.PLAN_NOT_FOUND:          404,
		respConst.NOT_FOUND:               404,
		respConst.METHOD_NOT_ALLOWED:      405,
		respConst.BOOK_ALREADY_OWNED:      409,
		respConst.SUBSCRIPTION_ACTIVE:     409,
		respConst.BOOK_UNAVAILABLE:        409,
		respConst.DUPLICATE_REQUEST:       409,
		respConst.BOOK_NOT_PURCHASABLE:    409,
		respConst.PIN_NOT_SET:             409,
		respConst.SUBSCRIPTION_NOT_ACTIVE: 409,
		respConst.PAYLOAD_TOO_LARGE:       413,
		respConst.TOO_MANY_ATTEMPTS:       429,
		respConst.GENERAL_ERROR:           500,
	}
	for code, status := range cases {
		t.Run(code, func(t *testing.T) {
			c, rec := newCtx()
			err := NewResponse(&models.SharedHolder{}).CommonResponse(models.ResponseArgs{
				Ctx: c, ErrorData: NewError(nil).Wrap(code, "secret internal detail"),
			})
			require.NoError(t, err)
			assert.Equal(t, status, rec.Code)
			b := decodeErr(t, rec)
			assert.Equal(t, code, b.Error.Code)
			assert.NotEmpty(t, b.Error.Message)
			if code != respConst.VALIDATION_ERROR {
				assert.NotContains(t, b.Error.Message, "secret internal detail")
			}
		})
	}
}

func TestCommonResponse_InvalidCredentials_UsesPRDMessage(t *testing.T) {
	c, rec := newCtx()
	_ = NewResponse(&models.SharedHolder{}).CommonResponse(models.ResponseArgs{
		Ctx: c, ErrorData: NewError(nil).Wrap(respConst.INVALID_CREDENTIALS, "bcrypt mismatch"),
	})
	assert.Equal(t, "Email or password is incorrect.", decodeErr(t, rec).Error.Message)
}

func TestCommonResponse_ValidationError_ExposesDetail(t *testing.T) {
	c, rec := newCtx()
	_ = NewResponse(&models.SharedHolder{}).CommonResponse(models.ResponseArgs{
		Ctx: c, ErrorData: NewError(nil).Wrap(respConst.VALIDATION_ERROR, "name is a required field."),
	})
	assert.Equal(t, 400, rec.Code)
	assert.Equal(t, "name is a required field.", decodeErr(t, rec).Error.Message)
}

func TestCommonResponse_PlainError_Returns500WithoutLeaking(t *testing.T) {
	c, rec := newCtx()
	_ = NewResponse(&models.SharedHolder{}).CommonResponse(models.ResponseArgs{
		Ctx: c, ErrorData: errors.New("pq: relation books does not exist"),
	})
	assert.Equal(t, 500, rec.Code)
	b := decodeErr(t, rec)
	assert.Equal(t, respConst.GENERAL_ERROR, b.Error.Code)
	assert.NotContains(t, b.Error.Message, "relation")
}

func TestCommonResponse_UnknownCode_Returns500(t *testing.T) {
	c, rec := newCtx()
	_ = NewResponse(&models.SharedHolder{}).CommonResponse(models.ResponseArgs{
		Ctx: c, ErrorData: NewError(nil).Wrap("NOT_A_REAL_CODE", "x"),
	})
	assert.Equal(t, 500, rec.Code)
	assert.Equal(t, respConst.GENERAL_ERROR, decodeErr(t, rec).Error.Code)
}

func TestErrorWrap_ErrorString_IncludesCodeAndInfo(t *testing.T) {
	err := NewError(nil).Wrap(respConst.BOOK_LOCKED, "user u1 book b1")
	assert.Equal(t, "BOOK_LOCKED: user u1 book b1", err.Error())
}

func TestErrCode_CodedPlainAndNil(t *testing.T) {
	assert.Equal(t, respConst.BOOK_LOCKED, ErrCode(NewError(nil).Wrap(respConst.BOOK_LOCKED, "")))
	assert.Equal(t, respConst.GENERAL_ERROR, ErrCode(errors.New("boom")))
	assert.Equal(t, "", ErrCode(nil))
}
