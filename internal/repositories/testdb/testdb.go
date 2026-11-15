//go:build integration

// Package testdb gives integration tests a freshly migrated Postgres schema.
// Run with `make test-integration` (packages run one at a time: they share one database).
package testdb

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"codebase/migrations"
	sql "codebase/pkg/orm"
	"codebase/pkg/orm/postgres"
)

const defaultDSN = "host=localhost user=postgres password=postgres dbname=storybook_test port=5433 sslmode=disable TimeZone=UTC"

// Open resets the public schema, applies every migration, and returns a connection.
func Open(t *testing.T) sql.ORM {
	t.Helper()
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		dsn = defaultDSN
	}
	db, err := postgres.NewPostgresDSN(dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Logf("close test db: %v", err)
		}
	})
	if err := db.Exec("DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;"); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	for _, name := range migrations.Ordered() {
		if err := migrations.LoadSQLScript(filepath.Join(dir, name+".sql"))(db); err != nil {
			t.Fatalf("migration %s: %v", name, err)
		}
	}
	return db
}
