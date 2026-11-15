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
