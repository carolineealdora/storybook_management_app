package models

import "github.com/labstack/echo/v4"

type DataResponse struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ResponsePayload struct {
	Desc     string
	HttpCode int
}

type ResponseArgs struct {
	Ctx       echo.Context
	Status    int // defaults to 200
	RespData  interface{}
	Meta      interface{}
	ErrorData error
}
