package adapters

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appadapters "github.com/guliangiam/guliangiam/internal/app/adapters"
	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	permissionsmock "github.com/guliangiam/guliangiam/internal/domain/permissions/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const foreignAdapterID = "foreign-adapter"

// stubConnectionTester stands in for the infra probe port (BE-B-16): the
// transport layer only sees the (connected, failure) result contract.
type stubConnectionTester struct{}

func (stubConnectionTester) TestDatabaseConnection(_ context.Context, _, host string, _ int, _, _, _ string) (bool, string) {
	if host == "" {
		return false, "host is required"
	}
	return true, ""
}

// TestAdapterByID_ForeignTenantRejected pins that every by-ID operation
// resolves the authenticated tenant, never the caller-supplied organizationId,
// and that a foreign adapter looks exactly like a missing one: 404, no write,
// no audit record, and the row untouched.
func TestAdapterByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/adapters/" + foreignAdapterID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/adapters/" + foreignAdapterID + "?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/adapters/" + foreignAdapterID + "?organizationId=" + forgedOrgRequestID, ""},
		{"test-connection", http.MethodPost, "/api/v1/adapters/" + foreignAdapterID + "/test-connection?organizationId=" + forgedOrgRequestID, `{}`},
		{"get-policies", http.MethodGet, "/api/v1/adapters/" + foreignAdapterID + "/policies?organizationId=" + forgedOrgRequestID, ""},
		{"get-filtered-policies", http.MethodGet, "/api/v1/adapters/" + foreignAdapterID + "/policies?organizationId=" + forgedOrgRequestID + "&fieldIndex=0&fieldValue=alice", ""},
		{"add-policy", http.MethodPost, "/api/v1/adapters/" + foreignAdapterID + "/policies?organizationId=" + forgedOrgRequestID, `{"pType":"p","rules":["bob","acme","orders","get"]}`},
		{"remove-policy", http.MethodDelete, "/api/v1/adapters/" + foreignAdapterID + "/policies?organizationId=" + forgedOrgRequestID, `{"pType":"p","rules":["bob","acme","orders","get"]}`},
		{"update-policy", http.MethodPut, "/api/v1/adapters/" + foreignAdapterID + "/policies?organizationId=" + forgedOrgRequestID, `{"oldPolicy":["p","alice"],"newPolicy":["p","bob"]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(foreignAdapterID, forgedOrgRequestID)}}
			service, records := newAdapterScopeService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := forgedOrgRequest(tt.method, tt.target, body)
			recorder := serveAdapterRequest(t, adapterRouter(service), req)

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign adapter must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the adapter")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the adapter")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			require.Len(t, repo.adapters, 1, "foreign adapter row must remain untouched")
			assert.NotContains(t, recorder.Body.String(), "super-secret-password")
			assert.NotContains(t, recorder.Body.String(), "super-secret-db-password")
		})
	}
}

// TestAdapterByID_WithoutTenantScopeRejected pins the fail-fast: when neither
// the session nor a platform-admin target provides a tenant, every by-ID route
// is rejected before touching the repository (MySQL has no RLS fallback).
func TestAdapterByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter("any-adapter", forgedOrgSessionID)}}
	service, _ := newAdapterScopeService(repo)
	router := adapterRouter(service)

	requests := map[string]*http.Request{
		"get":                   httptest.NewRequest(http.MethodGet, "/api/v1/adapters/any-adapter", nil),
		"patch":                 httptest.NewRequest(http.MethodPatch, "/api/v1/adapters/any-adapter", strings.NewReader(`{}`)),
		"delete":                httptest.NewRequest(http.MethodDelete, "/api/v1/adapters/any-adapter", nil),
		"test-connection":       httptest.NewRequest(http.MethodPost, "/api/v1/adapters/any-adapter/test-connection", strings.NewReader(`{}`)),
		"get-policies":          httptest.NewRequest(http.MethodGet, "/api/v1/adapters/any-adapter/policies", nil),
		"add-policy":            httptest.NewRequest(http.MethodPost, "/api/v1/adapters/any-adapter/policies", strings.NewReader(`{}`)),
		"remove-policy":         httptest.NewRequest(http.MethodDelete, "/api/v1/adapters/any-adapter/policies", strings.NewReader(`{}`)),
		"update-policy":         httptest.NewRequest(http.MethodPut, "/api/v1/adapters/any-adapter/policies", strings.NewReader(`{}`)),
		"list-without-scope":    httptest.NewRequest(http.MethodGet, "/api/v1/adapters", nil),
		"list-forged-query-org": httptest.NewRequest(http.MethodGet, "/api/v1/adapters?organizationId="+forgedOrgRequestID, nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveAdapterRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
}

// TestAdapterByID_OwnTenantSucceeds pins that the same operations still work
// for the authenticated tenant and that a delete audits the real organization.
func TestAdapterByID_OwnTenantSucceeds(t *testing.T) {
	const ownAdapterID = "own-adapter"

	t.Run("get", func(t *testing.T) {
		repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(ownAdapterID, forgedOrgSessionID)}}
		service, _ := newAdapterScopeService(repo)
		recorder := serveAdapterRequest(t, adapterRouter(service),
			forgedOrgRequest(http.MethodGet, "/api/v1/adapters/"+ownAdapterID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+forgedOrgSessionID+`"`)
		assert.Contains(t, recorder.Body.String(), "dbPasswordMasked")
		assert.NotContains(t, recorder.Body.String(), "super-secret-password")
		assert.NotContains(t, recorder.Body.String(), "super-secret-db-password")
	})

	t.Run("patch", func(t *testing.T) {
		repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(ownAdapterID, forgedOrgSessionID)}}
		service, records := newAdapterScopeService(repo)
		recorder := serveAdapterRequest(t, adapterRouter(service),
			forgedOrgRequest(http.MethodPatch, "/api/v1/adapters/"+ownAdapterID,
				strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(ownAdapterID, forgedOrgSessionID)}}
		service, records := newAdapterScopeService(repo)
		recorder := serveAdapterRequest(t, adapterRouter(service),
			forgedOrgRequest(http.MethodDelete, "/api/v1/adapters/"+ownAdapterID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID, "delete audit must carry the adapter organization")
	})

	t.Run("test-connection", func(t *testing.T) {
		own := tenantAdapter(ownAdapterID, forgedOrgSessionID)
		own.Host = ""
		own.Server = ""
		repo := &tenantByIDRepo{adapters: []*domain.Adapter{own}}
		service, _ := newAdapterScopeService(repo)
		service.ConnectionTester = stubConnectionTester{}
		recorder := serveAdapterRequest(t, adapterRouter(service),
			forgedOrgRequest(http.MethodPost, "/api/v1/adapters/"+ownAdapterID+"/test-connection", strings.NewReader(`{}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "host is required")
	})

	t.Run("policies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		casbin := permissionsmock.NewMockCasbinPolicyManager(ctrl)
		casbin.EXPECT().GetPolicies(ownAdapterID, "").Return([][]string{{"p", "alice", "acme", "orders", "get"}}, nil)
		casbin.EXPECT().GetFilteredPolicies(ownAdapterID, "", 0, []string{"alice"}).Return([][]string{{"p", "alice", "acme", "orders", "get"}}, nil)
		casbin.EXPECT().AddPolicy(ownAdapterID, "", []string{"p", "bob", "acme", "orders", "get"}).Return(nil)
		casbin.EXPECT().RemovePolicy(ownAdapterID, "", []string{"p", "bob", "acme", "orders", "get"}).Return(nil)
		casbin.EXPECT().UpdatePolicy(ownAdapterID, "", []string{"p", "alice"}, []string{"p", "bob"}).Return(nil)

		repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(ownAdapterID, forgedOrgSessionID)}}
		records := &recordingRecords{}
		service := appadapters.NewService(forgedOrgClock{}, forgedOrgUUID{}, &adapterScopeUOW{repo: repo, records: records}, casbin)
		router := adapterRouter(service)

		recorder := serveAdapterRequest(t, router,
			forgedOrgRequest(http.MethodGet, "/api/v1/adapters/"+ownAdapterID+"/policies", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "alice")

		recorder = serveAdapterRequest(t, router,
			forgedOrgRequest(http.MethodGet, "/api/v1/adapters/"+ownAdapterID+"/policies?fieldIndex=0&fieldValue=alice", nil))
		require.Equal(t, http.StatusOK, recorder.Code)

		recorder = serveAdapterRequest(t, router,
			forgedOrgRequest(http.MethodPost, "/api/v1/adapters/"+ownAdapterID+"/policies",
				strings.NewReader(`{"pType":"p","rules":["bob","acme","orders","get"]}`)))
		require.Equal(t, http.StatusCreated, recorder.Code)

		recorder = serveAdapterRequest(t, router,
			forgedOrgRequest(http.MethodDelete, "/api/v1/adapters/"+ownAdapterID+"/policies",
				strings.NewReader(`{"pType":"p","rules":["bob","acme","orders","get"]}`)))
		require.Equal(t, http.StatusOK, recorder.Code)

		recorder = serveAdapterRequest(t, router,
			forgedOrgRequest(http.MethodPut, "/api/v1/adapters/"+ownAdapterID+"/policies",
				strings.NewReader(`{"oldPolicy":["p","alice"],"newPolicy":["p","bob"]}`)))
		require.Equal(t, http.StatusOK, recorder.Code)
	})
}

// TestAdapterByID_PlatformAdminCanCrossTenant pins the single legitimate
// override: super_admin/system scopes may name another tenant explicitly.
func TestAdapterByID_PlatformAdminCanCrossTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(foreignAdapterID, forgedOrgRequestID)}}
		service, _ := newAdapterScopeService(repo)
		recorder := serveAdapterRequest(t, adapterRouter(service),
			platformRequest(http.MethodGet, "/api/v1/adapters/"+foreignAdapterID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), forgedOrgRequestID)
	})

	t.Run("patch keeps the adapter tenant", func(t *testing.T) {
		repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(foreignAdapterID, forgedOrgRequestID)}}
		service, records := newAdapterScopeService(repo)
		recorder := serveAdapterRequest(t, adapterRouter(service),
			platformRequest(http.MethodPatch, "/api/v1/adapters/"+foreignAdapterID+"?organizationId="+forgedOrgRequestID,
				strings.NewReader(`{"displayName":"admin-renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, repo.adapters, 1)
		assert.Equal(t, forgedOrgRequestID, repo.adapters[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgRequestID, records.created[0].OrganizationID)
	})

	t.Run("delete audits the adapter tenant", func(t *testing.T) {
		repo := &tenantByIDRepo{adapters: []*domain.Adapter{tenantAdapter(foreignAdapterID, forgedOrgRequestID)}}
		service, records := newAdapterScopeService(repo)
		recorder := serveAdapterRequest(t, adapterRouter(service),
			platformRequest(http.MethodDelete, "/api/v1/adapters/"+foreignAdapterID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgRequestID, records.created[0].OrganizationID)
	})
}

// TestAdapterByID_RepoReceivesTrustedOrganization pins the exact tenant
// argument seen by the service/repository for reads and deletes.
func TestAdapterByID_RepoReceivesTrustedOrganization(t *testing.T) {
	repo := &tenantByIDRepo{}
	service, _ := newAdapterScopeService(repo)
	router := adapterRouter(service)

	recorder := serveAdapterRequest(t, router,
		forgedOrgRequest(http.MethodGet, "/api/v1/adapters/foreign-adapter?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, 1, repo.getCalls, "the scoped lookup must run exactly once")
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
}
