package webhooks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	apppkg "github.com/guliangiam/guliangiam/internal/app/webhooks"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	domainmock "github.com/guliangiam/guliangiam/internal/domain/webhooks/mock"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	forgedOrgSessionID = "11111111-1111-1111-1111-111111111111"
	forgedOrgRequestID = "22222222-2222-2222-2222-222222222222"
	forgedWebhookID    = "44444444-4444-4444-4444-444444444444"
)

type forgedOrgClock struct{}

func (forgedOrgClock) Now() time.Time { return time.Unix(0, 0).UTC() }

type forgedOrgUUID struct{}

func (forgedOrgUUID) NewString() (string, error) { return "generated-webhook-id", nil }

func forgedOrgRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	ctx := tenantctx.WithTenantScope(req.Context(), forgedOrgSessionID, "session-org", tenantctx.TenantFilterStrict)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "33333333-3333-3333-3333-333333333333", OrganizationID: forgedOrgSessionID,
	})
	return req.WithContext(ctx)
}

func forgedOrgRouter(ctrl *gomock.Controller) (chi.Router, *domainmock.MockRepository) {
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	repo := domainmock.NewMockRepository(ctrl)
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()
	uow.EXPECT().Webhooks().Return(repo).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{
		Clock: forgedOrgClock{}, UUID: forgedOrgUUID{}, UOW: uow,
	}})
	return router, repo
}

// TestListWebhooks_ForgedQueryOrganizationIDIgnored pins that the query value
// only requests a tenant; the authenticated scope wins.
func TestListWebhooks_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, query domain.ListQuery) ([]*domain.Webhook, int64, error) {
			assert.Equal(t, forgedOrgSessionID, query.OrganizationID)
			return nil, 0, nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/webhooks?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestCreateWebhook_ForgedBodyOrganizationIDIgnored pins that a regular caller
// cannot move a webhook write into another tenant through the JSON body.
func TestCreateWebhook_ForgedBodyOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, webhook *domain.Webhook) error {
			assert.Equal(t, forgedOrgSessionID, webhook.OrganizationID)
			return nil
		},
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPost, "/api/v1/webhooks",
		strings.NewReader(`{"organizationId":"`+forgedOrgRequestID+`","name":"hook","url":"https://example.test/hook","method":"POST"}`)))
	require.Equal(t, http.StatusCreated, recorder.Code)
}

// TestGetWebhook_ForgedQueryOrganizationIDIgnored pins that a regular caller's
// forged ?organizationId cannot widen a by-ID read: the stored row belongs to
// the authenticated tenant, so only the trusted scope can make it succeed.
func TestGetWebhook_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), forgedWebhookID, forgedOrgSessionID).Return(
		&domain.Webhook{ID: forgedWebhookID, OrganizationID: forgedOrgSessionID}, nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodGet,
		"/api/v1/webhooks/"+forgedWebhookID+"?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), forgedWebhookID)
}

// TestPatchWebhook_ForgedQueryOrganizationIDIgnored pins the same guard on the
// write path: the update is scoped to the authenticated tenant, not the query.
func TestPatchWebhook_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), forgedWebhookID, forgedOrgSessionID).Return(
		&domain.Webhook{ID: forgedWebhookID, OrganizationID: forgedOrgSessionID, Revision: 1}, nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, webhook *domain.Webhook) error {
			assert.Equal(t, forgedOrgSessionID, webhook.OrganizationID)
			return nil
		})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodPatch,
		"/api/v1/webhooks/"+forgedWebhookID+"?organizationId="+forgedOrgRequestID,
		strings.NewReader(`{"displayName":"renamed"}`)))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestDeleteWebhook_ForgedQueryOrganizationIDIgnored pins that a forged query
// cannot steer a delete into another tenant.
func TestDeleteWebhook_ForgedQueryOrganizationIDIgnored(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), forgedWebhookID, forgedOrgSessionID).Return(
		&domain.Webhook{ID: forgedWebhookID, OrganizationID: forgedOrgSessionID}, nil)
	repo.EXPECT().Delete(gomock.Any(), forgedWebhookID).Return(nil)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, forgedOrgRequest(http.MethodDelete,
		"/api/v1/webhooks/"+forgedWebhookID+"?organizationId="+forgedOrgRequestID, nil))
	require.Equal(t, http.StatusOK, recorder.Code)
}

// TestGetWebhook_PlatformScopeNarrowsByQuery pins that only a platform scope
// may narrow an all-tenant by-ID read with organizationId.
func TestGetWebhook_PlatformScopeNarrowsByQuery(t *testing.T) {
	ctrl := gomock.NewController(t)
	router, repo := forgedOrgRouter(ctrl)
	repo.EXPECT().GetByIDForOrganization(gomock.Any(), forgedWebhookID, forgedOrgRequestID).Return(
		&domain.Webhook{ID: forgedWebhookID, OrganizationID: forgedOrgRequestID}, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/webhooks/"+forgedWebhookID+"?organizationId="+forgedOrgRequestID, nil)
	ctx := tenantctx.WithTenantScope(req.Context(), "", "session-org", tenantctx.TenantFilterSuperAdmin)
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{UserID: forgedOrgSessionID})
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), forgedWebhookID)
}

// TestWebhooks_WithoutTenantScopeRejected pins the fail-fast guard.
func TestWebhooks_WithoutTenantScopeRejected(t *testing.T) {
	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{})

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "list", method: http.MethodGet, target: "/api/v1/webhooks?organizationId=" + forgedOrgRequestID},
		{name: "create", method: http.MethodPost, target: "/api/v1/webhooks",
			body: `{"organizationId":"` + forgedOrgRequestID + `","name":"hook"}`},
		{name: "get by id", method: http.MethodGet, target: "/api/v1/webhooks/" + forgedWebhookID},
		{name: "patch by id", method: http.MethodPatch, target: "/api/v1/webhooks/" + forgedWebhookID,
			body: `{}`},
		{name: "delete by id", method: http.MethodDelete, target: "/api/v1/webhooks/" + forgedWebhookID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.target, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}
