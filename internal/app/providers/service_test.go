package providers

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
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
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
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
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
// Mock provider repository
// ---------------------------------------------------------------------------

type mockProviderRepo struct {
	providers    []*domain.Provider
	configs      []*domain.ProviderConfig
	secretStatus []domain.ProviderSecretStatus
	createErr    error
	updateErr    error
	rotationErr  error
}

func (r *mockProviderRepo) GetByID(ctx context.Context, id string) (*domain.Provider, error) {
	for _, p := range r.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *mockProviderRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Provider, error) {
	for _, p := range r.providers {
		if p.ID == id && p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("provider not found")
}

func (r *mockProviderRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Provider, int64, error) {
	return r.providers, int64(len(r.providers)), nil
}

func (r *mockProviderRepo) Create(ctx context.Context, provider *domain.Provider) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.providers = append(r.providers, provider)
	return nil
}

func (r *mockProviderRepo) Update(ctx context.Context, provider *domain.Provider) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	for i, p := range r.providers {
		if p.ID == provider.ID {
			r.providers[i] = provider
			return nil
		}
	}
	return apperrors.NewNotFound("provider not found")
}

func (r *mockProviderRepo) Delete(ctx context.Context, id string) error {
	for i, p := range r.providers {
		if p.ID == id {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("provider not found")
}

func (r *mockProviderRepo) GetConfig(ctx context.Context, providerID string) (*domain.ProviderConfig, error) {
	for _, c := range r.configs {
		if c.ProviderID == providerID && c.ConfigType != registryConfigType {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("config not found")
}

func (r *mockProviderRepo) GetConfigByType(ctx context.Context, providerID, configType string) (*domain.ProviderConfig, error) {
	for _, c := range r.configs {
		if c.ProviderID == providerID && c.ConfigType == configType {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("config not found")
}

func (r *mockProviderRepo) ListConfigsByType(ctx context.Context, providerIDs []string, configType string) ([]domain.ProviderConfig, error) {
	items := make([]domain.ProviderConfig, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		for _, c := range r.configs {
			if c.ProviderID == providerID && c.ConfigType == configType {
				items = append(items, *c)
			}
		}
	}
	return items, nil
}

func (r *mockProviderRepo) UpdateConfig(ctx context.Context, config *domain.ProviderConfig) error {
	for i, c := range r.configs {
		if c.ProviderID == config.ProviderID && c.ConfigType == config.ConfigType {
			r.configs[i] = config
			return nil
		}
	}
	r.configs = append(r.configs, config)
	return nil
}

func (r *mockProviderRepo) ListSecretStatuses(ctx context.Context, providerID string) ([]domain.ProviderSecretStatus, error) {
	items := make([]domain.ProviderSecretStatus, 0, len(r.secretStatus))
	for i := range r.secretStatus {
		item := &r.secretStatus[i]
		if item.ProviderID == providerID {
			items = append(items, *item)
		}
	}
	return items, nil
}

func (r *mockProviderRepo) RotateSecretStatus(ctx context.Context, current domain.ProviderSecretStatus, previousStatus string, previousExpiresAt time.Time) error {
	if r.rotationErr != nil {
		return r.rotationErr
	}
	for i := range r.secretStatus {
		item := &r.secretStatus[i]
		if item.ProviderID == current.ProviderID && item.SecretType == current.SecretType && item.Version < current.Version && (item.Status == "active" || item.Status == "grace") {
			item.Status = previousStatus
			item.ExpiresAt = previousExpiresAt
			item.UpdatedAt = current.UpdatedAt
		}
	}
	r.secretStatus = append(r.secretStatus, current)
	return nil
}

func (r *mockProviderRepo) snapshot() func() {
	providers := append([]*domain.Provider(nil), r.providers...)
	configs := append([]*domain.ProviderConfig(nil), r.configs...)
	secretStatus := append([]domain.ProviderSecretStatus(nil), r.secretStatus...)
	return func() {
		r.providers = providers
		r.configs = configs
		r.secretStatus = secretStatus
	}
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	providerRepo *mockProviderRepo
	recordRepo   recordsdomain.Repository
	outboxRepo   domainoutbox.Repository
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
func (u testUnitOfWork) Providers() domain.Repository                                 { return u.providerRepo }
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
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return u.recordRepo }
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
func (u testUnitOfWork) IDV() idvdomain.Repository                   { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository { return nil }

func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) Outbox() domainoutbox.Repository { return u.outboxRepo }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	restores := make([]func(), 0, 3)
	if u.providerRepo != nil {
		restores = append(restores, u.providerRepo.snapshot())
	}
	if repo, ok := u.recordRepo.(interface{ snapshot() func() }); ok {
		restores = append(restores, repo.snapshot())
	}
	if repo, ok := u.outboxRepo.(interface{ snapshot() func() }); ok {
		restores = append(restores, repo.snapshot())
	}
	err := fn(ctx, u)
	if err != nil {
		for i := len(restores) - 1; i >= 0; i-- {
			restores[i]()
		}
	}
	return err
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_NewProvider(t *testing.T) {
	clock := fakeClock{fixedTime: time.Now()}
	uuid := fakeUUIDGenerator{fixedID: "019test-provider-id"}

	svc := Service{Clock: clock, UUID: uuid, UOW: testUnitOfWork{}}

	provider, err := svc.NewProvider(CreateProviderCommand{
		OrganizationID: "org-1",
		Name:           "github",
		DisplayName:    "GitHub",
		Category:       "OAuth",
		ProviderType:   "github",
		ClientID:       "gh-client-id",
		ClientSecret:   "gh-client-secret",
	})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if provider.ID != "019test-provider-id" {
		t.Errorf("ID = %q, want %q", provider.ID, "019test-provider-id")
	}
	if provider.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", provider.OrganizationID, "org-1")
	}
	if provider.Category != domain.CategoryOAuth {
		t.Errorf("Category = %q, want %q", provider.Category, domain.CategoryOAuth)
	}
	if provider.ProviderType != domain.ProviderGitHub {
		t.Errorf("ProviderType = %q, want %q", provider.ProviderType, domain.ProviderGitHub)
	}
	if provider.Revision != 1 {
		t.Errorf("Revision = %d, want 1", provider.Revision)
	}
}

func TestService_CreateProvider_Validation(t *testing.T) {
	tests := []struct {
		name    string
		command CreateProviderCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateProviderCommand{
				OrganizationID: "org-1",
				Name:           "github",
				DisplayName:    "GitHub",
				Category:       "OAuth",
				ProviderType:   "github",
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateProviderCommand{
				Name:         "github",
				DisplayName:  "GitHub",
				Category:     "OAuth",
				ProviderType: "github",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			command: CreateProviderCommand{
				OrganizationID: "org-1",
				DisplayName:    "GitHub",
				Category:       "OAuth",
				ProviderType:   "github",
			},
			wantErr: true,
		},
		{
			name: "missing displayName",
			command: CreateProviderCommand{
				OrganizationID: "org-1",
				Name:           "github",
				Category:       "OAuth",
				ProviderType:   "github",
			},
			wantErr: true,
		},
		{
			name: "missing category",
			command: CreateProviderCommand{
				OrganizationID: "org-1",
				Name:           "github",
				DisplayName:    "GitHub",
				ProviderType:   "github",
			},
			wantErr: true,
		},
		{
			name: "missing providerType",
			command: CreateProviderCommand{
				OrganizationID: "org-1",
				Name:           "github",
				DisplayName:    "GitHub",
				Category:       "OAuth",
			},
			wantErr: true,
		},
		{
			name: "invalid provider URL",
			command: CreateProviderCommand{
				OrganizationID: "org-1",
				Name:           "github",
				DisplayName:    "GitHub",
				Category:       "OAuth",
				ProviderType:   "github",
				ProviderURL:    "not-a-url",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockProviderRepo{}
			svc := Service{
				Clock: fakeClock{fixedTime: time.Now()},
				UUID:  fakeUUIDGenerator{fixedID: "019test-id"},
				UOW:   testUnitOfWork{providerRepo: repo},
			}
			_, err := svc.CreateProvider(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreateProvider_Success(t *testing.T) {
	repo := &mockProviderRepo{}
	uow := testUnitOfWork{providerRepo: repo}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{fixedID: "019test-provider-id"},
		UOW:   uow,
	}

	_, err := svc.CreateProvider(context.Background(), CreateProviderCommand{
		OrganizationID: "org-1",
		Name:           "github",
		DisplayName:    "GitHub",
		Category:       "OAuth",
		ProviderType:   "github",
		ClientID:       "gh-client-id",
		ClientSecret:   "gh-client-secret",
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	if len(repo.providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(repo.providers))
	}
	if repo.providers[0].ID != "019test-provider-id" {
		t.Errorf("provider ID = %q, want %q", repo.providers[0].ID, "019test-provider-id")
	}
	if repo.providers[0].Name != "github" {
		t.Errorf("provider Name = %q, want %q", repo.providers[0].Name, "github")
	}
}

func TestService_UpdateProviderConfig_Validation(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UUID: fakeUUIDGenerator{}, UOW: testUnitOfWork{}}

	tests := []struct {
		name    string
		command UpdateProviderConfigCommand
		wantErr bool
	}{
		{
			name: "valid config update",
			command: UpdateProviderConfigCommand{
				ProviderID:     "prov-1",
				OrganizationID: "org-1",
				ConfigType:     "oauth",
				Config:         map[string]any{"clientId": "new-id"},
			},
			wantErr: false,
		},
		{
			name: "missing providerId",
			command: UpdateProviderConfigCommand{
				ConfigType: "oauth",
				Config:     map[string]any{"clientId": "new-id"},
			},
			wantErr: true,
		},
		{
			name: "missing configType",
			command: UpdateProviderConfigCommand{
				ProviderID: "prov-1",
				Config:     map[string]any{"clientId": "new-id"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockProviderRepo{
				providers: []*domain.Provider{{ID: "prov-1", OrganizationID: "org-1", Name: "test"}},
			}
			svc.UOW = testUnitOfWork{providerRepo: repo}
			err := svc.UpdateProviderConfig(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateProviderConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_UpdateProviderConfig_ProviderNotFound(t *testing.T) {
	repo := &mockProviderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  fakeUUIDGenerator{},
		UOW:   testUnitOfWork{providerRepo: repo},
	}

	err := svc.UpdateProviderConfig(context.Background(), UpdateProviderConfigCommand{
		ProviderID:     "nonexistent",
		OrganizationID: "org-1",
		ConfigType:     "oauth",
		Config:         map[string]any{},
	})
	if err == nil {
		t.Error("expected error for nonexistent provider, got nil")
	}
}

func TestService_DeleteProvider(t *testing.T) {
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "prov-1", OrganizationID: "org-1", Name: "test-provider"},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{providerRepo: repo},
	}

	err := svc.DeleteProvider(context.Background(), "prov-1", "org-1", "")
	if err != nil {
		t.Fatalf("DeleteProvider() error = %v", err)
	}
	if len(repo.providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(repo.providers))
	}
}

func TestService_DeleteProvider_NotFound(t *testing.T) {
	repo := &mockProviderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   testUnitOfWork{providerRepo: repo},
	}

	err := svc.DeleteProvider(context.Background(), "nonexistent", "org-1", "")
	if err == nil {
		t.Error("expected error for nonexistent provider, got nil")
	}
}

func TestService_GetProvider(t *testing.T) {
	now := time.Now()
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "prov-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{providerRepo: repo}}

	detail, err := svc.GetProvider(context.Background(), "prov-1", "org-1")
	if err != nil {
		t.Fatalf("GetProvider() error = %v", err)
	}
	if detail.ID != "prov-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "prov-1")
	}
	if detail.Name != "github" {
		t.Errorf("Name = %q, want %q", detail.Name, "github")
	}
}

func requireProviderAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != code {
		t.Errorf("error code = %q, want %q", appErr.Code, code)
	}
}

func TestService_GetProvider_EmptyOrganizationRejected(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{}}

	_, err := svc.GetProvider(context.Background(), "prov-1", "")
	requireProviderAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

func TestService_GetProvider_ForeignOrganizationNotFound(t *testing.T) {
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "prov-1", OrganizationID: "org-2", Name: "foreign"},
		},
	}
	svc := Service{UOW: testUnitOfWork{providerRepo: repo}}

	_, err := svc.GetProvider(context.Background(), "prov-1", "org-1")
	requireProviderAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_GetProviderConfig_ForeignOrganizationNotFound(t *testing.T) {
	repo := &mockProviderRepo{
		providers: []*domain.Provider{{ID: "prov-1", OrganizationID: "org-2"}},
	}
	svc := Service{UOW: testUnitOfWork{providerRepo: repo}}

	_, err := svc.GetProviderConfig(context.Background(), "prov-1", "org-1")
	requireProviderAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_GetProviderConfig_EmptyOrganizationRejected(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{}}

	_, err := svc.GetProviderConfig(context.Background(), "prov-1", "")
	requireProviderAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

func TestService_PatchProvider_EmptyOrganizationRejected(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{}}
	name := "renamed"

	err := svc.PatchProvider(context.Background(), PatchProviderCommand{ID: "prov-1", DisplayName: &name})
	requireProviderAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

func TestService_DeleteProvider_EmptyOrganizationRejected(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{}}

	err := svc.DeleteProvider(context.Background(), "prov-1", "", "")
	requireProviderAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

func TestService_ListProviders(t *testing.T) {
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "prov-1", Name: "github", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "prov-2", Name: "google", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		},
	}
	svc := Service{UOW: testUnitOfWork{providerRepo: repo}}

	items, total, err := svc.ListProviders(context.Background(), ListProvidersQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListProviders() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

func TestService_ListProviderSecretStatuses(t *testing.T) {
	now := time.Now()
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "prov-1", OrganizationID: "org-1", Name: "github"},
		},
		secretStatus: []domain.ProviderSecretStatus{
			{ID: "ss-1", ProviderID: "prov-1", SecretType: "client_secret", Status: "active", LastRotatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{providerRepo: repo}}

	items, err := svc.ListProviderSecretStatuses(context.Background(), "prov-1", "org-1")
	if err != nil {
		t.Fatalf("ListProviderSecretStatuses() error = %v", err)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1", len(items))
	}
	if items[0].SecretType != "client_secret" {
		t.Errorf("SecretType = %q, want %q", items[0].SecretType, "client_secret")
	}
}

func TestService_PatchProvider_Success(t *testing.T) {
	now := time.Now()
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "prov-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{providerRepo: repo},
	}

	newDisplayName := "GitHub Updated"
	err := svc.PatchProvider(context.Background(), PatchProviderCommand{
		ID:             "prov-1",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
		Revision:       1,
	})
	if err != nil {
		t.Fatalf("PatchProvider() error = %v", err)
	}
	if repo.providers[0].DisplayName != "GitHub Updated" {
		t.Errorf("DisplayName = %q, want %q", repo.providers[0].DisplayName, "GitHub Updated")
	}
	if repo.providers[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.providers[0].Revision)
	}
}

func TestService_PatchProvider_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "prov-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Revision: 5, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{providerRepo: repo},
	}

	newDisplayName := "GitHub Updated"
	err := svc.PatchProvider(context.Background(), PatchProviderCommand{
		ID:             "prov-1",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
		Revision:       3, // stale
	})
	if err == nil {
		t.Fatal("PatchProvider() expected error for revision mismatch, got nil")
	}
}

func TestService_PatchProvider_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockProviderRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{providerRepo: repo},
	}

	newDisplayName := "Updated"
	err := svc.PatchProvider(context.Background(), PatchProviderCommand{
		ID:             "nonexistent",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
	})
	if err == nil {
		t.Fatal("PatchProvider() expected error for nonexistent provider, got nil")
	}
}

func TestService_GetProvider_NotFound(t *testing.T) {
	repo := &mockProviderRepo{}
	svc := Service{UOW: testUnitOfWork{providerRepo: repo}}

	_, err := svc.GetProvider(context.Background(), "nonexistent", "org-1")
	if err == nil {
		t.Error("expected error for nonexistent provider, got nil")
	}
}

// TestEnsureProviderTenantScope pins the application ownership decision that
// backs every by-ID provider route: cross-tenant rows look missing, while an
// authenticated platform scope keeps the all-tenant semantics.
func TestEnsureProviderTenantScope(t *testing.T) {
	strict := tenantctx.WithTenantScope(context.Background(), "org-a", "org-a", tenantctx.TenantFilterStrict)
	system := tenantctx.WithTenantScope(context.Background(), "", "", tenantctx.TenantFilterSystem)

	tests := []struct {
		name           string
		ctx            context.Context
		providerOrgID  string
		requestedOrgID string
		wantNotFound   bool
	}{
		{name: "same tenant", ctx: strict, providerOrgID: "org-a", requestedOrgID: "org-a"},
		{name: "cross tenant", ctx: strict, providerOrgID: "org-b", requestedOrgID: "org-a", wantNotFound: true},
		{name: "missing tenant scope", ctx: context.Background(), providerOrgID: "org-a", requestedOrgID: "", wantNotFound: true},
		{name: "platform scope all tenants", ctx: tenantctx.SuperAdmin(context.Background()), providerOrgID: "org-b", requestedOrgID: ""},
		{name: "system scope all tenants", ctx: system, providerOrgID: "org-b", requestedOrgID: ""},
		{name: "platform scope explicit tenant", ctx: tenantctx.SuperAdmin(context.Background()), providerOrgID: "org-b", requestedOrgID: "org-b"},
		{name: "platform scope explicit foreign tenant", ctx: tenantctx.SuperAdmin(context.Background()), providerOrgID: "org-b", requestedOrgID: "org-c", wantNotFound: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ensureProviderTenantScope(tt.ctx, tt.providerOrgID, tt.requestedOrgID)
			if !tt.wantNotFound {
				if err != nil {
					t.Fatalf("ensureProviderTenantScope() error = %v, want nil", err)
				}
				return
			}
			var platformErr *apperrors.Error
			if !errors.As(err, &platformErr) || platformErr.Code != apperrors.CodeNotFound {
				t.Fatalf("ensureProviderTenantScope() error = %v, want not found", err)
			}
		})
	}
}
