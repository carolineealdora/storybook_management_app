package migrations

import (
	"errors"
	"strings"

	"codebase/config"
	sql "codebase/pkg/orm"

	"golang.org/x/crypto/bcrypt"
)

// Seed creates the mocked parent and admin accounts (PRD: no sign-up) and the default subscription plan.
// Values come from SEED_* env vars only. Safe to run repeatedly: existing rows are left untouched.
func Seed(db sql.ORM, cfg *config.SystemConfig) error {
	if cfg.SeedParentEmail == "" || cfg.SeedParentPassword == "" || cfg.SeedAdminEmail == "" || cfg.SeedAdminPassword == "" {
		return errors.New("seed: SEED_PARENT_EMAIL, SEED_PARENT_PASSWORD, SEED_ADMIN_EMAIL and SEED_ADMIN_PASSWORD are required")
	}
	parentHash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedParentPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	adminHash, err := bcrypt.GenerateFromPassword([]byte(cfg.SeedAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return sql.WithTx(db, func(tx sql.ORM) error {
		parentEmail := strings.ToLower(strings.TrimSpace(cfg.SeedParentEmail))
		if err := tx.Exec(`WITH u AS (
				INSERT INTO users (name) SELECT ? WHERE NOT EXISTS (SELECT 1 FROM credentials WHERE email = ?) RETURNING id)
			INSERT INTO credentials (email, password_hash, account_type, user_id) SELECT ?, ?, 'parent', id FROM u`,
			cfg.SeedParentName, parentEmail, parentEmail, string(parentHash)); err != nil {
			return err
		}
		adminEmail := strings.ToLower(strings.TrimSpace(cfg.SeedAdminEmail))
		if err := tx.Exec(`WITH a AS (
				INSERT INTO admins (name) SELECT ? WHERE NOT EXISTS (SELECT 1 FROM credentials WHERE email = ?) RETURNING id)
			INSERT INTO credentials (email, password_hash, account_type, admin_id) SELECT ?, ?, 'admin', id FROM a`,
			cfg.SeedAdminName, adminEmail, adminEmail, string(adminHash)); err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO subscription_plans (price, benefits) SELECT ?, ? WHERE NOT EXISTS (SELECT 1 FROM subscription_plans)`,
			cfg.SeedPlanPrice, cfg.SeedPlanBenefits)
	})
}
