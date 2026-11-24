package subscriptions

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appsubscriptions "github.com/guliangiam/guliangiam/internal/app/subscriptions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorSubscriptionSessionOrg = "11111111-1111-1111-1111-111111111111"
	idorSubscriptionForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorSubscriptionID         = "33333333-3333-3333-3333-333333333333"
)

type idorSubscriptionClock struct{}

func (idorSubscriptionClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorSubscriptionUUID struct{}

func (idorSubscriptionUUID) NewString() (string, error) { return "generated-subscription-id", nil }

// idorSubscriptionRepo scopes reads to the owning organization.
type idorSubscriptionRepo struct {
	domain.Repository
	subscriptions []*domain.Subscription
	scopedCalls   int
	scopedOrgs    []string
	updateCalls   int
	deleteCalls   int
}

func (r *idorSubscriptionRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Subscription, error) {
	r.scopedCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	for _, s := range r.subscriptions {
		if s.ID == id && s.OrganizationID == organizationID {
			return s, nil
		}
	}
	return nil, apperrors.NewNotFound("subscription not found")
}

func (r *idorSubscriptionRepo) Create(_ context.Context, s *domain.Subscription) error {
	r.subscriptions = append(r.subscriptions, s)
	return nil
}

func (r *idorSubscriptionRepo) Update(_ context.Context, s *domain.Subscription) error {
	r.updateCalls++
	for i, existing := range r.subscriptions {
		if existing.ID == s.ID {
			r.subscriptions[i] = s
			return nil
		}
	}
	return apperrors.NewNotFound("subscription not found")
}

func (r *idorSubscriptionRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, s := range r.subscriptions {
		if s.ID == id && (organizationID == "" || s.OrganizationID == organizationID) {
			r.subscriptions = append(r.subscriptions[:i], r.subscriptions[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("subscription not found")
}

type idorSubscriptionRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorSubscriptionRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorSubscriptionUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorSubscriptionRecords
}

func (u *idorSubscriptionUOW) Subscriptions() domain.Repository  { return u.repo }
func (u *idorSubscriptionUOW) Records() recordsdomain.Repository { return u.records }
func (u *idorSubscriptionUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *idorSubscriptionUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newIDORSubscriptionHarness(subs ...*domain.Subscription) (*idorSubscriptionRepo, *idorSubscriptionRecords, chi.Router) {
	repo := &idorSubscriptionRepo{subscriptions: subs}
	records := &idorSubscriptionRecords{}
	service := appsubscriptions.Service{
		Clock: idorSubscriptionClock{}, UUID: idorSubscriptionUUID{},
		UOW: &idorSubscriptionUOW{repo: repo, records: records},
	}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return repo, records, router
}

func idorSubscriptionRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorSubscriptionSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorSubscriptionSessionOrg,
	})
	return req.WithContext(ctx)
}

func serveSubscription(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func idorSubscriptionBody(body string) io.Reader {
	if body == "" {
		return nil
	}
	return strings.NewReader(body)
}

func tenantSubscription(organizationID string) *domain.Subscription {
	return &domain.Subscription{
		ID: idorSubscriptionID, OrganizationID: organizationID, Name: "tenant-subscription",
		DisplayName: "Tenant Subscription", Plan: "pro", Pricing: "monthly",
		State: domain.StateActive, Revision: 1,
	}
}

var idorSubscriptionByIDRoutes = []struct {
	name, method, path, body string
}{
	{"get", http.MethodGet, "/api/v1/subscriptions/" + idorSubscriptionID, ""},
	{"patch", http.MethodPatch, "/api/v1/subscriptions/" + idorSubscriptionID, `{"displayName":"hijacked","revision":1}`},
	{"delete", http.MethodDelete, "/api/v1/subscriptions/" + idorSubscriptionID, ""},
}

// A foreign subscription must look missing (404): no write, delete, audit, or row change.
func TestSubscriptionByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range idorSubscriptionByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDORSubscriptionHarness(tenantSubscription(idorSubscriptionForeignOrg))
			recorder := serveSubscription(router, idorSubscriptionRequest(tt.method,
				tt.path+"?organizationId="+idorSubscriptionForeignOrg, idorSubscriptionBody(tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			for _, org := range repo.scopedOrgs {
				assert.Equal(t, idorSubscriptionSessionOrg, org, "forged organizationId must be ignored")
			}
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
			require.Len(t, repo.subscriptions, 1)
		})
	}
}

// An empty trusted tenant is a 400 before the repository is touched.
func TestSubscriptionByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range idorSubscriptionByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDORSubscriptionHarness(tenantSubscription(idorSubscriptionSessionOrg))
			req := httptest.NewRequest(tt.method, tt.path, idorSubscriptionBody(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveSubscription(router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.scopedCalls)
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
		})
	}
}

// The same routes still work for the owning tenant; delete audits its organization.
func TestSubscriptionByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("patch", func(t *testing.T) {
		repo, records, router := newIDORSubscriptionHarness(tenantSubscription(idorSubscriptionSessionOrg))
		recorder := serveSubscription(router, idorSubscriptionRequest(http.MethodPatch,
			"/api/v1/subscriptions/"+idorSubscriptionID, strings.NewReader(`{"displayName":"renamed","revision":1}`)))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSubscriptionSessionOrg, records.created[0].OrganizationID)
	})
	t.Run("delete", func(t *testing.T) {
		repo, records, router := newIDORSubscriptionHarness(tenantSubscription(idorSubscriptionSessionOrg))
		recorder := serveSubscription(router,
			idorSubscriptionRequest(http.MethodDelete, "/api/v1/subscriptions/"+idorSubscriptionID, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSubscriptionSessionOrg, records.created[0].OrganizationID,
			"delete audit must carry the subscription organization")
	})
}

// ─── POST /subscriptions: body organizationId is caller input ────────

const platformAdminSubscriptionTargetOrg = "33333333-3333-3333-3333-333333333333"

func subscriptionCreateBody(organizationID string) string {
	return `{"organizationId":"` + organizationID + `","name":"created-subscription","displayName":"Created Subscription","user":"user-1","plan":"pro","pricing":"monthly"}`
}

func platformAdminSubscriptionRequest(body io.Reader) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), idorSubscriptionSessionOrg, "session-org", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorSubscriptionSessionOrg,
	})
	return req.WithContext(ctx)
}

// TestSubscriptionCreate_BindsOrganizationToTrustedTenant pins the three
// create-route states: no trusted tenant is a 400, a forged body
// organizationId is replaced by the authenticated tenant, and the owning tenant
// still writes normally. A platform administrator keeps the documented
// cross-tenant override.
func TestSubscriptionCreate_BindsOrganizationToTrustedTenant(t *testing.T) {
	t.Run("empty trusted tenant rejected", func(t *testing.T) {
		repo, records, router := newIDORSubscriptionHarness()

		recorder := serveSubscription(router,
			httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(subscriptionCreateBody(idorSubscriptionForeignOrg))))

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "organizationId is required")
		assert.Empty(t, repo.subscriptions)
		assert.Empty(t, records.created)
	})

	t.Run("forged organizationId is ignored", func(t *testing.T) {
		repo, records, router := newIDORSubscriptionHarness()

		recorder := serveSubscription(router,
			idorSubscriptionRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(subscriptionCreateBody(idorSubscriptionForeignOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		require.Len(t, repo.subscriptions, 1)
		assert.Equal(t, idorSubscriptionSessionOrg, repo.subscriptions[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorSubscriptionSessionOrg, records.created[0].OrganizationID)
	})

	t.Run("own tenant succeeds", func(t *testing.T) {
		repo, records, router := newIDORSubscriptionHarness()

		recorder := serveSubscription(router,
			idorSubscriptionRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(subscriptionCreateBody(idorSubscriptionSessionOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, idorSubscriptionSessionOrg, repo.subscriptions[0].OrganizationID)
		require.Len(t, records.created, 1)
	})

	t.Run("platform admin may target another tenant", func(t *testing.T) {
		repo, records, router := newIDORSubscriptionHarness()

		recorder := serveSubscription(router,
			platformAdminSubscriptionRequest(strings.NewReader(subscriptionCreateBody(platformAdminSubscriptionTargetOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, platformAdminSubscriptionTargetOrg, repo.subscriptions[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, platformAdminSubscriptionTargetOrg, records.created[0].OrganizationID)
	})
}
