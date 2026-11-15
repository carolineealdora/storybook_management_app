package services_mocks

import (
	"codebase/models/dto"

	"github.com/stretchr/testify/mock"
)

type OrderService struct{ mock.Mock }

func (m *OrderService) Create(userID, bookID, idempotencyKey string) (*dto.BookOrderView, error) {
	a := m.Called(userID, bookID, idempotencyKey)
	return ret[*dto.BookOrderView](a, 0), a.Error(1)
}

func (m *OrderService) Pay(userID, orderID, result string) (*dto.BookOrderView, error) {
	a := m.Called(userID, orderID, result)
	return ret[*dto.BookOrderView](a, 0), a.Error(1)
}

func (m *OrderService) List(userID string) ([]dto.BookOrderView, error) {
	a := m.Called(userID)
	return ret[[]dto.BookOrderView](a, 0), a.Error(1)
}
