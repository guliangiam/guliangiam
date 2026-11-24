package certs

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
// Mock cert repository
// ---------------------------------------------------------------------------

type mockCertRepo struct {
	certs []*certdomain.Cert
}

func (r *mockCertRepo) GetByID(ctx context.Context, id, organizationID string) (*certdomain.Cert, error) {
	for _, c := range r.certs {
		if c.ID == id && c.OrganizationID == organizationID {
			return c, nil
		}
	}
	return nil, apperrors.NewNotFound("cert not found")
}

func (r *mockCertRepo) List(ctx context.Context, query certdomain.ListQuery) ([]*certdomain.Cert, int64, error) {
	return r.certs, int64(len(r.certs)), nil
}

func (r *mockCertRepo) Create(ctx context.Context, cert *certdomain.Cert) error {
	r.certs = append(r.certs, cert)
	return nil
}

func (r *mockCertRepo) Update(ctx context.Context, cert *certdomain.Cert) error {
	for i, c := range r.certs {
		if c.ID == cert.ID && c.OrganizationID == cert.OrganizationID {
			r.certs[i] = cert
			return nil
		}
	}
	return apperrors.NewNotFound("cert not found")
}

func (r *mockCertRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, c := range r.certs {
		if c.ID == id && c.OrganizationID == organizationID {
			r.certs = append(r.certs[:i], r.certs[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("cert not found")
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	certRepo *mockCertRepo
}

func (u testUnitOfWork) Certs() certdomain.Repository                                 { return u.certRepo }
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
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return nil }
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

func (u testUnitOfWork) Outbox() domainoutbox.Repository { return nil }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_NewCert(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-cert-id-aaaa"},
	}

	cert, err := svc.NewCert(CreateCertCommand{
		OrganizationID:  "org-1",
		Name:            "my-cert",
		DisplayName:     "My Certificate",
		Scope:           "JWT",
		Type:            "x509",
		CryptoAlgorithm: "RS256",
	})
	if err != nil {
		t.Fatalf("NewCert() error = %v", err)
	}
	if cert.ID != "019test-cert-id-aaaa" {
		t.Errorf("ID = %q, want %q", cert.ID, "019test-cert-id-aaaa")
	}
	if cert.Scope != "JWT" {
		t.Errorf("Scope = %q, want %q", cert.Scope, "JWT")
	}
	if cert.Revision != 1 {
		t.Errorf("Revision = %d, want 1", cert.Revision)
	}
}

func TestService_CreateCert_Validation(t *testing.T) {
	svc := Service{
		Clock: fakeClock{},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-cert-id-aaaa"},
		UOW:   testUnitOfWork{certRepo: &mockCertRepo{}},
	}

	tests := []struct {
		name    string
		command CreateCertCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateCertCommand{
				OrganizationID:  "org-1",
				Name:            "my-cert",
				DisplayName:     "My Cert",
				Scope:           "JWT",
				Type:            "x509",
				CryptoAlgorithm: "RS256",
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateCertCommand{
				Name:        "my-cert",
				DisplayName: "My Cert",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			command: CreateCertCommand{
				OrganizationID: "org-1",
				DisplayName:    "My Cert",
			},
			wantErr: true,
		},
		{
			name: "invalid scope",
			command: CreateCertCommand{
				OrganizationID: "org-1",
				Name:           "my-cert",
				DisplayName:    "My Cert",
				Scope:          "INVALID",
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			command: CreateCertCommand{
				OrganizationID: "org-1",
				Name:           "my-cert",
				DisplayName:    "My Cert",
				Type:           "INVALID",
			},
			wantErr: true,
		},
		{
			name: "invalid cryptoAlgorithm",
			command: CreateCertCommand{
				OrganizationID:  "org-1",
				Name:            "my-cert",
				DisplayName:     "My Cert",
				CryptoAlgorithm: "INVALID",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateCert(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateCert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreateCert_Success(t *testing.T) {
	now := time.Now()
	repo := &mockCertRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-cert-id-aaaa"},
		UOW:   testUnitOfWork{certRepo: repo},
	}

	err := svc.CreateCert(context.Background(), CreateCertCommand{
		OrganizationID:  "org-1",
		Name:            "my-cert",
		DisplayName:     "My Cert",
		Scope:           "JWT",
		Type:            "x509",
		CryptoAlgorithm: "RS256",
	})
	if err != nil {
		t.Fatalf("CreateCert() error = %v", err)
	}
	if len(repo.certs) != 1 {
		t.Errorf("expected 1 cert in repo, got %d", len(repo.certs))
	}
}

func TestService_DeleteCert(t *testing.T) {
	repo := &mockCertRepo{
		certs: []*certdomain.Cert{
			{ID: "cert-1", OrganizationID: "org-1", Name: "my-cert"},
		},
	}
	svc := Service{UOW: testUnitOfWork{certRepo: repo}}

	err := svc.DeleteCert(context.Background(), "org-1", "cert-1")
	if err != nil {
		t.Fatalf("DeleteCert() error = %v", err)
	}
	if len(repo.certs) != 0 {
		t.Errorf("expected 0 certs in repo, got %d", len(repo.certs))
	}
}

func TestService_DeleteCert_NotFound(t *testing.T) {
	repo := &mockCertRepo{}
	svc := Service{UOW: testUnitOfWork{certRepo: repo}}

	err := svc.DeleteCert(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent cert, got nil")
	}
}

func TestService_GetCert(t *testing.T) {
	now := time.Now()
	repo := &mockCertRepo{
		certs: []*certdomain.Cert{
			{ID: "cert-1", OrganizationID: "org-1", Name: "my-cert", DisplayName: "My Cert",
				Scope: "JWT", Type: "x509", CryptoAlgorithm: "RS256",
				Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{certRepo: repo}}

	detail, err := svc.GetCert(context.Background(), "org-1", "cert-1")
	if err != nil {
		t.Fatalf("GetCert() error = %v", err)
	}
	if detail.ID != "cert-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "cert-1")
	}
	if detail.Revision != 1 {
		t.Errorf("Revision = %d, want 1", detail.Revision)
	}
}

func TestService_ListCerts(t *testing.T) {
	now := time.Now()
	repo := &mockCertRepo{
		certs: []*certdomain.Cert{
			{ID: "c-1", OrganizationID: "org-1", Name: "cert1", DisplayName: "Cert1",
				Scope: "JWT", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{certRepo: repo}}

	items, total, err := svc.ListCerts(context.Background(), ListCertsQuery{OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("ListCerts() error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1", len(items))
	}
}

func TestService_PatchCert(t *testing.T) {
	now := time.Now()
	repo := &mockCertRepo{
		certs: []*certdomain.Cert{
			{ID: "cert-1", OrganizationID: "org-1", Name: "my-cert", DisplayName: "My Cert",
				Scope: "JWT", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{certRepo: repo},
	}

	newDisplayName := "Updated Cert"
	err := svc.PatchCert(context.Background(), PatchCertCommand{
		ID:             "cert-1",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
		Revision:       1,
	})
	if err != nil {
		t.Fatalf("PatchCert() error = %v", err)
	}
	if repo.certs[0].DisplayName != "Updated Cert" {
		t.Errorf("DisplayName = %q, want %q", repo.certs[0].DisplayName, "Updated Cert")
	}
	if repo.certs[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.certs[0].Revision)
	}
}

func TestService_PatchCert_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockCertRepo{
		certs: []*certdomain.Cert{
			{ID: "cert-1", OrganizationID: "org-1", Name: "my-cert", DisplayName: "My Cert",
				Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{certRepo: repo},
	}

	newDisplayName := "Updated Cert"
	err := svc.PatchCert(context.Background(), PatchCertCommand{
		ID:             "cert-1",
		OrganizationID: "org-1",
		DisplayName:    &newDisplayName,
		Revision:       99,
	})
	if err == nil {
		t.Error("expected error for revision mismatch, got nil")
	}
}
