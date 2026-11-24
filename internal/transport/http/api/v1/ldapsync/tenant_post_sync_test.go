package ldapsync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	postLdapSyncTenantOrg  = "11111111-1111-1111-1111-111111111111"
	postLdapSyncForeignOrg = "22222222-2222-2222-2222-222222222222"
	postLdapSyncOwnID      = "33333333-3333-3333-3333-333333333333"
	postLdapSyncForeignID  = "44444444-4444-4444-4444-444444444444"
)

var errPostLdapSyncTaskUnavailable = errors.New("sync task storage unavailable")

// postLdapSyncRepo keeps both lookups observable: the system-scoped read is the
// pre-fix path, the scoped read is the tenant-bound path.
type postLdapSyncRepo struct {
	ldaps.SyncRepository
	configs     map[string]*ldaps.LdapSyncConfig
	systemGets  int
	scopedGets  int
	scopedOrg   string
	taskCreates int
	taskCreated *ldaps.LdapSyncTask
}

func (r *postLdapSyncRepo) GetSyncConfigByID(_ context.Context, id string) (*ldaps.LdapSyncConfig, error) {
	r.systemGets++
	if config, ok := r.configs[id]; ok {
		return config, nil
	}
	return nil, apperrors.NewNotFound("ldap sync config not found")
}

func (r *postLdapSyncRepo) GetSyncConfigByIDForOrganization(_ context.Context, id, organizationID string) (*ldaps.LdapSyncConfig, error) {
	r.scopedGets++
	r.scopedOrg = organizationID
	if config, ok := r.configs[id]; ok && config.OrganizationID == organizationID {
		return config, nil
	}
	return nil, apperrors.NewNotFound("ldap sync config not found")
}

func (r *postLdapSyncRepo) CreateSyncTask(_ context.Context, task *ldaps.LdapSyncTask) error {
	r.taskCreates++
	r.taskCreated = task
	return errPostLdapSyncTaskUnavailable
}

type postLdapSyncHarness struct {
	repo   *postLdapSyncRepo
	router chi.Router
}

func newPostLdapSyncHarness(configs ...*ldaps.LdapSyncConfig) *postLdapSyncHarness {
	inventory := make(map[string]*ldaps.LdapSyncConfig, len(configs))
	for _, config := range configs {
		inventory[config.ID] = config
	}
	repo := &postLdapSyncRepo{configs: inventory}
	router := chi.NewRouter()
	RegisterRoutes(router, NewHandler(appldapsync.NewService(repo, nil, nil)))
	return &postLdapSyncHarness{repo: repo, router: router}
}

func postLdapSyncRequest(target, organizationID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, nil)
	if organizationID != "" {
		ctx := tenantctx.WithTenantScope(req.Context(), organizationID, "session-org", tenantctx.TenantFilterStrict)
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
			UserID: "55555555-5555-5555-5555-555555555555", OrganizationID: organizationID,
		})
		req = req.WithContext(ctx)
	}
	return req
}

func postLdapSyncPlatformRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, nil)
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func servePostLdapSync(t *testing.T, harness *postLdapSyncHarness, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	harness.router.ServeHTTP(recorder, req)
	return recorder
}

func postLdapSyncConfig(id, organizationID string) *ldaps.LdapSyncConfig {
	return &ldaps.LdapSyncConfig{
		ID: id, OrganizationID: organizationID, LdapID: "ldap-" + id,
		SyncIntervalMinutes: 15, Enabled: true,
	}
}

// TestLdapSyncTrigger_ForeignTenantRejected pins that POST /ldap-sync-configs/
// {id}/sync is tenant-bound: a foreign config is a 404 and never creates a task.
func TestLdapSyncTrigger_ForeignTenantRejected(t *testing.T) {
	harness := newPostLdapSyncHarness(postLdapSyncConfig(postLdapSyncForeignID, postLdapSyncForeignOrg))

	recorder := servePostLdapSync(t, harness, postLdapSyncRequest(
		"/api/v1/ldap-sync-configs/"+postLdapSyncForeignID+"/sync?organizationId="+postLdapSyncForeignOrg,
		postLdapSyncTenantOrg))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign config must look missing")
	assert.Zero(t, harness.repo.systemGets, "the system-scoped lookup must not be used")
	assert.Zero(t, harness.repo.taskCreates, "foreign tenant must not create a sync task")
}

// TestLdapSyncTrigger_WithoutTenantScopeRejected pins the fail-fast.
func TestLdapSyncTrigger_WithoutTenantScopeRejected(t *testing.T) {
	harness := newPostLdapSyncHarness(postLdapSyncConfig(postLdapSyncOwnID, postLdapSyncTenantOrg))

	recorder := servePostLdapSync(t, harness, postLdapSyncRequest(
		"/api/v1/ldap-sync-configs/"+postLdapSyncOwnID+"/sync", ""))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Zero(t, harness.repo.systemGets)
	assert.Zero(t, harness.repo.scopedGets, "an empty tenant must not reach the repository")
	assert.Zero(t, harness.repo.taskCreates)
}

// TestLdapSyncTrigger_OwnTenantUsesScopedConfig pins that the same-tenant path
// resolves the config through the tenant-scoped read and carries the entity's
// real organization into the task. Task storage is stubbed to fail so this
// never attempts a real LDAP connection.
func TestLdapSyncTrigger_OwnTenantUsesScopedConfig(t *testing.T) {
	harness := newPostLdapSyncHarness(postLdapSyncConfig(postLdapSyncOwnID, postLdapSyncTenantOrg))

	recorder := servePostLdapSync(t, harness, postLdapSyncRequest(
		"/api/v1/ldap-sync-configs/"+postLdapSyncOwnID+"/sync", postLdapSyncTenantOrg))

	assert.NotEqual(t, http.StatusNotFound, recorder.Code, "own config must not look missing")
	assert.NotEqual(t, http.StatusBadRequest, recorder.Code)
	assert.Equal(t, 1, harness.repo.scopedGets)
	assert.Equal(t, postLdapSyncTenantOrg, harness.repo.scopedOrg)
	assert.Zero(t, harness.repo.systemGets, "the system-scoped lookup must not be used")
	require.NotNil(t, harness.repo.taskCreated)
	assert.Equal(t, postLdapSyncTenantOrg, harness.repo.taskCreated.OrganizationID,
		"the task must carry the config's real organization")
}

// TestLdapSyncTrigger_PlatformAdminCanTargetTenant pins the single legit
// override: super_admin/system scopes may name another tenant explicitly.
func TestLdapSyncTrigger_PlatformAdminCanTargetTenant(t *testing.T) {
	harness := newPostLdapSyncHarness(postLdapSyncConfig(postLdapSyncForeignID, postLdapSyncForeignOrg))

	recorder := servePostLdapSync(t, harness, postLdapSyncPlatformRequest(
		"/api/v1/ldap-sync-configs/"+postLdapSyncForeignID+"/sync?organizationId="+postLdapSyncForeignOrg))

	assert.NotEqual(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, 1, harness.repo.scopedGets)
	assert.Equal(t, postLdapSyncForeignOrg, harness.repo.scopedOrg)
}
