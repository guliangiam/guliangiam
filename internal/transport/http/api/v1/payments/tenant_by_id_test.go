package payments

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const platformAdminTargetOrgID = "33333333-3333-3333-3333-333333333333"

// ─── tenant-scoped payment repository ─────────────────────────────────

type tenantPaymentRepo struct {
	payments    map[string]*domain.Payment
	getOrgs     []string
	updateCalls int
	deleteCalls int
}

func newTenantPaymentRepo(items ...*domain.Payment) *tenantPaymentRepo {
	repo := &tenantPaymentRepo{payments: map[string]*domain.Payment{}}
	for _, p := range items {
		repo.payments[p.ID] = p
	}
	return repo
}

func (r *tenantPaymentRepo) GetByID(_ context.Context, id string) (*domain.Payment, error) {
	p, ok := r.payments[id]
	if !ok {
		return nil, apperrors.NewNotFound("payment not found")
	}
	cp := *p
	return &cp, nil
}

func (r *tenantPaymentRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Payment, error) {
	r.getOrgs = append(r.getOrgs, organizationID)
	p, ok := r.payments[id]
	if !ok || p.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("payment not found")
	}
	cp := *p
	return &cp, nil
}

func (r *tenantPaymentRepo) GetByChannelOrderNo(_ context.Context, _, orderNo string) (*domain.Payment, error) {
	for _, p := range r.payments {
		if p.ChannelOrderNo == orderNo {
			cp := *p
			return &cp, nil
		}
	}
	return nil, apperrors.NewNotFound("payment not found")
}

func (r *tenantPaymentRepo) GetByOutTradeNo(_ context.Context, _, outTradeNo string) (*domain.Payment, error) {
	for _, p := range r.payments {
		if p.OutOrderId == outTradeNo {
			cp := *p
			return &cp, nil
		}
	}
	return nil, apperrors.NewNotFound("payment not found")
}

func (r *tenantPaymentRepo) List(_ context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
	var out []*domain.Payment
	for _, p := range r.payments {
		if q.OrganizationID != "" && p.OrganizationID != q.OrganizationID {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out, int64(len(out)), nil
}

func (r *tenantPaymentRepo) Create(_ context.Context, p *domain.Payment) error {
	cp := *p
	r.payments[p.ID] = &cp
	return nil
}

func (r *tenantPaymentRepo) Update(_ context.Context, p *domain.Payment) error {
	r.updateCalls++
	stored, ok := r.payments[p.ID]
	if !ok || stored.OrganizationID != p.OrganizationID {
		return apperrors.NewNotFound("payment not found")
	}
	cp := *p
	r.payments[p.ID] = &cp
	return nil
}

func (r *tenantPaymentRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	p, ok := r.payments[id]
	if !ok || p.OrganizationID != organizationID {
		return apperrors.NewNotFound("payment not found")
	}
	delete(r.payments, id)
	return nil
}

// ─── tenant-scoped history repositories ───────────────────────────────

type tenantAttemptsRepo struct {
	items []*domain.PayAttempt
	orgs  []string
}

func (r *tenantAttemptsRepo) GetByID(_ context.Context, id string) (*domain.PayAttempt, error) {
	for _, a := range r.items {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("attempt not found")
}

func (r *tenantAttemptsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.PayAttempt, error) {
	r.orgs = append(r.orgs, organizationID)
	for _, a := range r.items {
		if a.ID == id && a.OrganizationID == organizationID {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("attempt not found")
}

func (r *tenantAttemptsRepo) ListByPayment(_ context.Context, paymentID string) ([]*domain.PayAttempt, error) {
	var out []*domain.PayAttempt
	for _, a := range r.items {
		if a.PaymentID == paymentID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *tenantAttemptsRepo) ListByPaymentForOrganization(_ context.Context, paymentID, organizationID string) ([]*domain.PayAttempt, error) {
	r.orgs = append(r.orgs, organizationID)
	var out []*domain.PayAttempt
	for _, a := range r.items {
		if a.PaymentID == paymentID && a.OrganizationID == organizationID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (r *tenantAttemptsRepo) ListByRefund(_ context.Context, refundID string) ([]*domain.PayAttempt, error) {
	return nil, nil
}

func (r *tenantAttemptsRepo) Create(_ context.Context, a *domain.PayAttempt) error { return nil }

type tenantRefundsRepo struct {
	items []*domain.Refund
	orgs  []string
}

func (r *tenantRefundsRepo) GetByID(_ context.Context, id string) (*domain.Refund, error) {
	for _, ref := range r.items {
		if ref.ID == id {
			return ref, nil
		}
	}
	return nil, apperrors.NewNotFound("refund not found")
}

func (r *tenantRefundsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Refund, error) {
	r.orgs = append(r.orgs, organizationID)
	for _, ref := range r.items {
		if ref.ID == id && ref.OrganizationID == organizationID {
			return ref, nil
		}
	}
	return nil, apperrors.NewNotFound("refund not found")
}

func (r *tenantRefundsRepo) GetByOutRefundNo(_ context.Context, _, _ string) (*domain.Refund, error) {
	return nil, apperrors.NewNotFound("refund not found")
}

func (r *tenantRefundsRepo) ListByPayment(_ context.Context, paymentID string) ([]*domain.Refund, error) {
	var out []*domain.Refund
	for _, ref := range r.items {
		if ref.PaymentID == paymentID {
			out = append(out, ref)
		}
	}
	return out, nil
}

func (r *tenantRefundsRepo) ListByPaymentForOrganization(_ context.Context, paymentID, organizationID string) ([]*domain.Refund, error) {
	r.orgs = append(r.orgs, organizationID)
	var out []*domain.Refund
	for _, ref := range r.items {
		if ref.PaymentID == paymentID && ref.OrganizationID == organizationID {
			out = append(out, ref)
		}
	}
	return out, nil
}

func (r *tenantRefundsRepo) List(context.Context, string, int, int) ([]*domain.Refund, int64, error) {
	return nil, 0, nil
}

func (r *tenantRefundsRepo) Create(context.Context, *domain.Refund) error { return nil }
func (r *tenantRefundsRepo) Update(context.Context, *domain.Refund) error { return nil }

// ─── tenant-scoped channel config repository ──────────────────────────

type tenantChannelConfigsRepo struct {
	byID    map[string]*domain.PayChannelConfig
	getOrgs []string
	updates int
}

func newTenantChannelConfigsRepo(items ...*domain.PayChannelConfig) *tenantChannelConfigsRepo {
	repo := &tenantChannelConfigsRepo{byID: map[string]*domain.PayChannelConfig{}}
	for _, item := range items {
		repo.byID[item.ID] = item
	}
	return repo
}

func (r *tenantChannelConfigsRepo) GetByID(_ context.Context, id string) (*domain.PayChannelConfig, error) {
	c, ok := r.byID[id]
	if !ok {
		return nil, apperrors.NewNotFound("channel config not found")
	}
	return c, nil
}

func (r *tenantChannelConfigsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.PayChannelConfig, error) {
	r.getOrgs = append(r.getOrgs, organizationID)
	c, ok := r.byID[id]
	if !ok || c.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("channel config not found")
	}
	return c, nil
}

func (r *tenantChannelConfigsRepo) GetActive(context.Context, string, string, string) (*domain.PayChannelConfig, error) {
	return nil, apperrors.NewNotFound("not configured")
}

func (r *tenantChannelConfigsRepo) List(context.Context, string) ([]*domain.PayChannelConfig, error) {
	return nil, nil
}

func (r *tenantChannelConfigsRepo) Create(_ context.Context, c *domain.PayChannelConfig) error {
	r.byID[c.ID] = c
	return nil
}

func (r *tenantChannelConfigsRepo) Update(_ context.Context, c *domain.PayChannelConfig) error {
	r.updates++
	stored, ok := r.byID[c.ID]
	if !ok || stored.OrganizationID != c.OrganizationID {
		return apperrors.NewNotFound("channel config not found")
	}
	r.byID[c.ID] = c
	return nil
}

// ─── tenant-scoped recon service ──────────────────────────────────────

type tenantReconService struct {
	ownerOrg     string
	listOrgs     []string
	resolveOrgs  []string
	runOrgs      []string
	resolveCalls int
	runCalls     int
}

func (s *tenantReconService) tenantOrg(organizationID string) error {
	if organizationID != s.ownerOrg {
		return apperrors.NewNotFound("recon resource not found")
	}
	return nil
}

func (s *tenantReconService) ListBatches(_ context.Context, orgID string, _, _ int) ([]*domain.ReconBatch, int64, error) {
	if orgID != s.ownerOrg {
		return nil, 0, nil
	}
	return []*domain.ReconBatch{{ID: "batch-1", OrganizationID: s.ownerOrg}}, 1, nil
}

func (s *tenantReconService) ListItems(_ context.Context, organizationID, _ string) ([]*domain.ReconItem, error) {
	s.listOrgs = append(s.listOrgs, organizationID)
	if err := s.tenantOrg(organizationID); err != nil {
		return nil, err
	}
	return []*domain.ReconItem{{ID: "item-1", BatchID: "batch-1"}}, nil
}

func (s *tenantReconService) ResolveItem(_ context.Context, organizationID, _, _ string) error {
	s.resolveOrgs = append(s.resolveOrgs, organizationID)
	if err := s.tenantOrg(organizationID); err != nil {
		return err
	}
	s.resolveCalls++
	return nil
}

func (s *tenantReconService) ApproveItem(_ context.Context, organizationID, _, _ string) error {
	if err := s.tenantOrg(organizationID); err != nil {
		return err
	}
	return nil
}

func (s *tenantReconService) RunManual(_ context.Context, organizationID, _ string) error {
	s.runOrgs = append(s.runOrgs, organizationID)
	if err := s.tenantOrg(organizationID); err != nil {
		return err
	}
	s.runCalls++
	return nil
}

func (s *tenantReconService) CreateAndRun(_ context.Context, orgID, _ string, batchDate, _ time.Time) (string, error) {
	if err := s.tenantOrg(orgID); err != nil {
		return "", err
	}
	return "batch-" + batchDate.Format("20060102"), nil
}

// ─── UoW + audit recorder ─────────────────────────────────────────────

type tenantRecordingRecords struct {
	created []*recordsdomain.Record
}

func (r *tenantRecordingRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (r *tenantRecordingRecords) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *tenantRecordingRecords) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}

func (r *tenantRecordingRecords) LoadLastChainEntry(context.Context, string) (int64, []byte, error) {
	return 0, nil, nil
}

func (r *tenantRecordingRecords) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}

func (r *tenantRecordingRecords) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}

func (r *tenantRecordingRecords) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type tenantPaymentsUOW struct {
	appuow.UnitOfWork
	payments domain.Repository
	records  *tenantRecordingRecords
}

func (u *tenantPaymentsUOW) Payments() domain.Repository       { return u.payments }
func (u *tenantPaymentsUOW) Records() recordsdomain.Repository { return u.records }
func (u *tenantPaymentsUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *tenantPaymentsUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newTenantPaymentService(repo *tenantPaymentRepo) (apppayments.Service, *tenantRecordingRecords) {
	records := &tenantRecordingRecords{}
	uow := &tenantPaymentsUOW{payments: repo, records: records}
	return apppayments.Service{Clock: tenantSystemClock{}, UUID: tenantUUIDGenerator{}, UOW: uow}, records
}

type tenantSystemClock struct{}

func (tenantSystemClock) Now() time.Time { return time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) }

type tenantUUIDGenerator struct{}

func (tenantUUIDGenerator) NewString() (string, error) { return "generated-id", nil }

func tenantPaymentRouter(service apppayments.Service) chi.Router {
	router := testRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return router
}

func tenantPayment(id, orgID string) *domain.Payment {
	return &domain.Payment{
		ID:             id,
		OrganizationID: orgID,
		Name:           "tenant payment",
		DisplayName:    "Tenant Payment",
		User:           "user-1",
		Price:          99.99,
		Currency:       "CNY",
		State:          domain.StateCreated,
		Revision:       1,
	}
}

func platformAdminPaymentRequest(method, target string, body io.Reader) *http.Request {
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

func serveTenantPaymentRequest(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestPaymentByID_ForeignTenantRejected pins that every management by-ID route
// treats a foreign payment exactly like a missing one and leaves no write or
// audit row behind.
func TestPaymentByID_ForeignTenantRejected(t *testing.T) {
	const foreignID = "foreign-payment"

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"get", http.MethodGet, "/api/v1/payments/" + foreignID + "?organizationId=" + forgedOrgRequestID, ""},
		{"patch", http.MethodPatch, "/api/v1/payments/" + foreignID + "?organizationId=" + forgedOrgRequestID, `{"displayName":"hijacked"}`},
		{"delete", http.MethodDelete, "/api/v1/payments/" + foreignID + "?organizationId=" + forgedOrgRequestID, ""},
		{"invoice", http.MethodPost, "/api/v1/payments/" + foreignID + "/invoice?organizationId=" + forgedOrgRequestID, `{"invoiceType":"electronic"}`},
		{"result", http.MethodGet, "/api/v1/payments/" + foreignID + "/result?organizationId=" + forgedOrgRequestID, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTenantPaymentRepo(tenantPayment(foreignID, forgedOrgRequestID))
			service, records := newTenantPaymentService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(
				httptest.NewRequest(tt.method, tt.path, body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign payment must look missing")
			assert.Zero(t, repo.updateCalls, "foreign tenant must not update the payment")
			assert.Zero(t, repo.deleteCalls, "foreign tenant must not delete the payment")
			assert.Empty(t, records.created, "foreign tenant must not produce an audit event")
		})
	}
}

// TestPaymentByID_OwnTenantSucceeds pins the happy path and that the DELETE
// audit row carries the payment's real organization.
func TestPaymentByID_OwnTenantSucceeds(t *testing.T) {
	const ownID = "own-payment"

	t.Run("get", func(t *testing.T) {
		repo := newTenantPaymentRepo(tenantPayment(ownID, forgedOrgSessionID))
		service, _ := newTenantPaymentService(repo)
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(
			httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+ownID+"?organizationId="+forgedOrgRequestID, nil)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), ownID)
		require.Len(t, repo.getOrgs, 1)
		assert.Equal(t, forgedOrgSessionID, repo.getOrgs[0], "the authenticated tenant must win")
	})

	t.Run("patch", func(t *testing.T) {
		repo := newTenantPaymentRepo(tenantPayment(ownID, forgedOrgSessionID))
		service, _ := newTenantPaymentService(repo)
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(
			httptest.NewRequest(http.MethodPatch, "/api/v1/payments/"+ownID, strings.NewReader(`{"displayName":"renamed"}`))))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
	})

	t.Run("invoice", func(t *testing.T) {
		repo := newTenantPaymentRepo(tenantPayment(ownID, forgedOrgSessionID))
		service, _ := newTenantPaymentService(repo)
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(
			httptest.NewRequest(http.MethodPost, "/api/v1/payments/"+ownID+"/invoice", strings.NewReader(`{"invoiceType":"electronic"}`))))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
	})

	t.Run("result", func(t *testing.T) {
		repo := newTenantPaymentRepo(tenantPayment(ownID, forgedOrgSessionID))
		service, _ := newTenantPaymentService(repo)
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(
			httptest.NewRequest(http.MethodGet, "/api/v1/payments/"+ownID+"/result", nil)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), ownID)
	})

	t.Run("delete", func(t *testing.T) {
		repo := newTenantPaymentRepo(tenantPayment(ownID, forgedOrgSessionID))
		service, records := newTenantPaymentService(repo)
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(
			httptest.NewRequest(http.MethodDelete, "/api/v1/payments/"+ownID, nil)))

		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID,
			"delete audit must carry the payment organization")
	})
}

// TestPaymentByID_WithoutTenantScopeRejected pins the empty-tenant fail-fast:
// every by-ID route must return 400 before touching the repository.
func TestPaymentByID_WithoutTenantScopeRejected(t *testing.T) {
	tests := map[string]*http.Request{
		"get":     httptest.NewRequest(http.MethodGet, "/api/v1/payments/any-payment", nil),
		"patch":   httptest.NewRequest(http.MethodPatch, "/api/v1/payments/any-payment", strings.NewReader(`{}`)),
		"delete":  httptest.NewRequest(http.MethodDelete, "/api/v1/payments/any-payment", nil),
		"invoice": httptest.NewRequest(http.MethodPost, "/api/v1/payments/any-payment/invoice", strings.NewReader(`{}`)),
		"result":  httptest.NewRequest(http.MethodGet, "/api/v1/payments/any-payment/result", nil),
	}
	for name, req := range tests {
		t.Run(name, func(t *testing.T) {
			repo := newTenantPaymentRepo(tenantPayment("any-payment", forgedOrgSessionID))
			service, records := newTenantPaymentService(repo)
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Empty(t, repo.getOrgs, "missing tenant must not reach the repository")
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
		})
	}
}

// TestPaymentByID_PlatformAdminCrossTenant pins that super_admin keeps explicit
// cross-tenant access through the organizationId query value.
func TestPaymentByID_PlatformAdminCrossTenant(t *testing.T) {
	tests := []struct {
		name   string
		method string
		body   string
		path   string
		status int
	}{
		{"get", http.MethodGet, "", "/api/v1/payments/target-payment?organizationId=" + platformAdminTargetOrgID, http.StatusOK},
		{"patch", http.MethodPatch, `{"displayName":"renamed-by-platform-admin"}`, "/api/v1/payments/target-payment?organizationId=" + platformAdminTargetOrgID, http.StatusOK},
		{"delete", http.MethodDelete, "", "/api/v1/payments/target-payment?organizationId=" + platformAdminTargetOrgID, http.StatusOK},
		{"invoice", http.MethodPost, `{"invoiceType":"electronic"}`, "/api/v1/payments/target-payment/invoice?organizationId=" + platformAdminTargetOrgID, http.StatusOK},
		{"result", http.MethodGet, "", "/api/v1/payments/target-payment/result?organizationId=" + platformAdminTargetOrgID, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newTenantPaymentRepo(tenantPayment("target-payment", platformAdminTargetOrgID))
			service, _ := newTenantPaymentService(repo)

			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service),
				platformAdminPaymentRequest(tt.method, tt.path, body))

			require.Equal(t, tt.status, recorder.Code)
		})
	}
}

// TestPaymentHistoryByID_TenantBound pins the attempts/refunds sub-resources.
func TestPaymentHistoryByID_TenantBound(t *testing.T) {
	attempt := &domain.PayAttempt{ID: "attempt-own", OrganizationID: forgedOrgSessionID, PaymentID: "payment-own", OccurredAt: time.Now()}
	refund := &domain.Refund{
		ID: "refund-own", OrganizationID: forgedOrgSessionID, PaymentID: "payment-own",
		RequestedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}

	t.Run("foreign-and-own", func(t *testing.T) {
		attempts := &tenantAttemptsRepo{items: []*domain.PayAttempt{
			{ID: "attempt-foreign", OrganizationID: forgedOrgRequestID, PaymentID: "payment-foreign", OccurredAt: time.Now()},
			attempt,
		}}
		refunds := &tenantRefundsRepo{items: []*domain.Refund{
			{
				ID: "refund-foreign", OrganizationID: forgedOrgRequestID, PaymentID: "payment-foreign",
				RequestedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
			},
			refund,
		}}
		router := testRouter()
		RegisterAttemptsChiRoutes(router, NewAttemptsHandler(apppayments.NewHistoryService(attempts, refunds)))
		RegisterRefundsChiRoutes(router, NewRefundsHandler(apppayments.NewHistoryService(attempts, refunds)))

		foreign := withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/pay-attempts/attempt-foreign?organizationId="+forgedOrgRequestID, nil))
		assert.Equal(t, http.StatusNotFound, serveTenantPaymentRequest(t, router, foreign).Code)

		own := withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/payments/payment-own/attempts", nil))
		assert.Equal(t, http.StatusOK, serveTenantPaymentRequest(t, router, own).Code)

		foreignRefund := withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/pay-refunds/refund-foreign?organizationId="+forgedOrgRequestID, nil))
		assert.Equal(t, http.StatusNotFound, serveTenantPaymentRequest(t, router, foreignRefund).Code)

		ownRefund := withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/payments/payment-own/refunds", nil))
		assert.Equal(t, http.StatusOK, serveTenantPaymentRequest(t, router, ownRefund).Code)
	})

	t.Run("platform", func(t *testing.T) {
		attempts := &tenantAttemptsRepo{items: []*domain.PayAttempt{
			{ID: "attempt-target", OrganizationID: platformAdminTargetOrgID, PaymentID: "payment-target", OccurredAt: time.Now()},
		}}
		refunds := &tenantRefundsRepo{items: []*domain.Refund{
			{
				ID: "refund-target", OrganizationID: platformAdminTargetOrgID, PaymentID: "payment-target",
				RequestedAt: time.Now(), CreatedAt: time.Now(), UpdatedAt: time.Now(),
			},
		}}
		router := testRouter()
		RegisterAttemptsChiRoutes(router, NewAttemptsHandler(apppayments.NewHistoryService(attempts, refunds)))
		RegisterRefundsChiRoutes(router, NewRefundsHandler(apppayments.NewHistoryService(attempts, refunds)))

		attemptRecorder := serveTenantPaymentRequest(t, router, platformAdminPaymentRequest(http.MethodGet,
			"/api/v1/pay-attempts/attempt-target?organizationId="+platformAdminTargetOrgID, nil))
		assert.Equal(t, http.StatusOK, attemptRecorder.Code)

		refundRecorder := serveTenantPaymentRequest(t, router, platformAdminPaymentRequest(http.MethodGet,
			"/api/v1/pay-refunds/refund-target?organizationId="+platformAdminTargetOrgID, nil))
		assert.Equal(t, http.StatusOK, refundRecorder.Code)
	})

	t.Run("without-tenant", func(t *testing.T) {
		attempts := &tenantAttemptsRepo{items: []*domain.PayAttempt{attempt}}
		refunds := &tenantRefundsRepo{items: []*domain.Refund{refund}}
		router := testRouter()
		RegisterAttemptsChiRoutes(router, NewAttemptsHandler(apppayments.NewHistoryService(attempts, refunds)))
		RegisterRefundsChiRoutes(router, NewRefundsHandler(apppayments.NewHistoryService(attempts, refunds)))

		for _, target := range []string{
			"/api/v1/payments/payment-1/attempts",
			"/api/v1/pay-attempts/attempt-1",
			"/api/v1/payments/payment-1/refunds",
			"/api/v1/pay-refunds/refund-1",
		} {
			recorder := serveTenantPaymentRequest(t, router, httptest.NewRequest(http.MethodGet, target, nil))
			assert.Equal(t, http.StatusBadRequest, recorder.Code, target)
			assert.Empty(t, attempts.orgs, target)
			assert.Empty(t, refunds.orgs, target)
		}
	})
}

// TestChannelConfigByID_TenantBound pins GET and the :transition mutation.
func TestChannelConfigByID_TenantBound(t *testing.T) {
	config := func(orgID string) *domain.PayChannelConfig {
		return &domain.PayChannelConfig{
			ID: "cfg-1", OrganizationID: orgID, Channel: "alipay", Environment: "production",
			Status: domain.ChannelConfigStatusDraft, VersionNo: 1, Revision: 1,
			CreatedAt: time.Now(), UpdatedAt: time.Now(),
		}
	}

	t.Run("foreign-rejected", func(t *testing.T) {
		repo := newTenantChannelConfigsRepo(config(forgedOrgRequestID))
		service := apppayments.NewChannelConfigService(repo, tenantSystemClock{}, nil, platformuuid.V7Generator{})
		router := testRouter()
		RegisterChannelConfigChiRoutes(router, NewChannelConfigHandler(service))

		get := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/pay-channel-configs/cfg-1?organizationId="+forgedOrgRequestID, nil)))
		assert.Equal(t, http.StatusNotFound, get.Code)
		assert.Zero(t, repo.updates)

		transition := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(http.MethodPost,
			"/api/v1/pay-channel-configs/cfg-1:transition?organizationId="+forgedOrgRequestID,
			strings.NewReader(`{"status":"testing"}`))))
		assert.Equal(t, http.StatusNotFound, transition.Code)
		assert.Zero(t, repo.updates, "foreign tenant must not transition the config")
	})

	t.Run("own", func(t *testing.T) {
		repo := newTenantChannelConfigsRepo(config(forgedOrgSessionID))
		service := apppayments.NewChannelConfigService(repo, tenantSystemClock{}, nil, platformuuid.V7Generator{})
		router := testRouter()
		RegisterChannelConfigChiRoutes(router, NewChannelConfigHandler(service))

		own := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/pay-channel-configs/cfg-1", nil)))
		assert.Equal(t, http.StatusOK, own.Code)
	})

	t.Run("platform", func(t *testing.T) {
		repo := newTenantChannelConfigsRepo(config(platformAdminTargetOrgID))
		service := apppayments.NewChannelConfigService(repo, tenantSystemClock{}, nil, platformuuid.V7Generator{})
		router := testRouter()
		RegisterChannelConfigChiRoutes(router, NewChannelConfigHandler(service))

		platform := serveTenantPaymentRequest(t, router, platformAdminPaymentRequest(http.MethodGet,
			"/api/v1/pay-channel-configs/cfg-1?organizationId="+platformAdminTargetOrgID, nil))
		assert.Equal(t, http.StatusOK, platform.Code)

		transition := serveTenantPaymentRequest(t, router, platformAdminPaymentRequest(http.MethodPost,
			"/api/v1/pay-channel-configs/cfg-1:transition?organizationId="+platformAdminTargetOrgID,
			strings.NewReader(`{"status":"testing"}`)))
		assert.Equal(t, http.StatusOK, transition.Code)
		assert.Equal(t, 1, repo.updates)
	})

	t.Run("without-tenant", func(t *testing.T) {
		repo := newTenantChannelConfigsRepo(config(forgedOrgSessionID))
		service := apppayments.NewChannelConfigService(repo, tenantSystemClock{}, nil, platformuuid.V7Generator{})
		router := testRouter()
		RegisterChannelConfigChiRoutes(router, NewChannelConfigHandler(service))

		requests := []*http.Request{
			httptest.NewRequest(http.MethodGet, "/api/v1/pay-channel-configs/cfg-1", nil),
			httptest.NewRequest(http.MethodPost,
				"/api/v1/pay-channel-configs/cfg-1:transition", strings.NewReader(`{"status":"testing"}`)),
		}
		for _, req := range requests {
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveTenantPaymentRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, req.URL.Path)
			assert.Empty(t, repo.getOrgs)
			assert.Zero(t, repo.updates)
		}
	})
}

// TestReconByID_TenantBound pins batch items, item resolution and manual runs.
func TestReconByID_TenantBound(t *testing.T) {
	t.Run("foreign-and-own", func(t *testing.T) {
		foreignSvc := &tenantReconService{ownerOrg: forgedOrgRequestID}
		foreignRouter := testRouter()
		RegisterReconChiRoutes(foreignRouter, NewReconHandler(foreignSvc))

		foreignItems := serveTenantPaymentRequest(t, foreignRouter, withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/pay-recon-batches/batch-1/items?organizationId="+forgedOrgRequestID, nil)))
		assert.Equal(t, http.StatusNotFound, foreignItems.Code)

		foreignResolve := serveTenantPaymentRequest(t, foreignRouter, withTrustedTenant(httptest.NewRequest(http.MethodPost,
			"/api/v1/pay-recon-items/item-1:resolve",
			strings.NewReader(`{"operator":"ops"}`))))
		assert.Equal(t, http.StatusNotFound, foreignResolve.Code)
		assert.Zero(t, foreignSvc.resolveCalls)

		foreignRun := serveTenantPaymentRequest(t, foreignRouter, withTrustedTenant(httptest.NewRequest(http.MethodPost,
			"/api/v1/pay-recon:run",
			strings.NewReader(`{"batchId":"batch-1","organizationId":"`+forgedOrgRequestID+`"}`))))
		assert.Equal(t, http.StatusNotFound, foreignRun.Code)
		assert.Zero(t, foreignSvc.runCalls)

		ownSvc := &tenantReconService{ownerOrg: forgedOrgSessionID}
		ownRouter := testRouter()
		RegisterReconChiRoutes(ownRouter, NewReconHandler(ownSvc))

		ownItems := serveTenantPaymentRequest(t, ownRouter, withTrustedTenant(httptest.NewRequest(http.MethodGet,
			"/api/v1/pay-recon-batches/batch-1/items", nil)))
		assert.Equal(t, http.StatusOK, ownItems.Code)
	})

	t.Run("own-and-platform", func(t *testing.T) {
		svc := &tenantReconService{ownerOrg: forgedOrgSessionID}
		router := testRouter()
		RegisterReconChiRoutes(router, NewReconHandler(svc))

		resolve := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(http.MethodPost,
			"/api/v1/pay-recon-items/item-1:resolve", strings.NewReader(`{"operator":"ops"}`))))
		assert.Equal(t, http.StatusOK, resolve.Code)
		assert.Equal(t, 1, svc.resolveCalls)

		run := serveTenantPaymentRequest(t, router, withTrustedTenant(httptest.NewRequest(http.MethodPost,
			"/api/v1/pay-recon:run", strings.NewReader(`{"batchId":"batch-1"}`))))
		assert.Equal(t, http.StatusAccepted, run.Code)
		assert.Equal(t, 1, svc.runCalls)

		platformSvc := &tenantReconService{ownerOrg: platformAdminTargetOrgID}
		platformRouter := testRouter()
		RegisterReconChiRoutes(platformRouter, NewReconHandler(platformSvc))
		platform := serveTenantPaymentRequest(t, platformRouter, platformAdminPaymentRequest(http.MethodPost,
			"/api/v1/pay-recon:run",
			strings.NewReader(`{"batchId":"batch-1","organizationId":"`+platformAdminTargetOrgID+`"}`)))
		assert.Equal(t, http.StatusAccepted, platform.Code)
		assert.Equal(t, 1, platformSvc.runCalls)
	})

	t.Run("without-tenant", func(t *testing.T) {
		svc := &tenantReconService{ownerOrg: forgedOrgSessionID}
		router := testRouter()
		RegisterReconChiRoutes(router, NewReconHandler(svc))

		requests := []*http.Request{
			httptest.NewRequest(http.MethodGet, "/api/v1/pay-recon-batches/batch-1/items", nil),
			httptest.NewRequest(http.MethodPost,
				"/api/v1/pay-recon-items/item-1:resolve", strings.NewReader(`{"operator":"ops"}`)),
			httptest.NewRequest(http.MethodPost,
				"/api/v1/pay-recon:run", strings.NewReader(`{"batchId":"batch-1"}`)),
		}
		for _, req := range requests {
			if req.Body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveTenantPaymentRequest(t, router, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, req.URL.Path)
			assert.Empty(t, svc.listOrgs)
			assert.Empty(t, svc.resolveOrgs)
			assert.Empty(t, svc.runOrgs)
		}
	})
}

// TestPaymentPublicAndCallbackRoutesUnchanged proves the callback and catalog
// endpoints still work without a tenant scope.
func TestPaymentPublicAndCallbackRoutesUnchanged(t *testing.T) {
	t.Run("notify-callback", func(t *testing.T) {
		router := testRouter()
		RegisterNotifyChiRoutes(router, NotifyHandler{})
		recorder := serveTenantPaymentRequest(t, router, httptest.NewRequest(http.MethodPost,
			"/api/v1/payments/notify/alipay", strings.NewReader("")))
		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code,
			"an unconfigured channel must fail closed without demanding a tenant")
	})

	t.Run("channel-catalog", func(t *testing.T) {
		router := testRouter()
		RegisterChannelCatalogChiRoutes(router, NewChannelCatalogHandler(apppayments.NewChannelCatalogService(tenantCatalogProvider{})))
		recorder := serveTenantPaymentRequest(t, router, httptest.NewRequest(http.MethodGet, "/api/v1/pay-channels", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "alipay")
	})
}

type tenantCatalogProvider struct{}

func (tenantCatalogProvider) ChannelMetadata() []domain.ChannelMetadata {
	return []domain.ChannelMetadata{{Channel: "alipay", SupportsCharge: true}}
}

// ─── POST /payments: body organizationId is caller input ─────────────

func paymentCreateBody(organizationID string) string {
	return `{"organizationId":"` + organizationID + `","name":"created-payment","displayName":"Created Payment","user":"user-1","price":9.99,"currency":"CNY"}`
}

// TestPaymentCreate_BindsOrganizationToTrustedTenant pins the three create-route
// states: no trusted tenant is a 400, a forged body organizationId is replaced
// by the authenticated tenant, and the owning tenant still writes normally. A
// platform administrator keeps the documented cross-tenant override.
func TestPaymentCreate_BindsOrganizationToTrustedTenant(t *testing.T) {
	t.Run("empty trusted tenant rejected", func(t *testing.T) {
		repo := newTenantPaymentRepo()
		service, records := newTenantPaymentService(repo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(paymentCreateBody(forgedOrgRequestID)))
		req.Header.Set("Content-Type", "application/json")
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), req)

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "organizationId is required")
		assert.Empty(t, repo.payments)
		assert.Empty(t, records.created)
	})

	t.Run("forged organizationId is ignored", func(t *testing.T) {
		repo := newTenantPaymentRepo()
		service, records := newTenantPaymentService(repo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(paymentCreateBody(forgedOrgRequestID)))
		req.Header.Set("Content-Type", "application/json")
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(req))

		require.Equal(t, http.StatusCreated, recorder.Code)
		require.Len(t, repo.payments, 1)
		assert.Equal(t, forgedOrgSessionID, repo.payments["generated-id"].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, forgedOrgSessionID, records.created[0].OrganizationID)
	})

	t.Run("own tenant succeeds", func(t *testing.T) {
		repo := newTenantPaymentRepo()
		service, records := newTenantPaymentService(repo)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(paymentCreateBody(forgedOrgSessionID)))
		req.Header.Set("Content-Type", "application/json")
		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service), withTrustedTenant(req))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, forgedOrgSessionID, repo.payments["generated-id"].OrganizationID)
		require.Len(t, records.created, 1)
	})

	t.Run("platform admin may target another tenant", func(t *testing.T) {
		repo := newTenantPaymentRepo()
		service, records := newTenantPaymentService(repo)

		recorder := serveTenantPaymentRequest(t, tenantPaymentRouter(service),
			platformAdminPaymentRequest(http.MethodPost, "/api/v1/payments", strings.NewReader(paymentCreateBody(platformAdminTargetOrgID))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, platformAdminTargetOrgID, repo.payments["generated-id"].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, platformAdminTargetOrgID, records.created[0].OrganizationID)
	})
}
