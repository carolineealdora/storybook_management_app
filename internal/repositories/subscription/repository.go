package subscription

import (
	"time"

	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type (
	Repository interface {
		GetPlan() (*models.SubscriptionPlan, error)
		SavePlan(p *models.SubscriptionPlan) error
		ListPriceHistory(planID string) ([]models.PlanPriceChange, error)
		CountActiveSubscribers(now time.Time) (int, error)
	}

	Repo struct {
		db sql.ORM
	}
)

func NewRepository(holder *models.SharedHolder) Repository {
	return &Repo{db: holder.Sql}
}

// GetPlan returns the configured plan (single row) or gorm.ErrRecordNotFound.
func (r *Repo) GetPlan() (*models.SubscriptionPlan, error) {
	var p models.SubscriptionPlan
	if err := r.db.Order("updated_at DESC").First(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SavePlan inserts the plan when it has no ID yet, otherwise updates it in place.
// A price change also writes a subscription_plan_price_history row in the same transaction,
// with the old price read under a row lock so concurrent edits each record the true previous price.
func (r *Repo) SavePlan(p *models.SubscriptionPlan) error {
	if p.ID == "" {
		p.ID = uuid.NewString()
		return r.db.Create(p)
	}
	return sql.WithTx(r.db, func(tx sql.ORM) error {
		var old []int
		if err := tx.RawSql(`SELECT price FROM subscription_plans WHERE id = ? FOR UPDATE`, &old, p.ID); err != nil {
			return err
		}
		if len(old) == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Exec(`UPDATE subscription_plans SET price = ?, benefits = ?, updated_at = ?, updated_by = ? WHERE id = ?`,
			p.Price, p.Benefits, p.UpdatedAt, p.UpdatedBy, p.ID); err != nil {
			return err
		}
		if old[0] == p.Price {
			return nil // E-CR6: no history row for the same price
		}
		return tx.Exec(`INSERT INTO subscription_plan_price_history (id, plan_id, old_price, new_price, changed_by, changed_at) VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), p.ID, old[0], p.Price, p.UpdatedBy, p.UpdatedAt)
	})
}

// ListPriceHistory returns the plan's price changes, newest first, with the editing admin's name.
func (r *Repo) ListPriceHistory(planID string) ([]models.PlanPriceChange, error) {
	var rows []models.PlanPriceChange
	err := r.db.RawSql(`SELECT h.id, h.plan_id, h.old_price, h.new_price, h.changed_by, h.changed_at, a.name AS changed_by_name
		FROM subscription_plan_price_history h LEFT JOIN admins a ON a.id = h.changed_by
		WHERE h.plan_id = ? ORDER BY h.changed_at DESC`, &rows, planID)
	return rows, err
}

// CountActiveSubscribers counts periods that are active and not yet expired: the same rule as the
// Subscribed badge (CR-001-1 B1). uq_subscriptions_one_active makes it one per user.
func (r *Repo) CountActiveSubscribers(now time.Time) (int, error) {
	var n []int
	if err := r.db.RawSql(`SELECT COUNT(*) FROM subscriptions WHERE status = 'active' AND expires_at > ?`, &n, now); err != nil {
		return 0, err
	}
	return n[0], nil
}
