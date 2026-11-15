package subscription

import (
	"time"

	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
)

type (
	Repository interface {
		GetActive(userID string) (*models.Subscription, error)
		GetPlan() (*models.SubscriptionPlan, error)
		SavePlan(p *models.SubscriptionPlan) error

		CreateOrder(o *models.SubscriptionOrder) error
		GetOrder(id, userID string) (*models.SubscriptionOrder, error)
		MarkOrderFailed(id string, at time.Time) error
		Activate(o *models.SubscriptionOrder, sub *models.Subscription, at time.Time) error
		Cancel(subID string, at time.Time) error
		ListHistory(userID string) ([]models.SubscriptionHistoryRow, error)
		ListDue(now time.Time, limit int) ([]models.Subscription, error)
		Renew(old, next *models.Subscription, order *models.SubscriptionOrder) error
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

// CreateOrder returns sql.ErrDuplicate when the user already used this idempotency key.
func (r *Repo) CreateOrder(o *models.SubscriptionOrder) error {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	if err := r.db.Create(o); err != nil {
		if sql.IsUniqueViolation(err) {
			return sql.ErrDuplicate
		}
		return err
	}
	return nil
}

func (r *Repo) GetOrder(id, userID string) (*models.SubscriptionOrder, error) {
	var o models.SubscriptionOrder
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&o); err != nil {
		return nil, err
	}
	return &o, nil
}

func (r *Repo) MarkOrderFailed(id string, at time.Time) error {
	return conditionalUpdate(r.db, `UPDATE subscription_orders SET status = 'failed', updated_at = ? WHERE id = ? AND status = 'pending' RETURNING id`, at, id)
}

// Activate inserts the active period and marks its order paid, atomically.
// sql.ErrDuplicate: the user already has an active subscription. sql.ErrStateChanged: the order was not pending.
func (r *Repo) Activate(o *models.SubscriptionOrder, sub *models.Subscription, at time.Time) error {
	if sub.ID == "" {
		sub.ID = uuid.NewString()
	}
	return sql.WithTx(r.db, func(tx sql.ORM) error {
		if err := tx.Create(sub); err != nil {
			if sql.IsUniqueViolation(err) {
				return sql.ErrDuplicate
			}
			return err
		}
		return conditionalUpdate(tx, `UPDATE subscription_orders SET status = 'paid', paid_at = ?, subscription_id = ?, updated_at = ?
			WHERE id = ? AND status = 'pending' RETURNING id`, at, sub.ID, at, o.ID)
	})
}

// Cancel stops access and auto-renewal immediately (US-11). sql.ErrStateChanged if it was no longer active.
func (r *Repo) Cancel(subID string, at time.Time) error {
	return conditionalUpdate(r.db, `UPDATE subscriptions SET status = 'cancelled', cancelled_at = ?, updated_at = ? WHERE id = ? AND status = 'active' RETURNING id`, at, at, subID)
}

// ListHistory returns every period, newest first, with the (new or renewal) order that paid for it.
func (r *Repo) ListHistory(userID string) ([]models.SubscriptionHistoryRow, error) {
	var rows []models.SubscriptionHistoryRow
	err := r.db.RawSql(`SELECT s.*, o.id AS order_id, o.plan_id AS order_plan_id, o.order_type, o.price AS order_price,
			o.status AS order_status, o.created_at AS order_created_at, o.paid_at AS order_paid_at
		FROM subscriptions s LEFT JOIN subscription_orders o ON o.subscription_id = s.id
		WHERE s.user_id = ? ORDER BY s.started_at DESC, s.created_at DESC`, &rows, userID)
	return rows, err
}

// ListDue returns active subscriptions whose period has ended, oldest first.
func (r *Repo) ListDue(now time.Time, limit int) ([]models.Subscription, error) {
	var subs []models.Subscription
	if _, err := r.db.Where("status = ? AND expires_at <= ?", models.SubActive, now).Order("expires_at").Limit(limit).All(&subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// Renew expires the old period and starts the next one with its paid renewal order, atomically.
// sql.ErrStateChanged: the old period was cancelled or already renewed meanwhile.
func (r *Repo) Renew(old, next *models.Subscription, order *models.SubscriptionOrder) error {
	if next.ID == "" {
		next.ID = uuid.NewString()
	}
	if order.ID == "" {
		order.ID = uuid.NewString()
	}
	order.SubscriptionID = &next.ID
	return sql.WithTx(r.db, func(tx sql.ORM) error {
		if err := conditionalUpdate(tx, `UPDATE subscriptions SET status = 'expired', updated_at = NOW() WHERE id = ? AND status = 'active' RETURNING id`, old.ID); err != nil {
			return err
		}
		if err := tx.Create(next); err != nil {
			if sql.IsUniqueViolation(err) {
				return sql.ErrDuplicate
			}
			return err
		}
		if err := tx.Create(order); err != nil {
			if sql.IsUniqueViolation(err) {
				return sql.ErrDuplicate
			}
			return err
		}
		return nil
	})
}

// conditionalUpdate runs a conditional UPDATE … RETURNING id and maps "no row matched" to sql.ErrStateChanged.
func conditionalUpdate(db sql.ORM, query string, args ...any) error {
	var ids []string
	if err := db.RawSql(query, &ids, args...); err != nil {
		return err
	}
	if len(ids) == 0 {
		return sql.ErrStateChanged
	}
	return nil
}
