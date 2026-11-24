package users

import (
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// The whitelist is keyed by the **API** field names a client sends (手册 4.5.1 /
// 4.8.1). The old table pinned the opposite: snake_case keys, `sortField=createdAt`
// outside the whitelist, and a silent fallback to `ORDER BY created_at DESC`.
// Those three assertions were the defect written down as expected behaviour.
func TestBuildOrderBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		sortField  string
		sortOrder  string
		wantClause string
	}{
		{"ascending username", "username", "asc", "ORDER BY username ASC, id ASC"},
		{"descending createdAt", "createdAt", "desc", "ORDER BY created_at DESC, id ASC"},
		{"updatedAt", "updatedAt", "desc", "ORDER BY updated_at DESC, id ASC"},
		{"status", "status", "asc", "ORDER BY status ASC, id ASC"},
		{"isAdmin", "isAdmin", "desc", "ORDER BY is_admin DESC, id ASC"},
		{"score", "score", "desc", "ORDER BY score DESC, id ASC"},
		{"lastSigninAt", "lastSigninAt", "desc", "ORDER BY last_signin_at DESC, id ASC"},
		// 手册 4.8.1: an empty field means "no preference", so the module default
		// applies; an empty direction means ascending.
		{"no field takes the default order", "", "desc", "ORDER BY created_at DESC, id ASC"},
		{"no direction means ascending", "username", "", "ORDER BY username ASC, id ASC"},
		{"uppercase direction is accepted", "username", "DESC", "ORDER BY username DESC, id ASC"},
		{"padded direction is accepted", "username", " desc ", "ORDER BY username DESC, id ASC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildOrderBy(tt.sortField, tt.sortOrder)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantClause, got)
			// 手册 4.8.1 第 3 条: every ORDER BY ends with the unique-key
			// tiebreaker, otherwise rows sharing a timestamp have no defined
			// order and consecutive pages can repeat or skip rows.
			assert.Contains(t, got, ", id ASC")
		})
	}
}

// A field outside the whitelist is a 422, never a silently different order.
func TestBuildOrderBy_RejectsUnlistedFields(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"unknown field":    "not_a_field",
		"snake_case name":  "created_at",
		"sensitive column": "password_hash",
		"sql injection":    "username; DROP TABLE guliangiam.users",
		"injection via id": "id--",
		"mixed case":       "CreatedAt",
	}

	for name, field := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := buildOrderBy(field, "asc")
			assert.Error(t, err)
			assert.Equal(t, "unprocessable_entity", string(appErrorCode(t, err)))
		})
	}
}

func TestBuildOrderBy_RejectsUnknownDirection(t *testing.T) {
	t.Parallel()

	for _, order := range []string{"invalid", "ascending", "descending", "rand()", "ASC; --"} {
		t.Run(order, func(t *testing.T) {
			_, err := buildOrderBy("username", order)
			assert.Error(t, err)
			assert.Equal(t, "unprocessable_entity", string(appErrorCode(t, err)))
		})
	}
}

func appErrorCode(t *testing.T, err error) apperrors.Code {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %v is not an *apperrors.Error", err)
	}
	return appErr.Code
}

func TestBuildColumnSearch(t *testing.T) {
	t.Parallel()

	t.Run("empty_field_returns_empty", func(t *testing.T) {
		clause, value := buildColumnSearch("", "test")
		assert.Equal(t, "", clause)
		assert.Nil(t, value)
	})

	t.Run("empty_value_returns_empty", func(t *testing.T) {
		clause, value := buildColumnSearch("username", "")
		assert.Equal(t, "", clause)
		assert.Nil(t, value)
	})

	t.Run("both_empty_returns_empty", func(t *testing.T) {
		clause, value := buildColumnSearch("", "")
		assert.Equal(t, "", clause)
		assert.Nil(t, value)
	})

	t.Run("valid_username_search", func(t *testing.T) {
		clause, value := buildColumnSearch("username", "john")
		assert.Contains(t, clause, "u.username ILIKE")
		// $8: $1-$4 are the fixed filters, $5 is the optional is_admin filter,
		// $6/$7 are LIMIT/OFFSET, so the column-search value is the eighth arg.
		assert.Contains(t, clause, "$8")
		assert.Equal(t, "john", value)
	})

	t.Run("invalid_field_returns_empty", func(t *testing.T) {
		clause, value := buildColumnSearch("invalid_field", "test")
		assert.Equal(t, "", clause)
		assert.Nil(t, value)
	})
}

func TestNullableString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  any
	}{
		{"empty_returns_nil", "", nil},
		{"non_empty_returns_value", "hello", "hello"},
		{"uuid_returns_value", "01900000-0000-7000-8000-000000000001", "01900000-0000-7000-8000-000000000001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullableString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
func TestAllowedSearchFields(t *testing.T) {
	t.Parallel()

	t.Run("username_is_allowed", func(t *testing.T) {
		col, ok := allowedSearchFields["username"]
		assert.True(t, ok)
		assert.Equal(t, "u.username", col)
	})

	t.Run("email_is_not_allowed", func(t *testing.T) {
		_, ok := allowedSearchFields["email"]
		assert.False(t, ok)
	})
}

// --- Enhanced boundary tests for Users Repository ---

// The boundary table used to assert that injection attempts and case mismatches
// were silently replaced by a default order — i.e. it pinned the fallback as
// correct. Each of those inputs must now be refused, and the refusal must be a
// 422 so the caller learns the request was wrong rather than receiving rows in an
// order it did not ask for.
func TestBuildOrderBy_Boundary(t *testing.T) {
	t.Parallel()

	rejected := []struct {
		name      string
		sortField string
		sortOrder string
	}{
		{"sql injection in field", "1; DROP TABLE users;--", "asc"},
		{"sql injection in order", "username", "DESC; DROP TABLE users;--"},
		{"uppercase field name", "USERNAME", "asc"},
		{"snake_case field name", "user_type", "asc"},
		{"snake_case field name desc", "is_forbidden", "desc"},
	}

	for _, tc := range rejected {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildOrderBy(tc.sortField, tc.sortOrder)
			assert.Error(t, err)
			assert.Equal(t, "unprocessable_entity", string(appErrorCode(t, err)))
		})
	}

	// Surrounding whitespace is tolerated (the same way sortOrder is), because a
	// padded value is unambiguous — unlike an unknown name, which is a mistake.
	t.Run("padded field name is trimmed", func(t *testing.T) {
		got, err := buildOrderBy(" username ", "asc")
		assert.NoError(t, err)
		assert.Equal(t, "ORDER BY username ASC, id ASC", got)
	})

	// The whitelist is exhaustive: every field the API documents must resolve.
	for apiName, column := range userSortSpec.Fields {
		t.Run("whitelisted "+apiName, func(t *testing.T) {
			got, err := buildOrderBy(apiName, "desc")
			assert.NoError(t, err)
			assert.Equal(t, "ORDER BY "+column+" DESC, id ASC", got)
		})
	}
}

func TestBuildColumnSearch_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		searchField     string
		searchValue     string
		wantClauseEmpty bool
		wantValueNil    bool
	}{
		{"whitespace_only_field", " ", "test", true, true},
		{"whitespace_only_value", "username", " ", false, false}, // " " is not empty, buildColumnSearch treats it as valid
		{"special_chars_value", "username", "O'Brien", false, false},
		{"unicode_value", "username", "用户名", false, false},
		{"very_long_value", "username", string(make([]byte, 1000)), false, false},
		{"sql_injection_field", "1; DROP TABLE users;--", "test", true, true},
		{"sql_injection_value", "username", "'; DROP TABLE users;--", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clause, value := buildColumnSearch(tt.searchField, tt.searchValue)
			if tt.wantClauseEmpty {
				assert.Empty(t, clause)
			} else {
				assert.NotEmpty(t, clause)
			}
			if tt.wantValueNil {
				assert.Nil(t, value)
			} else {
				assert.NotNil(t, value)
			}
		})
	}
}

func TestNullableString_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  any
	}{
		{"space_returns_space", " ", " "},
		{"tab_returns_tab", "\t", "\t"},
		{"newline_returns_newline", "\n", "\n"},
		{"uuid_format", "01900000-0000-7000-8000-000000000001", "01900000-0000-7000-8000-000000000001"},
		{"special_chars", "O'Brien", "O'Brien"},
		{"unicode", "用户", "用户"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullableString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildColumnSearch_SQLInjectionPrevention(t *testing.T) {
	t.Parallel()

	// Verify that only whitelisted fields produce search clauses
	dangerousFields := []string{
		"password_hash", "1=1", "id", "organization_id",
		"created_at", "updated_at", "deleted_at",
	}

	for _, field := range dangerousFields {
		t.Run("blocked_"+field, func(t *testing.T) {
			clause, value := buildColumnSearch(field, "malicious")
			assert.Empty(t, clause, "field %q should not produce a search clause", field)
			assert.Nil(t, value, "field %q should not produce a search value", field)
		})
	}
}
