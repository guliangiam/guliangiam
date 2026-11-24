package users

import (
	"context"
	"testing"
	"time"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// C08：删除路径的平台管理员目标保护 + 自删守卫
// ---------------------------------------------------------------------------

func deleteGuardsService(uow testUnitOfWork) *Service {
	return &Service{
		Clock: fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UOW:   uow,
	}
}

func TestDeleteUser_SelfDeleteRejected(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			user := newTestUser()
			return user, nil
		},
	}
	uow := testUnitOfWork{users: userRepo, platformAdmins: platformAdminNotFound()}

	err := deleteGuardsService(uow).DeleteUser(context.Background(), DeleteUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		ActorID:        testUserID,
	})
	requireCode(t, err, apperrors.CodeUnprocessableEntity)
}

func TestDeleteUser_PlatformAdminTargetRejectedForNonPlatformActor(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return newTestUser(), nil
		},
	}
	uow := testUnitOfWork{
		users: userRepo,
		platformAdmins: stubPlatformAdminRepo{admin: &platformadmins.PlatformAdmin{
			ID: "pa-1", UserID: testUserID, IsActive: true,
		}},
	}

	err := deleteGuardsService(uow).DeleteUser(context.Background(), DeleteUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		ActorID:        "019operator-0000-7000-8000-000000000002",
	})
	// 非平台管理员调用方 → 404，不暴露目标存在性。
	requireCode(t, err, apperrors.CodeNotFound)
}

func TestDeleteUser_BootstrapPlatformAdminRejectedEvenForPlatformActor(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return newTestUser(), nil
		},
	}
	uow := testUnitOfWork{
		users: userRepo,
		platformAdmins: stubPlatformAdminRepo{admin: &platformadmins.PlatformAdmin{
			ID: "pa-1", UserID: testUserID, IsActive: true,
		}},
		platformAdminScope: stubPlatformAdminScopeRepo{scopes: []*platformadmins.PlatformAdminScope{
			{PlatformAdminID: "pa-1", IsBootstrap: true},
		}},
	}

	err := deleteGuardsService(uow).DeleteUser(context.Background(), DeleteUserCommand{
		OrganizationID:       testOrganizationID,
		ID:                   testUserID,
		ActorID:              "019operator-0000-7000-8000-000000000002",
		ActorIsPlatformAdmin: true,
	})
	// bootstrap 平台管理员即使由平台管理员调用也 404。
	requireCode(t, err, apperrors.CodeNotFound)
}

// ---------------------------------------------------------------------------
// C09：GetUser 回填 webauthnCredentialCount / consentSummary
// ---------------------------------------------------------------------------

func TestGetUser_BackfillsWebauthnAndConsentCounts(t *testing.T) {
	user := newTestUser()
	user.ApplicationScopes = []userdomain.ConsentRecord{
		{Application: "app-1", Scopes: []string{"read"}, GrantedAt: "2026-01-01T00:00:00Z"},
		{Application: "app-2", Scopes: []string{"write"}, GrantedAt: "2026-02-01T00:00:00Z"},
	}
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(context.Context, string, string) (*userdomain.User, error) {
			return user, nil
		},
	}
	orgRepo := &mockOrgRepo{
		getByIDFn: func(context.Context, string) (*orgdomain.Organization, error) {
			return newTestOrganization(), nil
		},
	}
	webauthn := &waMockRepo{
		listByUserIDFn: func(_ context.Context, userID string) ([]*webauthndomain.Credential, error) {
			require.Equal(t, testUserID, userID)
			return []*webauthndomain.Credential{{}, {}, {}}, nil
		},
	}
	uow := testUnitOfWork{users: userRepo, organizations: orgRepo, webauthn: webauthn}

	detail, err := deleteGuardsService(uow).GetUser(context.Background(), testUserID, testOrganizationID)
	require.NoError(t, err)
	assert.Equal(t, 3, detail.WebauthnCredentialCount)
	assert.Equal(t, 2, detail.ConsentSummary["totalConsents"])
	assert.Equal(t, 2, detail.ConsentSummary["activeConsents"])
}

// ---------------------------------------------------------------------------
// C12：PATCH 空白 status/userType → 422（规范 4.14.1）
// ---------------------------------------------------------------------------

func TestValidatePatchUserCommand_BlankFieldsReturn422(t *testing.T) {
	blank := " "
	for _, cmd := range []PatchUserCommand{
		{UserType: &blank},
		{Status: &blank},
	} {
		err := validatePatchUserCommand(cmd)
		requireCode(t, err, apperrors.CodeUnprocessableEntity)
	}
}
