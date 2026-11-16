package healthcheck

import (
	"context"
	"time"

	"codebase/internal/repositories/healthcheck"
	"codebase/models"
	"codebase/pkg/shared"
)

type (
	ViewService interface {
		GetDatabaseStatus(ctx context.Context, requestID string) bool
	}

	repositoryHolder struct {
		HealthcheckRepository healthcheck.Repository
	}

	service struct {
		sharedHolder     *models.SharedHolder
		repositoryHolder repositoryHolder
		now              shared.Now
	}
)

func NewViewService(ctx context.Context, sharedHolder *models.SharedHolder) (ViewService, error) {
	healthcheckRepo, err := healthcheck.NewHealthcheckRepository(ctx, sharedHolder)
	if err != nil {
		panic(err)
	}
	rHolder := repositoryHolder{
		HealthcheckRepository: healthcheckRepo,
	}

	return &service{sharedHolder, rHolder, time.Now}, nil
}
