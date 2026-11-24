package faceidproviders

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func captureWarnings(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buf
}

func sealedSecret(t *testing.T, providerID, plaintext string) string {
	t.Helper()
	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(faceIdProviderAAD(providerID), plaintext)
	require.NoError(t, err)
	require.True(t, crypto.IsEnvelopeToken(sealed))
	return sealed
}

func baseRow(id, organizationID string) faceIdProviderRow {
	now := time.Now()
	return faceIdProviderRow{
		ID:             id,
		OrganizationID: organizationID,
		Name:           "aliyun-face",
		ProviderType:   "aliyun",
		AccessKeyID:    "ak-123",
		Endpoint:       "https://faceid.aliyuncs.com",
		Config:         []byte(`{"confidence":0.95,"liveness":true}`),
		IsEnabled:      true,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
}

func TestToFaceIdProvider(t *testing.T) {
	t.Run("sealed_secret_roundtrip", func(t *testing.T) {
		row := baseRow("fid-001", "org-001")
		row.AccessKeySecret = sealedSecret(t, row.ID, "sk-456")

		result, err := toFaceIdProvider(row)
		require.NoError(t, err)

		assert.Equal(t, "fid-001", result.ID)
		assert.Equal(t, "org-001", result.OrganizationID)
		assert.Equal(t, "aliyun-face", result.Name)
		assert.Equal(t, "aliyun", result.ProviderType)
		assert.Equal(t, "ak-123", result.AccessKeyID)
		assert.Equal(t, "sk-456", result.AccessKeySecret)
		assert.False(t, result.SecretNeedsRotation)
		assert.Equal(t, "https://faceid.aliyuncs.com", result.Endpoint)
		assert.True(t, result.IsEnabled)
		assert.Equal(t, 0.95, result.Config["confidence"])
	})

	t.Run("legacy_plaintext_falls_back_without_logging_value", func(t *testing.T) {
		logs := captureWarnings(t)
		row := baseRow("fid-002", "org-002")
		row.AccessKeySecret = "sk-legacy-plaintext"

		result, err := toFaceIdProvider(row)
		require.NoError(t, err)

		assert.Equal(t, "sk-legacy-plaintext", result.AccessKeySecret)
		assert.True(t, result.SecretNeedsRotation, "plaintext rows must be marked for rotation")
		assert.Contains(t, logs.String(), "fid-002")
		assert.Contains(t, logs.String(), "org-002")
		assert.NotContains(t, logs.String(), "sk-legacy-plaintext", "the WARN must never include the secret")
	})

	t.Run("aad_binding_rejects_another_row_id", func(t *testing.T) {
		row := baseRow("fid-003", "org-003")
		row.AccessKeySecret = sealedSecret(t, "other-provider-id", "sk-456")

		_, err := toFaceIdProvider(row)
		require.Error(t, err)
		assert.ErrorIs(t, err, crypto.ErrAADMismatch)
	})

	t.Run("empty_secret_is_not_configured", func(t *testing.T) {
		row := baseRow("fid-004", "org-004")
		row.AccessKeySecret = ""

		result, err := toFaceIdProvider(row)
		require.NoError(t, err)
		assert.Empty(t, result.AccessKeySecret)
		assert.False(t, result.SecretNeedsRotation)
	})

	t.Run("nil_config_becomes_empty_map", func(t *testing.T) {
		row := baseRow("fid-005", "org-005")
		row.Config = nil

		result, err := toFaceIdProvider(row)
		require.NoError(t, err)
		assert.Equal(t, map[string]any{}, result.Config)
	})

	t.Run("invalid_config_json_returns_error", func(t *testing.T) {
		row := baseRow("fid-006", "org-006")
		row.Config = []byte(`{invalid}`)

		_, err := toFaceIdProvider(row)
		assert.Error(t, err)
	})
}

func TestFaceIdAccessKeySecretForWrite(t *testing.T) {
	t.Run("empty_stays_empty", func(t *testing.T) {
		sealed, err := faceIdAccessKeySecretForWrite(&domain.FaceIdProvider{ID: "fid-1"})
		require.NoError(t, err)
		assert.Empty(t, sealed)
	})

	t.Run("seals_with_row_aad", func(t *testing.T) {
		sealed, err := faceIdAccessKeySecretForWrite(&domain.FaceIdProvider{ID: "fid-1", AccessKeySecret: "sk-456"})
		require.NoError(t, err)
		require.True(t, crypto.IsEnvelopeToken(sealed))
		assert.NotContains(t, sealed, "sk-456")
		assert.True(t, strings.HasSuffix(strings.SplitN(sealed, ":", 2)[0], "-aad"), "v2 AAD envelope expected")

		opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(faceIdProviderAAD("fid-1"), sealed)
		require.NoError(t, err)
		assert.Equal(t, "sk-456", opened)
	})

	t.Run("oversized_envelope_is_refused", func(t *testing.T) {
		secret := strings.Repeat("s", 200)
		_, err := faceIdAccessKeySecretForWrite(&domain.FaceIdProvider{ID: "fid-1", AccessKeySecret: secret})
		require.Error(t, err)
		assert.NotContains(t, err.Error(), secret)
	})
}
