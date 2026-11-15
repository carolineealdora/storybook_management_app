package repository_mock

import (
	"codebase/models"

	"github.com/stretchr/testify/mock"
)

type SubscriptionRepository struct{ mock.Mock }

func (m *SubscriptionRepository) GetActive(userID string) (*models.Subscription, error) {
	a := m.Called(userID)
	return ret[*models.Subscription](a, 0), a.Error(1)
}
