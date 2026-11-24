package providers

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireDelegatedKindError(t *testing.T, err error, kind domain.Kind, managedBy string) {
	t.Helper()
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, 422, appErr.HTTPStatus())
	assert.Equal(t, metaKeyKind, appErr.Meta[metaKeyField])
	assert.Equal(t, string(kind), appErr.Meta[metaKeyValue])
	assert.Equal(t, managedBy, appErr.Meta[metaKeyManagedBy])
}

func seedIdentityProvider(t *testing.T, repo *mockProviderRepo, now time.Time, kind domain.Kind, providerType string, status domain.Status) {
	t.Helper()
	registration := domain.Registration{
		ID: "provider-1", OrganizationID: "org-1", Name: string(kind), DisplayName: string(kind),
		Family: domain.FamilyIdentity, Kind: kind, Status: status,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	row := &domain.Provider{
		ID: "provider-1", OrganizationID: "org-1", Name: string(kind), DisplayName: string(kind),
		Category: domain.ProviderCategory("MFA"), ProviderType: domain.ProviderType(providerType),
		Kind: kind, Enabled: status == domain.StatusActive,
		CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	config, err := metadataFromRegistration(registration, registration.ID).config(registration.ID, now)
	require.NoError(t, err)
	repo.providers = []*domain.Provider{row}
	repo.configs = []*domain.ProviderConfig{config}
}

func TestDelegatedIdentityKindsDeclaration(t *testing.T) {
	want := map[domain.Kind]string{
		"ldap":             managedByLDAP,
		"recaptcha":        managedByCaptcha,
		"recaptcha_v3":     managedByCaptcha,
		"hcaptcha":         managedByCaptcha,
		"turnstile":        managedByCaptcha,
		"geetest":          managedByCaptcha,
		"aliyun_captcha":   managedByCaptcha,
		"tencent_captcha":  managedByCaptcha,
		"faceid":           managedByFaceID,
		"aliyun_face_body": managedByFaceID,
	}
	require.Equal(t, want, delegatedIdentityKinds)

	for kind, managedBy := range want {
		err := newDelegatedKindError(kind, managedBy)
		var appErr *apperrors.Error
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
		assert.Equal(t, 422, appErr.HTTPStatus())
		assert.Equal(t, metaKeyKind, appErr.Meta[metaKeyField])
		assert.Equal(t, string(kind), appErr.Meta[metaKeyValue])
		assert.Equal(t, managedBy, appErr.Meta[metaKeyManagedBy])
	}
}

func TestUnifiedCreateRejectsDelegatedIdentityKinds(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		providerType string
		kind         domain.Kind
		managedBy    string
	}{
		{name: "ldap", providerType: "LDAP", kind: "ldap", managedBy: managedByLDAP},
		{name: "recaptcha", providerType: "reCAPTCHA", kind: "recaptcha", managedBy: managedByCaptcha},
		{name: "recaptcha v3 alias", providerType: "reCAPTCHAv3", kind: "recaptcha_v3", managedBy: managedByCaptcha},
		{name: "hcaptcha", providerType: "HCaptcha", kind: "hcaptcha", managedBy: managedByCaptcha},
		{name: "turnstile", providerType: "Turnstile", kind: "turnstile", managedBy: managedByCaptcha},
		{name: "geetest", providerType: "Geetest", kind: "geetest", managedBy: managedByCaptcha},
		{name: "aliyun captcha alias", providerType: "AliyunCaptcha", kind: "aliyun_captcha", managedBy: managedByCaptcha},
		{name: "tencent captcha alias", providerType: "TencentCaptcha", kind: "tencent_captcha", managedBy: managedByCaptcha},
		{name: "faceid", providerType: "FaceID", kind: "faceid", managedBy: managedByFaceID},
		{name: "aliyun face body snake case", providerType: "AliyunFaceBody", kind: "aliyun_face_body", managedBy: managedByFaceID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, records, outbox := newUnifiedTestService(now)
			command := createIdentityCommand("initial-secret")
			command.ProviderType = tt.providerType

			_, err := svc.CreateProvider(context.Background(), command)
			requireDelegatedKindError(t, err, tt.kind, tt.managedBy)

			assert.Empty(t, repo.providers)
			assert.Empty(t, repo.configs)
			assert.Empty(t, repo.secretStatus)
			assert.Empty(t, records.items)
			assert.Empty(t, outbox.items)
		})
	}
}

func TestUnifiedCreateRejectsExplicitDelegatedKind(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	command := createIdentityCommand("initial-secret")
	command.Kind = "ldap"

	_, err := svc.CreateProvider(context.Background(), command)
	requireDelegatedKindError(t, err, "ldap", managedByLDAP)
	assert.Empty(t, repo.providers)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestUnifiedPatchRejectsDelegatedKindChanges(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		apply     func(*PatchProviderCommand)
		kind      domain.Kind
		managedBy string
	}{
		{
			name: "kind field", apply: func(command *PatchProviderCommand) { command.Kind = stringPtr("ldap") },
			kind: "ldap", managedBy: managedByLDAP,
		},
		{
			name: "provider type canonical", apply: func(command *PatchProviderCommand) { command.ProviderType = stringPtr("LDAP") },
			kind: "ldap", managedBy: managedByLDAP,
		},
		{
			name: "provider type alias", apply: func(command *PatchProviderCommand) { command.ProviderType = stringPtr("reCAPTCHAv3") },
			kind: "recaptcha_v3", managedBy: managedByCaptcha,
		},
		{
			name: "provider type snake case", apply: func(command *PatchProviderCommand) { command.ProviderType = stringPtr("AliyunFaceBody") },
			kind: "aliyun_face_body", managedBy: managedByFaceID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, records, outbox := newUnifiedTestService(now)
			seedIdentityProvider(t, repo, now, "github", "github", domain.StatusDisabled)

			command := PatchProviderCommand{ID: "provider-1", OrganizationID: "org-1"}
			tt.apply(&command)

			_, err := svc.Unified.Patch(context.Background(), command)
			requireDelegatedKindError(t, err, tt.kind, tt.managedBy)
			assert.Equal(t, domain.Kind("github"), repo.providers[0].Kind)
			assert.Empty(t, records.items)
			assert.Empty(t, outbox.items)
		})
	}
}

func TestUnifiedPatchRejectsMovingDelegatedRowOut(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		apply     func(*PatchProviderCommand)
		kind      domain.Kind
		managedBy string
	}{
		{
			name: "kind move out", apply: func(command *PatchProviderCommand) { command.Kind = stringPtr("github") },
			kind: "ldap", managedBy: managedByLDAP,
		},
		{
			name: "provider type move out", apply: func(command *PatchProviderCommand) { command.ProviderType = stringPtr("github") },
			kind: "ldap", managedBy: managedByLDAP,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, records, outbox := newUnifiedTestService(now)
			seedIdentityProvider(t, repo, now, "ldap", "LDAP", domain.StatusDisabled)

			command := PatchProviderCommand{ID: "provider-1", OrganizationID: "org-1"}
			tt.apply(&command)

			_, err := svc.Unified.Patch(context.Background(), command)
			requireDelegatedKindError(t, err, tt.kind, tt.managedBy)
			assert.Empty(t, records.items)
			assert.Empty(t, outbox.items)
		})
	}
}

func TestUnifiedPatchAllowsNonKindEditOnDelegatedRow(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	seedIdentityProvider(t, repo, now, "ldap", "LDAP", domain.StatusDisabled)

	displayName := "Renamed LDAP"
	_, err := svc.Unified.Patch(context.Background(), PatchProviderCommand{
		ID: "provider-1", OrganizationID: "org-1", DisplayName: &displayName,
	})
	require.NoError(t, err)
	assert.Equal(t, displayName, repo.providers[0].DisplayName)
	assert.Len(t, records.items, 1)
	assert.Len(t, outbox.items, 1)
}

func TestUnifiedPatchRejectsSecretRotationOnDelegatedRow(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	seedIdentityProvider(t, repo, now, "ldap", "LDAP", domain.StatusDisabled)

	secret := "rotated-secret"
	_, err := svc.Unified.Patch(context.Background(), PatchProviderCommand{
		ID: "provider-1", OrganizationID: "org-1", ClientSecret: &secret,
	})
	requireDelegatedKindError(t, err, "ldap", managedByLDAP)
	assert.Empty(t, repo.secretStatus)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestUnifiedActivateRejectsDelegatedKind(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	seedIdentityProvider(t, repo, now, "ldap", "LDAP", domain.StatusTesting)

	_, err := svc.Unified.ChangeStatus(context.Background(), ChangeProviderStatusCommand{
		ID: "provider-1", Family: "identity", Status: string(domain.StatusActive),
	})
	requireDelegatedKindError(t, err, "ldap", managedByLDAP)

	metadata, err := getRegistryMetadata(context.Background(), repo, "provider-1")
	require.NoError(t, err)
	assert.Equal(t, string(domain.StatusTesting), metadata.Status)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestUnifiedRotateSecretRejectsDelegatedKind(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	seedIdentityProvider(t, repo, now, "faceid", "FaceID", domain.StatusDisabled)

	_, err := svc.Unified.RotateSecret(context.Background(), RotateProviderSecretCommand{
		ID: "provider-1", Family: "identity", Plaintext: "rotated-secret",
	})
	requireDelegatedKindError(t, err, "faceid", managedByFaceID)
	assert.Empty(t, repo.secretStatus)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestUnifiedTestConnectionRejectsDelegatedKind(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	svc, repo, records, outbox := newUnifiedTestService(now)
	seedIdentityProvider(t, repo, now, "recaptcha_v3", "reCAPTCHAv3", domain.StatusDisabled)

	_, err := svc.TestConnection(context.Background(), "provider-1", "org-1", "")
	requireDelegatedKindError(t, err, "recaptcha_v3", managedByCaptcha)
	assert.Empty(t, records.items)
	assert.Empty(t, outbox.items)
}

func TestUnifiedDisableAndDeleteDelegatedKindStayAllowed(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)

	t.Run("disable", func(t *testing.T) {
		svc, repo, _, _ := newUnifiedTestService(now)
		seedIdentityProvider(t, repo, now, "ldap", "LDAP", domain.StatusActive)

		_, err := svc.Unified.ChangeStatus(context.Background(), ChangeProviderStatusCommand{
			ID: "provider-1", Family: "identity", Status: string(domain.StatusDisabled),
		})
		require.NoError(t, err)

		metadata, err := getRegistryMetadata(context.Background(), repo, "provider-1")
		require.NoError(t, err)
		assert.Equal(t, string(domain.StatusDisabled), metadata.Status)
	})

	t.Run("delete", func(t *testing.T) {
		svc, repo, _, _ := newUnifiedTestService(now)
		seedIdentityProvider(t, repo, now, "ldap", "LDAP", domain.StatusDisabled)

		require.NoError(t, svc.DeleteProvider(context.Background(), "provider-1", "org-1", ""))
		assert.Empty(t, repo.providers)
	})
}

func TestUnifiedReadDelegatedKindStaysAllowed(t *testing.T) {
	now := time.Date(2026, 11, 3, 8, 0, 0, 0, time.UTC)
	svc, repo, _, _ := newUnifiedTestService(now)
	seedIdentityProvider(t, repo, now, "faceid", "FaceID", domain.StatusDisabled)

	detail, err := svc.GetProvider(context.Background(), "provider-1", "org-1")
	require.NoError(t, err)
	assert.Equal(t, "faceid", detail.Kind)

	items, total, err := svc.ListProviders(context.Background(), ListProvidersQuery{
		OrganizationID: "org-1", Page: 1, PageSize: 20,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "faceid", items[0].Kind)
}

func stringPtr(value string) *string {
	return &value
}
