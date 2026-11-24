package actions

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appactions "github.com/guliangiam/guliangiam/internal/app/actions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/actions"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	trustedOrg = "11111111-1111-1111-1111-111111111111"
	forgedOrg  = "22222222-2222-2222-2222-222222222222"
	ownAction  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	otherAct   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
)

type actionLookup struct {
	actionID       string
	organizationID string
}

// tenantRepo is a domain.Repository double: it owns a fixed action → tenant
// map, records the tenant each read receives, and returns no rows when the
// requested action belongs to another tenant (what PG/MySQL both do once the
// tenant predicate is in place).
type tenantRepo struct {
	owners      map[string]string
	notFoundErr error

	getCalls         []actionLookup
	listOrg          string
	versionQuery     *domain.ListActionVersionsQuery
	execQuery        *domain.ListExecutionLogsQuery
	versionListCalls int
	execListCalls    int
}

func newTenantRepo() *tenantRepo {
	return &tenantRepo{owners: map[string]string{}, notFoundErr: sql.ErrNoRows}
}

func (r *tenantRepo) Get(_ context.Context, actionID, organizationID string) (*domain.Action, error) {
	r.getCalls = append(r.getCalls, actionLookup{actionID: actionID, organizationID: organizationID})
	if owner, ok := r.owners[actionID]; ok && owner == organizationID {
		return &domain.Action{
			ID:             actionID,
			OrganizationID: owner,
			Name:           "owned-action",
			Status:         domain.ActionStatusActive,
			Revision:       1,
		}, nil
	}
	if r.notFoundErr != nil {
		return nil, r.notFoundErr
	}
	return nil, sql.ErrNoRows
}

func (r *tenantRepo) List(_ context.Context, q domain.ListActionsQuery) ([]*domain.Action, int64, error) {
	r.listOrg = q.OrganizationID
	return []*domain.Action{{
		ID:             "listed-action",
		OrganizationID: q.OrganizationID,
		Name:           "listed-action",
		Status:         domain.ActionStatusActive,
		Revision:       1,
	}}, 1, nil
}

func (r *tenantRepo) ListVersions(_ context.Context, q domain.ListActionVersionsQuery) ([]*domain.ActionVersion, int64, error) {
	r.versionListCalls++
	r.versionQuery = &q
	return []*domain.ActionVersion{{ID: "ver-1", ActionID: q.ActionID, Version: 1, Code: "return 1;"}}, 1, nil
}

func (r *tenantRepo) ListExecutionLogs(_ context.Context, q domain.ListExecutionLogsQuery) ([]*domain.ExecutionLog, int64, error) {
	r.execListCalls++
	r.execQuery = &q
	return []*domain.ExecutionLog{{
		ID:       "log-1",
		ActionID: q.ActionID,
		Version:  1,
		Status:   domain.ExecStatusSuccess,
		Input:    json.RawMessage(`{"subject":"self"}`),
	}}, 1, nil
}

func (r *tenantRepo) Create(context.Context, *domain.Action, *domain.ActionVersion) error {
	return nil
}

func (r *tenantRepo) UpdateStatus(context.Context, domain.UpdateStatusCommand) error { return nil }

func (r *tenantRepo) PublishVersion(context.Context, string, string, *domain.ActionVersion) error {
	return nil
}

func (r *tenantRepo) GetLatestVersion(context.Context, string, string) (*domain.ActionVersion, error) {
	return nil, nil
}

func (r *tenantRepo) CreateExecutionLog(context.Context, *domain.ExecutionLog) error { return nil }

func (r *tenantRepo) UpdateExecutionLog(context.Context, domain.UpdateExecutionLogCommand) error {
	return nil
}

// stubActionsUoW satisfies appuow.ActionsUoW; the read paths never open a
// transaction.
type stubActionsUoW struct{ repo domain.Repository }

func (u stubActionsUoW) Actions() domain.Repository { return u.repo }

func (u stubActionsUoW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, nil)
}

func tenantRouter(repo domain.Repository) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appactions.Service{UOW: stubActionsUoW{repo: repo}}})
	return router
}

// actionsRequest attaches the middleware-provided tenant scope and session the
// way protected routes see them. Empty mode/session leaves that layer absent.
func actionsRequest(target string, mode tenantctx.TenantFilterMode, scopeOrg, sessionOrg string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	ctx := req.Context()
	if mode != "" {
		ctx = tenantctx.WithTenantScope(ctx, scopeOrg, "scope-org", mode)
	}
	if sessionOrg != "" {
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{OrganizationID: sessionOrg})
	}
	return req.WithContext(ctx)
}

func errorCode(t *testing.T, body []byte) string {
	t.Helper()
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Error.Code
}

// TestActions_ForgedQueryOrganizationIDIgnored pins that a regular caller
// cannot move the list to another tenant through the query string. The
// session-only case covers the TrustedOrganizationID fallback used when no
// tenant scope is present.
func TestActions_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	cases := []struct {
		name       string
		mode       tenantctx.TenantFilterMode
		sessionOrg string
	}{
		{name: "tenant scope", mode: tenantctx.TenantFilterStrict, sessionOrg: trustedOrg},
		{name: "session fallback", sessionOrg: trustedOrg},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := newTenantRepo()
			recorder := httptest.NewRecorder()

			tenantRouter(repo).ServeHTTP(recorder, actionsRequest(
				"/api/v1/actions?organizationId="+forgedOrg, tc.mode, trustedOrg, tc.sessionOrg))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			assert.Equal(t, trustedOrg, repo.listOrg)
		})
	}
}

// TestActions_EmptyTrustedOrganizationRejected pins the fail-fast: without an
// authenticated tenant the forged query value must not become the tenant.
func TestActions_EmptyTrustedOrganizationRejected(t *testing.T) {
	repo := newTenantRepo()
	recorder := httptest.NewRecorder()

	tenantRouter(repo).ServeHTTP(recorder, actionsRequest(
		"/api/v1/actions?organizationId="+forgedOrg, "", "", ""))

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Equal(t, "invalid_argument", errorCode(t, recorder.Body.Bytes()))
	assert.Empty(t, repo.listOrg)
}

// TestActionVersions_ForeignTenantActionIsNotFound pins the app-layer parent
// check and response shape: another tenant's action id yields 404, the lookup
// uses the trusted tenant, and no version payload is emitted.
func TestActionVersions_ForeignTenantActionIsNotFound(t *testing.T) {
	repo := newTenantRepo()
	repo.owners[otherAct] = forgedOrg
	repo.notFoundErr = pgx.ErrNoRows
	recorder := httptest.NewRecorder()

	tenantRouter(repo).ServeHTTP(recorder, actionsRequest(
		"/api/v1/actions/"+otherAct+"/versions?organizationId="+forgedOrg,
		tenantctx.TenantFilterStrict, trustedOrg, trustedOrg))

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	assert.Equal(t, "not_found", errorCode(t, recorder.Body.Bytes()))
	require.Len(t, repo.getCalls, 1)
	assert.Equal(t, actionLookup{actionID: otherAct, organizationID: trustedOrg}, repo.getCalls[0])
	assert.Zero(t, repo.versionListCalls)
	assert.NotContains(t, recorder.Body.String(), "actionId")
	assert.NotContains(t, recorder.Body.String(), "createdAt")
}

// TestActionExecutionLogs_ForeignTenantActionIsNotFound is the execution-log
// twin: the MySQL no-rows sentinel is mapped to the same 404 and no input /
// output fields leak.
func TestActionExecutionLogs_ForeignTenantActionIsNotFound(t *testing.T) {
	repo := newTenantRepo()
	repo.owners[otherAct] = forgedOrg
	repo.notFoundErr = sql.ErrNoRows
	recorder := httptest.NewRecorder()

	tenantRouter(repo).ServeHTTP(recorder, actionsRequest(
		"/api/v1/actions/"+otherAct+"/execution-logs?organizationId="+forgedOrg,
		tenantctx.TenantFilterStrict, trustedOrg, trustedOrg))

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	assert.Equal(t, "not_found", errorCode(t, recorder.Body.Bytes()))
	require.Len(t, repo.getCalls, 1)
	assert.Equal(t, actionLookup{actionID: otherAct, organizationID: trustedOrg}, repo.getCalls[0])
	assert.Zero(t, repo.execListCalls)
	assert.NotContains(t, recorder.Body.String(), `"input"`)
	assert.NotContains(t, recorder.Body.String(), `"output"`)
}

// TestActionVersions_SameTenantListStillWorks pins the non-regression path:
// the parent belongs to the trusted tenant and the child query receives that
// tenant even when the caller forges the query value.
func TestActionVersions_SameTenantListStillWorks(t *testing.T) {
	repo := newTenantRepo()
	repo.owners[ownAction] = trustedOrg
	recorder := httptest.NewRecorder()

	tenantRouter(repo).ServeHTTP(recorder, actionsRequest(
		"/api/v1/actions/"+ownAction+"/versions?organizationId="+forgedOrg,
		tenantctx.TenantFilterStrict, trustedOrg, trustedOrg))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, repo.versionQuery)
	assert.Equal(t, trustedOrg, repo.versionQuery.OrganizationID)
	assert.Equal(t, ownAction, repo.versionQuery.ActionID)
	assert.Contains(t, recorder.Body.String(), "ver-1")
}

// TestActionExecutionLogs_SameTenantListStillWorks is the execution-log twin.
func TestActionExecutionLogs_SameTenantListStillWorks(t *testing.T) {
	repo := newTenantRepo()
	repo.owners[ownAction] = trustedOrg
	recorder := httptest.NewRecorder()

	tenantRouter(repo).ServeHTTP(recorder, actionsRequest(
		"/api/v1/actions/"+ownAction+"/execution-logs?organizationId="+forgedOrg,
		tenantctx.TenantFilterStrict, trustedOrg, trustedOrg))

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotNil(t, repo.execQuery)
	assert.Equal(t, trustedOrg, repo.execQuery.OrganizationID)
	assert.Equal(t, ownAction, repo.execQuery.ActionID)
	assert.Contains(t, recorder.Body.String(), "log-1")
}

// TestActionVersions_PlatformAdminExplicitTenantHonoured pins that the query
// override remains available to an escalated administrator and that both
// super_admin and system scopes are accepted.
func TestActionVersions_PlatformAdminExplicitTenantHonoured(t *testing.T) {
	modes := []tenantctx.TenantFilterMode{
		tenantctx.TenantFilterSuperAdmin,
		tenantctx.TenantFilterSystem,
	}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			repo := newTenantRepo()
			repo.owners[otherAct] = forgedOrg
			recorder := httptest.NewRecorder()

			tenantRouter(repo).ServeHTTP(recorder, actionsRequest(
				"/api/v1/actions/"+otherAct+"/versions?organizationId="+forgedOrg,
				mode, trustedOrg, trustedOrg))

			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			require.Len(t, repo.getCalls, 1)
			assert.Equal(t, forgedOrg, repo.getCalls[0].organizationID)
			require.NotNil(t, repo.versionQuery)
			assert.Equal(t, forgedOrg, repo.versionQuery.OrganizationID)
		})
	}
}
