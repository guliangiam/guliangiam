package webhooks

import (
	"context"
	"encoding/json"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	eventdomainmock "github.com/guliangiam/guliangiam/internal/domain/webhookevents/mock"
	whdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	whdomainmock "github.com/guliangiam/guliangiam/internal/domain/webhooks/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func platformScopeContext() context.Context {
	return tenantctx.WithTenantScope(context.Background(), "", "", tenantctx.TenantFilterSuperAdmin)
}

const (
	eventTenantOrgID   = "01a0ea76-0c00-7000-8000-000000000101"
	eventOtherOrgID    = "01a0ea76-0c00-7000-8000-000000000202"
	eventTenantWebhook = "01a0ea76-0c00-7000-8000-000000000301"
	eventTenantEventID = "01a0ea76-0c00-7000-8000-000000000401"
)

type eventTenantHarness struct {
	service  Service
	webhooks *whdomainmock.MockRepository
	events   *eventdomainmock.MockRepository
}

func newEventTenantHarness(t *testing.T) eventTenantHarness {
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

	return eventTenantHarness{
		service:  Service{UOW: uow},
		webhooks: webhooks,
		events:   events,
	}
}

func otherTenantWebhook() *whdomain.Webhook {
	return &whdomain.Webhook{ID: eventTenantWebhook, OrganizationID: eventOtherOrgID}
}

func tenantWebhook() *whdomain.Webhook {
	return &whdomain.Webhook{ID: eventTenantWebhook, OrganizationID: eventTenantOrgID}
}

func crossTenantEvent() *eventdomain.WebhookEvent {
	return &eventdomain.WebhookEvent{
		ID:        eventTenantEventID,
		WebhookID: eventTenantWebhook,
		Payload:   json.RawMessage(`{"crossTenant":"payload"}`),
	}
}

func assertNotFound(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

func assertInvalidArgument(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// assertUnprocessableEntity 断言语义校验失败（手册 4.14.1：可解析但违反规则 → 422）。
func assertUnprocessableEntity(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
}

func TestListWebhookEvents_CrossTenantWebhookIsNotFound(t *testing.T) {
	h := newEventTenantHarness(t)
	h.webhooks.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantWebhook, eventTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook not found"))

	items, total, err := h.service.ListWebhookEvents(context.Background(), eventTenantWebhook, eventTenantOrgID, 1, 10, "", nil)
	assertNotFound(t, err)
	assert.Nil(t, items)
	assert.Zero(t, total)
}

func TestListWebhookEvents_ForeignRowFromScopedReadIsNotFound(t *testing.T) {
	h := newEventTenantHarness(t)
	h.webhooks.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantWebhook, eventTenantOrgID).
		Return(otherTenantWebhook(), nil)

	items, total, err := h.service.ListWebhookEvents(context.Background(), eventTenantWebhook, eventTenantOrgID, 1, 10, "", nil)
	assertNotFound(t, err)
	assert.Nil(t, items)
	assert.Zero(t, total)
}

func TestListWebhookEvents_SameTenantListsEvents(t *testing.T) {
	h := newEventTenantHarness(t)
	h.webhooks.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantWebhook, eventTenantOrgID).Return(tenantWebhook(), nil)
	h.events.EXPECT().List(gomock.Any(), eventdomain.ListQuery{
		Page: 1, PageSize: 10, WebhookID: eventTenantWebhook, OrganizationID: eventTenantOrgID,
	}).Return([]*eventdomain.WebhookEvent{{ID: eventTenantEventID, WebhookID: eventTenantWebhook}}, int64(1), nil)

	items, total, err := h.service.ListWebhookEvents(context.Background(), eventTenantWebhook, eventTenantOrgID, 1, 10, "", nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, eventTenantEventID, items[0].ID)
}

func TestListWebhookEvents_PlatformScopeSkipsParentLookup(t *testing.T) {
	h := newEventTenantHarness(t)
	h.events.EXPECT().List(gomock.Any(), gomock.Any()).Return(nil, int64(0), nil)

	_, _, err := h.service.ListWebhookEvents(platformScopeContext(), eventTenantWebhook, "", 1, 10, "", nil)
	require.NoError(t, err)
}

func TestGetWebhookEvent_CrossTenantWebhookIsNotFound(t *testing.T) {
	h := newEventTenantHarness(t)
	h.events.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantEventID, eventTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook event not found"))

	detail, err := h.service.GetWebhookEvent(context.Background(), eventTenantEventID, eventTenantOrgID)
	assertNotFound(t, err)
	assert.Nil(t, detail)
}

func TestGetWebhookEvent_SameTenantReturnsPayload(t *testing.T) {
	h := newEventTenantHarness(t)
	h.events.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantEventID, eventTenantOrgID).
		Return(crossTenantEvent(), nil)

	detail, err := h.service.GetWebhookEvent(context.Background(), eventTenantEventID, eventTenantOrgID)
	require.NoError(t, err)
	require.NotNil(t, detail)
	assert.Equal(t, eventTenantEventID, detail.ID)
	assert.JSONEq(t, `{"crossTenant":"payload"}`, string(detail.Payload))
}

func TestDeleteWebhookEvent_CrossTenantWebhookIsNotFoundBeforeDelete(t *testing.T) {
	h := newEventTenantHarness(t)
	h.events.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantEventID, eventTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook event not found"))
	// No Delete expectation: a cross-tenant request must not reach the delete.

	err := h.service.DeleteWebhookEvent(context.Background(), eventTenantEventID, eventTenantOrgID)
	assertNotFound(t, err)
}

func TestDeleteWebhookEvent_SameTenantDeletes(t *testing.T) {
	h := newEventTenantHarness(t)
	h.events.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantEventID, eventTenantOrgID).
		Return(crossTenantEvent(), nil)
	h.events.EXPECT().Delete(gomock.Any(), eventTenantEventID).Return(nil)

	require.NoError(t, h.service.DeleteWebhookEvent(context.Background(), eventTenantEventID, eventTenantOrgID))
}

func TestDeleteWebhookEvent_PlatformScopeSkipsParentLookup(t *testing.T) {
	h := newEventTenantHarness(t)
	h.events.EXPECT().GetByID(gomock.Any(), eventTenantEventID).Return(crossTenantEvent(), nil)
	h.events.EXPECT().Delete(gomock.Any(), eventTenantEventID).Return(nil)

	require.NoError(t, h.service.DeleteWebhookEvent(platformScopeContext(), eventTenantEventID, ""))
}

func TestGetWebhookEvent_EmptyTenantWithoutPlatformScopeRejected(t *testing.T) {
	h := newEventTenantHarness(t)

	detail, err := h.service.GetWebhookEvent(context.Background(), eventTenantEventID, "")
	assertInvalidArgument(t, err)
	assert.Nil(t, detail)
}

func TestReplayWebhookEvent_CrossTenantWebhookIsNotFound(t *testing.T) {
	h := newEventTenantHarness(t)
	h.events.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantEventID, eventTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook event not found"))
	// No Update expectation: a cross-tenant replay must not reach the write.

	assertNotFound(t, h.service.ReplayWebhookEvent(context.Background(), eventTenantEventID, eventTenantOrgID))
}

func TestListWebhookEvents_MissingParentIsNotFound(t *testing.T) {
	h := newEventTenantHarness(t)
	h.webhooks.EXPECT().GetByIDForOrganization(gomock.Any(), eventTenantWebhook, eventTenantOrgID).
		Return(nil, apperrors.NewNotFound("webhook not found"))

	_, _, err := h.service.ListWebhookEvents(context.Background(), eventTenantWebhook, eventTenantOrgID, 1, 10, "", nil)
	assertNotFound(t, err)
}

func TestListWebhookEvents_EmptyTenantWithoutPlatformScopeRejected(t *testing.T) {
	h := newEventTenantHarness(t)

	_, _, err := h.service.ListWebhookEvents(context.Background(), eventTenantWebhook, "", 1, 10, "", nil)
	assertInvalidArgument(t, err)
}
