package dto

import "time"

type PlanView struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Price     int       `json:"price"`
	Benefits  string    `json:"benefits"`
	Duration  string    `json:"duration"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminPlanView is the back-office plan page: the plan, who last edited it, and its price history (CR-001-2).
type AdminPlanView struct {
	PlanView
	UpdatedBy *string `json:"updated_by"`
	// ActiveSubscriberCount is active, unexpired periods: one per user (design E4).
	ActiveSubscriberCount int                   `json:"active_subscriber_count"`
	PriceHistory          []PlanPriceChangeView `json:"price_history"` // newest first
}

type PlanPriceChangeView struct {
	ID            string    `json:"id"`
	OldPrice      int       `json:"old_price"`
	NewPrice      int       `json:"new_price"`
	ChangedBy     string    `json:"changed_by"`
	ChangedByName *string   `json:"changed_by_name"` // null only if the admin row is gone
	ChangedAt     time.Time `json:"changed_at"`
}

type UpdatePlanRequest struct {
	Price    int    `json:"price"` // ≤ 0 is rejected by the service as INVALID_PRICE (E-CR2)
	Benefits string `json:"benefits" validate:"required,max=2000"`
}

func (r UpdatePlanRequest) Validate() interface{} { return r }
