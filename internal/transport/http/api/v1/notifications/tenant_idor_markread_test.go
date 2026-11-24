package notifications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	appnotifications "github.com/guliangiam/guliangiam/internal/app/notifications"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	domain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// tenantNotificationRepo is an in-memory notifications repository for the
// mark-read IDOR regression tests. It exposes both the system-level and the
// tenant-scoped write so the same test code runs before and after the binding
// change: pre-fix MarkSent succeeds for a foreign notification (200), post-fix
// the scoped read settles ownership first (404).
type tenantNotificationRepo struct {
	notifications map[string]*domain.Notification
	markSentCalls int
}

func (r *tenantNotificationRepo) GetByID(_ context.Context, id string) (*domain.Notification, error) {
	n, ok := r.notifications[id]
	if !ok {
		return nil, apperrors.NewNotFound("notification not found")
	}
	return n, nil
}

func (r *tenantNotificationRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Notification, error) {
	n, ok := r.notifications[id]
	if !ok || n.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("notification not found")
	}
	return n, nil
}

func (r *tenantNotificationRepo) List(context.Context, string, string, string, int, int) ([]*domain.Notification, int64, error) {
	return nil, 0, nil
}

func (r *tenantNotificationRepo) Create(context.Context, *domain.Notification) error { return nil }

func (r *tenantNotificationRepo) Update(context.Context, *domain.Notification) error { return nil }

func (r *tenantNotificationRepo) Delete(context.Context, string) error { return nil }

func (r *tenantNotificationRepo) MarkSent(context.Context, string) error {
	r.markSentCalls++
	return nil
}

func (r *tenantNotificationRepo) MarkSentForOrganization(_ context.Context, id, organizationID string) error {
	n, ok := r.notifications[id]
	if !ok || n.OrganizationID != organizationID {
		return apperrors.NewNotFound("notification not found")
	}
	r.markSentCalls++
	return nil
}

func (r *tenantNotificationRepo) MarkFailed(context.Context, string, string) error { return nil }

func (r *tenantNotificationRepo) IncrementRetryCount(context.Context, string) error { return nil }

func (r *tenantNotificationRepo) ListPendingRetries(context.Context, int) ([]*domain.Notification, error) {
	return nil, nil
}

func tenantNotificationRouter(ctrl *gomock.Controller, repo domain.Repository) chi.Router {
	uow := appuowmock.NewMockInfraUoW(ctrl)
	uow.EXPECT().Notifications().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: appnotifications.Service{UOW: uow}})
	return router
}

func markReadTenantRequest(target string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, target, nil)
	return req.WithContext(tenantctx.WithTenantScope(req.Context(), trustedOrg, "session-org", tenantctx.TenantFilterStrict))
}

// TestMarkRead_ForeignNotificationHidden pins the P1 IDOR fix: marking another
// tenant's notification read (even with a forged organizationId query) must
// surface as 404 and must not touch the row.
func TestMarkRead_ForeignNotificationHidden(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := &tenantNotificationRepo{notifications: map[string]*domain.Notification{
		"foreign-notification": {ID: "foreign-notification", OrganizationID: forgedOrg},
	}}
	router := tenantNotificationRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, markReadTenantRequest(
		"/api/v1/notifications/foreign-notification/mark-read?organizationId="+forgedOrg))

	require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
	assert.Zero(t, repo.markSentCalls, "foreign notification must not be marked read")
}

// TestMarkRead_EmptyTrustedTenantRejected pins fail-fast behaviour when the
// authenticated scope has no tenant.
func TestMarkRead_EmptyTrustedTenantRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := &tenantNotificationRepo{notifications: map[string]*domain.Notification{
		"foreign-notification": {ID: "foreign-notification", OrganizationID: forgedOrg},
	}}
	router := tenantNotificationRouter(ctrl, repo)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost,
		"/api/v1/notifications/foreign-notification/mark-read", nil))

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Zero(t, repo.markSentCalls, "no write may run without a trusted tenant")
}

// TestMarkRead_PlatformAdminExplicitTenantAllowed pins the platform
// administrator escape hatch.
func TestMarkRead_PlatformAdminExplicitTenantAllowed(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := &tenantNotificationRepo{notifications: map[string]*domain.Notification{
		"foreign-notification": {ID: "foreign-notification", OrganizationID: forgedOrg},
	}}
	router := tenantNotificationRouter(ctrl, repo)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/notifications/foreign-notification/mark-read?organizationId="+forgedOrg, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), "", "", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333",
	})
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, 1, repo.markSentCalls)
}
