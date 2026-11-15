package book

import (
	"errors"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	bookrepo "codebase/internal/repositories/book"
	"codebase/internal/repositories/order"
	"codebase/internal/repositories/subscription"
	"codebase/models"
	"codebase/models/dto"

	"gorm.io/gorm"
)

type (
	Service interface {
		List(userID string) ([]dto.BookCard, error)
		Get(userID, bookID string) (*dto.BookDetail, error)
		Cover(bookID string) (*models.BookFile, error)
		Content(userID, bookID string) (*models.Book, error)
		MyBooks(userID string) ([]dto.MyBook, error)
	}

	ServiceImpl struct {
		books  bookrepo.Repository
		orders order.Repository
		subs   subscription.Repository
		errors helpers.Error
	}
)

func NewService(holder *models.SharedHolder) Service {
	return &ServiceImpl{
		books: bookrepo.NewRepository(holder), orders: order.NewRepository(holder), subs: subscription.NewRepository(holder),
		errors: helpers.NewError(holder),
	}
}

func (s *ServiceImpl) List(userID string) ([]dto.BookCard, error) {
	books, err := s.books.ListPublished()
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	owned, err := s.orders.PurchasedBookIDs(userID)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	subscribed, err := s.isSubscribed(userID)
	if err != nil {
		return nil, err
	}
	cards := make([]dto.BookCard, 0, len(books))
	for _, b := range books {
		cards = append(cards, toCard(b, accessOf(b, owned[b.ID], subscribed)))
	}
	return cards, nil
}

func (s *ServiceImpl) Get(userID, bookID string) (*dto.BookDetail, error) {
	b, err := s.books.GetByID(bookID)
	if err != nil {
		return nil, s.notFoundOr(err, bookID)
	}
	access, err := s.access(userID, b)
	if err != nil {
		return nil, err
	}
	return &dto.BookDetail{BookCard: toCard(*b, access), Description: b.Description}, nil
}

func (s *ServiceImpl) Cover(bookID string) (*models.BookFile, error) {
	f, err := s.books.GetCover(bookID)
	if err != nil {
		return nil, s.notFoundOr(err, bookID)
	}
	return f, nil
}

// Content returns the EPUB only after the access check passes; locked content never leaves the server (E18).
func (s *ServiceImpl) Content(userID, bookID string) (*models.Book, error) {
	b, err := s.books.GetWithContent(bookID)
	if err != nil {
		return nil, s.notFoundOr(err, bookID)
	}
	access, err := s.access(userID, b)
	if err != nil {
		return nil, err
	}
	if access == models.AccessLocked {
		return nil, s.errors.Wrap(respConst.BOOK_LOCKED, "book "+bookID)
	}
	return b, nil
}

func (s *ServiceImpl) MyBooks(userID string) ([]dto.MyBook, error) {
	rows, err := s.orders.ListMyBooks(userID)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	out := make([]dto.MyBook, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.MyBook{BookCard: toCard(r.Book, models.AccessOwned), PricePaid: r.PricePaid, PurchasedAt: r.PurchasedAt})
	}
	return out, nil
}

// access applies PRD §12.1: exists & published → free → purchased → active subscription → locked.
// A purchased book stays readable after it is unpublished or soft-deleted (E5).
func (s *ServiceImpl) access(userID string, b *models.Book) (string, error) {
	owned, err := s.orders.HasPurchase(userID, b.ID)
	if err != nil {
		return "", s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if !b.Visible() && !owned {
		return "", s.errors.Wrap(respConst.BOOK_NOT_FOUND, "book "+b.ID+" not visible")
	}
	subscribed, err := s.isSubscribed(userID)
	if err != nil {
		return "", err
	}
	return accessOf(*b, owned, subscribed), nil
}

func (s *ServiceImpl) isSubscribed(userID string) (bool, error) {
	_, err := s.subs.GetActive(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return true, nil
}

func (s *ServiceImpl) notFoundOr(err error, bookID string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.errors.Wrap(respConst.BOOK_NOT_FOUND, "book "+bookID)
	}
	return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
}

func accessOf(b models.Book, owned, subscribed bool) string {
	switch {
	case b.Price == 0:
		return models.AccessFree
	case owned:
		return models.AccessOwned
	case subscribed:
		return models.AccessSubscribed
	default:
		return models.AccessLocked
	}
}

func toCard(b models.Book, access string) dto.BookCard {
	return dto.BookCard{
		ID: b.ID, Title: b.Title, Author: b.Author, AgeType: b.AgeType, AgeMin: b.AgeMin, AgeMax: b.AgeMax,
		AgeLabel: b.AgeLabel(), Price: b.Price, IsFree: b.Price == 0, Access: access, CoverURL: b.CoverURL(),
	}
}
