package auth

import (
	"context"
	"errors"
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
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	sessionsmock "github.com/guliangiam/guliangiam/internal/domain/sessions/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
)

// 裁定 Q2-A：cookie 会话存在时，显式携带且 metadata.impersonated_by 绑定到
// cookie 用户的 imp token 优先；无关/未绑定/无效 token 一律不得遮蔽 cookie 身份。

func overrideRequest(withToken bool) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	if withToken {
		req.Header.Set("Authorization", "Bearer imp_bound_token")
	}
	return req
}

func TestImpersonationOverrideForCookie_BoundTokenWins(t *testing.T) {
	resolver := &fakeImpersonationResolver{session: &appauth.ImpersonationSession{
		UserID: "user-target", Username: "bob", OrganizationID: "org-1", OrganizationSlug: "acme",
		ImpersonatedBy: "admin-1",
	}}
	got, ok := impersonationOverrideForCookie(overrideRequest(true), "admin-1", resolver)
	require.True(t, ok)
	require.Equal(t, 1, resolver.calls)
	require.Equal(t, "user-target", got.UserID)
	require.Equal(t, "bob", got.Username)
	require.Equal(t, "org-1", got.OrganizationID)
	require.Equal(t, "acme", got.OrganizationSlug)
	require.Equal(t, "impersonation", got.Source)
	require.Equal(t, "imp_bound_token", got.AccessToken)
}

func TestImpersonationOverrideForCookie_RejectionPaths(t *testing.T) {
	cases := map[string]struct {
		session      *appauth.ImpersonationSession
		err          error
		cookieUserID string
		withToken    bool
		nilResolver  bool
		wantCalls    int
	}{
		"foreign impersonator": {
			session:      &appauth.ImpersonationSession{UserID: "user-target", ImpersonatedBy: "admin-other"},
			cookieUserID: "admin-1", withToken: true, wantCalls: 1,
		},
		"unbound token": {
			session:      &appauth.ImpersonationSession{UserID: "user-target"},
			cookieUserID: "admin-1", withToken: true, wantCalls: 1,
		},
		"resolver error": {
			err: errors.New("not found"), cookieUserID: "admin-1", withToken: true, wantCalls: 1,
		},
		"no authorization header": {
			session:      &appauth.ImpersonationSession{UserID: "user-target", ImpersonatedBy: "admin-1"},
			cookieUserID: "admin-1", withToken: false, wantCalls: 0,
		},
		"empty cookie user": {
			session:      &appauth.ImpersonationSession{UserID: "user-target", ImpersonatedBy: "admin-1"},
			cookieUserID: " ", withToken: true, wantCalls: 0,
		},
		"nil resolver": {
			cookieUserID: "admin-1", withToken: true, nilResolver: true, wantCalls: 0,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resolver := &fakeImpersonationResolver{session: tc.session, err: tc.err}
			var resolved ImpersonationSessionResolver = resolver
			if tc.nilResolver {
				resolved = nil
			}
			_, ok := impersonationOverrideForCookie(overrideRequest(tc.withToken), tc.cookieUserID, resolved)
			require.False(t, ok)
			require.Equal(t, tc.wantCalls, resolver.calls)
		})
	}
}

// cookieSessionService wires a real appauth.Service whose repository resolves
// the admin cookie session; the imp side uses the fake resolver so the test
// isolates the middleware's bound-priority wiring.
func cookieSessionService(t *testing.T, ctrl *gomock.Controller) *appauth.Service {
	t.Helper()
	authUoW := appuowmock.NewMockAuthUoW(ctrl)
	tx := appuowmock.NewMockUnitOfWork(ctrl)
	sessionRepo := sessionsmock.NewMockRepository(ctrl)
	orgRepo := orgmock.NewMockRepository(ctrl)

	now := time.Now()
	authUoW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, tx)
		}).AnyTimes()
	tx.EXPECT().Sessions().Return(sessionRepo).AnyTimes()
	sessionRepo.EXPECT().GetByID(gomock.Any(), "sess-admin").Return(&sessiondomain.Session{
		ID: "sess-admin", UserID: "admin-1", Username: "admin",
		OrganizationID: "org-1", OrganizationSlug: "acme",
		ExpiresAt: now.Add(time.Hour), LastSeenAt: now,
	}, nil).AnyTimes()
	tx.EXPECT().Organizations().Return(orgRepo).AnyTimes()
	orgRepo.EXPECT().GetByID(gomock.Any(), "org-1").Return(&orgdomain.Organization{ID: "org-1", Slug: "acme"}, nil).AnyTimes()

	return &appauth.Service{UOW: authUoW, Clock: clock.SystemClock{}}
}

func serveWithCookieSession(t *testing.T, svc *appauth.Service, resolver ImpersonationSessionResolver, withToken bool) SessionContext {
	t.Helper()
	var got SessionContext
	handler := SessionContextMiddleware(Handler{Service: svc, ImpersonationResolver: resolver}, failingTokenVerifier{})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = SessionContextFromContext(r.Context())
	}))
	req := overrideRequest(withToken)
	req.AddCookie(&http.Cookie{Name: sessionCookieName(), Value: "sess-admin"})
	handler.ServeHTTP(httptest.NewRecorder(), req)
	return got
}

func TestSessionContextMiddleware_BoundImpTokenOverridesCookieSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := cookieSessionService(t, ctrl)
	resolver := &fakeImpersonationResolver{session: &appauth.ImpersonationSession{
		UserID: "user-target", Username: "bob", OrganizationID: "org-1", OrganizationSlug: "acme",
		ImpersonatedBy: "admin-1",
	}}
	got := serveWithCookieSession(t, svc, resolver, true)
	require.Equal(t, "user-target", got.UserID)
	require.Equal(t, "bob", got.Username)
	require.Equal(t, "impersonation", got.Source)
	require.Equal(t, "imp_bound_token", got.AccessToken)
}

func TestSessionContextMiddleware_ForeignImpTokenKeepsCookieSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := cookieSessionService(t, ctrl)
	resolver := &fakeImpersonationResolver{session: &appauth.ImpersonationSession{
		UserID: "user-target", Username: "bob", OrganizationID: "org-1", OrganizationSlug: "acme",
		ImpersonatedBy: "admin-other",
	}}
	got := serveWithCookieSession(t, svc, resolver, true)
	require.Equal(t, "admin-1", got.UserID)
	require.Equal(t, "admin", got.Username)
	require.Equal(t, "repository", got.Source)
}

func TestSessionContextMiddleware_NoImpTokenKeepsCookieSession(t *testing.T) {
	ctrl := gomock.NewController(t)
	svc := cookieSessionService(t, ctrl)
	resolver := &fakeImpersonationResolver{session: &appauth.ImpersonationSession{
		UserID: "user-target", ImpersonatedBy: "admin-1",
	}}
	got := serveWithCookieSession(t, svc, resolver, false)
	require.Equal(t, "admin-1", got.UserID)
	require.Equal(t, "repository", got.Source)
	require.Equal(t, 0, resolver.calls)
}
