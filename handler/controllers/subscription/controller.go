package subscription

import (
	helpers "codebase/internal/helper"
	"codebase/internal/services/subscription"
	"codebase/models"
	"codebase/models/dto"

	"github.com/labstack/echo/v4"
)

type Controller struct {
	Response helpers.Response
	Binding  helpers.Binding
	Service  subscription.Service
}

func NewController(holder *models.SharedHolder) *Controller {
	return &Controller{Response: helpers.NewResponse(holder), Binding: helpers.NewBinding(), Service: subscription.NewService(holder)}
}

// GetAdminPlan serves GET /admin/subscription-plan: the plan with its price history.
func (c *Controller) GetAdminPlan(ec echo.Context) error {
	p, err := c.Service.GetAdminPlan()
	return c.respond(ec, 0, p, err)
}

func (c *Controller) PutPlan(ec echo.Context) error {
	var req dto.UpdatePlanRequest
	if err := helpers.BindValid(c.Binding, ec, &req); err != nil {
		return c.respond(ec, 0, nil, err)
	}
	p, err := c.Service.UpdatePlan(helpers.ClaimsFrom(ec).AdminID, req.Price, req.Benefits)
	return c.respond(ec, 0, p, err)
}

func (c *Controller) respond(ec echo.Context, status int, data any, err error) error {
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, Status: status, RespData: data, ErrorData: err})
}
