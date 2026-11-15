package helpers

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"

	"github.com/labstack/echo/v4"
)

var validate = newValidator()

// newValidator reports fields by their json/param/query name, which is what clients send.
func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		for _, tag := range []string{"json", "param", "query", "form"} {
			if name := strings.Split(f.Tag.Get(tag), ",")[0]; name != "" && name != "-" {
				return name
			}
		}
		return f.Name
	})
	return v
}

type (
	BindingImpl struct{}

	Binding interface {
		Bind(customBinder CustomBinder) (err error)
	}

	request interface {
		Validate() interface{}
	}

	CustomBinder struct {
		Request request
		Context echo.Context
	}
)

func NewBinding() *BindingImpl {
	return &BindingImpl{}
}

func (b *BindingImpl) Bind(customBinder CustomBinder) (err error) {
	var (
		paramStruct, queryStruct, bodyStruct []reflect.StructField
		binder                               echo.DefaultBinder
	)

	if customBinder.Request == nil {
		return
	}

	//Split struct
	content := reflect.ValueOf(customBinder.Request.Validate())
	for i := 0; i < content.Type().NumField(); i++ {
		field := content.Type().Field(i)
		structField := reflect.StructField{
			Name: field.Name,
			Type: field.Type,
			Tag:  field.Tag,
		}
		if field.Tag.Get("param") != "" {
			paramStruct = append(paramStruct, structField)
		} else if field.Tag.Get("query") != "" {
			queryStruct = append(queryStruct, structField)
		} else if field.Tag.Get("json") != "" || field.Tag == "" {
			bodyStruct = append(bodyStruct, structField)
		}
	}

	//Bind Path Variable
	filledParam := initStruct(paramStruct)
	if err := binder.BindPathParams(customBinder.Context, filledParam); err != nil {
		return handleBindError(err)
	}
	if err := copyInto(filledParam, customBinder.Request); err != nil {
		return err
	}

	//Bind Query
	filledQuery := initStruct(queryStruct)
	if err := binder.BindQueryParams(customBinder.Context, filledQuery); err != nil {
		return handleBindError(err)
	}
	if err := copyInto(filledQuery, customBinder.Request); err != nil {
		return err
	}

	//Bind Body
	filledBody := initStruct(bodyStruct)
	if err := binder.BindBody(customBinder.Context, filledBody); err != nil {
		return handleBindError(err)
	}
	if err := copyInto(filledBody, customBinder.Request); err != nil {
		return err
	}

	if err = validate.Struct(customBinder.Request); err != nil {
		// If validation fails, return a bad request error
		var errs validator.ValidationErrors
		errors.As(err, &errs)
		var sb strings.Builder
		errorMessages := make(map[string]string)
		for _, e := range errs {
			field := e.Field()
			tag := e.Tag()
			var msg string
			switch tag {
			case "required":
				msg = field + " is a required field."
			case "email":
				msg = field + " must be a valid email address."
			case "matches":
				msg = field + " contains invalid input."
			case "min":
				msg = field + " must be at least " + e.Param() + " characters long."
			case "max":
				msg = field + " must be at most " + e.Param() + " characters long."
			case "len":
				msg = field + " must be exactly " + e.Param() + " characters long."
			case "oneof":
				msg = field + " must be one of: " + e.Param() + "."
			case "numeric":
				msg = field + " must contain digits only."
			case "eqfield":
				msg = field + " does not match."
			case "uuid":
				msg = field + " must be a valid id."
			case "gt", "gte":
				msg = field + " is out of range."
			default:
				msg = field + " is invalid."
			}
			errorMessages[field] = msg
			sb.WriteString(msg + " ") // Append each message with a newline
		}
		errorMessage := sb.String()
		return errors.New(strings.TrimSpace(errorMessage))
	}

	return
}

// copyInto moves the values bound onto the anonymous struct back onto the caller's request struct.
func copyInto(src any, dst request) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
