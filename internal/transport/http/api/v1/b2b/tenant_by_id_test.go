package b2b

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainb2b "github.com/guliangiam/guliangiam/internal/domain/b2b"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	b2bSessionOrgID = "11111111-1111-1111-1111-111111111111"
	b2bForeignOrgID = "22222222-2222-2222-2222-222222222222"
)

type tenantB2BRepo struct {
	items     map[string]*domainb2b.OrganizationDomain
	lastList  domainb2b.ListQuery
	getCalls  int
	createCnt int
	updateCnt int
	deleteCnt int
}

func (r *tenantB2BRepo) Create(_ context.Context, d *domainb2b.OrganizationDomain) error {
	r.createCnt++
	r.items[d.ID] = d
	return nil
}

func (r *tenantB2BRepo) GetByID(_ context.Context, id string) (*domainb2b.OrganizationDomain, error) {
	r.getCalls++
	d, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("organization_domain not found")
	}
	return d, nil
}

func (r *tenantB2BRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domainb2b.OrganizationDomain, error) {
	r.getCalls++
	d, ok := r.items[id]
	if !ok || d.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("organization_domain not found")
	}
	return d, nil
}

func (r *tenantB2BRepo) GetByDomain(_ context.Context, organizationID, domain string) (*domainb2b.OrganizationDomain, error) {
	for _, d := range r.items {
		if d.OrganizationID == organizationID && d.Domain == domain {
			return d, nil
		}
	}
	return nil, nil
}

func (r *tenantB2BRepo) FindVerifiedByDomain(_ context.Context, domain string) (*domainb2b.OrganizationDomain, error) {
	return nil, nil
}

func (r *tenantB2BRepo) List(_ context.Context, q domainb2b.ListQuery) ([]*domainb2b.OrganizationDomain, int64, error) {
	r.lastList = q
	items := make([]*domainb2b.OrganizationDomain, 0, len(r.items))
	for _, d := range r.items {
		if d.OrganizationID == q.OrganizationID {
			items = append(items, d)
		}
	}
	return items, int64(len(items)), nil
}

func (r *tenantB2BRepo) Update(_ context.Context, d *domainb2b.OrganizationDomain) error {
	r.updateCnt++
	r.items[d.ID] = d
	return nil
}

func (r *tenantB2BRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	d, ok := r.items[id]
	if !ok || d.OrganizationID != organizationID {
		return apperrors.NewNotFound("organization_domain not found")
	}
	delete(r.items, id)
	return nil
}

func (r *tenantB2BRepo) ClaimPending(_ context.Context, _ int, _ time.Duration) ([]*domainb2b.OrganizationDomain, error) {
	return nil, nil
}

type tenantB2BRecords struct {
	domainrecords.Repository
	created []*domainrecords.Record
}

func (r *tenantB2BRecords) Create(_ context.Context, record *domainrecords.Record) error {
	r.created = append(r.created, record)
	return nil
}

type tenantB2BOutbox struct {
	events []*domainoutbox.OutboxEvent
}

func (o *tenantB2BOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, event)
	return nil
}

func (o *tenantB2BOutbox) EnqueueBatch(_ context.Context, events []*domainoutbox.OutboxEvent) error {
	o.events = append(o.events, events...)
	return nil
}

type tenantB2BDNS struct {
	records map[string][]string
}

func (d *tenantB2BDNS) LookupTXT(_ context.Context, name string) ([]string, error) {
	return d.records[name], nil
}

type tenantB2BUOW struct {
	appuow.UnitOfWork
	repo    domainb2b.Repository
	records *tenantB2BRecords
	outbox  *tenantB2BOutbox
}

func (u *tenantB2BUOW) B2BDomains() domainb2b.Repository  { return u.repo }
func (u *tenantB2BUOW) Records() domainrecords.Repository { return u.records }
func (u *tenantB2BUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *tenantB2BUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func tenantB2BRouter(repo *tenantB2BRepo, records *tenantB2BRecords) chi.Router {
	router, _ := tenantB2BRouterWithOutbox(repo, records, &tenantB2BDNS{records: map[string][]string{}})
	return router
}

func tenantB2BRouterWithOutbox(repo *tenantB2BRepo, records *tenantB2BRecords, dns *tenantB2BDNS) (chi.Router, *tenantB2BOutbox) {
	router := chi.NewRouter()
	outbox := &tenantB2BOutbox{}
	service := appb2b.NewServiceWithDNS(clock.SystemClock{}, staticUUID{},
		&tenantB2BUOW{repo: repo, records: records, outbox: outbox}, dns)
	RegisterChiRoutes(router, Handler{Service: service})
	return router, outbox
}

type staticUUID struct{}

func (staticUUID) NewString() (string, error) { return "generated-b2b-id", nil }

func b2bRequest(method, target string, body io.Reader, scope tenantctx.TenantFilterMode, requestedOrgID string) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), scopeOrg(scope, requestedOrgID), "session-org", scope)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: scopeOrg(scope, requestedOrgID),
	})
	return req.WithContext(ctx)
}

func scopeOrg(scope tenantctx.TenantFilterMode, requestedOrgID string) string {
	if scope == tenantctx.TenantFilterSuperAdmin || scope == tenantctx.TenantFilterSystem {
		return requestedOrgID
	}
	return b2bSessionOrgID
}

func serveB2BRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func b2bDomain(id, orgID string) *domainb2b.OrganizationDomain {
	return &domainb2b.OrganizationDomain{
		ID: id, OrganizationID: orgID, Domain: id + ".example.com",
		VerificationToken: "token", Status: domainb2b.StatusPending,
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}
}

// TestB2BListDomains_UsesTrustedOrganization pins that the path orgId only
// requests a tenant: a regular caller always lists its own organization.
func TestB2BListDomains_UsesTrustedOrganization(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{
		"own-domain":     b2bDomain("own-domain", b2bSessionOrgID),
		"foreign-domain": b2bDomain("foreign-domain", b2bForeignOrgID),
	}}
	router := tenantB2BRouter(repo, &tenantB2BRecords{})

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodGet,
		"/api/v1/organizations/"+b2bForeignOrgID+"/domains", nil,
		tenantctx.TenantFilterStrict, b2bForeignOrgID))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, b2bSessionOrgID, repo.lastList.OrganizationID, "the authenticated tenant must win over the path orgId")
	assert.NotContains(t, recorder.Body.String(), "foreign-domain.example.com")
}

// TestB2BDeleteDomain_ForeignTenantRejected pins that DELETE ignores a forged
// path orgId for a regular caller: the foreign claim is never deleted or audited.
func TestB2BDeleteDomain_ForeignTenantRejected(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{
		"foreign-domain": b2bDomain("foreign-domain", b2bForeignOrgID),
	}}
	records := &tenantB2BRecords{}
	router := tenantB2BRouter(repo, records)

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodDelete,
		"/api/v1/organizations/"+b2bForeignOrgID+"/domains/foreign-domain", nil,
		tenantctx.TenantFilterStrict, b2bForeignOrgID))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign domain must be indistinguishable from missing")
	assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the domain claim")
	assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
	require.Len(t, repo.items, 1, "foreign domain row must remain untouched")
}

// TestB2BByID_WithoutTenantScopeRejected pins the fail-fast before the
// repository is touched.
func TestB2BByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{
		"foreign-domain": b2bDomain("foreign-domain", b2bForeignOrgID),
	}}
	records := &tenantB2BRecords{}
	router := tenantB2BRouter(repo, records)

	requests := map[string]*http.Request{
		"list": b2bNoScopeRequest(http.MethodGet, "/api/v1/organizations/"+b2bForeignOrgID+"/domains", nil),
		"delete": b2bNoScopeRequest(http.MethodDelete,
			"/api/v1/organizations/"+b2bForeignOrgID+"/domains/foreign-domain", nil),
		"claim": b2bNoScopeRequest(http.MethodPost,
			"/api/v1/organizations/"+b2bForeignOrgID+"/domains",
			strings.NewReader(`{"domain":"corp.example.com"}`)),
		"verify": b2bNoScopeRequest(http.MethodPost,
			"/api/v1/organizations/"+b2bForeignOrgID+"/domains/foreign-domain/verify", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			recorder := serveB2BRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
	assert.Zero(t, repo.createCnt)
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
	assert.Empty(t, records.created)
}

func b2bNoScopeRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

// TestB2BDeleteDomain_OwnTenantSucceeds pins that the same operation still
// works for the authenticated tenant and audits the real organization.
func TestB2BDeleteDomain_OwnTenantSucceeds(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{
		"own-domain": b2bDomain("own-domain", b2bSessionOrgID),
	}}
	records := &tenantB2BRecords{}
	recorder := serveB2BRequest(t, tenantB2BRouter(repo, records), b2bRequest(http.MethodDelete,
		"/api/v1/organizations/"+b2bSessionOrgID+"/domains/own-domain", nil,
		tenantctx.TenantFilterStrict, b2bSessionOrgID))

	require.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, 1, repo.deleteCnt)
	require.Len(t, records.created, 1)
	assert.Equal(t, b2bSessionOrgID, records.created[0].OrganizationID, "delete audit must carry the domain organization")
}

// TestB2BClaimDomain_ForeignPathTenantRejected pins that a regular caller
// cannot claim a domain into the organization named in the path: the path org
// must equal the trusted tenant, and a mismatch is indistinguishable from a
// missing organization (no write, no audit, no outbox).
func TestB2BClaimDomain_ForeignPathTenantRejected(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{}}
	records := &tenantB2BRecords{}
	router, outbox := tenantB2BRouterWithOutbox(repo, records, &tenantB2BDNS{records: map[string][]string{}})

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodPost,
		"/api/v1/organizations/"+b2bForeignOrgID+"/domains",
		strings.NewReader(`{"domain":"corp.example.com"}`),
		tenantctx.TenantFilterStrict, b2bForeignOrgID))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "a foreign path org must look missing")
	assert.Zero(t, repo.createCnt, "foreign tenant must not receive a domain claim")
	assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
	assert.Empty(t, outbox.events, "foreign tenant must not produce an outbox event")
	assert.Empty(t, repo.items)
}

// TestB2BVerifyDomain_ForeignTenantRejected pins that verify resolves the
// domain through the trusted tenant: a path org naming the victim organization
// no longer authorizes the write, and the cross-tenant result is 404.
func TestB2BVerifyDomain_ForeignTenantRejected(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{
		"foreign-domain": b2bDomain("foreign-domain", b2bForeignOrgID),
	}}
	records := &tenantB2BRecords{}
	router, outbox := tenantB2BRouterWithOutbox(repo, records, &tenantB2BDNS{records: map[string][]string{}})

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodPost,
		"/api/v1/organizations/"+b2bForeignOrgID+"/domains/foreign-domain/verify", nil,
		tenantctx.TenantFilterStrict, b2bForeignOrgID))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign domain must be indistinguishable from missing")
	assert.Zero(t, repo.updateCnt, "foreign tenant must not persist a verification attempt")
	assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
	assert.Empty(t, outbox.events, "foreign tenant must not produce an outbox event")
	require.Len(t, repo.items, 1, "foreign domain row must remain untouched")
	assert.Equal(t, domainb2b.StatusPending, repo.items["foreign-domain"].Status)
}

// TestB2BClaimDomain_OwnTenantSucceeds pins that the authenticated tenant still
// claims domains and that the audit/outbox rows carry the trusted organization.
func TestB2BClaimDomain_OwnTenantSucceeds(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{}}
	records := &tenantB2BRecords{}
	router, outbox := tenantB2BRouterWithOutbox(repo, records, &tenantB2BDNS{records: map[string][]string{}})

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodPost,
		"/api/v1/organizations/"+b2bSessionOrgID+"/domains",
		strings.NewReader(`{"domain":"own.example.com"}`),
		tenantctx.TenantFilterStrict, b2bSessionOrgID))

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, 1, repo.createCnt)
	require.Len(t, repo.items, 1)
	assert.Equal(t, b2bSessionOrgID, repo.items["generated-b2b-id"].OrganizationID)
	require.Len(t, records.created, 1)
	assert.Equal(t, b2bSessionOrgID, records.created[0].OrganizationID)
	require.Len(t, outbox.events, 1)
	require.NotNil(t, outbox.events[0].OrganizationID)
	assert.Equal(t, b2bSessionOrgID, *outbox.events[0].OrganizationID)
}

// TestB2BClaimDomain_BodyOrganizationIDRejected pins that the claim body cannot
// carry its own organizationId: the payload field is not part of the contract
// and a write is never attributed to a body-supplied tenant.
func TestB2BClaimDomain_BodyOrganizationIDRejected(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{}}
	records := &tenantB2BRecords{}
	router, _ := tenantB2BRouterWithOutbox(repo, records, &tenantB2BDNS{records: map[string][]string{}})

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodPost,
		"/api/v1/organizations/"+b2bSessionOrgID+"/domains",
		strings.NewReader(`{"organizationId":"`+b2bForeignOrgID+`","domain":"own.example.com"}`),
		tenantctx.TenantFilterStrict, b2bSessionOrgID))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Zero(t, repo.createCnt)
	assert.Empty(t, records.created)
}

// TestB2BVerifyDomain_OwnTenantSucceeds pins the happy path: one scoped write
// and one audit/outbox event carrying the domain organization.
func TestB2BVerifyDomain_OwnTenantSucceeds(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{
		"own-domain": b2bDomain("own-domain", b2bSessionOrgID),
	}}
	records := &tenantB2BRecords{}
	dns := &tenantB2BDNS{records: map[string][]string{
		"_guliangiam-verification.own-domain.example.com": {"token"},
	}}
	router, outbox := tenantB2BRouterWithOutbox(repo, records, dns)

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodPost,
		"/api/v1/organizations/"+b2bSessionOrgID+"/domains/own-domain/verify", nil,
		tenantctx.TenantFilterStrict, b2bSessionOrgID))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"verified":true`)
	assert.Equal(t, 1, repo.updateCnt)
	require.Len(t, records.created, 1)
	assert.Equal(t, b2bSessionOrgID, records.created[0].OrganizationID)
	require.Len(t, outbox.events, 1)
	require.NotNil(t, outbox.events[0].OrganizationID)
	assert.Equal(t, b2bSessionOrgID, *outbox.events[0].OrganizationID)
}

// TestB2BClaimDomain_PlatformAdminCanCrossTenant pins the single legitimate
// override: platform scope may name another tenant explicitly.
func TestB2BClaimDomain_PlatformAdminCanCrossTenant(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{}}
	records := &tenantB2BRecords{}
	router, outbox := tenantB2BRouterWithOutbox(repo, records, &tenantB2BDNS{records: map[string][]string{}})

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodPost,
		"/api/v1/organizations/"+b2bForeignOrgID+"/domains",
		strings.NewReader(`{"domain":"corp.example.com"}`),
		tenantctx.TenantFilterSuperAdmin, b2bForeignOrgID))

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, 1, repo.createCnt)
	assert.Equal(t, b2bForeignOrgID, repo.items["generated-b2b-id"].OrganizationID)
	require.Len(t, records.created, 1)
	assert.Equal(t, b2bForeignOrgID, records.created[0].OrganizationID)
	require.Len(t, outbox.events, 1)
	require.NotNil(t, outbox.events[0].OrganizationID)
	assert.Equal(t, b2bForeignOrgID, *outbox.events[0].OrganizationID)
}

// TestB2BClaimDomain_PathUUIDCaseInsensitive pins that the path org comparison
// follows UUID semantics: the same UUID in a different letter case is the same
// tenant, and the persisted organization is the trusted one.
func TestB2BClaimDomain_PathUUIDCaseInsensitive(t *testing.T) {
	repo := &tenantB2BRepo{items: map[string]*domainb2b.OrganizationDomain{}}
	records := &tenantB2BRecords{}
	router := tenantB2BRouter(repo, records)

	recorder := serveB2BRequest(t, router, b2bRequest(http.MethodPost,
		"/api/v1/organizations/"+strings.ToUpper(b2bSessionOrgID)+"/domains",
		strings.NewReader(`{"domain":"corp.example.com"}`),
		tenantctx.TenantFilterStrict, b2bSessionOrgID))

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Len(t, repo.items, 1)
	assert.Equal(t, b2bSessionOrgID, repo.items["generated-b2b-id"].OrganizationID)
}
