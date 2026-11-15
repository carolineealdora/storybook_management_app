package subscription

// SECURITY: manual review required — subscription checkout, mocked payment, cancellation, auto-renew.

import (
	"errors"
	"fmt"
	"time"

	respConst "codebase/internal/constant/response"
	"codebase/models"
	"codebase/models/dto"
	sql "codebase/pkg/orm"

	"gorm.io/gorm"
)

const (
	paymentSuccess = "success"
	renewBatch     = 500
)

// CreateOrder starts a new monthly subscription checkout at the current plan price (US-10, E3).
func (s *ServiceImpl) CreateOrder(userID, idempotencyKey string) (*dto.SubscriptionOrderView, error) {
	if err := s.ensureNotSubscribed(userID); err != nil {
		return nil, err
	}
	plan, err := s.subs.GetPlan()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.PLAN_NOT_FOUND, "no plan configured")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	now := s.now()
	o := &models.SubscriptionOrder{UserID: userID, PlanID: plan.ID, OrderType: models.OrderTypeNew, Price: plan.Price,
		Status: models.OrderPending, IdempotencyKey: idempotencyKey, CreatedAt: now, UpdatedAt: now}
	if err := s.subs.CreateOrder(o); errors.Is(err, sql.ErrDuplicate) {
		return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "idempotency key reused")
	} else if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return toOrderView(*o), nil
}

// PayOrder runs the mocked payment. Success creates an active period of one month at the order's price;
// failure creates no subscription and the parent can retry with a new order.
func (s *ServiceImpl) PayOrder(userID, orderID, result string) (*dto.SubscriptionView, error) {
	o, err := s.subs.GetOrder(orderID, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.ORDER_NOT_FOUND, "subscription order "+orderID)
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if o.Status != models.OrderPending {
		return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "order already "+o.Status)
	}
	if err := s.ensureNotSubscribed(userID); err != nil {
		return nil, err
	}

	now := s.now()
	if result != paymentSuccess {
		if err := s.subs.MarkOrderFailed(o.ID, now); errors.Is(err, sql.ErrStateChanged) {
			return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "order changed concurrently")
		} else if err != nil {
			return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
		}
		return nil, s.errors.Wrap(respConst.PAYMENT_FAILED, "mock payment failed for subscription order "+o.ID)
	}

	sub := &models.Subscription{UserID: userID, PlanID: o.PlanID, PricePaid: o.Price, Status: models.SubActive,
		StartedAt: now, ExpiresAt: addMonth(now), CreatedAt: now, UpdatedAt: now}
	switch err := s.subs.Activate(o, sub, now); {
	case errors.Is(err, sql.ErrDuplicate):
		return nil, s.errors.Wrap(respConst.SUBSCRIPTION_ACTIVE, "activated concurrently")
	case errors.Is(err, sql.ErrStateChanged):
		return nil, s.errors.Wrap(respConst.DUPLICATE_REQUEST, "order changed concurrently")
	case err != nil:
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	o.Status, o.PaidAt, o.SubscriptionID = models.OrderPaid, &now, &sub.ID
	v := toView(*sub)
	v.Order = toOrderView(*o)
	return &v, nil
}

// Cancel ends access to non-owned books and auto-renewal immediately; no refund (US-11, E14).
func (s *ServiceImpl) Cancel(userID string) (*dto.SubscriptionView, error) {
	sub, err := s.subs.GetActive(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.SUBSCRIPTION_NOT_ACTIVE, "nothing to cancel")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	now := s.now()
	if err := s.subs.Cancel(sub.ID, now); errors.Is(err, sql.ErrStateChanged) {
		return nil, s.errors.Wrap(respConst.SUBSCRIPTION_NOT_ACTIVE, "cancelled or renewed concurrently")
	} else if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	sub.Status, sub.CancelledAt = models.SubCancelled, &now
	v := toView(*sub)
	return &v, nil
}

func (s *ServiceImpl) History(userID string) ([]dto.SubscriptionView, error) {
	rows, err := s.subs.ListHistory(userID)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	out := make([]dto.SubscriptionView, 0, len(rows))
	for _, r := range rows {
		v := toView(r.Subscription)
		if r.OrderID != nil {
			v.Order = &dto.SubscriptionOrderView{ID: *r.OrderID, PlanID: deref(r.OrderPlanID), OrderType: deref(r.OrderType),
				Price: derefInt(r.OrderPrice), Status: deref(r.OrderStatus), PaidAt: r.OrderPaidAt}
			if r.OrderCreatedAt != nil {
				v.Order.CreatedAt = *r.OrderCreatedAt
			}
		}
		out = append(out, v)
	}
	return out, nil
}

// RenewDue renews every active period that has ended (mocked payment always succeeds, US-10).
// Each renewal expires the old period and starts a back-to-back one at the current plan price (US-13, US-17).
// Periods cancelled or renewed meanwhile are skipped; one failure does not stop the others.
func (s *ServiceImpl) RenewDue() (int, error) {
	now := s.now()
	due, err := s.subs.ListDue(now, renewBatch)
	if err != nil || len(due) == 0 {
		return 0, err
	}
	plan, err := s.subs.GetPlan()
	if err != nil {
		return 0, fmt.Errorf("renewal plan lookup: %w", err)
	}
	renewed, failures := 0, []error{}
	for _, old := range due {
		oldID := old.ID
		next := &models.Subscription{UserID: old.UserID, PlanID: plan.ID, RenewedFromID: &oldID, PricePaid: plan.Price,
			Status: models.SubActive, StartedAt: old.ExpiresAt, ExpiresAt: addMonth(old.ExpiresAt), CreatedAt: now, UpdatedAt: now}
		order := &models.SubscriptionOrder{UserID: old.UserID, PlanID: plan.ID, OrderType: models.OrderTypeRenewal, Price: plan.Price,
			Status: models.OrderPaid, IdempotencyKey: "renewal:" + old.ID, CreatedAt: now, UpdatedAt: now, PaidAt: &now}
		switch err := s.subs.Renew(&old, next, order); {
		case err == nil:
			renewed++
		case errors.Is(err, sql.ErrStateChanged), errors.Is(err, sql.ErrDuplicate):
			// cancelled meanwhile, or another run already renewed it
		default:
			failures = append(failures, fmt.Errorf("renew %s: %w", old.ID, err))
		}
	}
	return renewed, errors.Join(failures...)
}

func (s *ServiceImpl) ensureNotSubscribed(userID string) error {
	_, err := s.subs.GetActive(userID)
	if err == nil {
		return s.errors.Wrap(respConst.SUBSCRIPTION_ACTIVE, "already subscribed")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return nil
}

// addMonth moves t one calendar month ahead, clamping to the last day of a shorter month (Jan 31 → Feb 28).
func addMonth(t time.Time) time.Time {
	y, m, d := t.Date()
	firstOfNext := time.Date(y, m+1, 1, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	lastDay := firstOfNext.AddDate(0, 1, -1).Day()
	return firstOfNext.AddDate(0, 0, min(d, lastDay)-1)
}

func toView(sub models.Subscription) dto.SubscriptionView {
	return dto.SubscriptionView{ID: sub.ID, Status: sub.Status, PricePaid: sub.PricePaid, StartedAt: sub.StartedAt,
		ExpiresAt: sub.ExpiresAt, CancelledAt: sub.CancelledAt, RenewedFromID: sub.RenewedFromID}
}

func toOrderView(o models.SubscriptionOrder) *dto.SubscriptionOrderView {
	return &dto.SubscriptionOrderView{ID: o.ID, PlanID: o.PlanID, OrderType: o.OrderType, Price: o.Price, Status: o.Status, CreatedAt: o.CreatedAt, PaidAt: o.PaidAt}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
