package users

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// mfaCollectionsTestOrg is the organization fake used whenever a derived
// MfaAccount needs the same issuer / accountName source as InitiateUserMfaSetup.
func mfaCollectionsTestUnitOfWork(users *svcMockUserRepo) svcTestUnitOfWork {
	return svcTestUnitOfWork{
		users: users,
		organizations: &svcMockOrgRepo{
			getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
				return &orgdomain.Organization{ID: id, Slug: "built-in", DisplayName: "Built In"}, nil
			},
		},
	}
}

// TestService_UpdateUserCollections_PreservesServerOwnedMfaEntries pins M2:
// the collections write path can no longer touch mfaItems / mfaAccounts — the
// stored server-derived values are always carried over.
func TestService_UpdateUserCollections_PreservesServerOwnedMfaEntries(t *testing.T) {
	const totpSecret = "JBSWY3DPEHPK3PXP"
	stored := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		Addresses:      []userdomain.Address{{Street: "Old Street 1", CountryCode: "CN"}},
		MfaItems:       []userdomain.MfaItem{{MfaType: "totp", Secret: totpSecret}},
		MfaAccounts: []userdomain.MfaAccount{{
			AccountName: "built-in/alice",
			Issuer:      "Built In",
			SecretKey:   totpSecret,
			Origin:      "totp",
		}},
	}

	var captured userdomain.UserCollections
	userRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return stored, nil
		},
		updateCollectionsForOrganizationFn: func(_ context.Context, _, _ string, collections userdomain.UserCollections) error {
			captured = collections
			return nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UOW:   mfaCollectionsTestUnitOfWork(userRepo),
	}

	err := svc.UpdateUserCollections(context.Background(), UpdateUserCollectionsCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		Collections: UserCollectionsInput{
			Addresses: []AddressInput{{Street: "New Street 2", CountryCode: "US"}},
		},
	})
	require.NoError(t, err)

	// The writable keys are replaced...
	require.Len(t, captured.Addresses, 1)
	assert.Equal(t, "New Street 2", captured.Addresses[0].Street)
	// ...while the server-owned MFA mirror survives untouched.
	require.Len(t, captured.MfaItems, 1)
	assert.Equal(t, "totp", captured.MfaItems[0].MfaType)
	assert.Equal(t, totpSecret, captured.MfaItems[0].Secret)
	require.Len(t, captured.MfaAccounts, 1)
	assert.Equal(t, "built-in/alice", captured.MfaAccounts[0].AccountName)
	assert.Equal(t, totpSecret, captured.MfaAccounts[0].SecretKey)
}

// TestService_ReplaceUserMfaMethods_DisableClearsDerivedCollections pins M1 on
// the disable path: turning the TOTP factor off removes its collections mirror
// entries in the same transaction.
func TestService_ReplaceUserMfaMethods_DisableClearsDerivedCollections(t *testing.T) {
	const totpSecret = "JBSWY3DPEHPK3PXP"
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		TotpSecret:     totpSecret,
		Revision:       1,
		MfaMethods: []userdomain.UserMfaMethod{
			{ID: "method-1", UserID: "user-1", MethodType: "app", Enabled: true, Verified: true},
		},
		MfaItems:    []userdomain.MfaItem{{MfaType: "totp", Secret: totpSecret}},
		MfaAccounts: []userdomain.MfaAccount{{AccountName: "built-in/alice", Issuer: "Built In", SecretKey: totpSecret, Origin: "totp"}},
	}
	userRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UUID:  svcFakeUUID{fixedID: "generated-id"},
		UOW:   mfaCollectionsTestUnitOfWork(userRepo),
	}

	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "app", Enabled: false},
	})
	require.NoError(t, err)

	assert.Empty(t, user.MfaItems, "disabled factor must leave no MfaItem mirror entry")
	assert.Nil(t, user.MfaAccounts, "disabled factor must leave no MfaAccount mirror entry")
}

// TestService_ReplaceUserMfaMethods_EnabledTotpDerivesCollections pins the
// re-enable direction: an enabled+verified TOTP factor with a stored secret is
// mirrored with the same issuer / accountName source as InitiateUserMfaSetup.
func TestService_ReplaceUserMfaMethods_EnabledTotpDerivesCollections(t *testing.T) {
	const totpSecret = "JBSWY3DPEHPK3PXP"
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		TotpSecret:     totpSecret,
		Revision:       1,
		MfaMethods: []userdomain.UserMfaMethod{
			{ID: "method-1", UserID: "user-1", MethodType: "app", Enabled: true, Verified: true},
			{ID: "method-2", UserID: "user-1", MethodType: "email", Enabled: true, Verified: true},
			{ID: "method-3", UserID: "user-1", MethodType: "sms", Enabled: false, Verified: true},
		},
	}
	userRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UUID:  svcFakeUUID{fixedID: "generated-id"},
		UOW:   mfaCollectionsTestUnitOfWork(userRepo),
	}

	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "app", Enabled: true},
		{ID: "method-2", MethodType: "email", Enabled: true},
		{ID: "method-3", MethodType: "sms", Enabled: false},
	})
	require.NoError(t, err)

	assert.Equal(t, []userdomain.MfaItem{
		{MfaType: "totp", Secret: totpSecret},
		{MfaType: "email", Secret: ""},
	}, user.MfaItems)
	require.Len(t, user.MfaAccounts, 1)
	assert.Equal(t, "built-in/alice", user.MfaAccounts[0].AccountName)
	assert.Equal(t, "Built In", user.MfaAccounts[0].Issuer)
	assert.Equal(t, totpSecret, user.MfaAccounts[0].SecretKey)
	assert.Equal(t, "totp", user.MfaAccounts[0].Origin)
}

// TestService_AdminRebindTOTP_ClearsDerivedCollections pins M1 on the
// admin-rebind path: the reset factor disappears from the mirror while the
// remaining verified factors stay mirrored.
func TestService_AdminRebindTOTP_ClearsDerivedCollections(t *testing.T) {
	const totpSecret = "JBSWY3DPEHPK3PXP"
	target := &userdomain.User{
		ID:             "target-user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		TotpSecret:     totpSecret,
		Revision:       4,
		MfaMethods: []userdomain.UserMfaMethod{
			{ID: "mfa-totp", UserID: "target-user-1", MethodType: "totp", Enabled: true, Verified: true},
			{ID: "mfa-email", UserID: "target-user-1", MethodType: "email", Enabled: true, Verified: true},
		},
		MfaItems:    []userdomain.MfaItem{{MfaType: "totp", Secret: totpSecret}, {MfaType: "email"}},
		MfaAccounts: []userdomain.MfaAccount{{AccountName: "built-in/alice", Issuer: "Built In", SecretKey: totpSecret, Origin: "totp"}},
	}
	userRepo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return target, nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC)},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.AdminRebindTOTP(context.Background(), AdminRebindTOTPCommand{
		ActorID:            "platform-admin-1",
		ActorPrincipalType: "platform_admin",
		TargetUserID:       "target-user-1",
		Reason:             "device compromised",
		Confirm:            true,
	})
	require.NoError(t, err)

	assert.Equal(t, []userdomain.MfaItem{{MfaType: "email"}}, target.MfaItems)
	assert.Nil(t, target.MfaAccounts)
}

// TestToDetail_MfaCollectionsMaskSecrets pins the credential boundary: the
// server-derived plaintext secrets persisted in the JSONB columns must never
// echo back through the read DTO — only the Set / Masked projection is exposed.
func TestToDetail_MfaCollectionsMaskSecrets(t *testing.T) {
	const totpSecret = "JBSWY3DPEHPK3PXP"
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		MfaItems:       []userdomain.MfaItem{{MfaType: "totp", Secret: totpSecret}},
		MfaAccounts:    []userdomain.MfaAccount{{AccountName: "built-in/alice", Issuer: "Built In", SecretKey: totpSecret, Origin: "totp"}},
	}

	detail := toDetail(user, nil)

	require.Len(t, detail.MfaItems, 1)
	assert.True(t, detail.MfaItems[0].SecretSet)
	assert.Equal(t, "******", detail.MfaItems[0].SecretMasked)
	require.Len(t, detail.MfaAccounts, 1)
	assert.True(t, detail.MfaAccounts[0].SecretKeySet)
	assert.Equal(t, "******", detail.MfaAccounts[0].SecretKeyMasked)
}
