package dto

import "time"

type CreateBookOrderRequest struct {
	BookID         string `json:"book_id" validate:"required,uuid"`
	IdempotencyKey string `json:"idempotency_key" validate:"required,max=100"`
}

func (r CreateBookOrderRequest) Validate() interface{} { return r }

// PayRequest drives the mocked payment (PRD: only success or failed).
type PayRequest struct {
	Result string `json:"result" validate:"required,oneof=success failed"`
}

func (r PayRequest) Validate() interface{} { return r }

type BookOrderView struct {
	ID        string     `json:"id"`
	BookID    string     `json:"book_id"`
	BookTitle string     `json:"book_title"`
	Price     int        `json:"price"` // price shown at checkout = price paid
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at"`
}
