package order

// SECURITY: manual review required — single-book checkout and mocked payment.

import (
	"errors"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	bookrepo "codebase/internal/repositories/book"
	"codebase/internal/repositories/order"
	"codebase/internal/repositories/subscription"
	"codebase/models"
	"codebase/models/dto"
	sql "codebase/pkg/orm"
	"codebase/pkg/shared"

	"gorm.io/gorm"
)

const PaymentSuccess = "success"

type (
	Service interface {
		Create(userID, bookID, idempotencyKey string) (*dto.BookOrderView, error)
		Pay(userID, orderID, result string) (*dto.BookOrderView, error)
		List(userID string) ([]dto.BookOrderView, error)
	}

	ServiceImpl struct {
		books  bookrepo.Repository
		orders order.Repository
		subs   subscription.Repository
		errors helpers.Error
		now    shared.Now
	}
)

func NewService(holder *models.SharedHolder) Service {
	return &ServiceImpl{
		books: bookrepo.NewRepository(holder), orders: order.NewRepository(holder), subs: subscription.NewRepository(holder),
		errors: helpers.NewError(holder), now: time.Now,
	}
}

// Create is Buy Now: one book per order, price snapshotted from the book at checkout (US-09).
func (s *ServiceImpl) Create(userID, bookID, idempotencyKey string) (*dto.BookOrderView, error) {
	b, err := s.books.GetByID(bookID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !b.Visible()) {
		return nil, s.errors.Wrap(respConst.BOOK_NOT_FOUND, "book "+bookID)
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if b.Price == 0 {
		return nil, s.errors.Wrap(respConst.BOOK_NOT_PURCHASABLE, "free book "+bookID)
	}
	if err := s.checkPurchasable(userID, bookID); err != nil {
		return nil, err
	}

	now := s.now()
	o := &models.BookOrder{UserID: userID, BookID: bookID, Price: b.Price, Status: models.OrderPending, IdempotencyKey: idempotencyKey, CreatedAt: now, UpdatedAt: now}
	if err := s.orders.CreateBookOrder(o); errors.Is(err, sql.ErrDuplicate) {
		return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "idempotency key reused")
	} else if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return toView(*o, b.Title), nil
}

// Pay runs the mocked payment. It charges the order's snapshot price, never the current book price (E7).
func (s *ServiceImpl) Pay(userID, orderID, result string) (*dto.BookOrderView, error) {
	o, err := s.orders.GetBookOrder(orderID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.ORDER_NOT_FOUND, "order "+orderID)
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if o.Status != models.OrderPending {
		return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "order already "+o.Status)
	}
	b, err := s.books.GetByID(o.BookID)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !b.Visible()) {
		return nil, s.errors.Wrap(respConst.BOOK_UNAVAILABLE, "book "+o.BookID+" removed during checkout")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if err := s.checkPurchasable(userID, o.BookID); err != nil {
		return nil, err
	}

	now := s.now()
	if result != PaymentSuccess {
		if err := s.orders.MarkBookOrderFailed(o.ID, now); errors.Is(err, sql.ErrStateChanged) {
			return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "order changed concurrently")
		} else if err != nil {
			return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
		}
		return nil, s.errors.Wrap(respConst.PAYMENT_FAILED, "mock payment failed for order "+o.ID)
	}

	switch err := s.orders.MarkBookOrderPaid(o, now); {
	case errors.Is(err, sql.ErrStateChanged):
		return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "order changed concurrently")
	case errors.Is(err, sql.ErrDuplicate):
		return nil, s.errors.Wrap(respConst.BOOK_ALREADY_OWNED, "purchased by another order")
	case err != nil:
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	o.Status, o.PaidAt = models.OrderPaid, &now
	return toView(*o, b.Title), nil
}

func (s *ServiceImpl) List(userID string) ([]dto.BookOrderView, error) {
	rows, err := s.orders.ListBookOrders(userID)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	out := make([]dto.BookOrderView, 0, len(rows))
	for _, r := range rows {
		out = append(out, *toView(r.BookOrder, r.BookTitle))
	}
	return out, nil
}

// checkPurchasable rejects owned books (E1) and subscribed users (E2).
func (s *ServiceImpl) checkPurchasable(userID, bookID string) error {
	owned, err := s.orders.HasPurchase(userID, bookID)
	if err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if owned {
		return s.errors.Wrap(respConst.BOOK_ALREADY_OWNED, "book "+bookID)
	}
	_, err = s.subs.GetActive(userID)
	if err == nil {
		return s.errors.Wrap(respConst.SUBSCRIPTION_ACTIVE, "subscribed users cannot buy single books")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return nil
}

func toView(o models.BookOrder, title string) *dto.BookOrderView {
	return &dto.BookOrderView{ID: o.ID, BookID: o.BookID, BookTitle: title, Price: o.Price, Status: o.Status, CreatedAt: o.CreatedAt, PaidAt: o.PaidAt}
}
