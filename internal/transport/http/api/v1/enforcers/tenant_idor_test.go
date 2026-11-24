package enforcers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appenforcers "github.com/guliangiam/guliangiam/internal/app/enforcers"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

const (
	idorSessionOrg = "11111111-1111-1111-1111-111111111111"
	idorForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorEnforcerID = "33333333-3333-3333-3333-333333333333"
	idorModelID    = "44444444-4444-4444-4444-444444444444"
)

type idorClock struct{}

func (idorClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorUUID struct{}

func (idorUUID) NewString() (string, error) { return "generated-id", nil }

// idorEnforcerRepo is a tenant-aware in-memory repository. A foreign enforcer
// behaves like a missing one; counters expose whether Casbin was reached at all.
type idorEnforcerRepo struct {
	enforcers   []*domain.Enforcer
	getCalls    int
	scopedOrgs  []string
	updateCalls int
	deleteCalls int
}

func (r *idorEnforcerRepo) GetByID(_ context.Context, id string) (*domain.Enforcer, error) {
	r.getCalls++
	if e := r.find(id, ""); e != nil {
		return e, nil
	}
	return nil, apperrors.NewNotFound("enforcer not found")
}

func (r *idorEnforcerRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Enforcer, error) {
	r.getCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	if e := r.find(id, organizationID); e != nil {
		return e, nil
	}
	return nil, apperrors.NewNotFound("enforcer not found")
}

func (r *idorEnforcerRepo) find(id, organizationID string) *domain.Enforcer {
	for _, e := range r.enforcers {
		if e.ID != id {
			continue
		}
		if organizationID == "" || e.OrganizationID == organizationID {
			return e
		}
	}
	return nil
}

func (r *idorEnforcerRepo) List(context.Context, domain.ListQuery) ([]*domain.Enforcer, int64, error) {
	return r.enforcers, int64(len(r.enforcers)), nil
}

func (r *idorEnforcerRepo) Create(context.Context, *domain.Enforcer) error { return nil }

func (r *idorEnforcerRepo) Update(_ context.Context, e *domain.Enforcer) error {
	r.updateCalls++
	for i, existing := range r.enforcers {
		if existing.ID == e.ID && existing.OrganizationID == e.OrganizationID {
			r.enforcers[i] = e
			return nil
		}
	}
	return apperrors.NewNotFound("enforcer not found")
}

func (r *idorEnforcerRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, e := range r.enforcers {
		if e.ID == id && e.OrganizationID == organizationID {
			r.enforcers = append(r.enforcers[:i], r.enforcers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("enforcer not found")
}

type idorModels struct {
	calls int
}

func (m *idorModels) GetByID(context.Context, string) (*modeldomain.Model, error) {
	m.calls++
	return &modeldomain.Model{ID: idorModelID, ModelText: "model_text"}, nil
}

func (m *idorModels) GetByIDForOrganization(context.Context, string, string) (*modeldomain.Model, error) {
	m.calls++
	return &modeldomain.Model{ID: idorModelID, ModelText: "model_text"}, nil
}

func (m *idorModels) List(context.Context, modeldomain.ListQuery) ([]*modeldomain.Model, int64, error) {
	return nil, 0, nil
}

func (m *idorModels) Create(context.Context, *modeldomain.Model) error { return nil }

func (m *idorModels) Update(context.Context, *modeldomain.Model) error { return nil }

func (m *idorModels) Delete(context.Context, string, string) error { return nil }

// idorCasbin counts policy reads, writes and enforcement calls so a rejected
// request can be proven to have never touched casbin_rules (which has no
// organization column or RLS).
type idorCasbin struct {
	reads         int
	writes        int
	enforces      int
	batchEnforces int
	policyReloads int
}

func (c *idorCasbin) Enforce(context.Context, string, string, ...interface{}) (bool, error) {
	c.enforces++
	return true, nil
}

func (c *idorCasbin) BatchEnforce(context.Context, string, string, [][]interface{}) ([]bool, error) {
	c.batchEnforces++
	return []bool{true}, nil
}

func (c *idorCasbin) AddPolicy(string, string, []string) error {
	c.writes++
	return nil
}

func (c *idorCasbin) RemovePolicy(string, string, []string) error {
	c.writes++
	return nil
}

func (c *idorCasbin) GetPolicies(string, string) ([][]string, error) {
	c.reads++
	return [][]string{{"p", "alice", "data1", "read"}}, nil
}

func (c *idorCasbin) GetFilteredPolicies(string, string, int, ...string) ([][]string, error) {
	c.reads++
	return [][]string{{"p", "alice", "data1", "read"}}, nil
}

func (c *idorCasbin) UpdatePolicy(string, string, []string, []string) error {
	c.writes++
	return nil
}

func (c *idorCasbin) RemoveFilteredPolicy(string, string, int, ...string) error {
	return nil
}

func (c *idorCasbin) ReloadPolicy(string, string) error {
	c.policyReloads++
	return nil
}

func (c *idorCasbin) GetAllSubjects(string, string) ([]string, error) {
	c.reads++
	return []string{"alice"}, nil
}

func (c *idorCasbin) GetAllObjects(string, string) ([]string, error) {
	c.reads++
	return []string{"data1"}, nil
}

func (c *idorCasbin) GetAllActions(string, string) ([]string, error) {
	c.reads++
	return []string{"read"}, nil
}

func (c *idorCasbin) GetAllRoles(string, string) ([]string, error) {
	c.reads++
	return []string{"admin"}, nil
}

func (c *idorCasbin) AddRoleForUser(string, string, string, string) error { return nil }

func (c *idorCasbin) GetOrgEnforcerPolicies(string) ([][]string, error) { return nil, nil }

func (c *idorCasbin) AddOrgPolicy(string, []string) error { return nil }

func (c *idorCasbin) RemoveOrgFilteredPolicy(string, int, ...string) error { return nil }

func (c *idorCasbin) AddOrgRoleForUser(string, string, string) error { return nil }

func (c *idorCasbin) RemoveOrgFilteredGroupingPolicy(string, int, ...string) error { return nil }

func (c *idorCasbin) InvalidateAll() {}

func (c *idorCasbin) GetOrgSubjectPolicies(string, string) ([][]string, error) { return nil, nil }

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

type idorOutbox struct {
	events []*domainoutbox.OutboxEvent
}

func (o *idorOutbox) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	o.events = append(o.events, event)
	return nil
}

func (o *idorOutbox) EnqueueBatch(_ context.Context, events []*domainoutbox.OutboxEvent) error {
	o.events = append(o.events, events...)
	return nil
}

type idorUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	models  *idorModels
	records *idorRecords
	outbox  *idorOutbox
}

func (u *idorUOW) Enforcers() domain.Repository      { return u.repo }
func (u *idorUOW) Models() modeldomain.Repository    { return u.models }
func (u *idorUOW) Records() recordsdomain.Repository { return u.records }
func (u *idorUOW) Outbox() domainoutbox.Repository   { return u.outbox }
func (u *idorUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

type idorHarness struct {
	repo    *idorEnforcerRepo
	models  *idorModels
	records *idorRecords
	outbox  *idorOutbox
	casbin  *idorCasbin
	router  chi.Router
}

func newIDORHarness(enforcers ...*domain.Enforcer) *idorHarness {
	h := &idorHarness{
		repo:    &idorEnforcerRepo{enforcers: enforcers},
		models:  &idorModels{},
		records: &idorRecords{},
		outbox:  &idorOutbox{},
		casbin:  &idorCasbin{},
	}
	service := appenforcers.NewService(idorClock{}, idorUUID{},
		&idorUOW{repo: h.repo, models: h.models, records: h.records, outbox: h.outbox}, h.casbin)
	h.router = chi.NewRouter()
	RegisterChiRoutes(h.router, Handler{Service: service})
	return h
}

func idorRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "55555555-5555-5555-5555-555555555555", OrganizationID: idorSessionOrg,
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

func serveIDOR(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
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

var idorEnforcerRoutes = []struct {
	name   string
	method string
	path   string
	body   string
}{
	{"get", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "?organizationId=" + idorForeignOrg, ""},
	{"patch", http.MethodPatch, "/api/v1/enforcers/" + idorEnforcerID + "?organizationId=" + idorForeignOrg, `{"displayName":"hijacked","revision":1}`},
	{"delete", http.MethodDelete, "/api/v1/enforcers/" + idorEnforcerID + "?organizationId=" + idorForeignOrg, ""},
	{"get-policies", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "/policies?organizationId=" + idorForeignOrg, ""},
	{"get-filtered-policies", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "/policies?fieldIndex=0&fieldValue=alice&organizationId=" + idorForeignOrg, ""},
	{"remove-policy", http.MethodDelete, "/api/v1/enforcers/" + idorEnforcerID + "/policies?organizationId=" + idorForeignOrg, `{"pType":"p","rules":["alice","data1","read"]}`},
	{"update-policy", http.MethodPut, "/api/v1/enforcers/" + idorEnforcerID + "/policies?organizationId=" + idorForeignOrg, `{"oldPolicy":["p","alice"],"newPolicy":["p","bob"]}`},
	{"subjects", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "/subjects?organizationId=" + idorForeignOrg, ""},
	{"objects", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "/objects?organizationId=" + idorForeignOrg, ""},
	{"actions", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "/actions?organizationId=" + idorForeignOrg, ""},
	{"roles", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "/roles?organizationId=" + idorForeignOrg, ""},
	{"export-policies", http.MethodGet, "/api/v1/enforcers/" + idorEnforcerID + "/export-policies?organizationId=" + idorForeignOrg, ""},
	{"enforce", http.MethodPost, "/api/v1/enforcers/" + idorEnforcerID + "/enforce?organizationId=" + idorForeignOrg, `{"pType":"p","v0":"alice","v1":"data1","v2":"read"}`},
	{"batch-enforce", http.MethodPost, "/api/v1/enforcers/" + idorEnforcerID + "/batch-enforce?organizationId=" + idorForeignOrg, `{"requests":[{"pType":"p","v0":"alice","v1":"data1","v2":"read"}]}`},
	{"add-policy", http.MethodPost, "/api/v1/enforcers/" + idorEnforcerID + "/policies?organizationId=" + idorForeignOrg, `{"pType":"p","rules":["alice","data1","read"]}`},
	{"reload", http.MethodPost, "/api/v1/enforcers/" + idorEnforcerID + "/reload?organizationId=" + idorForeignOrg, ""},
	{"import-policies", http.MethodPost, "/api/v1/enforcers/" + idorEnforcerID + "/import-policies?organizationId=" + idorForeignOrg, `{"policies":[["p","alice","data1","read"]]}`},
}

// idorUploadBody builds a valid policy workbook so the upload route reaches the
// enforcer lookup instead of failing multipart parsing.
func idorUploadBody(t *testing.T) (io.Reader, string) {
	t.Helper()
	workbook := excelize.NewFile()
	sheet := workbook.GetSheetName(0)
	for cell, value := range map[string]string{
		"A1": "PType", "B1": "V0", "C1": "V1", "D1": "V2",
		"A2": "p", "B2": "alice", "C2": "data1", "D2": "read",
	} {
		require.NoError(t, workbook.SetCellValue(sheet, cell, value))
	}
	payload, err := workbook.WriteToBuffer()
	require.NoError(t, err)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "policies.xlsx")
	require.NoError(t, err)
	_, err = part.Write(payload.Bytes())
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return &body, writer.FormDataContentType()
}

func idorUploadRequest(t *testing.T, target string, withTenantScope bool) *http.Request {
	t.Helper()
	body, contentType := idorUploadBody(t)
	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set("Content-Type", contentType)
	if !withTenantScope {
		return req
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "55555555-5555-5555-5555-555555555555", OrganizationID: idorSessionOrg,
	})
	return req.WithContext(ctx)
}

func tenantEnforcer(organizationID string) *domain.Enforcer {
	return &domain.Enforcer{
		ID: idorEnforcerID, OrganizationID: organizationID, Name: "tenant-enforcer",
		DisplayName: "Tenant Enforcer", ModelID: idorModelID, IsEnabled: true, Revision: 1,
	}
}

// TestEnforcerByID_ForeignTenantRejected pins that every by-ID route resolves
// the authenticated tenant, not the caller-supplied organizationId, and that a
// foreign enforcer is indistinguishable from a missing one: no 403, no write,
// no audit event, and no casbin_rules access (that table has no org column/RLS).
func TestEnforcerByID_ForeignTenantRejected(t *testing.T) {
	assertRejected := func(t *testing.T, h *idorHarness, recorder *httptest.ResponseRecorder) {
		t.Helper()
		assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign enforcer must look missing")
		require.NotEmpty(t, h.repo.scopedOrgs, "ownership must be resolved through the scoped lookup")
		for _, org := range h.repo.scopedOrgs {
			assert.Equal(t, idorSessionOrg, org, "forged query organizationId must be ignored")
		}
		assert.Zero(t, h.casbin.reads, "foreign tenant must not read casbin policies")
		assert.Zero(t, h.casbin.writes, "foreign tenant must not write casbin policies")
		assert.Zero(t, h.casbin.enforces, "foreign tenant must not run enforce")
		assert.Zero(t, h.casbin.batchEnforces, "foreign tenant must not run batch enforce")
		assert.Zero(t, h.casbin.policyReloads, "foreign tenant must not reload casbin policies")
		assert.Zero(t, h.models.calls, "foreign tenant must not resolve the casbin model")
		assert.Zero(t, h.repo.updateCalls, "foreign tenant must not update the enforcer")
		assert.Zero(t, h.repo.deleteCalls, "foreign tenant must not delete the enforcer")
		assert.Empty(t, h.records.created, "foreign tenant must not produce an audit event")
		assert.Empty(t, h.outbox.events, "foreign tenant must not produce an outbox event")
		require.Len(t, h.repo.enforcers, 1, "foreign enforcer row must remain untouched")
	}

	for _, tt := range idorEnforcerRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(tenantEnforcer(idorForeignOrg))
			recorder := serveIDOR(h.router, idorRequest(tt.method, tt.path, idorBody(t, tt.body)))
			assertRejected(t, h, recorder)
		})
	}

	t.Run("upload-policies", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, idorUploadRequest(t,
			"/api/v1/enforcers/"+idorEnforcerID+"/upload-policies?organizationId="+idorForeignOrg, true))
		assertRejected(t, h, recorder)
	})
}

// TestEnforcerByID_WithoutTenantScopeRejected pins the fail-fast: when neither
// the session nor a platform-admin target provides a tenant, every by-ID route
// is rejected before touching the repository, the model store or Casbin.
func TestEnforcerByID_WithoutTenantScopeRejected(t *testing.T) {
	assertRejected := func(t *testing.T, h *idorHarness, recorder *httptest.ResponseRecorder) {
		t.Helper()
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Zero(t, h.repo.getCalls)
		assert.Zero(t, h.repo.updateCalls)
		assert.Zero(t, h.repo.deleteCalls)
		assert.Zero(t, h.models.calls)
		assert.Zero(t, h.casbin.reads)
		assert.Zero(t, h.casbin.writes)
		assert.Zero(t, h.casbin.enforces)
		assert.Zero(t, h.casbin.batchEnforces)
		assert.Zero(t, h.casbin.policyReloads)
		assert.Empty(t, h.records.created)
		assert.Empty(t, h.outbox.events)
	}

	for _, tt := range idorEnforcerRoutes {
		t.Run(tt.name, func(t *testing.T) {
			h := newIDORHarness(tenantEnforcer(idorForeignOrg))

			req := httptest.NewRequest(tt.method, tt.path, idorBody(t, tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			// Strip the forged query value: an empty trusted org is the failure mode.
			req.URL.RawQuery = ""

			assertRejected(t, h, serveIDOR(h.router, req))
		})
	}

	t.Run("upload-policies", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, idorUploadRequest(t,
			"/api/v1/enforcers/"+idorEnforcerID+"/upload-policies", false))
		assertRejected(t, h, recorder)
	})
}

// TestEnforcerByID_OwnTenantSucceeds pins that every by-ID route still works
// inside the authenticated tenant, that delete and policy writes audit the real
// organization (never the built-in fallback), and that policy reads/writes reach
// Casbin.
func TestEnforcerByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodGet, "/api/v1/enforcers/"+idorEnforcerID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "tenant-enforcer")
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorSessionOrg, h.repo.scopedOrgs[0])
	})

	t.Run("patch", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPatch, "/api/v1/enforcers/"+idorEnforcerID,
			strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodDelete, "/api/v1/enforcers/"+idorEnforcerID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.deleteCalls)
		assert.Empty(t, h.repo.enforcers)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorSessionOrg, h.records.created[0].OrganizationID,
			"delete audit must carry the enforcer organization, not the built-in fallback")
	})

	t.Run("get-policies", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodGet, "/api/v1/enforcers/"+idorEnforcerID+"/policies", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.reads)
		assert.Zero(t, h.casbin.writes)
		assert.Contains(t, recorder.Body.String(), "alice")
	})

	t.Run("get-filtered-policies", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodGet,
			"/api/v1/enforcers/"+idorEnforcerID+"/policies?fieldIndex=0&fieldValue=alice", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.reads)
	})

	t.Run("remove-policy", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodDelete, "/api/v1/enforcers/"+idorEnforcerID+"/policies",
			strings.NewReader(`{"pType":"p","rules":["alice","data1","read"]}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.writes)
		assert.Zero(t, h.casbin.reads)
	})

	t.Run("update-policy", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPut, "/api/v1/enforcers/"+idorEnforcerID+"/policies",
			strings.NewReader(`{"oldPolicy":["p","alice"],"newPolicy":["p","bob"]}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.writes)
	})

	for _, route := range []struct {
		name string
		path string
	}{
		{"subjects", "/subjects"},
		{"objects", "/objects"},
		{"actions", "/actions"},
		{"roles", "/roles"},
		{"export-policies", "/export-policies"},
	} {
		t.Run(route.name, func(t *testing.T) {
			h := newIDORHarness(tenantEnforcer(idorSessionOrg))
			recorder := serveIDOR(h.router, idorRequest(http.MethodGet,
				"/api/v1/enforcers/"+idorEnforcerID+route.path, nil))

			require.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, 1, h.casbin.reads)
			assert.Zero(t, h.casbin.writes)
		})
	}

	t.Run("enforce", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPost, "/api/v1/enforcers/"+idorEnforcerID+"/enforce",
			strings.NewReader(`{"pType":"p","v0":"alice","v1":"data1","v2":"read"}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.enforces)
		assert.Zero(t, h.casbin.writes)
		assert.Empty(t, h.records.created, "enforce is a read action and must not be audited")
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorSessionOrg, h.repo.scopedOrgs[0])
	})

	t.Run("batch-enforce", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPost, "/api/v1/enforcers/"+idorEnforcerID+"/batch-enforce",
			strings.NewReader(`{"requests":[{"pType":"p","v0":"alice","v1":"data1","v2":"read"}]}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.batchEnforces)
		assert.Zero(t, h.casbin.writes)
		assert.Empty(t, h.records.created)
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorSessionOrg, h.repo.scopedOrgs[0])
	})

	t.Run("add-policy", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPost, "/api/v1/enforcers/"+idorEnforcerID+"/policies",
			strings.NewReader(`{"pType":"p","rules":["alice","data1","read"]}`)))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, 1, h.casbin.writes)
		assertPolicyAudit(t, h, idorSessionOrg, "enforcers.policy.added")
	})

	t.Run("reload", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPost, "/api/v1/enforcers/"+idorEnforcerID+"/reload", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.policyReloads)
		assertPolicyAudit(t, h, idorSessionOrg, "enforcers.policy.reloaded")
	})

	t.Run("import-policies", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorRequest(http.MethodPost, "/api/v1/enforcers/"+idorEnforcerID+"/import-policies",
			strings.NewReader(`{"policies":[["p","alice","data1","read"]]}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 2, h.casbin.writes, "one remove for the existing rule plus one add")
		assertPolicyAudit(t, h, idorSessionOrg, "enforcers.policy.imported")
	})

	t.Run("upload-policies", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorSessionOrg))
		recorder := serveIDOR(h.router, idorUploadRequest(t,
			"/api/v1/enforcers/"+idorEnforcerID+"/upload-policies", true))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 2, h.casbin.writes)
		assertPolicyAudit(t, h, idorSessionOrg, "enforcers.policy.uploaded")
	})
}

// assertPolicyAudit pins that a policy write is audited with the real enforcer
// organization (casbin_rules itself has no organization column or RLS).
func assertPolicyAudit(t *testing.T, h *idorHarness, organizationID, action string) {
	t.Helper()
	require.Len(t, h.records.created, 1)
	assert.Equal(t, organizationID, h.records.created[0].OrganizationID)
	assert.Equal(t, action, h.records.created[0].Action)
	require.Len(t, h.outbox.events, 1)
	require.NotNil(t, h.outbox.events[0].OrganizationID)
	assert.Equal(t, organizationID, *h.outbox.events[0].OrganizationID)
	assert.Equal(t, action, h.outbox.events[0].EventType)
}

// TestEnforcerByID_PlatformAdminCanTargetTenant pins that a platform scope
// keeps the explicit cross-tenant target; the requested organization is then
// the repository predicate and the audit organization.
func TestEnforcerByID_PlatformAdminCanTargetTenant(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodGet,
			"/api/v1/enforcers/"+idorEnforcerID+"?organizationId="+idorForeignOrg, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorForeignOrg, h.repo.scopedOrgs[0])
	})

	t.Run("patch keeps the target tenant", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodPatch,
			"/api/v1/enforcers/"+idorEnforcerID+"?organizationId="+idorForeignOrg,
			strings.NewReader(`{"displayName":"admin-renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.repo.updateCalls)
		require.Len(t, h.repo.enforcers, 1)
		assert.Equal(t, idorForeignOrg, h.repo.enforcers[0].OrganizationID)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorForeignOrg, h.records.created[0].OrganizationID)
	})

	t.Run("delete audits the target tenant", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodDelete,
			"/api/v1/enforcers/"+idorEnforcerID+"?organizationId="+idorForeignOrg, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorForeignOrg, h.records.created[0].OrganizationID)
	})

	t.Run("empty target rejected", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodGet,
			"/api/v1/enforcers/"+idorEnforcerID, nil))

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Zero(t, h.repo.getCalls)
		assert.Zero(t, h.casbin.reads)
	})

	t.Run("enforce targets the requested tenant", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodPost,
			"/api/v1/enforcers/"+idorEnforcerID+"/enforce?organizationId="+idorForeignOrg,
			strings.NewReader(`{"pType":"p","v0":"alice","v1":"data1","v2":"read"}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, h.casbin.enforces)
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorForeignOrg, h.repo.scopedOrgs[0])
	})

	t.Run("add-policy audits the requested tenant", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodPost,
			"/api/v1/enforcers/"+idorEnforcerID+"/policies?organizationId="+idorForeignOrg,
			strings.NewReader(`{"pType":"p","rules":["alice","data1","read"]}`)))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, 1, h.casbin.writes)
		require.NotEmpty(t, h.repo.scopedOrgs)
		assert.Equal(t, idorForeignOrg, h.repo.scopedOrgs[0])
		require.Len(t, h.records.created, 1)
		assert.Equal(t, idorForeignOrg, h.records.created[0].OrganizationID)
	})

	t.Run("policy write without an explicit target rejected", func(t *testing.T) {
		h := newIDORHarness(tenantEnforcer(idorForeignOrg))
		recorder := serveIDOR(h.router, platformAdminRequest(http.MethodPost,
			"/api/v1/enforcers/"+idorEnforcerID+"/policies",
			strings.NewReader(`{"pType":"p","rules":["alice","data1","read"]}`)))

		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Zero(t, h.repo.getCalls)
		assert.Zero(t, h.casbin.writes)
		assert.Empty(t, h.records.created)
	})
}
