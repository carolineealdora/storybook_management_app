package healthcheck

import (
	"context"

	"codebase/models"

	"github.com/labstack/echo/v4"
)

func getStatus(status bool) string {
	if status {
		return "UP"
	}
	return "DOWN"
}

func (c *Controller) GetHealthcheck(ec echo.Context) error {
	var (
		requestId = ec.Request().Header.Get("x-request-id")
	)

	ctx := ec.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// check database connection
	databaseStatus := c.HealthcheckService.GetDatabaseStatus(ctx, requestId)

	//TODO: Refactor to use response builder
	return c.Response.CommonResponse(models.ResponseArgs{
		Ctx: ec,
		RespData: map[string]interface{}{
			"database": getStatus(databaseStatus),
		},
	})
}
