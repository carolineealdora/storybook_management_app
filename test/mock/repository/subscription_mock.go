package repository_mock

import (
	"time"

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

func (m *SubscriptionRepository) CreateOrder(o *models.SubscriptionOrder) error {
	return m.Called(o).Error(0)
}

func (m *SubscriptionRepository) GetOrder(id, userID string) (*models.SubscriptionOrder, error) {
	a := m.Called(id, userID)
	return ret[*models.SubscriptionOrder](a, 0), a.Error(1)
}

func (m *SubscriptionRepository) MarkOrderFailed(id string, at time.Time) error {
	return m.Called(id, at).Error(0)
}

func (m *SubscriptionRepository) Activate(o *models.SubscriptionOrder, sub *models.Subscription, at time.Time) error {
	return m.Called(o, sub, at).Error(0)
}

func (m *SubscriptionRepository) Cancel(subID string, at time.Time) error {
	return m.Called(subID, at).Error(0)
}

func (m *SubscriptionRepository) ListHistory(userID string) ([]models.SubscriptionHistoryRow, error) {
	a := m.Called(userID)
	return ret[[]models.SubscriptionHistoryRow](a, 0), a.Error(1)
}

func (m *SubscriptionRepository) ListDue(now time.Time, limit int) ([]models.Subscription, error) {
	a := m.Called(now, limit)
	return ret[[]models.Subscription](a, 0), a.Error(1)
}

func (m *SubscriptionRepository) Renew(old, next *models.Subscription, order *models.SubscriptionOrder) error {
	return m.Called(old, next, order).Error(0)
}
