package services_mocks

import (
	"codebase/models"
	"codebase/models/dto"

	"github.com/stretchr/testify/mock"
)

type BookService struct{ mock.Mock }

func (m *BookService) List(userID string) ([]dto.BookCard, error) {
	a := m.Called(userID)
	return ret[[]dto.BookCard](a, 0), a.Error(1)
}

func (m *BookService) Get(userID, bookID string) (*dto.BookDetail, error) {
	a := m.Called(userID, bookID)
	return ret[*dto.BookDetail](a, 0), a.Error(1)
}

func (m *BookService) Cover(bookID string) (*models.BookFile, error) {
	a := m.Called(bookID)
	return ret[*models.BookFile](a, 0), a.Error(1)
}

func (m *BookService) Content(userID, bookID string) (*models.Book, error) {
	a := m.Called(userID, bookID)
	return ret[*models.Book](a, 0), a.Error(1)
}

func (m *BookService) MyBooks(userID string) ([]dto.MyBook, error) {
	a := m.Called(userID)
	return ret[[]dto.MyBook](a, 0), a.Error(1)
}
