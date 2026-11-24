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

type l4PasswordStateProvider struct {
	bypasses []appauth.PasswordGateBypassInput
}

func (p *l4PasswordStateProvider) GetPasswordState(context.Context, appauth.PasswordStateQuery) (appauth.PasswordState, error) {
	return appauth.PasswordState{MustChange: true, Reason: "need_update_password"}, nil
}

func (p *l4PasswordStateProvider) RecordPasswordGateBypass(_ context.Context, input appauth.PasswordGateBypassInput) error {
	p.bypasses = append(p.bypasses, input)
	return nil
}

// TestPasswordGate_ImpersonationBypassHTTP is the interface-level L4 evidence
// for OPEN-API-58: an opaque impersonation request passes the session
// middleware, reaches a non-exempt route, and the gate records the bypass
// instead of returning 401/403.
func TestPasswordGate_ImpersonationBypassHTTP(t *testing.T) {
	ctrl := gomock.NewController(t)
	authUoW := appuowmock.NewMockAuthUoW(ctrl)
	tx := appuowmock.NewMockUnitOfWork(ctrl)
	tokenRepo := tokensmock.NewMockRepository(ctrl)
	userRepo := usermock.NewMockRepository(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)

	raw := "imp_l4_token"
	now := time.Now()
	token := &tokendomain.Token{
		ID: "imp-l4", UserID: "user-target", Username: "bob", OrganizationID: "org-1",
		TokenPrefix: "imp", TokenHash: platformcrypto.HashTokenValue(raw),
		Scopes:    []string{"impersonate"},
		Metadata:  map[string]any{"impersonated": true, "impersonated_by": "admin-1"},
		ExpiresAt: now.Add(time.Hour),
	}

	authUoW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		})
	authUoW.EXPECT().Tokens().Return(tokenRepo).AnyTimes()
	tx.EXPECT().Tokens().Return(tokenRepo).AnyTimes()
	tokenRepo.EXPECT().GetByTokenHash(gomock.Any(), token.TokenHash).Return(token, nil).AnyTimes()
	tx.EXPECT().Users().Return(userRepo).AnyTimes()
	userRepo.EXPECT().GetByID(gomock.Any(), "user-target").Return(&userdomain.User{ID: "user-target", Username: "bob", OrganizationID: "org-1"}, nil)
	tx.EXPECT().Organizations().Return(orgRepo).AnyTimes()
	orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{ID: "org-1", Slug: "acme"}, nil)

	svc := &appauth.Service{UOW: authUoW, Clock: clock.SystemClock{}}
	state := &l4PasswordStateProvider{}
	gate := NewPasswordGate(state, svc, "enforce", clock.SystemClock{})
	nextCalled := false
	chain := SessionContextMiddleware(Handler{Service: svc}, failingTokenVerifier{})(
		gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusNoContent)
		})),
	)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	recorder := httptest.NewRecorder()
	chain.ServeHTTP(recorder, req)

	require.True(t, nextCalled, "gate must let the impersonated request through")
	require.NotEqual(t, http.StatusUnauthorized, recorder.Code)
	require.Len(t, state.bypasses, 1)
	require.Equal(t, "user-target", state.bypasses[0].SubjectUserID)
	require.Equal(t, "admin-1", state.bypasses[0].ImpersonatorID)
	require.Equal(t, "/api/v1/users", state.bypasses[0].Path)
}
