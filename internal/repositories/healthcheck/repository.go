package healthcheck

import (
	"context"
	"time"

	"codebase/models"
	"codebase/pkg/shared"
)

type (
	Repository interface {
		Ping(ctx context.Context, requestId string) error
	}

	Repo struct {
		sharedHolder *models.SharedHolder
		now          shared.Now
	}
)

func NewHealthcheckRepository(ctx context.Context, sharedHolder *models.SharedHolder) (Repository, error) {
	return &Repo{sharedHolder, time.Now}, nil
}
