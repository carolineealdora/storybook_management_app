//go:build integration

package testdb

import (
	"testing"

	"codebase/config"
	"codebase/migrations"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	userID  = "11111111-1111-1111-1111-111111111111"
	planID  = "22222222-2222-2222-2222-222222222222"
	bookID  = "33333333-3333-3333-3333-333333333333"
	orderID = "44444444-4444-4444-4444-444444444444"
)

func TestSchema_SecondActiveSubscription_Rejected(t *testing.T) {
	db := Open(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, name) VALUES (?, 'P')`, userID))
	require.NoError(t, db.Exec(`INSERT INTO subscription_plans (id, price, benefits) VALUES (?, 49000, 'all')`, planID))
	insert := `INSERT INTO subscriptions (user_id, plan_id, price_paid, status, started_at, expires_at)
	           VALUES (?, ?, 49000, ?, NOW(), NOW() + INTERVAL '1 month')`
	require.NoError(t, db.Exec(insert, userID, planID, "active"))
	require.NoError(t, db.Exec(insert, userID, planID, "expired"), "history rows are allowed")
	assert.Error(t, db.Exec(insert, userID, planID, "active"), "only one active subscription per user")
}

func TestSchema_DuplicatePurchase_Rejected(t *testing.T) {
	db := Open(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, name) VALUES (?, 'P')`, userID))
	require.NoError(t, db.Exec(`INSERT INTO books (id, title, author, description, age_type, age_min, price, status, epub_file_name, epub_size_bytes, epub_content)
		VALUES (?, 'T', 'A', 'D', 'plus', 3, 15000, 'published', 'b.epub', 1, '\x00')`, bookID))
	require.NoError(t, db.Exec(`INSERT INTO book_orders (id, user_id, book_id, price, status, idempotency_key) VALUES (?, ?, ?, 15000, 'paid', 'k1')`, orderID, userID, bookID))
	insert := `INSERT INTO purchases (user_id, book_id, book_order_id, price_paid) VALUES (?, ?, ?, 15000)`
	require.NoError(t, db.Exec(insert, userID, bookID, orderID))
	assert.Error(t, db.Exec(insert, userID, bookID, orderID))
}

func TestSchema_DuplicateIdempotencyKeyPerUser_Rejected(t *testing.T) {
	db := Open(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, name) VALUES (?, 'P')`, userID))
	require.NoError(t, db.Exec(`INSERT INTO books (id, title, author, description, age_type, age_min, price, status, epub_file_name, epub_size_bytes, epub_content)
		VALUES (?, 'T', 'A', 'D', 'plus', 3, 15000, 'published', 'b.epub', 1, '\x00')`, bookID))
	insert := `INSERT INTO book_orders (user_id, book_id, price, status, idempotency_key) VALUES (?, ?, 15000, 'pending', 'same')`
	require.NoError(t, db.Exec(insert, userID, bookID))
	assert.Error(t, db.Exec(insert, userID, bookID))
}

func TestSchema_InvalidBookAgeRange_Rejected(t *testing.T) {
	db := Open(t)
	insert := `INSERT INTO books (title, author, description, age_type, age_min, age_max, price, status, epub_file_name, epub_size_bytes, epub_content)
		VALUES ('T', 'A', 'D', ?, ?, ?, 0, 'draft', 'b.epub', 1, '\x00')`
	assert.Error(t, db.Exec(insert, "range", 3, 3), "min must be lower than max")
	assert.Error(t, db.Exec(insert, "range", 3, nil), "range needs max")
	assert.Error(t, db.Exec(insert, "plus", 3, 5), "plus has no max")
	assert.NoError(t, db.Exec(insert, "range", 1, 3))
	assert.NoError(t, db.Exec(insert, "plus", 3, nil))
}

func TestSchema_NegativeBookPrice_Rejected(t *testing.T) {
	db := Open(t)
	assert.Error(t, db.Exec(`INSERT INTO books (title, author, description, age_type, age_min, price, status, epub_file_name, epub_size_bytes, epub_content)
		VALUES ('T', 'A', 'D', 'plus', 3, -1, 'draft', 'b.epub', 1, '\x00')`))
}

func TestSchema_CredentialOwner_MustMatchAccountType(t *testing.T) {
	db := Open(t)
	require.NoError(t, db.Exec(`INSERT INTO users (id, name) VALUES (?, 'P')`, userID))
	assert.Error(t, db.Exec(`INSERT INTO credentials (email, password_hash, account_type, user_id) VALUES ('a@x.io', 'h', 'admin', ?)`, userID))
	assert.NoError(t, db.Exec(`INSERT INTO credentials (email, password_hash, account_type, user_id) VALUES ('p@x.io', 'h', 'parent', ?)`, userID))
}

func TestSeed_RunTwice_IsIdempotent(t *testing.T) {
	db := Open(t)
	cfg := &config.SystemConfig{
		SeedParentName: "P", SeedParentEmail: "Parent@Example.com", SeedParentPassword: "parent-pass-123",
		SeedAdminName: "A", SeedAdminEmail: "admin@example.com", SeedAdminPassword: "admin-pass-123",
		SeedPlanPrice: 49000, SeedPlanBenefits: "all",
	}
	require.NoError(t, migrations.Seed(db, cfg))
	require.NoError(t, migrations.Seed(db, cfg))

	var counts []int64
	require.NoError(t, db.RawSql(`SELECT (SELECT COUNT(*) FROM credentials) UNION ALL SELECT (SELECT COUNT(*) FROM subscription_plans)`, &counts))
	assert.Equal(t, []int64{2, 1}, counts)

	var emails []string
	require.NoError(t, db.RawSql(`SELECT email FROM credentials ORDER BY email`, &emails))
	assert.Equal(t, []string{"admin@example.com", "parent@example.com"}, emails, "emails stored lowercase")
}

func TestSeed_MissingPassword_ReturnsError(t *testing.T) {
	db := Open(t)
	assert.Error(t, migrations.Seed(db, &config.SystemConfig{SeedParentEmail: "p@example.com", SeedAdminEmail: "a@example.com"}))
}
