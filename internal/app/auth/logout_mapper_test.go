package auth

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
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
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
// Mock session repository for logout tests
// ---------------------------------------------------------------------------

type mockSessionRepoForLogout struct {
	session      *sessiondomain.Session
	revokeErr    error
	revokeCalled bool
}

func (r *mockSessionRepoForLogout) GetByID(ctx context.Context, id string) (*sessiondomain.Session, error) {
	if r.session != nil && r.session.ID == id {
		return r.session, nil
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepoForLogout) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*sessiondomain.Session, error) {
	session, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if session.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("session not found")
	}
	return session, nil
}

func (r *mockSessionRepoForLogout) List(ctx context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	return nil, 0, nil
}

func (r *mockSessionRepoForLogout) Create(ctx context.Context, session *sessiondomain.Session) error {
	return nil
}

func (r *mockSessionRepoForLogout) Update(ctx context.Context, session *sessiondomain.Session) error {
	return nil
}

func (r *mockSessionRepoForLogout) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	return nil
}

func (r *mockSessionRepoForLogout) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	r.revokeCalled = true
	return r.revokeErr
}

func (r *mockSessionRepoForLogout) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	if _, err := r.GetByIDForOrganization(ctx, id, organizationID); err != nil {
		return err
	}
	return r.Revoke(ctx, id, revokedAt)
}

func (r *mockSessionRepoForLogout) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForLogout) RevokeByUserAndApp(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForLogout) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForLogout) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *mockSessionRepoForLogout) Delete(ctx context.Context, id string) error { return nil }
func (r *mockSessionRepoForLogout) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	_, err := r.GetByIDForOrganization(ctx, id, organizationID)
	return err
}
func (r *mockSessionRepoForLogout) CleanupExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *mockSessionRepoForLogout) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Test UOW for logout tests
// ---------------------------------------------------------------------------

type logoutTestUOW struct {
	sessions  *mockSessionRepoForLogout
	providers providerdomain.Repository
}

func (u logoutTestUOW) Applications() applicationdomain.Repository                   { return nil }
func (u logoutTestUOW) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u logoutTestUOW) Consents() consentdomain.Repository                           { return nil }
func (u logoutTestUOW) Gateway() gatewaydomain.Repository                            { return nil }
func (u logoutTestUOW) Groups() groupsdomain.Repository                              { return nil }
func (u logoutTestUOW) Invitations() invitationsdomain.Repository                    { return nil }
func (u logoutTestUOW) Enforcers() enforcerdomain.Repository                         { return nil }
func (u logoutTestUOW) MfaChallenges() mfachallengedomain.Repository                 { return nil }
func (u logoutTestUOW) Organizations() orgdomain.Repository                          { return nil }
func (u logoutTestUOW) Permissions() permissiondomain.Repository                     { return nil }
func (u logoutTestUOW) Providers() providerdomain.Repository                         { return u.providers }
func (u logoutTestUOW) Certs() certdomain.Repository                                 { return nil }
func (u logoutTestUOW) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u logoutTestUOW) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u logoutTestUOW) Notifications() notificationsdomain.Repository                { return nil }
func (u logoutTestUOW) Ldaps() ldapdomain.Repository                                 { return nil }
func (u logoutTestUOW) Models() modeldomain.Repository                               { return nil }
func (u logoutTestUOW) Adapters() adapterdomain.Repository                           { return nil }
func (u logoutTestUOW) Agents() agentdomain.Repository                               { return nil }
func (u logoutTestUOW) Forms() formdomain.Repository                                 { return nil }
func (u logoutTestUOW) Orders() orderdomain.Repository                               { return nil }
func (u logoutTestUOW) Payments() paymentdomain.Repository                           { return nil }
func (u logoutTestUOW) Plans() plandomain.Repository                                 { return nil }

func (u logoutTestUOW) PlatformAdmins() platformadmins.PlatformAdminRepository           { return nil }
func (u logoutTestUOW) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository { return nil }
func (u logoutTestUOW) CrossTenantOps() crosstenantops.TicketRepository                  { return nil }
func (u logoutTestUOW) Pricings() pricingdomain.Repository                               { return nil }
func (u logoutTestUOW) Products() productdomain.Repository                               { return nil }
func (u logoutTestUOW) Records() recordsdomain.Repository                                { return nil }
func (u logoutTestUOW) Resources() resourcedomain.Repository                             { return nil }
func (u logoutTestUOW) Roles() roledomain.Repository                                     { return nil }
func (u logoutTestUOW) Syncers() syncerdomain.Repository                                 { return nil }
func (u logoutTestUOW) Sessions() sessiondomain.Repository                               { return u.sessions }
func (u logoutTestUOW) Subscriptions() subscriptiondomain.Repository                     { return nil }
func (u logoutTestUOW) Tickets() ticketdomain.Repository                                 { return nil }
func (u logoutTestUOW) Tokens() tokendomain.Repository                                   { return nil }
func (u logoutTestUOW) Webhooks() webhookdomain.Repository                               { return nil }
func (u logoutTestUOW) Users() userdomain.Repository                                     { return nil }
func (u logoutTestUOW) Coupons() coupondomain.Repository                                 { return nil }
func (u logoutTestUOW) CouponUsages() coupondomain.CouponUsageRepository                 { return nil }
func (u logoutTestUOW) Entries() entrydomain.Repository                                  { return nil }
func (u logoutTestUOW) McpServers() mcpserverdomain.Repository                           { return nil }
func (u logoutTestUOW) RadiusAccountings() radiusaccountingdomain.Repository             { return nil }
func (u logoutTestUOW) Verifications() verificationsdomain.Repository                    { return nil }
func (u logoutTestUOW) WebAuthnCredentials() webauthndomain.Repository                   { return nil }
func (u logoutTestUOW) FaceIdProviders() faceidproviderdomain.Repository                 { return nil }
func (u logoutTestUOW) RadiusConfigs() radiusconfigdomain.Repository                     { return nil }
func (u logoutTestUOW) Rules() ruledomain.Repository                                     { return nil }
func (u logoutTestUOW) Transactions() transactiondomain.Repository                       { return nil }
func (u logoutTestUOW) WebhookEvents() webhookeventdomain.Repository                     { return nil }
func (u logoutTestUOW) Keys() keysdomain.Repository                                      { return nil }
func (u logoutTestUOW) QRCodeSessions() qrcodesessions.Repository                        { return nil }
func (u logoutTestUOW) DeviceAuthCodes() deviceauthcodes.Repository                      { return nil }

func (u logoutTestUOW) Web3Identities() web3identity.Repository     { return nil }
func (u logoutTestUOW) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u logoutTestUOW) CASTickets() castickets.Repository           { return nil }
func (u logoutTestUOW) KerberosTickets() kerberostickets.Repository { return nil }
func (u logoutTestUOW) LockoutAttempts() lockoutattempts.Repository { return nil }

func (u logoutTestUOW) Rebac() rebac.Repository     { return nil }
func (u logoutTestUOW) Risk() riskdomain.Repository { return nil }

func (u logoutTestUOW) Workload() workloaddomain.Repository { return nil }

func (u logoutTestUOW) IDV() idvdomain.Repository { return nil }

func (u logoutTestUOW) Actions() actionsdomain.Repository { return nil }

func (u logoutTestUOW) Metering() meteringdomain.Repository { return nil }

func (u logoutTestUOW) Outbox() domainoutbox.Repository { return nil }

func (u logoutTestUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u logoutTestUOW) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLogout_Success(t *testing.T) {
	now := time.Now()
	session := &sessiondomain.Session{
		ID:     "sess-001",
		UserID: "user-001",
	}
	repo := &mockSessionRepoForLogout{session: session}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   logoutTestUOW{sessions: repo},
	}
	ctx := context.Background()

	err := svc.Logout(ctx, LogoutCommand{SessionID: "sess-001"})
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if !repo.revokeCalled {
		t.Error("expected Revoke to be called")
	}
}

func TestLogout_EmptySessionID(t *testing.T) {
	repo := &mockSessionRepoForLogout{}
	svc := Service{
		Clock: fakeClock{fixedTime: time.Now()},
		UOW:   logoutTestUOW{sessions: repo},
	}
	ctx := context.Background()

	err := svc.Logout(ctx, LogoutCommand{SessionID: ""})
	if err != nil {
		t.Fatalf("Logout() with empty sessionID should return nil, got %v", err)
	}
	if repo.revokeCalled {
		t.Error("expected Revoke NOT to be called for empty session ID")
	}
}

func TestLogout_SessionNotFound_StillRevokes(t *testing.T) {
	now := time.Now()
	repo := &mockSessionRepoForLogout{session: nil}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   logoutTestUOW{sessions: repo},
	}
	ctx := context.Background()

	err := svc.Logout(ctx, LogoutCommand{SessionID: "nonexistent"})
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if !repo.revokeCalled {
		t.Error("expected Revoke to be called even when session not found")
	}
}

func TestLogout_RevokeError(t *testing.T) {
	now := time.Now()
	repo := &mockSessionRepoForLogout{
		session:   &sessiondomain.Session{ID: "sess-001", UserID: "user-001"},
		revokeErr: apperrors.NewInternal("database error"),
	}
	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UOW:   logoutTestUOW{sessions: repo},
	}
	ctx := context.Background()

	err := svc.Logout(ctx, LogoutCommand{SessionID: "sess-001"})
	if err == nil {
		t.Fatal("expected error from revoke, got nil")
	}
}

func TestLogout_ToAccountContext(t *testing.T) {
	org := &orgdomain.Organization{
		ID:          "org-001",
		Slug:        "test-org",
		DisplayName: "Test Organization",
		LogoURL:     "https://example.com/logo.png",
		FaviconURL:  "https://example.com/favicon.ico",
	}
	user := &userdomain.User{
		ID:       "user-001",
		Username: "testuser",
		IsAdmin:  true,
		Profile: userdomain.UserProfile{
			DisplayName: "Test User",
			AvatarURL:   "https://example.com/avatar.png",
		},
	}

	ctx := toAccountContext(org, user, "access-token-123")

	if ctx.Account.Owner != "test-org" {
		t.Errorf("Account.Owner = %q, want %q", ctx.Account.Owner, "test-org")
	}
	if ctx.Account.Name != "testuser" {
		t.Errorf("Account.Name = %q, want %q", ctx.Account.Name, "testuser")
	}
	if ctx.Account.DisplayName != "Test User" {
		t.Errorf("Account.DisplayName = %q, want %q", ctx.Account.DisplayName, "Test User")
	}
	if !ctx.Account.IsAdmin {
		t.Error("Account.IsAdmin = false, want true")
	}
	if ctx.Organization.ID != "org-001" {
		t.Errorf("Organization.ID = %q, want %q", ctx.Organization.ID, "org-001")
	}
	if ctx.Organization.DisplayName != "Test Organization" {
		t.Errorf("Organization.DisplayName = %q, want %q", ctx.Organization.DisplayName, "Test Organization")
	}
	if ctx.AccessToken != "access-token-123" {
		t.Errorf("AccessToken = %q, want %q", ctx.AccessToken, "access-token-123")
	}
}

func TestLogout_ToSessionIdentity(t *testing.T) {
	session := &sessiondomain.Session{
		ID:               "sess-001",
		OrganizationSlug: "my-org",
		Username:         "admin",
	}

	identity := toSessionIdentity(session)

	if identity.OrganizationSlug != "my-org" {
		t.Errorf("OrganizationSlug = %q, want %q", identity.OrganizationSlug, "my-org")
	}
	if identity.Username != "admin" {
		t.Errorf("Username = %q, want %q", identity.Username, "admin")
	}
}

func TestLogout_ToSuccessfulLoginResponse(t *testing.T) {
	account := AccountContext{
		Account: SessionAccount{
			Owner:       "test-org",
			Name:        "admin",
			DisplayName: "Admin User",
			IsAdmin:     true,
		},
		Organization: SessionOrganization{
			ID:          "org-001",
			Name:        "test-org",
			DisplayName: "Test Org",
		},
		AccessToken: "at-123",
	}

	resp := toSuccessfulLoginResponse(account)

	if resp.Status != LoginStatusSuccess {
		t.Errorf("Status = %q, want %q", resp.Status, LoginStatusSuccess)
	}
	if resp.Account == nil {
		t.Fatal("Account is nil")
	}
	if resp.Account.Owner != "test-org" {
		t.Errorf("Account.Owner = %q, want %q", resp.Account.Owner, "test-org")
	}
	if resp.Organization == nil {
		t.Fatal("Organization is nil")
	}
	if resp.Organization.ID != "org-001" {
		t.Errorf("Organization.ID = %q, want %q", resp.Organization.ID, "org-001")
	}
	if resp.AccessToken != "at-123" {
		t.Errorf("AccessToken = %q, want %q", resp.AccessToken, "at-123")
	}
}
