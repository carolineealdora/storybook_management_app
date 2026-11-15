package repository_mock

import (
	"time"

	"codebase/models"

	"github.com/stretchr/testify/mock"
)

type AccountRepository struct{ mock.Mock }

func (m *AccountRepository) GetCredentialByEmail(email string) (*models.Credential, error) {
	a := m.Called(email)
	return ret[*models.Credential](a, 0), a.Error(1)
}

func (m *AccountRepository) GetCredentialByID(id string) (*models.Credential, error) {
	a := m.Called(id)
	return ret[*models.Credential](a, 0), a.Error(1)
}

func (m *AccountRepository) TouchLastLogin(credentialID string, at time.Time) error {
	return m.Called(credentialID, at).Error(0)
}

func (m *AccountRepository) CreateRefreshToken(t *models.RefreshToken) error {
	return m.Called(t).Error(0)
}

func (m *AccountRepository) GetRefreshToken(tokenHash string) (*models.RefreshToken, error) {
	a := m.Called(tokenHash)
	return ret[*models.RefreshToken](a, 0), a.Error(1)
}

func (m *AccountRepository) RevokeRefreshToken(tokenHash, credentialID string, at time.Time) error {
	return m.Called(tokenHash, credentialID, at).Error(0)
}

func (m *AccountRepository) GetUser(id string) (*models.User, error) {
	a := m.Called(id)
	return ret[*models.User](a, 0), a.Error(1)
}

func (m *AccountRepository) GetCredentialByUserID(userID string) (*models.Credential, error) {
	a := m.Called(userID)
	return ret[*models.Credential](a, 0), a.Error(1)
}

func (m *AccountRepository) UpdateUserName(id, name string, at time.Time) error {
	return m.Called(id, name, at).Error(0)
}

func (m *AccountRepository) SetPin(userID, pinHash string, at time.Time) error {
	return m.Called(userID, pinHash, at).Error(0)
}

func (m *AccountRepository) IncrementPinFailures(userID string) (int, error) {
	a := m.Called(userID)
	return a.Int(0), a.Error(1)
}

func (m *AccountRepository) LockPin(userID string, until time.Time) error {
	return m.Called(userID, until).Error(0)
}

func (m *AccountRepository) ResetPinFailures(userID string) error {
	return m.Called(userID).Error(0)
}
