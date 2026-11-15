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
