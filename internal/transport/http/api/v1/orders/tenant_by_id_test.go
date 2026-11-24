package orders

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/orders"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorOrderSessionOrg = "11111111-1111-1111-1111-111111111111"
	idorOrderForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorOrderID         = "33333333-3333-3333-3333-333333333333"
)

type idorOrderClock struct{}

func (idorOrderClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type idorOrderUUID struct{}

func (idorOrderUUID) NewString() (string, error) { return "generated-order-id", nil }

// idorOrderRepo scopes reads to the owning organization.
type idorOrderRepo struct {
	domain.Repository
	orders      []*domain.Order
	scopedCalls int
	scopedOrgs  []string
	updateCalls int
	deleteCalls int
}

func (r *idorOrderRepo) GetByID(_ context.Context, id string) (*domain.Order, error) {
	return r.find(id, "")
}

func (r *idorOrderRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Order, error) {
	r.scopedCalls++
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	return r.find(id, organizationID)
}

func (r *idorOrderRepo) find(id, organizationID string) (*domain.Order, error) {
	for _, o := range r.orders {
		if o.ID == id && (organizationID == "" || o.OrganizationID == organizationID) {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("order not found")
}

func (r *idorOrderRepo) Update(_ context.Context, o *domain.Order) error {
	r.updateCalls++
	for i, existing := range r.orders {
		if existing.ID == o.ID {
			r.orders[i] = o
			return nil
		}
	}
	return apperrors.NewNotFound("order not found")
}

func (r *idorOrderRepo) Create(_ context.Context, o *domain.Order) error {
	r.orders = append(r.orders, o)
	return nil
}

func (r *idorOrderRepo) Delete(_ context.Context, id, organizationID string) error {
	r.deleteCalls++
	for i, o := range r.orders {
		if o.ID == id && (organizationID == "" || o.OrganizationID == organizationID) {
			r.orders = append(r.orders[:i], r.orders[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("order not found")
}

type idorOrderRecords struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *idorOrderRecords) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

type idorOrderUOW struct {
	appuow.UnitOfWork
	repo     domain.Repository
	records  *idorOrderRecords
	products productdomain.Repository
}

func (u *idorOrderUOW) Orders() domain.Repository          { return u.repo }
func (u *idorOrderUOW) Records() recordsdomain.Repository  { return u.records }
func (u *idorOrderUOW) Outbox() domainoutbox.Repository    { return nil }
func (u *idorOrderUOW) Products() productdomain.Repository { return u.products }
func (u *idorOrderUOW) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// idorOrderProductRepo is the catalog the checkout route prices a basket
// against. Every scoped read is recorded so a test can prove which tenant the
// lookup used, independent of what the request body claimed.
type idorOrderProductRepo struct {
	productdomain.Repository
	products   []*productdomain.Product
	scopedOrgs []string
}

func (r *idorOrderProductRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*productdomain.Product, error) {
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	for _, p := range r.products {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("product not found")
}

// IncrementSold mirrors the infra atomic stock guard (BE-C-11) so the place
// route exercises the real reservation semantics against the fake catalog.
func (r *idorOrderProductRepo) IncrementSold(ctx context.Context, id, organizationID string, quantity int, now time.Time) (*productdomain.Product, error) {
	p, err := r.GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if p.State != productdomain.StateActive {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "product is not available for purchase", map[string]any{"field": "state", "value": string(p.State)})
	}
	if quantity <= 0 {
		return nil, apperrors.NewInvalidArgument("quantity must be positive")
	}
	if p.Quantity > 0 && p.Sold+quantity > p.Quantity {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "insufficient product quantity", map[string]any{"field": "quantity", "value": quantity})
	}
	p.Sold += quantity
	p.UpdatedAt = now
	p.Revision++
	return p, nil
}

// ReleaseSold returns reserved stock, floored at zero (BE-C-11).
func (r *idorOrderProductRepo) ReleaseSold(ctx context.Context, id, organizationID string, quantity int, now time.Time) (*productdomain.Product, error) {
	p, err := r.GetByIDForOrganization(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	if quantity <= 0 {
		return nil, apperrors.NewInvalidArgument("quantity must be positive")
	}
	p.Sold -= quantity
	if p.Sold < 0 {
		p.Sold = 0
	}
	p.UpdatedAt = now
	p.Revision++
	return p, nil
}

func newIDOROrderHarness(orders ...*domain.Order) (*idorOrderRepo, *idorOrderRecords, chi.Router) {
	repo := &idorOrderRepo{orders: orders}
	records := &idorOrderRecords{}
	service := apporders.Service{
		Clock: idorOrderClock{}, UUID: idorOrderUUID{},
		UOW: &idorOrderUOW{repo: repo, records: records},
	}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return repo, records, router
}

func newIDOROrderCreateHarness(products ...*productdomain.Product) (*idorOrderRepo, *idorOrderRecords, *idorOrderProductRepo, chi.Router) {
	repo := &idorOrderRepo{}
	records := &idorOrderRecords{}
	productRepo := &idorOrderProductRepo{products: products}
	service := apporders.Service{
		Clock: idorOrderClock{}, UUID: idorOrderUUID{},
		UOW: &idorOrderUOW{repo: repo, records: records, products: productRepo},
	}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return repo, records, productRepo, router
}

func orderProduct(id, organizationID string) *productdomain.Product {
	return &productdomain.Product{
		ID: id, OrganizationID: organizationID, Name: id, DisplayName: id,
		Price: 9.99, Currency: "CNY", State: productdomain.StateActive,
	}
}

func orderCreateBody(organizationID string, products ...string) string {
	body := `{"organizationId":"` + organizationID + `","name":"created-order","displayName":"Created Order","user":"user-1","price":9.99,"currency":"CNY"`
	if len(products) > 0 {
		body += `,"products":["` + strings.Join(products, `","`) + `"]`
	}
	return body + "}"
}

func platformAdminOrderRequest(target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, body)
	req.Header.Set("Content-Type", "application/json")
	ctx := tenantctx.WithTenantScope(req.Context(), idorOrderSessionOrg, "session-org", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorOrderSessionOrg,
	})
	return req.WithContext(ctx)
}

func idorOrderRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorOrderSessionOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: idorOrderSessionOrg,
	})
	return req.WithContext(ctx)
}

func serveOrder(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func idorOrderBody(body string) io.Reader {
	if body == "" {
		return nil
	}
	return strings.NewReader(body)
}

func tenantOrder(organizationID string) *domain.Order {
	return &domain.Order{
		ID: idorOrderID, OrganizationID: organizationID, Name: "tenant-order",
		DisplayName: "Tenant Order", Price: 9.99, Currency: "CNY",
		State: domain.StatePaymentPending, PaymentURL: "https://pay.example.com/order",
		Revision: 1,
	}
}

var idorOrderByIDRoutes = []struct {
	name, method, path, body string
}{
	{"get", http.MethodGet, "/api/v1/orders/" + idorOrderID, ""},
	{"patch", http.MethodPatch, "/api/v1/orders/" + idorOrderID, `{"displayName":"hijacked","revision":1}`},
	{"delete", http.MethodDelete, "/api/v1/orders/" + idorOrderID, ""},
	{"pay-url", http.MethodGet, "/api/v1/orders/" + idorOrderID + "/pay-url", ""},
}

// A foreign order must look missing (404): no write, delete, audit, or row change.
func TestOrderByID_ForeignTenantRejected(t *testing.T) {
	for _, tt := range idorOrderByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDOROrderHarness(tenantOrder(idorOrderForeignOrg))
			recorder := serveOrder(router, idorOrderRequest(tt.method,
				tt.path+"?organizationId="+idorOrderForeignOrg, idorOrderBody(tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.NotContains(t, recorder.Body.String(), "pay.example.com")
			for _, org := range repo.scopedOrgs {
				assert.Equal(t, idorOrderSessionOrg, org, "forged organizationId must be ignored")
			}
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
			require.Len(t, repo.orders, 1)
		})
	}
}

// An empty trusted tenant is a 400 before the repository is touched.
func TestOrderByID_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range idorOrderByIDRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDOROrderHarness(tenantOrder(idorOrderSessionOrg))
			req := httptest.NewRequest(tt.method, tt.path, idorOrderBody(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveOrder(router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Zero(t, repo.scopedCalls)
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
		})
	}
}

// The same routes still work for the owning tenant; delete audits its organization.
func TestOrderByID_OwnTenantSucceeds(t *testing.T) {
	t.Run("pay-url", func(t *testing.T) {
		_, _, router := newIDOROrderHarness(tenantOrder(idorOrderSessionOrg))
		recorder := serveOrder(router, idorOrderRequest(http.MethodGet, "/api/v1/orders/"+idorOrderID+"/pay-url", nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "https://pay.example.com/order")
	})
	t.Run("patch", func(t *testing.T) {
		repo, records, router := newIDOROrderHarness(tenantOrder(idorOrderSessionOrg))
		recorder := serveOrder(router, idorOrderRequest(http.MethodPatch, "/api/v1/orders/"+idorOrderID,
			strings.NewReader(`{"displayName":"renamed","revision":1}`)))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.updateCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorOrderSessionOrg, records.created[0].OrganizationID)
	})
	t.Run("delete", func(t *testing.T) {
		repo, records, router := newIDOROrderHarness(tenantOrder(idorOrderSessionOrg))
		recorder := serveOrder(router, idorOrderRequest(http.MethodDelete, "/api/v1/orders/"+idorOrderID, nil))
		require.Equal(t, http.StatusOK, recorder.Code)
		assert.Equal(t, 1, repo.deleteCalls)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorOrderSessionOrg, records.created[0].OrganizationID,
			"delete audit must carry the order organization")
	})
}

// ─── POST /orders: body organizationId is caller input ───────────────

// TestOrderCreate_BindsOrganizationToTrustedTenant pins the three create-route
// states: no trusted tenant is a 400, a forged body organizationId is replaced
// by the authenticated tenant, and the owning tenant still writes normally. A
// platform administrator keeps the documented cross-tenant override.
func TestOrderCreate_BindsOrganizationToTrustedTenant(t *testing.T) {
	t.Run("empty trusted tenant rejected", func(t *testing.T) {
		repo, records, _, router := newIDOROrderCreateHarness()

		recorder := serveOrder(router,
			httptest.NewRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(orderCreateBody(idorOrderForeignOrg))))

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "organizationId is required")
		assert.Empty(t, repo.orders)
		assert.Empty(t, records.created)
	})

	t.Run("forged organizationId is ignored", func(t *testing.T) {
		repo, records, _, router := newIDOROrderCreateHarness()

		recorder := serveOrder(router,
			idorOrderRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(orderCreateBody(idorOrderForeignOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		require.Len(t, repo.orders, 1)
		assert.Equal(t, idorOrderSessionOrg, repo.orders[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorOrderSessionOrg, records.created[0].OrganizationID)
	})

	t.Run("own tenant succeeds", func(t *testing.T) {
		repo, records, _, router := newIDOROrderCreateHarness()

		recorder := serveOrder(router,
			idorOrderRequest(http.MethodPost, "/api/v1/orders", strings.NewReader(orderCreateBody(idorOrderSessionOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, idorOrderSessionOrg, repo.orders[0].OrganizationID)
		require.Len(t, records.created, 1)
	})

	t.Run("platform admin may target another tenant", func(t *testing.T) {
		repo, records, _, router := newIDOROrderCreateHarness()

		recorder := serveOrder(router,
			platformAdminOrderRequest("/api/v1/orders", strings.NewReader(orderCreateBody(idorOrderForeignOrg))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, idorOrderForeignOrg, repo.orders[0].OrganizationID)
		require.Len(t, records.created, 1)
		assert.Equal(t, idorOrderForeignOrg, records.created[0].OrganizationID)
	})
}

// ─── POST /orders/place: body organizationId and catalog scope ───────

// TestOrderPlace_BindsOrganizationAndCatalogToTrustedTenant pins the checkout
// route: the body organizationId cannot move the order, and the catalog is read
// with the trusted tenant only, so a foreign product id is not found instead of
// being snapshotted onto a foreign order.
func TestOrderPlace_BindsOrganizationAndCatalogToTrustedTenant(t *testing.T) {
	const productID = "plan-1"

	t.Run("empty trusted tenant rejected", func(t *testing.T) {
		repo, records, products, router := newIDOROrderCreateHarness(orderProduct(productID, idorOrderSessionOrg))

		recorder := serveOrder(router,
			httptest.NewRequest(http.MethodPost, "/api/v1/orders/place", strings.NewReader(orderCreateBody(idorOrderForeignOrg, productID))))

		require.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "organizationId is required")
		assert.Empty(t, repo.orders)
		assert.Empty(t, records.created)
		assert.Empty(t, products.scopedOrgs, "an empty tenant must fail before the catalog is read")
	})

	t.Run("foreign product is not readable through a forged organizationId", func(t *testing.T) {
		repo, records, products, router := newIDOROrderCreateHarness(orderProduct(productID, idorOrderForeignOrg))

		recorder := serveOrder(router,
			idorOrderRequest(http.MethodPost, "/api/v1/orders/place", strings.NewReader(orderCreateBody(idorOrderForeignOrg, productID))))

		require.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Empty(t, repo.orders, "a rejected basket must not write an order")
		assert.Empty(t, records.created, "a rejected basket must not write an audit row")
		assert.Equal(t, []string{idorOrderSessionOrg}, products.scopedOrgs,
			"the catalog lookup must use the trusted tenant, not the body organizationId")
	})

	t.Run("forged organizationId is ignored for a local product", func(t *testing.T) {
		repo, _, products, router := newIDOROrderCreateHarness(orderProduct(productID, idorOrderSessionOrg))

		recorder := serveOrder(router,
			idorOrderRequest(http.MethodPost, "/api/v1/orders/place", strings.NewReader(orderCreateBody(idorOrderForeignOrg, productID))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		require.Len(t, repo.orders, 1)
		assert.Equal(t, idorOrderSessionOrg, repo.orders[0].OrganizationID)
		// Two scoped reads: catalog pricing + atomic stock reservation
		// (BE-C-11); both must use the trusted tenant, never the body claim.
		assert.Equal(t, []string{idorOrderSessionOrg, idorOrderSessionOrg}, products.scopedOrgs)
	})

	t.Run("own tenant succeeds", func(t *testing.T) {
		repo, _, products, router := newIDOROrderCreateHarness(orderProduct(productID, idorOrderSessionOrg))

		recorder := serveOrder(router,
			idorOrderRequest(http.MethodPost, "/api/v1/orders/place", strings.NewReader(orderCreateBody(idorOrderSessionOrg, productID))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		assert.Equal(t, idorOrderSessionOrg, repo.orders[0].OrganizationID)
		// Two scoped reads: catalog pricing + atomic stock reservation
		// (BE-C-11); both must use the trusted tenant, never the body claim.
		assert.Equal(t, []string{idorOrderSessionOrg, idorOrderSessionOrg}, products.scopedOrgs)
	})

	t.Run("platform admin may target another tenant", func(t *testing.T) {
		repo, _, products, router := newIDOROrderCreateHarness(orderProduct(productID, idorOrderForeignOrg))

		recorder := serveOrder(router,
			platformAdminOrderRequest("/api/v1/orders/place", strings.NewReader(orderCreateBody(idorOrderForeignOrg, productID))))

		require.Equal(t, http.StatusCreated, recorder.Code)
		require.Len(t, repo.orders, 1)
		assert.Equal(t, idorOrderForeignOrg, repo.orders[0].OrganizationID)
		assert.Equal(t, []string{idorOrderForeignOrg, idorOrderForeignOrg}, products.scopedOrgs)
	})
}
