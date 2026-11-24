package messages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appmessages "github.com/guliangiam/guliangiam/internal/app/messages"
	domainnotificationproviders "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainnotifications "github.com/guliangiam/guliangiam/internal/domain/notifications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
)

const (
	trustedNotificationOrg = "11111111-1111-1111-1111-111111111111"
	forgedNotificationOrg  = "22222222-2222-2222-2222-222222222222"
	foreignProviderID      = "33333333-3333-3333-3333-333333333333"
)

type notificationDelivery struct {
	providerType   string
	organizationID string
	recipient      string
}

type recordingNotificationFactory struct {
	calls         []notificationDelivery
	rejectedTypes map[string]bool
}

func (f *recordingNotificationFactory) SendNotification(_ context.Context, providerType string, n *domainnotifications.SendNotification) error {
	f.calls = append(f.calls, notificationDelivery{
		providerType: providerType, organizationID: n.OrganizationID, recipient: n.Recipient,
	})
	if f.rejectedTypes[providerType] {
		return apperrors.NewUnavailablef("notification factory: provider %q not registered", providerType)
	}
	return nil
}

func notificationRequest(method, target, body string, scope *tenantctx.TenantScope) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := req.Context()
	if scope != nil {
		ctx = tenantctx.WithTenantScope(ctx, scope.OrganizationID, "session-org", scope.Mode)
	}
	ctx = authtransport.WithSessionContext(ctx, authtransport.SessionContext{
		UserID: "user-a", Username: "alice", OrganizationID: trustedNotificationOrg,
	})
	return req.WithContext(ctx)
}

// fakeNotificationProviderLookup is the tenant-scoped first stage of the
// providerID → providerType resolution (BE-B-12): a row is only visible to its
// owning organization.
type fakeNotificationProviderLookup struct {
	providers []*domainnotificationproviders.NotificationProvider
}

func (l fakeNotificationProviderLookup) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domainnotificationproviders.NotificationProvider, error) {
	for _, p := range l.providers {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("notification provider not found")
}

func notificationRouter(factory *recordingNotificationFactory, lookup appmessages.ProviderLookup) chi.Router {
	router := chi.NewRouter()
	RegisterChiRoutes(router, NewHandler(appmessages.NewServiceWithProviderLookup(factory, lookup)))
	return router
}

func TestSendNotificationUsesTrustedTenantForForgedBodyOrg(t *testing.T) {
	factory := &recordingNotificationFactory{rejectedTypes: map[string]bool{}}
	lookup := fakeNotificationProviderLookup{providers: []*domainnotificationproviders.NotificationProvider{
		{ID: "email", OrganizationID: trustedNotificationOrg, ProviderType: "email"},
	}}
	req := notificationRequest(http.MethodPost, "/api/v1/messages/notification",
		`{"organizationId":"`+forgedNotificationOrg+`","providerId":"email","title":"t","content":"c","targets":["a@example.com"]}`,
		&tenantctx.TenantScope{OrganizationID: trustedNotificationOrg, Mode: tenantctx.TenantFilterStrict})

	recorder := httptest.NewRecorder()
	notificationRouter(factory, lookup).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, factory.calls, 1)
	assert.Equal(t, trustedNotificationOrg, factory.calls[0].organizationID,
		"the delivery must carry the authenticated tenant, not the forged body value")
	assert.Equal(t, "email", factory.calls[0].providerType,
		"the provider row ID must be resolved to its providerType before reaching the factory")
}

func TestSendNotificationRejectsForeignProvider(t *testing.T) {
	factory := &recordingNotificationFactory{rejectedTypes: map[string]bool{}}
	// The foreign provider row exists but belongs to another organization:
	// the tenant-scoped lookup must hide it before the factory is reached.
	lookup := fakeNotificationProviderLookup{providers: []*domainnotificationproviders.NotificationProvider{
		{ID: foreignProviderID, OrganizationID: forgedNotificationOrg, ProviderType: "email"},
	}}
	req := notificationRequest(http.MethodPost, "/api/v1/messages/notification",
		`{"organizationId":"`+forgedNotificationOrg+`","providerId":"`+foreignProviderID+`","title":"t","content":"c","targets":["a@example.com"]}`,
		&tenantctx.TenantScope{OrganizationID: trustedNotificationOrg, Mode: tenantctx.TenantFilterStrict})

	recorder := httptest.NewRecorder()
	notificationRouter(factory, lookup).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Empty(t, factory.calls,
		"a providerID owned by another tenant must be rejected before any delivery attempt")
}

func TestSendNotificationPlatformAdminExplicitTargetHonoured(t *testing.T) {
	factory := &recordingNotificationFactory{rejectedTypes: map[string]bool{}}
	lookup := fakeNotificationProviderLookup{providers: []*domainnotificationproviders.NotificationProvider{
		{ID: "email", OrganizationID: forgedNotificationOrg, ProviderType: "email"},
	}}
	req := notificationRequest(http.MethodPost, "/api/v1/messages/notification",
		`{"organizationId":"`+forgedNotificationOrg+`","providerId":"email","title":"t","content":"c","targets":["a@example.com"]}`,
		&tenantctx.TenantScope{Mode: tenantctx.TenantFilterSuperAdmin})

	recorder := httptest.NewRecorder()
	notificationRouter(factory, lookup).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, factory.calls, 1)
	assert.Equal(t, forgedNotificationOrg, factory.calls[0].organizationID,
		"an explicit cross-tenant target stays available to platform administrators")
}

func TestSendNotificationWithoutTrustedTenantRejected(t *testing.T) {
	factory := &recordingNotificationFactory{rejectedTypes: map[string]bool{}}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/notification",
		strings.NewReader(`{"organizationId":"`+forgedNotificationOrg+`","providerId":"email","title":"t","content":"c","targets":["a@example.com"]}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	notificationRouter(factory, fakeNotificationProviderLookup{}).ServeHTTP(recorder, req)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "organizationId is required")
	assert.Empty(t, factory.calls)
}
