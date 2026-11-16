package services_mocks

import (
	"codebase/models/dto"

	"github.com/stretchr/testify/mock"
)

type SubscriptionService struct{ mock.Mock }

func (m *SubscriptionService) GetAdminPlan() (*dto.AdminPlanView, error) {
	a := m.Called()
	return ret[*dto.AdminPlanView](a, 0), a.Error(1)
}

func (m *SubscriptionService) UpdatePlan(adminID string, price int, benefits string) (*dto.AdminPlanView, error) {
	a := m.Called(adminID, price, benefits)
	return ret[*dto.AdminPlanView](a, 0), a.Error(1)
}
