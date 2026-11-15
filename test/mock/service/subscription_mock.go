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

func (m *SubscriptionService) UpdatePlan(price int, benefits string) (*dto.PlanView, error) {
	a := m.Called(price, benefits)
	return ret[*dto.PlanView](a, 0), a.Error(1)
}
