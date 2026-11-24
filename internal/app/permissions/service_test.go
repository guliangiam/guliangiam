package permissions

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
// Mock permission repository
// ---------------------------------------------------------------------------

type mockPermissionRepo struct {
	permissions []*permissiondomain.Permission
}

func (r *mockPermissionRepo) GetByID(ctx context.Context, id string) (*permissiondomain.Permission, error) {
	return r.find(id, "")
}

func (r *mockPermissionRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*permissiondomain.Permission, error) {
	return r.find(id, organizationID)
}

func (r *mockPermissionRepo) find(id, organizationID string) (*permissiondomain.Permission, error) {
	for _, p := range r.permissions {
		if p.ID != id {
			continue
		}
		if organizationID == "" || p.OrganizationID == organizationID {
			return p, nil
		}
	}
	return nil, apperrors.NewNotFound("permission not found")
}

func (r *mockPermissionRepo) List(ctx context.Context, query permissiondomain.ListQuery) ([]*permissiondomain.Permission, int64, error) {
	return r.permissions, int64(len(r.permissions)), nil
}

func (r *mockPermissionRepo) Create(ctx context.Context, permission *permissiondomain.Permission) error {
	r.permissions = append(r.permissions, permission)
	return nil
}

func (r *mockPermissionRepo) Update(ctx context.Context, permission *permissiondomain.Permission) error {
	for i, p := range r.permissions {
		if p.ID == permission.ID {
			r.permissions[i] = permission
			return nil
		}
	}
	return apperrors.NewNotFound("permission not found")
}

func (r *mockPermissionRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, p := range r.permissions {
		if p.ID != id {
			continue
		}
		if p.OrganizationID != organizationID {
			continue
		}
		r.permissions = append(r.permissions[:i], r.permissions[i+1:]...)
		return nil
	}
	return apperrors.NewNotFound("permission not found")
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	permissionRepo *mockPermissionRepo
	userRepo       userdomain.Repository
	roleRepo       roledomain.Repository
}

func (u testUnitOfWork) Permissions() permissiondomain.Repository { return u.permissionRepo }
func (u testUnitOfWork) Applications() applications.Repository    { return nil }
func (u testUnitOfWork) Outbox() domainoutbox.Repository          { return nil }

func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository                          { return nil }
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
func (u testUnitOfWork) Records() recordsdomain.Repository                                { return nil }
func (u testUnitOfWork) Resources() resourcedomain.Repository                             { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                                     { return u.roleRepo }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                                 { return nil }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                               { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository                     { return nil }
func (u testUnitOfWork) Tickets() ticketdomain.Repository                                 { return nil }
func (u testUnitOfWork) Tokens() tokendomain.Repository                                   { return nil }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository                               { return nil }
func (u testUnitOfWork) Users() userdomain.Repository                                     { return u.userRepo }
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

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }
func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_NewPermission(t *testing.T) {
	now := time.Now()
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id-aaaa"},
	}

	perm, err := svc.NewPermission(CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "read-users",
		DisplayName:    "Read Users",
		Effect:         "allow",
		IsEnabled:      boolPtr(true),
	})
	if err != nil {
		t.Fatalf("NewPermission() error = %v", err)
	}
	if perm.ID != "019test-perm-id-aaaa" {
		t.Errorf("ID = %q, want %q", perm.ID, "019test-perm-id-aaaa")
	}
	if perm.Effect != "allow" {
		t.Errorf("Effect = %q, want %q", perm.Effect, "allow")
	}
	if perm.Revision != 1 {
		t.Errorf("Revision = %d, want 1", perm.Revision)
	}
}

func TestService_CreatePermission_Validation(t *testing.T) {
	svc := Service{
		Clock: fakeClock{},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id-aaaa"},
		UOW:   testUnitOfWork{permissionRepo: &mockPermissionRepo{}},
	}

	tests := []struct {
		name    string
		command CreatePermissionCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreatePermissionCommand{
				OrganizationID: "org-1",
				Name:           "read-users",
				DisplayName:    "Read Users",
				ResourceType:   "application",
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreatePermissionCommand{
				Name:         "read-users",
				DisplayName:  "Read Users",
				ResourceType: "application",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			command: CreatePermissionCommand{
				OrganizationID: "org-1",
				DisplayName:    "Read Users",
				ResourceType:   "application",
			},
			wantErr: true,
		},
		{
			name: "missing displayName",
			command: CreatePermissionCommand{
				OrganizationID: "org-1",
				Name:           "read-users",
				ResourceType:   "application",
			},
			wantErr: true,
		},
		{
			name: "missing resourceType",
			command: CreatePermissionCommand{
				OrganizationID: "org-1",
				Name:           "read-users",
				DisplayName:    "Read Users",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreatePermission(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreatePermission() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreatePermission_Success(t *testing.T) {
	now := time.Now()
	repo := &mockPermissionRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-perm-id-aaaa"},
		UOW:   testUnitOfWork{permissionRepo: repo},
	}

	err := svc.CreatePermission(context.Background(), CreatePermissionCommand{
		OrganizationID: "org-1",
		Name:           "read-users",
		DisplayName:    "Read Users",
		ResourceType:   "application",
		Effect:         "allow",
		IsEnabled:      boolPtr(true),
	})
	if err != nil {
		t.Fatalf("CreatePermission() error = %v", err)
	}
	if len(repo.permissions) != 1 {
		t.Errorf("expected 1 permission in repo, got %d", len(repo.permissions))
	}
}

func TestService_DeletePermission(t *testing.T) {
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Name: "read", IsEnabled: true},
		},
	}
	svc := Service{UOW: testUnitOfWork{permissionRepo: repo}}

	err := svc.DeletePermission(context.Background(), "org-1", "perm-1")
	if err != nil {
		t.Fatalf("DeletePermission() error = %v", err)
	}
	if len(repo.permissions) != 0 {
		t.Errorf("expected 0 permissions in repo, got %d", len(repo.permissions))
	}
}

func TestService_DeletePermission_NotFound(t *testing.T) {
	repo := &mockPermissionRepo{}
	svc := Service{UOW: testUnitOfWork{permissionRepo: repo}}

	err := svc.DeletePermission(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent permission, got nil")
	}
}

func TestService_GetPermission(t *testing.T) {
	now := time.Now()
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Name: "read", DisplayName: "Read",
				IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{permissionRepo: repo}}

	detail, err := svc.GetPermission(context.Background(), "org-1", "perm-1")
	if err != nil {
		t.Fatalf("GetPermission() error = %v", err)
	}
	if detail.ID != "perm-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "perm-1")
	}
}

func TestService_ListPermissions(t *testing.T) {
	now := time.Now()
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "p-1", OrganizationID: "org-1", Name: "read", DisplayName: "Read", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{permissionRepo: repo}}

	items, total, err := svc.ListPermissions(context.Background(), ListPermissionsQuery{OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("ListPermissions() error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Errorf("len(items) = %d, want 1", len(items))
	}
}

func TestService_PatchPermission_Success(t *testing.T) {
	now := time.Now()
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Name: "read", DisplayName: "Read", IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{permissionRepo: repo},
	}

	newDisplayName := "Read All"
	err := svc.PatchPermission(context.Background(), "org-1", PatchPermissionCommand{
		ID:          "perm-1",
		DisplayName: &newDisplayName,
		Revision:    1,
	})
	if err != nil {
		t.Fatalf("PatchPermission() error = %v", err)
	}
	if repo.permissions[0].DisplayName != "Read All" {
		t.Errorf("DisplayName = %q, want %q", repo.permissions[0].DisplayName, "Read All")
	}
	if repo.permissions[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.permissions[0].Revision)
	}
}

func TestService_PatchPermission_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Name: "read", DisplayName: "Read", IsEnabled: true, Revision: 5, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{permissionRepo: repo},
	}

	newDisplayName := "Updated"
	err := svc.PatchPermission(context.Background(), "org-1", PatchPermissionCommand{
		ID:          "perm-1",
		DisplayName: &newDisplayName,
		Revision:    3, // stale
	})
	if err == nil {
		t.Fatal("PatchPermission() expected error for revision mismatch, got nil")
	}
}

func TestService_PatchPermission_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockPermissionRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{permissionRepo: repo},
	}

	newDisplayName := "Updated"
	err := svc.PatchPermission(context.Background(), "org-1", PatchPermissionCommand{
		ID:          "nonexistent",
		DisplayName: &newDisplayName,
	})
	if err == nil {
		t.Fatal("PatchPermission() expected error for nonexistent permission, got nil")
	}
}

func TestService_PatchPermission_DisablePermission(t *testing.T) {
	now := time.Now()
	repo := &mockPermissionRepo{
		permissions: []*permissiondomain.Permission{
			{ID: "perm-1", OrganizationID: "org-1", Name: "read", DisplayName: "Read", IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "test"},
		UOW:   testUnitOfWork{permissionRepo: repo},
	}

	isEnabled := false
	err := svc.PatchPermission(context.Background(), "org-1", PatchPermissionCommand{
		ID:        "perm-1",
		IsEnabled: &isEnabled,
		Revision:  1,
	})
	if err != nil {
		t.Fatalf("PatchPermission() error = %v", err)
	}
	if repo.permissions[0].IsEnabled {
		t.Error("IsEnabled = true, want false")
	}
}

func TestService_GetPermission_NotFound(t *testing.T) {
	repo := &mockPermissionRepo{}
	svc := Service{UOW: testUnitOfWork{permissionRepo: repo}}

	_, err := svc.GetPermission(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent permission, got nil")
	}
}
