package captchaproviders

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppkg "github.com/guliangiam/guliangiam/internal/app/captchaproviders"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type byIDClock struct{}

func (byIDClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type byIDUUID struct{}

func (byIDUUID) NewString() (string, error) { return "generated-captcha-provider-id", nil }

// byIDRepo is a tenant-aware in-memory repository. The transaction recorder
// below deliberately avoids gomock so audit writes stay observable.
type byIDRepo struct {
	providers   []*domain.CaptchaProvider
	getOrgs     []string
	updateCalls int
	deleteCalls int
}

func (r *byIDRepo) GetByID(_ context.Context, id, organizationID string) (*domain.CaptchaProvider, error) {
	r.getOrgs = append(r.getOrgs, organizationID)
	for _, provider := range r.providers {
		if provider.ID == id && provider.OrganizationID == organizationID {
			return provider, nil
		}
	}
	return nil, apperrors.NewNotFound("captcha provider not found")
}

func (r *byIDRepo) List(context.Context, domain.ListQuery) ([]*domain.CaptchaProvider, int64, error) {
	return r.providers, int64(len(r.providers)), nil
}

func (r *byIDRepo) Create(_ context.Context, provider *domain.CaptchaProvider) error {
	r.providers = append(r.providers, provider)
	return nil
}

func (r *byIDRepo) Update(_ context.Context, provider *domain.CaptchaProvider) error {
	r.updateCalls++
	for i, existing := range r.providers {
		if existing.ID == provider.ID && existing.OrganizationID == provider.OrganizationID {
			r.providers[i] = provider
			return nil
		}
	}
	return apperrors.NewNotFound("captcha provider not found")
}

func (r *byIDRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, existing := range r.providers {
		if existing.ID == id && existing.OrganizationID == organizationID {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("captcha provider not found")
}

type byIDRecords struct {
	created []*recordsdomain.Record
}

func (r *byIDRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *byIDRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *byIDRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *byIDRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *byIDRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *byIDRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *byIDRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

// byIDUOW exposes only the repositories used by the captcha provider service;
// the embedded UnitOfWork carries the remaining interface surface.
type byIDUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *byIDRecords
}

func (u *byIDUOW) CaptchaProviders() domain.Repository { return u.repo }
func (u *byIDUOW) Records() recordsdomain.Repository   { return u.records }
func (u *byIDUOW) Outbox() domainoutbox.Repository     { return nil }
func (u *byIDUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newByIDService(repo domain.Repository) (apppkg.Service, *byIDRecords) {
	records := &byIDRecords{}
	return apppkg.NewService(byIDClock{}, byIDUUID{}, &byIDUOW{repo: repo, records: records}), records
}

func byIDProvider(id, organizationID string) *domain.CaptchaProvider {
	return &domain.CaptchaProvider{
		ID:             id,
		OrganizationID: organizationID,
		Name:           "tenant-captcha",
		ProviderType:   "turnstile",
		SiteKey:        "site-key",
		SecretKey:      "super-secret-key",
		Config:         map[string]any{},
		IsEnabled:      true,
		Revision:       1,
	}
}

func byIDRouter(service apppkg.Service) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func serveByID(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func byIDBody(raw string) io.Reader {
	if raw == "" {
		return nil
	}
	return strings.NewReader(raw)
}

// TestCaptchaProviderByID_ForeignTenantRejected pins that a by-ID read/write
// against another tenant's provider is indistinguishable from a missing one,
// and that no write or audit event is produced.
func TestCaptchaProviderByID_ForeignTenantRejected(t *testing.T) {
	const foreignProviderID = "foreign-provider"

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/captcha-providers/" + foreignProviderID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/captcha-providers/" + foreignProviderID + "?organizationId=" + forgedOrgRequestID, `{"name":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/captcha-providers/" + foreignProviderID + "?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &byIDRepo{providers: []*domain.CaptchaProvider{byIDProvider(foreignProviderID, forgedOrgRequestID)}}
			service, records := newByIDService(repo)
			router := byIDRouter(service)

			recorder := serveByID(t, router, forgedOrgRequest(tt.method, tt.target, byIDBody(tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign provider must be indistinguishable from missing")
			assert.Zero(t, repo.updateCalls, "foreign tenant must not update the provider")
			assert.Zero(t, repo.deleteCalls, "foreign tenant must not delete the provider")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
			assert.NotContains(t, recorder.Body.String(), "super-secret-key")
		})
	}
}

// TestCaptchaProviderByID_OwnTenantSucceeds pins that the same three operations
// still work for the authenticated tenant and that writes audit the real
// organization.
func TestCaptchaProviderByID_OwnTenantSucceeds(t *testing.T) {
	const ownProviderID = "own-provider"

	t.Run("get", func(t *testing.T) {
		repo := &byIDRepo{providers: []*domain.CaptchaProvider{byIDProvider(ownProviderID, forgedOrgSessionID)}}
		service, _ := newByIDService(repo)

		recorder := serveByID(t, byIDRouter(service), forgedOrgRequest(http.MethodGet,
			"/api/v1/captcha-providers/"+ownProviderID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "secretKeyMasked")
		assert.NotContains(t, recorder.Body.String(), "super-secret-key")
	})

	t.Run("patch", func(t *testing.T) {
		repo := &byIDRepo{providers: []*domain.CaptchaProvider{byIDProvider(ownProviderID, forgedOrgSessionID)}}
		service, records := newByIDService(repo)

		recorder := serveByID(t, byIDRouter(service), forgedOrgRequest(http.MethodPatch,
			"/api/v1/captcha-providers/"+ownProviderID, strings.NewReader(`{"name":"renamed","revision":1}`)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := &byIDRepo{providers: []*domain.CaptchaProvider{byIDProvider(ownProviderID, forgedOrgSessionID)}}
		service, records := newByIDService(repo)

		recorder := serveByID(t, byIDRouter(service), forgedOrgRequest(http.MethodDelete,
			"/api/v1/captcha-providers/"+ownProviderID, nil))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID,
			"delete audit must carry the provider organization")
	})
}

// TestCaptchaProviderByID_WithoutTenantScopeRejected pins the fail-fast: a
// request without any trusted tenant is rejected before touching storage.
func TestCaptchaProviderByID_WithoutTenantScopeRejected(t *testing.T) {
	requests := map[string]*http.Request{
		"get":    httptest.NewRequest(http.MethodGet, "/api/v1/captcha-providers/any-provider", nil),
		"patch":  httptest.NewRequest(http.MethodPatch, "/api/v1/captcha-providers/any-provider", strings.NewReader(`{}`)),
		"delete": httptest.NewRequest(http.MethodDelete, "/api/v1/captcha-providers/any-provider", nil),
	}
	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			repo := &byIDRepo{providers: []*domain.CaptchaProvider{byIDProvider("any-provider", forgedOrgSessionID)}}
			service, _ := newByIDService(repo)
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveByID(t, byIDRouter(service), req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

// TestCaptchaProviderByID_PlatformAdminCrossTenant pins that platform admins
// keep the explicit cross-tenant capability: TrustedOrganizationID honours the
// requested organizationId for super_admin/system scopes.
func TestCaptchaProviderByID_PlatformAdminCrossTenant(t *testing.T) {
	const targetProviderID = "target-provider"
	const targetOrgID = "33333333-3333-3333-3333-333333333333"

	repo := &byIDRepo{providers: []*domain.CaptchaProvider{byIDProvider(targetProviderID, targetOrgID)}}
	service, _ := newByIDService(repo)
	router := byIDRouter(service)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/captcha-providers/"+targetProviderID+"?organizationId="+targetOrgID, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: forgedOrgSessionID,
	})
	req = req.WithContext(ctx)

	recorder := serveByID(t, router, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, repo.getOrgs, 1)
	assert.Equal(t, targetOrgID, repo.getOrgs[0])
}
