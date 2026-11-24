package auth

import (
	"context"
	"fmt"
	"net/http"
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
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-D-2 / BE-D-3 回归：refresh_tokens.family_id 带自引用 FK，首代必须
// FamilyID=自身ID（独立随机 UUID 会 23503；NULL 会让 reuse 家族吊销失效）。
// 这里用递增 UUID 生成器，使「自引用」与「另生成一个家族 ID」可区分。

type familySeqUUIDGenerator struct {
	n int
}

func (g *familySeqUUIDGenerator) NewString() (string, error) {
	g.n++
	return fmt.Sprintf("00000000-0000-7000-8000-%012d", g.n), nil
}

// familyAuthCodeRepository 是 authorizationcodes.Repository 的最小内存替身。
type familyAuthCodeRepository struct {
	code      *authorizationcodedomain.AuthorizationCode
	usedIDs   []string
	createdAt time.Time
}

func (r *familyAuthCodeRepository) GetByCode(_ context.Context, code string) (*authorizationcodedomain.AuthorizationCode, error) {
	if r.code != nil && r.code.Code == code {
		return r.code, nil
	}
	return nil, apperrors.NewNotFound("authorization code not found")
}

func (r *familyAuthCodeRepository) Create(_ context.Context, code *authorizationcodedomain.AuthorizationCode) error {
	r.code = code
	return nil
}

func (r *familyAuthCodeRepository) MarkUsed(_ context.Context, id string, usedAt time.Time) error {
	r.usedIDs = append(r.usedIDs, id)
	if r.code != nil && r.code.ID == id {
		r.code.UsedAt = &usedAt
	}
	return nil
}

type familyExchangeUnitOfWork struct {
	serviceTestUnitOfWork
	codeRepo *familyAuthCodeRepository
}

func (u familyExchangeUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository {
	return u.codeRepo
}

func (u familyExchangeUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func assertRefreshFamilySelfRoot(t *testing.T, created []*tokendomain.RefreshToken) {
	t.Helper()
	require.Len(t, created, 1, "exactly one refresh token must be issued")
	rt := created[0]
	assert.NotEmpty(t, rt.FamilyID, "family_id must not be NULL: reuse detection needs a family root")
	assert.Equal(t, rt.ID, rt.FamilyID, "first generation must be its own family root (self-referencing FK)")
}

// 授权码流（offline_access + refresh_token grant）。
func TestExchangeAuthorizationCode_RefreshFamilyIsSelfRoot(t *testing.T) {
	now := time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC)
	verifier := "family-test-code-verifier-0123456789-0123456789"
	app := &applicationdomain.Application{
		ID:                      "app-family",
		OrganizationID:          "org-1",
		Slug:                    "family-app",
		ClientID:                "family-client",
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
	tokenRepo := &testTokenRepository{}
	uow := familyExchangeUnitOfWork{
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
			tokenRepo: tokenRepo,
		},
		codeRepo: codeRepo,
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: now},
		UUIDGenerator: &familySeqUUIDGenerator{},
		UOW:           uow,
	}

	resp, err := svc.ExchangeAuthorizationCode(context.Background(), ExchangeAuthorizationCodeCommand{
		GrantType:    grantAuthorizationCode,
		ClientID:     app.ClientID,
		Code:         "plain-code",
		RedirectURI:  "https://portal.example.com/callback",
		CodeVerifier: verifier,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.RefreshToken)
	assertRefreshFamilySelfRoot(t, tokenRepo.createdRefresh)
}

// RFC 8693 本地主体换发路径。
func TestTokenExchange_RefreshFamilyIsSelfRoot(t *testing.T) {
	service, tokenRepo := exchangeTestService(t)
	service.UUIDGenerator = &familySeqUUIDGenerator{}

	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.SubjectToken = "gl_pat_family-own-token"
	})
	tokenRepo.tokensByHash[hashTokenValueForRevocation("gl_pat_family-own-token")] = &tokendomain.Token{
		ID:              "token-family-own",
		UserID:          "user-1",
		Username:        "jane",
		OrganizationID:  "org-1",
		ApplicationID:   "app-caller",
		ApplicationSlug: "caller",
		Scopes:          []string{"openid"},
		ExpiresAt:       time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC),
	}

	response, err := service.GetTokenExchangeToken(context.Background(), command)
	require.NoError(t, err)
	require.NotEmpty(t, response.RefreshToken)
	assertRefreshFamilySelfRoot(t, tokenRepo.createdRefresh)
}

// ADR-038 步骤 1：jwt-bearer 断言通过客户端认证后仍封堵签发（主体模型仅支持
// 用户）。即使 GrantTypes 含 refresh_token，也不得签发任何 access/refresh
// token——空 UserID 会在写库时 22P02，且 R1 限定 refresh 仅用户绑定流程。
func TestGetJwtBearerToken_BlockedAfterClientAuthentication(t *testing.T) {
	rsaKey := testAssertionRSAKey(t)
	svc, repo, app, uow := newAssertionService(t)
	app.GrantTypes = []string{jwtBearerGrantType, oauthRefreshToken}
	registerClientJWK(t, repo, testOrgID, testAppJWKKid, "RS256", rsaPublicJWKJSON(t, rsaKey, testAppJWKKid, "RS256"))

	tokenRepo := &testTokenRepository{}
	svc.UOW = serviceTestUnitOfWork{loginTestUnitOfWork: uow, tokenRepo: tokenRepo}
	svc.UUIDGenerator = &familySeqUUIDGenerator{}

	now := svc.Clock.Now().UTC()
	assertion := signClientAssertion(t, "RS256", testAppJWKKid,
		assertionClaims(app.ClientID, now.Add(2*time.Minute), nil), rsaKey)

	resp, err := svc.GetJwtBearerToken(context.Background(), GetJwtBearerTokenCommand{
		ClientAssertionType: ClientAssertionTypeJWT,
		ClientAssertion:     assertion,
		Scope:               "openid",
		TokenEndpointURL:    testTokenEndpointURL,
	})
	require.Nil(t, resp)
	requireClientCredentialsProtocolError(t, err, "unsupported_grant_type", http.StatusBadRequest)
	assert.Empty(t, tokenRepo.tokens, "no access token may be persisted for a blocked grant")
	assert.Empty(t, tokenRepo.createdRefresh, "R1: jwt-bearer must never issue refresh tokens")
}
