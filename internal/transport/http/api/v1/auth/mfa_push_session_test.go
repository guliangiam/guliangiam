package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainproviders "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	domainsessions "github.com/guliangiam/guliangiam/internal/domain/sessions"
	domaintokens "github.com/guliangiam/guliangiam/internal/domain/tokens"
	domainusers "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	pushMfaSessionUser = "session-user-a"
	pushMfaForeignUser = "body-user-b"
	pushMfaOrgID       = "org-a"
	pushMfaProviderID  = "provider-a"
)

type pushMfaUserRepo struct {
	domainusers.Repository
	requested []string
	user      *domainusers.User
}

func (r *pushMfaUserRepo) GetByID(_ context.Context, id string) (*domainusers.User, error) {
	r.requested = append(r.requested, id)
	if r.user == nil || r.user.ID != id {
		return nil, apperrors.NewNotFound("user not found")
	}
	return r.user, nil
}

func (r *pushMfaUserRepo) Update(_ context.Context, _ *domainusers.User) error { return nil }

func (r *pushMfaUserRepo) ReplaceMfaMethods(_ context.Context, _ string, _ []domainusers.UserMfaMethod) error {
	return nil
}

type pushMfaProviderRepo struct {
	domainproviders.Repository
	provider *domainproviders.Provider
}

func (r *pushMfaProviderRepo) GetByID(_ context.Context, id string) (*domainproviders.Provider, error) {
	if r.provider == nil || r.provider.ID != id {
		return nil, apperrors.NewNotFound("provider not found")
	}
	return r.provider, nil
}

type pushMfaSessionRepo struct {
	domainsessions.Repository
	revokedUsers []string
}

func (r *pushMfaSessionRepo) RevokeByUser(_ context.Context, userID string, _ time.Time) (int64, error) {
	r.revokedUsers = append(r.revokedUsers, userID)
	return 0, nil
}

type pushMfaTokenRepo struct {
	domaintokens.Repository
	revokedUsers []string
}

func (r *pushMfaTokenRepo) RevokeRefreshByUser(_ context.Context, userID string, _ time.Time) (int64, error) {
	r.revokedUsers = append(r.revokedUsers, userID)
	return 0, nil
}

func (r *pushMfaTokenRepo) RevokeByQuery(_ context.Context, query domaintokens.RevokeQuery, _ time.Time) (int64, error) {
	r.revokedUsers = append(r.revokedUsers, query.UserID)
	return 0, nil
}

type pushMfaRecordRepo struct {
	domainrecords.Repository
}

func (r *pushMfaRecordRepo) Create(_ context.Context, _ *domainrecords.Record) error { return nil }

type pushMfaOutboxRepo struct {
	domainoutbox.Repository
}

func (r *pushMfaOutboxRepo) Enqueue(_ context.Context, _ *domainoutbox.OutboxEvent) error { return nil }

type pushMfaUUID struct{}

func (pushMfaUUID) NewString() (string, error) { return "generated-push-mfa-id", nil }

// pushMfaTestUOW embeds the full UnitOfWork solely to satisfy WithinTransaction's
// callback type; every repository used by the push MFA mutations is overridden.
type pushMfaTestUOW struct {
	appuow.UnitOfWork
	users     *pushMfaUserRepo
	providers *pushMfaProviderRepo
	sessions  *pushMfaSessionRepo
	tokens    *pushMfaTokenRepo
}

func (u *pushMfaTestUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *pushMfaTestUOW) Users() domainusers.Repository { return u.users }

func (u *pushMfaTestUOW) Providers() domainproviders.Repository { return u.providers }

func (u *pushMfaTestUOW) Sessions() domainsessions.Repository { return u.sessions }

func (u *pushMfaTestUOW) Tokens() domaintokens.Repository { return u.tokens }

func (u *pushMfaTestUOW) Records() domainrecords.Repository { return &pushMfaRecordRepo{} }

func (u *pushMfaTestUOW) Outbox() domainoutbox.Repository { return &pushMfaOutboxRepo{} }

func newPushMfaTestHandler(t *testing.T) (Handler, *pushMfaUserRepo, *pushMfaSessionRepo, *pushMfaTokenRepo) {
	t.Helper()
	users := &pushMfaUserRepo{user: &domainusers.User{
		ID:             pushMfaSessionUser,
		OrganizationID: pushMfaOrgID,
		Username:       "alice",
		MfaMethods: []domainusers.UserMfaMethod{{
			ID: "mfa-push-a", UserID: pushMfaSessionUser, MethodType: "push",
			Enabled: true, Verified: true, DisplayName: "Push Notification",
		}},
	}}
	sessions := &pushMfaSessionRepo{}
	tokens := &pushMfaTokenRepo{}
	uow := &pushMfaTestUOW{
		users: users,
		providers: &pushMfaProviderRepo{provider: &domainproviders.Provider{
			ID: pushMfaProviderID, Category: domainproviders.ProviderCategory("Notification"),
		}},
		sessions: sessions,
		tokens:   tokens,
	}
	service := &appauth.Service{Clock: clock.SystemClock{}, UUIDGenerator: pushMfaUUID{}, UOW: uow}
	return Handler{Service: service}, users, sessions, tokens
}

func pushMfaSessionRequest(path, body, userID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID == "" {
		return req
	}
	ctx := WithSessionContext(req.Context(), SessionContext{
		UserID: userID, Username: "alice", OrganizationID: pushMfaOrgID,
	})
	return req.WithContext(ctx)
}

func pushMfaRouter(handler Handler) *chi.Mux {
	router := chi.NewRouter()
	registerMfaRoutes(router, handler)
	return router
}

func TestPushMfaEnableBindsUserIDToSession(t *testing.T) {
	handler, users, _, _ := newPushMfaTestHandler(t)
	req := pushMfaSessionRequest("/mfa/push/enable",
		`{"mfaPushReceiver":"receiver-a","mfaPushProvider":"`+pushMfaProviderID+`"}`, pushMfaSessionUser)

	recorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{pushMfaSessionUser}, users.requested,
		"enable must resolve the session user, not a body-supplied id")
}

func TestPushMfaEnableRejectsForeignUserID(t *testing.T) {
	handler, users, _, _ := newPushMfaTestHandler(t)
	req := pushMfaSessionRequest("/mfa/push/enable",
		`{"userId":"`+pushMfaForeignUser+`","mfaPushReceiver":"attacker-receiver","mfaPushProvider":"`+pushMfaProviderID+`"}`,
		pushMfaSessionUser)

	recorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"forbidden"`)
	assert.Empty(t, users.requested, "the foreign user must never reach the repository")
}

func TestPushMfaDisableBindsUserIDToSession(t *testing.T) {
	handler, users, sessions, tokens := newPushMfaTestHandler(t)
	req := pushMfaSessionRequest("/mfa/push/disable",
		`{"userId":"`+pushMfaSessionUser+`"}`, pushMfaSessionUser)

	recorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{pushMfaSessionUser}, users.requested)
	assert.Equal(t, []string{pushMfaSessionUser}, sessions.revokedUsers)
	assert.Equal(t, []string{pushMfaSessionUser, pushMfaSessionUser}, tokens.revokedUsers)
}

func TestPushMfaDisableRejectsForeignUserID(t *testing.T) {
	handler, users, sessions, _ := newPushMfaTestHandler(t)
	req := pushMfaSessionRequest("/mfa/push/disable",
		`{"userId":"`+pushMfaForeignUser+`"}`, pushMfaSessionUser)

	recorder := httptest.NewRecorder()
	pushMfaRouter(handler).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"forbidden"`)
	assert.Empty(t, users.requested)
	assert.Empty(t, sessions.revokedUsers, "a foreign disable must not revoke the victim's sessions")
}

func TestPushMfaMutationsRequireSessionUser(t *testing.T) {
	cases := []struct {
		path string
		body string
	}{
		{"/mfa/push/enable", `{"mfaPushReceiver":"receiver-a","mfaPushProvider":"` + pushMfaProviderID + `"}`},
		{"/mfa/push/disable", `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := pushMfaSessionRequest(tc.path, tc.body, "")

			recorder := httptest.NewRecorder()
			pushMfaRouter(Handler{}).ServeHTTP(recorder, req)

			require.Equal(t, http.StatusUnauthorized, recorder.Code)
			assert.Contains(t, recorder.Body.String(), `"code":"unauthorized"`)
		})
	}
}
