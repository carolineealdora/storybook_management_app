package models

import (
	"fmt"
	"time"
)

const (
	AccountParent = "parent"
	AccountAdmin  = "admin"

	AgeRange = "range"
	AgePlus  = "plus"

	BookDraft     = "draft"
	BookPublished = "published"

	OrderPending = "pending"
	OrderPaid    = "paid"
	OrderFailed  = "failed"

	OrderTypeNew     = "new"
	OrderTypeRenewal = "renewal"

	SubActive    = "active"
	SubCancelled = "cancelled"
	SubExpired   = "expired"

	AccessFree       = "free"
	AccessOwned      = "owned"
	AccessSubscribed = "subscribed"
	AccessLocked     = "locked"

	// Account badge (CR-001-1)
	PlanStatusSubscribed = "subscribed"
	PlanStatusFree       = "free"

	PlanCodeMonthly = "monthly"
	PlanNameMonthly = "Monthly Subscription"
)

type Credential struct {
	ID           string     `gorm:"column:id;primaryKey"`
	Email        string     `gorm:"column:email"`
	PasswordHash string     `gorm:"column:password_hash"`
	AccountType  string     `gorm:"column:account_type"`
	UserID       *string    `gorm:"column:user_id"`
	AdminID      *string    `gorm:"column:admin_id"`
	LastLoginAt  *time.Time `gorm:"column:last_login_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
}

func (Credential) TableName() string { return "credentials" }

type User struct {
	ID                string     `gorm:"column:id;primaryKey"`
	Name              string     `gorm:"column:name"`
	PinHash           *string    `gorm:"column:pin_hash"`
	PinFailedAttempts int        `gorm:"column:pin_failed_attempts"`
	PinLockedUntil    *time.Time `gorm:"column:pin_locked_until"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (User) TableName() string { return "users" }

type RefreshToken struct {
	ID           string     `gorm:"column:id;primaryKey"`
	CredentialID string     `gorm:"column:credential_id"`
	TokenHash    string     `gorm:"column:token_hash"`
	ExpiresAt    time.Time  `gorm:"column:expires_at"`
	RevokedAt    *time.Time `gorm:"column:revoked_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }

type Book struct {
	ID            string     `gorm:"column:id;primaryKey"`
	Title         string     `gorm:"column:title"`
	Author        string     `gorm:"column:author"`
	Description   string     `gorm:"column:description"`
	AgeType       string     `gorm:"column:age_type"`
	AgeMin        int        `gorm:"column:age_min"`
	AgeMax        *int       `gorm:"column:age_max"`
	Price         int        `gorm:"column:price"`
	Status        string     `gorm:"column:status"`
	EpubFileName  string     `gorm:"column:epub_file_name"`
	EpubSizeBytes int        `gorm:"column:epub_size_bytes"`
	EpubContent   []byte     `gorm:"column:epub_content"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
	DeletedAt     *time.Time `gorm:"column:deleted_at"`
}

func (Book) TableName() string { return "books" }

// Visible reports whether the book may appear in the library app (PRD US-04, US-16).
func (b Book) Visible() bool { return b.Status == BookPublished && b.DeletedAt == nil }

type BookFile struct {
	ID        string    `gorm:"column:id;primaryKey"`
	BookID    string    `gorm:"column:book_id"`
	MimeType  string    `gorm:"column:mime_type"`
	FileName  string    `gorm:"column:file_name"`
	SizeBytes int       `gorm:"column:size_bytes"`
	Content   []byte    `gorm:"column:content"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (BookFile) TableName() string { return "book_files" }

type SubscriptionPlan struct {
	ID        string    `gorm:"column:id;primaryKey"`
	Code      string    `gorm:"column:code"`
	Name      string    `gorm:"column:name"`
	Price     int       `gorm:"column:price"`
	Benefits  string    `gorm:"column:benefits"`
	Duration  string    `gorm:"column:duration"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
	UpdatedBy *string   `gorm:"column:updated_by"` // admin id; nil for the seeded plan
}

func (SubscriptionPlan) TableName() string { return "subscription_plans" }

// PlanPriceChange is one admin price edit (CR-001-2 audit trail).
type PlanPriceChange struct {
	ID        string    `gorm:"column:id;primaryKey"`
	PlanID    string    `gorm:"column:plan_id"`
	OldPrice  int       `gorm:"column:old_price"`
	NewPrice  int       `gorm:"column:new_price"`
	ChangedBy string    `gorm:"column:changed_by"`
	ChangedAt time.Time `gorm:"column:changed_at"`
}

func (PlanPriceChange) TableName() string { return "subscription_plan_price_history" }

type Subscription struct {
	ID            string     `gorm:"column:id;primaryKey"`
	UserID        string     `gorm:"column:user_id"`
	PlanID        string     `gorm:"column:plan_id"`
	RenewedFromID *string    `gorm:"column:renewed_from_id"`
	PricePaid     int        `gorm:"column:price_paid"`
	Status        string     `gorm:"column:status"`
	StartedAt     time.Time  `gorm:"column:started_at"`
	ExpiresAt     time.Time  `gorm:"column:expires_at"`
	CancelledAt   *time.Time `gorm:"column:cancelled_at"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (Subscription) TableName() string { return "subscriptions" }

type BookOrder struct {
	ID             string     `gorm:"column:id;primaryKey"`
	UserID         string     `gorm:"column:user_id"`
	BookID         string     `gorm:"column:book_id"`
	Price          int        `gorm:"column:price"`
	Status         string     `gorm:"column:status"`
	IdempotencyKey string     `gorm:"column:idempotency_key"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	PaidAt         *time.Time `gorm:"column:paid_at"`
}

func (BookOrder) TableName() string { return "book_orders" }

type SubscriptionOrder struct {
	ID             string     `gorm:"column:id;primaryKey"`
	UserID         string     `gorm:"column:user_id"`
	PlanID         string     `gorm:"column:plan_id"`
	SubscriptionID *string    `gorm:"column:subscription_id"`
	OrderType      string     `gorm:"column:order_type"`
	Price          int        `gorm:"column:price"`
	Status         string     `gorm:"column:status"`
	IdempotencyKey string     `gorm:"column:idempotency_key"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	PaidAt         *time.Time `gorm:"column:paid_at"`
}

func (SubscriptionOrder) TableName() string { return "subscription_orders" }

type Purchase struct {
	ID          string    `gorm:"column:id;primaryKey"`
	UserID      string    `gorm:"column:user_id"`
	BookID      string    `gorm:"column:book_id"`
	BookOrderID string    `gorm:"column:book_order_id"`
	PricePaid   int       `gorm:"column:price_paid"`
	PurchasedAt time.Time `gorm:"column:purchased_at"`
}

func (Purchase) TableName() string { return "purchases" }

// AgeLabel renders the age range as shown on covers: "1–3" for a range, "3+" for a number (PRD US-16).
func (b Book) AgeLabel() string {
	if b.AgeType == AgeRange && b.AgeMax != nil {
		return fmt.Sprintf("%d–%d", b.AgeMin, *b.AgeMax)
	}
	return fmt.Sprintf("%d+", b.AgeMin)
}

// CoverURL is the API path serving the book's cover image.
func (b Book) CoverURL() string { return "/api/v1/books/" + b.ID + "/cover" }

// OwnedBook is a purchased book joined with its purchase (My Book).
type OwnedBook struct {
	Book
	PricePaid   int       `gorm:"column:price_paid"`
	PurchasedAt time.Time `gorm:"column:purchased_at"`
}

// BookOrderRow is a book order joined with the book title (Order History).
type BookOrderRow struct {
	BookOrder
	BookTitle string `gorm:"column:book_title"`
}

// SubscriptionHistoryRow is a subscription period with the order that paid for it (Subscription History).
type SubscriptionHistoryRow struct {
	Subscription
	OrderID        *string    `gorm:"column:order_id"`
	OrderPlanID    *string    `gorm:"column:order_plan_id"`
	OrderType      *string    `gorm:"column:order_type"`
	OrderPrice     *int       `gorm:"column:order_price"`
	OrderStatus    *string    `gorm:"column:order_status"`
	OrderCreatedAt *time.Time `gorm:"column:order_created_at"`
	OrderPaidAt    *time.Time `gorm:"column:order_paid_at"`
}
