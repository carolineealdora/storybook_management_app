package subscription

import (
	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
)

type (
	Repository interface {
		GetActive(userID string) (*models.Subscription, error)
		GetPlan() (*models.SubscriptionPlan, error)
		SavePlan(p *models.SubscriptionPlan) error
	}

	Repo struct {
		db sql.ORM
	}
)

func NewRepository(holder *models.SharedHolder) Repository {
	return &Repo{db: holder.Sql}
}

// GetActive returns the user's active subscription or gorm.ErrRecordNotFound.
// Active means status = 'active'; renewal is mocked to always succeed, so expires_at is not compared (PLAN D6).
func (r *Repo) GetActive(userID string) (*models.Subscription, error) {
	var s models.Subscription
	if err := r.db.Where("user_id = ? AND status = ?", userID, models.SubActive).First(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetPlan returns the configured plan (single row) or gorm.ErrRecordNotFound.
func (r *Repo) GetPlan() (*models.SubscriptionPlan, error) {
	var p models.SubscriptionPlan
	if err := r.db.Order("updated_at DESC").First(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SavePlan inserts the plan when it has no ID yet, otherwise updates price and benefits in place.
func (r *Repo) SavePlan(p *models.SubscriptionPlan) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
		return r.db.Create(p)
	}
	return r.db.Exec(`UPDATE subscription_plans SET price = ?, benefits = ?, updated_at = ? WHERE id = ?`, p.Price, p.Benefits, p.UpdatedAt, p.ID)
}
