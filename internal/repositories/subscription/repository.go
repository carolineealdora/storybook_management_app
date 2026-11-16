package subscription

import (
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

// ListPriceHistory returns the plan's price changes, newest first.
func (r *Repo) ListPriceHistory(planID string) ([]models.PlanPriceChange, error) {
	var rows []models.PlanPriceChange
	err := r.db.RawSql(`SELECT id, plan_id, old_price, new_price, changed_by, changed_at FROM subscription_plan_price_history
		WHERE plan_id = ? ORDER BY changed_at DESC`, &rows, planID)
	return rows, err
}
