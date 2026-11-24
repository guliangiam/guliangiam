package sorting

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func testSpec() Spec {
	return NewSpec(map[string]string{
		"createdAt": "created_at",
		"username":  "username",
		"isAdmin":   "is_admin",
	})
}

func appErrorCode(t *testing.T, err error) apperrors.Code {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "error %v is not an *apperrors.Error", err)
	return appErr.Code
}

// The whitelist is keyed by the API name and valued by the physical column. The
// map this replaces had each value equal to its key, which is why the comment
// claiming it "maps API field names to database column names" was false and why
// the documented `sortField=createdAt` fell outside the whitelist.
func TestSpec_KeysAreAPINamesValuesAreColumns(t *testing.T) {
	got, err := testSpec().Resolve("createdAt", "desc", "created_at")
	require.NoError(t, err)
	assert.Equal(t, "ORDER BY created_at DESC, id ASC", got)
}

// 手册 4.8.1 第 3 条: every ORDER BY ends with a unique-key tiebreaker. Without
// one, rows sharing a timestamp have no defined order and consecutive pages can
// repeat or skip rows.
func TestSpec_AlwaysAppendsTheTiebreaker(t *testing.T) {
	cases := []struct {
		field string
		order string
	}{
		{"createdAt", "asc"},
		{"createdAt", "desc"},
		{"username", "asc"},
		{"", "desc"},
	}
	for _, tc := range cases {
		t.Run(tc.field+"/"+tc.order, func(t *testing.T) {
			got, err := testSpec().Resolve(tc.field, tc.order, "created_at")
			require.NoError(t, err)
			assert.True(t, strings.HasSuffix(got, ", id ASC"), "clause %q lacks the tiebreaker", got)
		})
	}
}

// A field outside the whitelist is a 422, never a silently different order: a
// caller that receives rows in an order it did not ask for has no way to notice.
func TestSpec_RejectsFieldsOutsideTheWhitelist(t *testing.T) {
	rejected := map[string]string{
		"unknown name":       "nope",
		"snake_case name":    "created_at",
		"database column":    "is_admin",
		"wrong case":         "CreatedAt",
		"sensitive column":   "password_hash",
		"sql injection":      "username; DROP TABLE guliangiam.users",
		"injection via dash": "id--",
		"empty-ish value":    "-",
	}
	for name, field := range rejected {
		t.Run(name, func(t *testing.T) {
			_, err := testSpec().Resolve(field, "asc", "created_at")
			require.Error(t, err)
			assert.Equal(t, apperrors.CodeUnprocessableEntity, appErrorCode(t, err))
		})
	}
}

// The error has to be actionable: it names the field and lists what is allowed,
// so a caller can fix the request without reading the source.
func TestSpec_RejectionNamesTheAllowedFields(t *testing.T) {
	_, err := testSpec().Resolve("nope", "asc", "created_at")
	require.Error(t, err)

	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr))
	assert.Contains(t, appErr.Message, "sortField")

	allowed, ok := appErr.Meta["allowed"].([]string)
	require.True(t, ok, "the details must carry the allowed set")
	assert.Equal(t, []string{"createdAt", "isAdmin", "username"}, allowed)
	assert.Equal(t, "nope", appErr.Meta["value"])
}

// An empty field means "no preference", which is not an error — the caller gets
// the module's default order. An empty direction means ascending.
func TestSpec_EmptyValuesTakeTheDocumentedDefaults(t *testing.T) {
	t.Run("empty field uses the fallback column", func(t *testing.T) {
		got, err := testSpec().Resolve("", "", "created_at")
		require.NoError(t, err)
		assert.Equal(t, "ORDER BY created_at ASC, id ASC", got)
	})

	t.Run("empty direction means ascending", func(t *testing.T) {
		got, err := testSpec().Resolve("username", "", "created_at")
		require.NoError(t, err)
		assert.Equal(t, "ORDER BY username ASC, id ASC", got)
	})

	t.Run("surrounding whitespace is trimmed", func(t *testing.T) {
		got, err := testSpec().Resolve("  username  ", "  desc  ", "created_at")
		require.NoError(t, err)
		assert.Equal(t, "ORDER BY username DESC, id ASC", got)
	})
}

// A direction that is not asc/desc is a 422. The old comparison was against the
// lowercase literal `desc`, so `sortOrder=DESC` silently produced ascending
// results — a client could not tell its parameter had been ignored.
func TestParseDirection(t *testing.T) {
	accepted := map[string]Direction{
		"":       Ascending,
		"asc":    Ascending,
		"ASC":    Ascending,
		"Asc":    Ascending,
		" asc ":  Ascending,
		"desc":   Descending,
		"DESC":   Descending,
		"Desc":   Descending,
		" desc ": Descending,
	}
	for input, want := range accepted {
		t.Run("accepts "+input, func(t *testing.T) {
			got, err := ParseDirection(input)
			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}

	for _, input := range []string{"ascending", "descending", "rand()", "up", "1", "ASC; --"} {
		t.Run("rejects "+input, func(t *testing.T) {
			_, err := ParseDirection(input)
			require.Error(t, err)
			assert.Equal(t, apperrors.CodeUnprocessableEntity, appErrorCode(t, err))
		})
	}
}

// Every clause this package produces must be safe to interpolate into SQL: the
// column comes from the whitelist, never from the request, and the direction is
// one of two literals. This is the property that makes the `%s` slot in the
// records queries acceptable (手册 3.9 / 4.8.1 第 2 条 SQL 注入红线).
func TestSpec_ClauseIsSafeToInterpolate(t *testing.T) {
	spec := testSpec()
	hostile := []string{
		"username' OR 1=1 --",
		"username; DELETE FROM guliangiam.users",
		"username) UNION SELECT * FROM guliangiam.users --",
		"*",
	}

	for _, field := range hostile {
		t.Run(field, func(t *testing.T) {
			_, err := spec.Resolve(field, "asc", "created_at")
			require.Error(t, err, "a hostile field must be refused, not interpolated")
		})
	}

	// And a resolved clause contains only the whitelisted column plus literals.
	clause, err := spec.Resolve("username", "desc", "created_at")
	require.NoError(t, err)
	assert.NotContains(t, clause, ";")
	assert.NotContains(t, clause, "--")
	assert.NotContains(t, clause, "(")
}

func TestSpec_AllowedFieldsIsSorted(t *testing.T) {
	assert.Equal(t, []string{"createdAt", "isAdmin", "username"}, testSpec().AllowedFields())
	assert.Empty(t, NewSpec(nil).AllowedFields())
}

func TestSpec_CustomTiebreaker(t *testing.T) {
	spec := Spec{
		Fields:     map[string]string{"providerId": "provider_id"},
		Tiebreaker: "config_type",
	}
	got, err := spec.Resolve("providerId", "asc", "provider_id")
	require.NoError(t, err)
	assert.Equal(t, "ORDER BY provider_id ASC, config_type ASC", got)
}

// A blank Tiebreaker falls back to the documented default rather than emitting
// an empty identifier, which would be a syntax error at the database.
func TestSpec_BlankTiebreakerFallsBack(t *testing.T) {
	spec := Spec{Fields: map[string]string{"a": "a_col"}, Tiebreaker: "   "}
	got, err := spec.Resolve("a", "asc", "a_col")
	require.NoError(t, err)
	assert.Contains(t, got, ", id ASC")
}
