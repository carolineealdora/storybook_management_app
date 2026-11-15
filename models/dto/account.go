package dto

import "time"

type Profile struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Email        string            `json:"email"`
	PinSet       bool              `json:"pin_set"`
	IsSubscriber bool              `json:"is_subscriber"`
	Subscription *SubscriptionView `json:"subscription"` // null when not subscribed
}

type SubscriptionView struct {
	ID            string                 `json:"id"`
	Status        string                 `json:"status"`
	PricePaid     int                    `json:"price_paid"`
	StartedAt     time.Time              `json:"started_at"`
	ExpiresAt     time.Time              `json:"expires_at"` // next renewal date while active
	CancelledAt   *time.Time             `json:"cancelled_at"`
	RenewedFromID *string                `json:"renewed_from_id"`
	Order         *SubscriptionOrderView `json:"order,omitempty"`
}

type UpdateNameRequest struct {
	Name string `json:"name" validate:"required,max=200"`
}

func (r UpdateNameRequest) Validate() interface{} { return r }

type SetPinRequest struct {
	Pin        string `json:"pin" validate:"required,len=4,numeric"`
	ConfirmPin string `json:"confirm_pin" validate:"required,eqfield=Pin"`
	Password   string `json:"password,omitempty" validate:"max=72"` // required when a PIN already exists
}

func (r SetPinRequest) Validate() interface{} { return r }

type VerifyPinRequest struct {
	Pin string `json:"pin" validate:"required,max=10"`
}

func (r VerifyPinRequest) Validate() interface{} { return r }

type PasswordRequest struct {
	Password string `json:"password" validate:"required,max=72"`
}

func (r PasswordRequest) Validate() interface{} { return r }
