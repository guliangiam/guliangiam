package tokens

import (
	"context"
	"errors"
	"testing"
	"time"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	adapterdomain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	agentdomain "github.com/guliangiam/guliangiam/internal/domain/agents"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	captchaproviderdomain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/domain/castickets"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	coupondomain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	entrydomain "github.com/guliangiam/guliangiam/internal/domain/entries"
	faceidproviderdomain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	formdomain "github.com/guliangiam/guliangiam/internal/domain/forms"
	gatewaydomain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	idvdomain "github.com/guliangiam/guliangiam/internal/domain/idv"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	keysdomain "github.com/guliangiam/guliangiam/internal/domain/keys"
	ldapdomain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	mcpserverdomain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	meteringdomain "github.com/guliangiam/guliangiam/internal/domain/metering"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	notificationproviderdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationsdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	orderdomain "github.com/guliangiam/guliangiam/internal/domain/orders"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	paymentdomain "github.com/guliangiam/guliangiam/internal/domain/payments"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	plandomain "github.com/guliangiam/guliangiam/internal/domain/plans"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	pricingdomain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	radiusaccountingdomain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	radiusconfigdomain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	rebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	resourcedomain "github.com/guliangiam/guliangiam/internal/domain/resources"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"
	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
	ruledomain "github.com/guliangiam/guliangiam/internal/domain/rules"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	subscriptiondomain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	syncerdomain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	ticketdomain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	transactiondomain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/domain/web3identity"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	webhookeventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// CreateToken tests
// ---------------------------------------------------------------------------

func TestService_CreateToken_Success(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*userdomain.User{
			{ID: "user-1", OrganizationID: "org-1", Username: "testuser"},
		},
	}
	appRepo := &mockAppRepo{
		apps: []*applications.Application{
			{ID: "app-1", OrganizationID: "org-1", Slug: "app-slug"},
		},
	}
	uow := &createTokenUOW{
		tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   uow,
	}

	result, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read", "write"},
		Audience:         []string{"app-1"},
		Metadata:         map[string]any{"key": "value"},
		ExpiresInSeconds: 3600,
	})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	if result.TokenValue == "" {
		t.Error("TokenValue should not be empty")
	}
	if result.ID == "" {
		t.Error("TokenDetail.ID should not be empty")
	}
	if result.RefreshToken != "" {
		t.Error("RefreshToken should be empty when IssueRefreshToken is false")
	}
}

func TestService_CreateToken_WithRefreshToken(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*userdomain.User{
			{ID: "user-1", OrganizationID: "org-1", Username: "testuser"},
		},
	}
	appRepo := &mockAppRepo{
		apps: []*applications.Application{
			{ID: "app-1", OrganizationID: "org-1", Slug: "app-slug"},
		},
	}
	uow := &createTokenUOW{
		tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   uow,
	}

	result, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:          "org-1",
		UserID:                  "user-1",
		ApplicationID:           "app-1",
		Scopes:                  []string{"read"},
		ExpiresInSeconds:        3600,
		IssueRefreshToken:       true,
		RefreshExpiresInSeconds: 86400,
	})
	if err != nil {
		t.Fatalf("CreateToken() error = %v", err)
	}
	if result.RefreshToken == "" {
		t.Error("RefreshToken should not be empty when IssueRefreshToken is true")
	}
}

func TestService_CreateToken_UserNotInOrganization(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*userdomain.User{
			{ID: "user-1", OrganizationID: "org-different", Username: "testuser"},
		},
	}
	appRepo := &mockAppRepo{}
	uow := &createTokenUOW{
		tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   uow,
	}

	_, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read"},
		ExpiresInSeconds: 3600,
	})
	if err == nil {
		t.Fatal("expected error when user does not belong to organization, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}

func TestService_CreateToken_AppNotInOrganization(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*userdomain.User{
			{ID: "user-1", OrganizationID: "org-1", Username: "testuser"},
		},
	}
	appRepo := &mockAppRepo{
		apps: []*applications.Application{
			{ID: "app-1", OrganizationID: "org-different", Slug: "app-slug"},
		},
	}
	uow := &createTokenUOW{
		tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   uow,
	}

	_, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read"},
		ExpiresInSeconds: 3600,
	})
	if err == nil {
		t.Fatal("expected error when app does not belong to organization, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}

func TestService_CreateToken_OrgNotFound(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{} // no orgs
	userRepo := &mockUserRepo{}
	appRepo := &mockAppRepo{}
	uow := &createTokenUOW{
		tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   uow,
	}

	_, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read"},
		ExpiresInSeconds: 3600,
	})
	if err == nil {
		t.Fatal("expected error when org not found, got nil")
	}
}

func TestService_CreateToken_UserNotFound(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{} // no users
	appRepo := &mockAppRepo{}
	uow := &createTokenUOW{
		tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   uow,
	}

	_, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read"},
		ExpiresInSeconds: 3600,
	})
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

func TestService_CreateToken_AppNotFound(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*userdomain.User{
			{ID: "user-1", OrganizationID: "org-1", Username: "testuser"},
		},
	}
	appRepo := &mockAppRepo{} // no apps
	uow := &createTokenUOW{
		tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   uow,
	}

	_, err := svc.CreateToken(context.Background(), CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read"},
		ExpiresInSeconds: 3600,
	})
	if err == nil {
		t.Fatal("expected error when app not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// RefreshAccessToken success path tests
// ---------------------------------------------------------------------------

func TestService_RefreshAccessToken_Success(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_valid")
	accessTokenID := "at-1"

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    accessTokenID,
				UserID:           "user-1",
				Username:         "testuser",
				OrganizationID:   "org-1",
				OrganizationSlug: "org-slug",
				ApplicationID:    "app-1",
				ApplicationSlug:  "app-slug",
				RefreshTokenHash: tokenHash,
				FamilyID:         "family-1",
				Scopes:           []string{"read", "write"},
				Audience:         []string{"app-1"},
				ExpiresAt:        now.Add(24 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
				UpdatedAt:        now.Add(-1 * time.Hour),
			},
		},
		tokens: []*domain.Token{
			{
				ID:               accessTokenID,
				UserID:           "user-1",
				Username:         "testuser",
				OrganizationID:   "org-1",
				OrganizationSlug: "org-slug",
				ApplicationID:    "app-1",
				ApplicationSlug:  "app-slug",
				Scopes:           []string{"read", "write"},
				Audience:         []string{"app-1"},
				ExpiresAt:        now.Add(1 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
				UpdatedAt:        now.Add(-1 * time.Hour),
			},
		},
	}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*userdomain.User{
			{ID: "user-1", OrganizationID: "org-1", Username: "testuser"},
		},
	}
	appRepo := &mockAppRepo{
		apps: []*applications.Application{
			{ID: "app-1", OrganizationID: "org-1", Slug: "app-slug"},
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW: testUnitOfWork{
			tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: appRepo,
		},
	}

	result, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_valid",
	})
	if err != nil {
		t.Fatalf("RefreshAccessToken() error = %v", err)
	}
	if result.TokenValue == "" {
		t.Error("TokenValue should not be empty")
	}
	if result.RefreshToken == "" {
		t.Error("RefreshToken should not be empty")
	}
	if result.ID == "" {
		t.Error("TokenDetail.ID should not be empty")
	}
}

func TestService_RefreshAccessToken_ExpiredRefreshToken(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_expired")

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    "at-1",
				RefreshTokenHash: tokenHash,
				ExpiresAt:        now.Add(-1 * time.Hour), // expired
				RevokedAt:        nil,
				CreatedAt:        now.Add(-2 * time.Hour),
			},
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   testUnitOfWork{tokenRepo: repo},
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_expired",
	})
	if err == nil {
		t.Fatal("expected error for expired refresh token, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}

func TestService_RefreshAccessToken_RevokedRefreshToken_ReuseDetection(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_revoked")
	revokedAt := now.Add(-1 * time.Hour)

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    "at-1",
				RefreshTokenHash: tokenHash,
				FamilyID:         "family-1",
				ExpiresAt:        now.Add(24 * time.Hour),
				RevokedAt:        &revokedAt, // revoked
				CreatedAt:        now.Add(-2 * time.Hour),
			},
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   testUnitOfWork{tokenRepo: repo},
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_revoked",
	})
	if err == nil {
		t.Fatal("expected error for revoked (reused) refresh token, got nil")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeUnauthorized {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeUnauthorized)
	}
}

func TestService_RefreshAccessToken_RevokedAccessToken(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_revoked_access")
	revokedAt := now.Add(-30 * time.Minute)

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    "at-1",
				RefreshTokenHash: tokenHash,
				ExpiresAt:        now.Add(24 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
			},
		},
		tokens: []*domain.Token{
			{
				ID:        "at-1",
				RevokedAt: &revokedAt, // access token is revoked
				ExpiresAt: now.Add(1 * time.Hour),
				CreatedAt: now.Add(-2 * time.Hour),
			},
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   testUnitOfWork{tokenRepo: repo},
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_revoked_access",
	})
	if err == nil {
		t.Fatal("expected error when access token is revoked, got nil")
	}
}

func TestService_RefreshAccessToken_AccessTokenNotFound(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_no_access")

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    "at-nonexistent",
				RefreshTokenHash: tokenHash,
				ExpiresAt:        now.Add(24 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
			},
		},
		// no access tokens
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   testUnitOfWork{tokenRepo: repo},
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_no_access",
	})
	if err == nil {
		t.Fatal("expected error when access token not found, got nil")
	}
}

func TestService_RefreshAccessToken_OrgNotFound(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_no_org")

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    "at-1",
				RefreshTokenHash: tokenHash,
				ExpiresAt:        now.Add(24 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
			},
		},
		tokens: []*domain.Token{
			{
				ID:             "at-1",
				OrganizationID: "org-nonexistent",
				UserID:         "user-1",
				ExpiresAt:      now.Add(1 * time.Hour),
				RevokedAt:      nil,
				CreatedAt:      now.Add(-2 * time.Hour),
			},
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   testUnitOfWork{tokenRepo: repo, orgRepo: &mockOrgRepo{}}, // empty orgs
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_no_org",
	})
	if err == nil {
		t.Fatal("expected error when org not found, got nil")
	}
}

func TestService_RefreshAccessToken_UserNotFound(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_no_user")

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    "at-1",
				RefreshTokenHash: tokenHash,
				ExpiresAt:        now.Add(24 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
			},
		},
		tokens: []*domain.Token{
			{
				ID:             "at-1",
				OrganizationID: "org-1",
				UserID:         "user-nonexistent",
				ApplicationID:  "app-1",
				ExpiresAt:      now.Add(1 * time.Hour),
				RevokedAt:      nil,
				CreatedAt:      now.Add(-2 * time.Hour),
			},
		},
	}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   testUnitOfWork{tokenRepo: repo, orgRepo: orgRepo, userRepo: &mockUserRepo{}}, // no users
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_no_user",
	})
	if err == nil {
		t.Fatal("expected error when user not found, got nil")
	}
}

func TestService_RefreshAccessToken_AppNotFound(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_prt_no_app")

	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				AccessTokenID:    "at-1",
				RefreshTokenHash: tokenHash,
				ExpiresAt:        now.Add(24 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
			},
		},
		tokens: []*domain.Token{
			{
				ID:             "at-1",
				OrganizationID: "org-1",
				UserID:         "user-1",
				ApplicationID:  "app-nonexistent",
				ExpiresAt:      now.Add(1 * time.Hour),
				RevokedAt:      nil,
				CreatedAt:      now.Add(-2 * time.Hour),
			},
		},
	}
	orgRepo := &mockOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}
	userRepo := &mockUserRepo{
		users: []*userdomain.User{
			{ID: "user-1", OrganizationID: "org-1", Username: "testuser"},
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "uuid-1234"},
		UOW:   testUnitOfWork{tokenRepo: repo, orgRepo: orgRepo, userRepo: userRepo, appRepo: &mockAppRepo{}}, // no apps
	}

	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		RefreshToken: "gl_prt_no_app",
	})
	if err == nil {
		t.Fatal("expected error when app not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// Mapper tests
// ---------------------------------------------------------------------------

func Test_toDetail_NilToken(t *testing.T) {
	result := toDetail(nil)
	if result != nil {
		t.Errorf("toDetail(nil) = %v, want nil", result)
	}
}

func Test_toListItem_NilToken(t *testing.T) {
	result := toListItem(nil)
	if result.ID != "" || result.UserID != "" {
		t.Errorf("toListItem(nil) should return zero value, got ID=%q UserID=%q", result.ID, result.UserID)
	}
}

func Test_cloneMap_NilMap(t *testing.T) {
	result := cloneMap(nil)
	if len(result) != 0 {
		t.Errorf("cloneMap(nil) len = %d, want 0", len(result))
	}
}

func Test_cloneMap_WithEntries(t *testing.T) {
	original := map[string]any{"key1": "value1", "key2": 42}
	cloned := cloneMap(original)
	if len(cloned) != 2 {
		t.Errorf("cloneMap len = %d, want 2", len(cloned))
	}
	if cloned["key1"] != "value1" {
		t.Errorf("cloneMap[\"key1\"] = %v, want \"value1\"", cloned["key1"])
	}
	if cloned["key2"] != 42 {
		t.Errorf("cloneMap[\"key2\"] = %v, want 42", cloned["key2"])
	}
	// Verify it's a clone, not the same map
	cloned["key3"] = "value3"
	if _, ok := original["key3"]; ok {
		t.Error("cloneMap should return a copy, not the same map")
	}
}

// ---------------------------------------------------------------------------
// min helper test
// ---------------------------------------------------------------------------

func Test_minInt(t *testing.T) {
	tests := []struct {
		name     string
		value    int
		fallback int
		want     int
	}{
		{name: "value less than fallback", value: 3, fallback: 5, want: 3},
		{name: "value greater than fallback", value: 10, fallback: 5, want: 5},
		{name: "value equal to fallback", value: 5, fallback: 5, want: 5},
		{name: "zero value", value: 0, fallback: 5, want: 0},
		{name: "negative value", value: -1, fallback: 5, want: -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minInt(tt.value, tt.fallback)
			if got != tt.want {
				t.Errorf("minInt(%d, %d) = %d, want %d", tt.value, tt.fallback, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// revokeTokenFamilyByUoW tests
// ---------------------------------------------------------------------------

func Test_revokeTokenFamilyByUoW_EmptyFamilyID(t *testing.T) {
	err := revokeTokenFamilyByUoW(context.Background(), testUnitOfWork{}, "")
	if err != nil {
		t.Errorf("revokeTokenFamilyByUoW with empty familyID should return nil, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// createTokenUOW - a UOW that delegates to mock repos (reuses existing mock types)
// ---------------------------------------------------------------------------

type createTokenUOW struct {
	tokenRepo *mockTokenRepo
	orgRepo   *mockOrgRepo
	userRepo  *mockUserRepo
	appRepo   *mockAppRepo
}

func (u createTokenUOW) Applications() applications.Repository                        { return u.appRepo }
func (u createTokenUOW) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u createTokenUOW) Consents() consentdomain.Repository                           { return nil }
func (u createTokenUOW) Gateway() gatewaydomain.Repository                            { return nil }
func (u createTokenUOW) Groups() groupsdomain.Repository                              { return nil }
func (u createTokenUOW) Invitations() invitationsdomain.Repository                    { return nil }
func (u createTokenUOW) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u createTokenUOW) Organizations() orgdomain.Repository                          { return u.orgRepo }
func (u createTokenUOW) Permissions() permissiondomain.Repository                     { return nil }
func (u createTokenUOW) Providers() providerdomain.Repository                         { return nil }
func (u createTokenUOW) Certs() certdomain.Repository                                 { return nil }
func (u createTokenUOW) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u createTokenUOW) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u createTokenUOW) Notifications() notificationsdomain.Repository                { return nil }
func (u createTokenUOW) Ldaps() ldapdomain.Repository                                 { return nil }
func (u createTokenUOW) Models() modeldomain.Repository                               { return nil }
func (u createTokenUOW) Adapters() adapterdomain.Repository                           { return nil }
func (u createTokenUOW) Agents() agentdomain.Repository                               { return nil }
func (u createTokenUOW) Enforcers() enforcerdomain.Repository                         { return nil }
func (u createTokenUOW) Forms() formdomain.Repository                                 { return nil }
func (u createTokenUOW) Orders() orderdomain.Repository                               { return nil }
func (u createTokenUOW) Payments() paymentdomain.Repository                           { return nil }
func (u createTokenUOW) Plans() plandomain.Repository                                 { return nil }

func (u createTokenUOW) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u createTokenUOW) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u createTokenUOW) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u createTokenUOW) Pricings() pricingdomain.Repository                               { return nil }
func (u createTokenUOW) Products() productdomain.Repository                               { return nil }
func (u createTokenUOW) Records() recordsdomain.Repository                                { return nil }
func (u createTokenUOW) Resources() resourcedomain.Repository                             { return nil }
func (u createTokenUOW) Roles() roledomain.Repository                                     { return nil }
func (u createTokenUOW) Syncers() syncerdomain.Repository                                 { return nil }
func (u createTokenUOW) Sessions() sessiondomain.Repository                               { return nil }
func (u createTokenUOW) Subscriptions() subscriptiondomain.Repository                     { return nil }
func (u createTokenUOW) Tickets() ticketdomain.Repository                                 { return nil }
func (u createTokenUOW) Tokens() domain.Repository                                        { return u.tokenRepo }
func (u createTokenUOW) Webhooks() webhookdomain.Repository                               { return nil }
func (u createTokenUOW) Users() userdomain.Repository                                     { return u.userRepo }
func (u createTokenUOW) Coupons() coupondomain.Repository                                 { return nil }
func (u createTokenUOW) CouponUsages() coupondomain.CouponUsageRepository                 { return nil }
func (u createTokenUOW) Entries() entrydomain.Repository                                  { return nil }
func (u createTokenUOW) McpServers() mcpserverdomain.Repository                           { return nil }
func (u createTokenUOW) RadiusAccountings() radiusaccountingdomain.Repository             { return nil }
func (u createTokenUOW) Verifications() verificationsdomain.Repository                    { return nil }
func (u createTokenUOW) WebAuthnCredentials() webauthndomain.Repository                   { return nil }
func (u createTokenUOW) FaceIdProviders() faceidproviderdomain.Repository                 { return nil }
func (u createTokenUOW) RadiusConfigs() radiusconfigdomain.Repository                     { return nil }
func (u createTokenUOW) Rules() ruledomain.Repository                                     { return nil }
func (u createTokenUOW) Transactions() transactiondomain.Repository                       { return nil }
func (u createTokenUOW) WebhookEvents() webhookeventdomain.Repository                     { return nil }
func (u createTokenUOW) Keys() keysdomain.Repository                                      { return nil }
func (u createTokenUOW) QRCodeSessions() qrcodesessions.Repository                        { return nil }
func (u createTokenUOW) DeviceAuthCodes() deviceauthcodes.Repository                      { return nil }

func (u createTokenUOW) Web3Identities() web3identity.Repository     { return nil }
func (u createTokenUOW) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u createTokenUOW) CASTickets() castickets.Repository           { return nil }
func (u createTokenUOW) KerberosTickets() kerberostickets.Repository { return nil }
func (u createTokenUOW) LockoutAttempts() lockoutattempts.Repository { return nil }

func (u createTokenUOW) Risk() riskdomain.Repository { return nil }
func (u createTokenUOW) IGA() igadomain.Repository   { return nil }

func (u createTokenUOW) Workload() workloaddomain.Repository { return nil }

func (u createTokenUOW) Rebac() rebac.Repository { return nil }

func (u createTokenUOW) IDV() idvdomain.Repository { return nil }

func (u createTokenUOW) Actions() actionsdomain.Repository { return nil }

func (u createTokenUOW) Outbox() domainoutbox.Repository { return nil }

func (u createTokenUOW) Metering() meteringdomain.Repository { return nil }

func (u createTokenUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func TestValidateCreateTokenCommand_SemanticRules(t *testing.T) {
	base := CreateTokenCommand{
		OrganizationID:   "org-1",
		UserID:           "user-1",
		ApplicationID:    "app-1",
		Scopes:           []string{"read"},
		ExpiresInSeconds: 3600,
	}

	zeroTTL := base
	zeroTTL.ExpiresInSeconds = 0
	appErr := apperrors.FromError(validateCreateTokenCommand(zeroTTL))
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("error code = %q, want 422", appErr.Code)
	}
	if appErr.Meta["field"] != "expiresInSeconds" || appErr.Meta["min"] != 1 {
		t.Fatalf("meta = %v, want field=expiresInSeconds min=1", appErr.Meta)
	}

	noScopes := base
	noScopes.Scopes = []string{}
	appErr = apperrors.FromError(validateCreateTokenCommand(noScopes))
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("error code = %q, want 422", appErr.Code)
	}
	if appErr.Meta["field"] != "scopes" {
		t.Fatalf("meta = %v, want field=scopes", appErr.Meta)
	}
}

func TestService_RefreshAccessToken_ApplicationScopeMatch(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{{
			ID: "rt-1", AccessTokenID: "at-1", UserID: "user-1", OrganizationID: "org-1",
			ApplicationID: "app-1", RefreshTokenHash: hashTokenValue("gl_prt_app"), FamilyID: "family-1",
			ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
		}},
		tokens: []*domain.Token{{
			ID: "at-1", UserID: "user-1", OrganizationID: "org-1", ApplicationID: "app-1",
			ExpiresAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now,
		}},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "uuid-1"}, UOW: testUnitOfWork{
		tokenRepo: repo,
		orgRepo:   &mockOrgRepo{orgs: []*orgdomain.Organization{{ID: "org-1", Slug: "org-slug"}}},
		userRepo:  &mockUserRepo{users: []*userdomain.User{{ID: "user-1", OrganizationID: "org-1", Username: "u"}}},
		appRepo:   &mockAppRepo{apps: []*applications.Application{{ID: "app-1", OrganizationID: "org-1", Slug: "app-slug"}}},
	}}
	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		OrganizationID: "org-1", ApplicationID: "app-1", RefreshToken: "gl_prt_app",
	})
	if err != nil {
		t.Fatalf("RefreshAccessToken() error = %v", err)
	}
}

func TestService_RefreshAccessToken_ApplicationMismatchIsRejected(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{{
			ID: "rt-1", AccessTokenID: "at-1", OrganizationID: "org-1", ApplicationID: "app-1",
			RefreshTokenHash: hashTokenValue("gl_prt_other"), ExpiresAt: now.Add(time.Hour),
		}},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{tokenRepo: repo}}
	_, err := svc.RefreshAccessToken(context.Background(), RefreshAccessTokenCommand{
		OrganizationID: "org-1", ApplicationID: "app-2", RefreshToken: "gl_prt_other",
	})
	if err == nil {
		t.Fatal("expected error for refresh token owned by another application")
	}
}
