package invitations

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	idorInvitationTenantOrg  = "11111111-1111-1111-1111-111111111111"
	idorInvitationForeignOrg = "22222222-2222-2222-2222-222222222222"
	idorInvitationForeignID  = "foreign-invitation"
	idorInvitationOwnID      = "own-invitation"
)

type idorInvitationRepo struct {
	items []*domain.Invitation
}

func (r *idorInvitationRepo) GetByID(_ context.Context, id string) (*domain.Invitation, error) {
	for _, item := range r.items {
		if item.ID == id {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *idorInvitationRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Invitation, error) {
	for _, item := range r.items {
		if item.ID == id && item.OrganizationID == organizationID {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *idorInvitationRepo) GetByCode(_ context.Context, code string) (*domain.Invitation, error) {
	for _, item := range r.items {
		if item.Code == code {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *idorInvitationRepo) GetByEmail(_ context.Context, organizationID, email string) (*domain.Invitation, error) {
	for _, item := range r.items {
		if item.OrganizationID == organizationID && item.Email == email {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *idorInvitationRepo) List(_ context.Context, _ domain.ListQuery) ([]*domain.Invitation, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r *idorInvitationRepo) Create(_ context.Context, item *domain.Invitation) error {
	r.items = append(r.items, item)
	return nil
}

func (r *idorInvitationRepo) Update(_ context.Context, item *domain.Invitation) error {
	r.items = append(r.items, item)
	return nil
}

func (r *idorInvitationRepo) Delete(_ context.Context, _ string) error { return nil }

type idorInvitationUOW struct {
	appuow.IdentityUoW
	repo domain.Repository
}

func (u *idorInvitationUOW) Invitations() domain.Repository { return u.repo }

func idorInvitationRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorInvitationTenantOrg, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: idorInvitationTenantOrg,
	})
	return req.WithContext(ctx)
}

func idorInvitationPlatformRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), idorInvitationTenantOrg, "session-org", tenantctx.TenantFilterSuperAdmin)
	return req.WithContext(ctx)
}

func idorInvitationRouter(repo domain.Repository) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appinvitations.Service{UOW: &idorInvitationUOW{repo: repo}}})
	return router
}

func idorInvitation(id, org string) *domain.Invitation {
	now := time.Unix(0, 0).UTC()
	return &domain.Invitation{
		ID: id, OrganizationID: org, Email: "user@example.com", Code: "invite-code",
		Status: domain.StatusPending, ExpiresAt: now, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
}

func serveIdorInvitation(t *testing.T, router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

// TestInvitationByID_ForeignTenantRejected pins that GET /invitations/{id}
// resolves the authenticated tenant, never the caller-supplied organizationId.
func TestInvitationByID_ForeignTenantRejected(t *testing.T) {
	repo := &idorInvitationRepo{items: []*domain.Invitation{
		idorInvitation(idorInvitationForeignID, idorInvitationForeignOrg),
	}}
	recorder := serveIdorInvitation(t, idorInvitationRouter(repo),
		idorInvitationRequest(http.MethodGet,
			"/api/v1/invitations/"+idorInvitationForeignID+"?organizationId="+idorInvitationForeignOrg, nil))

	assert.Equal(t, http.StatusNotFound, recorder.Code, "foreign invitation must be indistinguishable from missing")
}

// TestInvitationByID_WithoutTenantScopeRejected pins the fail-fast.
func TestInvitationByID_WithoutTenantScopeRejected(t *testing.T) {
	repo := &idorInvitationRepo{items: []*domain.Invitation{
		idorInvitation(idorInvitationOwnID, idorInvitationTenantOrg),
	}}
	recorder := serveIdorInvitation(t, idorInvitationRouter(repo),
		httptest.NewRequest(http.MethodGet, "/api/v1/invitations/"+idorInvitationOwnID, nil))

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

// TestInvitationByID_OwnTenantSucceeds pins the same-tenant success path.
func TestInvitationByID_OwnTenantSucceeds(t *testing.T) {
	repo := &idorInvitationRepo{items: []*domain.Invitation{
		idorInvitation(idorInvitationOwnID, idorInvitationTenantOrg),
	}}
	recorder := serveIdorInvitation(t, idorInvitationRouter(repo),
		idorInvitationRequest(http.MethodGet, "/api/v1/invitations/"+idorInvitationOwnID, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), idorInvitationTenantOrg)
}

// TestInvitationByID_PlatformAdminCanCrossTenant pins the single legitimate
// override: super_admin/system scopes may name another tenant explicitly.
func TestInvitationByID_PlatformAdminCanCrossTenant(t *testing.T) {
	repo := &idorInvitationRepo{items: []*domain.Invitation{
		idorInvitation(idorInvitationForeignID, idorInvitationForeignOrg),
	}}
	recorder := serveIdorInvitation(t, idorInvitationRouter(repo),
		idorInvitationPlatformRequest(http.MethodGet,
			"/api/v1/invitations/"+idorInvitationForeignID+"?organizationId="+idorInvitationForeignOrg, nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), idorInvitationForeignOrg)
}
