package migrations

import (
	"os"

	"codebase/models"
	"codebase/pkg/migrator/postgres"
	sql "codebase/pkg/orm"
)

// migrations is the apply order. Append new files; never reorder or rename applied ones.
var migrations = []string{}

func NewMigration(holder models.SharedHolder) postgres.Migrator {
	scripts := map[string]postgres.SQLMigration{}
	for _, migration := range migrations {
		scripts[migration] = postgres.SQLMigration{
			Up: LoadSQLScript("./migrations/" + migration + ".sql"),
		}
	}

	migrator := postgres.NewMigrator(holder.Sql, "migrations", "./migrations/", migrations, scripts)

	if err := migrator.Init(); err != nil {
		holder.Log.Error(err)
	}

	return migrator
}

func LoadSQLScript(path string) postgres.SQLScript {
	return func(db sql.ORM) error {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return db.Exec(string(content))
	}
}
