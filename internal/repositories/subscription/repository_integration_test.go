//go:build integration

package subscription

import (
	"testing"
	"time"

	"codebase/config"
	"codebase/internal/repositories/testdb"
	"codebase/migrations"
	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type env struct {
	repo   *Repo
	db     sql.ORM
	userID string
	planID string
}

func setup(t *testing.T) env {
	db := testdb.Open(t)
	require.NoError(t, migrations.Seed(db, &config.SystemConfig{
		SeedParentName: "P", SeedParentEmail: "parent@example.com", SeedParentPassword: "pw",
		SeedAdminName: "A", SeedAdminEmail: "admin@example.com", SeedAdminPassword: "pw",
		SeedPlanPrice: 49000, SeedPlanBenefits: "all",
	}))
	var ids []string
	require.NoError(t, db.RawSql(`SELECT user_id::text FROM credentials WHERE user_id IS NOT NULL`, &ids))
	var plans []string
	require.NoError(t, db.RawSql(`SELECT id::text FROM subscription_plans`, &plans))
	return env{repo: &Repo{db: db}, db: db, userID: ids[0], planID: plans[0]}
}

func (e env) insertSub(t *testing.T, status string, started time.Time) {
	require.NoError(t, e.db.Exec(`INSERT INTO subscriptions (user_id, plan_id, price_paid, status, started_at, expires_at) VALUES (?, ?, 49000, ?, ?, ?)`,
		e.userID, e.planID, status, started, started.AddDate(0, 1, 0)))
}

func TestGetActive_OnlyExpiredAndCancelled_ReturnsNotFound(t *testing.T) {
	e := setup(t)
	e.insertSub(t, models.SubExpired, time.Now().AddDate(0, -2, 0))
	e.insertSub(t, models.SubCancelled, time.Now().AddDate(0, -1, 0))
	_, err := e.repo.GetActive(e.userID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestGetActive_ActivePresent_ReturnsIt(t *testing.T) {
	e := setup(t)
	e.insertSub(t, models.SubExpired, time.Now().AddDate(0, -2, 0))
	e.insertSub(t, models.SubActive, time.Now())
	s, err := e.repo.GetActive(e.userID)
	require.NoError(t, err)
	assert.Equal(t, models.SubActive, s.Status)
}

func TestPlan_GetAndSave_UpdatesInPlace(t *testing.T) {
	e := setup(t)
	p, err := e.repo.GetPlan()
	require.NoError(t, err)
	assert.Equal(t, 49000, p.Price)

	admin := e.adminID(t)
	p.Price, p.Benefits, p.UpdatedAt, p.UpdatedBy = 59000, "more", time.Now(), &admin
	require.NoError(t, e.repo.SavePlan(p))
	again, err := e.repo.GetPlan()
	require.NoError(t, err)
	assert.Equal(t, p.ID, again.ID)
	assert.Equal(t, 59000, again.Price)
	assert.Equal(t, "monthly", again.Code, "migration default")
	assert.Equal(t, "Monthly Subscription", again.Name)
	require.NotNil(t, again.UpdatedBy)
	assert.Equal(t, admin, *again.UpdatedBy)
}

func (e env) adminID(t *testing.T) string {
	var ids []string
	require.NoError(t, e.db.RawSql(`SELECT admin_id::text FROM credentials WHERE admin_id IS NOT NULL`, &ids))
	return ids[0]
}

func TestSavePlan_PriceChanged_WritesHistoryRowsNewestFirst(t *testing.T) {
	e := setup(t)
	admin := e.adminID(t)
	p, err := e.repo.GetPlan()
	require.NoError(t, err)
	t0 := time.Now().UTC().Truncate(time.Second)
	p.Price, p.UpdatedAt, p.UpdatedBy = 59000, t0, &admin
	require.NoError(t, e.repo.SavePlan(p))
	p.Price, p.UpdatedAt = 69000, t0.Add(time.Minute)
	require.NoError(t, e.repo.SavePlan(p))

	h, err := e.repo.ListPriceHistory(p.ID)
	require.NoError(t, err)
	require.Len(t, h, 2)
	assert.Equal(t, []int{59000, 69000}, []int{h[0].OldPrice, h[0].NewPrice}, "newest first")
	assert.Equal(t, []int{49000, 59000}, []int{h[1].OldPrice, h[1].NewPrice})
	assert.Equal(t, admin, h[0].ChangedBy)
	assert.True(t, h[0].ChangedAt.Equal(t0.Add(time.Minute)))
}

func TestSavePlan_SamePrice_NoHistoryRow(t *testing.T) {
	e := setup(t)
	admin := e.adminID(t)
	p, err := e.repo.GetPlan()
	require.NoError(t, err)
	p.Benefits, p.UpdatedAt, p.UpdatedBy = "new benefits", time.Now(), &admin
	require.NoError(t, e.repo.SavePlan(p))
	h, err := e.repo.ListPriceHistory(p.ID)
	require.NoError(t, err)
	assert.Empty(t, h)
	again, err := e.repo.GetPlan()
	require.NoError(t, err)
	assert.Equal(t, "new benefits", again.Benefits)
}

func TestSavePlan_PriceChangedWithoutAdmin_FailsAndRollsBack(t *testing.T) {
	e := setup(t)
	p, err := e.repo.GetPlan()
	require.NoError(t, err)
	p.Price, p.UpdatedAt = 59000, time.Now()
	assert.Error(t, e.repo.SavePlan(p), "the audit row needs changed_by")
	again, err := e.repo.GetPlan()
	require.NoError(t, err)
	assert.Equal(t, 49000, again.Price, "price update rolled back with the failed audit insert")
}

func TestPlan_NoneConfigured_SaveCreates(t *testing.T) {
	db := testdb.Open(t)
	r := &Repo{db: db}
	_, err := r.GetPlan()
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	p := &models.SubscriptionPlan{Code: "monthly", Name: "Monthly Subscription", Price: 1000, Benefits: "x", Duration: "1 month", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, r.SavePlan(p))
	assert.NotEmpty(t, p.ID)
}

func (e env) newOrder(t *testing.T, key string) *models.SubscriptionOrder {
	o := &models.SubscriptionOrder{UserID: e.userID, PlanID: e.planID, OrderType: models.OrderTypeNew, Price: 49000,
		Status: models.OrderPending, IdempotencyKey: key, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, e.repo.CreateOrder(o))
	return o
}

func (e env) newSub(start time.Time) *models.Subscription {
	return &models.Subscription{UserID: e.userID, PlanID: e.planID, PricePaid: 49000, Status: models.SubActive,
		StartedAt: start, ExpiresAt: start.AddDate(0, 1, 0), CreatedAt: time.Now(), UpdatedAt: time.Now()}
}

func TestCreateOrder_ReusedKey_ErrDuplicate(t *testing.T) {
	e := setup(t)
	e.newOrder(t, "k1")
	err := e.repo.CreateOrder(&models.SubscriptionOrder{UserID: e.userID, PlanID: e.planID, OrderType: models.OrderTypeNew, Price: 1, Status: models.OrderPending, IdempotencyKey: "k1"})
	assert.ErrorIs(t, err, sql.ErrDuplicate)
}

func TestActivate_PaysOrderAndLinksSubscription(t *testing.T) {
	e := setup(t)
	o := e.newOrder(t, "k1")
	sub := e.newSub(time.Now())
	require.NoError(t, e.repo.Activate(o, sub, time.Now()))

	got, err := e.repo.GetOrder(o.ID, e.userID)
	require.NoError(t, err)
	assert.Equal(t, models.OrderPaid, got.Status)
	require.NotNil(t, got.SubscriptionID)
	assert.Equal(t, sub.ID, *got.SubscriptionID)

	assert.ErrorIs(t, e.repo.Activate(o, e.newSub(time.Now()), time.Now()), sql.ErrDuplicate, "second active period rejected by DB")
}

func TestActivate_OrderNotPending_ErrStateChangedAndNoSubscription(t *testing.T) {
	e := setup(t)
	o := e.newOrder(t, "k1")
	require.NoError(t, e.repo.MarkOrderFailed(o.ID, time.Now()))
	assert.ErrorIs(t, e.repo.Activate(o, e.newSub(time.Now()), time.Now()), sql.ErrStateChanged)
	_, err := e.repo.GetActive(e.userID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound, "subscription insert rolled back")
	assert.ErrorIs(t, e.repo.MarkOrderFailed(o.ID, time.Now()), sql.ErrStateChanged)
}

func TestGetOrder_OtherUser_NotFound(t *testing.T) {
	e := setup(t)
	o := e.newOrder(t, "k1")
	_, err := e.repo.GetOrder(o.ID, "99999999-9999-9999-9999-999999999999")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestCancel_ActiveOnce(t *testing.T) {
	e := setup(t)
	sub := e.newSub(time.Now())
	require.NoError(t, e.repo.Activate(e.newOrder(t, "k1"), sub, time.Now()))
	require.NoError(t, e.repo.Cancel(sub.ID, time.Now()))
	assert.ErrorIs(t, e.repo.Cancel(sub.ID, time.Now()), sql.ErrStateChanged)
	_, err := e.repo.GetActive(e.userID)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRenew_ExpiresOldStartsNextWithPaidRenewalOrder(t *testing.T) {
	e := setup(t)
	start := time.Now().UTC().AddDate(0, -1, -1).Truncate(time.Second)
	old := e.newSub(start)
	require.NoError(t, e.repo.Activate(e.newOrder(t, "k1"), old, start))

	due, err := e.repo.ListDue(time.Now(), 10)
	require.NoError(t, err)
	require.Len(t, due, 1)

	oldID := old.ID
	next := e.newSub(old.ExpiresAt)
	next.RenewedFromID = &oldID
	paidAt := time.Now()
	order := &models.SubscriptionOrder{UserID: e.userID, PlanID: e.planID, OrderType: models.OrderTypeRenewal, Price: 59000,
		Status: models.OrderPaid, IdempotencyKey: "renewal:" + old.ID, PaidAt: &paidAt, CreatedAt: paidAt, UpdatedAt: paidAt}
	require.NoError(t, e.repo.Renew(&due[0], next, order))

	active, err := e.repo.GetActive(e.userID)
	require.NoError(t, err)
	assert.Equal(t, next.ID, active.ID)

	again := e.newSub(old.ExpiresAt)
	assert.ErrorIs(t, e.repo.Renew(&due[0], again, &models.SubscriptionOrder{UserID: e.userID, PlanID: e.planID, OrderType: models.OrderTypeRenewal,
		Status: models.OrderPaid, IdempotencyKey: "renewal:" + old.ID}), sql.ErrStateChanged, "double renewal blocked")

	hist, err := e.repo.ListHistory(e.userID)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.Equal(t, models.SubActive, hist[0].Status)
	assert.Equal(t, models.OrderTypeRenewal, *hist[0].OrderType)
	assert.Equal(t, 59000, *hist[0].OrderPrice)
	assert.Equal(t, models.SubExpired, hist[1].Status, "previous period marked expired")
	assert.Equal(t, models.OrderTypeNew, *hist[1].OrderType)
}

func TestListDue_SkipsCancelledAndFuture(t *testing.T) {
	e := setup(t)
	past := time.Now().AddDate(0, -2, 0)
	cancelled := e.newSub(past)
	require.NoError(t, e.repo.Activate(e.newOrder(t, "k1"), cancelled, past))
	require.NoError(t, e.repo.Cancel(cancelled.ID, time.Now()))
	require.NoError(t, e.repo.Activate(e.newOrder(t, "k2"), e.newSub(time.Now()), time.Now()))
	due, err := e.repo.ListDue(time.Now(), 10)
	require.NoError(t, err)
	assert.Empty(t, due)
}
