package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type logoutStubSessionRepo struct {
	sessiondomain.Repository
	session   *sessiondomain.Session
	revokeErr error
}

func (r logoutStubSessionRepo) GetByID(_ context.Context, _ string) (*sessiondomain.Session, error) {
	if r.session != nil {
		return r.session, nil
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r logoutStubSessionRepo) Revoke(_ context.Context, _ string, _ time.Time) error {
	return r.revokeErr
}

type logoutStubUOW struct {
	appuow.UnitOfWork
	sessions sessiondomain.Repository
}

func (u logoutStubUOW) Sessions() sessiondomain.Repository { return u.sessions }
func (u logoutStubUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newLogoutStubRouter(repo logoutStubSessionRepo) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: &appauth.Service{
		Clock: clock.SystemClock{},
		UOW:   logoutStubUOW{sessions: repo},
	}})
	return router
}

// TestLogoutEndpoint_DependencyFailureIsNotReportedAsSuccess pins BE-06 at the
// transport face: a revoke failure answers 5xx and keeps the session cookie,
// instead of swallowing the error, clearing the cookie and claiming logged_out.
func TestLogoutEndpoint_DependencyFailureIsNotReportedAsSuccess(t *testing.T) {
	router := newLogoutStubRouter(logoutStubSessionRepo{
		session:   &sessiondomain.Session{ID: "sess-001", UserID: "user-001", OrganizationID: "org-1"},
		revokeErr: apperrors.NewInternal("database error"),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "sess-001"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	for _, cookie := range rec.Result().Cookies() {
		assert.NotEqual(t, SessionCookieName, cookie.Name,
			"a failed logout must not clear the cookie of a still-live session")
	}
}

// TestLogoutEndpoint_MissingSessionStaysIdempotent keeps the repeat-visit
// contract: an absent session is success, and the cookie is cleared.
func TestLogoutEndpoint_MissingSessionStaysIdempotent(t *testing.T) {
	router := newLogoutStubRouter(logoutStubSessionRepo{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "sess-missing"})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var cleared bool
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == SessionCookieName && cookie.MaxAge < 0 {
			cleared = true
		}
	}
	assert.True(t, cleared, "the session cookie must be cleared on success")
}
