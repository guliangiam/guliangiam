package sessions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	idorSessionID     = "11111111-1111-1111-1111-111111111111"
	idorSessionOrg    = "22222222-2222-2222-2222-222222222222"
	idorForeignOrg    = "33333333-3333-3333-3333-333333333333"
	idorSessionUserID = "44444444-4444-4444-4444-444444444444"
)

type idorSessionClock struct{}

func (idorSessionClock) Now() time.Time { return time.Unix(0, 0).UTC() }

// idorSessionRepo is a tenant-aware in-memory sessions repository. Unscoped
// methods mimic the MySQL mirror (no RLS): they return or mutate any row. The
// scoped methods are the tenant predicates the by-ID routes must go through.
type idorSessionRepo struct {
	sessions    map[string]*domain.Session
	getCalls    int
	scopedOrgs  []string
	updateCalls int
	deleteCalls int
	revokeCalls int
}

func newIDORSessionRepo(sessionOrg string) *idorSessionRepo {
	return &idorSessionRepo{sessions: map[string]*domain.Session{
		idorSessionID: {
			ID:               idorSessionID,
			UserID:           idorSessionUserID,
			Username:         "tenant-user",
			OrganizationID:   sessionOrg,
			OrganizationSlug: "tenant",
			ApplicationID:    "app-1",
			SessionState:     domain.StateActive,
			ExpiresAt:        idorSessionClock{}.Now().Add(24 * time.Hour),
			Revision:         1,
		},
	}}
}

func (r *idorSessionRepo) find(id, organizationID string) *domain.Session {
	session, ok := r.sessions[id]
	if !ok {
		return nil
	}
	if organizationID != "" && session.OrganizationID != organizationID {
		return nil
	}
	return session
}

func (r *idorSessionRepo) GetByID(_ context.Context, id string) (*domain.Session, error) {
	r.getCalls++
	if session := r.find(id, ""); session != nil {
		return session, nil
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *idorSessionRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Session, error) {
	r.getCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if session := r.find(id, organizationID); session != nil {
		return session, nil
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *idorSessionRepo) List(context.Context, domain.ListQuery) ([]*domain.Session, int64, error) {
	return nil, 0, nil
}

func (r *idorSessionRepo) Create(context.Context, *domain.Session) error { return nil }

func (r *idorSessionRepo) Update(_ context.Context, session *domain.Session) error {
	r.updateCalls++
	if existing := r.find(session.ID, ""); existing != nil {
		r.sessions[session.ID] = session
		return nil
	}
	return apperrors.NewNotFound("session not found")
}

func (r *idorSessionRepo) Touch(context.Context, string, time.Time) error { return nil }

func (r *idorSessionRepo) Revoke(_ context.Context, id string, revokedAt time.Time) error {
	r.revokeCalls++
	if session := r.find(id, ""); session != nil {
		session.RevokedAt = &revokedAt
		return nil
	}
	return apperrors.NewNotFound("session not found")
}

func (r *idorSessionRepo) RevokeForOrganization(_ context.Context, id, organizationID string, revokedAt time.Time) error {
	r.revokeCalls++
	if session := r.find(id, organizationID); session != nil {
		session.RevokedAt = &revokedAt
		return nil
	}
	return apperrors.NewNotFound("session not found")
}

func (r *idorSessionRepo) RevokeByUserExcept(context.Context, string, string, time.Time) (int64, error) {
	return 0, nil
}

func (r *idorSessionRepo) RevokeByUserAndApp(context.Context, string, string, string, time.Time) (int64, error) {
	return 0, nil
}

func (r *idorSessionRepo) RevokeByUser(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}

func (r *idorSessionRepo) RevokeByOrganization(context.Context, string, time.Time) (int64, error) {
	return 0, nil
}

func (r *idorSessionRepo) Delete(_ context.Context, id string) error {
	r.deleteCalls++
	if r.find(id, "") == nil {
		return apperrors.NewNotFound("session not found")
	}
	delete(r.sessions, id)
	return nil
}

func (r *idorSessionRepo) DeleteForOrganization(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	if r.find(id, organizationID) == nil {
		return apperrors.NewNotFound("session not found")
	}
	delete(r.sessions, id)
	return nil
}

func (r *idorSessionRepo) CleanupExpired(context.Context, time.Time) (int64, error) { return 0, nil }

type idorSessionHarness struct {
	router chi.Router
	repo   *idorSessionRepo
}

func newIDORSessionHarness(t *testing.T, sessionOrg string) *idorSessionHarness {
	t.Helper()

	ctrl := gomock.NewController(t)
	repo := newIDORSessionRepo(sessionOrg)
	authUOW := appuowmock.NewMockAuthUoW(ctrl)
	authUOW.EXPECT().Sessions().Return(repo).AnyTimes()
	authUOW.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			tx := appuowmock.NewMockUnitOfWork(ctrl)
			tx.EXPECT().Sessions().Return(repo).AnyTimes()
			tx.EXPECT().Outbox().Return(nil).AnyTimes()
			return fn(ctx, tx)
		},
	).AnyTimes()

	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: appsessions.Service{Clock: idorSessionClock{}, UOW: authUOW}})
	return &idorSessionHarness{router: router, repo: repo}
}

func idorSessionRequest(method, target, body string, withTenant bool) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if withTenant {
		ctx := tenantctx.WithTenantScope(req.Context(), idorSessionOrg, "session-org", tenantctx.TenantFilterStrict)
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
			UserID: idorSessionUserID, OrganizationID: idorSessionOrg,
		})
		req = req.WithContext(ctx)
	}
	return req
}

func serveIDORSession(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

var sessionIDORRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"get", http.MethodGet, "/api/v1/sessions/" + idorSessionID, ""},
	{"patch", http.MethodPatch, "/api/v1/sessions/" + idorSessionID, `{"sessionState":"idle","revision":1}`},
	{"delete", http.MethodDelete, "/api/v1/sessions/" + idorSessionID, ""},
	{"revoke", http.MethodPost, "/api/v1/sessions/" + idorSessionID + "/revoke", ""},
}

// TestSessionByID_ForeignTenantRejected pins that a session owned by another
// tenant is indistinguishable from a missing one on every by-ID route, and that
// no read-side leakage or mutation happens before the rejection.
func TestSessionByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range sessionIDORRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORSessionHarness(t, idorForeignOrg)

			req := idorSessionRequest(tt.method, tt.path+"?organizationId="+idorForeignOrg, tt.body, true)
			recorder := serveIDORSession(t, h.router, req)

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign session must look missing")
			require.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, idorSessionOrg, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.repo.updateCalls, "foreign tenant must not update the session")
			assert.Zero(t, h.repo.deleteCalls, "foreign tenant must not delete the session")
			assert.Zero(t, h.repo.revokeCalls, "foreign tenant must not revoke the session")
			require.Len(t, h.repo.sessions, 1, "foreign session row must remain untouched")
		})
	}
}

// TestSessionByID_WithoutTenantScopeRejected pins the empty-org fail-fast: no
// repository call is made when the authenticated scope cannot name a tenant.
func TestSessionByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range sessionIDORRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORSessionHarness(t, idorForeignOrg)

			recorder := serveIDORSession(t, h.router, idorSessionRequest(tt.method, tt.path, tt.body, false))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getCalls)
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.deleteCalls)
			assert.Zero(t, h.repo.revokeCalls)
		})
	}
}

// TestSessionByID_OwnTenantSucceeds pins that the four routes keep working
// inside the authenticated tenant and resolve ownership through the scoped read.
func TestSessionByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORSessionHarness(t, idorSessionOrg)
		recorder := serveIDORSession(t, h.router, idorSessionRequest(http.MethodGet, "/api/v1/sessions/"+idorSessionID, "", true))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "tenant-user")
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorSessionOrg, h.repo.scopedOrgs[0])
	})

	t.Run("patch", func(t *testing.T) {
		h := newIDORSessionHarness(t, idorSessionOrg)
		recorder := serveIDORSession(t, h.router, idorSessionRequest(http.MethodPatch,
			"/api/v1/sessions/"+idorSessionID, `{"sessionState":"idle","revision":1}`, true))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		assert.Equal(t, domain.StateIdle, h.repo.sessions[idorSessionID].SessionState)
	})

	t.Run("delete", func(t *testing.T) {
		h := newIDORSessionHarness(t, idorSessionOrg)
		recorder := serveIDORSession(t, h.router, idorSessionRequest(http.MethodDelete, "/api/v1/sessions/"+idorSessionID, "", true))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.deleteCalls)
		assert.Empty(t, h.repo.sessions, "own-tenant delete must remove the session")
	})

	t.Run("revoke", func(t *testing.T) {
		h := newIDORSessionHarness(t, idorSessionOrg)
		recorder := serveIDORSession(t, h.router, idorSessionRequest(http.MethodPost,
			"/api/v1/sessions/"+idorSessionID+"/revoke", "", true))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.revokeCalls)
		require.NotNil(t, h.repo.sessions[idorSessionID].RevokedAt)
	})
}
