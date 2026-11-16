package repository_mock

import (
	"time"

	"codebase/models"

	"github.com/stretchr/testify/mock"
)

type SubscriptionRepository struct{ mock.Mock }

func (m *SubscriptionRepository) GetPlan() (*models.SubscriptionPlan, error) {
	a := m.Called()
	return ret[*models.SubscriptionPlan](a, 0), a.Error(1)
}

func (m *SubscriptionRepository) SavePlan(p *models.SubscriptionPlan) error {
	return m.Called(p).Error(0)
}

func (m *SubscriptionRepository) ListPriceHistory(planID string) ([]models.PlanPriceChange, error) {
	a := m.Called(planID)
	return ret[[]models.PlanPriceChange](a, 0), a.Error(1)
}

func (m *SubscriptionRepository) CountActiveSubscribers(now time.Time) (int, error) {
	a := m.Called(now)
	return a.Int(0), a.Error(1)
}
