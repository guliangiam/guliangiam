package auth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	infrajwt "github.com/guliangiam/guliangiam/internal/infra/jwt"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
)

// These tests pin AC-CRYPTO-002: an application's token_signing_method must
// decide the ID token's JOSE alg and, through the algorithm-specific key
// lookup, its kid prefix. A stub that only echoed the parameters would not
// catch a real issuer that silently signed RS256, so this file records the
// hand-off and delegates to the real JWT issuer (a test-only infra import,
// consistent with contract_test.go).
type recordingTokenIssuer struct {
	inner  tokendomain.TokenIssuer
	params []tokendomain.IssueIDTokenParams
}

func (r *recordingTokenIssuer) IssueIDToken(params tokendomain.IssueIDTokenParams) (string, error) {
	r.params = append(r.params, params)
	return r.inner.IssueIDToken(params)
}

func (r *recordingTokenIssuer) IssueAccessToken(params tokendomain.IssueAccessTokenParams) (string, error) {
	return r.inner.IssueAccessToken(params)
}

func (r *recordingTokenIssuer) ValidateToken(token string) (map[string]interface{}, error) {
	return r.inner.ValidateToken(token)
}

func idTokenSigningService() (*Service, *recordingTokenIssuer) {
	issuer := &recordingTokenIssuer{inner: infrajwt.NewIssuer(platformjwks.NewManager(2048, nil))}
	return &Service{TokenIssuer: issuer}, issuer
}

func buildIDTokenForSigningMethod(t *testing.T, service *Service, method string) (string, error) {
	t.Helper()
	return service.buildIDToken(
		&applicationdomain.Application{ClientID: "portal-client", TokenSigningMethod: method},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{
			Issuer:   "https://iam.acme.example",
			AuthTime: time.Now(),
			Scopes:   []string{"openid", "profile"},
		},
	)
}

func decodeTokenHeader(t *testing.T, token string) map[string]string {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d, want 3", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatalf("decode JOSE header: %v", err)
	}
	var header map[string]string
	if err := json.Unmarshal(raw, &header); err != nil {
		t.Fatalf("parse JOSE header: %v", err)
	}
	return header
}

func TestBuildIDToken_SigningAlgorithmFollowsApplication(t *testing.T) {
	cases := []struct {
		name          string
		method        string
		wantAlgorithm string
		wantKidPrefix string
	}{
		{"sm2-sm3", applicationdomain.TokenSigningMethodSM2SM3, "SM2-SM3", "gm_"},
		{"rs256", applicationdomain.TokenSigningMethodRS256, "RS256", "rsa_"},
		{"empty falls back to rs256", "", "RS256", "rsa_"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service, issuer := idTokenSigningService()

			token, err := buildIDTokenForSigningMethod(t, service, testCase.method)
			if err != nil {
				t.Fatalf("buildIDToken() error = %v", err)
			}
			if len(issuer.params) != 1 {
				t.Fatalf("issuer calls = %d, want 1", len(issuer.params))
			}
			// The application value must reach the issuer: an app set to SM2-SM3
			// whose issuer still receives "" would sign RS256 by the issuer default.
			if got := issuer.params[0].Algorithm; got != testCase.wantAlgorithm {
				t.Fatalf("issuer algorithm = %q, want %q (token_signing_method=%q)",
					got, testCase.wantAlgorithm, testCase.method)
			}

			header := decodeTokenHeader(t, token)
			if got := header["alg"]; got != testCase.wantAlgorithm {
				t.Fatalf("JWT header alg = %q, want %q", got, testCase.wantAlgorithm)
			}
			if kid := header["kid"]; !strings.HasPrefix(kid, testCase.wantKidPrefix) {
				t.Fatalf("JWT header kid = %q, want %q prefix", kid, testCase.wantKidPrefix)
			}
		})
	}
}

// TestBuildIDToken_WhitespaceSigningMethodIsRejectedUnchanged pins the boundary
// between "unset" and "set to whitespace". Only an exact empty string selects the
// RS256 default: whitespace is non-empty and outside the enum, so it must fail
// closed instead of being trimmed into the default. The issuer must also receive
// the value verbatim so that a future normalization in the auth layer shows up as
// a behaviour change rather than a silent RS256 downgrade.
func TestBuildIDToken_WhitespaceSigningMethodIsRejectedUnchanged(t *testing.T) {
	service, issuer := idTokenSigningService()
	const method = "   "

	token, err := buildIDTokenForSigningMethod(t, service, method)
	if err == nil {
		t.Fatalf("buildIDToken with token_signing_method %q returned a token instead of failing closed", method)
	}
	if token != "" {
		t.Fatalf("buildIDToken with token_signing_method %q returned token %q alongside an error", method, token)
	}
	if len(issuer.params) != 1 {
		t.Fatalf("issuer calls = %d, want 1", len(issuer.params))
	}
	if got := issuer.params[0].Algorithm; got != method {
		t.Fatalf("issuer algorithm = %q, want the untrimmed application value %q", got, method)
	}
}

func TestBuildIDToken_RejectsUnknownSigningAlgorithm(t *testing.T) {
	cases := map[string]string{
		"forbidden":   "HS256",
		"unsupported": "SM2-SM3-TEST",
	}

	for name, method := range cases {
		t.Run(name, func(t *testing.T) {
			service, issuer := idTokenSigningService()

			token, err := buildIDTokenForSigningMethod(t, service, method)
			if err == nil {
				t.Fatalf("buildIDToken(%q) returned a token instead of rejecting the algorithm", method)
			}
			if token != "" {
				t.Fatalf("buildIDToken(%q) returned token %q alongside an error", method, token)
			}
			// The rejection must come from the configured value itself, not from a
			// silent rewrite to RS256: the issuer saw exactly what the application
			// declared.
			if len(issuer.params) != 1 || issuer.params[0].Algorithm != method {
				t.Fatalf("issuer saw %v, want the application value %q passed through unchanged",
					issuer.params, method)
			}
		})
	}
}
