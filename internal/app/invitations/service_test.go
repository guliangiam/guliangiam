package invitations

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
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
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
// Mock invitation repository
// ---------------------------------------------------------------------------

type mockInvitationRepo struct {
	invitations []*domain.Invitation
	// getByCodeErr overrides GetByCode with an infrastructure failure so
	// tests can distinguish NotFound from outage (BE-A-15).
	getByCodeErr error
}

func (r *mockInvitationRepo) GetByID(ctx context.Context, id string) (*domain.Invitation, error) {
	for _, inv := range r.invitations {
		if inv.ID == id {
			return inv, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *mockInvitationRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Invitation, error) {
	for _, inv := range r.invitations {
		if inv.ID == id && inv.OrganizationID == organizationID {
			return inv, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *mockInvitationRepo) GetByCode(ctx context.Context, code string) (*domain.Invitation, error) {
	if r.getByCodeErr != nil {
		return nil, r.getByCodeErr
	}
	for _, inv := range r.invitations {
		if inv.Code == code {
			return inv, nil
		}
	}
	return nil, apperrors.NewNotFound("invitation not found")
}

func (r *mockInvitationRepo) GetByEmail(ctx context.Context, organizationID, email string) (*domain.Invitation, error) {
	return nil, apperrors.NewNotFound("not found")
}

func (r *mockInvitationRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Invitation, int64, error) {
	return r.invitations, int64(len(r.invitations)), nil
}

func (r *mockInvitationRepo) Create(ctx context.Context, invitation *domain.Invitation) error {
	r.invitations = append(r.invitations, invitation)
	return nil
}

func (r *mockInvitationRepo) Update(ctx context.Context, invitation *domain.Invitation) error {
	for i, inv := range r.invitations {
		if inv.ID == invitation.ID {
			r.invitations[i] = invitation
			return nil
		}
	}
	return apperrors.NewNotFound("invitation not found")
}

func (r *mockInvitationRepo) Delete(ctx context.Context, id string) error {
	for i, inv := range r.invitations {
		if inv.ID == id {
			r.invitations = append(r.invitations[:i], r.invitations[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("invitation not found")
}

// ---------------------------------------------------------------------------
// Mock organizations repository（仅 GetByID 有数据，供 Verify 的组织元数据解析）
// ---------------------------------------------------------------------------

type mockOrganizationsRepo struct {
	organizations map[string]*orgdomain.Organization
}

func (r *mockOrganizationsRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	if r == nil {
		return nil, apperrors.NewNotFound("organization not found")
	}
	if org, ok := r.organizations[id]; ok {
		return org, nil
	}
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *mockOrganizationsRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *mockOrganizationsRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *mockOrganizationsRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, nil
}

func (r *mockOrganizationsRepo) Create(ctx context.Context, organization *orgdomain.Organization) error {
	return apperrors.NewInternal("not implemented")
}

func (r *mockOrganizationsRepo) Update(ctx context.Context, organization *orgdomain.Organization) error {
	return apperrors.NewInternal("not implemented")
}

func (r *mockOrganizationsRepo) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	return apperrors.NewInternal("not implemented")
}

func (r *mockOrganizationsRepo) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	return apperrors.NewInternal("not implemented")
}

func (r *mockOrganizationsRepo) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	return apperrors.NewInternal("not implemented")
}

func (r *mockOrganizationsRepo) Delete(ctx context.Context, id string) error {
	return apperrors.NewInternal("not implemented")
}

func (r *mockOrganizationsRepo) HasDeletedWithSlug(ctx context.Context, slug string) (bool, error) {
	return false, nil
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	invitationRepo *mockInvitationRepo
	orgRepo        *mockOrganizationsRepo
}

func (u testUnitOfWork) Applications() applications.Repository                        { return nil }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() domain.Repository                               { return u.invitationRepo }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository                          { return u.orgRepo }
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

func TestService_CreateInvitation_Validation(t *testing.T) {
	repo := &mockInvitationRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-inv-id-aaaaaa", altID: "019test-code-id-bbbbbb"},
		UOW:   testUnitOfWork{invitationRepo: repo},
	}

	tests := []struct {
		name    string
		command CreateInvitationCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateInvitationCommand{
				OrganizationID: "org-1",
				Email:          "user@example.com",
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateInvitationCommand{
				Email: "user@example.com",
			},
			wantErr: true,
		},
		{
			name: "missing email",
			command: CreateInvitationCommand{
				OrganizationID: "org-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateInvitation(context.Background(), tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateInvitation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_CreateInvitation_Success(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  &fakeUUIDGenerator{fixedID: "019test-inv-id-aaaaaa", altID: "019test-code-id-bbbbbb"},
		UOW:   testUnitOfWork{invitationRepo: repo},
	}

	inv, err := svc.CreateInvitation(context.Background(), CreateInvitationCommand{
		OrganizationID: "org-1",
		Email:          "user@example.com",
		RoleIDs:        []string{"role-1"},
		GroupIDs:       []string{"group-1"},
	})
	if err != nil {
		t.Fatalf("CreateInvitation() error = %v", err)
	}
	if inv.ID != "019test-inv-id-aaaaaa" {
		t.Errorf("ID = %q, want %q", inv.ID, "019test-inv-id-aaaaaa")
	}
	if inv.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", inv.OrganizationID, "org-1")
	}
	if inv.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", inv.Email, "user@example.com")
	}
	if inv.Status != domain.StatusPending {
		t.Errorf("Status = %q, want %q", inv.Status, domain.StatusPending)
	}
	if inv.Revision != 1 {
		t.Errorf("Revision = %d, want 1", inv.Revision)
	}
	if len(repo.invitations) != 1 {
		t.Errorf("expected 1 invitation in repo, got %d", len(repo.invitations))
	}
}

func TestService_ValidateInvitationCode_Valid(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{
				ID:             "inv-1",
				OrganizationID: "org-1",
				Email:          "user@example.com",
				Code:           "abc123",
				Status:         domain.StatusPending,
				ExpiresAt:      now.Add(7 * 24 * time.Hour),
				RoleIDs:        []string{"role-1"},
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	result, err := svc.ValidateInvitationCode(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("ValidateInvitationCode() error = %v", err)
	}
	if !result.Valid {
		t.Error("expected Valid = true")
	}
	if result.OrganizationID != "org-1" {
		t.Errorf("OrganizationID = %q, want %q", result.OrganizationID, "org-1")
	}
	if result.Email != "user@example.com" {
		t.Errorf("Email = %q, want %q", result.Email, "user@example.com")
	}
}

func TestService_ValidateInvitationCode_Expired(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{
				ID:        "inv-1",
				Code:      "abc123",
				Status:    domain.StatusPending,
				ExpiresAt: now.Add(-1 * time.Hour), // expired
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	result, err := svc.ValidateInvitationCode(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("ValidateInvitationCode() error = %v", err)
	}
	if result.Valid {
		t.Error("expected Valid = false for expired invitation")
	}
}

func TestService_ValidateInvitationCode_NotFound(t *testing.T) {
	repo := &mockInvitationRepo{}
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{invitationRepo: repo}}

	result, err := svc.ValidateInvitationCode(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("ValidateInvitationCode() error = %v", err)
	}
	if result.Valid {
		t.Error("expected Valid = false for not-found code")
	}
}

func TestService_AcceptInvitation(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{
				ID:        "inv-1",
				Code:      "abc123",
				Status:    domain.StatusPending,
				ExpiresAt: now.Add(7 * 24 * time.Hour),
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.AcceptInvitation(context.Background(), AcceptInvitationCommand{Code: "abc123"})
	if err != nil {
		t.Fatalf("AcceptInvitation() error = %v", err)
	}
	if repo.invitations[0].Status != domain.StatusAccepted {
		t.Errorf("Status = %q, want %q", repo.invitations[0].Status, domain.StatusAccepted)
	}
	if repo.invitations[0].AcceptedAt == nil {
		t.Error("expected AcceptedAt to be set")
	}
}

func TestService_AcceptInvitation_Expired(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{
				ID:        "inv-1",
				Code:      "abc123",
				Status:    domain.StatusPending,
				ExpiresAt: now.Add(-1 * time.Hour),
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.AcceptInvitation(context.Background(), AcceptInvitationCommand{Code: "abc123"})
	if err == nil {
		t.Error("expected error for expired invitation, got nil")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", code, apperrors.CodeUnprocessableEntity)
	}
}

func TestService_AcceptInvitation_NotPending(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{
				ID:        "inv-1",
				Code:      "abc123",
				Status:    domain.StatusRevoked,
				ExpiresAt: now.Add(time.Hour),
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.AcceptInvitation(context.Background(), AcceptInvitationCommand{Code: "abc123"})
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want 422", appErr.Code)
	}
	if appErr.Meta["field"] != "status" || appErr.Meta["value"] != domain.StatusRevoked {
		t.Errorf("meta = %v, want field=status value=%s", appErr.Meta, domain.StatusRevoked)
	}
}

func TestService_AcceptInvitation_NotFound(t *testing.T) {
	repo := &mockInvitationRepo{}
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.AcceptInvitation(context.Background(), AcceptInvitationCommand{Code: "nonexistent"})
	if err == nil {
		t.Error("expected error for not-found invitation, got nil")
	}
}

func TestService_RevokeInvitation(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{ID: "inv-1", OrganizationID: "org-1", Status: domain.StatusPending, Revision: 1},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.RevokeInvitation(context.Background(), RevokeInvitationCommand{ID: "inv-1", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("RevokeInvitation() error = %v", err)
	}
	if repo.invitations[0].Status != "revoked" {
		t.Errorf("Status = %q, want %q", repo.invitations[0].Status, "revoked")
	}
}

func TestService_RevokeInvitation_EmptyID(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{}}

	err := svc.RevokeInvitation(context.Background(), RevokeInvitationCommand{ID: "", OrganizationID: "org-1"})
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

func TestService_RevokeInvitation_AlreadyRevoked(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{ID: "inv-1", OrganizationID: "org-1", Status: "revoked"},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.RevokeInvitation(context.Background(), RevokeInvitationCommand{ID: "inv-1", OrganizationID: "org-1"})
	if err == nil {
		t.Error("expected error for already-revoked invitation, got nil")
	}
}

func TestService_ResendInvitation(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{ID: "inv-1", OrganizationID: "org-1", Status: domain.StatusPending, ExpiresAt: now, Revision: 1},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.ResendInvitation(context.Background(), ResendInvitationCommand{ID: "inv-1", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("ResendInvitation() error = %v", err)
	}
	// ExpiresAt should be extended to now + 7 days
	expectedExpiry := now.Add(7 * 24 * time.Hour)
	if !repo.invitations[0].ExpiresAt.Equal(expectedExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", repo.invitations[0].ExpiresAt, expectedExpiry)
	}
}

func TestService_ResendInvitation_NotPending(t *testing.T) {
	now := time.Now()
	repo := &mockInvitationRepo{
		invitations: []*domain.Invitation{
			{ID: "inv-1", OrganizationID: "org-1", Status: domain.StatusAccepted},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{invitationRepo: repo}}

	err := svc.ResendInvitation(context.Background(), ResendInvitationCommand{ID: "inv-1", OrganizationID: "org-1"})
	if err == nil {
		t.Error("expected error for non-pending invitation, got nil")
	}
}

func TestService_ResendInvitation_EmptyID(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{}}

	err := svc.ResendInvitation(context.Background(), ResendInvitationCommand{ID: "", OrganizationID: "org-1"})
	if err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

// ---------------------------------------------------------------------------
// 批次7：有效期天数、Verify/Send 拒绝路径回归
// ---------------------------------------------------------------------------

func TestService_CreateInvitation_ValidDays(t *testing.T) {
	now := time.Now()
	newSvc := func() *Service {
		return &Service{
			Clock: fakeClock{fixedTime: now},
			UUID:  &fakeUUIDGenerator{fixedID: "019test-inv-id-aaaaaa", altID: "019test-code-id-bbbbbb"},
			UOW:   testUnitOfWork{invitationRepo: &mockInvitationRepo{}},
		}
	}

	t.Run("validDays=3 sets expiry to now+3d", func(t *testing.T) {
		inv, err := newSvc().CreateInvitation(context.Background(), CreateInvitationCommand{
			OrganizationID: "org-1",
			Email:          "user@example.com",
			ValidDays:      3,
		})
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		want := now.Add(3 * 24 * time.Hour)
		if !inv.ExpiresAt.Equal(want) {
			t.Errorf("ExpiresAt = %v, want %v", inv.ExpiresAt, want)
		}
	})

	t.Run("validDays=0 falls back to default 7d", func(t *testing.T) {
		inv, err := newSvc().CreateInvitation(context.Background(), CreateInvitationCommand{
			OrganizationID: "org-1",
			Email:          "user@example.com",
		})
		if err != nil {
			t.Fatalf("CreateInvitation() error = %v", err)
		}
		want := now.Add(7 * 24 * time.Hour)
		if !inv.ExpiresAt.Equal(want) {
			t.Errorf("ExpiresAt = %v, want %v", inv.ExpiresAt, want)
		}
	})

	t.Run("validDays out of range is rejected", func(t *testing.T) {
		for _, days := range []int{-1, 366, 1000} {
			_, err := newSvc().CreateInvitation(context.Background(), CreateInvitationCommand{
				OrganizationID: "org-1",
				Email:          "user@example.com",
				ValidDays:      days,
			})
			if err == nil {
				t.Errorf("expected error for validDays=%d, got nil", days)
			}
		}
	})
}

func TestService_VerifyInvitation(t *testing.T) {
	now := time.Now()
	pending := &domain.Invitation{
		ID:             "inv-1",
		OrganizationID: "org-1",
		Email:          "user@example.com",
		Code:           "pending-code",
		Status:         domain.StatusPending,
		ExpiresAt:      now.Add(24 * time.Hour),
		RoleIDs:        []string{"role-1"},
		GroupIDs:       []string{"group-1"},
	}
	expired := &domain.Invitation{
		ID: "inv-2", OrganizationID: "org-1", Code: "expired-code",
		Status: domain.StatusPending, ExpiresAt: now.Add(-time.Hour),
	}
	accepted := &domain.Invitation{
		ID: "inv-3", OrganizationID: "org-1", Code: "accepted-code",
		Status: domain.StatusAccepted, ExpiresAt: now.Add(24 * time.Hour),
	}
	revoked := &domain.Invitation{
		ID: "inv-4", OrganizationID: "org-1", Code: "revoked-code",
		Status: domain.StatusRevoked, ExpiresAt: now.Add(24 * time.Hour),
	}
	newSvc := func() *Service {
		return &Service{
			Clock: fakeClock{fixedTime: now},
			UOW: testUnitOfWork{
				invitationRepo: &mockInvitationRepo{invitations: []*domain.Invitation{pending, expired, accepted, revoked}},
				orgRepo:        &mockOrganizationsRepo{organizations: map[string]*orgdomain.Organization{"org-1": {ID: "org-1", Slug: "built-in"}}},
			},
		}
	}

	t.Run("valid pending returns details with organization slug", func(t *testing.T) {
		result, err := newSvc().VerifyInvitation(context.Background(), "pending-code")
		if err != nil {
			t.Fatalf("VerifyInvitation() error = %v", err)
		}
		if result.OrganizationID != "org-1" || result.Email != "user@example.com" {
			t.Errorf("unexpected result: %+v", result)
		}
		if result.OrganizationSlug != "built-in" {
			t.Errorf("OrganizationSlug = %q, want %q", result.OrganizationSlug, "built-in")
		}
		if len(result.RoleIDs) != 1 || len(result.GroupIDs) != 1 {
			t.Errorf("roleIds/groupIds not propagated: %+v", result)
		}
	})

	t.Run("empty code is rejected", func(t *testing.T) {
		if _, err := newSvc().VerifyInvitation(context.Background(), ""); err == nil {
			t.Error("expected error for empty code, got nil")
		}
	})

	t.Run("unknown code is rejected as not found", func(t *testing.T) {
		_, err := newSvc().VerifyInvitation(context.Background(), "nope")
		if err == nil {
			t.Fatal("expected error for unknown code, got nil")
		}
		if code := apperrors.FromError(err).Code; code != apperrors.CodeNotFound {
			t.Errorf("error code = %q, want %q", code, apperrors.CodeNotFound)
		}
	})

	t.Run("expired invitation is rejected", func(t *testing.T) {
		_, err := newSvc().VerifyInvitation(context.Background(), "expired-code")
		if err == nil {
			t.Fatal("expected error for expired invitation, got nil")
		}
		if code := apperrors.FromError(err).Code; code != apperrors.CodeInvalidArgument {
			t.Errorf("error code = %q, want %q", code, apperrors.CodeInvalidArgument)
		}
	})

	t.Run("accepted invitation is rejected as conflict", func(t *testing.T) {
		_, err := newSvc().VerifyInvitation(context.Background(), "accepted-code")
		if err == nil {
			t.Fatal("expected error for accepted invitation, got nil")
		}
		if code := apperrors.FromError(err).Code; code != apperrors.CodeConflict {
			t.Errorf("error code = %q, want %q", code, apperrors.CodeConflict)
		}
	})

	t.Run("revoked invitation is rejected", func(t *testing.T) {
		_, err := newSvc().VerifyInvitation(context.Background(), "revoked-code")
		if err == nil {
			t.Fatal("expected error for revoked invitation, got nil")
		}
		if code := apperrors.FromError(err).Code; code != apperrors.CodeInvalidArgument {
			t.Errorf("error code = %q, want %q", code, apperrors.CodeInvalidArgument)
		}
	})

	t.Run("organization lookup failure keeps verify usable", func(t *testing.T) {
		svc := &Service{
			Clock: fakeClock{fixedTime: now},
			UOW: testUnitOfWork{
				invitationRepo: &mockInvitationRepo{invitations: []*domain.Invitation{pending}},
			},
		}
		result, err := svc.VerifyInvitation(context.Background(), "pending-code")
		if err != nil {
			t.Fatalf("VerifyInvitation() error = %v", err)
		}
		if result.OrganizationSlug != "" {
			t.Errorf("OrganizationSlug = %q, want empty when org is invisible", result.OrganizationSlug)
		}
	})
}

func TestService_SendInvitation(t *testing.T) {
	now := time.Now()
	newSvc := func(status string) *Service {
		return &Service{
			Clock: fakeClock{fixedTime: now},
			UOW: testUnitOfWork{invitationRepo: &mockInvitationRepo{
				invitations: []*domain.Invitation{
					{ID: "inv-1", OrganizationID: "org-1", Status: status, ExpiresAt: now.Add(24 * time.Hour)},
				},
			}},
		}
	}

	t.Run("pending invitation can be sent", func(t *testing.T) {
		if err := newSvc(domain.StatusPending).SendInvitation(context.Background(), "org-1", "inv-1"); err != nil {
			t.Fatalf("SendInvitation() error = %v", err)
		}
	})

	t.Run("revoked invitation cannot be sent", func(t *testing.T) {
		if err := newSvc(domain.StatusRevoked).SendInvitation(context.Background(), "org-1", "inv-1"); err == nil {
			t.Error("expected error for revoked invitation, got nil")
		}
	})

	t.Run("accepted invitation cannot be sent", func(t *testing.T) {
		if err := newSvc(domain.StatusAccepted).SendInvitation(context.Background(), "org-1", "inv-1"); err == nil {
			t.Error("expected error for accepted invitation, got nil")
		}
	})

	t.Run("unknown invitation is rejected", func(t *testing.T) {
		if err := newSvc(domain.StatusPending).SendInvitation(context.Background(), "org-1", "nope"); err == nil {
			t.Error("expected error for unknown invitation, got nil")
		}
	})

	t.Run("empty organization is rejected", func(t *testing.T) {
		if err := newSvc(domain.StatusPending).SendInvitation(context.Background(), "", "inv-1"); err == nil {
			t.Error("expected error for empty organizationId, got nil")
		}
	})
}
