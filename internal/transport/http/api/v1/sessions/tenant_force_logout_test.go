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
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	tokensdomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	forceLogoutUserID = "55555555-5555-5555-5555-555555555555"
)

// forceLogoutUserRepo is a tenant-aware in-memory users repository. The scoped
// read is the tenant predicate the force-logout route must go through before
// any session/token is revoked.
type forceLogoutUserRepo struct {
	userdomain.Repository
	users        map[string]*userdomain.User
	getByIDCalls int
	scopedCalls  int
	scopedOrgs   []string
}

func (r *forceLogoutUserRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	r.getByIDCalls++
	if user, ok := r.users[id]; ok {
		return user, nil
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *forceLogoutUserRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*userdomain.User, error) {
	r.scopedCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	user, ok := r.users[id]
	if !ok || user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return user, nil
}

// forceLogoutSessionRepo models the sessions table: no organization_id column,
// revocation is by user. The query records the tenant predicate so the test can
// prove the target tenant is the only scope in play.
type forceLogoutSessionRepo struct {
	domain.Repository
	listQueries      []domain.ListQuery
	revokeUserCalls  int
	revokeOrgCalls   int
	lastRevokedUsers []string
}

func (r *forceLogoutSessionRepo) List(_ context.Context, query domain.ListQuery) ([]*domain.Session, int64, error) {
	r.listQueries = append(r.listQueries, query)
	return nil, 0, nil
}

func (r *forceLogoutSessionRepo) RevokeByUser(_ context.Context, userID string, _ time.Time) (int64, error) {
	r.revokeUserCalls++
	r.lastRevokedUsers = append(r.lastRevokedUsers, userID)
	return 2, nil
}

func (r *forceLogoutSessionRepo) RevokeByOrganization(_ context.Context, _ string, _ time.Time) (int64, error) {
	r.revokeOrgCalls++
	return 0, nil
}

type forceLogoutTokenRepo struct {
	tokensdomain.Repository
	refreshCalls  int
	revokeQueries []tokensdomain.RevokeQuery
}

func (r *forceLogoutTokenRepo) RevokeRefreshByUser(_ context.Context, _ string, _ time.Time) (int64, error) {
	r.refreshCalls++
	return 1, nil
}

func (r *forceLogoutTokenRepo) RevokeByQuery(_ context.Context, query tokensdomain.RevokeQuery, _ time.Time) (int64, error) {
	r.revokeQueries = append(r.revokeQueries, query)
	return 3, nil
}

type forceLogoutRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *forceLogoutRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type forceLogoutOutbox struct {
	domainoutbox.Repository
	enqueued []*domainoutbox.OutboxEvent
}

func (r *forceLogoutOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, event)
	return nil
}

type forceLogoutUnitOfWork struct {
	appuow.UnitOfWork
	users    *forceLogoutUserRepo
	sessions *forceLogoutSessionRepo
	tokens   *forceLogoutTokenRepo
	records  *forceLogoutRecords
	outbox   *forceLogoutOutbox
}

func (u *forceLogoutUnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func (u *forceLogoutUnitOfWork) Users() userdomain.Repository      { return u.users }
func (u *forceLogoutUnitOfWork) Sessions() domain.Repository       { return u.sessions }
func (u *forceLogoutUnitOfWork) Tokens() tokensdomain.Repository   { return u.tokens }
func (u *forceLogoutUnitOfWork) Records() recordsdomain.Repository { return u.records }
func (u *forceLogoutUnitOfWork) Outbox() domainoutbox.Repository   { return u.outbox }

type forceLogoutHarness struct {
	router   chi.Router
	users    *forceLogoutUserRepo
	sessions *forceLogoutSessionRepo
	tokens   *forceLogoutTokenRepo
	records  *forceLogoutRecords
	outbox   *forceLogoutOutbox
}

func newForceLogoutHarness(ownerOrg string) *forceLogoutHarness {
	h := &forceLogoutHarness{
		users: &forceLogoutUserRepo{users: map[string]*userdomain.User{
			forceLogoutUserID: {
				ID: forceLogoutUserID, OrganizationID: ownerOrg, Username: "target-user",
			},
		}},
		sessions: &forceLogoutSessionRepo{},
		tokens:   &forceLogoutTokenRepo{},
		records:  &forceLogoutRecords{},
		outbox:   &forceLogoutOutbox{},
	}
	uow := &forceLogoutUnitOfWork{
		users: h.users, sessions: h.sessions, tokens: h.tokens,
		records: h.records, outbox: h.outbox,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: appsessions.Service{
		Clock: idorSessionClock{}, UOW: uow,
	}})
	return h
}

func forceLogoutTenantRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/force-logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), idorSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: idorSessionUserID, OrganizationID: idorSessionOrg,
	})
	return req.WithContext(ctx)
}

func forceLogoutPlatformRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/force-logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: idorSessionUserID, OrganizationID: idorSessionOrg, OrganizationSlug: "built-in",
	})
	return req.WithContext(ctx)
}

func serveForceLogout(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func assertForceLogoutNoWrites(t *testing.T, h *forceLogoutHarness) {
	t.Helper()
	assert.Zero(t, h.sessions.revokeUserCalls, "no session may be revoked")
	assert.Zero(t, h.sessions.revokeOrgCalls, "org-wide revocation must not run")
	assert.Zero(t, h.tokens.refreshCalls, "no refresh token may be revoked")
	assert.Empty(t, h.tokens.revokeQueries, "no access token may be revoked")
	assert.Empty(t, h.records.created, "no audit event may be written")
	assert.Empty(t, h.outbox.enqueued, "no outbox event may be emitted")
}

// TestForceLogoutUser_ForeignTenantRejected pins the IDOR closure: the target
// user is resolved inside the authenticated tenant first, so a user owned by
// another tenant looks exactly like a missing one (404) before any revocation
// or audit write. Both the legacy body (no organizationId) and a forged
// organizationId are covered.
func TestForceLogoutUser_ForeignTenantRejected(t *testing.T) {
	cases := map[string]string{
		"legacy body without organizationId": `{"userId":"` + forceLogoutUserID + `"}`,
		"forged body organizationId": `{"userId":"` + forceLogoutUserID +
			`","organizationId":"` + idorForeignOrg + `"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := newForceLogoutHarness(idorForeignOrg)

			recorder := serveForceLogout(t, h.router, forceLogoutTenantRequest(body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign user must look missing")
			assert.Zero(t, h.users.getByIDCalls, "the unscoped lookup must never be used")
			require.NotEmpty(t, h.users.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.users.scopedOrgs {
				assert.Equal(t, idorSessionOrg, org, "forged body organizationId must be ignored")
			}
			assert.Empty(t, h.sessions.listQueries, "no session read before the tenant check")
			assertForceLogoutNoWrites(t, h)
		})
	}
}

// TestForceLogoutUser_WithoutTenantScopeRejected pins the empty-org fail-fast:
// a caller without a trusted tenant is a 400 before any read or write.
func TestForceLogoutUser_WithoutTenantScopeRejected(t *testing.T) {
	h := newForceLogoutHarness(idorSessionOrg)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/force-logout",
		strings.NewReader(`{"userId":"`+forceLogoutUserID+`"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := serveForceLogout(t, h.router, req)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Zero(t, h.users.getByIDCalls)
	assert.Zero(t, h.users.scopedCalls)
	assert.Empty(t, h.sessions.listQueries)
	assertForceLogoutNoWrites(t, h)
}

// TestForceLogoutUser_OwnTenantSucceeds pins the scoped write: the revocation
// runs inside the trusted tenant, counts stay accurate, and audit carries the
// entity organization.
func TestForceLogoutUser_OwnTenantSucceeds(t *testing.T) {
	cases := map[string]string{
		"legacy body without organizationId": `{"userId":"` + forceLogoutUserID + `"}`,
		"forged body organizationId": `{"userId":"` + forceLogoutUserID +
			`","organizationId":"` + idorForeignOrg + `"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			h := newForceLogoutHarness(idorSessionOrg)

			recorder := serveForceLogout(t, h.router, forceLogoutTenantRequest(body))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.JSONEq(t, `{"status":"revoked","revokedCount":2,"refreshCount":1,"accessCount":3}`, recorder.Body.String())
			assert.Zero(t, h.users.getByIDCalls)
			require.NotEmpty(t, h.users.scopedOrgs)
			for _, org := range h.users.scopedOrgs {
				assert.Equal(t, idorSessionOrg, org, "forged body organizationId must be ignored")
			}
			require.Len(t, h.sessions.listQueries, 1)
			assert.Equal(t, idorSessionOrg, h.sessions.listQueries[0].OrganizationID)
			assert.Equal(t, 1, h.sessions.revokeUserCalls)
			assert.Contains(t, h.sessions.lastRevokedUsers, forceLogoutUserID)
			assert.Equal(t, 1, h.tokens.refreshCalls)
			require.Len(t, h.tokens.revokeQueries, 1)
			assert.Equal(t, forceLogoutUserID, h.tokens.revokeQueries[0].UserID)
			require.Len(t, h.records.created, 1, "the scoped force logout must audit")
			assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID,
				"audit must carry the entity organization")
			require.Len(t, h.outbox.enqueued, 1)
		})
	}
}

// TestForceLogoutUser_PlatformAdminTargetsExplicitTenant pins that a platform
// administrator keeps cross-tenant operations: the explicit organizationId is
// honoured and is the only scope used for the revocation.
func TestForceLogoutUser_PlatformAdminTargetsExplicitTenant(t *testing.T) {
	h := newForceLogoutHarness(idorForeignOrg)

	recorder := serveForceLogout(t, h.router, forceLogoutPlatformRequest(
		`{"userId":"`+forceLogoutUserID+`","organizationId":"`+idorForeignOrg+`"}`))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Zero(t, h.users.getByIDCalls)
	require.NotEmpty(t, h.users.scopedOrgs)
	assert.Equal(t, idorForeignOrg, h.users.scopedOrgs[0],
		"platform admin's explicit organization is the predicate")
	assert.Equal(t, []string{idorForeignOrg}, h.users.scopedOrgs)
	require.Len(t, h.sessions.listQueries, 1)
	assert.Equal(t, idorForeignOrg, h.sessions.listQueries[0].OrganizationID)
	assert.Equal(t, 1, h.sessions.revokeUserCalls)
	require.Len(t, h.records.created, 1)
	assert.Equal(t, idorForeignOrg, h.records.created[0].OrganizationID)
}
