package migrations

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// storybook_library_app applies every *.sql file here in filename order (ADR-010),
// so the apply list must be exactly the .sql files, in sorted order.
func TestOrdered_MatchesSortedSQLFiles(t *testing.T) {
	files, err := filepath.Glob("*.sql")
	require.NoError(t, err)
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, strings.TrimSuffix(f, ".sql"))
	}
	assert.Equal(t, names, Ordered(), "every .sql file listed once")
	assert.True(t, sort.StringsAreSorted(Ordered()), "apply order must equal filename order")
}
