package roles

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

func boolPtr(b bool) *bool { return &b }

// ---------------------------------------------------------------------------
// Mock role repository
// ---------------------------------------------------------------------------

type mockRoleRepo struct {
	roles []*roledomain.Role
}

func (r *mockRoleRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*roledomain.Role, error) {
	for _, rl := range r.roles {
		if rl.ID == id && rl.OrganizationID == organizationID {
			return rl, nil
		}
	}
	return nil, apperrors.NewNotFound("role not found")
}

func (r *mockRoleRepo) List(ctx context.Context, query roledomain.ListQuery) ([]*roledomain.Role, int64, error) {
	return r.roles, int64(len(r.roles)), nil
}

func (r *mockRoleRepo) Create(ctx context.Context, role *roledomain.Role) error {
	r.roles = append(r.roles, role)
	return nil
}

func (r *mockRoleRepo) Update(ctx context.Context, role *roledomain.Role) error {
	for i, rl := range r.roles {
		if rl.ID == role.ID {
			r.roles[i] = role
			return nil
		}
	}
	return apperrors.NewNotFound("role not found")
}

func (r *mockRoleRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, rl := range r.roles {
		if rl.ID == id && rl.OrganizationID == organizationID {
			r.roles = append(r.roles[:i], r.roles[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("role not found")
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	roleRepo *mockRoleRepo
	records  recordsdomain.Repository
}

// fakeRecordsRepo captures audit rows written inside the transaction. Only
// Create is exercised; the embedded interface keeps the chain methods out of
// scope.
type fakeRecordsRepo struct {
	recordsdomain.Repository

	created []*recordsdomain.Record
}

func (r *fakeRecordsRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func (u testUnitOfWork) Roles() roledomain.Repository                                 { return u.roleRepo }
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
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return u.records }
func (u testUnitOfWork) Resources() resourcedomain.Repository                             { return nil }
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

func TestService_NewRole(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-role-id-aaaa"},
	}

	role, err := svc.NewRole(CreateRoleCommand{
		OrganizationID: "org-1",
		Name:           "admin",
		DisplayName:    "Admin",
		Description:    "Administrator role",
		IsEnabled:      boolPtr(true),
	})
	if err != nil {
		t.Fatalf("NewRole() error = %v", err)
	}
	if role.ID != "019test-role-id-aaaa" {
		t.Errorf("ID = %q, want %q", role.ID, "019test-role-id-aaaa")
	}
	if role.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", role.OrganizationID, "org-1")
	}
	if !role.IsEnabled {
		t.Error("IsEnabled = false, want true")
	}
	if role.Revision != 1 {
		t.Errorf("Revision = %d, want 1", role.Revision)
	}
}

func TestService_NewRole_DefaultIsEnabled(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-role-id-aaaa"},
	}

	role, err := svc.NewRole(CreateRoleCommand{
		OrganizationID: "org-1",
		Name:           "viewer",
		DisplayName:    "Viewer",
	})
	if err != nil {
		t.Fatalf("NewRole() error = %v", err)
	}
	if !role.IsEnabled {
		t.Error("IsEnabled should default to true when nil")
	}
}

func TestService_CreateRole_Validation(t *testing.T) {
	svc := Service{
		Clock: fakeClock{},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-role-id-aaaa"},
		UOW:   testUnitOfWork{roleRepo: &mockRoleRepo{}},
	}

	tests := []struct {
		name    string
		command CreateRoleCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateRoleCommand{
				OrganizationID: "org-1",
				Name:           "admin",
				DisplayName:    "Admin",
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateRoleCommand{
				Name:        "admin",
				DisplayName: "Admin",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			command: CreateRoleCommand{
				OrganizationID: "org-1",
				DisplayName:    "Admin",
			},
			wantErr: true,
		},
		{
			name: "missing displayName",
			command: CreateRoleCommand{
				OrganizationID: "org-1",
				Name:           "admin",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateRole(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreateRole_Success(t *testing.T) {
	now := time.Now()
	repo := &mockRoleRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-role-id-aaaa"},
		UOW:   testUnitOfWork{roleRepo: repo},
	}

	err := svc.CreateRole(context.Background(), CreateRoleCommand{
		OrganizationID: "org-1",
		Name:           "admin",
		DisplayName:    "Admin",
		Description:    "Administrator",
		IsEnabled:      boolPtr(true),
	})
	if err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	if len(repo.roles) != 1 {
		t.Errorf("expected 1 role in repo, got %d", len(repo.roles))
	}
}

func TestService_DeleteRole(t *testing.T) {
	repo := &mockRoleRepo{
		roles: []*roledomain.Role{
			{ID: "role-1", OrganizationID: "org-1", Name: "admin", IsEnabled: true},
		},
	}
	svc := Service{UOW: testUnitOfWork{roleRepo: repo}}

	err := svc.DeleteRole(context.Background(), "org-1", "role-1")
	if err != nil {
		t.Fatalf("DeleteRole() error = %v", err)
	}
	if len(repo.roles) != 0 {
		t.Errorf("expected 0 roles in repo, got %d", len(repo.roles))
	}
}

func TestService_DeleteRole_NotFound(t *testing.T) {
	repo := &mockRoleRepo{}
	svc := Service{UOW: testUnitOfWork{roleRepo: repo}}

	err := svc.DeleteRole(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent role, got nil")
	}
}

func TestService_GetRole(t *testing.T) {
	now := time.Now()
	repo := &mockRoleRepo{
		roles: []*roledomain.Role{
			{ID: "role-1", OrganizationID: "org-1", Name: "admin", DisplayName: "Admin",
				IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{roleRepo: repo}}

	detail, err := svc.GetRole(context.Background(), "org-1", "role-1")
	if err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	if detail.ID != "role-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "role-1")
	}
}

func TestService_ListRoles(t *testing.T) {
	now := time.Now()
	repo := &mockRoleRepo{
		roles: []*roledomain.Role{
			{ID: "r-1", OrganizationID: "org-1", Name: "admin", DisplayName: "Admin", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{roleRepo: repo}}

	items, total, err := svc.ListRoles(context.Background(), ListRolesQuery{OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1", len(items))
	}
}

func TestService_PatchRole_Success(t *testing.T) {
	now := time.Now()
	repo := &mockRoleRepo{
		roles: []*roledomain.Role{
			{ID: "role-1", OrganizationID: "org-1", Name: "admin", DisplayName: "Admin", IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	recordsRepo := &fakeRecordsRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{roleRepo: repo, records: recordsRepo},
	}

	const actorID = "019operator-0000-7000-8000-000000000003"
	newDisplayName := "Administrator"
	err := svc.PatchRole(context.Background(), "org-1", PatchRoleCommand{
		ID:          "role-1",
		ActorID:     actorID,
		DisplayName: &newDisplayName,
		Revision:    1,
	})
	if err != nil {
		t.Fatalf("PatchRole() error = %v", err)
	}
	if repo.roles[0].DisplayName != "Administrator" {
		t.Errorf("DisplayName = %q, want %q", repo.roles[0].DisplayName, "Administrator")
	}
	if repo.roles[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.roles[0].Revision)
	}

	// Verify the audit row carries the operator, not the target role, as actor.
	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	record := recordsRepo.created[0]
	if record.Action != "role.updated" {
		t.Errorf("audit action = %q, want %q", record.Action, "role.updated")
	}
	if record.ActorID != actorID {
		t.Errorf("audit actorID = %q, want %q", record.ActorID, actorID)
	}
}

func TestService_PatchRole_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockRoleRepo{
		roles: []*roledomain.Role{
			{ID: "role-1", OrganizationID: "org-1", Name: "admin", DisplayName: "Admin", IsEnabled: true, Revision: 5, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{roleRepo: repo},
	}

	newDisplayName := "Updated"
	err := svc.PatchRole(context.Background(), "org-1", PatchRoleCommand{
		ID:          "role-1",
		DisplayName: &newDisplayName,
		Revision:    3, // stale
	})
	if err == nil {
		t.Fatal("PatchRole() expected error for revision mismatch, got nil")
	}
}

func TestService_PatchRole_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockRoleRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{roleRepo: repo},
	}

	newDisplayName := "Updated"
	err := svc.PatchRole(context.Background(), "org-1", PatchRoleCommand{
		ID:          "nonexistent",
		DisplayName: &newDisplayName,
	})
	if err == nil {
		t.Fatal("PatchRole() expected error for nonexistent role, got nil")
	}
}

func TestService_PatchRole_DisableRole(t *testing.T) {
	now := time.Now()
	repo := &mockRoleRepo{
		roles: []*roledomain.Role{
			{ID: "role-1", OrganizationID: "org-1", Name: "admin", DisplayName: "Admin", IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{roleRepo: repo},
	}

	isEnabled := false
	err := svc.PatchRole(context.Background(), "org-1", PatchRoleCommand{
		ID:        "role-1",
		IsEnabled: &isEnabled,
		Revision:  1,
	})
	if err != nil {
		t.Fatalf("PatchRole() error = %v", err)
	}
	if repo.roles[0].IsEnabled {
		t.Error("IsEnabled = true, want false")
	}
}

func TestService_GetRole_NotFound(t *testing.T) {
	repo := &mockRoleRepo{}
	svc := Service{UOW: testUnitOfWork{roleRepo: repo}}

	_, err := svc.GetRole(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent role, got nil")
	}
}
