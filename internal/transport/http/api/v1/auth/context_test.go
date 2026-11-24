package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

func TestWithSessionContext(t *testing.T) {
	session := SessionContext{
		OrganizationSlug: "org1",
		Username:         "user1",
		AccessToken:      "token1",
		Source:           "header",
	}
	ctx := WithSessionContext(context.Background(), session)
	got, ok := SessionContextFromContext(ctx)
	assert.True(t, ok)
	assert.Equal(t, session, got)
}

func TestSessionContextFromContext_Missing(t *testing.T) {
	_, ok := SessionContextFromContext(context.Background())
	assert.False(t, ok)
}

func TestSessionContextFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), sessionContextKey{}, "not a session")
	_, ok := SessionContextFromContext(ctx)
	assert.False(t, ok)
}

func TestHasValidatedBearerSession(t *testing.T) {
	tests := []struct {
		name    string
		session SessionContext
		want    bool
	}{
		{name: "validated header session", session: SessionContext{Source: "header", AccessToken: "token"}, want: true},
		{name: "cookie session", session: SessionContext{Source: "repository", AccessToken: "token"}, want: false},
		{name: "missing access token", session: SessionContext{Source: "header"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithSessionContext(context.Background(), tt.session)
			assert.Equal(t, tt.want, HasValidatedBearerSession(ctx))
		})
	}
	assert.False(t, HasValidatedBearerSession(context.Background()))
}

type tokenVerifierFunc func(string) (map[string]interface{}, error)

func (f tokenVerifierFunc) ValidateToken(token string) (map[string]interface{}, error) {
	return f(token)
}

func TestSessionContextFromHeaders_PopulatesVerifiedSubject(t *testing.T) {
	verifier := tokenVerifierFunc(func(token string) (map[string]interface{}, error) {
		assert.Equal(t, "valid-token", token)
		return map[string]interface{}{
			"sub":             "01a0ea76-0c00-7000-8000-000000000003",
			"username":        "alice",
			"organization_id": "org-001",
		}, nil
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/backchannel-logout", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	session, ok := sessionContextFromHeaders(req, verifier)
	assert.True(t, ok)
	assert.Equal(t, "01a0ea76-0c00-7000-8000-000000000003", session.UserID)
	assert.Equal(t, "alice", session.Username)
	assert.Equal(t, "org-001", session.OrganizationID)
	assert.Equal(t, "valid-token", session.AccessToken)
}

func TestSessionContextFromHeaders_DoesNotMapExternalSubjectToLocalUser(t *testing.T) {
	verifier := tokenVerifierFunc(func(string) (map[string]interface{}, error) {
		return map[string]interface{}{
			"sub":             "okta-user-abc",
			"username":        "federated-user",
			"organization_id": "org-001",
		}, nil
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer valid-token")

	session, ok := sessionContextFromHeaders(req, verifier)
	assert.True(t, ok)
	assert.Empty(t, session.UserID)
	assert.Equal(t, "federated-user", session.Username)
	assert.Equal(t, "org-001", session.OrganizationID)
}

// TrustedOrganizationID is the security boundary between a client-supplied
// organizationId and the tenant a request may actually act on. A forged value must
// never move a write into another tenant (MySQL has no RLS to catch it).
func TestTrustedOrganizationID(t *testing.T) {
	regular := func(ctx context.Context) context.Context {
		return tenantctx.WithTenantScope(ctx, "org-session", "org-session", tenantctx.TenantFilterStrict)
	}
	platformAdmin := func(ctx context.Context) context.Context {
		return tenantctx.WithTenantScope(ctx, "org-session", "org-session", tenantctx.TenantFilterSuperAdmin)
	}

	tests := []struct {
		name      string
		requested string
		ctx       func(context.Context) context.Context
		headers   map[string]string
		want      string
	}{
		{
			name:      "regular user: payload value is ignored",
			requested: "org-victim",
			ctx:       regular,
			headers:   map[string]string{"X-Organization-Id": "org-header-spoof"},
			want:      "org-session",
		},
		{
			name:      "regular user: empty payload still uses the session tenant",
			requested: "",
			ctx:       regular,
			want:      "org-session",
		},
		{
			name:      "platform admin: explicit cross-tenant target is honoured",
			requested: "org-target",
			ctx:       platformAdmin,
			want:      "org-target",
		},
		{
			name:      "platform admin: no target falls back to own tenant",
			requested: "",
			ctx:       platformAdmin,
			want:      "org-session",
		},
		{
			name:      "no tenant scope: payload is not trusted, result is empty",
			requested: "org-victim",
			ctx:       func(ctx context.Context) context.Context { return ctx },
			headers:   map[string]string{"X-Organization-Id": "org-header-spoof"},
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/adapters", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			req = req.WithContext(tt.ctx(req.Context()))
			assert.Equal(t, tt.want, TrustedOrganizationID(req, tt.requested))
		})
	}
}
