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
