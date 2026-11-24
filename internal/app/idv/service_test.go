package idv

import (
	"encoding/json"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A provider rejecting its own credentials must read as a dependency outage
// (503), because 401 would make the UI treat it as an expired user session.
func TestProviderCredentialErrorRemapsAuthFailures(t *testing.T) {
	for _, code := range []apperrors.Code{apperrors.CodeUnauthorized, apperrors.CodeForbidden} {
		err := providerCredentialError(apperrors.NewWithMeta(code, "provider said no", nil))
		appErr, ok := err.(*apperrors.Error)
		require.True(t, ok)
		assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
		assert.Contains(t, appErr.Message, "rejected its credentials")
	}

	invalid := apperrors.NewInvalidArgument("bad request")
	assert.Equal(t, invalid, providerCredentialError(invalid))
	assert.NoError(t, providerCredentialError(nil))
}

func TestNewServiceDefaultsAreUsable(t *testing.T) {
	svc := NewService(nil)
	if svc.UUID == nil || svc.Clock == nil {
		t.Fatalf("defaults not filled: %+v", svc)
	}
	first, second := svc.newID(), svc.newID()
	if first == "" || second == "" || first == second {
		t.Fatalf("newID() not unique: %q, %q", first, second)
	}
	fixed := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	svc.Clock = func() time.Time { return fixed }
	if got := svc.now(); !got.Equal(fixed) {
		t.Fatalf("now() = %v, want %v", got, fixed)
	}
}

func TestToDTOMapsAllStatusAndOptionalFields(t *testing.T) {
	userID := "user-1"
	redirect := "https://idv.example/cb"
	reason := "document blurry"
	confidence := 0.82
	idNumber := "AB1234567"
	completed := time.Date(2026, 10, 29, 13, 0, 0, 0, time.UTC)
	v := &domain.Verification{
		ID:             "ver-1",
		OrganizationID: "org-1",
		UserID:         &userID,
		Provider:       "faceid",
		Status:         domain.VerificationStatus("completed"),
		RedirectURL:    &redirect,
		IDNumber:       &idNumber,
		Reason:         &reason,
		Confidence:     &confidence,
		RawResult:      json.RawMessage(`{"ok":true}`),
		CompletedAt:    &completed,
		CreatedAt:      completed,
		UpdatedAt:      completed,
	}
	dto := toDTO(v)
	if dto.ID != v.ID || dto.OrganizationID != v.OrganizationID || dto.Status != "completed" {
		t.Fatalf("dto = %+v", dto)
	}
	if dto.UserID == nil || *dto.UserID != userID || dto.RedirectURL == nil || *dto.RedirectURL != redirect {
		t.Fatalf("optional fields dropped: %+v", dto)
	}
	if dto.Reason == nil || *dto.Reason != reason || dto.Confidence == nil || *dto.Confidence != confidence {
		t.Fatalf("reason/confidence dropped: %+v", dto)
	}
	if !dto.IDNumberSet || dto.IDNumberMasked != "AB***67" || !dto.RawResultSet || dto.RawResultMasked != "***" {
		t.Fatalf("PII fields not masked: %+v", dto)
	}
}

func TestToDTO_MasksSessionToken(t *testing.T) {
	token := "sess-secret-123"
	dto := toDTO(&domain.Verification{ID: "ver-1", OrganizationID: "org-1", SessionToken: &token})
	assert.True(t, dto.SessionTokenSet)
	assert.Equal(t, "se***23", dto.SessionTokenMasked)

	encoded, err := json.Marshal(dto)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "sess-secret-123")
	assert.NotContains(t, string(encoded), `"sessionToken":`)
	assert.Contains(t, string(encoded), `"sessionTokenSet":true`)
}

func TestToDTO_MasksPIIFields(t *testing.T) {
	idNumber := "AB1234567"
	dto := toDTO(&domain.Verification{
		ID:             "ver-1",
		OrganizationID: "org-1",
		IDNumber:       &idNumber,
		RawResult:      json.RawMessage(`{"document":{"number":"AB1234567"}}`),
	})
	assert.True(t, dto.IDNumberSet)
	assert.Equal(t, "AB***67", dto.IDNumberMasked)
	assert.True(t, dto.RawResultSet)
	assert.Equal(t, "***", dto.RawResultMasked)

	encoded, err := json.Marshal(dto)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), "AB1234567")
	assert.NotContains(t, string(encoded), `"idNumber":`)
	assert.NotContains(t, string(encoded), `"rawResult":`)
	assert.Contains(t, string(encoded), `"idNumberSet":true`)
	assert.Contains(t, string(encoded), `"rawResultSet":true`)
}

func TestToDTO_UnsetPIIFields(t *testing.T) {
	dto := toDTO(&domain.Verification{ID: "ver-1", OrganizationID: "org-1"})
	assert.False(t, dto.IDNumberSet)
	assert.Empty(t, dto.IDNumberMasked)
	assert.False(t, dto.RawResultSet)
	assert.Empty(t, dto.RawResultMasked)

	encoded, err := json.Marshal(dto)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"idNumber":`)
	assert.NotContains(t, string(encoded), `"rawResult":`)
}

func TestToDTO_UnsetSessionToken(t *testing.T) {
	dto := toDTO(&domain.Verification{ID: "ver-1", OrganizationID: "org-1"})
	assert.False(t, dto.SessionTokenSet)
	assert.Empty(t, dto.SessionTokenMasked)

	encoded, err := json.Marshal(dto)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"sessionToken"`)
	assert.NotContains(t, string(encoded), "sessionTokenMasked")
}

// 重映射为依赖不可用时必须保留 infra 层 Meta：providerCode 诊断信息要能
// 穿过 infra→app 两层到达响应信封，否则凭据拒绝面同样只剩泛化 503。
func TestProviderCredentialErrorPreservesMeta(t *testing.T) {
	meta := map[string]any{"provider": "aliyun_idv", "providerCode": "Forbidden.NoPermission"}
	err := providerCredentialError(apperrors.NewWithMeta(apperrors.CodeUnauthorized, "credential rejected", meta))
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
	assert.Equal(t, meta, appErr.Meta)
}
