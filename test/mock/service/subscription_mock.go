package services_mocks

import (
	"codebase/models/dto"

	"github.com/stretchr/testify/mock"
)

type SubscriptionService struct{ mock.Mock }

func (m *SubscriptionService) GetPlan() (*dto.PlanView, error) {
	a := m.Called()
	return ret[*dto.PlanView](a, 0), a.Error(1)
}

func (m *SubscriptionService) GetAdminPlan() (*dto.AdminPlanView, error) {
	a := m.Called()
	return ret[*dto.AdminPlanView](a, 0), a.Error(1)
}

func (m *SubscriptionService) UpdatePlan(adminID string, price int, benefits string) (*dto.AdminPlanView, error) {
	a := m.Called(adminID, price, benefits)
	return ret[*dto.AdminPlanView](a, 0), a.Error(1)
}

func (m *SubscriptionService) CreateOrder(userID, idempotencyKey string, expectedPrice int) (*dto.SubscriptionOrderView, error) {
	a := m.Called(userID, idempotencyKey, expectedPrice)
	return ret[*dto.SubscriptionOrderView](a, 0), a.Error(1)
}

func (m *SubscriptionService) PayOrder(userID, orderID, result string) (*dto.SubscriptionView, error) {
	a := m.Called(userID, orderID, result)
	return ret[*dto.SubscriptionView](a, 0), a.Error(1)
}

func (m *SubscriptionService) Cancel(userID string) (*dto.SubscriptionView, error) {
	a := m.Called(userID)
	return ret[*dto.SubscriptionView](a, 0), a.Error(1)
}

func (m *SubscriptionService) History(userID string) ([]dto.SubscriptionView, error) {
	a := m.Called(userID)
	return ret[[]dto.SubscriptionView](a, 0), a.Error(1)
}

func (m *SubscriptionService) RenewDue() (int, error) {
	a := m.Called()
	return a.Int(0), a.Error(1)
}
