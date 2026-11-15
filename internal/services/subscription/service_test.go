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

func TestGetPlan_Configured_ReturnsPriceAndBenefits(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 49000, Benefits: "All books", Duration: "1 month"}, nil)
	got, err := f.svc.GetPlan()
	require.NoError(t, err)
	assert.Equal(t, 49000, got.Price)
	assert.Equal(t, "All books", got.Benefits)
}

func TestGetPlan_NotConfigured_ReturnsPlanNotFound(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.GetPlan()
	assert.Equal(t, respConst.PLAN_NOT_FOUND, code(err))
}

func TestUpdatePlan_Existing_UpdatesInPlace(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 49000, Benefits: "old", Duration: "1 month"}, nil)
	f.subs.On("SavePlan", mock.MatchedBy(func(p *models.SubscriptionPlan) bool {
		return p.ID == "plan-1" && p.Price == 59000 && p.Benefits == "new" && p.UpdatedAt.Equal(now)
	})).Return(nil)
	got, err := f.svc.UpdatePlan(59000, "  new ")
	require.NoError(t, err)
	assert.Equal(t, 59000, got.Price)
	f.subs.AssertExpectations(t)
}

func TestUpdatePlan_NoneYet_CreatesOne(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound)
	f.subs.On("SavePlan", mock.MatchedBy(func(p *models.SubscriptionPlan) bool { return p.ID == "" && p.Duration == "1 month" })).Return(nil)
	_, err := f.svc.UpdatePlan(49000, "all")
	require.NoError(t, err)
}

func TestUpdatePlan_InvalidInput_ReturnsValidationError(t *testing.T) {
	f := newFixture()
	_, err := f.svc.UpdatePlan(0, "all")
	assert.Equal(t, respConst.VALIDATION_ERROR, code(err), "price must be > 0")
	_, err = f.svc.UpdatePlan(-5, "all")
	assert.Equal(t, respConst.VALIDATION_ERROR, code(err))
	_, err = f.svc.UpdatePlan(49000, "   ")
	assert.Equal(t, respConst.VALIDATION_ERROR, code(err), "benefits required")
}

func TestUpdatePlan_SaveFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound)
	f.subs.On("SavePlan", mock.Anything).Return(errors.New("db down"))
	_, err := f.svc.UpdatePlan(49000, "all")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}
