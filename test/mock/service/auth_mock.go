package services_mocks

import (
	"codebase/models/dto"

	"github.com/stretchr/testify/mock"
)

type AuthService struct{ mock.Mock }

func (m *AuthService) Login(email, password, accountType string) (*dto.TokenResponse, error) {
	a := m.Called(email, password, accountType)
	return ret[*dto.TokenResponse](a, 0), a.Error(1)
}

func (m *AuthService) Refresh(refreshToken string) (*dto.TokenResponse, error) {
	a := m.Called(refreshToken)
	return ret[*dto.TokenResponse](a, 0), a.Error(1)
}

func (m *AuthService) Logout(credentialID, refreshToken string) error {
	return m.Called(credentialID, refreshToken).Error(0)
}
