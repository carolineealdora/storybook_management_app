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

func (m *SubscriptionRepository) GetPlan() (*models.SubscriptionPlan, error) {
	a := m.Called()
	return ret[*models.SubscriptionPlan](a, 0), a.Error(1)
}

func (m *SubscriptionRepository) SavePlan(p *models.SubscriptionPlan) error {
	return m.Called(p).Error(0)
}
