package auth

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func impersonationFixture(t *testing.T, mutate func(*tokendomain.Token)) (*Service, string, *tokendomain.Token) {
	t.Helper()
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	raw := "imp_opaque_token_value"
	token := &tokendomain.Token{
		ID: "imp-token-1", UserID: "user-1", Username: "target",
		OrganizationID: "org-1", OrganizationSlug: "acme",
		TokenPrefix: "imp", TokenHash: platformcrypto.HashTokenValue(raw),
		Scopes:    []string{"impersonate"},
		Metadata:  map[string]any{"impersonated": true, "impersonated_by": "admin-1"},
		ExpiresAt: now.Add(time.Hour),
	}
	if mutate != nil {
		mutate(token)
	}
	tokenRepo := &testTokenRepository{tokensByHash: map[string]*tokendomain.Token{token.TokenHash: token}}
	uow := loginTestUnitOfWork{
		tokenRepo: tokenRepo,
		userRepo: &loginTestUserRepository{
			users:      map[string]*userdomain.User{"user-1": {ID: "user-1", Username: "target", OrganizationID: "org-1"}},
			usersByOrg: map[string]map[string]*userdomain.User{"org-1": {"target": {ID: "user-1", Username: "target", OrganizationID: "org-1"}}},
		},
		orgRepo: &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{"acme": {ID: "org-1", Slug: "acme"}}},
	}
	svc := newTestService(uow)
	svc.Clock = fakeClock{fixedTime: now}
	return &svc, raw, token
}

func TestResolveImpersonationSession_ValidToken(t *testing.T) {
	svc, raw, _ := impersonationFixture(t, nil)
	session, err := svc.ResolveImpersonationSession(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, "user-1", session.UserID)
	require.Equal(t, "target", session.Username)
	require.Equal(t, "org-1", session.OrganizationID)
	require.Equal(t, "acme", session.OrganizationSlug)
	require.Equal(t, "admin-1", session.ImpersonatedBy)
}

func TestResolveImpersonationSession_MissingImpersonatedByResolvesUnbound(t *testing.T) {
	svc, raw, _ := impersonationFixture(t, func(token *tokendomain.Token) {
		delete(token.Metadata, "impersonated_by")
	})
	session, err := svc.ResolveImpersonationSession(context.Background(), raw)
	require.NoError(t, err)
	require.Equal(t, "", session.ImpersonatedBy)
}

func TestResolveImpersonationSession_RejectsInvalidTokens(t *testing.T) {
	cases := map[string]func(*tokendomain.Token){
		"wrong prefix":     func(token *tokendomain.Token) { token.TokenPrefix = "pat" },
		"not impersonated": func(token *tokendomain.Token) { token.Metadata["impersonated"] = false },
		"expired":          func(token *tokendomain.Token) { token.ExpiresAt = time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC) },
		"revoked": func(token *tokendomain.Token) {
			at := time.Date(2026, 5, 15, 9, 0, 0, 0, time.UTC)
			token.RevokedAt = &at
		},
		"missing scope": func(token *tokendomain.Token) { token.Scopes = []string{"read"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			svc, raw, _ := impersonationFixture(t, mutate)
			_, err := svc.ResolveImpersonationSession(context.Background(), raw)
			require.Error(t, err)
			require.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
		})
	}
}
