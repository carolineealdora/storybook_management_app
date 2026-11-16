package postgres

import (
	"fmt"
	"os"

	sql "codebase/pkg/orm"
)

type SQLScript func(db sql.ORM) error

type SQLMigration struct {
	Up SQLScript
}

type Migrator interface {
	Init() error
	Up() error
	Down(name string) error
	Create(name string) error
}

type migrationRecord struct {
	ID string `gorm:"column:id"`
}

type migratorImpl struct {
	orm       sql.ORM
	tableName string
	path      string
	scripts   map[string]SQLMigration
	order     []string
}

// NewMigrator applies scripts in the given order (map iteration order is random, so it is never used for ordering).
func NewMigrator(orm sql.ORM, tableName, path string, order []string, scripts map[string]SQLMigration) Migrator {
	return &migratorImpl{orm: orm, tableName: tableName, path: path, scripts: scripts, order: order}
}

func (m *migratorImpl) Init() error {
	return m.orm.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS "%s" (
			id VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, m.tableName))
}

func (m *migratorImpl) Up() error {
	var records []migrationRecord
	if err := m.orm.RawSql(fmt.Sprintf(`SELECT id FROM "%s"`, m.tableName), &records); err != nil {
		return err
	}

	applied := make(map[string]bool, len(records))
	for _, r := range records {
		applied[r.ID] = true
	}

	for _, name := range m.order {
		if applied[name] {
			continue
		}
		script, ok := m.scripts[name]
		if !ok {
			continue
		}
		if err := script.Up(m.orm); err != nil {
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		if err := m.orm.Exec(fmt.Sprintf(`INSERT INTO "%s" (id) VALUES (?)`, m.tableName), name); err != nil {
			return err
		}
	}
	return nil
}

func (m *migratorImpl) Down(name string) error {
	if name == "" {
		return m.orm.Exec(fmt.Sprintf(`TRUNCATE TABLE "%s"`, m.tableName))
	}
	return m.orm.Exec(fmt.Sprintf(`DELETE FROM "%s" WHERE id = ?`, m.tableName), name)
}

func (m *migratorImpl) Create(name string) error {
	path := fmt.Sprintf("%s%s.sql", m.path, name)
	return os.WriteFile(path, []byte(""), 0644)
}
