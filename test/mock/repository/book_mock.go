package repository_mock

import (
	"time"

	"codebase/models"

	"github.com/stretchr/testify/mock"
)

type BookRepository struct{ mock.Mock }

func (m *BookRepository) GetByID(id string) (*models.Book, error) {
	a := m.Called(id)
	return ret[*models.Book](a, 0), a.Error(1)
}

func (m *BookRepository) AdminList(search, status string) ([]models.Book, error) {
	a := m.Called(search, status)
	return ret[[]models.Book](a, 0), a.Error(1)
}

func (m *BookRepository) Create(book *models.Book, cover *models.BookFile) error {
	return m.Called(book, cover).Error(0)
}

func (m *BookRepository) Update(book *models.Book, cover *models.BookFile) error {
	return m.Called(book, cover).Error(0)
}

func (m *BookRepository) SoftDelete(id string, at time.Time) error {
	return m.Called(id, at).Error(0)
}

func (m *BookRepository) GetCover(bookID string) (*models.BookFile, error) {
	a := m.Called(bookID)
	return ret[*models.BookFile](a, 0), a.Error(1)
}
