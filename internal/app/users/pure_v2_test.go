package users

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToWebAuthnCredentialItem(t *testing.T) {
	now := time.Now()
	cred := &webauthndomain.Credential{
		ID:              "cred-1",
		CredentialID:    []byte("authenticator-credential"),
		Name:            "My Key",
		AttestationType: "none",
		Transport:       []string{"usb"},
		SignCount:       42,
		Status:          webauthndomain.CredentialStatusActive,
		AAGUID:          "aaguid-1",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	item := toWebAuthnCredentialItem(cred)
	assert.Equal(t, "cred-1", item.ID)
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(cred.CredentialID), item.CredentialID)
	assert.Equal(t, "My Key", item.Name)
	assert.Equal(t, "none", item.AttestationType)
	assert.Equal(t, []string{"usb"}, item.Transport)
	assert.Equal(t, int64(42), item.SignCount)
	assert.Equal(t, "active", item.Status)
	assert.Equal(t, "aaguid-1", item.AAGUID)
	assert.Equal(t, now.Format(time.RFC3339), item.CreatedAt)
	assert.Equal(t, now.Format(time.RFC3339), item.UpdatedAt)
}

func TestCheckPasswordHistoryAgainstEntries_Empty(t *testing.T) {
	err := checkPasswordHistoryAgainstEntries([]userdomain.PasswordHistoryEntry{}, "newpassword", 5)
	require.NoError(t, err)
}

func TestWireMfaVerification_Nil(t *testing.T) {
	WireMfaVerification(nil)
}

// 凭证状态机（§31.3.3）：revoked 是终态，只有三条合法迁移。
func TestCredentialStatusTransition(t *testing.T) {
	revoked := webauthndomain.CredentialStatusRevoked
	deactivated := webauthndomain.CredentialStatusDeactivated
	active := webauthndomain.CredentialStatusActive

	allowed := []struct {
		from, to webauthndomain.CredentialStatus
		action   string
	}{
		{active, revoked, "webauthn.credential.revoked"},
		{deactivated, revoked, "webauthn.credential.revoked"},
		{active, deactivated, "webauthn.credential.deactivated"},
		{deactivated, active, "webauthn.credential.restored"},
	}
	for _, tt := range allowed {
		action, err := credentialStatusTransition(tt.from, tt.to)
		if err != nil {
			t.Fatalf("credentialStatusTransition(%s → %s) error = %v", tt.from, tt.to, err)
		}
		if action != tt.action {
			t.Errorf("credentialStatusTransition(%s → %s) action = %q, want %q", tt.from, tt.to, action, tt.action)
		}
	}

	rejected := []struct {
		name     string
		from, to webauthndomain.CredentialStatus
	}{
		{"revoked is terminal", revoked, active},
		{"revoked cannot be re-revoked", revoked, revoked},
		{"same state is a no-op error", active, active},
		{"deactivated cannot be re-deactivated", deactivated, deactivated},
	}
	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			action, err := credentialStatusTransition(tt.from, tt.to)
			if err == nil {
				t.Fatalf("credentialStatusTransition(%s → %s) 应被拒绝，却返回动作 %q", tt.from, tt.to, action)
			}
			if apperrors.FromError(err).Code != apperrors.CodeConflict {
				t.Errorf("错误码 = %s, want %s（状态冲突而非参数错误）", apperrors.FromError(err).Code, apperrors.CodeConflict)
			}
		})
	}
}

// 非法状态值是闭集拒绝（422），与状态机冲突（409）分开。
func TestUpdateWebAuthnCredentialStatus_RejectsUnknownStatus(t *testing.T) {
	svc := Service{}
	_, err := svc.UpdateWebAuthnCredentialStatus(context.Background(), UpdateWebAuthnCredentialStatusCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		CredentialID:   "Y3JlZC0x",
		Status:         "paused",
	})
	appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "status" || appErr.Meta["allowed"] == nil {
		t.Fatalf("meta = %v, want field=status + allowed", appErr.Meta)
	}
}

func TestUpdateWebAuthnCredentialStatus_RejectsNonBase64CredentialID(t *testing.T) {
	svc := Service{}
	_, err := svc.UpdateWebAuthnCredentialStatus(context.Background(), UpdateWebAuthnCredentialStatusCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		CredentialID:   "not base64!",
		Status:         webauthndomain.CredentialStatusRevoked,
	})
	appErr := assertUsersAppError(t, err, apperrors.CodeUnprocessableEntity)
	if appErr.Meta["field"] != "credentialId" || appErr.Meta["format"] != "base64url" {
		t.Fatalf("meta = %v, want field=credentialId + format=base64url", appErr.Meta)
	}
}
