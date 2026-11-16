package account

import (
	"errors"
	"testing"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models"
	"codebase/models/dto"
	repository_mock "codebase/test/mock/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var now = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

type fixture struct {
	accounts *repository_mock.AccountRepository
	subs     *repository_mock.SubscriptionRepository
	svc      *ServiceImpl
}

func newFixture() fixture {
	f := fixture{accounts: new(repository_mock.AccountRepository), subs: new(repository_mock.SubscriptionRepository)}
	f.svc = &ServiceImpl{accounts: f.accounts, subscriptions: f.subs, errors: helpers.NewError(nil), now: func() time.Time { return now }, hashCost: bcrypt.MinCost}
	return f
}

func hash(t *testing.T, s string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func userWithPin(t *testing.T, pin string) *models.User {
	u := &models.User{ID: "user-1", Name: "Parent"}
	if pin != "" {
		h := hash(t, pin)
		u.PinHash = &h
	}
	return u
}

func (f fixture) withPassword(t *testing.T, pw string) {
	uid := "user-1"
	f.accounts.On("GetCredentialByUserID", "user-1").Return(&models.Credential{ID: "cred-1", Email: "parent@example.com", PasswordHash: hash(t, pw), UserID: &uid}, nil)
}

// --- Profile ---

func TestGetProfile_ActiveSubscription_ShowsSubscriberAndRenewalDate(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, "1234"), nil)
	f.withPassword(t, "pw")
	exp := now.AddDate(0, 1, 0)
	f.subs.On("GetActive", "user-1").Return(&models.Subscription{ID: "sub-1", Status: models.SubActive, PricePaid: 49000, StartedAt: now, ExpiresAt: exp}, nil)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 49000}, nil)

	p, err := f.svc.GetProfile("user-1")
	require.NoError(t, err)
	assert.Equal(t, "Parent", p.Name)
	assert.Equal(t, "parent@example.com", p.Email)
	assert.True(t, p.PinSet)
	assert.True(t, p.IsSubscriber)
	assert.Equal(t, models.PlanStatusSubscribed, p.PlanStatus)
	require.NotNil(t, p.Subscription)
	assert.Equal(t, exp, p.Subscription.ExpiresAt)
	assert.Equal(t, 49000, p.Subscription.NextRenewalPrice)
	assert.False(t, p.Subscription.PriceChangePending)
}

// CR-001-2 P2: the subscriber keeps price_paid until renewal and sees the upcoming price.
func TestGetProfile_PlanPriceChanged_ShowsPendingPriceChange(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, ""), nil)
	f.withPassword(t, "pw")
	f.subs.On("GetActive", "user-1").Return(&models.Subscription{ID: "sub-1", Status: models.SubActive, PricePaid: 49000, StartedAt: now, ExpiresAt: now.AddDate(0, 1, 0)}, nil)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 59000}, nil)

	p, err := f.svc.GetProfile("user-1")
	require.NoError(t, err)
	assert.Equal(t, 49000, p.Subscription.PricePaid)
	assert.Equal(t, 59000, p.Subscription.NextRenewalPrice)
	assert.True(t, p.Subscription.PriceChangePending)
}

func TestGetProfile_PlanLookupFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, ""), nil)
	f.withPassword(t, "pw")
	f.subs.On("GetActive", "user-1").Return(&models.Subscription{ID: "sub-1", Status: models.SubActive, PricePaid: 49000}, nil)
	f.subs.On("GetPlan").Return(nil, errors.New("db down"))
	_, err := f.svc.GetProfile("user-1")
	assert.Equal(t, respConst.GENERAL_ERROR, helpers.ErrCode(err))
}

func TestGetProfile_NoSubscriptionNoPin_ShowsNonSubscriber(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, ""), nil)
	f.withPassword(t, "pw")
	f.subs.On("GetActive", "user-1").Return(nil, gorm.ErrRecordNotFound)

	p, err := f.svc.GetProfile("user-1")
	require.NoError(t, err)
	assert.False(t, p.PinSet)
	assert.False(t, p.IsSubscriber)
	assert.Equal(t, models.PlanStatusFree, p.PlanStatus, "no, expired, or cancelled subscription → free (B2, B3); purchases are not consulted (B4)")
	assert.Nil(t, p.Subscription)
	f.subs.AssertNotCalled(t, "GetPlan")
}

func TestGetProfile_SubscriptionLookupFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, ""), nil)
	f.withPassword(t, "pw")
	f.subs.On("GetActive", "user-1").Return(nil, errors.New("db down"))
	_, err := f.svc.GetProfile("user-1")
	assert.Equal(t, respConst.GENERAL_ERROR, helpers.ErrCode(err))
}

func TestGetProfile_UserMissing_ReturnsUnauthorized(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.GetProfile("user-1")
	assert.Equal(t, respConst.UNAUTHORIZED, helpers.ErrCode(err))
}

func TestUpdateName_Trimmed_SavesAndReturnsProfile(t *testing.T) {
	f := newFixture()
	f.accounts.On("UpdateUserName", "user-1", "New Name", now).Return(nil)
	f.accounts.On("GetUser", "user-1").Return(&models.User{ID: "user-1", Name: "New Name"}, nil)
	f.withPassword(t, "pw")
	f.subs.On("GetActive", "user-1").Return(nil, gorm.ErrRecordNotFound)

	p, err := f.svc.UpdateName("user-1", "  New Name  ")
	require.NoError(t, err)
	assert.Equal(t, "New Name", p.Name)
}

func TestUpdateName_Blank_ReturnsValidationError(t *testing.T) {
	f := newFixture()
	_, err := f.svc.UpdateName("user-1", "   ")
	assert.Equal(t, respConst.VALIDATION_ERROR, helpers.ErrCode(err))
	f.accounts.AssertNotCalled(t, "UpdateUserName", mock.Anything, mock.Anything, mock.Anything)
}

// --- Set PIN (US-14) ---

func TestSetPin_FirstPin_NoPasswordNeeded(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, ""), nil)
	var stored string
	f.accounts.On("SetPin", "user-1", mock.Anything, now).Run(func(a mock.Arguments) { stored = a.String(1) }).Return(nil)

	require.NoError(t, f.svc.SetPin("user-1", dto.SetPinRequest{Pin: "1234", ConfirmPin: "1234"}))
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored), []byte("1234")), "PIN stored as bcrypt hash")
	assert.NotEqual(t, "1234", stored)
}

func TestSetPin_ChangeWithoutPassword_ReturnsValidationError(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, "1111"), nil)
	err := f.svc.SetPin("user-1", dto.SetPinRequest{Pin: "1234", ConfirmPin: "1234"})
	assert.Equal(t, respConst.VALIDATION_ERROR, helpers.ErrCode(err))
}

func TestSetPin_ChangeWithWrongPassword_ReturnsInvalidPassword(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, "1111"), nil)
	f.withPassword(t, "right")
	err := f.svc.SetPin("user-1", dto.SetPinRequest{Pin: "1234", ConfirmPin: "1234", Password: "wrong"})
	assert.Equal(t, respConst.INVALID_PASSWORD, helpers.ErrCode(err))
	f.accounts.AssertNotCalled(t, "SetPin", mock.Anything, mock.Anything, mock.Anything)
}

func TestSetPin_ChangeWithPassword_Saves(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, "1111"), nil)
	f.withPassword(t, "right")
	f.accounts.On("SetPin", "user-1", mock.Anything, now).Return(nil)
	require.NoError(t, f.svc.SetPin("user-1", dto.SetPinRequest{Pin: "1234", ConfirmPin: "1234", Password: "right"}))
}

func TestSetPin_Mismatch_ReturnsValidationError(t *testing.T) {
	f := newFixture()
	err := f.svc.SetPin("user-1", dto.SetPinRequest{Pin: "1234", ConfirmPin: "4321"})
	assert.Equal(t, respConst.VALIDATION_ERROR, helpers.ErrCode(err))
}

func TestSetPin_NotFourDigits_ReturnsValidationError(t *testing.T) {
	f := newFixture()
	for _, pin := range []string{"123", "12345", "12a4", "１２３４"} {
		err := f.svc.SetPin("user-1", dto.SetPinRequest{Pin: pin, ConfirmPin: pin})
		assert.Equal(t, respConst.VALIDATION_ERROR, helpers.ErrCode(err), pin)
	}
}

// --- Verify PIN (US-03, E11, E12) ---

func TestVerifyPin_NoPinSet_ReturnsPinNotSet(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, ""), nil)
	assert.Equal(t, respConst.PIN_NOT_SET, helpers.ErrCode(f.svc.VerifyPin("user-1", "1234")))
}

func TestVerifyPin_Correct_ResetsFailures(t *testing.T) {
	f := newFixture()
	u := userWithPin(t, "1234")
	u.PinFailedAttempts = 2
	f.accounts.On("GetUser", "user-1").Return(u, nil)
	f.accounts.On("ResetPinFailures", "user-1").Return(nil)
	require.NoError(t, f.svc.VerifyPin("user-1", "1234"))
	f.accounts.AssertExpectations(t)
}

func TestVerifyPin_CorrectWithNoPriorFailures_SkipsReset(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, "1234"), nil)
	require.NoError(t, f.svc.VerifyPin("user-1", "1234"))
	f.accounts.AssertNotCalled(t, "ResetPinFailures", mock.Anything)
}

func TestVerifyPin_WrongBelowLimit_ReturnsInvalidPin(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, "1234"), nil)
	f.accounts.On("IncrementPinFailures", "user-1").Return(4, nil)
	assert.Equal(t, respConst.INVALID_PIN, helpers.ErrCode(f.svc.VerifyPin("user-1", "0000")))
	f.accounts.AssertNotCalled(t, "LockPin", mock.Anything, mock.Anything)
}

func TestVerifyPin_FifthWrongAttempt_LocksForOneMinute(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(userWithPin(t, "1234"), nil)
	f.accounts.On("IncrementPinFailures", "user-1").Return(5, nil)
	f.accounts.On("LockPin", "user-1", now.Add(time.Minute)).Return(nil)
	assert.Equal(t, respConst.TOO_MANY_ATTEMPTS, helpers.ErrCode(f.svc.VerifyPin("user-1", "0000")))
	f.accounts.AssertExpectations(t)
}

func TestVerifyPin_WhileLocked_RejectsEvenCorrectPin(t *testing.T) {
	f := newFixture()
	u := userWithPin(t, "1234")
	until := now.Add(30 * time.Second)
	u.PinLockedUntil = &until
	f.accounts.On("GetUser", "user-1").Return(u, nil)
	assert.Equal(t, respConst.TOO_MANY_ATTEMPTS, helpers.ErrCode(f.svc.VerifyPin("user-1", "1234")))
	f.accounts.AssertNotCalled(t, "IncrementPinFailures", mock.Anything)
}

func TestVerifyPin_LockExpired_AllowsAgain(t *testing.T) {
	f := newFixture()
	u := userWithPin(t, "1234")
	until := now.Add(-time.Second)
	u.PinLockedUntil = &until
	f.accounts.On("GetUser", "user-1").Return(u, nil)
	f.accounts.On("ResetPinFailures", "user-1").Return(nil)
	assert.NoError(t, f.svc.VerifyPin("user-1", "1234"))
}

func TestVerifyPin_DBDown_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.accounts.On("GetUser", "user-1").Return(nil, errors.New("db down"))
	assert.Equal(t, respConst.GENERAL_ERROR, helpers.ErrCode(f.svc.VerifyPin("user-1", "1234")))
}

// --- Forgot PIN (E13) ---

func TestForgotPin_CorrectPassword_ClearsLock(t *testing.T) {
	f := newFixture()
	f.withPassword(t, "right")
	f.accounts.On("ResetPinFailures", "user-1").Return(nil)
	require.NoError(t, f.svc.ForgotPin("user-1", "right"))
	f.accounts.AssertExpectations(t)
}

func TestForgotPin_WrongPassword_ReturnsInvalidPassword(t *testing.T) {
	f := newFixture()
	f.withPassword(t, "right")
	assert.Equal(t, respConst.INVALID_PASSWORD, helpers.ErrCode(f.svc.ForgotPin("user-1", "wrong")))
	f.accounts.AssertNotCalled(t, "ResetPinFailures", mock.Anything)
}
