package order

import (
	"net/http"

	helpers "codebase/internal/helper"
	"codebase/internal/services/order"
	"codebase/models"
	"codebase/models/dto"

	"github.com/labstack/echo/v4"
)

type Controller struct {
	Response helpers.Response
	Binding  helpers.Binding
	Service  order.Service
}

func NewController(holder *models.SharedHolder) *Controller {
	return &Controller{Response: helpers.NewResponse(holder), Binding: helpers.NewBinding(), Service: order.NewService(holder)}
}

func (c *Controller) Create(ec echo.Context) error {
	var req dto.CreateBookOrderRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.respond(ec, 0, nil, err)
	}
	o, err := c.Service.Create(helpers.ClaimsFrom(ec).UserID, req.BookID, req.IdempotencyKey)
	return c.respond(ec, http.StatusCreated, o, err)
}

func (c *Controller) Pay(ec echo.Context) error {
	id, err := helpers.PathUUID(ec, "id")
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	var req dto.PayRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.respond(ec, 0, nil, err)
	}
	o, err := c.Service.Pay(helpers.ClaimsFrom(ec).UserID, id, req.Result)
	return c.respond(ec, http.StatusOK, o, err)
}

func (c *Controller) List(ec echo.Context) error {
	orders, err := c.Service.List(helpers.ClaimsFrom(ec).UserID)
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: orders, Meta: map[string]int{"total": len(orders)}})
}

func (c *Controller) respond(ec echo.Context, status int, data any, err error) error {
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, Status: status, RespData: data, ErrorData: err})
}
