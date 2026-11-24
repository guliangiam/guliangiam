package entries

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appentries "github.com/guliangiam/guliangiam/internal/app/entries"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/entries"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tenantEntryRepo is an in-memory entry repository whose write/audit call
// counts stay observable across a by-ID request.
type tenantEntryRepo struct {
	entries   []*domain.Entry
	getCalls  int
	updateCnt int
	deleteCnt int
}

func (r *tenantEntryRepo) GetByID(_ context.Context, id string) (*domain.Entry, error) {
	r.getCalls++
	for _, e := range r.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, apperrors.NewNotFound("entry not found")
}

func (r *tenantEntryRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Entry, error) {
	r.getCalls++
	for _, e := range r.entries {
		if e.ID == id && e.OrganizationID == organizationID {
			return e, nil
		}
	}
	return nil, apperrors.NewNotFound("entry not found")
}

func (r *tenantEntryRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Entry, int64, error) {
	return r.entries, int64(len(r.entries)), nil
}

func (r *tenantEntryRepo) Create(_ context.Context, e *domain.Entry) error {
	r.entries = append(r.entries, e)
	return nil
}

func (r *tenantEntryRepo) Update(_ context.Context, e *domain.Entry) error {
	r.updateCnt++
	for i, item := range r.entries {
		if item.ID == e.ID {
			r.entries[i] = e
			return nil
		}
	}
	return apperrors.NewNotFound("entry not found")
}

func (r *tenantEntryRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCnt++
	for i, e := range r.entries {
		if e.ID == id && e.OrganizationID == organizationID {
			r.entries = append(r.entries[:i], r.entries[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("entry not found")
}

type tenantEntryRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *tenantEntryRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type tenantEntryUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *tenantEntryRecords
}

func (u *tenantEntryUOW) Entries() domain.Repository        { return u.repo }
func (u *tenantEntryUOW) Records() recordsdomain.Repository { return u.records }
func (u *tenantEntryUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *tenantEntryUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func tenantEntryRouter(repo *tenantEntryRepo, records *tenantEntryRecords) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appentries.NewService(forgedOrgClock{}, forgedOrgUUID{}, &tenantEntryUOW{repo: repo, records: records})})
	return router
}

func tenantEntryRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func serveTenantEntryRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func foreignTenantEntry() *domain.Entry {
	return &domain.Entry{
		ID: "foreign-entry", OrganizationID: forgedOrgRequestID,
		Name: "foreign", DisplayName: "Foreign", Revision: 1,
	}
}

func ownTenantEntry() *domain.Entry {
	return &domain.Entry{
		ID: "own-entry", OrganizationID: forgedOrgSessionID,
		Name: "own", DisplayName: "Own", Revision: 1,
	}
}

// TestEntryByID_ForeignTenantRejected pins that GET/PATCH/DELETE resolve the
// authenticated tenant, never the caller-supplied organizationId: a foreign
// entry looks exactly like a missing one, with zero writes and zero audits.
func TestEntryByID_ForeignTenantRejected(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/entries/foreign-entry?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/entries/foreign-entry?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/entries/foreign-entry?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &tenantEntryRepo{entries: []*domain.Entry{foreignTenantEntry()}}
			records := &tenantEntryRecords{}
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records), forgedOrgRequest(tt.method, tt.path, body))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign entry must be indistinguishable from missing")
			assert.Zero(t, repo.updateCnt, "foreign tenant must not update the entry")
			assert.Zero(t, repo.deleteCnt, "foreign tenant must not delete the entry")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			require.Len(t, repo.entries, 1, "foreign entry row must remain untouched")
		})
	}
}

// TestEntryByID_WithoutTenantScopeRejected pins the fail-fast before the
// repository is touched (MySQL has no RLS fallback).
func TestEntryByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &tenantEntryRepo{entries: []*domain.Entry{ownTenantEntry()}}
	records := &tenantEntryRecords{}
	router := tenantEntryRouter(repo, records)

	requests := map[string]*http.Request{
		"get":    tenantEntryRequest(http.MethodGet, "/api/v1/entries/own-entry", nil),
		"patch":  tenantEntryRequest(http.MethodPatch, "/api/v1/entries/own-entry", strings.NewReader(`{"displayName":"renamed"}`)),
		"delete": tenantEntryRequest(http.MethodDelete, "/api/v1/entries/own-entry", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			recorder := serveTenantEntryRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
	assert.Zero(t, repo.updateCnt)
	assert.Zero(t, repo.deleteCnt)
	assert.Empty(t, records.created)
}

// TestEntryByID_OwnTenantSucceeds pins that the same operations still work for
// the authenticated tenant and that writes audit the entry's organization.
func TestEntryByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		repo := &tenantEntryRepo{entries: []*domain.Entry{ownTenantEntry()}}
		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, &tenantEntryRecords{}),
			forgedOrgRequest(http.MethodGet, "/api/v1/entries/own-entry", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), `"organizationId":"`+forgedOrgSessionID+`"`)
	})

	t.Run("patch", func(t *testing.T) {
		repo := &tenantEntryRepo{entries: []*domain.Entry{ownTenantEntry()}}
		records := &tenantEntryRecords{}
		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			forgedOrgRequest(http.MethodPatch, "/api/v1/entries/own-entry",
				strings.NewReader(`{"displayName":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &tenantEntryRepo{entries: []*domain.Entry{ownTenantEntry()}}
		records := &tenantEntryRecords{}
		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			forgedOrgRequest(http.MethodDelete, "/api/v1/entries/own-entry", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCnt)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID, "delete audit must carry the entry organization")
	})
}

// TestEntryTranscript_TenantBindingUnchanged confirms this batch leaves the
// already-bound transcript route alone: a foreign entry is not found, and an
// empty trusted tenant is a client error.
func TestEntryTranscript_TenantBindingUnchanged(t *testing.T) {
	t.Run("foreign entry is not found", func(t *testing.T) {
		repo := &tenantEntryRepo{entries: []*domain.Entry{foreignTenantEntry()}}
		records := &tenantEntryRecords{}
		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			forgedOrgRequest(http.MethodGet, "/api/v1/entries/foreign-entry/transcript?organizationId="+forgedOrgRequestID, nil))

		require.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Empty(t, records.created, "a rejected transcript read must not be audited")
	})

	t.Run("empty tenant is rejected", func(t *testing.T) {
		repo := &tenantEntryRepo{entries: []*domain.Entry{ownTenantEntry()}}
		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, &tenantEntryRecords{}),
			tenantEntryRequest(http.MethodGet, "/api/v1/entries/own-entry/transcript", nil))

		require.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("own entry remains readable", func(t *testing.T) {
		repo := &tenantEntryRepo{entries: []*domain.Entry{ownTenantEntry()}}
		records := &tenantEntryRecords{}
		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			forgedOrgRequest(http.MethodGet, "/api/v1/entries/own-entry/transcript", nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})
}

// TestEntryByID_ForgedOrgDoesNotLeakSessionTenant pins that the forged query
// value also cannot narrow the caller to another tenant on a successful read.
func TestEntryByID_ForgedOrgDoesNotLeakSessionTenant(t *testing.T) {
	repo := &tenantEntryRepo{entries: []*domain.Entry{
		ownTenantEntry(),
		{ID: "second-entry", OrganizationID: forgedOrgSessionID, Name: "second", Revision: 1},
	}}
	recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, &tenantEntryRecords{}),
		forgedOrgRequest(http.MethodGet, "/api/v1/entries/own-entry?organizationId="+forgedOrgRequestID, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, 1, repo.getCalls)
}

// ─── POST /entries: body organizationId is caller input ──────────────

const platformAdminEntryTargetOrg = "33333333-3333-3333-3333-333333333333"

func entryCreateBody(organizationID string) string {
	return `{"organizationId":"` + organizationID + `","name":"created-entry","displayName":"Created Entry","message":"created"}`
}

func platformAdminEntryRequest(body io.Reader) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/entries", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

// TestEntryCreate_BindsOrganizationToTrustedTenant pins the three create-route
// states: no trusted tenant is a 400, a forged body organizationId is replaced
// by the authenticated tenant, and the owning tenant still writes normally. A
// platform administrator keeps the documented cross-tenant override.
func TestEntryCreate_BindsOrganizationToTrustedTenant(t *testing.T) {
	t.Run("empty trusted tenant rejected", func(t *testing.T) {
		repo := &tenantEntryRepo{}
		records := &tenantEntryRecords{}

		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			tenantEntryRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(entryCreateBody(forgedOrgRequestID))))

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "organizationId is required")
		assert.Empty(t, repo.entries)
		assert.Empty(t, records.created)
	})

	t.Run("forged organizationId is ignored", func(t *testing.T) {
		repo := &tenantEntryRepo{}
		records := &tenantEntryRecords{}

		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			forgedOrgRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(entryCreateBody(forgedOrgRequestID))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		require.Len(t, repo.entries, 1)
		assert.Equal(t, forgedOrgSessionID, repo.entries[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("own tenant succeeds", func(t *testing.T) {
		repo := &tenantEntryRepo{}
		records := &tenantEntryRecords{}

		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			forgedOrgRequest(http.MethodPost, "/api/v1/entries", strings.NewReader(entryCreateBody(forgedOrgSessionID))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, forgedOrgSessionID, repo.entries[0].OrganizationID)
		require.Len(t, records.created, 1)
	})

	t.Run("platform admin may target another tenant", func(t *testing.T) {
		repo := &tenantEntryRepo{}
		records := &tenantEntryRecords{}

		recorder := serveTenantEntryRequest(t, tenantEntryRouter(repo, records),
			platformAdminEntryRequest(strings.NewReader(entryCreateBody(platformAdminEntryTargetOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, platformAdminEntryTargetOrg, repo.entries[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, platformAdminEntryTargetOrg, records.created[0].OrganizationID)
	})
}
