package transactions

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
	apptransactions "github.com/guliangiam/guliangiam/internal/app/transactions"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	domain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorTransactionSessionOrg = "11111111-1111-1111-1111-111111111111"
	idorTransactionForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorTransactionID         = "33333333-3333-3333-3333-333333333333"
)

type idorTransactionClock struct{}

func (idorTransactionClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorTransactionUUID struct{}

func (idorTransactionUUID) NewString() (string, error) { return "generated-transaction-id", nil }

// idorTransactionRepo scopes reads to the owning organization.
type idorTransactionRepo struct {
	domain.Repository
	transactions []*domain.Transaction
	scopedCalls  int
	scopedOrgs   []string
	updateCalls  int
	deleteCalls  int
}

func (r *idorTransactionRepo) GetByID(_ context.Context, id string) (*domain.Transaction, error) {
	return r.find(id, "")
}

func (r *idorTransactionRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Transaction, error) {
	r.scopedCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	return r.find(id, organizationID)
}

func (r *idorTransactionRepo) find(id, organizationID string) (*domain.Transaction, error) {
	for _, t := range r.transactions {
		if t.ID == id && (organizationID == "" || t.OrganizationID == organizationID) {
			return t, nil
		}
	}
	return nil, apperrors.NewNotFound("transaction not found")
}

func (r *idorTransactionRepo) Create(_ context.Context, t *domain.Transaction) error {
	r.transactions = append(r.transactions, t)
	return nil
}

func (r *idorTransactionRepo) Update(_ context.Context, t *domain.Transaction) error {
	r.updateCalls++
	for i, existing := range r.transactions {
		if existing.ID == t.ID {
			r.transactions[i] = t
			return nil
		}
	}
	return apperrors.NewNotFound("transaction not found")
}

func (r *idorTransactionRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, t := range r.transactions {
		if t.ID == id && (organizationID == "" || t.OrganizationID == organizationID) {
			r.transactions = append(r.transactions[:i], r.transactions[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("transaction not found")
}

type idorTransactionRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorTransactionRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorTransactionUOW struct {
	appuow.UnitOfWork
	repo    domain.Repository
	records *idorTransactionRecords
}

func (u *idorTransactionUOW) Transactions() domain.Repository   { return u.repo }
func (u *idorTransactionUOW) Records() recordsdomain.Repository { return u.records }
func (u *idorTransactionUOW) Outbox() domainoutbox.Repository   { return nil }
func (u *idorTransactionUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

func newIDORTransactionHarness(txs ...*domain.Transaction) (*idorTransactionRepo, *idorTransactionRecords, chi.Router) {
	repo := &idorTransactionRepo{transactions: txs}
	records := &idorTransactionRecords{}
	service := apptransactions.Service{
		Clock: idorTransactionClock{}, UUID: idorTransactionUUID{},
		UOW: &idorTransactionUOW{repo: repo, records: records},
	}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return repo, records, router
}

func idorTransactionRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorTransactionSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorTransactionSessionOrg,
	})
	return req.WithContext(ctx)
}

func serveTransaction(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func idorTransactionBody(body string) io.Reader {
	if body == "" {
		return nil
	}
	return strings.NewReader(body)
}

func tenantTransaction(organizationID string) *domain.Transaction {
	return &domain.Transaction{
		ID: idorTransactionID, OrganizationID: organizationID,
		TransactionType: "payment", Amount: 9.99, Currency: "CNY",
		Status: domain.StatusPending, Metadata: json.RawMessage(`{}`), Revision: 1,
	}
}

var idorTransactionByIDRoutes = []struct {
	name, method, path, body string
}{
	{"get", http.MethodGet, "/api/v1/transactions/" + idorTransactionID, ""},
	{"patch", http.MethodPatch, "/api/v1/transactions/" + idorTransactionID, `{"status":"completed","revision":1}`},
	{"delete", http.MethodDelete, "/api/v1/transactions/" + idorTransactionID, ""},
}

// A foreign transaction must look missing (404): no write, delete, audit, or row change.
func TestTransactionByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range idorTransactionByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDORTransactionHarness(tenantTransaction(idorTransactionForeignOrg))
			recorder := serveTransaction(router, idorTransactionRequest(tt.method,
				tt.path+"?organizationId="+idorTransactionForeignOrg, idorTransactionBody(tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			for _, org := range repo.scopedOrgs {
				assert.Equal(t, idorTransactionSessionOrg, org, "forged organizationId must be ignored")
			}
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
			require.Len(t, repo.transactions, 1)
		})
	}
}

// An empty trusted tenant is a 400 before the repository is touched.
func TestTransactionByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range idorTransactionByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDORTransactionHarness(tenantTransaction(idorTransactionSessionOrg))
			req := httptest.NewRequest(tt.method, tt.path, idorTransactionBody(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveTransaction(router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.scopedCalls)
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
		})
	}
}

// The same routes still work for the owning tenant; delete audits its organization.
func TestTransactionByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("patch", func(t *testing.T) {
		repo, records, router := newIDORTransactionHarness(tenantTransaction(idorTransactionSessionOrg))
		recorder := serveTransaction(router, idorTransactionRequest(http.MethodPatch,
			"/api/v1/transactions/"+idorTransactionID, strings.NewReader(`{"status":"completed","revision":1}`)))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorTransactionSessionOrg, records.created[0].OrganizationID)
	})
	t.Run("delete", func(t *testing.T) {
		repo, records, router := newIDORTransactionHarness(tenantTransaction(idorTransactionSessionOrg))
		recorder := serveTransaction(router,
			idorTransactionRequest(http.MethodDelete, "/api/v1/transactions/"+idorTransactionID, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorTransactionSessionOrg, records.created[0].OrganizationID,
			"delete audit must carry the transaction organization")
	})
}

// ─── POST /transactions: body organizationId is caller input ─────────

const platformAdminTransactionTargetOrg = "33333333-3333-3333-3333-333333333333"

func transactionCreateBody(organizationID string) string {
	return `{"organizationId":"` + organizationID + `","transactionType":"payment","amount":9.99,"currency":"CNY","userId":"user-1"}`
}

func platformAdminTransactionRequest(body io.Reader) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions", body)
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), idorTransactionSessionOrg, "session-org", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorTransactionSessionOrg,
	})
	return req.WithContext(ctx)
}

// TestTransactionCreate_BindsOrganizationToTrustedTenant pins the three
// create-route states: no trusted tenant is a 400, a forged body
// organizationId is replaced by the authenticated tenant, and the owning tenant
// still writes normally. A platform administrator keeps the documented
// cross-tenant override.
func TestTransactionCreate_BindsOrganizationToTrustedTenant(t *testing.T) {
	t.Run("empty trusted tenant rejected", func(t *testing.T) {
		repo, records, router := newIDORTransactionHarness()

		recorder := serveTransaction(router,
			httptest.NewRequest(http.MethodPost, "/api/v1/transactions", strings.NewReader(transactionCreateBody(idorTransactionForeignOrg))))

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "organizationId is required")
		assert.Empty(t, repo.transactions)
		assert.Empty(t, records.created)
	})

	t.Run("forged organizationId is ignored", func(t *testing.T) {
		repo, records, router := newIDORTransactionHarness()

		recorder := serveTransaction(router,
			idorTransactionRequest(http.MethodPost, "/api/v1/transactions", strings.NewReader(transactionCreateBody(idorTransactionForeignOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		require.Len(t, repo.transactions, 1)
		assert.Equal(t, idorTransactionSessionOrg, repo.transactions[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorTransactionSessionOrg, records.created[0].OrganizationID)
	})

	t.Run("own tenant succeeds", func(t *testing.T) {
		repo, records, router := newIDORTransactionHarness()

		recorder := serveTransaction(router,
			idorTransactionRequest(http.MethodPost, "/api/v1/transactions", strings.NewReader(transactionCreateBody(idorTransactionSessionOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, idorTransactionSessionOrg, repo.transactions[0].OrganizationID)
		require.Len(t, records.created, 1)
	})

	t.Run("platform admin may target another tenant", func(t *testing.T) {
		repo, records, router := newIDORTransactionHarness()

		recorder := serveTransaction(router,
			platformAdminTransactionRequest(strings.NewReader(transactionCreateBody(platformAdminTransactionTargetOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, platformAdminTransactionTargetOrg, repo.transactions[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, platformAdminTransactionTargetOrg, records.created[0].OrganizationID)
	})
}
