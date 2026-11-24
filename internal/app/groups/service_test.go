package groups

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

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
	usermock "github.com/guliangiam/guliangiam/internal/domain/users/mock"
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
// Mock group repository
// ---------------------------------------------------------------------------

type mockGroupRepo struct {
	groups []*groupsdomain.Group
}

func (r *mockGroupRepo) GetByID(ctx context.Context, id string) (*groupsdomain.Group, error) {
	for _, g := range r.groups {
		if g.ID == id {
			return g, nil
		}
	}
	return nil, apperrors.NewNotFound("group not found")
}

func (r *mockGroupRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*groupsdomain.Group, error) {
	for _, g := range r.groups {
		if g.ID == id && g.OrganizationID == organizationID {
			return g, nil
		}
	}
	return nil, apperrors.NewNotFound("group not found")
}

func (r *mockGroupRepo) List(ctx context.Context, query groupsdomain.ListQuery) ([]*groupsdomain.Group, int64, error) {
	return r.groups, int64(len(r.groups)), nil
}

func (r *mockGroupRepo) Create(ctx context.Context, group *groupsdomain.Group) error {
	r.groups = append(r.groups, group)
	return nil
}

func (r *mockGroupRepo) Update(ctx context.Context, group *groupsdomain.Group) error {
	for i, g := range r.groups {
		if g.ID == group.ID {
			r.groups[i] = group
			return nil
		}
	}
	return apperrors.NewNotFound("group not found")
}

func (r *mockGroupRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, g := range r.groups {
		if g.ID == id && g.OrganizationID == organizationID {
			r.groups = append(r.groups[:i], r.groups[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("group not found")
}

func (r *mockGroupRepo) CountChildrenByParentID(ctx context.Context, organizationID, parentID string) (int64, error) {
	var count int64
	for _, g := range r.groups {
		if g.ParentID == parentID && g.OrganizationID == organizationID {
			count++
		}
	}
	return count, nil
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	groupRepo *mockGroupRepo
	records   recordsdomain.Repository
	userRepo  userdomain.Repository
}

func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return u.groupRepo }
func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
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
func (u testUnitOfWork) Roles() roledomain.Repository                                     { return nil }
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

func TestService_CreateGroup_Validation(t *testing.T) {
	repo := &mockGroupRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-aaaa"},
		UOW:   testUnitOfWork{groupRepo: repo},
	}

	tests := []struct {
		name    string
		command CreateGroupCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateGroupCommand{
				OrganizationID: "org-1",
				Name:           "engineering",
				DisplayName:    "Engineering",
				IsEnabled:      true,
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateGroupCommand{
				Name:        "engineering",
				DisplayName: "Engineering",
			},
			wantErr: true,
		},
		{
			name: "missing name",
			command: CreateGroupCommand{
				OrganizationID: "org-1",
				DisplayName:    "Engineering",
			},
			wantErr: true,
		},
		{
			name: "name contains slash",
			command: CreateGroupCommand{
				OrganizationID: "org-1",
				Name:           "eng/neer",
				DisplayName:    "Engineering",
			},
			wantErr: true,
		},
		{
			name: "missing displayName",
			command: CreateGroupCommand{
				OrganizationID: "org-1",
				Name:           "engineering",
			},
			wantErr: true,
		},
		{
			name: "type outside closed set",
			command: CreateGroupCommand{
				OrganizationID: "org-1",
				Name:           "engineering",
				DisplayName:    "Engineering",
				Type:           "admin",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateGroup(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateGroup() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreateGroup_Success(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{}
	recordsRepo := &groupTestRecordsRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-aaaa"},
		UOW:   testUnitOfWork{groupRepo: repo, records: recordsRepo},
	}

	const actorID = "019operator-0000-7000-8000-000000000004"
	group, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1",
		ActorID:        actorID,
		Name:           "engineering",
		DisplayName:    "Engineering",
		Type:           "department",
		Manager:        "manager-1",
		ContactEmail:   "eng@example.com",
		IsEnabled:      true,
		Users:          []string{"user-1", "user-2"},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if group.ID != "019test-group-id-aaaa" {
		t.Errorf("ID = %q, want %q", group.ID, "019test-group-id-aaaa")
	}
	if group.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", group.OrganizationID, "org-1")
	}
	if group.Revision != 1 {
		t.Errorf("Revision = %d, want 1", group.Revision)
	}
	if len(repo.groups) != 1 {
		t.Errorf("expected 1 group in repo, got %d", len(repo.groups))
	}
	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	if recordsRepo.created[0].Action != "groups.created" {
		t.Errorf("Action = %q, want %q", recordsRepo.created[0].Action, "groups.created")
	}
	if recordsRepo.created[0].ActorID != actorID {
		t.Errorf("ActorID = %q, want operator %q", recordsRepo.created[0].ActorID, actorID)
	}
	if group.Type != "department" {
		t.Errorf("Type = %q, want department", group.Type)
	}
}

// 闭集裁定（G7-s）：创建留空回落 virtual，新建行不允许落在闭集之外。
func TestService_CreateGroup_EmptyTypeDefaultsVirtual(t *testing.T) {
	repo := &mockGroupRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-cccc"},
		UOW:   testUnitOfWork{groupRepo: repo, records: &groupTestRecordsRepo{}},
	}

	group, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1",
		Name:           "no-type",
		DisplayName:    "No Type",
		Type:           "   ",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if group.Type != "virtual" {
		t.Errorf("Type = %q, want virtual", group.Type)
	}
}

// TestService_CreateGroup_SystemAction_ActorIDEmpty is the control case for
// machine-created groups: no operator means the audit row keeps a NULL actor.
func TestService_CreateGroup_SystemAction_ActorIDEmpty(t *testing.T) {
	repo := &mockGroupRepo{}
	recordsRepo := &groupTestRecordsRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-bbbb"},
		UOW:   testUnitOfWork{groupRepo: repo, records: recordsRepo},
	}

	_, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1",
		ActorID:        "",
		Name:           "system-imported",
		DisplayName:    "System Imported",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	if recordsRepo.created[0].ActorID != "" {
		t.Errorf("ActorID = %q, want empty for system action", recordsRepo.created[0].ActorID)
	}
}

// groupTestRecordsRepo captures audit records written inside the transaction.
type groupTestRecordsRepo struct {
	recordsdomain.Repository
	created []*recordsdomain.Record
}

func (r *groupTestRecordsRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

func TestService_GetGroup(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{
		groups: []*groupsdomain.Group{
			{
				ID: "group-1", OrganizationID: "org-1", Name: "engineering", DisplayName: "Engineering",
				Manager: "manager-1", ContactEmail: "eng@example.com", IsEnabled: true,
				Revision: 1, CreatedAt: now, UpdatedAt: now,
			},
		},
	}
	svc := Service{UOW: testUnitOfWork{groupRepo: repo}}

	detail, err := svc.GetGroup(context.Background(), "org-1", "group-1")
	if err != nil {
		t.Fatalf("GetGroup() error = %v", err)
	}
	if detail.ID != "group-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "group-1")
	}
	if detail.Manager != "manager-1" {
		t.Errorf("Manager = %q, want %q", detail.Manager, "manager-1")
	}
	if detail.Revision != 1 {
		t.Errorf("Revision = %d, want 1", detail.Revision)
	}
}

func TestService_GetGroup_NotFound(t *testing.T) {
	repo := &mockGroupRepo{}
	svc := Service{UOW: testUnitOfWork{groupRepo: repo}}

	_, err := svc.GetGroup(context.Background(), "org-1", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent group, got nil")
	}
}

func TestService_DeleteGroup_Success(t *testing.T) {
	repo := &mockGroupRepo{
		groups: []*groupsdomain.Group{
			{ID: "group-1", OrganizationID: "org-1", Name: "engineering"},
		},
	}
	svc := Service{UOW: testUnitOfWork{groupRepo: repo}}

	err := svc.DeleteGroup(context.Background(), "org-1", DeleteGroupCommand{ID: "group-1"})
	if err != nil {
		t.Fatalf("DeleteGroup() error = %v", err)
	}
	if len(repo.groups) != 0 {
		t.Errorf("expected 0 groups in repo, got %d", len(repo.groups))
	}
}

func TestService_DeleteGroup_HasChildren(t *testing.T) {
	repo := &mockGroupRepo{
		groups: []*groupsdomain.Group{
			{ID: "group-1", OrganizationID: "org-1", Name: "engineering"},
			{ID: "group-2", OrganizationID: "org-1", ParentID: "group-1"},
		},
	}
	svc := Service{UOW: testUnitOfWork{groupRepo: repo}}

	err := svc.DeleteGroup(context.Background(), "org-1", DeleteGroupCommand{ID: "group-1"})
	if err == nil {
		t.Error("expected error when group has children, got nil")
	}
}

func TestService_ListGroups(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{
		groups: []*groupsdomain.Group{
			{ID: "g-1", OrganizationID: "org-1", Name: "eng", DisplayName: "Eng", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
			{ID: "g-2", OrganizationID: "org-1", Name: "sales", DisplayName: "Sales", IsEnabled: false, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{groupRepo: repo}}

	items, total, err := svc.ListGroups(context.Background(), ListGroupsQuery{OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("ListGroups() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

func TestService_PatchGroup(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{
		groups: []*groupsdomain.Group{
			{ID: "group-1", OrganizationID: "org-1", Name: "eng", DisplayName: "Eng", IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{groupRepo: repo},
	}

	newDisplayName := "Engineering"
	err := svc.PatchGroup(context.Background(), "org-1", PatchGroupCommand{
		ID:          "group-1",
		DisplayName: &newDisplayName,
		Revision:    1,
	})
	if err != nil {
		t.Fatalf("PatchGroup() error = %v", err)
	}
	if repo.groups[0].DisplayName != "Engineering" {
		t.Errorf("DisplayName = %q, want %q", repo.groups[0].DisplayName, "Engineering")
	}
	if repo.groups[0].Revision != 2 {
		t.Errorf("Revision = %d, want 2", repo.groups[0].Revision)
	}
}

func TestService_PatchGroup_RevisionMismatch(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{
		groups: []*groupsdomain.Group{
			{ID: "group-1", OrganizationID: "org-1", Name: "eng", DisplayName: "Eng", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{groupRepo: repo},
	}

	newDisplayName := "Engineering"
	err := svc.PatchGroup(context.Background(), "org-1", PatchGroupCommand{
		ID:          "group-1",
		DisplayName: &newDisplayName,
		Revision:    99,
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %v", err)
	}
	// 规范 4.10：版本冲突 412（不再是 409），响应体指明当前 revision。
	if appErr.Code != apperrors.CodePreconditionFailed {
		t.Errorf("code = %q, want %q", appErr.Code, apperrors.CodePreconditionFailed)
	}
	if appErr.Meta["revision"] != int64(1) {
		t.Errorf("meta revision = %v, want current 1", appErr.Meta["revision"])
	}
	if repo.groups[0].Revision != 1 {
		t.Errorf("Revision = %d, want unchanged 1", repo.groups[0].Revision)
	}
}

// TestService_PatchGroup_MissingRevisionRejected pins 规范 4.10 / R2-C05: a
// patch without an optimistic-lock precondition is 428, not last-write-wins.
func TestService_PatchGroup_MissingRevisionRejected(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{
		groups: []*groupsdomain.Group{
			{ID: "group-1", OrganizationID: "org-1", Name: "eng", DisplayName: "Eng", Revision: 1, CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   testUnitOfWork{groupRepo: repo},
	}

	newDisplayName := "Engineering"
	err := svc.PatchGroup(context.Background(), "org-1", PatchGroupCommand{
		ID:          "group-1",
		DisplayName: &newDisplayName,
	})
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %v", err)
	}
	if appErr.Code != apperrors.CodePreconditionRequired {
		t.Errorf("code = %q, want %q", appErr.Code, apperrors.CodePreconditionRequired)
	}
	if appErr.HTTPStatus() != 428 {
		t.Errorf("http status = %d, want 428", appErr.HTTPStatus())
	}
	if repo.groups[0].DisplayName != "Eng" {
		t.Errorf("DisplayName = %q, want unchanged", repo.groups[0].DisplayName)
	}
	if repo.groups[0].Revision != 1 {
		t.Errorf("Revision = %d, want unchanged 1", repo.groups[0].Revision)
	}
}

func TestService_CreateGroup_RejectsCrossOrganizationParent(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{groups: []*groupsdomain.Group{
		{ID: "parent-1", OrganizationID: "org-2", Name: "parent", DisplayName: "Parent", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "019test-group-id-aaaa"}, UOW: testUnitOfWork{groupRepo: repo}}
	_, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1", Name: "child", DisplayName: "Child", Type: "virtual", ParentID: "parent-1",
	})
	requireGroupError(t, err, apperrors.CodeUnprocessableEntity)
}

func TestService_PatchGroup_RejectsCycle(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{groups: []*groupsdomain.Group{
		{ID: "group-1", OrganizationID: "org-1", Name: "root", DisplayName: "Root", Revision: 1, CreatedAt: now, UpdatedAt: now},
		{ID: "group-2", OrganizationID: "org-1", Name: "child", DisplayName: "Child", ParentID: "group-1", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{groupRepo: repo}}
	parentID := "group-2"
	err := svc.PatchGroup(context.Background(), "org-1", PatchGroupCommand{ID: "group-1", ParentID: &parentID, Revision: 1})
	requireGroupError(t, err, apperrors.CodeUnprocessableEntity)
	if repo.groups[0].ParentID != "" {
		t.Fatalf("cycle patch must not persist, parent = %q", repo.groups[0].ParentID)
	}
}

func TestService_PatchGroup_RejectsSelfParent(t *testing.T) {
	now := time.Now()
	repo := &mockGroupRepo{groups: []*groupsdomain.Group{
		{ID: "group-1", OrganizationID: "org-1", Name: "root", DisplayName: "Root", Revision: 1, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{groupRepo: repo}}
	self := "group-1"
	err := svc.PatchGroup(context.Background(), "org-1", PatchGroupCommand{ID: "group-1", ParentID: &self, Revision: 1})
	requireGroupError(t, err, apperrors.CodeUnprocessableEntity)
}

func requireGroupError(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != code {
		t.Fatalf("error = %v, want code %s", err, code)
	}
}

const memberUserID = "019d0000-0000-7000-8000-0000000000a1"

func TestService_CreateGroup_ValidatesMemberOrganization(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	userRepo := usermock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), memberUserID, "org-1").Return(&userdomain.User{ID: memberUserID, OrganizationID: "org-1"}, nil)
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-bbbb"},
		UOW:   testUnitOfWork{groupRepo: &mockGroupRepo{}, userRepo: userRepo},
	}
	_, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1", Name: "engineering", DisplayName: "Engineering", Type: "department",
		Users: []string{memberUserID},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
}

func TestService_CreateGroup_RejectsCrossOrganizationMember(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	userRepo := usermock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByIDForOrganization(gomock.Any(), memberUserID, "org-1").Return(nil, apperrors.NewNotFound("user not found"))
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-cccc"},
		UOW:   testUnitOfWork{groupRepo: &mockGroupRepo{}, userRepo: userRepo},
	}
	_, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1", Name: "engineering", DisplayName: "Engineering", Type: "department",
		Users: []string{memberUserID},
	})
	requireGroupError(t, err, apperrors.CodeUnprocessableEntity)
}

func TestService_CreateGroup_ResolvesMemberByUsername(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	groupRepo := &mockGroupRepo{}
	userRepo := usermock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "alice").Return(&userdomain.User{ID: memberUserID, OrganizationID: "org-1"}, nil)
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-dddd"},
		UOW:   testUnitOfWork{groupRepo: groupRepo, userRepo: userRepo},
	}
	_, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1", Name: "engineering", DisplayName: "Engineering", Type: "department",
		Users: []string{"alice"},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	// 成员原样落库（username 口径，与种子与 Casbin sub 一致），仅做存在性校验。
	if len(groupRepo.groups) != 1 {
		t.Fatalf("expected 1 created group, got %d", len(groupRepo.groups))
	}
	if got := groupRepo.groups[0].Users; len(got) != 1 || got[0] != "alice" {
		t.Fatalf("created.Users = %v, want [alice]", got)
	}
}

func TestService_CreateGroup_RejectsUnknownMemberUsername(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	now := time.Now()
	userRepo := usermock.NewMockRepository(ctrl)
	userRepo.EXPECT().GetByUsername(gomock.Any(), "org-1", "ghost").Return(nil, apperrors.NewNotFound("user not found"))
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-group-id-eeee"},
		UOW:   testUnitOfWork{groupRepo: &mockGroupRepo{}, userRepo: userRepo},
	}
	_, err := svc.CreateGroup(context.Background(), CreateGroupCommand{
		OrganizationID: "org-1", Name: "engineering", DisplayName: "Engineering", Type: "department",
		Users: []string{"ghost"},
	})
	requireGroupError(t, err, apperrors.CodeUnprocessableEntity)
}
