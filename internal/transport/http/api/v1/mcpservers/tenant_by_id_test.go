package mcpservers

import (
	"context"
	"encoding/json"
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
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	tenantSessionOrgID = "11111111-1111-1111-1111-111111111111"
	tenantForeignOrgID = "22222222-2222-2222-2222-222222222222"
	tenantServerID     = "33333333-3333-3333-3333-333333333333"
	tenantUserID       = "44444444-4444-4444-4444-444444444444"
	tenantOtherUserID  = "55555555-5555-5555-5555-555555555555"
)

type tenantClock struct{}

func (tenantClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type tenantUUID struct{}

func (tenantUUID) NewString() (string, error) { return "generated-id", nil }

// tenantJWKS records signing calls so a rejected mint can be distinguished from
// one that actually produced a token.
type tenantJWKS struct {
	domainauth.JWKSProvider
	signed int
	claims []map[string]any
}

func (j *tenantJWKS) SignToken(claims map[string]any, _ string) (string, error) {
	j.signed++
	j.claims = append(j.claims, claims)
	return "mcp-access-token", nil
}

func (j *tenantJWKS) SignTokenWithAlgorithm(claims map[string]any, _, _ string) (string, error) {
	return j.SignToken(claims, "JWT")
}

// tenantServerRepo is a tenant-aware in-memory repository. Scoped reads only
// return rows owned by the requested organization, so a foreign id looks
// exactly like a missing one. Write counters expose cross-tenant mutations.
type tenantServerRepo struct {
	domain.Repository
	servers     []*domain.McpServer
	getCalls    int
	updateCalls int
	deleteCalls int
}

func (r *tenantServerRepo) GetByID(_ context.Context, id string) (*domain.McpServer, error) {
	r.getCalls++
	for _, server := range r.servers {
		if server.ID == id {
			return server, nil
		}
	}
	return nil, apperrors.NewNotFound("mcp server not found")
}

func (r *tenantServerRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.McpServer, error) {
	r.getCalls++
	for _, server := range r.servers {
		if server.ID == id && server.OrganizationID == organizationID {
			return server, nil
		}
	}
	return nil, apperrors.NewNotFound("mcp server not found")
}

func (r *tenantServerRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.McpServer, int64, error) {
	return r.servers, int64(len(r.servers)), nil
}

func (r *tenantServerRepo) Create(_ context.Context, server *domain.McpServer) error {
	r.servers = append(r.servers, server)
	return nil
}

func (r *tenantServerRepo) Update(_ context.Context, server *domain.McpServer) error {
	for i, existing := range r.servers {
		if existing.ID == server.ID && existing.OrganizationID == server.OrganizationID {
			r.updateCalls++
			r.servers[i] = server
			return nil
		}
	}
	return apperrors.NewNotFound("mcp server not found")
}

func (r *tenantServerRepo) Delete(_ context.Context, id, organizationID string) error {
	for i, existing := range r.servers {
		if existing.ID == id && existing.OrganizationID == organizationID {
			r.deleteCalls++
			r.servers = append(r.servers[:i], r.servers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("mcp server not found")
}

type tenantUserRepo struct {
	userdomain.Repository
	users    []*userdomain.User
	getCalls int
}

func (r *tenantUserRepo) GetByID(_ context.Context, id string) (*userdomain.User, error) {
	r.getCalls++
	for _, user := range r.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, apperrors.NewNotFound("user not found")
}

type tenantRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *tenantRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type tenantOutbox struct {
	domainoutbox.Repository
	events []*domainoutbox.OutboxEvent
}

func (o *tenantOutbox) Enqueue(_ context.Context, item *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, item)
	return nil
}

type tenantUOW struct {
	appuow.UnitOfWork
	servers *tenantServerRepo
	users   *tenantUserRepo
	records *tenantRecords
	outbox  *tenantOutbox
}

func (u *tenantUOW) McpServers() domain.Repository     { return u.servers }
func (u *tenantUOW) Users() userdomain.Repository      { return u.users }
func (u *tenantUOW) Records() recordsdomain.Repository { return u.records }
func (u *tenantUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *tenantUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type tenantFixture struct {
	router  chi.Router
	servers *tenantServerRepo
	users   *tenantUserRepo
	records *tenantRecords
	outbox  *tenantOutbox
	jwks    *tenantJWKS
}

func newTenantFixture(servers []*domain.McpServer, users []*userdomain.User) *tenantFixture {
	f := &tenantFixture{
		servers: &tenantServerRepo{servers: servers},
		users:   &tenantUserRepo{users: users},
		records: &tenantRecords{},
		outbox:  &tenantOutbox{},
		jwks:    &tenantJWKS{},
	}
	service := appmcpservers.NewServiceWithDeps(
		tenantClock{}, tenantUUID{},
		&tenantUOW{servers: f.servers, users: f.users, records: f.records, outbox: f.outbox},
		nil, f.jwks,
	)
	f.router = chi.NewRouter()
	RegisterChiRoutes(f.router, NewHandler(service))
	return f
}

// tenantRequest authenticates a request as a member of organizationID; the
// query string may still carry a forged organizationId.
func tenantRequest(method, target, body, organizationID string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := req.Context()
	if organizationID != "" {
		ctx = tenantctx.WithTenantScope(ctx, organizationID, "session-org", tenantctx.TenantFilterStrict)
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
			UserID: tenantUserID, OrganizationID: organizationID,
		})
	}
	return req.WithContext(ctx)
}

// tenantScopedRequest authenticates only the tenant scope, without a session
// user. It models a system/internal context that must not mint user tokens.
func tenantScopedRequest(method, target, body, organizationID string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), organizationID, "session-org", tenantctx.TenantFilterStrict)
	return req.WithContext(ctx)
}

func serveTenant(f *tenantFixture, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	f.router.ServeHTTP(recorder, req)
	return recorder
}

func tenantServer(organizationID string) *domain.McpServer {
	return &domain.McpServer{
		ID:             tenantServerID,
		OrganizationID: organizationID,
		Name:           "tenant-server",
		DisplayName:    "Tenant Server",
		IsEnabled:      true,
		Revision:       1,
	}
}

func tenantUser(organizationID string) *userdomain.User {
	return &userdomain.User{ID: tenantUserID, OrganizationID: organizationID}
}

var tenantWriteRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"get", http.MethodGet, "/api/v1/mcp-servers/" + tenantServerID, ""},
	{"patch", http.MethodPatch, "/api/v1/mcp-servers/" + tenantServerID, `{"displayName":"hijacked"}`},
	{"delete", http.MethodDelete, "/api/v1/mcp-servers/" + tenantServerID, ""},
}

// TestMcpServerByID_ForeignTenantRejected pins that a forged organizationId in
// the query cannot reach another tenant's server: reads look missing, writes
// are never issued, and no audit row is produced.
func TestMcpServerByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range tenantWriteRoutes {
		t.Run(tt.name, func(t *testing.T) {
			f := newTenantFixture([]*domain.McpServer{tenantServer(tenantForeignOrgID)}, nil)

			recorder := serveTenant(f, tenantRequest(tt.method,
				tt.path+"?organizationId="+tenantForeignOrgID, tt.body, tenantSessionOrgID))

			assert.Equal(t, http.StatusNotFound, recorder.Code,
				"foreign server must look missing")
			assert.Zero(t, f.servers.updateCalls, "foreign tenant must not update the server")
			assert.Zero(t, f.servers.deleteCalls, "foreign tenant must not delete the server")
			assert.Empty(t, f.records.created, "foreign tenant must not produce an audit event")
			assert.Empty(t, f.outbox.events, "foreign tenant must not produce an outbox event")
		})
	}
}

// TestMcpServerByID_WithoutTenantScopeRejected pins the fail-fast: without a
// trusted tenant every by-ID route is a 400 before the repository is touched.
func TestMcpServerByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range tenantWriteRoutes {
		t.Run(tt.name, func(t *testing.T) {
			f := newTenantFixture([]*domain.McpServer{tenantServer(tenantSessionOrgID)}, nil)

			recorder := serveTenant(f, tenantRequest(tt.method, tt.path, tt.body, ""))

			assert.Equal(t, http.StatusBadRequest, recorder.Code,
				"missing trusted tenant must be a client error")
			assert.Zero(t, f.servers.getCalls, "missing tenant must not reach the repository")
			assert.Zero(t, f.servers.updateCalls)
			assert.Zero(t, f.servers.deleteCalls)
			assert.Empty(t, f.records.created)
		})
	}
}

// TestMcpServerByID_OwnTenantSucceeds pins that the same routes keep working
// inside the authenticated tenant and that write audits carry the real tenant.
func TestMcpServerByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		f := newTenantFixture([]*domain.McpServer{tenantServer(tenantSessionOrgID)}, nil)

		recorder := serveTenant(f, tenantRequest(http.MethodGet,
			"/api/v1/mcp-servers/"+tenantServerID+"?organizationId="+tenantForeignOrgID, "", tenantSessionOrgID))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "tenant-server")
		assert.Zero(t, f.servers.updateCalls)
		assert.Zero(t, f.servers.deleteCalls)
	})

	t.Run("patch", func(t *testing.T) {
		f := newTenantFixture([]*domain.McpServer{tenantServer(tenantSessionOrgID)}, nil)

		recorder := serveTenant(f, tenantRequest(http.MethodPatch,
			"/api/v1/mcp-servers/"+tenantServerID, `{"displayName":"renamed"}`, tenantSessionOrgID))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, f.servers.updateCalls)
		require.Len(t, f.records.created, 1)
		assert.Equal(t, tenantSessionOrgID, f.records.created[0].OrganizationID)
		assert.Equal(t, tenantServerID, f.records.created[0].ResourceID)
	})

	t.Run("delete", func(t *testing.T) {
		f := newTenantFixture([]*domain.McpServer{tenantServer(tenantSessionOrgID)}, nil)

		recorder := serveTenant(f, tenantRequest(http.MethodDelete,
			"/api/v1/mcp-servers/"+tenantServerID, "", tenantSessionOrgID))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, f.servers.deleteCalls)
		require.Len(t, f.records.created, 1)
		assert.Equal(t, tenantSessionOrgID, f.records.created[0].OrganizationID,
			"delete audit must carry the server organization")
	})
}

// TestMcpServerAccessToken_ForeignTenantRejected pins that the token mint is
// bound to the trusted tenant: another tenant's server is a 404 and nothing is
// signed. The denial is audited against the caller's own tenant.
func TestMcpServerAccessToken_ForeignTenantRejected(t *testing.T) {
	f := newTenantFixture(
		[]*domain.McpServer{tenantServer(tenantForeignOrgID)},
		[]*userdomain.User{tenantUser(tenantSessionOrgID)},
	)

	recorder := serveTenant(f, tenantRequest(http.MethodGet,
		"/api/v1/mcp-servers/"+tenantServerID+"/access-token?userId="+tenantUserID+
			"&organizationId="+tenantForeignOrgID, "", tenantSessionOrgID))

	assert.Equal(t, http.StatusNotFound, recorder.Code,
		"foreign server must look missing for token minting")
	assert.Zero(t, f.jwks.signed, "no token may be signed for a foreign server")
	require.Len(t, f.records.created, 1, "denial is audited in the caller tenant")
	assert.Equal(t, tenantSessionOrgID, f.records.created[0].OrganizationID)
	assert.Equal(t, tenantServerID, f.records.created[0].ResourceID)
	assertAccessTokenAudit(t, f, "denied", tenantUserID)
}

// TestMcpServerAccessToken_TargetUserForeignTenantRejected pins that the
// query userId is validated against the trusted tenant: a user that belongs to
// another tenant is a 404 and nothing is signed.
func TestMcpServerAccessToken_TargetUserForeignTenantRejected(t *testing.T) {
	f := newTenantFixture(
		[]*domain.McpServer{tenantServer(tenantSessionOrgID)},
		[]*userdomain.User{tenantUser(tenantForeignOrgID)},
	)

	recorder := serveTenant(f, tenantRequest(http.MethodGet,
		"/api/v1/mcp-servers/"+tenantServerID+"/access-token?userId="+tenantUserID, "", tenantSessionOrgID))

	assert.Equal(t, http.StatusNotFound, recorder.Code,
		"foreign user must look missing for token minting")
	assert.Zero(t, f.jwks.signed, "no token may be signed for a foreign user")
	assert.Equal(t, 1, f.users.getCalls, "the target user must be resolved before signing")
	require.Len(t, f.records.created, 1)
	assert.Equal(t, tenantSessionOrgID, f.records.created[0].OrganizationID)
	assertAccessTokenAudit(t, f, "denied", tenantUserID)
}

// TestMcpServerAccessToken_QueryUserMismatchRejected pins the A-04 boundary:
// mcpservers:read alone must not mint a token for another user. The session user
// is the only accepted subject, and a mismatching query userId is a 403 before
// the repository or signer is touched.
func TestMcpServerAccessToken_QueryUserMismatchRejected(t *testing.T) {
	f := newTenantFixture(
		[]*domain.McpServer{tenantServer(tenantSessionOrgID)},
		[]*userdomain.User{
			tenantUser(tenantSessionOrgID),
			{ID: tenantOtherUserID, OrganizationID: tenantSessionOrgID},
		},
	)

	recorder := serveTenant(f, tenantRequest(http.MethodGet,
		"/api/v1/mcp-servers/"+tenantServerID+"/access-token?userId="+tenantOtherUserID, "", tenantSessionOrgID))

	assert.Equal(t, http.StatusForbidden, recorder.Code,
		"another user must not be impersonated for token minting")
	assert.Contains(t, recorder.Body.String(), `"code":"forbidden"`)
	assert.Zero(t, f.jwks.signed, "no token may be signed for another user")
	assert.Empty(t, f.records.created, "a transport-level rejection is not a token decision")
}

// TestMcpServerAccessToken_QueryUserMatchesSessionIssued pins the compatible
// path: a legacy client that sends its own id still receives its own token.
func TestMcpServerAccessToken_QueryUserMatchesSessionIssued(t *testing.T) {
	f := newTenantFixture(
		[]*domain.McpServer{tenantServer(tenantSessionOrgID)},
		[]*userdomain.User{tenantUser(tenantSessionOrgID)},
	)

	recorder := serveTenant(f, tenantRequest(http.MethodGet,
		"/api/v1/mcp-servers/"+tenantServerID+"/access-token?userId="+tenantUserID, "", tenantSessionOrgID))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 1, f.jwks.signed)
	assert.Equal(t, tenantUserID, f.jwks.claims[0]["sub"])
}

// TestMcpServerAccessToken_WithoutSessionUserRejected pins fail-closed: a
// trusted tenant without a session user cannot mint a user-scoped token.
func TestMcpServerAccessToken_WithoutSessionUserRejected(t *testing.T) {
	f := newTenantFixture(
		[]*domain.McpServer{tenantServer(tenantSessionOrgID)},
		[]*userdomain.User{tenantUser(tenantSessionOrgID)},
	)

	recorder := serveTenant(f, tenantScopedRequest(http.MethodGet,
		"/api/v1/mcp-servers/"+tenantServerID+"/access-token?userId="+tenantUserID, "", tenantSessionOrgID))

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Zero(t, f.jwks.signed)
	assert.Empty(t, f.records.created)
}

// TestMcpServerAccessToken_OwnTenantIssued pins the happy path, including the
// audit payload (server id + target user id + result) and the caller's tenant.
func TestMcpServerAccessToken_OwnTenantIssued(t *testing.T) {
	f := newTenantFixture(
		[]*domain.McpServer{tenantServer(tenantSessionOrgID)},
		[]*userdomain.User{tenantUser(tenantSessionOrgID)},
	)

	recorder := serveTenant(f, tenantRequest(http.MethodGet,
		"/api/v1/mcp-servers/"+tenantServerID+"/access-token?userId="+tenantUserID+"&organizationId="+tenantForeignOrgID,
		"", tenantSessionOrgID))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "mcp-access-token")
	require.Equal(t, 1, f.jwks.signed, "exactly one token is minted")
	require.Len(t, f.records.created, 1)
	assert.Equal(t, tenantSessionOrgID, f.records.created[0].OrganizationID)
	assert.Equal(t, tenantServerID, f.records.created[0].ResourceID)
	assertAccessTokenAudit(t, f, "issued", tenantUserID)
}

func assertAccessTokenAudit(t *testing.T, f *tenantFixture, result, userID string) {
	t.Helper()
	require.Len(t, f.outbox.events, 1, "one audit outbox event per token decision")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(f.outbox.events[0].Payload, &payload))
	assert.Equal(t, tenantServerID, payload["serverId"])
	assert.Equal(t, userID, payload["userId"])
	assert.Equal(t, result, payload["result"])
}
