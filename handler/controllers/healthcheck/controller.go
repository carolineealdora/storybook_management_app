package healthcheck

import (
	"context"

	helpers "codebase/internal/helper"
	"codebase/internal/services/healthcheck"
	"codebase/models"
)

type (
	Controller struct {
		sharedHolder *models.SharedHolder
		Response     helpers.Response

		//input services here
		HealthcheckService healthcheck.ViewService
	}
)

func NewController(holder *models.SharedHolder) Controller {
	response := helpers.NewResponse(holder)

	//init services here
	healthcheckService, err := healthcheck.NewViewService(context.Background(), holder)
	if err != nil {
		panic(err)
	}

	return Controller{
		sharedHolder:       holder,
		HealthcheckService: healthcheckService,
		Response:           response,
	}
}
