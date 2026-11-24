package jwt

import (
	"testing"
	"time"

	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

// BenchmarkJWTIssueAccessToken benchmarks JWT access token issuance.
// Spec §4.1: JWT签发 < 10ms.
func BenchmarkJWTIssueAccessToken(b *testing.B) {
	jwksManager := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(jwksManager)

	// Pre-warm: generate the RSA key pair so benchmark measures signing only
	params := IssueAccessTokenParams{
		Issuer:         "https://iam.guliang.tech",
		Subject:        "user-12345678-1234-1234-1234-123456789012",
		Audience:       []string{"app-built-in"},
		TTL:            time.Hour,
		JWTID:          "jti-abc123",
		Scope:          []string{"openid", "profile", "email"},
		ClientID:       "client-built-in",
		OrganizationID: "org-12345678-1234-1234-1234-123456789012",
		Username:       "admin",
	}

	// Warm up key generation
	_, _ = issuer.IssueAccessToken(params)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := issuer.IssueAccessToken(params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJWTIssueIDToken benchmarks JWT ID token issuance.
func BenchmarkJWTIssueIDToken(b *testing.B) {
	jwksManager := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(jwksManager)

	params := IssueIDTokenParams{
		Issuer:             "https://iam.guliang.tech",
		Subject:            "user-12345678-1234-1234-1234-123456789012",
		Audience:           []string{"app-built-in"},
		Nonce:              "nonce-abc123",
		AccessTokenHash:    "ath-hash123",
		AuthenticationTime: time.Now(),
		JWTID:              "jti-id-token",
		Name:               "Admin User",
		Email:              "admin@guliang.tech",
	}

	// Warm up key generation
	_, _ = issuer.IssueIDToken(params)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := issuer.IssueIDToken(params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJWTValidateToken benchmarks JWT token validation.
// Spec §4.1: Session恢复 < 20ms → token validation should be < 1ms.
func BenchmarkJWTValidateToken(b *testing.B) {
	jwksManager := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(jwksManager)

	params := IssueAccessTokenParams{
		Issuer:         "https://iam.guliang.tech",
		Subject:        "user-12345678-1234-1234-1234-123456789012",
		Audience:       []string{"app-built-in"},
		TTL:            time.Hour,
		JWTID:          "jti-validate-bench",
		Scope:          []string{"openid", "profile"},
		ClientID:       "client-built-in",
		OrganizationID: "org-12345678-1234-1234-1234-123456789012",
		Username:       "admin",
	}

	token, err := issuer.IssueAccessToken(params)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := issuer.ValidateToken(token)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJWTValidateWithIssuerCheck benchmarks JWT validation with issuer check.
func BenchmarkJWTValidateWithIssuerCheck(b *testing.B) {
	jwksManager := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(jwksManager)

	params := IssueAccessTokenParams{
		Issuer:         "https://iam.guliang.tech",
		Subject:        "user-12345678-1234-1234-1234-123456789012",
		Audience:       []string{"app-built-in"},
		TTL:            time.Hour,
		JWTID:          "jti-iss-check",
		Scope:          []string{"openid"},
		ClientID:       "client-built-in",
		OrganizationID: "org-12345678-1234-1234-1234-123456789012",
		Username:       "admin",
	}

	token, err := issuer.IssueAccessToken(params)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := issuer.ValidateTokenWithConfig(token, ValidationConfig{
			ExpectedIssuer: "https://iam.guliang.tech",
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkJWTFullCycle benchmarks the complete issue → validate cycle.
func BenchmarkJWTFullCycle(b *testing.B) {
	jwksManager := platformjwks.NewManager(2048, nil)
	issuer := NewIssuer(jwksManager)

	params := IssueAccessTokenParams{
		Issuer:         "https://iam.guliang.tech",
		Subject:        "user-12345678-1234-1234-1234-123456789012",
		Audience:       []string{"app-built-in"},
		TTL:            time.Hour,
		JWTID:          "jti-full-cycle",
		Scope:          []string{"openid", "profile"},
		ClientID:       "client-built-in",
		OrganizationID: "org-12345678-1234-1234-1234-123456789012",
		Username:       "admin",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		token, err := issuer.IssueAccessToken(params)
		if err != nil {
			b.Fatal(err)
		}
		_, err = issuer.ValidateToken(token)
		if err != nil {
			b.Fatal(err)
		}
	}
}
