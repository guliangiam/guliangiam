package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

type fakeImpersonationResolver struct {
	session *appauth.ImpersonationSession
	err     error
	calls   int
}

func (f *fakeImpersonationResolver) ResolveImpersonationSession(_ context.Context, _ string) (*appauth.ImpersonationSession, error) {
	f.calls++
	return f.session, f.err
}

type failingTokenVerifier struct{}

func (failingTokenVerifier) ValidateToken(string) (map[string]any, error) {
	return nil, errors.New("not a jwt")
}

func TestSessionContextMiddleware_AcceptsOpaqueImpersonationToken(t *testing.T) {
	resolver := &fakeImpersonationResolver{session: &appauth.ImpersonationSession{
		UserID: "user-target", Username: "bob", OrganizationID: "org-1", OrganizationSlug: "acme",
	}}
	var got SessionContext
	handler := SessionContextMiddleware(Handler{ImpersonationResolver: resolver}, failingTokenVerifier{})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = SessionContextFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer imp_opaque")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	require.Equal(t, 1, resolver.calls)
	require.Equal(t, "user-target", got.UserID)
	require.Equal(t, "acme", got.OrganizationSlug)
	require.Equal(t, "impersonation", got.Source)
	require.Equal(t, "imp_opaque", got.AccessToken)
}

func TestSessionContextMiddleware_InvalidOpaqueTokenFallsThrough(t *testing.T) {
	resolver := &fakeImpersonationResolver{err: errors.New("not found")}
	called := false
	handler := SessionContextMiddleware(Handler{ImpersonationResolver: resolver}, failingTokenVerifier{})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, ok := SessionContextFromContext(r.Context())
		called = true
		require.False(t, ok)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer imp_bad")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	require.True(t, called)
	require.Equal(t, 1, resolver.calls)
}
