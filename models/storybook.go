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

// AgeLabel renders the age range as shown on covers: "1–3" for a range, "3+" for a number (PRD US-16).
func (b Book) AgeLabel() string {
	if b.AgeType == AgeRange && b.AgeMax != nil {
		return fmt.Sprintf("%d–%d", b.AgeMin, *b.AgeMax)
	}
	return fmt.Sprintf("%d+", b.AgeMin)
}

// CoverURL is the API path serving the book's cover image.
func (b Book) CoverURL() string { return "/api/v1/books/" + b.ID + "/cover" }
