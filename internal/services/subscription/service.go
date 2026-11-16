package subscription

import (
	"errors"
	"strings"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/internal/repositories/subscription"
	"codebase/models"
	"codebase/models/dto"
	"codebase/pkg/shared"

	"gorm.io/gorm"
)

type (
	Service interface {
		GetAdminPlan() (*dto.AdminPlanView, error)
		UpdatePlan(adminID string, price int, benefits string) (*dto.AdminPlanView, error)
	}

	ServiceImpl struct {
		subs   subscription.Repository
		errors helpers.Error
		now    shared.Now
	}
)

func NewService(holder *models.SharedHolder) Service {
	return &ServiceImpl{subs: subscription.NewRepository(holder), errors: helpers.NewError(holder), now: time.Now}
}

// GetAdminPlan is the back-office view: the plan plus its price history, newest first (CR-001-2).
func (s *ServiceImpl) GetAdminPlan() (*dto.AdminPlanView, error) {
	p, err := s.plan()
	if err != nil {
		return nil, err
	}
	rows, err := s.subs.ListPriceHistory(p.ID)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	v := &dto.AdminPlanView{PlanView: *toPlanView(p), UpdatedBy: p.UpdatedBy, PriceHistory: make([]dto.PlanPriceChangeView, 0, len(rows))}
	for _, h := range rows {
		v.PriceHistory = append(v.PriceHistory, dto.PlanPriceChangeView{ID: h.ID, OldPrice: h.OldPrice, NewPrice: h.NewPrice, ChangedBy: h.ChangedBy, ChangedAt: h.ChangedAt})
	}
	return v, nil
}

// UpdatePlan changes the offer for new subscriptions and renewals; active periods keep their price_paid
// until they renew (US-17, CR-001-2 P1–P6). A price change is audited in the price history by the repository.
// SECURITY: manual review required — changes what every future subscription and renewal is charged.
func (s *ServiceImpl) UpdatePlan(adminID string, price int, benefits string) (*dto.AdminPlanView, error) {
	benefits = strings.TrimSpace(benefits)
	if price <= 0 {
		return nil, s.errors.Wrap(respConst.INVALID_PRICE, "price must be a whole number above 0")
	}
	if benefits == "" {
		return nil, s.errors.Wrap(respConst.VALIDATION_ERROR, "benefits is a required field.")
	}
	p, err := s.subs.GetPlan()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		p, err = &models.SubscriptionPlan{Code: models.PlanCodeMonthly, Name: models.PlanNameMonthly, Duration: "1 month", CreatedAt: s.now()}, nil
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if p.ID != "" && p.Price == price && p.Benefits == benefits {
		return s.GetAdminPlan() // E-CR6: nothing changed, nothing written
	}
	p.Price, p.Benefits, p.UpdatedAt, p.UpdatedBy = price, benefits, s.now(), &adminID
	if err := s.subs.SavePlan(p); err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return s.GetAdminPlan()
}

func (s *ServiceImpl) plan() (*models.SubscriptionPlan, error) {
	p, err := s.subs.GetPlan()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.PLAN_NOT_FOUND, "no plan configured")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return p, nil
}

func toPlanView(p *models.SubscriptionPlan) *dto.PlanView {
	return &dto.PlanView{ID: p.ID, Code: p.Code, Name: p.Name, Price: p.Price, Benefits: p.Benefits, Duration: p.Duration, UpdatedAt: p.UpdatedAt}
}
