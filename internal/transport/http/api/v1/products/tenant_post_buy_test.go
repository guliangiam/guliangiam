package products

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appproducts "github.com/guliangiam/guliangiam/internal/app/products"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/products"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	postProductTenantOrg  = "11111111-1111-1111-1111-111111111111"
	postProductForeignOrg = "22222222-2222-2222-2222-222222222222"
	postProductID         = "33333333-3333-3333-3333-333333333333"
)

type postProductClock struct{}

func (postProductClock) Now() time.Time { return time.Unix(0, 0).UTC() }

// postProductRepo keeps the system-scoped lookup observable so the pre-fix
// cross-tenant purchase can be reproduced.
type postProductRepo struct {
	domain.Repository
	products   []*domain.Product
	systemGets int
	scopedOrgs []string
}

func (r *postProductRepo) GetByID(_ context.Context, id string) (*domain.Product, error) {
	r.systemGets++
	for _, product := range r.products {
		if product.ID == id {
			return product, nil
		}
	}
	return nil, apperrors.NewNotFound("product not found")
}

func (r *postProductRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Product, error) {
	r.scopedOrgs = append(r.scopedOrgs, organizationID)
	for _, product := range r.products {
		if product.ID == id && product.OrganizationID == organizationID {
			return product, nil
		}
	}
	return nil, apperrors.NewNotFound("product not found")
}

type postProductUOW struct {
	appuow.CommerceUoW
	repo domain.Repository
}

func (u *postProductUOW) Products() domain.Repository { return u.repo }

type postProductHarness struct {
	repo   *postProductRepo
	router chi.Router
}

func newPostProductHarness(items ...*domain.Product) *postProductHarness {
	repo := &postProductRepo{products: items}
	service := appproducts.Service{Clock: postProductClock{}, UOW: &postProductUOW{repo: repo}}
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: service})
	return &postProductHarness{repo: repo, router: router}
}

func postProductRequest(method, target, body, organizationID string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if organizationID != "" {
		ctx := tenantctx.WithTenantScope(req.Context(), organizationID, "session-org", tenantctx.TenantFilterStrict)
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
			UserID: "44444444-4444-4444-4444-444444444444", OrganizationID: organizationID,
		})
		req = req.WithContext(ctx)
	}
	return req
}

func postProductPlatformRequest(target, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin))
}

func servePostProduct(t *testing.T, harness *postProductHarness, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	harness.router.ServeHTTP(recorder, req)
	return recorder
}

func postProduct(organizationID string) *domain.Product {
	return &domain.Product{
		ID: postProductID, OrganizationID: organizationID, Name: "tenant-product",
		State: domain.StateActive, Revision: 1,
	}
}

const postProductBody = `{"user":"buyer","quantity":1}`

// TestProductBuy_ForeignTenantRejected pins that POST /products/{id}/buy is
// tenant-bound: a foreign product is a 404 and never read through GetByID.
func TestProductBuy_ForeignTenantRejected(t *testing.T) {
	harness := newPostProductHarness(postProduct(postProductForeignOrg))

	recorder := servePostProduct(t, harness, postProductRequest(http.MethodPost,
		"/api/v1/products/"+postProductID+"/buy?organizationId="+postProductForeignOrg,
		postProductBody, postProductTenantOrg))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign product must look missing")
	assert.Zero(t, harness.repo.systemGets, "the system-scoped lookup must not be used")
	assert.Equal(t, []string{postProductTenantOrg}, harness.repo.scopedOrgs,
		"the forged query organizationId must be ignored")
}

// TestProductBuy_WithoutTenantScopeRejected pins the fail-fast.
func TestProductBuy_WithoutTenantScopeRejected(t *testing.T) {
	harness := newPostProductHarness(postProduct(postProductTenantOrg))

	recorder := servePostProduct(t, harness, postProductRequest(http.MethodPost,
		"/api/v1/products/"+postProductID+"/buy", postProductBody, ""))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Zero(t, harness.repo.systemGets)
	assert.Empty(t, harness.repo.scopedOrgs, "an empty tenant must not reach the repository")
}

// TestProductBuy_OwnTenantSucceeds pins the same-tenant path.
func TestProductBuy_OwnTenantSucceeds(t *testing.T) {
	harness := newPostProductHarness(postProduct(postProductTenantOrg))

	recorder := servePostProduct(t, harness, postProductRequest(http.MethodPost,
		"/api/v1/products/"+postProductID+"/buy", postProductBody, postProductTenantOrg))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{postProductTenantOrg}, harness.repo.scopedOrgs)
	assert.Contains(t, recorder.Body.String(), "tenant-product")
}

// TestProductBuy_PlatformAdminCanTargetTenant pins the single legit override.
func TestProductBuy_PlatformAdminCanTargetTenant(t *testing.T) {
	harness := newPostProductHarness(postProduct(postProductForeignOrg))

	recorder := servePostProduct(t, harness, postProductPlatformRequest(
		"/api/v1/products/"+postProductID+"/buy?organizationId="+postProductForeignOrg, postProductBody))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []string{postProductForeignOrg}, harness.repo.scopedOrgs)
}
