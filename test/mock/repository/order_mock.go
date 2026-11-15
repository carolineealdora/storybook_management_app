package repository_mock

import (
	"codebase/models"

	"github.com/stretchr/testify/mock"
)

type OrderRepository struct{ mock.Mock }

func (m *OrderRepository) PurchasedBookIDs(userID string) (map[string]bool, error) {
	a := m.Called(userID)
	return ret[map[string]bool](a, 0), a.Error(1)
}

func (m *OrderRepository) HasPurchase(userID, bookID string) (bool, error) {
	a := m.Called(userID, bookID)
	return a.Bool(0), a.Error(1)
}

func (m *OrderRepository) ListMyBooks(userID string) ([]models.OwnedBook, error) {
	a := m.Called(userID)
	return ret[[]models.OwnedBook](a, 0), a.Error(1)
}
