package ldapsync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

// forgedSyncRepo records the organization each sync-config operation used.
// The embedded nil interface covers methods these routes do not exercise.
type forgedSyncRepo struct {
	ldaps.SyncRepository
	org      string
	lastOp   string
	lastID   string
	lastLDAP string
}

func (r *forgedSyncRepo) ListSyncConfigs(_ context.Context, organizationID string) ([]*ldaps.LdapSyncConfig, error) {
	r.org, r.lastOp = organizationID, "ListSyncConfigs"
	return nil, nil
}

func (r *forgedSyncRepo) CreateSyncConfig(_ context.Context, config *ldaps.LdapSyncConfig) error {
	r.org, r.lastOp, r.lastID, r.lastLDAP = config.OrganizationID, "CreateSyncConfig", config.ID, config.LdapID
	return nil
}

func forgedOrgRouter(repo ldaps.SyncRepository) chi.Router {
	router := chi.NewRouter()
	RegisterRoutes(router, NewHandler(appldapsync.NewService(repo, nil, nil)))
	return router
}

// TestListSyncConfigs_ForgedQueryOrganizationIDIgnored pins the list route.
func TestListSyncConfigs_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	repo := &forgedSyncRepo{}
	recorder := httptest.NewRecorder()
	forgedOrgRouter(repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/ldap-sync-configs/?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "ListSyncConfigs", repo.lastOp)
	assert.Equal(t, forgedOrgSessionID, repo.org)
}

// TestCreateSyncConfig_ForgedBodyOrganizationIDIgnored pins the create route:
// the body's organizationId must not move the config into another tenant.
func TestCreateSyncConfig_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	repo := &forgedSyncRepo{}
	recorder := httptest.NewRecorder()
	forgedOrgRouter(repo).ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/ldap-sync-configs/",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","ldapId":"ldap-1","syncIntervalMinutes":15,"baseDn":"dc=example,dc=com","enabled":true}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, "CreateSyncConfig", repo.lastOp)
	assert.Equal(t, forgedOrgSessionID, repo.org)
	assert.Equal(t, "ldap-1", repo.lastLDAP)

	var body struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body), recorder.Body.String())
	require.Equal(t, repo.lastID, body.ID, "the body must still carry the created config id")
	assert.Equal(t, "/api/v1/ldap-sync-configs/"+body.ID, recorder.Header().Get("Location"))
}

// TestCreateSyncConfig_WithoutTenantScopeRejected pins the fail-fast guard.
func TestCreateSyncConfig_WithoutTenantScopeRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ldap-sync-configs/",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","ldapId":"ldap-1","syncIntervalMinutes":15}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router := chi.NewRouter()
	RegisterRoutes(router, NewHandler(appldapsync.NewService(&forgedSyncRepo{}, nil, nil)))
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
