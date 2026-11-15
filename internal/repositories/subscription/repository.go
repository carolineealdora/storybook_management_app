package subscription

import (
	"codebase/models"
	sql "codebase/pkg/orm"
)

type (
	Repository interface {
		GetActive(userID string) (*models.Subscription, error)
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
