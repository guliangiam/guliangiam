package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	domainworkload "github.com/guliangiam/guliangiam/internal/domain/workload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Stub FederationTrustResolver ──

type stubTrustResolver struct {
	trusts          map[string][]*domainworkload.FederationTrust
	includeInactive bool
}

func newStubTrustResolver() *stubTrustResolver {
	return &stubTrustResolver{trusts: make(map[string][]*domainworkload.FederationTrust)}
}

func (s *stubTrustResolver) Add(trust *domainworkload.FederationTrust) {
	s.trusts[trust.Issuer] = append(s.trusts[trust.Issuer], trust)
}

func (s *stubTrustResolver) FindActiveTrustsByIssuer(ctx context.Context, issuer string) ([]*domainworkload.FederationTrust, error) {
	items := s.trusts[issuer]
	active := make([]*domainworkload.FederationTrust, 0, len(items))
	for _, trust := range items {
		if s.includeInactive || trust.Status == domainworkload.TrustStatusActive {
			active = append(active, trust)
		}
	}
	return active, nil
}

// ── Stub ExternalJWTVerifier ──

type stubJWTVerifier struct {
	verifiedClaims map[string]interface{}
	verifyErr      error
	verifyFunc     func(tokenString string, trust *domainworkload.FederationTrust) (map[string]interface{}, error)
}

func (s *stubJWTVerifier) Verify(tokenString string, trust *domainworkload.FederationTrust) (map[string]interface{}, error) {
	if s.verifyFunc != nil {
		return s.verifyFunc(tokenString, trust)
	}
	if s.verifyErr != nil {
		return nil, s.verifyErr
	}
	if s.verifiedClaims != nil {
		return s.verifiedClaims, nil
	}
	audience := "guliangiam"
	if len(trust.Audience) > 0 {
		audience = trust.Audience[0]
	}
	return map[string]interface{}{
		"iss": trust.Issuer,
		"sub": "external-user-42",
		"aud": []interface{}{audience},
		"exp": float64(time.Now().Add(time.Hour).Unix()),
	}, nil
}

// makeFederationJWT 构造一个"合法三部分"的 JWT tokenString。
// stub verifier 不验证签名，我们只需要 parseJWTClaimsUnverified 能正确拿到 iss。
func makeFederationJWT(iss, sub string, exp time.Time) string {
	header, _ := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT", "kid": "k1"})
	payload, _ := json.Marshal(map[string]any{
		"iss": iss,
		"sub": sub,
		"aud": []string{"guliangiam"},
		"exp": exp.Unix(),
	})
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + ".fakesig"
}

// ═══════════════════════════════════════════════
// resolveJWTSubjectToken federation 分支测试
// ═══════════════════════════════════════════════

func TestResolveJWTSubject_FederationFound(t *testing.T) {
	resolver := newStubTrustResolver()
	resolver.Add(&domainworkload.FederationTrust{
		ID:       "ft-1",
		Issuer:   "https://accounts.okta.com",
		JWKSURI:  "https://accounts.okta.com/oauth2/v1/keys",
		Audience: []string{"guliangiam"},
		Subjects: []string{"okta-user-abc"},
		Status:   domainworkload.TrustStatusActive,
	})

	verifier := &stubJWTVerifier{
		verifiedClaims: map[string]interface{}{
			"iss":   "https://accounts.okta.com",
			"sub":   "okta-user-abc",
			"aud":   []interface{}{"guliangiam"},
			"exp":   float64(time.Now().Add(time.Hour).Unix()),
			"scope": "openid email profile",
		},
	}

	s := Service{
		federationResolver: resolver,
		federationVerifier: verifier,
	}

	token := makeFederationJWT("https://accounts.okta.com", "okta-user-abc", time.Now().Add(time.Hour))
	subject, err := s.resolveJWTSubjectToken(context.Background(), token)

	require.NoError(t, err)
	require.NotNil(t, subject)
	assert.Equal(t, "okta-user-abc", subject.UserID, "federation sub → subject.UserID")
	assert.Contains(t, subject.Scopes, "openid")
	assert.True(t, subject.Metadata["federation"].(bool))
}

func TestResolveJWTSubject_FederationNotFound_FallsBack(t *testing.T) {
	resolver := newStubTrustResolver() // 空
	verifier := &stubJWTVerifier{}

	s := Service{
		federationResolver: resolver,
		federationVerifier: verifier,
	}

	token := makeFederationJWT("https://unclaimed.example.com", "some-sub", time.Now().Add(time.Hour))
	_, err := s.resolveJWTSubjectToken(context.Background(), token)
	assert.Error(t, err, "降级到本地路径后因为没有 jti 而失败")
}

func TestResolveJWTSubject_FederationDisabled_Skips(t *testing.T) {
	// resolver+verifier 都是 nil → 完全跳过 federation lookup
	s := Service{}

	token := makeFederationJWT("https://any.example.com", "any-sub", time.Now().Add(time.Hour))
	_, err := s.resolveJWTSubjectToken(context.Background(), token)
	assert.Error(t, err, "没有 federation 注入 → 应该走本地路径 → 失败")
}

func TestResolveJWTSubject_FederationVerifierFails(t *testing.T) {
	resolver := newStubTrustResolver()
	resolver.Add(&domainworkload.FederationTrust{
		Issuer:   "https://bad-issuer.example.com",
		Audience: []string{"guliangiam"},
		Subjects: []string{"user-x"},
		Status:   domainworkload.TrustStatusActive,
	})

	verifier := &stubJWTVerifier{verifyErr: assert.AnError}

	s := Service{
		federationResolver: resolver,
		federationVerifier: verifier,
	}

	token := makeFederationJWT("https://bad-issuer.example.com", "user-x", time.Now().Add(time.Hour))
	_, err := s.resolveJWTSubjectToken(context.Background(), token)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "federation JWT verification failed")
}
