package subscriptions

import (
	"context"
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
	webhookeventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	webhookdomain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type fakeClock struct{ fixedTime time.Time }

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct{ fixedID string }

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

type mockSubscriptionRepo struct {
	subscriptions []*subscriptiondomain.Subscription
}

func (r *mockSubscriptionRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*subscriptiondomain.Subscription, error) {
	for _, item := range r.subscriptions {
		if item.ID == id && item.OrganizationID == organizationID {
			return item, nil
		}
	}
	return nil, apperrors.NewNotFound("subscription not found")
}
func (r *mockSubscriptionRepo) List(_ context.Context, _ subscriptiondomain.ListQuery) ([]*subscriptiondomain.Subscription, int64, error) {
	return r.subscriptions, int64(len(r.subscriptions)), nil
}
func (r *mockSubscriptionRepo) Create(_ context.Context, item *subscriptiondomain.Subscription) error {
	r.subscriptions = append(r.subscriptions, item)
	return nil
}
func (r *mockSubscriptionRepo) Update(_ context.Context, item *subscriptiondomain.Subscription) error {
	for i, x := range r.subscriptions {
		if x.ID == item.ID {
			r.subscriptions[i] = item
			return nil
		}
	}
	return apperrors.NewNotFound("subscription not found")
}
func (r *mockSubscriptionRepo) Delete(_ context.Context, id, organizationID string) error {
	for i, x := range r.subscriptions {
		if x.ID != id {
			continue
		}
		if organizationID != "" && x.OrganizationID != organizationID {
			break
		}
		r.subscriptions = append(r.subscriptions[:i], r.subscriptions[i+1:]...)
		return nil
	}
	return apperrors.NewNotFound("subscription not found")
}

type testUnitOfWork struct{ subscriptionRepo *mockSubscriptionRepo }

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) Orders() orderdomain.Repository                               { return nil }
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
func (u testUnitOfWork) Payments() paymentdomain.Repository                           { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository                                 { return nil }

func (u testUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u testUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u testUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u testUnitOfWork) Pricings() pricingdomain.Repository                               { return nil }
func (u testUnitOfWork) Products() productdomain.Repository                               { return nil }
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return nil }
func (u testUnitOfWork) Resources() resourcedomain.Repository                             { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                                     { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                                 { return nil }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                               { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository                     { return u.subscriptionRepo }
func (u testUnitOfWork) Tickets() ticketdomain.Repository                                 { return nil }
func (u testUnitOfWork) Tokens() tokendomain.Repository                                   { return nil }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository                               { return nil }
func (u testUnitOfWork) Users() userdomain.Repository                                     { return nil }
func (u testUnitOfWork) Coupons() coupondomain.Repository                                 { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository                 { return nil }
func (u testUnitOfWork) Entries() entrydomain.Repository                                  { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository                           { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository             { return nil }
func (u testUnitOfWork) Verifications() verificationsdomain.Repository                    { return nil }
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository                   { return nil }
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository                 { return nil }
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository                     { return nil }
func (u testUnitOfWork) Rules() ruledomain.Repository                                     { return nil }
func (u testUnitOfWork) Transactions() transactiondomain.Repository                       { return nil }
func (u testUnitOfWork) WebhookEvents() webhookeventdomain.Repository                     { return nil }
func (u testUnitOfWork) Keys() keysdomain.Repository                                      { return nil }
func (u testUnitOfWork) QRCodeSessions() qrcodesessions.Repository                        { return nil }
func (u testUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository                      { return nil }

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

func (u testUnitOfWork) Outbox() domainoutbox.Repository { return nil }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

func newSvc(repo *mockSubscriptionRepo) Service {
	return Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: fakeUUIDGenerator{fixedID: "test-id"}, UOW: testUnitOfWork{subscriptionRepo: repo}}
}

func TestCreateSubscription_Validation(t *testing.T) {
	svc := newSvc(&mockSubscriptionRepo{})
	tests := []struct {
		name    string
		cmd     CreateSubscriptionCommand
		wantErr bool
	}{
		{name: "valid", cmd: CreateSubscriptionCommand{OrganizationID: "org-1", Name: "s-1"}, wantErr: false},
		{name: "missing orgId", cmd: CreateSubscriptionCommand{Name: "x"}, wantErr: true},
		{name: "missing name", cmd: CreateSubscriptionCommand{OrganizationID: "org-1"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateSubscription(context.Background(), tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateSubscription_Success(t *testing.T) {
	repo := &mockSubscriptionRepo{}
	svc := newSvc(repo)
	err := svc.CreateSubscription(context.Background(), CreateSubscriptionCommand{OrganizationID: "org-1", Name: "s-1", DisplayName: "Test"})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(repo.subscriptions) != 1 {
		t.Fatalf("expected 1, got %d", len(repo.subscriptions))
	}
	if repo.subscriptions[0].ID != "test-id" {
		t.Errorf("ID = %q", repo.subscriptions[0].ID)
	}
	if repo.subscriptions[0].Revision != 1 {
		t.Errorf("Revision = %d", repo.subscriptions[0].Revision)
	}
}

func TestGetSubscription(t *testing.T) {
	now := time.Now()
	repo := &mockSubscriptionRepo{subscriptions: []*subscriptiondomain.Subscription{{ID: "x-1", OrganizationID: "org-1", Name: "test", CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	d, err := svc.GetSubscription(context.Background(), "org-1", "x-1")
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if d.ID != "x-1" {
		t.Errorf("ID = %q", d.ID)
	}
}

func TestListSubscriptions(t *testing.T) {
	now := time.Now()
	repo := &mockSubscriptionRepo{subscriptions: []*subscriptiondomain.Subscription{
		{ID: "x-1", CreatedAt: now, UpdatedAt: now},
		{ID: "x-2", CreatedAt: now, UpdatedAt: now},
	}}
	svc := newSvc(repo)
	items, total, err := svc.ListSubscriptions(context.Background(), ListSubscriptionsQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d", total)
	}
	if len(items) != 2 {
		t.Errorf("len = %d", len(items))
	}
}

func TestDeleteSubscription(t *testing.T) {
	repo := &mockSubscriptionRepo{subscriptions: []*subscriptiondomain.Subscription{{ID: "x-1", OrganizationID: "org-1"}}}
	svc := newSvc(repo)
	if err := svc.DeleteSubscription(context.Background(), "org-1", "x-1"); err != nil {
		t.Fatalf("error = %v", err)
	}
	if len(repo.subscriptions) != 0 {
		t.Errorf("expected 0, got %d", len(repo.subscriptions))
	}
}

func TestPatchSubscription_RevisionMismatch(t *testing.T) {
	repo := &mockSubscriptionRepo{subscriptions: []*subscriptiondomain.Subscription{{ID: "x-1", OrganizationID: "org-1", Revision: 2}}}
	svc := newSvc(repo)
	err := svc.PatchSubscription(context.Background(), "org-1", PatchSubscriptionCommand{ID: "x-1", Revision: 1})
	if err == nil {
		t.Error("expected error")
	}
}

func TestPatchSubscription_Success(t *testing.T) {
	now := time.Now()
	repo := &mockSubscriptionRepo{subscriptions: []*subscriptiondomain.Subscription{{ID: "x-1", OrganizationID: "org-1", Name: "test", State: subscriptiondomain.StateActive, Revision: 1, CreatedAt: now, UpdatedAt: now}}}
	svc := newSvc(repo)
	displayName := "Updated"
	err := svc.PatchSubscription(context.Background(), "org-1", PatchSubscriptionCommand{ID: "x-1", DisplayName: &displayName, Revision: 1})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if repo.subscriptions[0].DisplayName != "Updated" {
		t.Errorf("DisplayName = %q", repo.subscriptions[0].DisplayName)
	}
	if repo.subscriptions[0].Revision != 2 {
		t.Errorf("Revision = %d", repo.subscriptions[0].Revision)
	}
}
