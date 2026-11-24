package applications

import (
	"context"
	"errors"
	"testing"
	"time"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	adapterdomain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	agentdomain "github.com/guliangiam/guliangiam/internal/domain/agents"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
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

// ---------------------------------------------------------------------------
// Mock org repo for applications tests
// ---------------------------------------------------------------------------

type appTestOrgRepo struct {
	orgs         []*orgdomain.Organization
	getBySlugErr error
}

func (r *appTestOrgRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	for _, o := range r.orgs {
		if o.ID == id {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *appTestOrgRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	if r.getBySlugErr != nil {
		return nil, r.getBySlugErr
	}
	for _, o := range r.orgs {
		if o.Slug == slug {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("organization not found")
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *appTestOrgRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *appTestOrgRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, nil
}
func (r *appTestOrgRepo) Create(ctx context.Context, org *orgdomain.Organization) error { return nil }
func (r *appTestOrgRepo) Update(ctx context.Context, org *orgdomain.Organization) error { return nil }
func (r *appTestOrgRepo) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	return nil
}
func (r *appTestOrgRepo) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	return nil
}
func (r *appTestOrgRepo) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	return nil
}
func (r *appTestOrgRepo) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}
func (r *appTestOrgRepo) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// appTestProviderRepo is the tenant-aware provider lookup used by the
// provider-binding validation: a foreign or missing id is NotFound.
type appTestProviderRepo struct {
	providers []*providerdomain.Provider
}

func (r *appTestProviderRepo) GetByID(_ context.Context, id string) (*providerdomain.Provider, error) {
	for _, p := range r.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *appTestProviderRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*providerdomain.Provider, error) {
	for _, p := range r.providers {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *appTestProviderRepo) List(_ context.Context, _ providerdomain.ListQuery) ([]*providerdomain.Provider, int64, error) {
	return nil, 0, nil
}
func (r *appTestProviderRepo) Create(_ context.Context, _ *providerdomain.Provider) error { return nil }
func (r *appTestProviderRepo) Update(_ context.Context, _ *providerdomain.Provider) error { return nil }
func (r *appTestProviderRepo) Delete(_ context.Context, _ string) error                   { return nil }
func (r *appTestProviderRepo) GetConfig(_ context.Context, _ string) (*providerdomain.ProviderConfig, error) {
	return nil, nil
}
func (r *appTestProviderRepo) UpdateConfig(_ context.Context, _ *providerdomain.ProviderConfig) error {
	return nil
}
func (r *appTestProviderRepo) ListSecretStatuses(_ context.Context, _ string) ([]providerdomain.ProviderSecretStatus, error) {
	return nil, nil
}

// appTestUOW includes both app and org repos
type appTestUOW struct {
	appRepo              *mockApplicationRepo
	orgRepo              *appTestOrgRepo
	providerRepo         *appTestProviderRepo
	withinTransactionRan *bool
}

func (u appTestUOW) Applications() domain.Repository                              { return u.appRepo }
func (u appTestUOW) Organizations() orgdomain.Repository                          { return u.orgRepo }
func (u appTestUOW) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u appTestUOW) Consents() consentdomain.Repository                           { return nil }
func (u appTestUOW) Gateway() gatewaydomain.Repository                            { return nil }
func (u appTestUOW) Groups() groupsdomain.Repository                              { return nil }
func (u appTestUOW) Invitations() invitationsdomain.Repository                    { return nil }
func (u appTestUOW) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u appTestUOW) Permissions() permissiondomain.Repository                     { return nil }
func (u appTestUOW) Providers() providerdomain.Repository                         { return u.providerRepo }
func (u appTestUOW) Certs() certdomain.Repository                                 { return nil }
func (u appTestUOW) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u appTestUOW) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u appTestUOW) Notifications() notificationsdomain.Repository                { return nil }
func (u appTestUOW) Ldaps() ldapdomain.Repository                                 { return nil }
func (u appTestUOW) Models() modeldomain.Repository                               { return nil }
func (u appTestUOW) Adapters() adapterdomain.Repository                           { return nil }
func (u appTestUOW) Agents() agentdomain.Repository                               { return nil }
func (u appTestUOW) Enforcers() enforcerdomain.Repository                         { return nil }
func (u appTestUOW) Forms() formdomain.Repository                                 { return nil }
func (u appTestUOW) Orders() orderdomain.Repository                               { return nil }
func (u appTestUOW) Payments() paymentdomain.Repository                           { return nil }
func (u appTestUOW) Plans() plandomain.Repository                                 { return nil }

func (u appTestUOW) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u appTestUOW) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u appTestUOW) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u appTestUOW) Pricings() pricingdomain.Repository                               { return nil }
func (u appTestUOW) Products() productdomain.Repository                               { return nil }
func (u appTestUOW) Records() recordsdomain.Repository                                { return nil }
func (u appTestUOW) Resources() resourcedomain.Repository                             { return nil }
func (u appTestUOW) Roles() roledomain.Repository                                     { return nil }
func (u appTestUOW) Syncers() syncerdomain.Repository                                 { return nil }
func (u appTestUOW) Sessions() sessiondomain.Repository                               { return nil }
func (u appTestUOW) Subscriptions() subscriptiondomain.Repository                     { return nil }
func (u appTestUOW) Tickets() ticketdomain.Repository                                 { return nil }
func (u appTestUOW) Tokens() tokendomain.Repository                                   { return nil }
func (u appTestUOW) Webhooks() webhookdomain.Repository                               { return nil }
func (u appTestUOW) Users() userdomain.Repository                                     { return nil }
func (u appTestUOW) Coupons() coupondomain.Repository                                 { return nil }
func (u appTestUOW) CouponUsages() coupondomain.CouponUsageRepository                 { return nil }
func (u appTestUOW) Entries() entrydomain.Repository                                  { return nil }
func (u appTestUOW) McpServers() mcpserverdomain.Repository                           { return nil }
func (u appTestUOW) RadiusAccountings() radiusaccountingdomain.Repository             { return nil }
func (u appTestUOW) Verifications() verificationsdomain.Repository                    { return nil }
func (u appTestUOW) WebAuthnCredentials() webauthndomain.Repository                   { return nil }
func (u appTestUOW) FaceIdProviders() faceidproviderdomain.Repository                 { return nil }
func (u appTestUOW) RadiusConfigs() radiusconfigdomain.Repository                     { return nil }
func (u appTestUOW) Rules() ruledomain.Repository                                     { return nil }
func (u appTestUOW) Transactions() transactiondomain.Repository                       { return nil }
func (u appTestUOW) WebhookEvents() webhookeventdomain.Repository                     { return nil }
func (u appTestUOW) Keys() keysdomain.Repository                                      { return nil }
func (u appTestUOW) QRCodeSessions() qrcodesessions.Repository                        { return nil }
func (u appTestUOW) DeviceAuthCodes() deviceauthcodes.Repository                      { return nil }

func (u appTestUOW) Web3Identities() web3identity.Repository     { return nil }
func (u appTestUOW) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u appTestUOW) CASTickets() castickets.Repository           { return nil }
func (u appTestUOW) KerberosTickets() kerberostickets.Repository { return nil }
func (u appTestUOW) LockoutAttempts() lockoutattempts.Repository { return nil }
func (u appTestUOW) IDV() idvdomain.Repository                   { return nil }
func (u appTestUOW) IGA() igadomain.Repository                   { return nil }

func (u appTestUOW) Workload() workloaddomain.Repository { return nil }

func (u appTestUOW) Rebac() rebac.Repository { return nil }

func (u appTestUOW) Risk() riskdomain.Repository { return nil }

func (u appTestUOW) Actions() actionsdomain.Repository { return nil }

func (u appTestUOW) Metering() meteringdomain.Repository { return nil }

func (u appTestUOW) Outbox() domainoutbox.Repository { return nil }

func (u appTestUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	if u.withinTransactionRan != nil {
		*u.withinTransactionRan = true
	}
	return fn(ctx, u)
}

// ---------------------------------------------------------------------------
// NewService test
// ---------------------------------------------------------------------------

func TestNewService_Applications(t *testing.T) {
	svc := NewService(fakeClock{}, &fakeUUIDGenerator{fixedID: "test"}, appTestUOW{})
	if svc.Clock == nil {
		t.Error("Clock should not be nil")
	}
	if svc.UUID == nil {
		t.Error("UUID should not be nil")
	}
	if svc.UOW == nil {
		t.Error("UOW should not be nil")
	}
}

// ---------------------------------------------------------------------------
// GetApplicationByOrgAndSlug tests
// ---------------------------------------------------------------------------

func TestService_GetApplicationByOrgAndSlug_Success(t *testing.T) {
	withinTransactionRan := false
	now := time.Now()
	appRepo := &mockApplicationRepo{
		apps: []*domain.Application{
			{ID: "app-1", OrganizationID: "org-1", Slug: "my-app", DisplayName: "My App", CreatedAt: now, UpdatedAt: now},
		},
	}
	orgRepo := &appTestOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}

	svc := Service{UOW: appTestUOW{
		appRepo:              appRepo,
		orgRepo:              orgRepo,
		withinTransactionRan: &withinTransactionRan,
	}}

	detail, err := svc.GetApplicationByOrgAndSlug(context.Background(), "org-slug", "my-app")
	if err != nil {
		t.Fatalf("GetApplicationByOrgAndSlug() error = %v", err)
	}
	if detail.ID != "app-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "app-1")
	}
	if detail.Slug != "my-app" {
		t.Errorf("Slug = %q, want %q", detail.Slug, "my-app")
	}
	if !withinTransactionRan {
		t.Fatal("GetApplicationByOrgAndSlug must resolve through a transaction so RLS context is applied")
	}
}

func TestService_GetApplicationByOrgAndSlug_EmptySlugs(t *testing.T) {
	svc := Service{UOW: appTestUOW{}}

	_, err := svc.GetApplicationByOrgAndSlug(context.Background(), "", "my-app")
	if err == nil {
		t.Fatal("expected error for empty org slug, got nil")
	}

	_, err = svc.GetApplicationByOrgAndSlug(context.Background(), "org-slug", "")
	if err == nil {
		t.Fatal("expected error for empty app slug, got nil")
	}
}

func TestService_GetApplicationByOrgAndSlug_OrgNotFound(t *testing.T) {
	orgRepo := &appTestOrgRepo{} // no orgs
	svc := Service{UOW: appTestUOW{orgRepo: orgRepo}}

	_, err := svc.GetApplicationByOrgAndSlug(context.Background(), "nonexistent", "my-app")
	if err == nil {
		t.Fatal("expected error when org not found, got nil")
	}
}

func TestService_GetApplicationByOrgAndSlug_AppNotFound(t *testing.T) {
	orgRepo := &appTestOrgRepo{
		orgs: []*orgdomain.Organization{
			{ID: "org-1", Slug: "org-slug"},
		},
	}

	appRepo := &mockApplicationRepo{} // no apps
	svc := Service{UOW: appTestUOW{appRepo: appRepo, orgRepo: orgRepo}}

	_, err := svc.GetApplicationByOrgAndSlug(context.Background(), "org-slug", "nonexistent")
	if err == nil {
		t.Fatal("expected error when app not found, got nil")
	}
}

func TestService_GetApplicationByOrgAndSlug_PropagatesOrganizationLookupErrors(t *testing.T) {
	dbErr := errors.New("database unavailable")
	svc := Service{UOW: appTestUOW{orgRepo: &appTestOrgRepo{getBySlugErr: dbErr}}}

	_, err := svc.GetApplicationByOrgAndSlug(context.Background(), "org-slug", "my-app")
	if !errors.Is(err, dbErr) {
		t.Fatalf("GetApplicationByOrgAndSlug() error = %v, want wrapped database error", err)
	}
}

func TestService_GetApplicationByOrgAndSlug_PropagatesApplicationLookupErrors(t *testing.T) {
	dbErr := errors.New("database unavailable")
	svc := Service{UOW: appTestUOW{
		orgRepo: &appTestOrgRepo{orgs: []*orgdomain.Organization{{ID: "org-1", Slug: "org-slug"}}},
		appRepo: &mockApplicationRepo{getByOrganizationAndSlugErr: dbErr},
	}}

	_, err := svc.GetApplicationByOrgAndSlug(context.Background(), "org-slug", "my-app")
	if !errors.Is(err, dbErr) {
		t.Fatalf("GetApplicationByOrgAndSlug() error = %v, want wrapped database error", err)
	}
}

// ---------------------------------------------------------------------------
// ReplaceProviderBindings tests
// ---------------------------------------------------------------------------

func TestService_ReplaceProviderBindings_Success(t *testing.T) {
	now := time.Now()
	appRepo := &mockApplicationRepo{
		apps: []*domain.Application{
			{ID: "app-1", OrganizationID: "org-1", Slug: "my-app", CreatedAt: now, UpdatedAt: now},
		},
	}
	providerRepo := &appTestProviderRepo{providers: []*providerdomain.Provider{
		{ID: "provider-1", OrganizationID: "org-1"},
		{ID: "provider-2", OrganizationID: "org-1"},
	}}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   appTestUOW{appRepo: appRepo, providerRepo: providerRepo},
	}

	err := svc.ReplaceProviderBindings(context.Background(), "org-1", "app-1", "", []ReplaceApplicationProviderBindingCommand{
		{ProviderID: "provider-1"},
		{ProviderID: "provider-2"},
	})
	if err != nil {
		t.Fatalf("ReplaceProviderBindings() error = %v", err)
	}
	if appRepo.updateCalls != 1 || appRepo.replaceBindingsCalls != 1 {
		t.Fatalf("expected one update and one replace, got update=%d replace=%d",
			appRepo.updateCalls, appRepo.replaceBindingsCalls)
	}
}

// TestService_ReplaceProviderBindings_UnknownProviderRejected pins that a
// providerId outside the tenant is a client error (422), not a foreign-key
// failure bubbling up as 500, and that the batch writes nothing.
func TestService_ReplaceProviderBindings_UnknownProviderRejected(t *testing.T) {
	now := time.Now()
	appRepo := &mockApplicationRepo{
		apps: []*domain.Application{
			{ID: "app-1", OrganizationID: "org-1", Slug: "my-app", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	providerRepo := &appTestProviderRepo{providers: []*providerdomain.Provider{
		{ID: "provider-1", OrganizationID: "org-1"},
	}}

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   appTestUOW{appRepo: appRepo, providerRepo: providerRepo},
	}

	const missingProviderID = "00000000-0000-0000-0000-000000000000"
	err := svc.ReplaceProviderBindings(context.Background(), "org-1", "app-1", "", []ReplaceApplicationProviderBindingCommand{
		{ProviderID: "provider-1"},
		{ProviderID: missingProviderID},
	})
	if err == nil {
		t.Fatal("expected error when provider does not exist, got nil")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("error code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
	}
	if appErr.Meta["index"] != 1 || appErr.Meta["providerId"] != missingProviderID {
		t.Fatalf("unexpected meta: %#v", appErr.Meta)
	}
	if appRepo.updateCalls != 0 || appRepo.replaceBindingsCalls != 0 {
		t.Fatalf("no write expected, got update=%d replace=%d",
			appRepo.updateCalls, appRepo.replaceBindingsCalls)
	}
}

func TestService_ReplaceProviderBindings_AppNotFound(t *testing.T) {
	appRepo := &mockApplicationRepo{} // no apps
	svc := Service{
		Clock: fakeClock{},
		UOW:   appTestUOW{appRepo: appRepo},
	}

	err := svc.ReplaceProviderBindings(context.Background(), "org-1", "nonexistent", "", []ReplaceApplicationProviderBindingCommand{
		{ProviderID: "provider-1"},
	})
	if err == nil {
		t.Fatal("expected error when app not found, got nil")
	}
}
