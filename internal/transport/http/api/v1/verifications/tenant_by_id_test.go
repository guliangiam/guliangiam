package verifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	domain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorVerificationTenantOrg  = "11111111-1111-1111-1111-111111111111"
	idorVerificationForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorVerificationForeignID  = "foreign-verification"
	idorVerificationOwnID      = "own-verification"
	idorVerificationCode       = "123456"
)

type idorVerificationRepo struct {
	domain.Repository
	items []*domain.Verification
}

func (r *idorVerificationRepo) GetByID(_ context.Context, id string) (*domain.Verification, error) {
	for _, item := range r.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("verification not found")
}

func (r *idorVerificationRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Verification, error) {
	for _, item := range r.items {
		if item.ID == id && item.OrganizationID == organizationID {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("verification not found")
}

type idorVerificationUOW struct {
	appuow.AuthUoW
	repo domain.Repository
}

func (u *idorVerificationUOW) Verifications() domain.Repository { return u.repo }

func idorVerificationRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), idorVerificationTenantOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: idorVerificationTenantOrg,
	})
	return req.WithContext(ctx)
}

func idorVerificationPlatformRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), idorVerificationTenantOrg, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func idorVerificationRouter(repo domain.Repository) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appverifications.Service{UOW: &idorVerificationUOW{repo: repo}}})
	return router
}

func idorVerification(id, org string) *domain.Verification {
	now := time.Unix(0, 0).UTC()
	return &domain.Verification{
		ID: id, OrganizationID: org, Type: "email", Recipient: "user@example.com",
		Code: idorVerificationCode, Status: "pending", ExpiresAt: now, CreatedAt: now,
	}
}

func serveIdorVerification(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestVerificationByID_ForeignTenantRejected pins that GET /verifications/{id}
// resolves the authenticated tenant, never the caller-supplied organizationId.
func TestVerificationByID_ForeignTenantRejected(t *testing.T) {
	repo := &idorVerificationRepo{items: []*domain.Verification{
		idorVerification(idorVerificationForeignID, idorVerificationForeignOrg),
	}}
	recorder := serveIdorVerification(t, idorVerificationRouter(repo),
		idorVerificationRequest(http.MethodGet,
			"/api/v1/verifications/"+idorVerificationForeignID+"?organizationId="+idorVerificationForeignOrg))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign verification must be indistinguishable from missing")
}

// TestVerificationByID_WithoutTenantScopeRejected pins the fail-fast.
func TestVerificationByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &idorVerificationRepo{items: []*domain.Verification{
		idorVerification(idorVerificationOwnID, idorVerificationTenantOrg),
	}}
	recorder := serveIdorVerification(t, idorVerificationRouter(repo),
		httptest.NewRequest(http.MethodGet, "/api/v1/verifications/"+idorVerificationOwnID, nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestVerificationByID_OwnTenantSucceeds pins the same-tenant success path and
// that the response never carries the OTP plaintext.
func TestVerificationByID_OwnTenantSucceeds(t *testing.T) {
	repo := &idorVerificationRepo{items: []*domain.Verification{
		idorVerification(idorVerificationOwnID, idorVerificationTenantOrg),
	}}
	recorder := serveIdorVerification(t, idorVerificationRouter(repo),
		idorVerificationRequest(http.MethodGet, "/api/v1/verifications/"+idorVerificationOwnID))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), idorVerificationTenantOrg)
	assert.NotContains(t, recorder.Body.String(), idorVerificationCode)
}

// TestVerificationByID_PlatformAdminCanCrossTenant pins the single legitimate
// override: super_admin/system scopes may name another tenant explicitly.
func TestVerificationByID_PlatformAdminCanCrossTenant(t *testing.T) {
	repo := &idorVerificationRepo{items: []*domain.Verification{
		idorVerification(idorVerificationForeignID, idorVerificationForeignOrg),
	}}
	recorder := serveIdorVerification(t, idorVerificationRouter(repo),
		idorVerificationPlatformRequest(http.MethodGet,
			"/api/v1/verifications/"+idorVerificationForeignID+"?organizationId="+idorVerificationForeignOrg))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), idorVerificationForeignOrg)
}
