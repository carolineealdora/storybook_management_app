package subscription

import (
	"errors"
	"testing"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models"
	repository_mock "codebase/test/mock/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var now = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

type fixture struct {
	subs *repository_mock.SubscriptionRepository
	svc  *ServiceImpl
}

func newFixture() fixture {
	f := fixture{subs: new(repository_mock.SubscriptionRepository)}
	f.svc = &ServiceImpl{subs: f.subs, errors: helpers.NewError(nil), now: func() time.Time { return now }}
	return f
}

func code(err error) string { return helpers.ErrCode(err) }

// --- Plan (US-17) ---

func plan49k() *models.SubscriptionPlan {
	return &models.SubscriptionPlan{ID: "plan-1", Code: "monthly", Name: "Monthly Subscription", Price: 49000, Benefits: "old", Duration: "1 month"}
}

func history() []models.PlanPriceChange {
	return []models.PlanPriceChange{{ID: "h-1", PlanID: "plan-1", OldPrice: 49000, NewPrice: 59000, ChangedBy: "admin-1", ChangedAt: now}}
}

// --- Admin plan view with price history (CR-001-2) ---

func TestGetAdminPlan_ReturnsPlanWithPriceHistory(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(plan49k(), nil)
	f.subs.On("ListPriceHistory", "plan-1").Return(history(), nil)
	got, err := f.svc.GetAdminPlan()
	require.NoError(t, err)
	assert.Equal(t, "monthly", got.Code)
	assert.Equal(t, 49000, got.Price)
	require.Len(t, got.PriceHistory, 1)
	assert.Equal(t, 49000, got.PriceHistory[0].OldPrice)
	assert.Equal(t, 59000, got.PriceHistory[0].NewPrice)
	assert.Equal(t, "admin-1", got.PriceHistory[0].ChangedBy)
}

func TestGetAdminPlan_NoHistory_ReturnsEmptyList(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(plan49k(), nil)
	f.subs.On("ListPriceHistory", "plan-1").Return(nil, nil)
	got, err := f.svc.GetAdminPlan()
	require.NoError(t, err)
	assert.NotNil(t, got.PriceHistory, "renders as [] not null")
}

func TestGetAdminPlan_Errors(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.GetAdminPlan()
	assert.Equal(t, respConst.PLAN_NOT_FOUND, code(err))

	f = newFixture()
	f.subs.On("GetPlan").Return(plan49k(), nil)
	f.subs.On("ListPriceHistory", "plan-1").Return(nil, errors.New("db down"))
	_, err = f.svc.GetAdminPlan()
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- Update plan (US-17, CR-001-2 P1, E-CR2, E-CR6) ---

func TestUpdatePlan_PriceChanged_SavesWithAdminAndReturnsHistory(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(plan49k(), nil).Once()
	f.subs.On("SavePlan", mock.MatchedBy(func(p *models.SubscriptionPlan) bool {
		return p.ID == "plan-1" && p.Price == 59000 && p.Benefits == "new" && p.UpdatedAt.Equal(now) &&
			p.UpdatedBy != nil && *p.UpdatedBy == "admin-1"
	})).Return(nil)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 59000, Benefits: "new"}, nil).Once()
	f.subs.On("ListPriceHistory", "plan-1").Return(history(), nil)

	got, err := f.svc.UpdatePlan("admin-1", 59000, "  new ")
	require.NoError(t, err)
	assert.Equal(t, 59000, got.Price)
	assert.Len(t, got.PriceHistory, 1)
	f.subs.AssertExpectations(t)
}

func TestUpdatePlan_SamePriceNewBenefits_SavesBenefits(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(plan49k(), nil)
	f.subs.On("SavePlan", mock.MatchedBy(func(p *models.SubscriptionPlan) bool { return p.Price == 49000 && p.Benefits == "new" })).Return(nil)
	f.subs.On("ListPriceHistory", "plan-1").Return(nil, nil)
	_, err := f.svc.UpdatePlan("admin-1", 49000, "new")
	require.NoError(t, err)
	f.subs.AssertCalled(t, "SavePlan", mock.Anything)
}

func TestUpdatePlan_SamePriceSameBenefits_NoOp(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(plan49k(), nil)
	f.subs.On("ListPriceHistory", "plan-1").Return(nil, nil)
	got, err := f.svc.UpdatePlan("admin-1", 49000, "old")
	require.NoError(t, err)
	assert.Equal(t, 49000, got.Price)
	f.subs.AssertNotCalled(t, "SavePlan", mock.Anything)
}

func TestUpdatePlan_NoneYet_CreatesMonthlyPlan(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound).Once()
	f.subs.On("SavePlan", mock.MatchedBy(func(p *models.SubscriptionPlan) bool {
		return p.ID == "" && p.Code == "monthly" && p.Name == "Monthly Subscription" && p.Duration == "1 month" && *p.UpdatedBy == "admin-1"
	})).Return(nil)
	f.subs.On("GetPlan").Return(plan49k(), nil).Once()
	f.subs.On("ListPriceHistory", "plan-1").Return(nil, nil)
	_, err := f.svc.UpdatePlan("admin-1", 49000, "all")
	require.NoError(t, err)
	f.subs.AssertExpectations(t)
}

func TestUpdatePlan_PriceNotPositive_ReturnsInvalidPrice(t *testing.T) {
	f := newFixture()
	for _, price := range []int{0, -5} {
		_, err := f.svc.UpdatePlan("admin-1", price, "all")
		assert.Equal(t, respConst.INVALID_PRICE, code(err), price)
	}
	f.subs.AssertNotCalled(t, "SavePlan", mock.Anything)
}

func TestUpdatePlan_BlankBenefits_ReturnsValidationError(t *testing.T) {
	f := newFixture()
	_, err := f.svc.UpdatePlan("admin-1", 49000, "   ")
	assert.Equal(t, respConst.VALIDATION_ERROR, code(err))
}

func TestUpdatePlan_SaveFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound)
	f.subs.On("SavePlan", mock.Anything).Return(errors.New("db down"))
	_, err := f.svc.UpdatePlan("admin-1", 49000, "all")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}
