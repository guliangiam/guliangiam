package ldapsync

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorLdapTenantOrg  = "11111111-1111-1111-1111-111111111111"
	idorLdapForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorLdapForeignID  = "foreign-ldap-sync-config"
	idorLdapOwnID      = "own-ldap-sync-config"
)

// idorLdapSyncRepo is a tenant-aware in-memory SyncRepository. Write counts
// stay observable so a rejected cross-tenant request proves it never wrote.
type idorLdapSyncRepo struct {
	ldaps.SyncRepository
	configs   map[string]*ldaps.LdapSyncConfig
	tasks     []*ldaps.LdapSyncTask
	updateCnt int
	deleteCnt int
}

func (r *idorLdapSyncRepo) GetSyncConfigByID(_ context.Context, id string) (*ldaps.LdapSyncConfig, error) {
	if config, ok := r.configs[id]; ok {
		return config, nil
	}
	return nil, apperrors.NewNotFound("ldap sync config not found")
}

func (r *idorLdapSyncRepo) GetSyncConfigByIDForOrganization(_ context.Context, id, organizationID string) (*ldaps.LdapSyncConfig, error) {
	if config, ok := r.configs[id]; ok && config.OrganizationID == organizationID {
		return config, nil
	}
	return nil, apperrors.NewNotFound("ldap sync config not found")
}

func (r *idorLdapSyncRepo) UpdateSyncConfig(_ context.Context, config *ldaps.LdapSyncConfig) error {
	r.updateCnt++
	if existing, ok := r.configs[config.ID]; ok && existing.OrganizationID == config.OrganizationID {
		r.configs[config.ID] = config
		return nil
	}
	return apperrors.NewNotFound("ldap sync config not found")
}

func (r *idorLdapSyncRepo) DeleteSyncConfig(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	if config, ok := r.configs[id]; ok && config.OrganizationID == organizationID {
		delete(r.configs, id)
		return nil
	}
	return apperrors.NewNotFound("ldap sync config not found")
}

func (r *idorLdapSyncRepo) ListSyncTasksForOrganization(_ context.Context, ldapID, organizationID string, limit int) ([]*ldaps.LdapSyncTask, error) {
	var out []*ldaps.LdapSyncTask
	for _, task := range r.tasks {
		if task.LdapID == ldapID && task.OrganizationID == organizationID {
			out = append(out, task)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func idorLdapRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorLdapTenantOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: idorLdapTenantOrg,
	})
	return req.WithContext(ctx)
}

func idorLdapPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorLdapTenantOrg, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func idorLdapBareRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func idorLdapSyncRouter(repo ldaps.SyncRepository) chi.Router {
	router := chi.NewRouter()
	RegisterRoutes(router, NewHandler(appldapsync.NewService(repo, nil, nil)))
	return router
}

func idorLdapSyncConfig(id, org string) *ldaps.LdapSyncConfig {
	return &ldaps.LdapSyncConfig{
		ID: id, OrganizationID: org, LdapID: "ldap-" + id, Enabled: true, SyncIntervalMinutes: 15,
	}
}

func serveIdorLdap(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestLdapSyncByID_ForeignTenantRejected pins that GET/PATCH/DELETE and /logs
// resolve the authenticated tenant, never the caller-supplied organizationId,
// and that a foreign config looks exactly like a missing one.
func TestLdapSyncByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/ldap-sync-configs/" + idorLdapForeignID + "?organizationId=" + idorLdapForeignOrg, ""},
		{"patch", http.MethodPatch, "/api/v1/ldap-sync-configs/" + idorLdapForeignID + "?organizationId=" + idorLdapForeignOrg, `{"enabled":true}`},
		{"delete", http.MethodDelete, "/api/v1/ldap-sync-configs/" + idorLdapForeignID + "?organizationId=" + idorLdapForeignOrg, ""},
		{"logs", http.MethodGet, "/api/v1/ldap-sync-configs/" + idorLdapForeignID + "/logs?organizationId=" + idorLdapForeignOrg, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorLdapSyncRepo{configs: map[string]*ldaps.LdapSyncConfig{
				idorLdapForeignID: idorLdapSyncConfig(idorLdapForeignID, idorLdapForeignOrg),
			}}
			repo.tasks = []*ldaps.LdapSyncTask{{
				ID: "task-1", OrganizationID: idorLdapForeignOrg, LdapID: "ldap-" + idorLdapForeignID,
			}}

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveIdorLdap(t, idorLdapSyncRouter(repo), idorLdapRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign config must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
			require.Len(t, repo.configs, 1, "foreign config row must remain untouched")
		})
	}
}

// TestLdapSyncByID_WithoutTenantScopeRejected pins the fail-fast: an empty
// trusted tenant is rejected before touching the repository (MySQL has no RLS).
func TestLdapSyncByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/ldap-sync-configs/" + idorLdapOwnID, ""},
		{"patch", http.MethodPatch, "/api/v1/ldap-sync-configs/" + idorLdapOwnID, `{"enabled":true}`},
		{"delete", http.MethodDelete, "/api/v1/ldap-sync-configs/" + idorLdapOwnID, ""},
		{"logs", http.MethodGet, "/api/v1/ldap-sync-configs/" + idorLdapOwnID + "/logs", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &idorLdapSyncRepo{configs: map[string]*ldaps.LdapSyncConfig{
				idorLdapOwnID: idorLdapSyncConfig(idorLdapOwnID, idorLdapTenantOrg),
			}}
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveIdorLdap(t, idorLdapSyncRouter(repo), idorLdapBareRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.updateCnt)
			assert.Zero(t, repo.deleteCnt)
		})
	}
}

// TestLdapSyncByID_OwnTenantSucceeds pins same-tenant GET/PATCH/DELETE/logs.
func TestLdapSyncByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &idorLdapSyncRepo{configs: map[string]*ldaps.LdapSyncConfig{
			idorLdapOwnID: idorLdapSyncConfig(idorLdapOwnID, idorLdapTenantOrg),
		}}
		recorder := serveIdorLdap(t, idorLdapSyncRouter(repo),
			idorLdapRequest(http.MethodGet, "/api/v1/ldap-sync-configs/"+idorLdapOwnID, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), idorLdapTenantOrg)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &idorLdapSyncRepo{configs: map[string]*ldaps.LdapSyncConfig{
			idorLdapOwnID: idorLdapSyncConfig(idorLdapOwnID, idorLdapTenantOrg),
		}}
		recorder := serveIdorLdap(t, idorLdapSyncRouter(repo),
			idorLdapRequest(http.MethodPatch, "/api/v1/ldap-sync-configs/"+idorLdapOwnID,
				strings.NewReader(`{"enabled":false}`)))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		assert.False(t, repo.configs[idorLdapOwnID].Enabled)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &idorLdapSyncRepo{configs: map[string]*ldaps.LdapSyncConfig{
			idorLdapOwnID: idorLdapSyncConfig(idorLdapOwnID, idorLdapTenantOrg),
		}}
		recorder := serveIdorLdap(t, idorLdapSyncRouter(repo),
			idorLdapRequest(http.MethodDelete, "/api/v1/ldap-sync-configs/"+idorLdapOwnID, nil))
		require.Equal(t, http.StatusNoContent, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Empty(t, repo.configs)
	})

	t.Run("logs", func(t *testing.T) {
		repo := &idorLdapSyncRepo{configs: map[string]*ldaps.LdapSyncConfig{
			idorLdapOwnID: idorLdapSyncConfig(idorLdapOwnID, idorLdapTenantOrg),
		}}
		repo.tasks = []*ldaps.LdapSyncTask{{
			ID: "task-1", OrganizationID: idorLdapTenantOrg, LdapID: "ldap-" + idorLdapOwnID,
		}}
		recorder := serveIdorLdap(t, idorLdapSyncRouter(repo),
			idorLdapRequest(http.MethodGet, "/api/v1/ldap-sync-configs/"+idorLdapOwnID+"/logs", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "task-1")
	})
}

// TestLdapSyncByID_PlatformAdminCanCrossTenant pins the single legitimate
// override: super_admin/system scopes may name another tenant explicitly.
func TestLdapSyncByID_PlatformAdminCanCrossTenant(t *testing.T) {
	repo := &idorLdapSyncRepo{configs: map[string]*ldaps.LdapSyncConfig{
		idorLdapForeignID: idorLdapSyncConfig(idorLdapForeignID, idorLdapForeignOrg),
	}}
	recorder := serveIdorLdap(t, idorLdapSyncRouter(repo),
		idorLdapPlatformRequest(http.MethodGet, "/api/v1/ldap-sync-configs/"+idorLdapForeignID+"?organizationId="+idorLdapForeignOrg, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), idorLdapForeignOrg)
}
