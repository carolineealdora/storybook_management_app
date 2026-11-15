package services_mocks

import (
	"codebase/models/dto"

	"github.com/stretchr/testify/mock"
)

type AccountService struct{ mock.Mock }

func (m *AccountService) GetProfile(userID string) (*dto.Profile, error) {
	a := m.Called(userID)
	return ret[*dto.Profile](a, 0), a.Error(1)
}

func (m *AccountService) UpdateName(userID, name string) (*dto.Profile, error) {
	a := m.Called(userID, name)
	return ret[*dto.Profile](a, 0), a.Error(1)
}

func (m *AccountService) SetPin(userID string, req dto.SetPinRequest) error {
	return m.Called(userID, req).Error(0)
}

func (m *AccountService) VerifyPin(userID, pin string) error {
	return m.Called(userID, pin).Error(0)
}

func (m *AccountService) ForgotPin(userID, password string) error {
	return m.Called(userID, password).Error(0)
}
