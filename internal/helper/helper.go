package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"

	respConst "codebase/internal/constant/response"
	"codebase/models"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func init() {
	if err := validate.RegisterValidation("matches", func(fl validator.FieldLevel) bool {
		return regexp.MustCompile(fl.Param()).MatchString(fl.Field().String())
	}); err != nil {
		panic(err)
	}
}

// NewHTTPErrorHandler renders echo's own errors (unknown route, wrong method, body too large, panics)
// in the same `{"error":{…}}` envelope as the rest of the API.
func NewHTTPErrorHandler(holder *models.SharedHolder) echo.HTTPErrorHandler {
	response := NewResponse(holder)
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		code := respConst.GENERAL_ERROR
		var he *echo.HTTPError
		if errors.As(err, &he) {
			switch he.Code {
			case http.StatusNotFound:
				code = respConst.NOT_FOUND
			case http.StatusMethodNotAllowed:
				code = respConst.METHOD_NOT_ALLOWED
			case http.StatusRequestEntityTooLarge:
				code = respConst.PAYLOAD_TOO_LARGE
			case http.StatusBadRequest, http.StatusUnsupportedMediaType:
				code = respConst.VALIDATION_ERROR
			case http.StatusUnauthorized:
				code = respConst.UNAUTHORIZED
			}
		}
		info := err.Error()
		if code == respConst.VALIDATION_ERROR && he != nil {
			info = fmt.Sprint(he.Message)
		}
		if rerr := response.CommonResponse(models.ResponseArgs{Ctx: c, ErrorData: NewError(holder).Wrap(code, info)}); rerr != nil {
			c.Logger().Error(rerr)
		}
	}
}

func SlicesContains(arrayData []string, str string) bool {
	for _, data := range arrayData {
		if data == str {
			return true
		}
	}
	return false
}

func initStruct(data []reflect.StructField) any {
	typ := reflect.StructOf(data)
	val := reflect.New(typ).Elem()
	obj := val.Addr().Interface()
	return obj
}

func handleBindError(err error) error {
	var unmarshalTypeError *json.UnmarshalTypeError
	if errors.As(err, &unmarshalTypeError) {
		msg := fmt.Sprintf("Field '%s' must be %s, but got %s",
			unmarshalTypeError.Field,
			unmarshalTypeError.Type.String(),
			unmarshalTypeError.Value,
		)
		return errors.New(msg)
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		var ute *json.UnmarshalTypeError
		if errors.As(he.Internal, &ute) {
			return handleBindError(ute)
		}
		return fmt.Errorf("%v", he.Message)
	}
	return err
}
