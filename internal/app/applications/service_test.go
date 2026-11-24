package applications

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
// Fake platform implementations
// ---------------------------------------------------------------------------

type fakeClock struct {
	fixedTime time.Time
}

func (c fakeClock) Now() time.Time { return c.fixedTime }

type fakeUUIDGenerator struct {
	fixedID string
	callNum int
	altID   string
}

func (g *fakeUUIDGenerator) NewString() (string, error) {
	g.callNum++
	if g.callNum > 1 && g.altID != "" {
		return g.altID, nil
	}
	return g.fixedID, nil
}

// ---------------------------------------------------------------------------
// Mock application repository
// ---------------------------------------------------------------------------

type mockApplicationRepo struct {
	apps                        []*domain.Application
	getByOrganizationAndSlugErr error
	scopedGetCalls              int
	// registrationHashes models the narrow registration_access_token_hash write:
	// it is deliberately NOT a field of the application row, mirroring the real
	// schema where the value never travels through the 114-column statements.
	registrationHashes map[string]string
	// jwksURIs models the narrow app_client_jwks_uris side table (OPEN-API-61).
	jwksURIs map[string]string
	// updateCalls / replaceBindingsCalls make "no write happened" observable.
	updateCalls          int
	replaceBindingsCalls int
}

func (r *mockApplicationRepo) GetByID(ctx context.Context, id string) (*domain.Application, error) {
	for _, a := range r.apps {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *mockApplicationRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Application, error) {
	r.scopedGetCalls++
	for _, a := range r.apps {
		if a.ID == id && a.OrganizationID == organizationID {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *mockApplicationRepo) GetByClientID(ctx context.Context, clientID string) (*domain.Application, error) {
	for _, a := range r.apps {
		if a.ClientID == clientID {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found by client ID")
}

func (r *mockApplicationRepo) GetByOrganizationAndSlug(ctx context.Context, orgID, slug string) (*domain.Application, error) {
	if r.getByOrganizationAndSlugErr != nil {
		return nil, r.getByOrganizationAndSlugErr
	}
	for _, a := range r.apps {
		if a.OrganizationID == orgID && a.Slug == slug {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}

func (r *mockApplicationRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Application, int64, error) {
	return r.apps, int64(len(r.apps)), nil
}

func (r *mockApplicationRepo) Create(ctx context.Context, app *domain.Application) error {
	r.apps = append(r.apps, app)
	return nil
}

func (r *mockApplicationRepo) Update(ctx context.Context, app *domain.Application) error {
	r.updateCalls++
	for i, a := range r.apps {
		if a.ID == app.ID {
			r.apps[i] = app
			return nil
		}
	}
	return apperrors.NewNotFound("application not found")
}

func (r *mockApplicationRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, a := range r.apps {
		if a.ID == id && a.OrganizationID == organizationID {
			r.apps = append(r.apps[:i], r.apps[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("application not found")
}

func (r *mockApplicationRepo) ReplaceProviderBindings(ctx context.Context, applicationID string, bindings []domain.ApplicationProviderBinding) error {
	r.replaceBindingsCalls++
	return nil
}

func (r *mockApplicationRepo) HasDeletedWithOrgAndSlug(ctx context.Context, organizationID, slug string) (bool, error) {
	return false, nil
}

func (r *mockApplicationRepo) FindHostConflicts(_ context.Context, _, _ string, _ []string) ([]domain.HostConflict, error) {
	return nil, nil
}

func (r *mockApplicationRepo) SetRegistrationAccessToken(_ context.Context, applicationID, tokenHash string) error {
	for _, a := range r.apps {
		if a.ID == applicationID {
			if r.registrationHashes == nil {
				r.registrationHashes = map[string]string{}
			}
			r.registrationHashes[a.ClientID] = tokenHash
			return nil
		}
	}
	return apperrors.NewNotFound("application not found")
}

func (r *mockApplicationRepo) GetRegistrationAccessTokenHash(_ context.Context, clientID string) (string, error) {
	return r.registrationHashes[clientID], nil
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	appRepo *mockApplicationRepo
	records recordsdomain.Repository
}

func (u testUnitOfWork) Outbox() domainoutbox.Repository { return nil }

func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) Applications() domain.Repository                              { return u.appRepo }
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
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return u.records }
func (u testUnitOfWork) Resources() resourcedomain.Repository                             { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                                     { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                                 { return nil }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                               { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository                     { return nil }
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

func (u testUnitOfWork) IDV() idvdomain.Repository { return nil }
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_CreateApplication(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{}
	recordsRepo := &auditWrapperRecords{}
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id-aaaaaa", altID: "client-id-001"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid, UOW: testUnitOfWork{appRepo: repo, records: recordsRepo}}
	ctx := context.Background()
	const actorID = "019operator-0000-7000-8000-000000000002"
	created, err := svc.CreateApplication(ctx, CreateApplicationCommand{
		OrganizationID: "org-001", ActorID: actorID, Slug: "my-app", DisplayName: "My Application",
		Category: "web", AppType: "native", Status: "active",
		ClientID:     "", // Empty → must auto-generate
		RedirectURIs: []string{"https://example.com/callback"},
		GrantTypes:   []string{"authorization_code"},
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	app := created.Application
	if app.ID != "019app-test-id-aaaaaa" {
		t.Errorf("expected ID '019app-test-id-aaaaaa', got %q", app.ID)
	}
	if app.ClientID == "" {
		t.Error("expected ClientID to be auto-generated, got empty string")
	}
	if app.ClientID == "client-id-001" {
		t.Errorf("expected ClientID auto-generated with gl_cid_ prefix, got frontend-supplied value %q", app.ClientID)
	}
	if app.Slug != "my-app" {
		t.Errorf("expected Slug 'my-app', got %q", app.Slug)
	}
	if len(repo.apps) != 1 {
		t.Fatalf("expected 1 application in repo, got %d", len(repo.apps))
	}
	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	record := recordsRepo.created[0]
	if record.Action != "application.created" {
		t.Errorf("Action = %q, want %q", record.Action, "application.created")
	}
	if record.ActorID != actorID {
		t.Errorf("ActorID = %q, want operator %q", record.ActorID, actorID)
	}
}

func TestService_GetApplication(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", OrganizationID: "org-1", Slug: "test-app", DisplayName: "Test App", CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()
	detail, err := svc.GetApplication(ctx, "org-1", "app-1")
	if err != nil {
		t.Fatalf("GetApplication() error = %v", err)
	}
	if detail.ID != "app-1" {
		t.Errorf("expected ID 'app-1', got %q", detail.ID)
	}
}

func TestService_GetApplication_NotFound(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()
	_, err := svc.GetApplication(ctx, "org-1", "nonexistent")
	if err == nil {
		t.Fatalf("expected error for nonexistent application, got nil")
	}
}

func TestService_DeleteApplication(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", OrganizationID: "org-1", Slug: "test-app", CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()
	err := svc.DeleteApplication(ctx, DeleteApplicationCommand{ID: "app-1", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("DeleteApplication() error = %v", err)
	}
	if len(repo.apps) != 0 {
		t.Errorf("expected 0 applications after delete, got %d", len(repo.apps))
	}
}

func TestService_ListApplications(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", Slug: "app-1", DisplayName: "App 1", CreatedAt: now, UpdatedAt: now},
		{ID: "app-2", Slug: "app-2", DisplayName: "App 2", CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()
	items, total, err := svc.ListApplications(ctx, ListApplicationsQuery{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("ListApplications() error = %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestService_GetApplicationByClientID_Success(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", Slug: "test-app", ClientID: "client-abc", DisplayName: "Test App", CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()
	detail, err := svc.GetApplicationByClientID(ctx, "client-abc")
	if err != nil {
		t.Fatalf("GetApplicationByClientID() error = %v", err)
	}
	if detail.ID != "app-1" {
		t.Errorf("expected ID 'app-1', got %q", detail.ID)
	}
	if detail.ClientID != "client-abc" {
		t.Errorf("expected ClientID 'client-abc', got %q", detail.ClientID)
	}
}

func TestService_GetApplicationByClientID_NotFound(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()
	_, err := svc.GetApplicationByClientID(ctx, "nonexistent-client-id")
	if err == nil {
		t.Fatalf("expected error for nonexistent client ID, got nil")
	}
}

func TestService_DeleteApplication_NotFound(t *testing.T) {
	repo := &mockApplicationRepo{}
	svc := Service{UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()
	err := svc.DeleteApplication(ctx, DeleteApplicationCommand{ID: "nonexistent", OrganizationID: "org-1"})
	if err == nil {
		t.Fatalf("expected error for nonexistent application, got nil")
	}
}

func TestService_PatchApplication_Success(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", OrganizationID: "org-1", Slug: "test-app", DisplayName: "Test App", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()

	newDisplayName := "Updated App"
	err := svc.PatchApplication(ctx, PatchApplicationCommand{
		ID:             "app-1",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
		Revision:       1,
	})
	if err != nil {
		t.Fatalf("PatchApplication() error = %v", err)
	}
	if repo.apps[0].DisplayName != "Updated App" {
		t.Errorf("DisplayName = %q, want %q", repo.apps[0].DisplayName, "Updated App")
	}
	if repo.apps[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.apps[0].Revision)
	}
}

func TestService_PatchApplication_PersistsNestedConfigs(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", OrganizationID: "org-1", Slug: "test-app", DisplayName: "Test App", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()

	displayName := "Updated App"
	err := svc.PatchApplication(ctx, PatchApplicationCommand{
		ID:             "app-1",
		OrganizationID: "org-1",
		Revision:       1,
		DisplayName:    &displayName,
		SignupItems:    json.RawMessage(`[{"name":"Email","type":"Input","visible":true,"required":true,"label":"Email"}]`),
		ProviderItems:  json.RawMessage(`[{"provider":"prov-1","canSignIn":true}]`),
		SigninMethods:  json.RawMessage(`["Password"]`),
		SigninItems:    json.RawMessage(`[{"name":"Username","visible":true}]`),
		SamlAttributes: json.RawMessage(`[{"name":"email","value":"user.email"}]`),
		TokenAttributes: json.RawMessage(
			`[{"name":"role","sourceField":"user.role"}]`,
		),
	})
	if err != nil {
		t.Fatalf("PatchApplication() error = %v", err)
	}

	updated := repo.apps[0]
	if len(updated.SignupItems) != 1 || updated.SignupItems[0].Type != "Input" || !updated.SignupItems[0].Required {
		t.Errorf("SignupItems = %+v, want the submitted row including its type", updated.SignupItems)
	}
	if len(updated.ProviderItems) != 1 || updated.ProviderItems[0].Provider != "prov-1" {
		t.Errorf("ProviderItems = %+v, want the submitted provider row", updated.ProviderItems)
	}
	if len(updated.SigninMethods) != 1 || updated.SigninMethods[0] != "Password" {
		t.Errorf("SigninMethods = %v, want [Password]", updated.SigninMethods)
	}
	if len(updated.SigninItems) != 1 || updated.SigninItems[0].Name != "Username" {
		t.Errorf("SigninItems = %+v, want the submitted signin item", updated.SigninItems)
	}
	if len(updated.SamlAttributes) != 1 || len(updated.TokenAttributes) != 1 {
		t.Errorf("attributes = %+v / %+v, want one each", updated.SamlAttributes, updated.TokenAttributes)
	}
}

func TestService_PatchApplication_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", OrganizationID: "org-1", Slug: "test-app", DisplayName: "Test App", Revision: 5, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()

	newDisplayName := "Updated App"
	err := svc.PatchApplication(ctx, PatchApplicationCommand{
		ID:             "app-1",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
		Revision:       3, // stale
	})
	if err == nil {
		t.Fatal("PatchApplication() expected error for revision mismatch, got nil")
	}
}

func TestService_PatchApplication_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()

	newDisplayName := "Updated App"
	err := svc.PatchApplication(ctx, PatchApplicationCommand{
		ID:             "nonexistent",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
	})
	if err == nil {
		t.Fatal("PatchApplication() expected error for nonexistent application, got nil")
	}
}

func TestService_CreateApplication_DuplicateSlug(t *testing.T) {
	now := time.Now()
	repo := &mockApplicationRepo{apps: []*domain.Application{
		{ID: "app-1", OrganizationID: "org-1", Slug: "my-app", DisplayName: "Existing", CreatedAt: now, UpdatedAt: now},
	}}
	uuid := &fakeUUIDGenerator{fixedID: "019app-test-id-aaaaaa", altID: "client-id-001"}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: uuid, UOW: testUnitOfWork{appRepo: repo}}
	ctx := context.Background()

	_, err := svc.CreateApplication(ctx, CreateApplicationCommand{
		OrganizationID: "org-1",
		Slug:           "my-app",
		DisplayName:    "Duplicate",
		Category:       "web",
		Status:         "active",
		ClientID:       "client-id-002",
	})
	if err == nil {
		t.Fatal("CreateApplication() expected error for duplicate slug, got nil")
	}
}
