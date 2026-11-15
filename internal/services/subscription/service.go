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
		GetPlan() (*dto.PlanView, error)
		UpdatePlan(price int, benefits string) (*dto.PlanView, error)
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

func (s *ServiceImpl) GetPlan() (*dto.PlanView, error) {
	p, err := s.subs.GetPlan()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.PLAN_NOT_FOUND, "no plan configured")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return toPlanView(p), nil
}

// UpdatePlan changes the offer for new subscriptions and renewals; active periods keep their price_paid (US-17).
func (s *ServiceImpl) UpdatePlan(price int, benefits string) (*dto.PlanView, error) {
	benefits = strings.TrimSpace(benefits)
	if price <= 0 {
		return nil, s.errors.Wrap(respConst.VALIDATION_ERROR, "price must be a whole number above 0.")
	}
	if benefits == "" {
		return nil, s.errors.Wrap(respConst.VALIDATION_ERROR, "benefits is a required field.")
	}
	p, err := s.subs.GetPlan()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		p, err = &models.SubscriptionPlan{Duration: "1 month", CreatedAt: s.now()}, nil
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	p.Price, p.Benefits, p.UpdatedAt = price, benefits, s.now()
	if err := s.subs.SavePlan(p); err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return toPlanView(p), nil
}

func toPlanView(p *models.SubscriptionPlan) *dto.PlanView {
	return &dto.PlanView{ID: p.ID, Price: p.Price, Benefits: p.Benefits, Duration: p.Duration, UpdatedAt: p.UpdatedAt}
}
