//go:build integration

package account

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

func seeded(t *testing.T) (*Repo, sql.ORM) {
	db := testdb.Open(t)
	require.NoError(t, migrations.Seed(db, &config.SystemConfig{
		SeedParentName: "P", SeedParentEmail: "parent@example.com", SeedParentPassword: "pw-parent",
		SeedAdminName: "A", SeedAdminEmail: "admin@example.com", SeedAdminPassword: "pw-admin",
		SeedPlanPrice: 49000, SeedPlanBenefits: "all",
	}))
	return &Repo{db: db}, db
}

func TestGetCredentialByEmail_Seeded_ReturnsParentWithUserID(t *testing.T) {
	r, _ := seeded(t)
	c, err := r.GetCredentialByEmail("parent@example.com")
	require.NoError(t, err)
	assert.Equal(t, models.AccountParent, c.AccountType)
	require.NotNil(t, c.UserID)
	assert.Nil(t, c.AdminID)

	byID, err := r.GetCredentialByID(c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.Email, byID.Email)
}

func TestGetCredentialByEmail_Unknown_ReturnsRecordNotFound(t *testing.T) {
	r, _ := seeded(t)
	_, err := r.GetCredentialByEmail("nobody@example.com")
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestRefreshToken_CreateGetRevoke_OnlyForOwner(t *testing.T) {
	r, _ := seeded(t)
	parent, err := r.GetCredentialByEmail("parent@example.com")
	require.NoError(t, err)
	admin, err := r.GetCredentialByEmail("admin@example.com")
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Microsecond)

	require.NoError(t, r.CreateRefreshToken(&models.RefreshToken{CredentialID: parent.ID, TokenHash: "h1", ExpiresAt: now.Add(time.Hour), CreatedAt: now}))
	require.NoError(t, r.TouchLastLogin(parent.ID, now))

	require.NoError(t, r.RevokeRefreshToken("h1", admin.ID, now), "other account: no-op")
	got, err := r.GetRefreshToken("h1")
	require.NoError(t, err)
	assert.Nil(t, got.RevokedAt)

	require.NoError(t, r.RevokeRefreshToken("h1", parent.ID, now))
	got, err = r.GetRefreshToken("h1")
	require.NoError(t, err)
	require.NotNil(t, got.RevokedAt)
	assert.WithinDuration(t, now, *got.RevokedAt, time.Second)
}
