package records

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// GET /api/v1/audit/records and POST /api/v1/audit/records:search read
// `sortField`/`sortOrder` and forward them all the way to the repository, where
// they used to be ignored: both statements ended with the literal
// `ORDER BY created_at DESC`. The endpoint therefore advertised a capability it
// did not have, and no test noticed because the parameter *was* read — it just
// never reached the SQL (开发规范手册 4.8.2「SQL 注入红线」；开放项注册表 OPEN-API-05).
//
// These cases pin the two halves of the fix: the statement has a slot for the
// clause, and the clause is built from the whitelist rather than from the
// request.
//
// The slot is a named token, not a `%s` verb: these statements carry literal `%`
// for the ILIKE wildcards, and `fmt.Sprintf` would consume each of those as a
// verb. An earlier version of this fix used Sprintf and the first `%s` it filled
// was the keyword pattern, leaving the ORDER BY slot as `%!s(MISSING)` — which is
// exactly the shape of silent breakage this round is about.
func TestRecordStatements_HaveAnOrderBySlot(t *testing.T) {
	for name, query := range map[string]string{
		"list":   queryListRecords,
		"search": querySearchRecords,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Contains(t, query, "{ORDER_BY}",
				"the statement must leave a slot for the ORDER BY clause; "+
					"a hardcoded clause is what made sortField a no-op")
			assert.NotContains(t, query, "ORDER BY created_at DESC",
				"the default order must come from the sort spec, not the statement")

			// And the slot must actually accept a clause.
			rendered := strings.Replace(query, "{ORDER_BY}", "ORDER BY created_at DESC, id ASC", 1)
			assert.Contains(t, rendered, "ORDER BY created_at DESC, id ASC")
			assert.NotContains(t, rendered, "{ORDER_BY}")
			// The ILIKE wildcards must survive the substitution: they are literal
			// percent signs, not format verbs.
			assert.Contains(t, rendered, "ILIKE '%' ||")
		})
	}
}

func TestRecordSortSpec_Contract(t *testing.T) {
	t.Run("the default order is time-descending", func(t *testing.T) {
		got, err := recordSortSpec.Resolve("", "", "created_at")
		require.NoError(t, err)
		assert.Equal(t, "ORDER BY created_at ASC, id ASC", got)
	})

	t.Run("every documented field resolves", func(t *testing.T) {
		for apiName, column := range recordSortSpec.Fields {
			got, err := recordSortSpec.Resolve(apiName, "desc", "created_at")
			require.NoError(t, err)
			assert.Equal(t, "ORDER BY "+column+" DESC, id ASC", got)
		}
	})

	t.Run("an unlisted field is refused", func(t *testing.T) {
		_, err := recordSortSpec.Resolve("metadata", "asc", "created_at")
		require.Error(t, err)

		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	})

	t.Run("the audit surface exposes no sensitive column", func(t *testing.T) {
		// actor_id is a UUID and organization_id is a tenant boundary; neither is
		// a field a client should be able to order the audit stream by name, and
		// neither is in the whitelist.
		for _, forbidden := range []string{"actor_id", "organization_id", "metadata"} {
			_, err := recordSortSpec.Resolve(forbidden, "asc", "created_at")
			assert.Error(t, err, "%s must not be sortable", forbidden)
		}
	})
}

// The rendered statement must still be a single statement with the expected
// placeholder count — interpolating a clause must not introduce another verb.
func TestRecordStatements_PlaceholdersSurviveInterpolation(t *testing.T) {
	rendered := strings.Replace(queryListRecords, "{ORDER_BY}", "ORDER BY created_at DESC, id ASC", 1)
	assert.Equal(t, 1, strings.Count(rendered, "FROM guliangiam.records"))
	assert.NotContains(t, rendered, ";", "the statement must not gain a terminator")
	assert.Equal(t, 8, countPlaceholders(rendered), "list uses $1..$8")
}

func countPlaceholders(query string) int {
	seen := map[string]struct{}{}
	for _, field := range strings.Fields(query) {
		field = strings.Trim(field, ",()")
		if len(field) > 1 && field[0] == '$' {
			seen[field] = struct{}{}
		}
	}
	return len(seen)
}
