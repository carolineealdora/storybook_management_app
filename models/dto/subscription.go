package dto

import "time"

type SubscriptionOrderView struct {
	ID        string     `json:"id"`
	PlanID    string     `json:"plan_id"`
	OrderType string     `json:"order_type"`
	Price     int        `json:"price"`
	Status    string     `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	PaidAt    *time.Time `json:"paid_at"`
}

type PlanView struct {
	ID        string    `json:"id"`
	Price     int       `json:"price"`
	Benefits  string    `json:"benefits"`
	Duration  string    `json:"duration"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdatePlanRequest struct {
	Price    int    `json:"price" validate:"required,gt=0"`
	Benefits string `json:"benefits" validate:"required,max=2000"`
}

func (r UpdatePlanRequest) Validate() interface{} { return r }

type CreateSubscriptionOrderRequest struct {
	IdempotencyKey string `json:"idempotency_key" validate:"required,max=100"`
}

func (r CreateSubscriptionOrderRequest) Validate() interface{} { return r }
