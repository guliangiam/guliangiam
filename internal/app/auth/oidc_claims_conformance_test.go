package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// These tests pin 设计报告 14.8 / 14.9: the ID token's claim set, the issuer it
// declares, the authentication time it reports, the scope gating of profile
// claims, and the at_hash binding. Each assertion names the rule it protects.

// captureTokenIssuer records the parameters the service handed to the issuer, so
// the tests assert on what would have been signed rather than on a decoded
// string (there is no key material in these unit tests).
type captureTokenIssuer struct {
	idTokenParams []tokendomain.IssueIDTokenParams
}

func (c *captureTokenIssuer) IssueIDToken(params tokendomain.IssueIDTokenParams) (string, error) {
	c.idTokenParams = append(c.idTokenParams, params)
	return "signed-id-token", nil
}

func (c *captureTokenIssuer) IssueAccessToken(tokendomain.IssueAccessTokenParams) (string, error) {
	return "signed-access-token", nil
}

func (c *captureTokenIssuer) ValidateToken(string) (map[string]interface{}, error) {
	return nil, apperrors.NewUnauthorized("not implemented in test")
}

func idTokenTestUser() *userdomain.User {
	return &userdomain.User{
		ID:       "user-1",
		Username: "jane",
		Profile: userdomain.UserProfile{
			UserID:        "user-1",
			DisplayName:   "Jane Doe",
			FirstName:     "Jane",
			LastName:      "Doe",
			AvatarURL:     "https://cdn.example.com/jane.png",
			Email:         "jane@example.com",
			EmailVerified: true,
			Phone:         "+15551234567",
			PhoneVerified: false,
		},
	}
}

func TestBuildIDToken_UsesTheDeploymentIssuer(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}
	now := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	if _, err := service.buildIDToken(
		&applicationdomain.Application{ClientID: "portal-client"},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{
			Issuer:   "https://iam.acme.example",
			AuthTime: now,
			Scopes:   []string{"openid", "profile"},
		},
	); err != nil {
		t.Fatalf("buildIDToken() error = %v", err)
	}

	if len(issuer.idTokenParams) != 1 {
		t.Fatalf("expected exactly one ID token, got %d", len(issuer.idTokenParams))
	}
	// A hardcoded issuer made every deployment on another domain unusable.
	if got := issuer.idTokenParams[0].Issuer; got != "https://iam.acme.example" {
		t.Fatalf("iss = %q, want the deployment issuer", got)
	}
	if got := issuer.idTokenParams[0].Issuer; got == "guliangiam" {
		t.Fatal("iss fell back to the retired hardcoded literal")
	}
}

func TestBuildIDToken_RefusesMissingIssuerAndAuthTime(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}

	cases := map[string]idTokenIssuance{
		"missing issuer":    {AuthTime: time.Now(), Scopes: []string{"openid"}},
		"missing auth time": {Issuer: "https://iam.example.com", Scopes: []string{"openid"}},
	}
	for name, issuance := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := service.buildIDToken(
				&applicationdomain.Application{ClientID: "portal-client"},
				idTokenTestUser(),
				&orgdomain.Organization{Slug: "built-in"},
				issuance,
			); err == nil {
				t.Fatal("expected an error rather than a token with a wrong claim")
			}
		})
	}
}

func TestBuildIDToken_AuthTimeIsTheAuthenticationMoment(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}
	authenticatedAt := time.Date(2026, 9, 15, 9, 30, 0, 0, time.UTC)
	codeIssuedAt := authenticatedAt.Add(45 * time.Minute)

	if _, err := service.buildIDToken(
		&applicationdomain.Application{ClientID: "portal-client"},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{
			Issuer:   "https://iam.example.com",
			AuthTime: authenticatedAt,
			Scopes:   []string{"openid"},
		},
	); err != nil {
		t.Fatalf("buildIDToken() error = %v", err)
	}

	// The point of: auth_time must NOT collapse onto the code issue time.
	if got := issuer.idTokenParams[0].AuthenticationTime; !got.Equal(authenticatedAt) {
		t.Fatalf("auth_time = %v, want %v", got, authenticatedAt)
	}
	if got := issuer.idTokenParams[0].AuthenticationTime; got.Equal(codeIssuedAt) {
		t.Fatal("auth_time equals the code issue time")
	}
}

func TestBuildIDToken_TrimsClaimsByScope(t *testing.T) {
	cases := []struct {
		name        string
		scopes      []string
		wantName    string
		wantEmail   string
		wantPhone   string
		wantPicture string
		wantOrg     string
	}{
		{
			name:   "openid only",
			scopes: []string{"openid"},
		},
		{
			name:        "profile",
			scopes:      []string{"openid", "profile"},
			wantName:    "Jane Doe",
			wantPicture: "https://cdn.example.com/jane.png",
			wantOrg:     "built-in",
		},
		{
			name:      "email",
			scopes:    []string{"openid", "email"},
			wantEmail: "jane@example.com",
		},
		{
			name:      "phone",
			scopes:    []string{"openid", "phone"},
			wantPhone: "+15551234567",
		},
		{
			name:        "all",
			scopes:      []string{"openid", "profile", "email", "phone"},
			wantName:    "Jane Doe",
			wantEmail:   "jane@example.com",
			wantPhone:   "+15551234567",
			wantPicture: "https://cdn.example.com/jane.png",
			wantOrg:     "built-in",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			issuer := &captureTokenIssuer{}
			service := Service{TokenIssuer: issuer}

			if _, err := service.buildIDToken(
				&applicationdomain.Application{ClientID: "portal-client"},
				idTokenTestUser(),
				&orgdomain.Organization{Slug: "built-in"},
				idTokenIssuance{
					Issuer:   "https://iam.example.com",
					AuthTime: time.Now(),
					Scopes:   testCase.scopes,
				},
			); err != nil {
				t.Fatalf("buildIDToken() error = %v", err)
			}

			params := issuer.idTokenParams[0]
			// Un-granted claims must be omitted, not present-and-empty: an empty
			// string asserts "the value is empty" instead of "not disclosed".
			assertClaim(t, "name", params.Name, testCase.wantName)
			assertClaim(t, "email", params.Email, testCase.wantEmail)
			assertClaim(t, "phone_number", params.Phone, testCase.wantPhone)
			assertClaim(t, "picture", params.Picture, testCase.wantPicture)
			assertClaim(t, "org", params.Organization, testCase.wantOrg)

			if testCase.wantEmail == "" && params.EmailVerified != nil {
				t.Error("email_verified was set without the email scope")
			}
			if testCase.wantPhone == "" && params.PhoneVerified != nil {
				t.Error("phone_number_verified was set without the phone scope")
			}
		})
	}
}

func assertClaim(t *testing.T, name, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %q, want %q", name, got, want)
	}
}

func TestBuildIDToken_SetsAtHash(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}
	const accessToken = "gl_pat_abcdef"

	_, err := service.buildIDToken(
		&applicationdomain.Application{ClientID: "portal-client"},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{
			Issuer:      "https://iam.example.com",
			AuthTime:    time.Now(),
			Scopes:      []string{"openid"},
			AccessToken: accessToken,
		},
	)
	if err != nil {
		t.Fatalf("buildIDToken() error = %v", err)
	}

	// at_hash is the left half of the SHA-256 of the access token, base64url
	// encoded (OIDC Core §3.1.3.6).
	sum := sha256.Sum256([]byte(accessToken))
	want := base64.RawURLEncoding.EncodeToString(sum[:len(sum)/2])
	if got := issuer.idTokenParams[0].AccessTokenHash; got != want {
		t.Fatalf("at_hash = %q, want %q", got, want)
	}
}

// TestBuildIDToken_SetsSM3AtHash pins the algorithm-dependent half of OIDC Core
// §3.1.3.6: when the application signs with SM2-SM3, at_hash is the left half of
// the SM3 digest of the access token, not SHA-256. The SHA-256 case above and
// this case together make a silently fixed hash (always SHA-256) fail.
func TestBuildIDToken_SetsSM3AtHash(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}
	const accessToken = "gl_pat_abcdef"

	_, err := service.buildIDToken(
		&applicationdomain.Application{
			ClientID:           "portal-client",
			TokenSigningMethod: applicationdomain.TokenSigningMethodSM2SM3,
		},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{
			Issuer:      "https://iam.example.com",
			AuthTime:    time.Now(),
			Scopes:      []string{"openid"},
			AccessToken: accessToken,
		},
	)
	if err != nil {
		t.Fatalf("buildIDToken() error = %v", err)
	}

	digest := platformcrypto.SM3Sum([]byte(accessToken))
	want := base64.RawURLEncoding.EncodeToString(digest[:len(digest)/2])
	if got := issuer.idTokenParams[0].AccessTokenHash; got != want {
		t.Fatalf("at_hash = %q, want the SM3-based %q", got, want)
	}
}

func TestBuildIDToken_OmitsAtHashWithoutAccessToken(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}

	if _, err := service.buildIDToken(
		&applicationdomain.Application{ClientID: "portal-client"},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{Issuer: "https://iam.example.com", AuthTime: time.Now(), Scopes: []string{"openid"}},
	); err != nil {
		t.Fatalf("buildIDToken() error = %v", err)
	}

	if got := issuer.idTokenParams[0].AccessTokenHash; got != "" {
		t.Fatalf("at_hash = %q, want empty when no access token accompanies the ID token", got)
	}
}

func TestResolveIssuer_PrefersTheRequest(t *testing.T) {
	service := Service{IssuerURL: "https://configured.example.com"}
	t.Setenv("ISSUER_URL", "")

	if got := service.resolveIssuer("https://request.example.com"); got != "https://request.example.com" {
		t.Fatalf("resolveIssuer() = %q, want the request value", got)
	}
	if got := service.resolveIssuer(""); got != "https://configured.example.com" {
		t.Fatalf("resolveIssuer() = %q, want the configured value", got)
	}
}

// TestResolveIssuer_IsTheSameValueDiscoveryWouldReport is the executable form of
// the acceptance criterion: id_token.iss == logout_token.iss == discovery.issuer.
func TestResolveIssuer_IsTheSameValueDiscoveryWouldReport(t *testing.T) {
	t.Setenv("ISSUER_URL", "https://iam.acme.example/")
	service := Service{}

	discovery, err := service.GetOIDCDiscovery(context.Background(), "https://iam.acme.example")
	if err != nil {
		t.Fatalf("GetOIDCDiscovery() error = %v", err)
	}

	if got := service.resolveIssuer(""); got != discovery.Issuer {
		t.Fatalf("token issuer = %q, want discovery issuer %q", got, discovery.Issuer)
	}
}

// ---------------------------------------------------------------------------
// UserInfo scope gating (设计报告 14.9)
// ---------------------------------------------------------------------------

func userInfoTestService(t *testing.T, scopes []string) Service {
	t.Helper()

	user := idTokenTestUser()
	tokenRepo := &testTokenRepository{
		tokensByHash: map[string]*tokendomain.Token{
			platformcrypto.HashTokenValue("gl_pat_abcdef"): {
				ID:               "token-1",
				UserID:           user.ID,
				Username:         user.Username,
				OrganizationSlug: "built-in",
				Scopes:           scopes,
				ExpiresAt:        time.Now().Add(time.Hour),
			},
		},
	}
	uow := serviceTestUnitOfWork{
		loginTestUnitOfWork: loginTestUnitOfWork{
			userRepo: &loginTestUserRepository{users: map[string]*userdomain.User{user.ID: user}},
		},
		tokenRepo: tokenRepo,
	}
	return Service{Clock: fixedAuthClock{}, UOW: uow}
}

type fixedAuthClock struct{}

func (fixedAuthClock) Now() time.Time { return time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC) }

func TestGetUserInfo_RequiresOpenIDScope(t *testing.T) {
	service := userInfoTestService(t, []string{"profile", "email"})

	_, err := service.GetUserInfo(context.Background(), "gl_pat_abcdef")
	if err == nil {
		t.Fatal("expected a non-openid access token to be refused")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeForbidden {
		t.Fatalf("error = %v, want forbidden", err)
	}
}

func TestGetUserInfo_TrimsClaimsByScope(t *testing.T) {
	cases := []struct {
		name      string
		scopes    []string
		wantEmail string
		wantPhone string
		wantUser  string
		wantOrg   string
	}{
		{name: "openid only", scopes: []string{"openid"}},
		{
			name:     "profile",
			scopes:   []string{"openid", "profile"},
			wantUser: "jane",
			wantOrg:  "built-in",
		},
		{
			name:      "email",
			scopes:    []string{"openid", "email"},
			wantEmail: "jane@example.com",
		},
		{
			name:      "phone",
			scopes:    []string{"openid", "phone"},
			wantPhone: "+15551234567",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := userInfoTestService(t, testCase.scopes)

			response, err := service.GetUserInfo(context.Background(), "gl_pat_abcdef")
			if err != nil {
				t.Fatalf("GetUserInfo() error = %v", err)
			}
			if response.Sub != "user-1" {
				t.Fatalf("sub = %q, want user-1", response.Sub)
			}
			assertClaim(t, "email", response.Email, testCase.wantEmail)
			assertClaim(t, "phone_number", response.Phone, testCase.wantPhone)
			assertClaim(t, "preferred_username", response.Username, testCase.wantUser)
			assertClaim(t, "org", response.Organization, testCase.wantOrg)
		})
	}
}

func TestGetUserInfo_ScopesAreNormalized(t *testing.T) {
	// A token whose scopes carry padding must still satisfy the openid check.
	service := userInfoTestService(t, []string{" openid ", "email"})

	if _, err := service.GetUserInfo(context.Background(), "gl_pat_abcdef"); err != nil {
		t.Fatalf("GetUserInfo() with padded scopes error = %v", err)
	}
}

func TestScopeSet_Membership(t *testing.T) {
	set := newScopeSet([]string{"openid", " profile ", "", "email"})

	for _, scope := range []string{"openid", "profile", "email"} {
		if !set.has(scope) {
			t.Errorf("expected %q to be granted", scope)
		}
	}
	for _, scope := range []string{"phone", "address", ""} {
		if set.has(scope) {
			t.Errorf("did not expect %q to be granted", scope)
		}
	}
}
