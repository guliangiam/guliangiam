package users

import (
	"context"
	"errors"
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// NewService test
// ---------------------------------------------------------------------------

func TestNewService_Users(t *testing.T) {
	svc := NewService(svcFakeClock{fixedTime: time.Now()}, svcFakeUUID{fixedID: "test"}, svcTestUnitOfWork{}, nil)
	if svc.Clock == nil {
		t.Error("Clock should not be nil")
	}
	if svc.UUID == nil {
		t.Error("UUID should not be nil")
	}
	if svc.UOW == nil {
		t.Error("UOW should not be nil")
	}
}

// ---------------------------------------------------------------------------
// DeleteUserIdentity tests
// ---------------------------------------------------------------------------

func TestService_DeleteUserIdentity_Success(t *testing.T) {
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID: "user-1", OrganizationID: "org-1",
				Identities: []userdomain.UserIdentity{{ID: "identity-1", UserID: "user-1"}},
			}, nil
		},
		deleteIdentityFn: func(ctx context.Context, userID string, identityID string) error {
			return nil
		},
	}

	svc := Service{UOW: svcTestUnitOfWork{users: userRepo}}
	err := svc.DeleteUserIdentity(context.Background(), "org-1", "user-1", "identity-1")
	if err != nil {
		t.Fatalf("DeleteUserIdentity() error = %v", err)
	}
}

func TestService_DeleteUserIdentity_EmptyUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.DeleteUserIdentity(context.Background(), "org-1", "", "identity-1")
	if err == nil {
		t.Fatal("expected error for empty userID, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}

func TestService_DeleteUserIdentity_EmptyIdentityID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.DeleteUserIdentity(context.Background(), "org-1", "user-1", "")
	if err == nil {
		t.Fatal("expected error for empty identityID, got nil")
	}
}

func TestService_DeleteUserIdentity_UserNotFound(t *testing.T) {
	userRepo := &svcMockUserRepo{} // returns NotFound by default
	svc := Service{UOW: svcTestUnitOfWork{users: userRepo}}
	err := svc.DeleteUserIdentity(context.Background(), "org-1", "user-nonexistent", "identity-1")
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// DeleteUserRoleBinding tests
// ---------------------------------------------------------------------------

func TestService_DeleteUserRoleBinding_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID: "user-1",
				RoleBindings: []userdomain.UserRoleBinding{
					{UserID: "user-1", RoleID: "role-1"},
					{UserID: "user-1", RoleID: "role-2"},
				},
			}, nil
		},
	}

	svc := Service{Clock: svcFakeClock{fixedTime: now}, UOW: svcTestUnitOfWork{users: userRepo}}
	err := svc.DeleteUserRoleBinding(context.Background(), "org-1", "user-1", "role-1")
	if err != nil {
		t.Fatalf("DeleteUserRoleBinding() error = %v", err)
	}
}

func TestService_DeleteUserRoleBinding_EmptyUserID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.DeleteUserRoleBinding(context.Background(), "org-1", "", "role-1")
	if err == nil {
		t.Fatal("expected error for empty userID, got nil")
	}
}

func TestService_DeleteUserRoleBinding_EmptyRoleID(t *testing.T) {
	svc := Service{UOW: svcTestUnitOfWork{}}
	err := svc.DeleteUserRoleBinding(context.Background(), "org-1", "user-1", "")
	if err == nil {
		t.Fatal("expected error for empty roleID, got nil")
	}
}

func TestService_DeleteUserRoleBinding_UserNotFound(t *testing.T) {
	userRepo := &svcMockUserRepo{} // returns NotFound
	svc := Service{UOW: svcTestUnitOfWork{users: userRepo}}
	err := svc.DeleteUserRoleBinding(context.Background(), "org-1", "user-nonexistent", "role-1")
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// DeleteUserGroupMembership tests
// ---------------------------------------------------------------------------

func TestService_DeleteUserGroupMembership_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID: "user-1",
				GroupMemberships: []userdomain.UserGroupMembership{
					{UserID: "user-1", GroupID: "group-1"},
					{UserID: "user-1", GroupID: "group-2"},
				},
			}, nil
		},
	}

	svc := Service{Clock: svcFakeClock{fixedTime: now}, UOW: svcTestUnitOfWork{users: userRepo}}
	err := svc.DeleteUserGroupMembership(context.Background(), "org-1", "user-1", "group-1")
	if err != nil {
		t.Fatalf("DeleteUserGroupMembership() error = %v", err)
	}
}

func TestService_DeleteUserGroupMembership_UserNotFound(t *testing.T) {
	userRepo := &svcMockUserRepo{} // returns NotFound
	svc := Service{UOW: svcTestUnitOfWork{users: userRepo}}
	err := svc.DeleteUserGroupMembership(context.Background(), "org-1", "user-nonexistent", "group-1")
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// ReplaceUserIdentities tests
// ---------------------------------------------------------------------------

func TestService_ReplaceUserIdentities_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{ID: "user-1", OrganizationID: "org-1"}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UUID:  svcFakeUUID{fixedID: "id-123"},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.ReplaceUserIdentities(context.Background(), "org-1", "user-1", []ReplaceUserIdentityCommand{
		{ProviderID: "provider-1", IdentityType: "oauth", Subject: "sub-1"},
	})
	if err != nil {
		t.Fatalf("ReplaceUserIdentities() error = %v", err)
	}
}

func TestService_ReplaceUserIdentities_UserNotFound(t *testing.T) {
	userRepo := &svcMockUserRepo{} // returns NotFound
	svc := Service{
		Clock: svcFakeClock{},
		UUID:  svcFakeUUID{fixedID: "id-123"},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.ReplaceUserIdentities(context.Background(), "org-1", "user-nonexistent", []ReplaceUserIdentityCommand{
		{ProviderID: "provider-1", IdentityType: "oauth", Subject: "sub-1"},
	})
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// ReplaceUserRoleBindings tests
// ---------------------------------------------------------------------------

func TestService_ReplaceUserRoleBindings_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{ID: "user-1", OrganizationID: "org-1"}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo, roles: svcAnyRoleRepo{}},
	}

	err := svc.ReplaceUserRoleBindings(context.Background(), "org-1", "user-1", []ReplaceUserRoleBindingCommand{
		{RoleID: "role-1"},
		{RoleID: "role-2"},
	})
	if err != nil {
		t.Fatalf("ReplaceUserRoleBindings() error = %v", err)
	}
}

func TestService_ReplaceUserRoleBindings_UserNotFound(t *testing.T) {
	userRepo := &svcMockUserRepo{} // returns NotFound
	svc := Service{
		Clock: svcFakeClock{},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.ReplaceUserRoleBindings(context.Background(), "org-1", "user-nonexistent", []ReplaceUserRoleBindingCommand{
		{RoleID: "role-1"},
	})
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// ReplaceUserGroupMemberships tests
// ---------------------------------------------------------------------------

func TestService_ReplaceUserGroupMemberships_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{ID: "user-1", OrganizationID: "org-1"}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo, groups: svcAnyGroupRepo{}},
	}

	err := svc.ReplaceUserGroupMemberships(context.Background(), "org-1", "user-1", []ReplaceUserGroupMembershipCommand{
		{GroupID: "group-1"},
		{GroupID: "group-2"},
	})
	if err != nil {
		t.Fatalf("ReplaceUserGroupMemberships() error = %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReplaceUserMfaMethods tests
// ---------------------------------------------------------------------------

func TestService_ReplaceUserMfaMethods_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID:               "user-1",
				Revision:         1,
				PreferredMfaType: "totp",
				MfaMethods: []userdomain.UserMfaMethod{
					{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true},
				},
			}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UUID:  svcFakeUUID{fixedID: "id-123"},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "totp", Enabled: true, DisplayName: "Authenticator"},
	})
	if err != nil {
		t.Fatalf("ReplaceUserMfaMethods() error = %v", err)
	}
}

func TestService_ReplaceUserMfaMethods_ClearPreferredMfa(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID:               "user-1",
				Revision:         1,
				PreferredMfaType: "sms",
				MfaMethods: []userdomain.UserMfaMethod{
					{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true},
				},
			}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UUID:  svcFakeUUID{fixedID: "id-123"},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	// Replace with only "totp" - preferred "sms" is no longer enabled, should be cleared
	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "totp", Enabled: true, DisplayName: "Authenticator"},
	})
	if err != nil {
		t.Fatalf("ReplaceUserMfaMethods() error = %v", err)
	}
}

func newReplaceMfaService(revision int64, methods []userdomain.UserMfaMethod) (Service, *userdomain.User) {
	user := &userdomain.User{ID: "user-1", Revision: revision, MfaMethods: methods}
	repo := &svcMockUserRepo{
		getByIDFn: func(context.Context, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UUID:  svcFakeUUID{fixedID: "generated-id"},
		UOW: svcTestUnitOfWork{
			users: repo,
			organizations: &svcMockOrgRepo{
				getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
					return &orgdomain.Organization{ID: id, Slug: "built-in", DisplayName: "Built In"}, nil
				},
			},
		},
	}
	return svc, user
}

func assertMfaConflict(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T (%v)", err, err)
	}
	if appErr.Code != apperrors.CodeConflict || appErr.HTTPStatus() != 409 {
		t.Fatalf("code = %q status = %d, want conflict/409", appErr.Code, appErr.HTTPStatus())
	}
}

func TestService_ReplaceUserMfaMethods_RequiresRevision(t *testing.T) {
	svc, _ := newReplaceMfaService(1, nil)
	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 0, []ReplaceUserMfaMethodCommand{
		{MethodType: "totp", Enabled: false},
	})
	assertMfaConflict(t, err)
}

func TestService_ReplaceUserMfaMethods_StaleRevisionConflict(t *testing.T) {
	svc, _ := newReplaceMfaService(2, []userdomain.UserMfaMethod{
		{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true},
	})
	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "totp", Enabled: true},
	})
	assertMfaConflict(t, err)
}

func TestService_ReplaceUserMfaMethods_RejectsClientVerified(t *testing.T) {
	svc, user := newReplaceMfaService(1, []userdomain.UserMfaMethod{
		{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true},
	})
	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "totp", Enabled: true, Verified: true},
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected 422 unprocessable entity, got %v", err)
	}
	if user.Revision != 1 {
		t.Fatalf("revision = %d, want unchanged 1", user.Revision)
	}
}

func TestService_ReplaceUserMfaMethods_RejectsEnablingUnverified(t *testing.T) {
	svc, _ := newReplaceMfaService(1, []userdomain.UserMfaMethod{
		{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: false, Verified: false},
	})
	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "totp", Enabled: true},
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected 422 unprocessable entity, got %v", err)
	}
}

func TestService_ReplaceUserMfaMethods_RejectsUnsupportedMethodType(t *testing.T) {
	svc, user := newReplaceMfaService(1, nil)
	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{MethodType: "u2f", Enabled: false},
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected 422 unprocessable entity, got %v", err)
	}
	if user.Revision != 1 {
		t.Fatalf("revision = %d, want unchanged 1", user.Revision)
	}
}

func TestService_ReplaceUserMfaMethods_AcceptsAppAlias(t *testing.T) {
	svc, _ := newReplaceMfaService(1, []userdomain.UserMfaMethod{
		{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true},
	})
	if err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "app", Enabled: true},
	}); err != nil {
		t.Fatalf("ReplaceUserMfaMethods() with app alias error = %v", err)
	}
}

func TestService_ReplaceUserMfaMethods_RevisionIncrementsAndReplayIsRejected(t *testing.T) {
	svc, user := newReplaceMfaService(1, []userdomain.UserMfaMethod{
		{ID: "method-1", UserID: "user-1", MethodType: "totp", Enabled: true, Verified: true},
	})
	if err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "totp", Enabled: true},
	}); err != nil {
		t.Fatalf("first ReplaceUserMfaMethods() error = %v", err)
	}
	if user.Revision != 2 {
		t.Fatalf("revision = %d, want 2 after a successful write", user.Revision)
	}

	// The same revision is now stale: a concurrent tab replaying its snapshot
	// must be rejected instead of overwriting the newer state.
	err := svc.ReplaceUserMfaMethods(context.Background(), "org-1", "user-1", 1, []ReplaceUserMfaMethodCommand{
		{ID: "method-1", MethodType: "totp", Enabled: true},
	})
	assertMfaConflict(t, err)
}

// ---------------------------------------------------------------------------
// UpdateUserProfile tests
// ---------------------------------------------------------------------------
func TestService_UpdateUserProfile_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID: "user-1",
				Profile: userdomain.UserProfile{
					UserID:        "user-1",
					EmailVerified: true,
					PhoneVerified: true,
				},
			}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		FirstName:      "Alice",
		LastName:       "Smith",
		Email:          "alice@example.com",
	})
	if err != nil {
		t.Fatalf("UpdateUserProfile() error = %v", err)
	}
}

// PUT /profile 的证件字段必须原样落到仓储 profile（列贯通，不再静默丢弃）。
func TestService_UpdateUserProfile_PersistsCertificateFields(t *testing.T) {
	now := time.Now()
	var savedProfile *userdomain.UserProfile
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{ID: "user-1", Profile: userdomain.UserProfile{UserID: "user-1"}}, nil
		},
		updateProfileFn: func(ctx context.Context, profile *userdomain.UserProfile) error {
			savedProfile = profile
			return nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID:        "org-1",
		UserID:                "user-1",
		PassportNo:            "E12345678",
		DriverLicenseNo:       "D1234567",
		IDCardExpiry:          "2030-12-31",
		IssueCountry:          "CN",
		IssuingAuthority:      "MPS Beijing",
		PassportPhotoURL:      "https://cdn.example.com/passport.png",
		DriverLicenseFrontURL: "https://cdn.example.com/dl-front.png",
		DriverLicenseBackURL:  "https://cdn.example.com/dl-back.png",
	})
	if err != nil {
		t.Fatalf("UpdateUserProfile() error = %v", err)
	}
	if savedProfile == nil {
		t.Fatal("UpdateProfile was not called")
	}
	if savedProfile.PassportNo != "E12345678" || savedProfile.DriverLicenseNo != "D1234567" {
		t.Fatalf("certificate numbers not persisted: %+v", savedProfile)
	}
	if savedProfile.IDCardExpiry != "2030-12-31" || savedProfile.IssueCountry != "CN" || savedProfile.IssuingAuthority != "MPS Beijing" {
		t.Fatalf("expiry/authority fields not persisted: %+v", savedProfile)
	}
	if savedProfile.PassportPhotoURL == "" || savedProfile.DriverLicenseFrontURL == "" || savedProfile.DriverLicenseBackURL == "" {
		t.Fatalf("document photo URLs not persisted: %+v", savedProfile)
	}
}

func TestService_UpdateUserProfile_UserNotFound(t *testing.T) {
	userRepo := &svcMockUserRepo{} // returns NotFound
	svc := Service{
		Clock: svcFakeClock{},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-nonexistent",
	})
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

// profile.properties 的系统保留键（karma/ranking/homepage 等）不是自定义属性：
// 新建租户没有任何 definitions 时，PUT /profile 不得因这些键返回 422；
// 未声明的自定义键仍必须被拒绝（fail-closed）。
func TestService_UpdateUserProfile_SystemKeysExemptWhenNoDefinitions(t *testing.T) {
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{ID: "user-1", Profile: userdomain.UserProfile{UserID: "user-1"}}, nil
		},
	}
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UOW: customAttributeTestUnitOfWork{
			svcTestUnitOfWork: svcTestUnitOfWork{users: userRepo},
			definitions:       &fakeDefinitionRepo{},
		},
	}

	err := svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		DisplayName:    "Guoxin",
		Properties:     map[string]any{"karma": 0, "ranking": 0, "homepage": "https://example.com"},
	})
	if err != nil {
		t.Fatalf("system keys must be exempt from tenant definitions: %v", err)
	}

	err = svc.UpdateUserProfile(context.Background(), UpdateUserProfileCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		DisplayName:    "Guoxin",
		Properties:     map[string]any{"undeclaredCustom": "x"},
	})
	if err == nil {
		t.Fatal("undeclared custom property must still be rejected when the tenant has no definitions")
	}
}

// ---------------------------------------------------------------------------
// UpdateUserPreferredMfa tests
// ---------------------------------------------------------------------------

func TestService_UpdateUserPreferredMfa_Success(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID:               "user-1",
				PreferredMfaType: "sms",
				MfaMethods: []userdomain.UserMfaMethod{
					{MethodType: "totp", Enabled: true},
				},
				Revision: 1,
			}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.UpdateUserPreferredMfa(context.Background(), UpdateUserPreferredMfaCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
		Revision:       1,
	})
	if err != nil {
		t.Fatalf("UpdateUserPreferredMfa() error = %v", err)
	}
}

func TestService_UpdateUserPreferredMfa_RevisionMismatch(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID:       "user-1",
				Revision: 2,
				MfaMethods: []userdomain.UserMfaMethod{
					{MethodType: "totp", Enabled: true},
				},
			}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.UpdateUserPreferredMfa(context.Background(), UpdateUserPreferredMfaCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
		Revision:       1, // mismatch
	})
	if err == nil {
		t.Fatal("expected error for revision mismatch, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeConflict {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeConflict)
	}
}

func TestService_UpdateUserPreferredMfa_MethodNotEnabled(t *testing.T) {
	now := time.Now()
	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return &userdomain.User{
				ID: "user-1",
				MfaMethods: []userdomain.UserMfaMethod{
					{MethodType: "totp", Enabled: false},
				},
			}, nil
		},
	}

	svc := Service{
		Clock: svcFakeClock{fixedTime: now},
		UOW:   svcTestUnitOfWork{users: userRepo},
	}

	err := svc.UpdateUserPreferredMfa(context.Background(), UpdateUserPreferredMfaCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
		MfaType:        "totp",
	})
	if err == nil {
		t.Fatal("expected error when MFA method not enabled, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
	}
}

// ---------------------------------------------------------------------------
// Validation tests for MFA setup commands
// ---------------------------------------------------------------------------

func Test_validateInitiateUserMfaSetupCommand(t *testing.T) {
	tests := []struct {
		name    string
		command InitiateUserMfaSetupCommand
		wantErr bool
	}{
		{
			name:    "valid command",
			command: InitiateUserMfaSetupCommand{UserID: "user-1", MfaType: "totp"},
			wantErr: false,
		},
		{
			name:    "empty userID",
			command: InitiateUserMfaSetupCommand{MfaType: "totp"},
			wantErr: true,
		},
		{
			name:    "empty mfaType",
			command: InitiateUserMfaSetupCommand{UserID: "user-1"},
			wantErr: true,
		},
		{
			name:    "whitespace userID",
			command: InitiateUserMfaSetupCommand{UserID: "   ", MfaType: "totp"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInitiateUserMfaSetupCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateInitiateUserMfaSetupCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateVerifyUserMfaSetupCommand(t *testing.T) {
	tests := []struct {
		name    string
		command VerifyUserMfaSetupCommand
		wantErr bool
	}{
		{
			name:    "valid command",
			command: VerifyUserMfaSetupCommand{UserID: "user-1", MfaType: "totp", Passcode: "123456"},
			wantErr: false,
		},
		{
			name:    "empty userID",
			command: VerifyUserMfaSetupCommand{MfaType: "totp", Passcode: "123456"},
			wantErr: true,
		},
		{
			name:    "empty passcode",
			command: VerifyUserMfaSetupCommand{UserID: "user-1", MfaType: "totp"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVerifyUserMfaSetupCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVerifyUserMfaSetupCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
