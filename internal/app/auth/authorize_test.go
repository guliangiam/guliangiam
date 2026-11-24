package auth

import (
	"context"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	uowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	appmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	authorizationcodesdomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	authorizationcodesmock "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes/mock"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	consentmock "github.com/guliangiam/guliangiam/internal/domain/consents/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// ─── resolveAuthorizeContext 测试辅助 ──────────────────────────────────

type resolveAuthorizeSetup struct {
	Org     *orgdomain.Organization
	OrgErr  error
	User    *userdomain.User
	UserErr error
	App     *applicationdomain.Application
	AppErr  error
}

func runResolveSetup(ctrl *gomock.Controller, s resolveAuthorizeSetup) *uowmock.MockAuthUoW {
	uow := uowmock.NewMockAuthUoW(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)
	appRepo := appmock.NewMockRepository(ctrl)

	uow.EXPECT().Organizations().Return(orgRepo)
	orgRepo.EXPECT().GetBySlug(gomock.Any(), "acme").Return(s.Org, s.OrgErr)

	if s.Org != nil && s.OrgErr == nil {
		uow.EXPECT().Users().Return(userRepo)
		userRepo.EXPECT().GetByUsername(gomock.Any(), s.Org.ID, "alice").Return(s.User, s.UserErr)
		if s.User != nil && s.UserErr == nil {
			uow.EXPECT().Applications().Return(appRepo)
			appRepo.EXPECT().GetByClientID(gomock.Any(), "client-abc").Return(s.App, s.AppErr)
		}
	}
	return uow
}

func newAuthorizeTestSvc(uow *uowmock.MockAuthUoW) Service {
	return Service{
		Clock:         clock.SystemClock{},
		UUIDGenerator: platformuuid.V7Generator{},
		UOW:           uow,
	}
}

// ═══════════════════════════════════════════════════════════════════════
// validateAuthorizeScopes
// ═══════════════════════════════════════════════════════════════════════

func TestValidateAuthorizeScopes_Empty_ReturnsEmpty(t *testing.T) {
	result, err := validateAuthorizeScopes(nil, nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestValidateAuthorizeScopes_WhitespaceOnlyScopes_Dropped(t *testing.T) {
	result, err := validateAuthorizeScopes([]string{"", "  ", "   "}, nil)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestValidateAuthorizeScopes_DefaultScopes_Accepted(t *testing.T) {
	result, err := validateAuthorizeScopes([]string{"openid", "profile", "email"}, nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"openid", "profile", "email"}, result)
}

func TestValidateAuthorizeScopes_CustomScopes_Accepted(t *testing.T) {
	custom := []applicationdomain.ApplicationCustomScope{
		{Scope: "my_custom_scope"},
		{Scope: "  another_scope  "},
		{Scope: ""},
	}
	result, err := validateAuthorizeScopes([]string{"my_custom_scope", "another_scope"}, custom)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"my_custom_scope", "another_scope"}, result)
}

func TestValidateAuthorizeScopes_UnknownScope_ReturnsError(t *testing.T) {
	_, err := validateAuthorizeScopes([]string{"totally_unknown"}, nil)
	require.Error(t, err)
}

// ═══════════════════════════════════════════════════════════════════════
// resolveAuthorizeContext
// ═══════════════════════════════════════════════════════════════════════

func TestResolveAuthorizeContext_OrgNotFound_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{Org: nil, OrgErr: assert.AnError}))

	_, _, _, _, err := svc.resolveAuthorizeContext(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
	})
	require.Error(t, err)
}

func TestResolveAuthorizeContext_UserNotFound_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{
		Org: org, OrgErr: nil, User: nil, UserErr: assert.AnError,
	}))

	_, _, _, _, err := svc.resolveAuthorizeContext(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
	})
	require.Error(t, err)
}

func TestResolveAuthorizeContext_AppNotFound_ReturnsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{
		Org: org, OrgErr: nil, User: user, UserErr: nil,
		App: nil, AppErr: assert.AnError,
	}))

	_, _, _, _, err := svc.resolveAuthorizeContext(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
	})
	require.Error(t, err)
}

func TestResolveAuthorizeContext_AppOrgMismatch_ReturnsInvalidArgument(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	app := &applicationdomain.Application{
		ID: "app-1", OrganizationID: "org-2", ClientID: "client-abc",
		GrantTypes:   []string{"authorization_code"},
		RedirectURIs: []string{"https://app.example.com/callback"},
	}
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{
		Org: org, OrgErr: nil, User: user, UserErr: nil,
		App: app, AppErr: nil,
	}))

	_, _, _, _, err := svc.resolveAuthorizeContext(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to organization")
}

func TestResolveAuthorizeContext_RedirectURINotRegistered_ReturnsInvalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	app := &applicationdomain.Application{
		ID: "app-1", OrganizationID: "org-1", ClientID: "client-abc",
		GrantTypes:   []string{"authorization_code"},
		RedirectURIs: []string{"https://app.example.com/other"},
	}
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{
		Org: org, OrgErr: nil, User: user, UserErr: nil,
		App: app, AppErr: nil,
	}))

	_, _, _, _, err := svc.resolveAuthorizeContext(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
		RedirectURI: "https://app.example.com/callback",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redirectUri is not registered")
}

func TestResolveAuthorizeContext_NoAuthCodeGrant_ReturnsInvalid(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	app := &applicationdomain.Application{
		ID: "app-1", OrganizationID: "org-1", ClientID: "client-abc",
		GrantTypes:   []string{"client_credentials"},
		RedirectURIs: []string{"https://app.example.com/callback"},
	}
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{
		Org: org, OrgErr: nil, User: user, UserErr: nil,
		App: app, AppErr: nil,
	}))

	_, _, _, _, err := svc.resolveAuthorizeContext(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
		RedirectURI: "https://app.example.com/callback",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not allow authorization_code grant")
}

func TestResolveAuthorizeContext_ValidContext_Succeeds(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	org := &orgdomain.Organization{ID: "org-1", Slug: "acme", DisplayName: "Acme Inc"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	app := &applicationdomain.Application{
		ID: "app-1", OrganizationID: "org-1", ClientID: "client-abc", Slug: "acme-app",
		GrantTypes:   []string{"authorization_code"},
		RedirectURIs: []string{"https://app.example.com/callback"},
	}
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{
		Org: org, OrgErr: nil, User: user, UserErr: nil,
		App: app, AppErr: nil,
	}))

	o, u, a, scopes, err := svc.resolveAuthorizeContext(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
		RedirectURI:     "https://app.example.com/callback",
		RequestedScopes: []string{"openid", "profile"},
	})
	require.NoError(t, err)
	assert.Equal(t, org, o)
	assert.Equal(t, user, u)
	assert.Equal(t, app, a)
	assert.ElementsMatch(t, []string{"openid", "profile"}, scopes)
}

// ═══════════════════════════════════════════════════════════════════════
// Authorize — 仅早返回路径（避免 WithinTransaction + AuthorizationCodes）
// ═══════════════════════════════════════════════════════════════════════

func TestAuthorize_EmptyOrgSlug_ReturnsValidationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	svc := newAuthorizeTestSvc(uowmock.NewMockAuthUoW(ctrl))
	result, err := svc.Authorize(context.Background(), AuthorizeCommand{
		Username: "a", ClientID: "c", RedirectURI: "https://x.com",
	})
	require.Error(t, err)
	assert.Nil(t, result)
}

func TestAuthorize_ResolveContextOrgError_BubblesUp(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	// org repo returns error → validateAuthorizeCommand 已过，但 resolveAuthorizeContext 失败
	svc := newAuthorizeTestSvc(runResolveSetup(ctrl, resolveAuthorizeSetup{
		Org: nil, OrgErr: assert.AnError,
	}))
	result, err := svc.Authorize(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
		RedirectURI: "https://app.example.com/callback",
	})
	require.Error(t, err)
	assert.Nil(t, result)
}

// ═══════════════════════════════════════════════════════════════════════
// Authorize — consent_required 路径（WithinTransaction 之前返回）
// ═══════════════════════════════════════════════════════════════════════

func TestAuthorize_ConsentRequired_ReturnsEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	app := &applicationdomain.Application{
		ID: "app-1", OrganizationID: "org-1", ClientID: "client-abc", Slug: "acme-app",
		GrantTypes:   []string{"authorization_code"},
		RedirectURIs: []string{"https://app.example.com/callback"},
		CustomScopes: []applicationdomain.ApplicationCustomScope{
			{Scope: "custom:admin"},
		},
	}

	// runResolveSetup + Consents mock（Revoked → grantedScopes 空）
	uow := runResolveSetup(ctrl, resolveAuthorizeSetup{Org: org, User: user, App: app})
	consentRepo := consentmock.NewMockRepository(ctrl)
	uow.EXPECT().Consents().Return(consentRepo)
	revokedAt := time.Now().Add(-time.Hour)
	consentRepo.EXPECT().GetByUserAndApplication(gomock.Any(), "user-1", "app-1").Return(
		&consentdomain.Consent{GrantedScopes: []string{"email"}, RevokedAt: &revokedAt}, nil,
	)

	svc := newAuthorizeTestSvc(uow)
	result, err := svc.Authorize(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
		RedirectURI:     "https://app.example.com/callback",
		RequestedScopes: []string{"openid", "profile", "custom:admin"},
		CodeChallenge:   "abc1234567890123456789012345678901234567890123456789012345678901",
		ChallengeMethod: "S256",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "consent_required", result.Status)
	require.NotNil(t, result.Evaluation)
	assert.Equal(t, "app-1", result.Evaluation.ApplicationID)
	assert.Equal(t, "acme-app", result.Evaluation.ApplicationSlug)
	assert.ElementsMatch(t, []string{"openid", "profile", "custom:admin"}, result.Evaluation.RequestedScopes)
	// consentableScopes 只保留 custom scope → missingScopes 必有 custom:admin
	assert.Contains(t, result.Evaluation.MissingScopes, "custom:admin")
	assert.NotContains(t, result.Evaluation.MissingScopes, "openid")
	assert.NotContains(t, result.Evaluation.MissingScopes, "profile")
}

// ═══════════════════════════════════════════════════════════════════════
// Authorize — 成功路径（WithinTransaction + AuthorizationCodes.Create）
// ═══════════════════════════════════════════════════════════════════════

func TestAuthorize_Success_ReturnsAuthorizedCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	app := &applicationdomain.Application{
		ID: "app-1", OrganizationID: "org-1", ClientID: "client-abc", Slug: "acme-app",
		GrantTypes:   []string{"authorization_code"},
		RedirectURIs: []string{"https://app.example.com/callback"},
	}

	// resolveAuthorizeContext 正常通过
	uow := runResolveSetup(ctrl, resolveAuthorizeSetup{Org: org, User: user, App: app})

	// getActiveGrantedScopes — 无 custom scope → consentableScopes 空 → missingScopes 空
	consentRepo := consentmock.NewMockRepository(ctrl)
	uow.EXPECT().Consents().Return(consentRepo)
	consentRepo.EXPECT().GetByUserAndApplication(gomock.Any(), "user-1", "app-1").Return(
		&consentdomain.Consent{GrantedScopes: []string{"openid", "profile"}}, nil,
	)

	// WithinTransaction → fullTx → AuthorizationCodes.Create
	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	codesRepo := authorizationcodesmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().AuthorizationCodes().Return(codesRepo)
	codesRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, code *authorizationcodesdomain.AuthorizationCode) error {
			require.NotEmpty(t, code.ID)
			require.NotEmpty(t, code.Code)
			assert.Equal(t, "user-1", code.UserID)
			assert.Equal(t, "app-1", code.ApplicationID)
			assert.Equal(t, "https://app.example.com/callback", code.RedirectURI)
			assert.Equal(t, "state-xyz", code.State)
			return nil
		})

	svc := newAuthorizeTestSvc(uow)
	result, err := svc.Authorize(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
		RedirectURI:     "https://app.example.com/callback",
		RequestedScopes: []string{"openid", "profile"},
		CodeChallenge:   "abc1234567890123456789012345678901234567890123456789012345678901",
		ChallengeMethod: "S256",
		State:           "state-xyz",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "authorized", result.Status)
	require.NotNil(t, result.Code)
	assert.NotEmpty(t, result.Code.Code)
	assert.Equal(t, "https://app.example.com/callback", result.Code.RedirectURI)
	assert.Equal(t, "state-xyz", result.Code.State)
	assert.NotEmpty(t, result.Code.RedirectTo)
	assert.Contains(t, result.Code.RedirectTo, "https://app.example.com/callback")
	assert.Contains(t, result.Code.RedirectTo, result.Code.Code)
	assert.NotNil(t, result.Code.ExpiresAt)
}

// ═══════════════════════════════════════════════════════════════════════
// Authorize — transaction 内 AuthorizationCodes.Create 失败 → WrapInternal
// ═══════════════════════════════════════════════════════════════════════

func TestAuthorize_CreateCodeError_WrapsInternal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	org := &orgdomain.Organization{ID: "org-1", Slug: "acme"}
	user := &userdomain.User{ID: "user-1", OrganizationID: "org-1", Username: "alice"}
	app := &applicationdomain.Application{
		ID: "app-1", OrganizationID: "org-1", ClientID: "client-abc", Slug: "acme-app",
		GrantTypes:   []string{"authorization_code"},
		RedirectURIs: []string{"https://app.example.com/callback"},
	}

	uow := runResolveSetup(ctrl, resolveAuthorizeSetup{Org: org, User: user, App: app})
	consentRepo := consentmock.NewMockRepository(ctrl)
	uow.EXPECT().Consents().Return(consentRepo)
	consentRepo.EXPECT().GetByUserAndApplication(gomock.Any(), "user-1", "app-1").Return(
		&consentdomain.Consent{GrantedScopes: []string{"openid"}}, nil,
	)

	fullTx := uowmock.NewMockUnitOfWork(ctrl)
	codesRepo := authorizationcodesmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, fullTx)
		})
	fullTx.EXPECT().AuthorizationCodes().Return(codesRepo)
	codesRepo.EXPECT().Create(gomock.Any(), gomock.Any()).Return(assert.AnError)

	svc := newAuthorizeTestSvc(uow)
	_, err := svc.Authorize(context.Background(), AuthorizeCommand{
		OrganizationSlug: "acme", Username: "alice", ClientID: "client-abc",
		RedirectURI:     "https://app.example.com/callback",
		RequestedScopes: []string{"openid"},
		CodeChallenge:   "abc1234567890123456789012345678901234567890123456789012345678901",
		ChallengeMethod: "S256",
	})
	require.Error(t, err)
	var target *apperrors.Error
	require.ErrorAs(t, err, &target)
	assert.Equal(t, apperrors.CodeInternal, target.Code)
	assert.Contains(t, err.Error(), "authorize transaction")
}
