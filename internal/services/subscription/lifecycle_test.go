package subscription

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	respConst "codebase/internal/constant/response"
	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func (f fixture) active(yes bool) {
	if yes {
		f.subs.On("GetActive", "user-1").Return(&models.Subscription{ID: "sub-1", UserID: "user-1", Status: models.SubActive, PricePaid: 49000, StartedAt: now, ExpiresAt: now.AddDate(0, 1, 0)}, nil)
	} else {
		f.subs.On("GetActive", "user-1").Return(nil, gorm.ErrRecordNotFound)
	}
}

func (f fixture) plan(price int) {
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: price}, nil)
}

func pendingOrder() *models.SubscriptionOrder {
	return &models.SubscriptionOrder{ID: "so-1", UserID: "user-1", PlanID: "plan-1", OrderType: models.OrderTypeNew, Price: 49000, Status: models.OrderPending, IdempotencyKey: "k1"}
}

// --- addMonth ---

func TestAddMonth_ClampsToEndOfShorterMonth(t *testing.T) {
	d := func(y int, m time.Month, day int) time.Time { return time.Date(y, m, day, 9, 30, 0, 0, time.UTC) }
	assert.Equal(t, d(2026, 12, 16), addMonth(d(2026, 11, 16)))
	assert.Equal(t, d(2027, 1, 31), addMonth(d(2026, 12, 31)), "year rollover")
	assert.Equal(t, d(2027, 2, 28), addMonth(d(2027, 1, 31)), "not March 3")
	assert.Equal(t, d(2028, 2, 29), addMonth(d(2028, 1, 31)), "leap year")
	assert.Equal(t, d(2026, 6, 30), addMonth(d(2026, 5, 31)))
}

// --- Create order (US-10, E3) ---

func TestCreateOrder_NoActive_CreatesPendingNewOrderAtPlanPrice(t *testing.T) {
	f := newFixture()
	f.active(false)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 49000}, nil)
	f.subs.On("CreateOrder", mock.MatchedBy(func(o *models.SubscriptionOrder) bool {
		return o.UserID == "user-1" && o.PlanID == "plan-1" && o.Price == 49000 && o.OrderType == models.OrderTypeNew &&
			o.Status == models.OrderPending && o.IdempotencyKey == "k1"
	})).Return(nil)
	got, err := f.svc.CreateOrder("user-1", "k1", 49000)
	require.NoError(t, err)
	assert.Equal(t, 49000, got.Price)
}

func TestCreateOrder_AlreadySubscribed_ReturnsSubscriptionActive(t *testing.T) {
	f := newFixture()
	f.active(true)
	_, err := f.svc.CreateOrder("user-1", "k1", 49000)
	assert.Equal(t, respConst.SUBSCRIPTION_ACTIVE, code(err))
}

func TestCreateOrder_NoPlan_ReturnsPlanNotFound(t *testing.T) {
	f := newFixture()
	f.active(false)
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.CreateOrder("user-1", "k1", 49000)
	assert.Equal(t, respConst.PLAN_NOT_FOUND, code(err))
}

func TestCreateOrder_ReusedKey_ReturnsDuplicateRequest(t *testing.T) {
	f := newFixture()
	f.active(false)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 49000}, nil)
	f.subs.On("CreateOrder", mock.Anything).Return(sql.ErrDuplicate)
	_, err := f.svc.CreateOrder("user-1", "k1", 49000)
	assert.Equal(t, respConst.DUPLICATE_REQUEST, code(err))
}

// --- Pay (US-10) ---

func TestPayOrder_Success_ActivatesForOneMonthAtOrderPrice(t *testing.T) {
	f := newFixture()
	f.subs.On("GetOrder", "so-1", "user-1").Return(pendingOrder(), nil)
	f.active(false)
	f.plan(49000)
	var sub *models.Subscription
	f.subs.On("Activate", mock.Anything, mock.Anything, now).Run(func(a mock.Arguments) { sub = a.Get(1).(*models.Subscription) }).Return(nil)

	got, err := f.svc.PayOrder("user-1", "so-1", "success")
	require.NoError(t, err)
	require.NotNil(t, sub)
	assert.Equal(t, models.SubActive, sub.Status)
	assert.Equal(t, 49000, sub.PricePaid, "price from the order snapshot, not the current plan")
	assert.Equal(t, now, sub.StartedAt)
	assert.Equal(t, now.AddDate(0, 1, 0), sub.ExpiresAt)
	assert.Equal(t, models.SubActive, got.Status)
	require.NotNil(t, got.Order)
	assert.Equal(t, models.OrderPaid, got.Order.Status)
}

func TestPayOrder_Failed_NoSubscriptionCreated(t *testing.T) {
	f := newFixture()
	f.subs.On("GetOrder", "so-1", "user-1").Return(pendingOrder(), nil)
	f.active(false)
	f.plan(49000)
	f.subs.On("MarkOrderFailed", "so-1", now).Return(nil)
	_, err := f.svc.PayOrder("user-1", "so-1", "failed")
	assert.Equal(t, respConst.PAYMENT_FAILED, code(err))
	f.subs.AssertNotCalled(t, "Activate", mock.Anything, mock.Anything, mock.Anything)
}

func TestPayOrder_NotPending_ReturnsDuplicateRequest(t *testing.T) {
	f := newFixture()
	o := pendingOrder()
	o.Status = models.OrderPaid
	f.subs.On("GetOrder", "so-1", "user-1").Return(o, nil)
	_, err := f.svc.PayOrder("user-1", "so-1", "success")
	assert.Equal(t, respConst.DUPLICATE_REQUEST, code(err))
}

func TestPayOrder_AlreadyActive_ReturnsSubscriptionActive(t *testing.T) {
	f := newFixture()
	f.subs.On("GetOrder", "so-1", "user-1").Return(pendingOrder(), nil)
	f.active(true)
	_, err := f.svc.PayOrder("user-1", "so-1", "success")
	assert.Equal(t, respConst.SUBSCRIPTION_ACTIVE, code(err))
}

func TestPayOrder_ConcurrentActivation_MapsRepoErrors(t *testing.T) {
	for repoErr, want := range map[error]string{
		sql.ErrDuplicate:    respConst.SUBSCRIPTION_ACTIVE,
		sql.ErrStateChanged: respConst.DUPLICATE_REQUEST,
		errors.New("down"):  respConst.GENERAL_ERROR,
	} {
		f := newFixture()
		f.subs.On("GetOrder", "so-1", "user-1").Return(pendingOrder(), nil)
		f.active(false)
		f.plan(49000)
		f.subs.On("Activate", mock.Anything, mock.Anything, now).Return(repoErr)
		_, err := f.svc.PayOrder("user-1", "so-1", "success")
		assert.Equal(t, want, code(err), repoErr.Error())
	}
}

func TestPayOrder_OtherUsersOrder_ReturnsOrderNotFound(t *testing.T) {
	f := newFixture()
	f.subs.On("GetOrder", "so-1", "user-1").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.PayOrder("user-1", "so-1", "success")
	assert.Equal(t, respConst.ORDER_NOT_FOUND, code(err))
}

func TestCreateOrder_StaleExpectedPrice_ReturnsPriceChanged(t *testing.T) {
	f := newFixture()
	f.active(false)
	f.plan(59000)
	_, err := f.svc.CreateOrder("user-1", "k1", 49000)
	assert.Equal(t, respConst.PRICE_CHANGED, code(err))
	f.subs.AssertNotCalled(t, "CreateOrder", mock.Anything)
}

func TestCreateOrder_AfterPriceChange_ChargesNewPrice(t *testing.T) {
	f := newFixture()
	f.active(false)
	f.plan(59000)
	f.subs.On("CreateOrder", mock.MatchedBy(func(o *models.SubscriptionOrder) bool { return o.Price == 59000 })).Return(nil)
	got, err := f.svc.CreateOrder("user-1", "k1", 59000)
	require.NoError(t, err)
	assert.Equal(t, 59000, got.Price)
}

func TestPayOrder_PlanPriceChangedSinceCheckout_ReturnsPriceChanged(t *testing.T) {
	for _, result := range []string{"success", "failed"} {
		f := newFixture()
		f.subs.On("GetOrder", "so-1", "user-1").Return(pendingOrder(), nil)
		f.active(false)
		f.plan(59000)
		_, err := f.svc.PayOrder("user-1", "so-1", result)
		assert.Equal(t, respConst.PRICE_CHANGED, code(err), result)
		f.subs.AssertNotCalled(t, "Activate", mock.Anything, mock.Anything, mock.Anything)
		f.subs.AssertNotCalled(t, "MarkOrderFailed", mock.Anything, mock.Anything)
	}
}

func TestPayOrder_PlanLookupFails_ReturnsGeneralError(t *testing.T) {
	f := newFixture()
	f.subs.On("GetOrder", "so-1", "user-1").Return(pendingOrder(), nil)
	f.active(false)
	f.subs.On("GetPlan").Return(nil, errors.New("db down"))
	_, err := f.svc.PayOrder("user-1", "so-1", "success")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- Cancel (US-11, E14) ---

func TestCancel_Active_CancelsImmediately(t *testing.T) {
	f := newFixture()
	f.active(true)
	f.subs.On("Cancel", "sub-1", now).Return(nil)
	got, err := f.svc.Cancel("user-1")
	require.NoError(t, err)
	assert.Equal(t, models.SubCancelled, got.Status)
	require.NotNil(t, got.CancelledAt)
	assert.Equal(t, now, *got.CancelledAt)
}

func TestCancel_NoneActive_ReturnsSubscriptionNotActive(t *testing.T) {
	f := newFixture()
	f.active(false)
	_, err := f.svc.Cancel("user-1")
	assert.Equal(t, respConst.SUBSCRIPTION_NOT_ACTIVE, code(err))
}

func TestCancel_ConcurrentlyCancelled_ReturnsSubscriptionNotActive(t *testing.T) {
	f := newFixture()
	f.active(true)
	f.subs.On("Cancel", "sub-1", now).Return(sql.ErrStateChanged)
	_, err := f.svc.Cancel("user-1")
	assert.Equal(t, respConst.SUBSCRIPTION_NOT_ACTIVE, code(err))
}

// --- History (US-13) ---

func TestHistory_MapsPeriodsWithTheirOrders(t *testing.T) {
	f := newFixture()
	oid, otype, ostatus, oprice := "so-2", models.OrderTypeRenewal, models.OrderPaid, 59000
	f.subs.On("ListHistory", "user-1").Return([]models.SubscriptionHistoryRow{
		{Subscription: models.Subscription{ID: "s2", Status: models.SubActive, PricePaid: 59000}, OrderID: &oid, OrderType: &otype, OrderStatus: &ostatus, OrderPrice: &oprice},
		{Subscription: models.Subscription{ID: "s1", Status: models.SubExpired, PricePaid: 49000}},
	}, nil)
	got, err := f.svc.History("user-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.NotNil(t, got[0].Order)
	assert.Equal(t, models.OrderTypeRenewal, got[0].Order.OrderType)
	assert.Nil(t, got[1].Order)
}

func TestHistory_None_ReturnsEmptySlice(t *testing.T) {
	f := newFixture()
	f.subs.On("ListHistory", "user-1").Return(nil, nil)
	got, err := f.svc.History("user-1")
	require.NoError(t, err)
	assert.NotNil(t, got)
}

// --- Auto-renew (US-10, US-13) ---

func TestRenewDue_RenewsAtCurrentPlanPriceWithContinuousPeriod(t *testing.T) {
	f := newFixture()
	expired := now.Add(-time.Hour)
	old := models.Subscription{ID: "s1", UserID: "user-1", PlanID: "plan-1", Status: models.SubActive, PricePaid: 49000, StartedAt: expired.AddDate(0, -1, 0), ExpiresAt: expired}
	f.subs.On("ListDue", now, renewBatch).Return([]models.Subscription{old}, nil)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 59000}, nil)
	f.subs.On("Renew", mock.MatchedBy(func(o *models.Subscription) bool { return o.ID == "s1" }),
		mock.MatchedBy(func(n *models.Subscription) bool {
			return n.Status == models.SubActive && n.PricePaid == 59000 && n.StartedAt.Equal(expired) &&
				n.ExpiresAt.Equal(addMonth(expired)) && n.RenewedFromID != nil && *n.RenewedFromID == "s1"
		}),
		mock.MatchedBy(func(o *models.SubscriptionOrder) bool {
			return o.OrderType == models.OrderTypeRenewal && o.Status == models.OrderPaid && o.Price == 59000 &&
				o.IdempotencyKey == "renewal:s1" && o.PaidAt != nil
		})).Return(nil)

	n, err := f.svc.RenewDue()
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	f.subs.AssertExpectations(t)
}

func TestRenewDue_CancelledMeanwhileOrAlreadyRenewed_Skipped(t *testing.T) {
	f := newFixture()
	f.subs.On("ListDue", now, renewBatch).Return([]models.Subscription{{ID: "a", ExpiresAt: now}, {ID: "b", ExpiresAt: now}}, nil)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 49000}, nil)
	f.subs.On("Renew", mock.MatchedBy(func(o *models.Subscription) bool { return o.ID == "a" }), mock.Anything, mock.Anything).Return(sql.ErrStateChanged)
	f.subs.On("Renew", mock.MatchedBy(func(o *models.Subscription) bool { return o.ID == "b" }), mock.Anything, mock.Anything).Return(sql.ErrDuplicate)
	n, err := f.svc.RenewDue()
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestRenewDue_OneFails_OthersStillRenewed(t *testing.T) {
	f := newFixture()
	f.subs.On("ListDue", now, renewBatch).Return([]models.Subscription{{ID: "a", ExpiresAt: now}, {ID: "b", ExpiresAt: now}}, nil)
	f.subs.On("GetPlan").Return(&models.SubscriptionPlan{ID: "plan-1", Price: 49000}, nil)
	f.subs.On("Renew", mock.MatchedBy(func(o *models.Subscription) bool { return o.ID == "a" }), mock.Anything, mock.Anything).Return(errors.New("db blip"))
	f.subs.On("Renew", mock.MatchedBy(func(o *models.Subscription) bool { return o.ID == "b" }), mock.Anything, mock.Anything).Return(nil)
	n, err := f.svc.RenewDue()
	assert.Error(t, err)
	assert.Equal(t, 1, n)
}

func TestRenewDue_NothingDue_SkipsPlanLookup(t *testing.T) {
	f := newFixture()
	f.subs.On("ListDue", now, renewBatch).Return(nil, nil)
	n, err := f.svc.RenewDue()
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	f.subs.AssertNotCalled(t, "GetPlan")
}

func TestRenewDue_NoPlan_ReturnsError(t *testing.T) {
	f := newFixture()
	f.subs.On("ListDue", now, renewBatch).Return([]models.Subscription{{ID: "a"}}, nil)
	f.subs.On("GetPlan").Return(nil, gorm.ErrRecordNotFound)
	_, err := f.svc.RenewDue()
	assert.Error(t, err)
}

// --- Scheduler ---

type countingRenewer struct{ calls atomic.Int32 }

func (c *countingRenewer) RenewDue() (int, error) { c.calls.Add(1); return 0, nil }

func TestStartRenewalJob_RunsImmediatelyThenOnTickUntilCancelled(t *testing.T) {
	r := &countingRenewer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := StartRenewalJob(ctx, r, 10*time.Millisecond, nil)
	assert.Eventually(t, func() bool { return r.calls.Load() >= 3 }, time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("job did not stop on cancel")
	}
}
