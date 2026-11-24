package auth

import (
	"context"
	"testing"
	"time"

	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	domainworkload "github.com/guliangiam/guliangiam/internal/domain/workload"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WLI 短期凭据回归：联邦（workload identity）主体换发的凭据必须短寿命，
// 不得签发 30 天 Refresh Token —— 否则外部 IdP 的会话约束被本地刷新令牌架空。
func TestTokenExchange_FederatedSubjectGetsNoRefreshToken(t *testing.T) {
	service, tokenRepo := exchangeTestService(t)

	resolver := newStubTrustResolver()
	resolver.Add(&domainworkload.FederationTrust{
		ID:             "ft-1",
		OrganizationID: "org-1",
		Issuer:         "https://accounts.okta.com",
		Audience:       []string{"guliangiam"},
		Subjects:       []string{"okta-user-abc"},
		Status:         domainworkload.TrustStatusActive,
	})
	service.federationResolver = resolver
	service.federationVerifier = &stubJWTVerifier{
		verifiedClaims: map[string]interface{}{
			"iss":   "https://accounts.okta.com",
			"sub":   "okta-user-abc",
			"aud":   []interface{}{"guliangiam"},
			"exp":   float64(time.Now().Add(time.Hour).Unix()),
			"scope": "openid profile",
		},
	}

	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.SubjectTokenType = SubjectTokenTypeJWT
		c.SubjectToken = makeFederationJWT("https://accounts.okta.com", "okta-user-abc", time.Now().Add(time.Hour))
	})

	response, err := service.GetTokenExchangeToken(context.Background(), command)
	require.NoError(t, err)
	assert.NotEmpty(t, response.AccessToken, "federated exchange should still issue a short-lived access token")
	assert.Empty(t, response.RefreshToken, "federated/workload subjects must not receive refresh tokens")
	assert.Empty(t, tokenRepo.createdRefresh, "no refresh token row may be persisted for federated subjects")
}

func TestTokenExchange_FederatedTrustMustMatchCallerOrganization(t *testing.T) {
	service, tokenRepo := exchangeTestService(t)

	resolver := newStubTrustResolver()
	resolver.Add(&domainworkload.FederationTrust{
		ID: "ft-other-org", OrganizationID: "org-2", Issuer: "https://accounts.okta.com",
		Audience: []string{"guliangiam"}, Subjects: []string{"okta-user-abc"},
		Status: domainworkload.TrustStatusActive,
	})
	service.federationResolver = resolver
	verifyCalls := 0
	service.federationVerifier = &stubJWTVerifier{verifyFunc: func(_ string, _ *domainworkload.FederationTrust) (map[string]interface{}, error) {
		verifyCalls++
		return map[string]interface{}{
			"iss": "https://accounts.okta.com", "sub": "okta-user-abc",
			"aud": "guliangiam", "exp": float64(time.Now().Add(time.Hour).Unix()),
		}, nil
	}}

	response, err := service.GetTokenExchangeToken(context.Background(), exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.SubjectTokenType = SubjectTokenTypeJWT
		c.SubjectToken = makeFederationJWT("https://accounts.okta.com", "okta-user-abc", time.Now().Add(time.Hour))
	}))
	assert.Nil(t, response)
	require.Error(t, err)
	assert.Zero(t, verifyCalls, "another organization's JWKS and trust must not be used")
	assert.Empty(t, tokenRepo.tokens)
}

// 对照组：本地（交互式）主体保持既有 Refresh Token 流程 —— 修复按主体类型区分，
// 不能全局删除其他 OAuth 流程的 Refresh Token。
func TestTokenExchange_LocalSubjectStillGetsRefreshToken(t *testing.T) {
	service, tokenRepo := exchangeTestService(t)

	command := exchangeCommand(func(c *GetTokenExchangeCommand) {
		c.SubjectToken = "gl_pat_own-token"
	})
	tokenRepo.tokensByHash[platformcrypto.HashTokenValue("gl_pat_own-token")] = &tokendomain.Token{
		ID:              "token-own",
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
	assert.NotEmpty(t, response.RefreshToken, "local subjects keep the standard refresh flow")
	assert.Len(t, tokenRepo.createdRefresh, 1)
}

// isFederatedSubject 判定边界：nil、普通 token、fed: 前缀、federation 标记。
func TestIsFederatedSubject(t *testing.T) {
	assert.False(t, isFederatedSubject(nil))
	assert.False(t, isFederatedSubject(&tokendomain.Token{ID: "token-own"}))
	assert.True(t, isFederatedSubject(&tokendomain.Token{ID: "fed:okta-user-abc"}))
	assert.True(t, isFederatedSubject(&tokendomain.Token{
		ID:       "token-x",
		Metadata: map[string]any{"federation": true},
	}))
}
