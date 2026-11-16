package postgres

import (
	"testing"

	sql "codebase/pkg/orm"
	sql_mock "codebase/test/mock/sql"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestUp_ManyMigrations_AppliedInDeclaredOrder(t *testing.T) {
	orm := new(sql_mock.ORMWrapperMock)
	orm.On("RawSql", mock.Anything, mock.Anything).Return(nil)
	orm.On("Exec", mock.Anything, mock.Anything).Return(nil)

	var ran []string
	order := []string{"001_a", "002_b", "003_c", "004_d", "005_e", "006_f", "007_g", "008_h"}
	scripts := map[string]SQLMigration{}
	for _, name := range order {
		name := name
		scripts[name] = SQLMigration{Up: func(sql.ORM) error { ran = append(ran, name); return nil }}
	}

	require.NoError(t, NewMigrator(orm, "migrations", "./", order, scripts).Up())
	assert.Equal(t, order, ran)
}
