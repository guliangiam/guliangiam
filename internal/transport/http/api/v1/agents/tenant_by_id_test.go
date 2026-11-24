package agents

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appagents "github.com/guliangiam/guliangiam/internal/app/agents"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/agents"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tenantAgentRepo is a tenant-scoped in-memory agent repository used by the
// HTTP-level regression tests. Counting writes keeps cross-tenant attempts
// observable, and the transaction recorder below avoids gomock so audit rows
// are visible (gomock UoW short-circuits audit writes).
type tenantAgentRepo struct {
	agents      []*domain.Agent
	getOrgs     []string
	updateCalls int
	deleteCalls int
}

func (r *tenantAgentRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Agent, error) {
	r.getOrgs = append(r.getOrgs, organizationID)
	for _, a := range r.agents {
		if a.ID == id && a.OrganizationID == organizationID {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("agent not found")
}

func (r *tenantAgentRepo) List(context.Context, domain.ListQuery) ([]*domain.Agent, int64, error) {
	return r.agents, int64(len(r.agents)), nil
}

func (r *tenantAgentRepo) Create(_ context.Context, agent *domain.Agent) error {
	r.agents = append(r.agents, agent)
	return nil
}

func (r *tenantAgentRepo) Update(_ context.Context, agent *domain.Agent) error {
	r.updateCalls++
	for i, a := range r.agents {
		if a.ID == agent.ID && a.OrganizationID == agent.OrganizationID {
			r.agents[i] = agent
			return nil
		}
	}
	return apperrors.NewNotFound("agent not found")
}

func (r *tenantAgentRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, a := range r.agents {
		if a.ID == id && a.OrganizationID == organizationID {
			r.agents = append(r.agents[:i], r.agents[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("agent not found")
}

type agentRecordingRecords struct {
	created []*recordsdomain.Record
}

func (r *agentRecordingRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *agentRecordingRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *agentRecordingRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *agentRecordingRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *agentRecordingRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *agentRecordingRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *agentRecordingRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

// agentScopeUOW exposes only the repos used by the agent service; the embedded
// UnitOfWork carries the remaining interface surface without implementation.
type agentScopeUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *agentRecordingRecords
}

func (u *agentScopeUOW) Agents() domain.Repository         { return u.repo }
func (u *agentScopeUOW) Records() recordsdomain.Repository { return u.records }
func (u *agentScopeUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *agentScopeUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newAgentScopeService(repo domain.Repository) (appagents.Service, *agentRecordingRecords) {
	records := &agentRecordingRecords{}
	return appagents.NewService(forgedOrgClock{}, forgedOrgUUID{}, &agentScopeUOW{repo: repo, records: records}), records
}

func tenantAgent(agentID, orgID string) *domain.Agent {
	return &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "tenant-agent",
		DisplayName:    "Tenant Agent",
		Type:           "gateway-proxy",
		Host:           "agent.example.com",
		Port:           8080,
		Url:            "https://agent.example.com",
		Token:          "super-secret-agent-token",
		IsEnabled:      true,
		Revision:       1,
	}
}

func agentRouter(service appagents.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveAgentRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// platformAdminAgentRequest builds a super_admin request whose target tenant
// comes from the explicit organizationId query value.
func platformAdminAgentRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

// TestAgentByID_ForeignTenantRejected pins that a caller can only reach agents
// of its authenticated tenant: a foreign id must look exactly like a missing
// one, leave the row untouched, and write no audit row.
func TestAgentByID_ForeignTenantRejected(t *testing.T) {
	const foreignAgentID = "foreign-agent"

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/agents/" + foreignAgentID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/agents/" + foreignAgentID + "?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/agents/" + foreignAgentID + "?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &tenantAgentRepo{agents: []*domain.Agent{tenantAgent(foreignAgentID, forgedOrgRequestID)}}
			service, records := newAgentScopeService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveAgentRequest(t, agentRouter(service), forgedOrgRequest(tt.method, tt.target, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign agent must be indistinguishable from missing")
			assert.Zero(t, repo.updateCalls, "foreign tenant must not update the agent")
			assert.Zero(t, repo.deleteCalls, "foreign tenant must not delete the agent")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.NotContains(t, recorder.Body.String(), "super-secret-agent-token")
		})
	}
}

// TestAgentByID_OwnTenantSucceeds pins that the same three operations still
// work for the authenticated tenant, that the forged query value is ignored,
// and that writes audit the real tenant.
func TestAgentByID_OwnTenantSucceeds(t *testing.T) {
	const ownAgentID = "own-agent"

	t.Run("get", func(t *testing.T) {
		repo := &tenantAgentRepo{agents: []*domain.Agent{tenantAgent(ownAgentID, forgedOrgSessionID)}}
		service, _ := newAgentScopeService(repo)
		recorder := serveAgentRequest(t, agentRouter(service), forgedOrgRequest(http.MethodGet,
			"/api/v1/agents/"+ownAgentID+"?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), ownAgentID)
		assert.NotContains(t, recorder.Body.String(), "super-secret-agent-token")
		require.Len(t, repo.getOrgs, 1)
		assert.Equal(t, forgedOrgSessionID, repo.getOrgs[0], "the authenticated tenant must win over ?organizationId=")
	})

	t.Run("patch", func(t *testing.T) {
		repo := &tenantAgentRepo{agents: []*domain.Agent{tenantAgent(ownAgentID, forgedOrgSessionID)}}
		service, records := newAgentScopeService(repo)
		recorder := serveAgentRequest(t, agentRouter(service), forgedOrgRequest(http.MethodPatch,
			"/api/v1/agents/"+ownAgentID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &tenantAgentRepo{agents: []*domain.Agent{tenantAgent(ownAgentID, forgedOrgSessionID)}}
		service, records := newAgentScopeService(repo)
		recorder := serveAgentRequest(t, agentRouter(service), forgedOrgRequest(http.MethodDelete, "/api/v1/agents/"+ownAgentID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID, "delete audit must carry the agent organization")
	})
}

// TestAgentByID_WithoutTenantScopeRejected pins the fail-fast: when neither the
// session nor a platform-admin target provides a tenant, every by-ID route is
// rejected before touching the repository.
func TestAgentByID_WithoutTenantScopeRejected(t *testing.T) {
	requests := map[string]*http.Request{
		"get":    httptest.NewRequest(http.MethodGet, "/api/v1/agents/any-agent", nil),
		"patch":  httptest.NewRequest(http.MethodPatch, "/api/v1/agents/any-agent", strings.NewReader(`{}`)),
		"delete": httptest.NewRequest(http.MethodDelete, "/api/v1/agents/any-agent", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			repo := &tenantAgentRepo{agents: []*domain.Agent{tenantAgent("any-agent", forgedOrgSessionID)}}
			service, _ := newAgentScopeService(repo)
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveAgentRequest(t, agentRouter(service), req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Empty(t, repo.getOrgs, "missing tenant must not reach the repository")
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
		})
	}
}

// TestAgentByID_PlatformAdminCrossTenant pins that platform admins keep the
// explicit cross-tenant capability: TrustedOrganizationID honours the
// requested organizationId for super_admin/system scopes.
func TestAgentByID_PlatformAdminCrossTenant(t *testing.T) {
	const targetAgentID = "target-agent"
	const targetOrgID = "33333333-3333-3333-3333-333333333333"

	tests := []struct {
		name   string
		method string
		body   string
	}{
		{"get", http.MethodGet, ""},
		{"patch", http.MethodPatch, `{"displayName":"renamed-by-platform-admin","revision":1}`},
		{"delete", http.MethodDelete, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &tenantAgentRepo{agents: []*domain.Agent{tenantAgent(targetAgentID, targetOrgID)}}
			service, _ := newAgentScopeService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			req := platformAdminAgentRequest(tt.method,
				"/api/v1/agents/"+targetAgentID+"?organizationId="+targetOrgID, body)
			recorder := serveAgentRequest(t, agentRouter(service), req)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Len(t, repo.getOrgs, 1)
			assert.Equal(t, targetOrgID, repo.getOrgs[0], "platform scope must honour the explicit target tenant")
		})
	}
}
