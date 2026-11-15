package healthcheck

import (
	"context"
)

func (s *service) GetDatabaseStatus(ctx context.Context, requestID string) bool {
	err := s.repositoryHolder.HealthcheckRepository.Ping(ctx, requestID)
	if err != nil {
		return false
	} else {
		return true
	}
}
