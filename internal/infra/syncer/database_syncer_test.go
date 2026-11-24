package syncer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabaseSyncer_IsValidIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"simple", "users", true},
		{"with_underscore", "user_name", true},
		{"with_schema", "public.users", true},
		{"starts_with_digit", "1table", false},
		{"empty", "", false},
		{"special_chars", "user-name", false},
		{"sql_injection", "users; DROP TABLE", false},
		{"space", "user name", false},
		{"single_letter", "a", true},
		{"with_digits", "table123", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isValidIdentifier(tc.input))
		})
	}
}

func TestDatabaseSyncer_BuildUserQuery(t *testing.T) {
	s := &DatabaseSyncer{}

	t.Run("default_table_and_fields", func(t *testing.T) {
		query, args, err := s.buildUserQuery(&SyncerConfig{})
		require.NoError(t, err)
		assert.Contains(t, query, "SELECT")
		assert.Contains(t, query, "FROM users")
		assert.Empty(t, args)
	})

	t.Run("custom_table", func(t *testing.T) {
		query, args, err := s.buildUserQuery(&SyncerConfig{Filter: "public.app_users"})
		require.NoError(t, err)
		assert.Contains(t, query, "FROM public.app_users")
		assert.Empty(t, args)
	})

	t.Run("invalid_table_name", func(t *testing.T) {
		_, _, err := s.buildUserQuery(&SyncerConfig{Filter: "users; DROP TABLE users"})
		assert.Error(t, err)
	})

	t.Run("with_attribute_map", func(t *testing.T) {
		config := &SyncerConfig{
			AttributeMap: map[string]string{
				"email":        "mail",
				"display_name": "name",
			},
		}
		query, args, err := s.buildUserQuery(config)
		require.NoError(t, err)
		assert.Contains(t, query, `"email"`)
		assert.Contains(t, query, `"display_name"`)
		assert.Empty(t, args)
	})

	t.Run("invalid_column_in_attribute_map", func(t *testing.T) {
		config := &SyncerConfig{
			AttributeMap: map[string]string{
				"email; DROP TABLE users": "mail",
			},
		}
		_, _, err := s.buildUserQuery(config)
		assert.Error(t, err)
	})

	t.Run("incremental_sync_with_last_sync_at", func(t *testing.T) {
		now := time.Now()
		query, args, err := s.buildUserQuery(&SyncerConfig{LastSyncAt: &now})
		require.NoError(t, err)
		assert.Contains(t, query, "WHERE updated_at > $1")
		assert.Len(t, args, 1)
		assert.Equal(t, &now, args[0])
	})
}

func TestDatabaseSyncer_BuildGroupQuery(t *testing.T) {
	s := &DatabaseSyncer{}

	t.Run("default_table", func(t *testing.T) {
		query, args, err := s.buildGroupQuery(&SyncerConfig{})
		require.NoError(t, err)
		assert.Contains(t, query, "FROM groups")
		assert.Empty(t, args)
	})

	t.Run("custom_table", func(t *testing.T) {
		query, args, err := s.buildGroupQuery(&SyncerConfig{Filter: "public.app_groups"})
		require.NoError(t, err)
		assert.Contains(t, query, "FROM public.app_groups")
		assert.Empty(t, args)
	})

	t.Run("invalid_table_name", func(t *testing.T) {
		_, _, err := s.buildGroupQuery(&SyncerConfig{Filter: "groups; DROP TABLE groups"})
		assert.Error(t, err)
	})

	t.Run("incremental_sync", func(t *testing.T) {
		now := time.Now()
		query, args, err := s.buildGroupQuery(&SyncerConfig{LastSyncAt: &now})
		require.NoError(t, err)
		assert.Contains(t, query, "WHERE updated_at > $1")
		assert.Len(t, args, 1)
	})
}

func TestDatabaseSyncer_ApplyFieldMapping(t *testing.T) {
	s := &DatabaseSyncer{}

	t.Run("no_mapping_returns_original_columns", func(t *testing.T) {
		columns := []string{"id", "name", "email"}
		values := []interface{}{1, "John", "john@example.com"}
		result := s.applyFieldMapping(columns, values, nil)
		assert.Equal(t, 1, result["id"])
		assert.Equal(t, "John", result["name"])
		assert.Equal(t, "john@example.com", result["email"])
	})

	t.Run("empty_mapping_returns_original_columns", func(t *testing.T) {
		columns := []string{"id", "name"}
		values := []interface{}{42, "test"}
		result := s.applyFieldMapping(columns, values, map[string]string{})
		assert.Equal(t, 42, result["id"])
		assert.Equal(t, "test", result["name"])
	})

	t.Run("mapped_columns_use_target_names", func(t *testing.T) {
		columns := []string{"mail", "cn"}
		values := []interface{}{"user@example.com", "John"}
		attributeMap := map[string]string{
			"mail": "email",
			"cn":   "displayName",
		}
		result := s.applyFieldMapping(columns, values, attributeMap)
		assert.Equal(t, "user@example.com", result["email"])
		assert.Equal(t, "John", result["displayName"])
	})

	t.Run("unmapped_columns_keep_original_names", func(t *testing.T) {
		columns := []string{"mail", "phone"}
		values := []interface{}{"user@example.com", "123-456"}
		attributeMap := map[string]string{
			"mail": "email",
		}
		result := s.applyFieldMapping(columns, values, attributeMap)
		assert.Equal(t, "user@example.com", result["email"])
		assert.Equal(t, "123-456", result["phone"])
	})
}
