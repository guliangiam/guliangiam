package webhooks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	adapterdomain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	agentdomain "github.com/guliangiam/guliangiam/internal/domain/agents"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	captchaproviderdomain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/domain/castickets"
	certdomain "github.com/guliangiam/guliangiam/internal/domain/certs"
	consentdomain "github.com/guliangiam/guliangiam/internal/domain/consents"
	coupondomain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	entrydomain "github.com/guliangiam/guliangiam/internal/domain/entries"
	faceidproviderdomain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	formdomain "github.com/guliangiam/guliangiam/internal/domain/forms"
	gatewaydomain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	groupsdomain "github.com/guliangiam/guliangiam/internal/domain/groups"
	idvdomain "github.com/guliangiam/guliangiam/internal/domain/idv"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	invitationsdomain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	keysdomain "github.com/guliangiam/guliangiam/internal/domain/keys"
	ldapdomain "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	mcpserverdomain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	meteringdomain "github.com/guliangiam/guliangiam/internal/domain/metering"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	notificationproviderdomain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	notificationsdomain "github.com/guliangiam/guliangiam/internal/domain/notifications"
	orderdomain "github.com/guliangiam/guliangiam/internal/domain/orders"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	paymentdomain "github.com/guliangiam/guliangiam/internal/domain/payments"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	plandomain "github.com/guliangiam/guliangiam/internal/domain/plans"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	pricingdomain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	productdomain "github.com/guliangiam/guliangiam/internal/domain/products"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	radiusaccountingdomain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	radiusconfigdomain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	rebac "github.com/guliangiam/guliangiam/internal/domain/rebac"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	resourcedomain "github.com/guliangiam/guliangiam/internal/domain/resources"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"
	roledomain "github.com/guliangiam/guliangiam/internal/domain/roles"
	ruledomain "github.com/guliangiam/guliangiam/internal/domain/rules"
	sessiondomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	subscriptiondomain "github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	syncerdomain "github.com/guliangiam/guliangiam/internal/domain/syncers"
	ticketdomain "github.com/guliangiam/guliangiam/internal/domain/tickets"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	transactiondomain "github.com/guliangiam/guliangiam/internal/domain/transactions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/domain/web3identity"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Fake platform implementations
// ---------------------------------------------------------------------------

type fakeClock struct {
	fixedTime time.Time
}

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct {
	fixedID string
}

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

// ---------------------------------------------------------------------------
// Mock webhook repository
// ---------------------------------------------------------------------------

type mockWebhookRepo struct {
	webhooks   []*domain.Webhook
	updates    int
	deletes    int
	listErr    error
	getByIDErr error
	lastOrgID  string
	lastEvent  string
}

func (r *mockWebhookRepo) GetByID(ctx context.Context, id string) (*domain.Webhook, error) {
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	for _, w := range r.webhooks {
		if w.ID == id {
			return w, nil
		}
	}
	return nil, apperrors.NewNotFound("webhook not found")
}

func (r *mockWebhookRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Webhook, error) {
	for _, w := range r.webhooks {
		if w.ID == id && w.OrganizationID == organizationID {
			return w, nil
		}
	}
	return nil, apperrors.NewNotFound("webhook not found")
}

func (r *mockWebhookRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Webhook, int64, error) {
	return r.webhooks, int64(len(r.webhooks)), nil
}

func (r *mockWebhookRepo) ListByOrganizationAndEvent(ctx context.Context, organizationID, event string) ([]*domain.Webhook, error) {
	r.lastOrgID = organizationID
	r.lastEvent = event
	if r.listErr != nil {
		return nil, r.listErr
	}
	out := make([]*domain.Webhook, 0)
	for _, w := range r.webhooks {
		if w.OrganizationID != organizationID || !w.IsEnabled {
			continue
		}
		for _, subscribed := range w.Events {
			if subscribed == event {
				out = append(out, w)
				break
			}
		}
	}
	return out, nil
}

func (r *mockWebhookRepo) Create(ctx context.Context, webhook *domain.Webhook) error {
	r.webhooks = append(r.webhooks, webhook)
	return nil
}

func (r *mockWebhookRepo) Update(ctx context.Context, webhook *domain.Webhook) error {
	r.updates++
	for i, w := range r.webhooks {
		if w.ID == webhook.ID {
			r.webhooks[i] = webhook
			return nil
		}
	}
	return apperrors.NewNotFound("webhook not found")
}

func (r *mockWebhookRepo) Delete(ctx context.Context, id string) error {
	r.deletes++
	for i, w := range r.webhooks {
		if w.ID == id {
			r.webhooks = append(r.webhooks[:i], r.webhooks[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("webhook not found")
}

// ---------------------------------------------------------------------------
// Mock webhook event repository
// ---------------------------------------------------------------------------

type mockEventRepo struct {
	events            []*eventdomain.WebhookEvent
	getByIDCalls      int
	getByIDErr        error
	listExistingCalls int
	createBatchCalls  int
}

func (r *mockEventRepo) GetByID(ctx context.Context, id string) (*eventdomain.WebhookEvent, error) {
	r.getByIDCalls++
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	for _, e := range r.events {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, apperrors.NewNotFound("event not found")
}

// GetByIDForOrganization delegates to the by-ID lookup: this hand-written test
// double has no parent webhook table, and the scoped event tests use the gomock
// repositories in event_mgmt_tenant_test.go.
func (r *mockEventRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*eventdomain.WebhookEvent, error) {
	return r.GetByID(ctx, id)
}

func (r *mockEventRepo) List(ctx context.Context, q eventdomain.ListQuery) ([]*eventdomain.WebhookEvent, int64, error) {
	return r.events, int64(len(r.events)), nil
}

func (r *mockEventRepo) ListExistingIDs(ctx context.Context, ids []string) ([]string, error) {
	r.listExistingCalls++
	seen := make(map[string]bool, len(r.events))
	for _, e := range r.events {
		seen[e.ID] = true
	}
	found := make([]string, 0, len(ids))
	for _, id := range ids {
		if seen[id] {
			found = append(found, id)
		}
	}
	return found, nil
}

func (r *mockEventRepo) CreateBatch(ctx context.Context, events []*eventdomain.WebhookEvent) error {
	r.createBatchCalls++
	r.events = append(r.events, events...)
	return nil
}

func (r *mockEventRepo) Create(ctx context.Context, event *eventdomain.WebhookEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *mockEventRepo) Update(ctx context.Context, event *eventdomain.WebhookEvent) error {
	for i, e := range r.events {
		if e.ID == event.ID {
			r.events[i] = event
			return nil
		}
	}
	return apperrors.NewNotFound("event not found")
}

func (r *mockEventRepo) Delete(ctx context.Context, id string) error {
	for i, e := range r.events {
		if e.ID == id {
			r.events = append(r.events[:i], r.events[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("event not found")
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	webhookRepo *mockWebhookRepo
	eventRepo   *mockEventRepo
	auditCalls  *int
	outboxRepo  domainoutbox.Repository
}

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository                          { return nil }
func (u testUnitOfWork) Permissions() permissiondomain.Repository                     { return nil }
func (u testUnitOfWork) Providers() providerdomain.Repository                         { return nil }
func (u testUnitOfWork) Certs() certdomain.Repository                                 { return nil }
func (u testUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u testUnitOfWork) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u testUnitOfWork) Notifications() notificationsdomain.Repository                { return nil }
func (u testUnitOfWork) Ldaps() ldapdomain.Repository                                 { return nil }
func (u testUnitOfWork) Models() modeldomain.Repository                               { return nil }
func (u testUnitOfWork) Adapters() adapterdomain.Repository                           { return nil }
func (u testUnitOfWork) Agents() agentdomain.Repository                               { return nil }
func (u testUnitOfWork) Enforcers() enforcerdomain.Repository                         { return nil }
func (u testUnitOfWork) Forms() formdomain.Repository                                 { return nil }
func (u testUnitOfWork) Orders() orderdomain.Repository                               { return nil }
func (u testUnitOfWork) Payments() paymentdomain.Repository                           { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository                                 { return nil }

func (u testUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u testUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u testUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u testUnitOfWork) Pricings() pricingdomain.Repository                               { return nil }
func (u testUnitOfWork) Products() productdomain.Repository                               { return nil }
func (u testUnitOfWork) Records() recordsdomain.Repository {
	if u.auditCalls != nil {
		*u.auditCalls++
	}
	return nil
}
func (u testUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                     { return nil }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                   { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u testUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u testUnitOfWork) Tokens() tokendomain.Repository                       { return nil }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository                   { return u.webhookRepo }
func (u testUnitOfWork) Users() userdomain.Repository                         { return nil }
func (u testUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u testUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u testUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository     { return nil }
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository         { return nil }
func (u testUnitOfWork) Rules() ruledomain.Repository                         { return nil }
func (u testUnitOfWork) Transactions() transactiondomain.Repository           { return nil }
func (u testUnitOfWork) WebhookEvents() eventdomain.Repository                { return u.eventRepo }
func (u testUnitOfWork) Keys() keysdomain.Repository                          { return nil }
func (u testUnitOfWork) QRCodeSessions() qrcodesessions.Repository            { return nil }
func (u testUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository          { return nil }

func (u testUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u testUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u testUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u testUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u testUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }
func (u testUnitOfWork) IDV() idvdomain.Repository                   { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository { return nil }

func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) Outbox() domainoutbox.Repository {
	if u.auditCalls != nil {
		*u.auditCalls++
	}
	return u.outboxRepo
}

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_CreateWebhook_Validation(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "019test-wh-id"},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	tests := []struct {
		name    string
		command CreateWebhookCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateWebhookCommand{
				OrganizationID: "org-1",
				Name:           "test-webhook",
				URL:            "https://example.com/hook",
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateWebhookCommand{
				Name: "test-webhook",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			command: CreateWebhookCommand{
				OrganizationID: "org-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateWebhook(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateWebhook() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreateWebhook_Success(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "019test-wh-id"},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	_, err := svc.CreateWebhook(context.Background(), CreateWebhookCommand{
		OrganizationID: "org-1",
		Name:           "test-webhook",
		DisplayName:    "Test Webhook",
		URL:            "https://example.com/webhook",
		Events:         []string{"user.created", "user.updated"},
	})
	if err != nil {
		t.Fatalf("CreateWebhook() error = %v", err)
	}
	if len(repo.webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(repo.webhooks))
	}
	if repo.webhooks[0].ID != "019test-wh-id" {
		t.Errorf("ID = %q, want %q", repo.webhooks[0].ID, "019test-wh-id")
	}
	if len(repo.webhooks[0].Events) != 2 {
		t.Errorf("len(Events) = %d, want 2", len(repo.webhooks[0].Events))
	}
	if repo.webhooks[0].Revision != 1 {
		t.Errorf("Revision = %d, want 1", repo.webhooks[0].Revision)
	}
}

func TestService_GetWebhook(t *testing.T) {
	now := time.Now()
	repo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-1", Name: "test-webhook", DisplayName: "Test", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}}}

	detail, err := svc.GetWebhook(context.Background(), "wh-1", "org-1")
	if err != nil {
		t.Fatalf("GetWebhook() error = %v", err)
	}
	if detail.ID != "wh-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "wh-1")
	}
}

func TestService_ListWebhooks(t *testing.T) {
	now := time.Now()
	repo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", Name: "webhook-1", CreatedAt: now, UpdatedAt: now},
			{ID: "wh-2", Name: "webhook-2", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}}}

	items, total, err := svc.ListWebhooks(context.Background(), ListWebhooksQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListWebhooks() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

func TestService_DeleteWebhook(t *testing.T) {
	repo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-1", Name: "test"},
		},
	}
	svc := Service{UOW: testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}}}

	err := svc.DeleteWebhook(context.Background(), "wh-1", "org-1")
	if err != nil {
		t.Fatalf("DeleteWebhook() error = %v", err)
	}
	if len(repo.webhooks) != 0 {
		t.Errorf("expected 0 webhooks, got %d", len(repo.webhooks))
	}
}

func TestService_PatchWebhook_RevisionMismatch(t *testing.T) {
	repo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-1", Name: "test", Revision: 2},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}},
	}

	err := svc.PatchWebhook(context.Background(), "org-1", PatchWebhookCommand{
		ID:       "wh-1",
		Revision: 1,
	})
	if err == nil {
		t.Error("expected error for revision mismatch, got nil")
	}
}

// captureOutboxRepo records enqueued outbox rows so replay's re-enqueue
// becomes observable in the hand-written test UOW.
type captureOutboxRepo struct {
	enqueued []*domainoutbox.OutboxEvent
}

func (r *captureOutboxRepo) Enqueue(ctx context.Context, item *domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, item)
	return nil
}

func (r *captureOutboxRepo) EnqueueBatch(ctx context.Context, items []*domainoutbox.OutboxEvent) error {
	r.enqueued = append(r.enqueued, items...)
	return nil
}

func TestService_ReplayWebhookEvent(t *testing.T) {
	now := time.Now()
	eventRepo := &mockEventRepo{
		events: []*eventdomain.WebhookEvent{
			{ID: "evt-1", WebhookID: "wh-1", EventType: "user.created", Payload: json.RawMessage(`{"id":"u1"}`), Status: "failed", RetryCount: 3, CreatedAt: now, UpdatedAt: now},
		},
	}
	webhookRepo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-1", URL: "https://example.com/hook", Secret: "s3cret", RetryCount: 2},
		},
	}
	outboxRepo := &captureOutboxRepo{}
	svc := Service{
		UUID: fakeUUIDGenerator{fixedID: "replay-outbox-1"},
		UOW:  testUnitOfWork{webhookRepo: webhookRepo, eventRepo: eventRepo, outboxRepo: outboxRepo},
	}

	err := svc.ReplayWebhookEvent(platformScopeContext(), "evt-1", "")
	if err != nil {
		t.Fatalf("ReplayWebhookEvent() error = %v", err)
	}
	if eventRepo.events[0].Status != "pending" {
		t.Errorf("Status = %q, want %q", eventRepo.events[0].Status, "pending")
	}
	if eventRepo.events[0].RetryCount != 0 {
		t.Errorf("RetryCount = %d, want 0", eventRepo.events[0].RetryCount)
	}
	// H40：重放必须同事务入队一条新 outbox 行，否则台账重置后无人投递。
	// 审计嵌入还会写一条 WebhookEvent 审计行，按 aggregate 过滤投递行。
	var deliveries []*domainoutbox.OutboxEvent
	for _, ov := range outboxRepo.enqueued {
		if ov.AggregateType == "webhook" {
			deliveries = append(deliveries, ov)
		}
	}
	if len(deliveries) != 1 {
		t.Fatalf("webhook deliveries = %d, want 1", len(deliveries))
	}
	ov := deliveries[0]
	if ov.ID != "replay-outbox-1" || ov.AggregateType != "webhook" || ov.AggregateID != "wh-1" {
		t.Errorf("outbox row = %+v, want new webhook delivery for wh-1", ov)
	}
	if ov.MaxAttempts != 2 {
		t.Errorf("MaxAttempts = %d, want 2 (webhook.RetryCount)", ov.MaxAttempts)
	}
	if ov.OrganizationID == nil || *ov.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %v, want org-1", ov.OrganizationID)
	}
	var hdrs map[string]any
	if err := json.Unmarshal(ov.Headers, &hdrs); err != nil {
		t.Fatalf("headers unmarshal: %v", err)
	}
	if hdrs["event_id"] != "evt-1" {
		t.Errorf("headers.event_id = %v, want evt-1 (ledger row)", hdrs["event_id"])
	}
}

func TestService_DeleteWebhookEvent(t *testing.T) {
	eventRepo := &mockEventRepo{
		events: []*eventdomain.WebhookEvent{
			{ID: "evt-1"},
		},
	}
	svc := Service{UOW: testUnitOfWork{webhookRepo: &mockWebhookRepo{}, eventRepo: eventRepo}}

	err := svc.DeleteWebhookEvent(platformScopeContext(), "evt-1", "")
	if err != nil {
		t.Fatalf("DeleteWebhookEvent() error = %v", err)
	}
	if len(eventRepo.events) != 0 {
		t.Errorf("expected 0 events, got %d", len(eventRepo.events))
	}
}

func TestService_GetWebhook_NotFound(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{UOW: testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}}}

	_, err := svc.GetWebhook(context.Background(), "nonexistent", "org-1")
	if err == nil {
		t.Error("expected error for nonexistent webhook, got nil")
	}
}

func TestService_DeleteWebhook_NotFound(t *testing.T) {
	repo := &mockWebhookRepo{}
	svc := Service{UOW: testUnitOfWork{webhookRepo: repo, eventRepo: &mockEventRepo{}}}

	err := svc.DeleteWebhook(context.Background(), "nonexistent", "org-1")
	if err == nil {
		t.Error("expected error for nonexistent webhook, got nil")
	}
}

func TestService_DeleteWebhookEvent_NotFound(t *testing.T) {
	eventRepo := &mockEventRepo{}
	svc := Service{UOW: testUnitOfWork{webhookRepo: &mockWebhookRepo{}, eventRepo: eventRepo}}

	err := svc.DeleteWebhookEvent(platformScopeContext(), "nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent event, got nil")
	}
}

func TestService_ReplayWebhookEvent_NotFailed(t *testing.T) {
	now := time.Now()
	eventRepo := &mockEventRepo{
		events: []*eventdomain.WebhookEvent{
			{ID: "evt-1", WebhookID: "wh-1", Status: "delivered", RetryCount: 0, CreatedAt: now, UpdatedAt: now},
		},
	}
	webhookRepo := &mockWebhookRepo{webhooks: []*domain.Webhook{{ID: "wh-1", OrganizationID: "org-1"}}}
	svc := Service{
		UUID: fakeUUIDGenerator{fixedID: "replay-outbox-1"},
		UOW:  testUnitOfWork{webhookRepo: webhookRepo, eventRepo: eventRepo, outboxRepo: &captureOutboxRepo{}},
	}

	// ReplayWebhookEvent does not check current state - it always resets to pending
	err := svc.ReplayWebhookEvent(platformScopeContext(), "evt-1", "")
	if err != nil {
		t.Fatalf("ReplayWebhookEvent() error = %v", err)
	}
	if eventRepo.events[0].Status != "pending" {
		t.Errorf("Status = %q, want %q", eventRepo.events[0].Status, "pending")
	}
}

// H40：父配置已删除的事件不可重放 —— 没有可投递的目标，返回 not found。
func TestService_ReplayWebhookEvent_ParentWebhookGone(t *testing.T) {
	now := time.Now()
	eventRepo := &mockEventRepo{
		events: []*eventdomain.WebhookEvent{
			{ID: "evt-1", WebhookID: "wh-gone", Status: "failed", CreatedAt: now, UpdatedAt: now},
		},
	}
	outboxRepo := &captureOutboxRepo{}
	svc := Service{
		UUID: fakeUUIDGenerator{fixedID: "replay-outbox-1"},
		UOW:  testUnitOfWork{webhookRepo: &mockWebhookRepo{}, eventRepo: eventRepo, outboxRepo: outboxRepo},
	}

	err := svc.ReplayWebhookEvent(platformScopeContext(), "evt-1", "")
	if err == nil {
		t.Fatal("expected error when parent webhook is gone, got nil")
	}
	if len(outboxRepo.enqueued) != 0 {
		t.Errorf("enqueued = %d, want 0", len(outboxRepo.enqueued))
	}
	if eventRepo.events[0].Status != "failed" {
		t.Errorf("Status = %q, want failed (unchanged)", eventRepo.events[0].Status)
	}
}

func TestService_ReplayWebhookEvent_NotFound(t *testing.T) {
	eventRepo := &mockEventRepo{}
	svc := Service{UOW: testUnitOfWork{webhookRepo: &mockWebhookRepo{}, eventRepo: eventRepo}}

	err := svc.ReplayWebhookEvent(platformScopeContext(), "nonexistent", "")
	if err == nil {
		t.Error("expected error for nonexistent event, got nil")
	}
}
