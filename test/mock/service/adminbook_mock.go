package services_mocks

import (
	"codebase/models/dto"

	"github.com/stretchr/testify/mock"
)

type AdminBookService struct{ mock.Mock }

func (m *AdminBookService) List(search, status string) ([]dto.AdminBook, error) {
	a := m.Called(search, status)
	return ret[[]dto.AdminBook](a, 0), a.Error(1)
}

func (m *AdminBookService) Get(id string) (*dto.AdminBook, error) {
	a := m.Called(id)
	return ret[*dto.AdminBook](a, 0), a.Error(1)
}

func (m *AdminBookService) Create(in dto.BookInput) (*dto.AdminBook, error) {
	a := m.Called(in)
	return ret[*dto.AdminBook](a, 0), a.Error(1)
}

func (m *AdminBookService) Update(id string, in dto.BookInput) (*dto.AdminBook, error) {
	a := m.Called(id, in)
	return ret[*dto.AdminBook](a, 0), a.Error(1)
}

func (m *AdminBookService) Delete(id string) error {
	return m.Called(id).Error(0)
}
