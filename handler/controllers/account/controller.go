package account

import (
	"net/http"

	helpers "codebase/internal/helper"
	"codebase/internal/services/account"
	"codebase/models"
	"codebase/models/dto"

	"github.com/labstack/echo/v4"
)

type Controller struct {
	Response       helpers.Response
	Binding        helpers.Binding
	AccountService account.Service
}

func NewController(holder *models.SharedHolder) *Controller {
	return &Controller{Response: helpers.NewResponse(holder), Binding: helpers.NewBinding(), AccountService: account.NewService(holder)}
}

func (c *Controller) GetMe(ec echo.Context) error {
	p, err := c.AccountService.GetProfile(helpers.ClaimsFrom(ec).UserID)
	return c.respond(ec, p, err)
}

func (c *Controller) PatchMe(ec echo.Context) error {
	var req dto.UpdateNameRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.respond(ec, nil, err)
	}
	p, err := c.AccountService.UpdateName(helpers.ClaimsFrom(ec).UserID, req.Name)
	return c.respond(ec, p, err)
}

func (c *Controller) PutPin(ec echo.Context) error {
	var req dto.SetPinRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.respond(ec, nil, err)
	}
	return c.noContent(ec, c.AccountService.SetPin(helpers.ClaimsFrom(ec).UserID, req))
}

func (c *Controller) VerifyPin(ec echo.Context) error {
	var req dto.VerifyPinRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.respond(ec, nil, err)
	}
	return c.noContent(ec, c.AccountService.VerifyPin(helpers.ClaimsFrom(ec).UserID, req.Pin))
}

func (c *Controller) ForgotPin(ec echo.Context) error {
	var req dto.PasswordRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.respond(ec, nil, err)
	}
	return c.noContent(ec, c.AccountService.ForgotPin(helpers.ClaimsFrom(ec).UserID, req.Password))
}

func (c *Controller) respond(ec echo.Context, data any, err error) error {
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: data, ErrorData: err})
}

func (c *Controller) noContent(ec echo.Context, err error) error {
	if err != nil {
		return c.respond(ec, nil, err)
	}
	return ec.NoContent(http.StatusNoContent)
}
