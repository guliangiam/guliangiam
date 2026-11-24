package ldaps

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appldaps "github.com/guliangiam/guliangiam/internal/app/ldaps"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	domain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	usersdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorSessionOrg = "11111111-1111-1111-1111-111111111111"
	idorForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorLdapID     = "33333333-3333-3333-3333-333333333333"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-id", nil }

// idorRepo is a tenant-aware in-memory repository. Scoped reads and deletes
// only match the owning organization; a foreign ldap behaves like a missing one.
type idorRepo struct {
	ldaps       []*domain.Ldap
	getCalls    int
	scopedOrgs  []string
	updateCalls int
	deleteCalls int
}

func (r *idorRepo) GetByID(ctx context.Context, id string) (*domain.Ldap, error) {
	return r.find(id, "")
}

func (r *idorRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Ldap, error) {
	r.getCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	return r.find(id, organizationID)
}

func (r *idorRepo) find(id, organizationID string) (*domain.Ldap, error) {
	for _, l := range r.ldaps {
		if l.ID != id {
			continue
		}
		if organizationID == "" || l.OrganizationID == organizationID {
			return l, nil
		}
	}
	return nil, apperrors.NewNotFound("ldap not found")
}

func (r *idorRepo) List(context.Context, domain.ListQuery) ([]*domain.Ldap, int64, error) {
	return r.ldaps, int64(len(r.ldaps)), nil
}

func (r *idorRepo) Create(context.Context, *domain.Ldap) error { return nil }

func (r *idorRepo) Update(_ context.Context, l *domain.Ldap) error {
	r.updateCalls++
	return nil
}

func (r *idorRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	if _, err := r.find(id, organizationID); err != nil {
		return err
	}
	return nil
}

type idorRecords struct {
	created []*recordsdomain.Record
}

func (r *idorRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *idorRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *idorRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *idorRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *idorRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *idorRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error { return nil }

func (r *idorRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type idorUsers struct{ usersdomain.Repository }

func (idorUsers) List(context.Context, usersdomain.ListQuery) ([]*usersdomain.User, int64, error) {
	return nil, 0, nil
}

type idorGroups struct{ groupsdomain.Repository }

func (idorGroups) List(context.Context, groupsdomain.ListQuery) ([]*groupsdomain.Group, int64, error) {
	return nil, 0, nil
}

type idorUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorRecords
}

func (u *idorUOW) Ldaps() domain.Repository          { return u.repo }
func (u *idorUOW) Records() recordsdomain.Repository { return u.records }
func (u *idorUOW) Users() usersdomain.Repository     { return idorUsers{} }
func (u *idorUOW) Groups() groupsdomain.Repository   { return idorGroups{} }
func (u *idorUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type idorSyncer struct {
	previewCalls int
	syncCalls    int
}

func (s *idorSyncer) PreviewUsersDomain(context.Context, *domain.SyncerConfig) ([]*domain.LDAPPreviewUser, error) {
	s.previewCalls++
	return nil, nil
}

func (s *idorSyncer) GetMappedUsersDomain(context.Context, *domain.SyncerConfig, []string) ([]*domain.LDAPMappedUser, error) {
	s.syncCalls++
	return nil, nil
}

func (s *idorSyncer) GetMappedGroupsDomain(context.Context, *domain.SyncerConfig) ([]*domain.LDAPMappedGroup, error) {
	return nil, nil
}

type idorClient struct{ calls int }

func (c *idorClient) TestConnection(string, int, string, string, bool) error {
	c.calls++
	return nil
}

type idorHarness struct {
	repo    *idorRepo
	records *idorRecords
	syncer  *idorSyncer
	client  *idorClient
	router  chi.Router
}

func newIDORHarness(ldaps ...*domain.Ldap) *idorHarness {
	h := &idorHarness{
		repo:    &idorRepo{ldaps: ldaps},
		records: &idorRecords{},
		syncer:  &idorSyncer{},
		client:  &idorClient{},
	}
	service := appldaps.Service{
		Clock: idorClock{}, UUID: idorUUID{},
		UOW:        &idorUOW{repo: h.repo, records: h.records},
		LDAPSyncer: h.syncer,
		LDAPClient: h.client,
	}
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: service})
	return h
}

func idorRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// The authenticated tenant is A; the query asks for B.
	ctx := tenantctx.WithTenantScope(req.Context(), idorSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorSessionOrg,
	})
	return req.WithContext(ctx)
}

func platformAdminRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func serve(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func idorBody(t *testing.T, body string) io.Reader {
	t.Helper()
	if body == "" {
		return nil
	}
	return strings.NewReader(body)
}

var idorByIDRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"get", http.MethodGet, "/api/v1/ldaps/" + idorLdapID, ""},
	{"patch", http.MethodPatch, "/api/v1/ldaps/" + idorLdapID, `{"host":"hijacked.example.com","revision":1}`},
	{"delete", http.MethodDelete, "/api/v1/ldaps/" + idorLdapID, ""},
	{"sync", http.MethodPost, "/api/v1/ldaps/" + idorLdapID + "/sync", `{}`},
	{"preview-users", http.MethodGet, "/api/v1/ldaps/" + idorLdapID + "/preview-users", ""},
	{"test-connection", http.MethodPost, "/api/v1/ldaps/" + idorLdapID + "/test-connection", ""},
}

func tenantLdap(organizationID string) *domain.Ldap {
	return &domain.Ldap{
		ID: idorLdapID, OrganizationID: organizationID, Name: "tenant-ldap",
		Host: "ldap.example.com", Port: 389, IsEnabled: true,
		BindDN: "cn=admin,dc=example,dc=com", BindPassword: "bind-secret", Revision: 1,
	}
}

// TestLdapByID_ForeignTenantRejected pins that every by-ID route resolves the
// authenticated tenant, not the caller-supplied organizationId, and that a
// foreign configuration is indistinguishable from a missing one: no 403, no
// existence probe, no write, no audit, no external connection.
func TestLdapByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range idorByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(tenantLdap(idorForeignOrg))

			recorder := serve(h.router, idorRequest(tt.method, tt.path+"?organizationId="+idorForeignOrg, idorBody(t, tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign ldap must look missing")
			assert.NotContains(t, recorder.Body.String(), "bind-secret")
			require.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
			for _, org := range h.repo.scopedOrgs {
				assert.Equal(t, idorSessionOrg, org, "forged query organizationId must be ignored")
			}
			assert.Zero(t, h.repo.updateCalls, "foreign tenant must not update the ldap config")
			assert.Zero(t, h.repo.deleteCalls, "foreign tenant must not delete the ldap config")
			assert.Empty(t, h.records.created, "foreign tenant must not produce an audit event")
			assert.Zero(t, h.syncer.previewCalls, "foreign tenant must not trigger ldap preview")
			assert.Zero(t, h.syncer.syncCalls, "foreign tenant must not trigger ldap sync")
			assert.Zero(t, h.client.calls, "foreign tenant must not open an ldap connection")
		})
	}
}

// TestLdapByID_WithoutTenantScopeRejected pins the fail-fast: when neither the
// session nor a platform-admin target provides a tenant, every by-ID route is
// rejected before touching the repository or any external system.
func TestLdapByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range idorByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(tenantLdap(idorForeignOrg))

			req := httptest.NewRequest(tt.method, tt.path, idorBody(t, tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serve(h.router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, h.repo.getCalls)
			assert.Zero(t, h.repo.updateCalls)
			assert.Zero(t, h.repo.deleteCalls)
			assert.Empty(t, h.records.created)
			assert.Zero(t, h.syncer.previewCalls)
			assert.Zero(t, h.syncer.syncCalls)
			assert.Zero(t, h.client.calls)
		})
	}
}

// TestLdapByID_OwnTenantSucceeds pins that the six routes still work inside the
// authenticated tenant, and that delete/patch audit the real organization.
func TestLdapByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(tenantLdap(idorSessionOrg))
		recorder := serve(h.router, idorRequest(http.MethodGet, "/api/v1/ldaps/"+idorLdapID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "ldap.example.com")
		assert.NotContains(t, recorder.Body.String(), "bind-secret", "read responses must not echo the bind password")
	})

	t.Run("patch", func(t *testing.T) {
		h := newIDORHarness(tenantLdap(idorSessionOrg))
		recorder := serve(h.router, idorRequest(http.MethodPatch, "/api/v1/ldaps/"+idorLdapID,
			strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		h := newIDORHarness(tenantLdap(idorSessionOrg))
		recorder := serve(h.router, idorRequest(http.MethodDelete, "/api/v1/ldaps/"+idorLdapID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.deleteCalls)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID, "delete audit must carry the ldap organization")
	})

	t.Run("sync", func(t *testing.T) {
		h := newIDORHarness(tenantLdap(idorSessionOrg))
		recorder := serve(h.router, idorRequest(http.MethodPost, "/api/v1/ldaps/"+idorLdapID+"/sync",
			strings.NewReader(`{}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.syncer.syncCalls)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID)
	})

	t.Run("preview-users", func(t *testing.T) {
		h := newIDORHarness(tenantLdap(idorSessionOrg))
		recorder := serve(h.router, idorRequest(http.MethodGet, "/api/v1/ldaps/"+idorLdapID+"/preview-users", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.syncer.previewCalls)
	})

	t.Run("test-connection", func(t *testing.T) {
		h := newIDORHarness(tenantLdap(idorSessionOrg))
		recorder := serve(h.router, idorRequest(http.MethodPost, "/api/v1/ldaps/"+idorLdapID+"/test-connection", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.client.calls)
	})
}

// TestLdapByID_PlatformAdminCanTargetTenant pins that a platform scope keeps
// the explicit cross-tenant target; the requested organization is then the
// repository predicate.
func TestLdapByID_PlatformAdminCanTargetTenant(t *testing.T) {
	h := newIDORHarness(tenantLdap(idorForeignOrg))
	recorder := serve(h.router, platformAdminRequest(http.MethodGet,
		"/api/v1/ldaps/"+idorLdapID+"?organizationId="+idorForeignOrg, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotEmpty(t, h.repo.scopedOrgs)
	assert.Equal(t, idorForeignOrg, h.repo.scopedOrgs[0])
}
