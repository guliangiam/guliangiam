package webhooks

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	apppkg "github.com/guliangiam/guliangiam/internal/app/webhooks"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	eventdomainmock "github.com/guliangiam/guliangiam/internal/domain/webhookevents/mock"
	whdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	whdomainmock "github.com/guliangiam/guliangiam/internal/domain/webhooks/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	isolationTenantOrgID = "01a0ea76-0c00-7000-8000-000000000501"
	isolationOtherOrgID  = "01a0ea76-0c00-7000-8000-000000000502"
	isolationWebhookID   = "01a0ea76-0c00-7000-8000-000000000503"
	isolationEventID     = "01a0ea76-0c00-7000-8000-000000000504"
	isolationActorID     = "01a0ea76-0c00-7000-8000-000000000505"
	isolationPayload     = "CROSS_TENANT_PAYLOAD_MARKER"
)

type isolationRouter struct {
	router   chi.Router
	webhooks *whdomainmock.MockRepository
	events   *eventdomainmock.MockRepository
}

func newIsolationRouter(t *testing.T) isolationRouter {
	t.Helper()
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockUnitOfWork(ctrl)
	webhooks := whdomainmock.NewMockRepository(ctrl)
	events := eventdomainmock.NewMockRepository(ctrl)

	uow.EXPECT().Webhooks().Return(webhooks).AnyTimes()
	uow.EXPECT().WebhookEvents().Return(events).AnyTimes()
	uow.EXPECT().WithinTransaction(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
			return fn(ctx, uow)
		},
	).AnyTimes()

	router := chi.NewRouter()
	RegisterChiRoutes(router, Handler{Service: apppkg.Service{UOW: uow}})
	return isolationRouter{router: router, webhooks: webhooks, events: events}
}

func isolationRequest(method, target, orgID string, mode tenantctx.TenantFilterMode) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	if orgID != "" || mode == tenantctx.TenantFilterSuperAdmin || mode == tenantctx.TenantFilterSystem {
		ctx := tenantctx.WithTenantScope(req.Context(), orgID, "session-org", mode)
		ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
			UserID: isolationActorID, OrganizationID: orgID, PrincipalType: "user",
		})
		req = req.WithContext(ctx)
	}
	return req
}

func isolationWebhook(orgID string) *whdomain.Webhook {
	return &whdomain.Webhook{ID: isolationWebhookID, OrganizationID: orgID}
}

func isolationEvent() *eventdomain.WebhookEvent {
	return &eventdomain.WebhookEvent{
		ID:        isolationEventID,
		WebhookID: isolationWebhookID,
		EventType: "user.created",
		Payload:   json.RawMessage(`{"marker":"` + isolationPayload + `"}`),
		Status:    "delivered",
	}
}

func serveIsolation(r isolationRouter, req *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	r.router.ServeHTTP(recorder, req)
	return recorder
}

func TestListEvents_CrossTenantWebhookRejected(t *testing.T) {
	r := newIsolationRouter(t)
	r.webhooks.EXPECT().GetByIDForOrganization(gomock.Any(), isolationWebhookID, isolationTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook not found"))
	// No List expectation: a cross-tenant list must not enumerate events.

	resp := serveIsolation(r, isolationRequest(http.MethodGet,
		"/api/v1/webhooks/"+isolationWebhookID+"/events?organizationId="+isolationOtherOrgID,
		isolationTenantOrgID, tenantctx.TenantFilterStrict))
	require.Equal(t, http.StatusNotFound, resp.Code)
	require.NotContains(t, resp.Body.String(), isolationPayload)
}

func TestListEvents_SameTenantListsForgedQueryIgnored(t *testing.T) {
	r := newIsolationRouter(t)
	r.webhooks.EXPECT().GetByIDForOrganization(gomock.Any(), isolationWebhookID, isolationTenantOrgID).
		Return(isolationWebhook(isolationTenantOrgID), nil)
	r.events.EXPECT().List(gomock.Any(), eventdomain.ListQuery{
		Page: 1, PageSize: 20, WebhookID: isolationWebhookID, OrganizationID: isolationTenantOrgID,
	}).Return([]*eventdomain.WebhookEvent{isolationEvent()}, int64(1), nil)

	resp := serveIsolation(r, isolationRequest(http.MethodGet,
		"/api/v1/webhooks/"+isolationWebhookID+"/events?organizationId="+isolationOtherOrgID,
		isolationTenantOrgID, tenantctx.TenantFilterStrict))
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), isolationEventID)
}

func TestGetEvent_CrossTenantRejectedWithoutPayload(t *testing.T) {
	r := newIsolationRouter(t)
	r.events.EXPECT().GetByIDForOrganization(gomock.Any(), isolationEventID, isolationTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook event not found"))

	resp := serveIsolation(r, isolationRequest(http.MethodGet,
		"/api/v1/webhook-events/"+isolationEventID+"?organizationId="+isolationOtherOrgID,
		isolationTenantOrgID, tenantctx.TenantFilterStrict))
	require.Equal(t, http.StatusNotFound, resp.Code)
	assert.NotContains(t, resp.Body.String(), isolationPayload)
	assert.NotContains(t, resp.Body.String(), "user.created")
}

func TestGetEvent_SameTenantReturnsPayload(t *testing.T) {
	r := newIsolationRouter(t)
	r.events.EXPECT().GetByIDForOrganization(gomock.Any(), isolationEventID, isolationTenantOrgID).
		Return(isolationEvent(), nil)

	resp := serveIsolation(r, isolationRequest(http.MethodGet,
		"/api/v1/webhook-events/"+isolationEventID,
		isolationTenantOrgID, tenantctx.TenantFilterStrict))
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), isolationPayload)
}

func TestDeleteEvent_CrossTenantRejectedBeforeDelete(t *testing.T) {
	r := newIsolationRouter(t)
	r.events.EXPECT().GetByIDForOrganization(gomock.Any(), isolationEventID, isolationTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook event not found"))
	// No Delete expectation: the row must still exist after a cross-tenant request.

	resp := serveIsolation(r, isolationRequest(http.MethodDelete,
		"/api/v1/webhook-events/"+isolationEventID+"?organizationId="+isolationOtherOrgID,
		isolationTenantOrgID, tenantctx.TenantFilterStrict))
	require.Equal(t, http.StatusNotFound, resp.Code)
	assert.NotContains(t, resp.Body.String(), isolationPayload)
}

func TestDeleteEvent_SameTenantDeletes(t *testing.T) {
	r := newIsolationRouter(t)
	r.events.EXPECT().GetByIDForOrganization(gomock.Any(), isolationEventID, isolationTenantOrgID).
		Return(isolationEvent(), nil)
	r.events.EXPECT().Delete(gomock.Any(), isolationEventID).Return(nil)

	resp := serveIsolation(r, isolationRequest(http.MethodDelete,
		"/api/v1/webhook-events/"+isolationEventID,
		isolationTenantOrgID, tenantctx.TenantFilterStrict))
	require.Equal(t, http.StatusOK, resp.Code)
}

func TestReplayEvent_CrossTenantRejectedBeforeWrite(t *testing.T) {
	r := newIsolationRouter(t)
	r.events.EXPECT().GetByIDForOrganization(gomock.Any(), isolationEventID, isolationTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook event not found"))
	// No Update expectation: a cross-tenant replay must not reset another tenant's event.

	resp := serveIsolation(r, isolationRequest(http.MethodPost,
		"/api/v1/webhook-events/"+isolationEventID+"/replay",
		isolationTenantOrgID, tenantctx.TenantFilterStrict))
	require.Equal(t, http.StatusNotFound, resp.Code)
}

func TestGetEvent_PlatformScopeKeepsAllTenantAccess(t *testing.T) {
	r := newIsolationRouter(t)
	r.events.EXPECT().GetByID(gomock.Any(), isolationEventID).Return(isolationEvent(), nil)
	// No parent lookup: the platform scope is authorized to read any tenant.

	resp := serveIsolation(r, isolationRequest(http.MethodGet,
		"/api/v1/webhook-events/"+isolationEventID,
		"", tenantctx.TenantFilterSuperAdmin))
	require.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), isolationPayload)
}

func TestGetEvent_RegularCallerWithoutTenantRejected(t *testing.T) {
	r := newIsolationRouter(t)

	resp := serveIsolation(r, httptest.NewRequest(http.MethodGet,
		"/api/v1/webhook-events/"+isolationEventID, nil))
	require.Equal(t, http.StatusBadRequest, resp.Code)
	assert.NotContains(t, resp.Body.String(), isolationPayload)
}
