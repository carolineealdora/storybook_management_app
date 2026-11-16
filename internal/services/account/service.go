package account

// SECURITY: manual review required — PIN gate between Child and Parent view.

import (
	"errors"
	"strings"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/internal/repositories/account"
	"codebase/internal/repositories/subscription"
	"codebase/models"
	"codebase/models/dto"
	"codebase/pkg/shared"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	maxPinAttempts = 5
	pinLockFor     = time.Minute
)

type (
	Service interface {
		GetProfile(userID string) (*dto.Profile, error)
		UpdateName(userID, name string) (*dto.Profile, error)
		SetPin(userID string, req dto.SetPinRequest) error
		VerifyPin(userID, pin string) error
		ForgotPin(userID, password string) error
	}

	ServiceImpl struct {
		accounts      account.Repository
		subscriptions subscription.Repository
		errors        helpers.Error
		now           shared.Now
		hashCost      int
	}
)

func NewService(holder *models.SharedHolder) Service {
	return &ServiceImpl{
		accounts:      account.NewRepository(holder),
		subscriptions: subscription.NewRepository(holder),
		errors:        helpers.NewError(holder),
		now:           time.Now,
		hashCost:      bcrypt.DefaultCost,
	}
}

func (s *ServiceImpl) GetProfile(userID string) (*dto.Profile, error) {
	user, err := s.accounts.GetUser(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.UNAUTHORIZED, "user in token no longer exists")
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	cred, err := s.accounts.GetCredentialByUserID(userID)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	p := &dto.Profile{ID: user.ID, Name: user.Name, Email: cred.Email, PinSet: user.PinHash != nil, PlanStatus: models.PlanStatusFree}

	// Badge (CR-001-1): an active period means "subscribed", the same rule as the book access check (ADR-006).
	// Purchases never affect it (B4).
	sub, err := s.subscriptions.GetActive(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, nil
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	plan, err := s.subscriptions.GetPlan()
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	p.IsSubscriber, p.PlanStatus = true, models.PlanStatusSubscribed
	p.Subscription = &dto.ProfileSubscription{
		SubscriptionView: dto.SubscriptionView{
			ID: sub.ID, Status: sub.Status, PricePaid: sub.PricePaid, StartedAt: sub.StartedAt,
			ExpiresAt: sub.ExpiresAt, CancelledAt: sub.CancelledAt, RenewedFromID: sub.RenewedFromID,
		},
		// CR-001-2 P2/P5: the period keeps price_paid; renewal charges the plan price at that moment.
		NextRenewalPrice:   plan.Price,
		PriceChangePending: plan.Price != sub.PricePaid,
	}
	return p, nil
}

func (s *ServiceImpl) UpdateName(userID, name string) (*dto.Profile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, s.errors.Wrap(respConst.VALIDATION_ERROR, "name is a required field.")
	}
	if err := s.accounts.UpdateUserName(userID, name, s.now()); err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return s.GetProfile(userID)
}

func (s *ServiceImpl) SetPin(userID string, req dto.SetPinRequest) error {
	if !isFourDigits(req.Pin) {
		return s.errors.Wrap(respConst.VALIDATION_ERROR, "pin must be exactly 4 digits.")
	}
	if req.Pin != req.ConfirmPin {
		return s.errors.Wrap(respConst.VALIDATION_ERROR, "confirm_pin does not match.")
	}
	user, err := s.accounts.GetUser(userID)
	if err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if user.PinHash != nil { // changing an existing PIN requires the account password (US-14)
		if req.Password == "" {
			return s.errors.Wrap(respConst.VALIDATION_ERROR, "password is required to change the PIN.")
		}
		if err := s.checkPassword(userID, req.Password); err != nil {
			return err
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Pin), s.hashCost)
	if err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if err := s.accounts.SetPin(userID, string(hash), s.now()); err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return nil
}

// VerifyPin gates Child → Parent view. 5 wrong attempts lock verification for 1 minute (E12).
func (s *ServiceImpl) VerifyPin(userID, pin string) error {
	user, err := s.accounts.GetUser(userID)
	if err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if user.PinHash == nil {
		return s.errors.Wrap(respConst.PIN_NOT_SET, "no pin set")
	}
	now := s.now()
	if user.PinLockedUntil != nil && now.Before(*user.PinLockedUntil) {
		return s.errors.Wrap(respConst.TOO_MANY_ATTEMPTS, "pin locked")
	}
	if bcrypt.CompareHashAndPassword([]byte(*user.PinHash), []byte(pin)) == nil {
		if user.PinFailedAttempts > 0 || user.PinLockedUntil != nil {
			if err := s.accounts.ResetPinFailures(userID); err != nil {
				return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
			}
		}
		return nil
	}
	attempts, err := s.accounts.IncrementPinFailures(userID)
	if err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if attempts >= maxPinAttempts {
		if err := s.accounts.LockPin(userID, now.Add(pinLockFor)); err != nil {
			return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
		}
		return s.errors.Wrap(respConst.TOO_MANY_ATTEMPTS, "pin locked after 5 wrong attempts")
	}
	return s.errors.Wrap(respConst.INVALID_PIN, "wrong pin")
}

// ForgotPin lets the account password stand in for the PIN (E13) and clears any PIN lock.
func (s *ServiceImpl) ForgotPin(userID, password string) error {
	if err := s.checkPassword(userID, password); err != nil {
		return err
	}
	if err := s.accounts.ResetPinFailures(userID); err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return nil
}

func (s *ServiceImpl) checkPassword(userID, password string) error {
	cred, err := s.accounts.GetCredentialByUserID(userID)
	if err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	if bcrypt.CompareHashAndPassword([]byte(cred.PasswordHash), []byte(password)) != nil {
		return s.errors.Wrap(respConst.INVALID_PASSWORD, "wrong account password")
	}
	return nil
}

func isFourDigits(s string) bool {
	if len(s) != 4 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
