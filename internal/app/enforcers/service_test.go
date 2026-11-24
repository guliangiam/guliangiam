package enforcers

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
	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
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
// Mock enforcer repository
// ---------------------------------------------------------------------------

type mockEnforcerRepo struct {
	enforcers []*domain.Enforcer
}

func (r *mockEnforcerRepo) GetByID(ctx context.Context, id string) (*domain.Enforcer, error) {
	return r.find(id, "")
}

func (r *mockEnforcerRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Enforcer, error) {
	return r.find(id, organizationID)
}

func (r *mockEnforcerRepo) find(id, organizationID string) (*domain.Enforcer, error) {
	for _, e := range r.enforcers {
		if e.ID != id {
			continue
		}
		if organizationID == "" || e.OrganizationID == organizationID {
			return e, nil
		}
	}
	return nil, apperrors.NewNotFound("enforcer not found")
}

func (r *mockEnforcerRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Enforcer, int64, error) {
	return r.enforcers, int64(len(r.enforcers)), nil
}

func (r *mockEnforcerRepo) Create(ctx context.Context, e *domain.Enforcer) error {
	r.enforcers = append(r.enforcers, e)
	return nil
}

func (r *mockEnforcerRepo) Update(ctx context.Context, e *domain.Enforcer) error {
	for i, en := range r.enforcers {
		if en.ID == e.ID {
			r.enforcers[i] = e
			return nil
		}
	}
	return apperrors.NewNotFound("enforcer not found")
}

func (r *mockEnforcerRepo) Delete(ctx context.Context, id, organizationID string) error {
	for i, e := range r.enforcers {
		if e.ID != id {
			continue
		}
		if e.OrganizationID != organizationID {
			continue
		}
		r.enforcers = append(r.enforcers[:i], r.enforcers[i+1:]...)
		return nil
	}
	return apperrors.NewNotFound("enforcer not found")
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

// validateEnforcerRefs (BE-D-12) resolves model/adapter ownership on every
// create; these stubs accept any id within the caller's organization.
type stubModelRepo struct{ modeldomain.Repository }

func (stubModelRepo) GetByIDForOrganization(_ context.Context, id, _ string) (*modeldomain.Model, error) {
	return &modeldomain.Model{ID: id, ModelText: "model_text"}, nil
}

type stubAdapterRepo struct{ adapterdomain.Repository }

func (stubAdapterRepo) GetByID(_ context.Context, id, _ string) (*adapterdomain.Adapter, error) {
	return &adapterdomain.Adapter{ID: id}, nil
}

type testUnitOfWork struct {
	enforcerRepo *mockEnforcerRepo
}

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWork) Enforcers() domain.Repository                                 { return u.enforcerRepo }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository                          { return nil }
func (u testUnitOfWork) Permissions() permissiondomain.Repository                     { return nil }
func (u testUnitOfWork) Providers() providerdomain.Repository                         { return nil }
func (u testUnitOfWork) Certs() certdomain.Repository                                 { return nil }
func (u testUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u testUnitOfWork) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u testUnitOfWork) Notifications() notificationsdomain.Repository                { return nil }
func (u testUnitOfWork) Ldaps() ldapdomain.Repository                                 { return nil }
func (u testUnitOfWork) Models() modeldomain.Repository                               { return stubModelRepo{} }
func (u testUnitOfWork) Adapters() adapterdomain.Repository                           { return stubAdapterRepo{} }
func (u testUnitOfWork) Agents() agentdomain.Repository                               { return nil }

func (u testUnitOfWork) Forms() formdomain.Repository       { return nil }
func (u testUnitOfWork) Orders() orderdomain.Repository     { return nil }
func (u testUnitOfWork) Payments() paymentdomain.Repository { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository       { return nil }

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

func (u testUnitOfWork) Rebac() rebac.Repository { return nil }

type testUnitOfWorkWrap struct {
	enforcerRepo *mockEnforcerRepoWrap
}

func (u testUnitOfWorkWrap) Outbox() domainoutbox.Repository { return nil }
func (u testUnitOfWorkWrap) Risk() riskdomain.Repository     { return nil }

func (u testUnitOfWorkWrap) Workload() workloaddomain.Repository { return nil }
func (u testUnitOfWork) Risk() riskdomain.Repository             { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) IDV() idvdomain.Repository { return nil }

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

func TestService_CreateEnforcer(t *testing.T) {
	now := time.Now()
	repo := &mockEnforcerRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "019enf-test-id-aaaaaa"}, UOW: testUnitOfWork{enforcerRepo: repo}}
	ctx := context.Background()
	err := svc.CreateEnforcer(ctx, CreateEnforcerCommand{OrganizationID: "org-001", Name: "enforcer-rbac", DisplayName: "RBAC Enforcer", ModelID: "model-001", AdapterID: "adapter-001"})
	if err != nil {
		t.Fatalf("CreateEnforcer() error = %v", err)
	}
	if len(repo.enforcers) != 1 {
		t.Fatalf("expected 1 enforcer, got %d", len(repo.enforcers))
	}
	e := repo.enforcers[0]
	if e.ID != "019enf-test-id-aaaaaa" {
		t.Errorf("expected ID '019enf-test-id-aaaaaa', got %q", e.ID)
	}
	if !e.IsEnabled {
		t.Errorf("expected IsEnabled true by default, got false")
	}
	if e.Revision != 1 {
		t.Errorf("expected Revision 1, got %d", e.Revision)
	}
}

func TestService_CreateEnforcer_DisabledByDefault(t *testing.T) {
	now := time.Now()
	repo := &mockEnforcerRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UUID: &fakeUUIDGenerator{fixedID: "019enf-test-id-bbbbbb"}, UOW: testUnitOfWork{enforcerRepo: repo}}
	ctx := context.Background()
	disabled := false
	err := svc.CreateEnforcer(ctx, CreateEnforcerCommand{OrganizationID: "org-001", Name: "enforcer-abac", IsEnabled: &disabled})
	if err != nil {
		t.Fatalf("CreateEnforcer() error = %v", err)
	}
	if repo.enforcers[0].IsEnabled {
		t.Errorf("expected IsEnabled false, got true")
	}
}

func TestService_CreateEnforcer_Validation(t *testing.T) {
	svc := Service{Clock: fakeClock{fixedTime: time.Now()}, UUID: &fakeUUIDGenerator{fixedID: "test"}, UOW: testUnitOfWork{enforcerRepo: &mockEnforcerRepo{}}}
	ctx := context.Background()
	tests := []struct {
		name    string
		cmd     CreateEnforcerCommand
		wantErr string
	}{
		{name: "missing organizationId", cmd: CreateEnforcerCommand{Name: "enf-1"}, wantErr: "organizationId is required"},
		{name: "missing name", cmd: CreateEnforcerCommand{OrganizationID: "org-1"}, wantErr: "name is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.CreateEnforcer(ctx, tt.cmd)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("expected error %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestService_DeleteEnforcer(t *testing.T) {
	now := time.Now()
	repo := &mockEnforcerRepo{enforcers: []*domain.Enforcer{{ID: "enf-1", OrganizationID: "org-001", Name: "rbac", IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now}}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{enforcerRepo: repo}}
	ctx := context.Background()
	err := svc.DeleteEnforcer(ctx, "org-001", "enf-1")
	if err != nil {
		t.Fatalf("DeleteEnforcer() error = %v", err)
	}
	if len(repo.enforcers) != 0 {
		t.Errorf("expected 0 enforcers after delete, got %d", len(repo.enforcers))
	}
}

func TestService_GetEnforcer(t *testing.T) {
	now := time.Now()
	enf := &domain.Enforcer{
		ID: "enf-1", OrganizationID: "org-001", Name: "rbac",
		DisplayName: "RBAC Enforcer", Description: "Role-based access control",
		ModelID: "model-1", AdapterID: "adapter-1",
		IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	repo := &mockEnforcerRepo{enforcers: []*domain.Enforcer{enf}}
	svc := Service{UOW: testUnitOfWork{enforcerRepo: repo}}
	ctx := context.Background()

	detail, err := svc.GetEnforcer(ctx, "org-001", "enf-1")
	if err != nil {
		t.Fatalf("GetEnforcer() error = %v", err)
	}
	if detail.ID != "enf-1" {
		t.Errorf("expected ID 'enf-1', got %q", detail.ID)
	}
	if detail.Name != "rbac" {
		t.Errorf("expected Name 'rbac', got %q", detail.Name)
	}
	if detail.DisplayName != "RBAC Enforcer" {
		t.Errorf("expected DisplayName 'RBAC Enforcer', got %q", detail.DisplayName)
	}
	if detail.Description != "Role-based access control" {
		t.Errorf("expected Description 'Role-based access control', got %q", detail.Description)
	}
	if detail.Revision != 1 {
		t.Errorf("expected Revision 1, got %d", detail.Revision)
	}
}

func TestService_GetEnforcer_NotFound(t *testing.T) {
	repo := &mockEnforcerRepo{}
	svc := Service{UOW: testUnitOfWork{enforcerRepo: repo}}
	ctx := context.Background()

	_, err := svc.GetEnforcer(ctx, "org-001", "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeNotFound {
		t.Errorf("expected not_found error, got %v", err)
	}
}

func TestService_ListEnforcers(t *testing.T) {
	now := time.Now()
	repo := &mockEnforcerRepo{enforcers: []*domain.Enforcer{
		{ID: "enf-1", Name: "rbac", IsEnabled: true, CreatedAt: now, UpdatedAt: now},
		{ID: "enf-2", Name: "abac", IsEnabled: false, CreatedAt: now, UpdatedAt: now},
	}}
	svc := Service{UOW: testUnitOfWork{enforcerRepo: repo}}
	ctx := context.Background()

	items, total, err := svc.ListEnforcers(ctx, ListEnforcersQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListEnforcers() error = %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "enf-1" {
		t.Errorf("expected items[0].ID 'enf-1', got %q", items[0].ID)
	}
	if items[1].ID != "enf-2" {
		t.Errorf("expected items[1].ID 'enf-2', got %q", items[1].ID)
	}
	if !items[0].IsEnabled {
		t.Errorf("expected items[0].IsEnabled true, got false")
	}
	if items[1].IsEnabled {
		t.Errorf("expected items[1].IsEnabled false, got true")
	}
}

func TestService_PatchEnforcer(t *testing.T) {
	now := time.Now()
	enf := &domain.Enforcer{
		ID: "enf-1", OrganizationID: "org-001", Name: "rbac",
		DisplayName: "Before", Description: "Old desc",
		IsEnabled: true, Revision: 1, CreatedAt: now, UpdatedAt: now,
	}
	var updated *domain.Enforcer
	repo := &mockEnforcerRepo{enforcers: []*domain.Enforcer{enf}}
	origUpdate := repo.Update
	_ = origUpdate
	repoWrapped := &mockEnforcerRepoWrap{
		inner:    repo,
		onUpdate: func(e *domain.Enforcer) { updated = e },
	}
	svc := Service{Clock: fakeClock{fixedTime: now.Add(time.Minute)}, UOW: testUnitOfWorkWrap{enforcerRepo: repoWrapped}}
	ctx := context.Background()

	newName := "After"
	disabled := false
	err := svc.PatchEnforcer(ctx, "org-001", PatchEnforcerCommand{
		ID:          "enf-1",
		DisplayName: &newName,
		IsEnabled:   &disabled,
		Revision:    1,
	})
	if err != nil {
		t.Fatalf("PatchEnforcer() error = %v", err)
	}
	if updated == nil {
		t.Fatal("expected Update to be called")
	}
	if updated.DisplayName != "After" {
		t.Errorf("expected DisplayName 'After', got %q", updated.DisplayName)
	}
	if updated.IsEnabled {
		t.Errorf("expected IsEnabled false, got true")
	}
	if updated.Revision != 2 {
		t.Errorf("expected Revision 2, got %d", updated.Revision)
	}
}

func TestService_PatchEnforcer_RevisionMismatch(t *testing.T) {
	now := time.Now()
	enf := &domain.Enforcer{ID: "enf-1", OrganizationID: "org-001", Name: "rbac", Revision: 3, CreatedAt: now, UpdatedAt: now}
	repo := &mockEnforcerRepo{enforcers: []*domain.Enforcer{enf}}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{enforcerRepo: repo}}
	ctx := context.Background()

	displayName := "Updated"
	err := svc.PatchEnforcer(ctx, "org-001", PatchEnforcerCommand{ID: "enf-1", DisplayName: &displayName, Revision: 1})
	if err == nil {
		t.Fatal("expected revision mismatch error, got nil")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeConflict {
		t.Errorf("expected conflict error, got %v", err)
	}
}

func TestService_RefreshEngines(t *testing.T) {
	mgr := &mockCasbinManager{}
	svc := Service{CasbinManager: mgr}
	ctx := context.Background()

	err := svc.RefreshEngines(ctx)
	if err != nil {
		t.Fatalf("RefreshEngines() error = %v", err)
	}
	if !mgr.invalidateAllCalled {
		t.Error("expected InvalidateAll to be called")
	}
}

func TestService_Enforce_InvalidID(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{enforcerRepo: &mockEnforcerRepo{}}}
	ctx := context.Background()
	_, err := svc.Enforce(ctx, "org-1", EnforceCommand{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Mock wrapper for tracking Update calls
// ---------------------------------------------------------------------------

type mockEnforcerRepoWrap struct {
	inner    *mockEnforcerRepo
	onUpdate func(e *domain.Enforcer)
}

func (r *mockEnforcerRepoWrap) GetByID(ctx context.Context, id string) (*domain.Enforcer, error) {
	return r.inner.GetByID(ctx, id)
}
func (r *mockEnforcerRepoWrap) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Enforcer, error) {
	return r.inner.GetByIDForOrganization(ctx, id, organizationID)
}
func (r *mockEnforcerRepoWrap) List(ctx context.Context, query domain.ListQuery) ([]*domain.Enforcer, int64, error) {
	return r.inner.List(ctx, query)
}
func (r *mockEnforcerRepoWrap) Create(ctx context.Context, e *domain.Enforcer) error {
	return r.inner.Create(ctx, e)
}
func (r *mockEnforcerRepoWrap) Update(ctx context.Context, e *domain.Enforcer) error {
	if r.onUpdate != nil {
		r.onUpdate(e)
	}
	return r.inner.Update(ctx, e)
}
func (r *mockEnforcerRepoWrap) Delete(ctx context.Context, id, organizationID string) error {
	return r.inner.Delete(ctx, id, organizationID)
}

func (u testUnitOfWorkWrap) Applications() applications.Repository                        { return nil }
func (u testUnitOfWorkWrap) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWorkWrap) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWorkWrap) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWorkWrap) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWorkWrap) Invitations() invitationsdomain.Repository                    { return nil }
func (u testUnitOfWorkWrap) Enforcers() domain.Repository                                 { return u.enforcerRepo }
func (u testUnitOfWorkWrap) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u testUnitOfWorkWrap) Organizations() orgdomain.Repository                          { return nil }
func (u testUnitOfWorkWrap) Permissions() permissiondomain.Repository                     { return nil }
func (u testUnitOfWorkWrap) Providers() providerdomain.Repository                         { return nil }
func (u testUnitOfWorkWrap) Certs() certdomain.Repository                                 { return nil }
func (u testUnitOfWorkWrap) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u testUnitOfWorkWrap) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u testUnitOfWorkWrap) Notifications() notificationsdomain.Repository                { return nil }
func (u testUnitOfWorkWrap) Ldaps() ldapdomain.Repository                                 { return nil }
func (u testUnitOfWorkWrap) Models() modeldomain.Repository                               { return stubModelRepo{} }
func (u testUnitOfWorkWrap) Adapters() adapterdomain.Repository                           { return stubAdapterRepo{} }
func (u testUnitOfWorkWrap) Agents() agentdomain.Repository                               { return nil }
func (u testUnitOfWorkWrap) Forms() formdomain.Repository                                 { return nil }
func (u testUnitOfWorkWrap) Orders() orderdomain.Repository                               { return nil }
func (u testUnitOfWorkWrap) Payments() paymentdomain.Repository                           { return nil }
func (u testUnitOfWorkWrap) Plans() plandomain.Repository                                 { return nil }

func (u testUnitOfWorkWrap) PlatformAdmins() platformadmins.PlatformAdminRepository { return nil }
func (u testUnitOfWorkWrap) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u testUnitOfWorkWrap) CrossTenantOps() crosstenantops.TicketRepository      { return nil }
func (u testUnitOfWorkWrap) Pricings() pricingdomain.Repository                   { return nil }
func (u testUnitOfWorkWrap) Products() productdomain.Repository                   { return nil }
func (u testUnitOfWorkWrap) Records() recordsdomain.Repository                    { return nil }
func (u testUnitOfWorkWrap) Resources() resourcedomain.Repository                 { return nil }
func (u testUnitOfWorkWrap) Roles() roledomain.Repository                         { return nil }
func (u testUnitOfWorkWrap) Syncers() syncerdomain.Repository                     { return nil }
func (u testUnitOfWorkWrap) Sessions() sessiondomain.Repository                   { return nil }
func (u testUnitOfWorkWrap) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u testUnitOfWorkWrap) Tickets() ticketdomain.Repository                     { return nil }
func (u testUnitOfWorkWrap) Tokens() tokendomain.Repository                       { return nil }
func (u testUnitOfWorkWrap) Webhooks() webhookdomain.Repository                   { return nil }
func (u testUnitOfWorkWrap) Users() userdomain.Repository                         { return nil }
func (u testUnitOfWorkWrap) Coupons() coupondomain.Repository                     { return nil }
func (u testUnitOfWorkWrap) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u testUnitOfWorkWrap) Entries() entrydomain.Repository                      { return nil }
func (u testUnitOfWorkWrap) McpServers() mcpserverdomain.Repository               { return nil }
func (u testUnitOfWorkWrap) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u testUnitOfWorkWrap) Verifications() verificationsdomain.Repository        { return nil }
func (u testUnitOfWorkWrap) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u testUnitOfWorkWrap) FaceIdProviders() faceidproviderdomain.Repository     { return nil }
func (u testUnitOfWorkWrap) RadiusConfigs() radiusconfigdomain.Repository         { return nil }
func (u testUnitOfWorkWrap) Rules() ruledomain.Repository                         { return nil }
func (u testUnitOfWorkWrap) Transactions() transactiondomain.Repository           { return nil }
func (u testUnitOfWorkWrap) WebhookEvents() webhookeventdomain.Repository         { return nil }
func (u testUnitOfWorkWrap) Keys() keysdomain.Repository                          { return nil }
func (u testUnitOfWorkWrap) QRCodeSessions() qrcodesessions.Repository            { return nil }
func (u testUnitOfWorkWrap) DeviceAuthCodes() deviceauthcodes.Repository          { return nil }

func (u testUnitOfWorkWrap) Web3Identities() web3identity.Repository     { return nil }
func (u testUnitOfWorkWrap) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u testUnitOfWorkWrap) CASTickets() castickets.Repository           { return nil }
func (u testUnitOfWorkWrap) KerberosTickets() kerberostickets.Repository { return nil }
func (u testUnitOfWorkWrap) LockoutAttempts() lockoutattempts.Repository { return nil }
func (u testUnitOfWorkWrap) Rebac() rebac.Repository                     { return nil }

func (u testUnitOfWorkWrap) Actions() actionsdomain.Repository { return nil }
func (u testUnitOfWorkWrap) IDV() idvdomain.Repository         { return nil }

func (u testUnitOfWorkWrap) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWorkWrap) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWorkWrap) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Mock CasbinManager
// ---------------------------------------------------------------------------

type mockCasbinManager struct {
	invalidateAllCalled bool
}

func (m *mockCasbinManager) Enforce(ctx context.Context, enforcerID, modelText string, rvals ...interface{}) (bool, error) {
	return true, nil
}
func (m *mockCasbinManager) BatchEnforce(ctx context.Context, enforcerID, modelText string, rvals [][]interface{}) ([]bool, error) {
	return make([]bool, len(rvals)), nil
}
func (m *mockCasbinManager) AddPolicy(enforcerID, modelText string, params []string) error {
	return nil
}
func (m *mockCasbinManager) RemovePolicy(enforcerID, modelText string, params []string) error {
	return nil
}
func (m *mockCasbinManager) RemoveFilteredPolicy(enforcerID, modelText string, fieldIndex int, fieldValues ...string) error {
	return nil
}
func (m *mockCasbinManager) GetAllSubjects(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}
func (m *mockCasbinManager) GetAllObjects(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}
func (m *mockCasbinManager) GetAllActions(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}
func (m *mockCasbinManager) GetAllRoles(enforcerID, modelText string) ([]string, error) {
	return nil, nil
}
func (m *mockCasbinManager) AddRoleForUser(enforcerID, modelText, user, role string) error {
	return nil
}
func (m *mockCasbinManager) ReloadPolicy(enforcerID, modelText string) error {
	return nil
}
func (m *mockCasbinManager) GetPolicies(enforcerID, modelText string) ([][]string, error) {
	return nil, nil
}
func (m *mockCasbinManager) GetFilteredPolicies(enforcerID, modelText string, fieldIndex int, fieldValues ...string) ([][]string, error) {
	return nil, nil
}
func (m *mockCasbinManager) UpdatePolicy(enforcerID, modelText string, oldPolicy, newPolicy []string) error {
	return nil
}
func (m *mockCasbinManager) GetOrgEnforcerPolicies(orgID string) ([][]string, error) {
	return nil, nil
}
func (m *mockCasbinManager) AddOrgPolicy(orgID string, params []string) error {
	return nil
}
func (m *mockCasbinManager) RemoveOrgFilteredPolicy(orgID string, fieldIndex int, fieldValues ...string) error {
	return nil
}
func (m *mockCasbinManager) AddOrgRoleForUser(orgID, user, role string) error {
	return nil
}
func (m *mockCasbinManager) RemoveOrgFilteredGroupingPolicy(orgID string, fieldIndex int, fieldValues ...string) error {
	return nil
}
func (m *mockCasbinManager) InvalidateAll() {
	m.invalidateAllCalled = true
}

func (m *mockCasbinManager) GetOrgSubjectPolicies(orgID, subject string) ([][]string, error) {
	return nil, nil
}
