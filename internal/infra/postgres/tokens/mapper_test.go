package tokens

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToToken(t *testing.T) {
	now := time.Now()
	scopes := []string{"openid", "profile", "email"}
	audience := []string{"app-001"}
	metadata, _ := json.Marshal(map[string]any{"dpop_jkt": "thumbprint123"})

	t.Run("full_mapping", func(t *testing.T) {
		row := tokenRow{
			ID:                    "tok-001",
			UserID:                "user-001",
			Username:              "testuser",
			OrganizationID:        "org-001",
			OrganizationSlug:      "test-org",
			ApplicationID:         "app-001",
			ApplicationSlug:       "test-app",
			TokenPrefix:           "gl_",
			TokenHash:             "sha256hash",
			DeviceSecretHash:      "",
			DeviceSecretExpiresIn: 0,
			Scopes:                scopes,
			Audience:              audience,
			Metadata:              metadata,
			ExpiresAt:             now.Add(1 * time.Hour),
			CreatedAt:             now,
			UpdatedAt:             now,
			Revision:              1,
		}

		token, err := toToken(row)
		require.NoError(t, err)

		assert.Equal(t, "tok-001", token.ID)
		assert.Equal(t, "user-001", token.UserID)
		assert.Equal(t, "sha256hash", token.TokenHash)
		assert.Equal(t, []string{"openid", "profile", "email"}, token.Scopes)
		assert.Equal(t, []string{"app-001"}, token.Audience)
		assert.Equal(t, "thumbprint123", token.Metadata["dpop_jkt"])
		assert.Equal(t, int64(1), token.Revision)
	})

	t.Run("nil_fields_default_to_empty", func(t *testing.T) {
		row := tokenRow{
			ID:        "tok-002",
			UserID:    "user-002",
			Scopes:    nil,
			Audience:  nil,
			Metadata:  nil,
			ExpiresAt: now.Add(1 * time.Hour),
			CreatedAt: now,
			UpdatedAt: now,
		}

		token, err := toToken(row)
		require.NoError(t, err)

		assert.NotNil(t, token.Scopes)
		assert.Empty(t, token.Scopes)
		assert.NotNil(t, token.Audience)
		assert.Empty(t, token.Audience)
		assert.NotNil(t, token.Metadata)
		assert.Empty(t, token.Metadata)
	})

	t.Run("revoked_token", func(t *testing.T) {
		revokedAt := now.Add(-10 * time.Minute)
		row := tokenRow{
			ID:        "tok-003",
			UserID:    "user-003",
			RevokedAt: &revokedAt,
			Scopes:    scopes,
			Audience:  audience,
			Metadata:  metadata,
			ExpiresAt: now.Add(1 * time.Hour),
			CreatedAt: now,
			UpdatedAt: now,
		}

		token, err := toToken(row)
		require.NoError(t, err)
		assert.NotNil(t, token.RevokedAt)
	})
}

// scopes and audience are text[] columns, so only the jsonb metadata column can
// still fail to decode.
func TestToToken_InvalidMetadataJSON(t *testing.T) {
	now := time.Now()

	row := tokenRow{
		ID:        "tok-004",
		Scopes:    []string{},
		Audience:  []string{},
		Metadata:  []byte("invalid json"),
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := toToken(row)
	assert.Error(t, err)
}

func TestToRefreshToken(t *testing.T) {
	now := time.Now()
	scopes := []string{"openid"}
	audience := []string{"app-001"}

	t.Run("full_mapping", func(t *testing.T) {
		row := refreshTokenRow{
			ID:               "rt-001",
			AccessTokenID:    "tok-001",
			UserID:           "user-001",
			RefreshTokenHash: "rt_hash",
			FamilyID:         "family-001",
			Scopes:           scopes,
			Audience:         audience,
			ExpiresAt:        now.Add(24 * time.Hour),
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}

		rt := toRefreshToken(row)

		assert.Equal(t, "rt-001", rt.ID)
		assert.Equal(t, "tok-001", rt.AccessTokenID)
		assert.Equal(t, "rt_hash", rt.RefreshTokenHash)
		assert.Equal(t, "family-001", rt.FamilyID)
		assert.Equal(t, []string{"openid"}, rt.Scopes)
		assert.Equal(t, []string{"app-001"}, rt.Audience)
	})

	t.Run("nil_scopes_default_to_empty", func(t *testing.T) {
		row := refreshTokenRow{
			ID:        "rt-002",
			Scopes:    nil,
			Audience:  nil,
			ExpiresAt: now.Add(24 * time.Hour),
			CreatedAt: now,
			UpdatedAt: now,
		}

		rt := toRefreshToken(row)
		assert.NotNil(t, rt.Scopes)
		assert.Empty(t, rt.Scopes)
		assert.NotNil(t, rt.Audience)
		assert.Empty(t, rt.Audience)
	})
}
