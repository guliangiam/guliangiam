package mcpservers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appmcpservers "github.com/guliangiam/guliangiam/internal/app/mcpservers"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	postMcpTenantOrg   = "11111111-1111-1111-1111-111111111111"
	postMcpForeignOrg  = "22222222-2222-2222-2222-222222222222"
	postMcpForeignSlug = "foreign-org-slug"
	postMcpOwnID       = "33333333-3333-3333-3333-333333333333"
	postMcpForeignID   = "44444444-4444-4444-4444-444444444444"
	postMcpOwnSlug     = "own-org-slug"
	postMcpOwnName     = "own-server"
	postMcpForeignName = "foreign-server"
	postMcpSessionUser = "55555555-5555-5555-5555-555555555555"
)

type postMcpClock struct{}

func (postMcpClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type postMcpUUID struct{}

func (postMcpUUID) NewString() (string, error) { return "post-mcp-id", nil }

// postMcpRepo models resolve_org_id: management reads match by organization id
// or slug, while the pre-fix proxy passed the raw path slug through List.
type postMcpRepo struct {
	domain.Repository
	servers     []*domain.McpServer
	gets        int
	listOrg     string
	updateCalls int
}

func (r *postMcpRepo) resolveOrg(organizationID string) string {
	if organizationID == postMcpForeignSlug {
		return postMcpForeignOrg
	}
	if organizationID == postMcpOwnSlug {
		return postMcpTenantOrg
	}
	return organizationID
}

func (r *postMcpRepo) GetByID(_ context.Context, id string) (*domain.McpServer, error) {
	r.gets++
	for _, server := range r.servers {
		if server.ID == id {
			return server, nil
		}
	}
	return nil, apperrors.NewNotFound("mcp server not found")
}

func (r *postMcpRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.McpServer, error) {
	r.gets++
	resolved := r.resolveOrg(organizationID)
	for _, server := range r.servers {
		if server.ID == id && server.OrganizationID == resolved {
			return server, nil
		}
	}
	return nil, apperrors.NewNotFound("mcp server not found")
}

func (r *postMcpRepo) List(_ context.Context, query domain.ListQuery) ([]*domain.McpServer, int64, error) {
	r.listOrg = query.OrganizationID
	resolved := r.resolveOrg(query.OrganizationID)
	items := make([]*domain.McpServer, 0)
	for _, server := range r.servers {
		if server.OrganizationID == resolved && server.Name == query.Keyword {
			items = append(items, server)
		}
	}
	return items, int64(len(items)), nil
}

func (r *postMcpRepo) Update(_ context.Context, server *domain.McpServer) error {
	for i, existing := range r.servers {
		if existing.ID == server.ID && existing.OrganizationID == server.OrganizationID {
			r.updateCalls++
			r.servers[i] = server
			return nil
		}
	}
	return apperrors.NewNotFound("mcp server not found")
}

type postMcpProxy struct {
	syncCalls int
	callCalls int
	tools     []domain.Tool
}

func (p *postMcpProxy) SyncToolsFromServer(context.Context, string) error {
	p.syncCalls++
	return nil
}

func (p *postMcpProxy) ListTools(context.Context) []domain.Tool { return p.tools }

func (p *postMcpProxy) CallTool(context.Context, string, string, map[string]any) (any, error) {
	p.callCalls++
	return map[string]any{"ok": true}, nil
}

type postMcpUOW struct {
	appuow.UnitOfWork
	repo domain.Repository
}

func (u *postMcpUOW) McpServers() domain.Repository { return u.repo }
func (u *postMcpUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type postMcpHarness struct {
	repo   *postMcpRepo
	proxy  *postMcpProxy
	router chi.Router
}

func newPostMcpHarness(servers ...*domain.McpServer) *postMcpHarness {
	repo := &postMcpRepo{servers: servers}
	proxy := &postMcpProxy{tools: []domain.Tool{{Name: "echo"}}}
	service := appmcpservers.NewServiceWithDeps(
		postMcpClock{}, postMcpUUID{},
		&postMcpUOW{repo: repo}, proxy, nil,
	)
	router := chi.NewRouter()
	RegisterChiRoutes(router, NewHandler(service))
	return &postMcpHarness{repo: repo, proxy: proxy, router: router}
}

func postMcpRequest(method, target, body, organizationID string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if organizationID != "" {
		ctx := tenantctx.WithTenantScope(req.Context(), organizationID, "session-org", tenantctx.TenantFilterStrict)
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
			UserID: "55555555-5555-5555-5555-555555555555", OrganizationID: organizationID,
		})
		req = req.WithContext(ctx)
	}
	return req
}

func postMcpPlatformRequest(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: postMcpSessionUser,
	})
	return req.WithContext(ctx)
}

func servePostMcp(t *testing.T, harness *postMcpHarness, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	harness.router.ServeHTTP(recorder, req)
	return recorder
}

func postMcpServer(id, organizationID, name string) *domain.McpServer {
	return &domain.McpServer{
		ID: id, OrganizationID: organizationID, Name: name, DisplayName: name,
		IsEnabled: true, Revision: 1,
		Tools: []domain.Tool{{Name: "echo", IsAllowed: true}},
	}
}

// TestMcpPostAction_ForeignTenantRejected pins that sync-tools and the proxy
// are tenant-bound: a foreign server is a 404 and never reaches the upstream
// MCP call or the database write.
func TestMcpPostAction_ForeignTenantRejected(t *testing.T) {
	t.Run("sync-tools", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpForeignID, postMcpForeignOrg, postMcpForeignName))

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpForeignID+"/sync-tools?organizationId="+postMcpForeignOrg,
			"", postMcpTenantOrg))

		assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign server must look missing")
		assert.Zero(t, harness.proxy.syncCalls, "foreign tenant must not trigger a tool sync")
		assert.Zero(t, harness.repo.updateCalls)
	})

	t.Run("proxy", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpForeignID, postMcpForeignOrg, postMcpForeignName))

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpForeignSlug+"/"+postMcpForeignName+"/proxy",
			`{"toolName":"echo"}`, postMcpTenantOrg))

		assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign server must look missing")
		assert.Zero(t, harness.proxy.callCalls, "foreign tenant must not call the tool")
	})
}

// TestMcpPostAction_WithoutTenantScopeRejected pins the fail-fast: no trusted
// tenant yields 400 before decoding the body or touching the repository.
func TestMcpPostAction_WithoutTenantScopeRejected(t *testing.T) {
	t.Run("sync-tools", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName))

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpOwnID+"/sync-tools", "", ""))

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Zero(t, harness.repo.gets, "an empty tenant must not reach the repository")
		assert.Zero(t, harness.proxy.syncCalls)
	})

	t.Run("proxy", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName))

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpOwnSlug+"/"+postMcpOwnName+"/proxy",
			`{"toolName":"echo"}`, ""))

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Zero(t, harness.repo.listOrg, "an empty tenant must not reach the repository")
		assert.Zero(t, harness.proxy.callCalls)
	})
}

// TestMcpPostAction_OwnTenantSucceeds pins the same-tenant path.
func TestMcpPostAction_OwnTenantSucceeds(t *testing.T) {
	t.Run("sync-tools", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName))

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpOwnID+"/sync-tools", "", postMcpTenantOrg))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, harness.proxy.syncCalls)
		assert.Equal(t, 1, harness.repo.updateCalls)
		assert.Contains(t, recorder.Body.String(), "echo")
	})

	t.Run("proxy", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName))

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpOwnSlug+"/"+postMcpOwnName+"/proxy",
			`{"toolName":"echo"}`, postMcpTenantOrg))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, postMcpTenantOrg, harness.repo.listOrg,
			"the proxy must resolve the trusted tenant, not the path slug")
		assert.Equal(t, 1, harness.proxy.callCalls)
		assert.Contains(t, recorder.Body.String(), "true")
	})
}

// TestMcpPostAction_ProxyToolAuthorization pins A-05: the endpoint-level
// mcpservers.proxy permission is not a tool permission. A tool that is absent
// from the server catalogue or marked IsAllowed=false is a 403 and never reaches
// the upstream call.
func TestMcpPostAction_ProxyToolAuthorization(t *testing.T) {
	t.Run("undeclared tool", func(t *testing.T) {
		server := postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName)
		server.Tools = nil
		harness := newPostMcpHarness(server)

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpOwnSlug+"/"+postMcpOwnName+"/proxy",
			`{"toolName":"echo"}`, postMcpTenantOrg))

		assert.Equal(t, http.StatusForbidden, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"code":"forbidden"`)
		assert.Zero(t, harness.proxy.callCalls, "undeclared tool must not reach the upstream")
	})

	t.Run("disallowed tool", func(t *testing.T) {
		server := postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName)
		server.Tools = []domain.Tool{{Name: "echo", IsAllowed: false}}
		harness := newPostMcpHarness(server)

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpOwnSlug+"/"+postMcpOwnName+"/proxy",
			`{"toolName":"echo"}`, postMcpTenantOrg))

		assert.Equal(t, http.StatusForbidden, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"code":"forbidden"`)
		assert.Zero(t, harness.proxy.callCalls, "denied tool must not reach the upstream")
	})

	t.Run("allowed tool", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName))

		recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpOwnSlug+"/"+postMcpOwnName+"/proxy",
			`{"toolName":"echo"}`, postMcpTenantOrg))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, harness.proxy.callCalls)
	})
}

// TestMcpPostAction_ProxyQueryUserIgnored pins that a query userId cannot
// replace the validated session identity.
func TestMcpPostAction_ProxyQueryUserIgnored(t *testing.T) {
	harness := newPostMcpHarness(postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName))

	recorder := servePostMcp(t, harness, postMcpRequest(http.MethodPost,
		"/api/v1/mcp-servers/"+postMcpOwnSlug+"/"+postMcpOwnName+"/proxy?userId=attacker",
		`{"toolName":"echo"}`, postMcpTenantOrg))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, harness.proxy.callCalls)
}

// TestMcpPostAction_ProxyWithoutSessionRejected pins fail-closed for a
// tenant-scoped request that carries no authenticated user.
func TestMcpPostAction_ProxyWithoutSessionRejected(t *testing.T) {
	harness := newPostMcpHarness(postMcpServer(postMcpOwnID, postMcpTenantOrg, postMcpOwnName))

	recorder := servePostMcp(t, harness, tenantScopedRequest(http.MethodPost,
		"/api/v1/mcp-servers/"+postMcpOwnSlug+"/"+postMcpOwnName+"/proxy",
		`{"toolName":"echo"}`, postMcpTenantOrg))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Zero(t, harness.proxy.callCalls)
}

// TestMcpPostAction_PlatformAdminCanTargetTenant pins the single legit override:
// super_admin/system scopes may name another tenant explicitly.
func TestMcpPostAction_PlatformAdminCanTargetTenant(t *testing.T) {
	t.Run("sync-tools", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpForeignID, postMcpForeignOrg, postMcpForeignName))

		recorder := servePostMcp(t, harness, postMcpPlatformRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpForeignID+"/sync-tools?organizationId="+postMcpForeignOrg, ""))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, harness.proxy.syncCalls)
		assert.Equal(t, 1, harness.repo.updateCalls)
	})

	t.Run("proxy", func(t *testing.T) {
		harness := newPostMcpHarness(postMcpServer(postMcpForeignID, postMcpForeignOrg, postMcpForeignName))

		recorder := servePostMcp(t, harness, postMcpPlatformRequest(http.MethodPost,
			"/api/v1/mcp-servers/"+postMcpForeignSlug+"/"+postMcpForeignName+"/proxy", `{"toolName":"echo"}`))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, postMcpForeignSlug, harness.repo.listOrg,
			"a platform scope may name another tenant through the path")
		assert.Equal(t, 1, harness.proxy.callCalls)
	})
}
