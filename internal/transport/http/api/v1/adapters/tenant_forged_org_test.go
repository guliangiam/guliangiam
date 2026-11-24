package adapters

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appadapters "github.com/guliangiam/guliangiam/internal/app/adapters"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-adapter-id", nil }

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

// platformRequest simulates a platform administrator scope: only this mode may
// narrow the tenant with an explicit organizationId.
func platformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

// tenantByIDRepo is a tenant-aware in-memory adapter repository. It deliberately
// avoids gomock so update/delete/audit call counts stay observable.
type tenantByIDRepo struct {
	adapters  []*domain.Adapter
	getCalls  int
	updateCnt int
	deleteCnt int
	lastList  domain.ListQuery
}

func (r *tenantByIDRepo) GetByID(_ context.Context, id, organizationID string) (*domain.Adapter, error) {
	r.getCalls++
	for _, a := range r.adapters {
		if a.ID == id && a.OrganizationID == organizationID {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("adapter not found")
}

func (r *tenantByIDRepo) List(_ context.Context, query domain.ListQuery) ([]*domain.Adapter, int64, error) {
	r.lastList = query
	return r.adapters, int64(len(r.adapters)), nil
}

func (r *tenantByIDRepo) Create(_ context.Context, adapter *domain.Adapter) error {
	r.adapters = append(r.adapters, adapter)
	return nil
}

func (r *tenantByIDRepo) Update(_ context.Context, adapter *domain.Adapter) error {
	r.updateCnt++
	for i, a := range r.adapters {
		if a.ID == adapter.ID && a.OrganizationID == adapter.OrganizationID {
			r.adapters[i] = adapter
			return nil
		}
	}
	return apperrors.NewNotFound("adapter not found")
}

func (r *tenantByIDRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, a := range r.adapters {
		if a.ID == id && a.OrganizationID == organizationID {
			r.adapters = append(r.adapters[:i], r.adapters[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("adapter not found")
}

type recordingRecords struct {
	created []*recordsdomain.Record
}

func (r *recordingRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *recordingRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *recordingRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *recordingRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *recordingRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *recordingRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *recordingRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

// adapterScopeUOW exposes only the repos used by the adapter service; the
// embedded UnitOfWork carries the remaining interface surface without
// implementation.
type adapterScopeUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *recordingRecords
}

func (u *adapterScopeUOW) Adapters() domain.Repository       { return u.repo }
func (u *adapterScopeUOW) Records() recordsdomain.Repository { return u.records }
func (u *adapterScopeUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *adapterScopeUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newAdapterScopeService(repo domain.Repository) (appadapters.Service, *recordingRecords) {
	records := &recordingRecords{}
	return appadapters.NewService(forgedOrgClock{}, forgedOrgUUID{}, &adapterScopeUOW{repo: repo, records: records}, nil), records
}

func tenantAdapter(adapterID, orgID string) *domain.Adapter {
	return &domain.Adapter{
		ID: adapterID, OrganizationID: orgID, Name: "tenant-adapter",
		DisplayName: "Tenant Adapter", Server: "db.example.com",
		Username: "admin", Password: "super-secret-password",
		DbPassword: "super-secret-db-password", Revision: 1,
	}
}

func adapterRouter(service appadapters.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveAdapterRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestListAdapters_ForgedQueryOrganizationIDIgnored pins that the query value
// only requests a tenant; the authenticated scope decides the repository filter.
func TestListAdapters_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	repo := &tenantByIDRepo{}
	service, _ := newAdapterScopeService(repo)
	recorder := serveAdapterRequest(t, adapterRouter(service), forgedOrgRequest(http.MethodGet,
		"/api/v1/adapters?organizationId="+forgedOrgRequestID, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, forgedOrgSessionID, repo.lastList.OrganizationID)
}

// TestCreateAdapter_ForgedBodyOrganizationIgnored pins that a write cannot be
// moved into another tenant by a forged body organizationId.
func TestCreateAdapter_ForgedBodyOrganizationIgnored(t *testing.T) {
	repo := &tenantByIDRepo{}
	service, _ := newAdapterScopeService(repo)
	recorder := serveAdapterRequest(t, adapterRouter(service), forgedOrgRequest(http.MethodPost,
		"/api/v1/adapters",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"new-adapter"}`)))

	require.Equal(t, http.StatusCreated, recorder.Code)
	require.Len(t, repo.adapters, 1)
	assert.Equal(t, forgedOrgSessionID, repo.adapters[0].OrganizationID)
}

// TestCreateAdapter_WithoutTenantScopeRejected pins the write fail-fast: a
// forged body organizationId must not let the request reach the repository.
func TestCreateAdapter_WithoutTenantScopeRejected(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/adapters",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"adapter"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})
	router.ServeHTTP(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}
