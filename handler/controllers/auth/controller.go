package auth

import (
	"net/http"

	helpers "codebase/internal/helper"
	"codebase/internal/services/auth"
	"codebase/models"
	"codebase/models/dto"

	"github.com/labstack/echo/v4"
)

type Controller struct {
	Response    helpers.Response
	Binding     helpers.Binding
	AuthService auth.Service
}

func NewController(holder *models.SharedHolder) *Controller {
	return &Controller{Response: helpers.NewResponse(holder), Binding: helpers.NewBinding(), AuthService: auth.NewService(holder)}
}

func (c *Controller) AdminLogin(ec echo.Context) error { return c.login(ec, models.AccountAdmin) }

func (c *Controller) login(ec echo.Context, accountType string) error {
	var req dto.LoginRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.fail(ec, err)
	}
	resp, err := c.AuthService.Login(req.Email, req.Password, accountType)
	if err != nil {
		return c.fail(ec, err)
	}
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: resp})
}

func (c *Controller) Refresh(ec echo.Context) error {
	var req dto.RefreshRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.fail(ec, err)
	}
	resp, err := c.AuthService.Refresh(req.RefreshToken)
	if err != nil {
		return c.fail(ec, err)
	}
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: resp})
}

// Logout serves both /auth/logout and /admin/auth/logout; the caller comes from the access token.
func (c *Controller) Logout(ec echo.Context) error {
	var req dto.RefreshRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.fail(ec, err)
	}
	if err := c.AuthService.Logout(helpers.ClaimsFrom(ec).Subject, req.RefreshToken); err != nil {
		return c.fail(ec, err)
	}
	return ec.NoContent(http.StatusNoContent)
}

func (c *Controller) fail(ec echo.Context, err error) error {
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, ErrorData: err})
}
