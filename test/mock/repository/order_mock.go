package repository_mock

import (
	"time"

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

func (m *OrderRepository) CreateBookOrder(o *models.BookOrder) error {
	return m.Called(o).Error(0)
}

func (m *OrderRepository) GetBookOrder(id, userID string) (*models.BookOrder, error) {
	a := m.Called(id, userID)
	return ret[*models.BookOrder](a, 0), a.Error(1)
}

func (m *OrderRepository) MarkBookOrderPaid(o *models.BookOrder, at time.Time) error {
	return m.Called(o, at).Error(0)
}

func (m *OrderRepository) MarkBookOrderFailed(id string, at time.Time) error {
	return m.Called(id, at).Error(0)
}

func (m *OrderRepository) ListBookOrders(userID string) ([]models.BookOrderRow, error) {
	a := m.Called(userID)
	return ret[[]models.BookOrderRow](a, 0), a.Error(1)
}
