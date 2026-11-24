package auth

import (
	"context"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
	"time"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin 设计报告 14.12.3 / 14.12.4. The defect was that the
// logout token's `sid` was the internal `sessions.id` — the value in the session
// cookie and a component of a CAS ticket — while the ID token carried no sid at
// all, so a relying party had no way to connect the two and was being handed a
// handle to our session as a bonus.

// TestSessionSid_IsNotTheInternalSessionID is the headline property: `sid` must
// be its own value, never the primary key.
func TestSessionSid_IsNotTheInternalSessionID(t *testing.T) {
	sid, err := newSessionSid()
	require.NoError(t, err)

	assert.NotEmpty(t, sid)
	assert.True(t, strings.HasPrefix(sid, "gl_sid_"), "sid = %q, want the gl_sid_ prefix", sid)

	raw := strings.TrimPrefix(sid, "gl_sid_")
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	require.NoError(t, err, "the body must be unpadded base64url")
	// 14.12.3 requires at least 128 bits; the constructor draws 256.
	assert.GreaterOrEqual(t, len(decoded), 16)
	assert.Equal(t, 32, len(decoded), "the sid must draw the full 256 bits")
}

func TestSessionSid_IsUniquePerCall(t *testing.T) {
	seen := make(map[string]struct{}, 512)
	for i := 0; i < 512; i++ {
		sid, err := newSessionSid()
		require.NoError(t, err)
		_, duplicate := seen[sid]
		require.False(t, duplicate, "collision after %d draws: %s", i, sid)
		seen[sid] = struct{}{}
	}
}

// A sid that looked like a UUIDv7 would mean the value came from the clock, the
// shape was about for authorization codes. base64url legitimately contains
// `-`, so the check is on the canonical UUID shape rather than on the character.
func TestSessionSid_IsNotAUUID(t *testing.T) {
	uuidShape := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	for i := 0; i < 64; i++ {
		sid, err := newSessionSid()
		require.NoError(t, err)

		body := strings.TrimPrefix(sid, sessionSidPrefix)
		assert.False(t, uuidShape.MatchString(body), "sid %q carries a UUID body", sid)
		assert.NotEqual(t, 36, len(body), "a 36-character body is the UUID length")
	}
}

// The claim the relying party receives must be the sid, never the row's key.
func TestBuildIDToken_CarriesTheSessionSid(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}

	if _, err := service.buildIDToken(
		&applicationdomain.Application{ClientID: "portal-client"},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{
			Issuer:    "https://iam.example.com",
			AuthTime:  time.Now(),
			Scopes:    []string{"openid"},
			SessionID: "gl_sid_abc123",
		},
	); err != nil {
		t.Fatalf("buildIDToken() error = %v", err)
	}

	require.Len(t, issuer.idTokenParams, 1)
	assert.Equal(t, "gl_sid_abc123", issuer.idTokenParams[0].SessionID)
}

// Both tokens must report the same value. This is the acceptance criterion in
// 14.12.3 verbatim: `id_token.sid == logout_token.sid`.
func TestSessionSid_IsTheSameValueInBothTokens(t *testing.T) {
	issuer := &captureTokenIssuer{}
	service := Service{TokenIssuer: issuer}

	const sid = "gl_sid_shared-value"
	authTime := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)

	// ID token: issued at the token endpoint from the sid on the code.
	if _, err := service.buildIDToken(
		&applicationdomain.Application{ClientID: "portal-client"},
		idTokenTestUser(),
		&orgdomain.Organization{Slug: "built-in"},
		idTokenIssuance{
			Issuer:    "https://iam.example.com",
			AuthTime:  authTime,
			Scopes:    []string{"openid"},
			SessionID: sid,
		},
	); err != nil {
		t.Fatalf("buildIDToken() error = %v", err)
	}
	idTokenSid := issuer.idTokenParams[0].SessionID

	// Logout token: the claim map the back-channel path signs.
	claims := map[string]any{"sid": sid}
	logoutTokenSid, _ := claims["sid"].(string)

	assert.Equal(t, idTokenSid, logoutTokenSid)
}

// The code is the only carrier of the sid between the authorize call and the
// token call: the token endpoint has no session in hand. If the constructor
// dropped it, the ID token would silently lose the claim.
func TestNewAuthorizationCode_BindsTheSessionSid(t *testing.T) {
	service := Service{UUIDGenerator: fakeUUIDGenerator{fixedID: "code-id"}}

	code, err := service.newAuthorizationCode(authorizationCodeIssuance{
		UserID:          "user-1",
		ApplicationID:   "app-1",
		RedirectURI:     "https://portal.example.com/callback",
		CodeChallenge:   "challenge",
		ChallengeMethod: "S256",
		Sid:             "gl_sid_from-the-session",
		AuthTime:        time.Now(),
	}, time.Now())
	require.NoError(t, err)

	assert.Equal(t, "gl_sid_from-the-session", code.Sid)
}

// sessionSidFor is what turns the cookie value into the claim. It must report ""
// rather than an empty-looking value for a session it cannot resolve, so the
// caller omits the claim instead of publishing a stale handle.
func TestSessionSidFor_ResolvesAndOmits(t *testing.T) {
	cases := map[string]struct {
		sessionID string
		want      string
	}{
		"resolves the sid": {"sess-1", "gl_sid_lookup"},
		"unknown session":  {"missing", ""},
		"blank session id": {"", ""},
		"whitespace only":  {"   ", ""},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			uow := serviceTestUnitOfWork{
				sessionRepo: &sidTestSessionRepository{
					sessions: map[string]*sessiondomain.Session{
						"sess-1": {ID: "sess-1", Sid: "gl_sid_lookup"},
					},
				},
			}
			assert.Equal(t, tc.want, sessionSidFor(context.Background(), uow, tc.sessionID))
		})
	}
}

// A session whose sid is empty (a row created before the column existed) must
// not produce a literal placeholder.
func TestSessionSidFor_EmptyStoredSidYieldsEmpty(t *testing.T) {
	uow := serviceTestUnitOfWork{
		sessionRepo: &sidTestSessionRepository{
			sessions: map[string]*sessiondomain.Session{
				"sess-legacy": {ID: "sess-legacy"},
			},
		},
	}
	assert.Empty(t, sessionSidFor(context.Background(), uow, "sess-legacy"))
}

// The two capability flags are the protocol-facing half of this change: the OP
// must not claim `backchannel_logout_session_supported` unless the tokens really
// carry a per-session sid.
func TestDiscovery_AnnouncesBackchannelLogoutCapabilities(t *testing.T) {
	service := Service{}

	discovery, err := service.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	require.NoError(t, err)

	assert.True(t, discovery.BackchannelLogoutSupported)
	assert.True(t, discovery.BackchannelLogoutSessionSupported)
	// Front-channel delivery is still [Target] (14.12.4 / T11): announcing it
	// would be the same kind of drift as announcing a sid we did not send.
	assert.NotContains(t, discovery.EndSessionEndpoint, "frontchannel")
}

// A sid is a handle, not a secret, but it still must not be pasted into a URL:
// the redirect carries the code and the client's own state, nothing else.
func TestAuthorizationRedirect_DoesNotLeakTheSid(t *testing.T) {
	redirect := buildAuthorizationRedirectURL(
		"https://portal.example.com/callback",
		"gl_ac_code-value",
		"client-state",
	)

	assert.NotContains(t, redirect, "gl_sid_")
	assert.Contains(t, redirect, "code=gl_ac_code-value")
	assert.Contains(t, redirect, "state=client-state")
}

// sidTestSessionRepository resolves session ids to their sid. The embedded test
// repository answers NotFound for everything, so it cannot exercise the lookup.
type sidTestSessionRepository struct {
	sessiondomain.Repository
	sessions map[string]*sessiondomain.Session
}

func (r *sidTestSessionRepository) GetByID(_ context.Context, id string) (*sessiondomain.Session, error) {
	session, ok := r.sessions[id]
	if !ok {
		return nil, apperrors.NewNotFound("session not found")
	}
	return session, nil
}
