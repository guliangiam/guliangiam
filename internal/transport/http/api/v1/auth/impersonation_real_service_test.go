package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	orgmock "github.com/guliangiam/guliangiam/internal/domain/organizations/mock"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	tokensmock "github.com/guliangiam/guliangiam/internal/domain/tokens/mock"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// TestSessionContextMiddleware_OpaqueImpersonationRealService is the interface
// level evidence for OPEN-API-58: a real appauth.Service resolves the opaque
// "imp" token through its repositories and the request reaches the protected
// handler with a complete session.
func TestSessionContextMiddleware_OpaqueImpersonationRealService(t *testing.T) {
	ctrl := gomock.NewController(t)
	authUoW := appuowmock.NewMockAuthUoW(ctrl)
	tx := appuowmock.NewMockUnitOfWork(ctrl)
	tokenRepo := tokensmock.NewMockRepository(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)

	raw := "imp_opaque_token_value"
	now := time.Now()
	token := &tokendomain.Token{
		ID: "imp-1", UserID: "user-target", Username: "bob", OrganizationID: "org-1",
		TokenPrefix: "imp", TokenHash: platformcrypto.HashTokenValue(raw),
		Scopes: []string{"impersonate"}, Metadata: map[string]any{"impersonated": true},
		ExpiresAt: now.Add(time.Hour),
	}

	authUoW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		})
	tx.EXPECT().Tokens().Return(tokenRepo).AnyTimes()
	tokenRepo.EXPECT().GetByTokenHash(gomock.Any(), token.TokenHash).Return(token, nil)
	tx.EXPECT().Users().Return(userRepo).AnyTimes()
	userRepo.EXPECT().GetByID(gomock.Any(), "user-target").Return(&userdomain.User{ID: "user-target", Username: "bob", OrganizationID: "org-1"}, nil)
	tx.EXPECT().Organizations().Return(orgRepo).AnyTimes()
	orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{ID: "org-1", Slug: "acme"}, nil)

	svc := &appauth.Service{UOW: authUoW, Clock: clock.SystemClock{}}
	var got SessionContext
	handler := SessionContextMiddleware(Handler{Service: svc}, failingTokenVerifier{})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = SessionContextFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, "user-target", got.UserID)
	require.Equal(t, "bob", got.Username)
	require.Equal(t, "org-1", got.OrganizationID)
	require.Equal(t, "acme", got.OrganizationSlug)
	require.Equal(t, "impersonation", got.Source)
	require.Equal(t, raw, got.AccessToken)
}
