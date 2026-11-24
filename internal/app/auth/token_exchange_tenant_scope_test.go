package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// INC-BE-05 回归：authorization_code 交换必须是两段式 —— System 只读定位/
// 客户端认证事务 + 回钉应用租户的读写事务；整个交换不得跑在 System 作用域。

type scopeRecordingUnitOfWork struct {
	familyExchangeUnitOfWork
	scopes []tenantctx.TenantScope
}

func (u *scopeRecordingUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	u.scopes = append(u.scopes, tenantctx.TenantScopeFromContext(ctx))
	return fn(ctx, u)
}

func TestExchangeAuthorizationCode_TwoPhaseTenantScopes(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	verifier := "tenant-scope-test-verifier-0123456789-0123456789"
	app := &applicationdomain.Application{
		ID:                      "app-scope",
		OrganizationID:          "org-1",
		Slug:                    "scope-app",
		ClientID:                "scope-client",
		TokenEndpointAuthMethod: applicationdomain.TokenEndpointAuthMethodNone,
		GrantTypes:              []string{grantAuthorizationCode, oauthRefreshToken},
	}
	codeRepo := &familyAuthCodeRepository{code: &authorizationcodedomain.AuthorizationCode{
		ID:              "code-1",
		Code:            "plain-code",
		UserID:          "user-1",
		OrganizationID:  "org-1",
		ApplicationID:   app.ID,
		RedirectURI:     "https://portal.example.com/callback",
		Scopes:          []string{scopeOfflineAccess},
		CodeChallenge:   buildPKCEChallenge(verifier),
		ChallengeMethod: pkceChallengeMethodS256,
		ExpiresAt:       now.Add(5 * time.Minute),
	}}
	uow := &scopeRecordingUnitOfWork{
		familyExchangeUnitOfWork: familyExchangeUnitOfWork{
			serviceTestUnitOfWork: serviceTestUnitOfWork{
				loginTestUnitOfWork: loginTestUnitOfWork{
					appRepo: &loginTestAppRepository{
						apps:       map[string]*applicationdomain.Application{app.ID: app},
						byClientID: map[string]*applicationdomain.Application{app.ClientID: app},
					},
					orgRepo: &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{
						"org-1": {ID: "org-1", Slug: "org-1"},
					}},
					userRepo: &loginTestUserRepository{users: map[string]*userdomain.User{
						"user-1": {ID: "user-1", OrganizationID: "org-1", Username: "jane"},
					}},
				},
				tokenRepo: &testTokenRepository{},
			},
			codeRepo: codeRepo,
		},
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: now},
		UUIDGenerator: &familySeqUUIDGenerator{},
		UOW:           uow,
	}

	_, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeCommand{
		GrantType:    grantAuthorizationCode,
		ClientID:     app.ClientID,
		Code:         "plain-code",
		RedirectURI:  "https://portal.example.com/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)
	require.Len(t, uow.scopes, 2, "exchange must run exactly two transactions")
	assert.Equal(t, tenantctx.TenantFilterSystem, uow.scopes[0].Mode,
		"phase 1 (client location + authentication) is a System read-only transaction")
	assert.Equal(t, tenantctx.TenantFilterStrict, uow.scopes[1].Mode,
		"phase 2 (code redemption + token writes) must be tenant-pinned, not System")
	assert.Equal(t, "org-1", uow.scopes[1].OrganizationID,
		"phase 2 must pin to the application's own organization")
}

// INC-BE-05 回归：GetUserInfo 的 token 定位用 System 只读，用户读取回钉
// token 自身租户。

type ctxRecordingUserRepository struct {
	*loginTestUserRepository
	gotCtx context.Context
}

func (r *ctxRecordingUserRepository) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	r.gotCtx = ctx
	return r.loginTestUserRepository.GetByID(ctx, id)
}

type userInfoScopeUnitOfWork struct {
	serviceTestUnitOfWork
	users *ctxRecordingUserRepository
}

func (u userInfoScopeUnitOfWork) Users() userdomain.Repository { return u.users }

func TestGetUserInfo_UserReadPinnedToTokenTenant(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	const accessToken = "userinfo-scope-token"
	users := &ctxRecordingUserRepository{
		loginTestUserRepository: &loginTestUserRepository{users: map[string]*userdomain.User{
			"user-1": {ID: "user-1", OrganizationID: "org-1", Username: "jane"},
		}},
	}
	uow := userInfoScopeUnitOfWork{
		serviceTestUnitOfWork: serviceTestUnitOfWork{
			tokenRepo: &testTokenRepository{
				tokensByHash: map[string]*tokendomain.Token{
					platformcrypto.HashTokenValue(accessToken): {
						ID:             "token-1",
						UserID:         "user-1",
						OrganizationID: "org-1",
						Scopes:         []string{scopeOpenID, scopeProfile},
						ExpiresAt:      now.Add(time.Hour),
					},
				},
			},
		},
		users: users,
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: uow}

	resp, err := svc.GetUserInfo(context.Background(), accessToken)
	require.NoError(t, err)
	assert.Equal(t, "user-1", resp.Sub)
	require.NotNil(t, users.gotCtx)
	scope := tenantctx.TenantScopeFromContext(users.gotCtx)
	assert.Equal(t, tenantctx.TenantFilterStrict, scope.Mode)
	assert.Equal(t, "org-1", scope.OrganizationID)
}
