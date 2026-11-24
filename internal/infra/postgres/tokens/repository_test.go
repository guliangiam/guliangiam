package tokens

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenRepository_Max(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{"positive_value", 5, 10, 5},
		{"zero_uses_fallback", 0, 10, 10},
		{"negative_uses_fallback", -1, 20, 20},
		{"both_positive", 15, 10, 15},
		{"same_values", 10, 10, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

// scopes and audience are text[] columns, so they are passed to pgx as Go
// []string and never JSON-encoded. nonNilStrings is what keeps a nil slice from
// becoming a NULL parameter (the columns are NOT NULL DEFAULT '{}').
func TestNonNilStrings(t *testing.T) {
	t.Parallel()

	t.Run("nil_returns_empty_slice", func(t *testing.T) {
		result := nonNilStrings(nil)
		require.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("empty_slice_is_preserved", func(t *testing.T) {
		result := nonNilStrings([]string{})
		require.NotNil(t, result)
		assert.Empty(t, result)
	})

	t.Run("values_are_preserved", func(t *testing.T) {
		result := nonNilStrings([]string{"openid", "profile"})
		assert.Equal(t, []string{"openid", "profile"}, result)
	})
}

func TestMarshalJSONObject(t *testing.T) {
	t.Parallel()

	t.Run("nil_returns_empty_object", func(t *testing.T) {
		result, err := marshalJSONObject(nil)
		require.NoError(t, err)
		assert.Equal(t, []byte("{}"), result)
	})

	t.Run("with_values", func(t *testing.T) {
		result, err := marshalJSONObject(map[string]any{"ip": "127.0.0.1"})
		require.NoError(t, err)
		assert.Contains(t, string(result), "127.0.0.1")
	})
}

func TestNullableString_Tokens(t *testing.T) {
	t.Parallel()

	t.Run("empty_returns_nil", func(t *testing.T) {
		result := nullableString("")
		assert.Nil(t, result)
	})

	t.Run("non_empty_returns_value", func(t *testing.T) {
		result := nullableString("devicehash")
		assert.Equal(t, "devicehash", result)
	})
}

func TestTokenRow_DeviceSecretFields(t *testing.T) {
	t.Parallel()

	t.Run("device_secret_fields", func(t *testing.T) {
		row := tokenRow{
			ID:                    "tok-001",
			DeviceSecretHash:      "devicehash789",
			DeviceSecretExpiresIn: 3600,
		}
		assert.Equal(t, "devicehash789", row.DeviceSecretHash)
		assert.Equal(t, 3600, row.DeviceSecretExpiresIn)
	})

	t.Run("empty_device_secret", func(t *testing.T) {
		row := tokenRow{
			ID:                    "tok-002",
			DeviceSecretHash:      "",
			DeviceSecretExpiresIn: 0,
		}
		assert.Equal(t, "", row.DeviceSecretHash)
		assert.Equal(t, 0, row.DeviceSecretExpiresIn)
	})
}

func TestRefreshTokenRow_FamilyFields(t *testing.T) {
	t.Parallel()

	row := refreshTokenRow{
		ID:               "rt-001",
		AccessTokenID:    "at-001",
		FamilyID:         "family-001",
		RefreshTokenHash: "refreshhash123",
	}
	assert.Equal(t, "family-001", row.FamilyID)
	assert.Equal(t, "at-001", row.AccessTokenID)
	assert.Equal(t, "refreshhash123", row.RefreshTokenHash)
}

// --- Enhanced boundary tests for Tokens Repository ---

func TestNonNilStrings_Boundary(t *testing.T) {
	t.Parallel()

	t.Run("single_element", func(t *testing.T) {
		assert.Equal(t, []string{"openid"}, nonNilStrings([]string{"openid"}))
	})

	t.Run("many_elements", func(t *testing.T) {
		scopes := make([]string, 100)
		for i := range scopes {
			scopes[i] = "scope"
		}
		assert.Len(t, nonNilStrings(scopes), 100)
	})

	t.Run("special_chars_in_element", func(t *testing.T) {
		result := nonNilStrings([]string{"openid", "profile/email", "custom<scope>"})
		assert.Contains(t, result, "profile/email")
	})

	t.Run("unicode_elements", func(t *testing.T) {
		result := nonNilStrings([]string{"读取", "写入"})
		assert.Contains(t, result, "读取")
	})
}

func TestMarshalJSONObject_Boundary(t *testing.T) {
	t.Parallel()

	t.Run("multiple_keys", func(t *testing.T) {
		result, err := marshalJSONObject(map[string]any{
			"ip":         "127.0.0.1",
			"user_agent": "Mozilla/5.0",
			"dpop_jkt":   "thumbprint",
		})
		require.NoError(t, err)
		assert.Contains(t, string(result), "127.0.0.1")
		assert.Contains(t, string(result), "Mozilla/5.0")
		assert.Contains(t, string(result), "thumbprint")
	})

	t.Run("nested_value", func(t *testing.T) {
		result, err := marshalJSONObject(map[string]any{
			"device": map[string]any{"browser": "Chrome"},
		})
		require.NoError(t, err)
		assert.Contains(t, string(result), "Chrome")
	})

	t.Run("empty_map", func(t *testing.T) {
		result, err := marshalJSONObject(map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, []byte("{}"), result)
	})
}

func TestNullableString_Tokens_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  any
	}{
		{"space_returns_space", " ", " "},
		{"uuid_returns_value", "01900000-0000-7000-8000-000000000001", "01900000-0000-7000-8000-000000000001"},
		{"hash_returns_value", "$argon2id$v=19$m=65536", "$argon2id$v=19$m=65536"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullableString(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTokenRow_AllFieldsAccessible(t *testing.T) {
	t.Parallel()

	row := tokenRow{
		ID:                    "tok-full",
		UserID:                "user-full",
		Username:              "fulluser",
		OrganizationID:        "org-full",
		OrganizationSlug:      "full-org",
		ApplicationID:         "app-full",
		ApplicationSlug:       "full-app",
		TokenPrefix:           "gl_",
		TokenHash:             "hash123",
		DeviceSecretHash:      "devicehash",
		DeviceSecretExpiresIn: 7200,
		Scopes:                []string{"openid"},
		Audience:              []string{"app-full"},
		Metadata:              []byte(`{"ip":"10.0.0.1"}`),
		ExpiresAt:             time.Now().Add(time.Hour),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
		Revision:              3,
	}

	assert.Equal(t, "tok-full", row.ID)
	assert.Equal(t, "user-full", row.UserID)
	assert.Equal(t, "fulluser", row.Username)
	assert.Equal(t, "org-full", row.OrganizationID)
	assert.Equal(t, "full-org", row.OrganizationSlug)
	assert.Equal(t, "app-full", row.ApplicationID)
	assert.Equal(t, "full-app", row.ApplicationSlug)
	assert.Equal(t, "gl_", row.TokenPrefix)
	assert.Equal(t, "hash123", row.TokenHash)
	assert.Equal(t, "devicehash", row.DeviceSecretHash)
	assert.Equal(t, 7200, row.DeviceSecretExpiresIn)
	assert.Equal(t, int64(3), row.Revision)
}

func TestRefreshTokenRow_AllFieldsAccessible(t *testing.T) {
	t.Parallel()

	now := time.Now()
	row := refreshTokenRow{
		ID:               "rt-full",
		AccessTokenID:    "at-full",
		UserID:           "user-full",
		Username:         "fulluser",
		OrganizationID:   "org-full",
		OrganizationSlug: "full-org",
		ApplicationID:    "app-full",
		ApplicationSlug:  "full-app",
		RefreshTokenHash: "rt_hash_full",
		FamilyID:         "family-full",
		Scopes:           []string{"openid", "profile"},
		Audience:         []string{"app-full"},
		ExpiresAt:        now.Add(24 * time.Hour),
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         2,
	}

	assert.Equal(t, "rt-full", row.ID)
	assert.Equal(t, "at-full", row.AccessTokenID)
	assert.Equal(t, "family-full", row.FamilyID)
	assert.Equal(t, "rt_hash_full", row.RefreshTokenHash)
	assert.Equal(t, int64(2), row.Revision)
}

func TestMax_Tokens_Boundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{"large_page_size", 1000, 20, 1000},
		{"zero_page_size_uses_fallback", 0, 20, 20},
		{"negative_page_size_uses_fallback", -1, 20, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maxInt(tt.value, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}
