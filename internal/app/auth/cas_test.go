package auth

import (
	"context"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	"strings"
	"testing"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	adapterdomain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	agentdomain "github.com/guliangiam/guliangiam/internal/domain/agents"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	captchaproviderdomain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	castickets "github.com/guliangiam/guliangiam/internal/domain/castickets"
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
	kerberostickets "github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
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
// CAS test helpers
// ---------------------------------------------------------------------------

type casTestUnitOfWork struct {
	orgRepo        *loginTestOrgRepository
	userRepo       *loginTestUserRepository
	appRepo        *loginTestAppRepository
	casTicketRepo  *casTestTicketRepository
	sessRepo       *loginTestSessionRepository
	lockoutAttempt *loginTestLockoutAttemptRepository
}

func (u casTestUnitOfWork) Outbox() domainoutbox.Repository                        { return nil }
func (u casTestUnitOfWork) Applications() applications.Repository                  { return u.appRepo }
func (u casTestUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository { return nil }
func (u casTestUnitOfWork) Certs() certdomain.Repository                           { return nil }
func (u casTestUnitOfWork) Adapters() adapterdomain.Repository                     { return nil }
func (u casTestUnitOfWork) Agents() agentdomain.Repository                         { return nil }
func (u casTestUnitOfWork) Enforcers() enforcerdomain.Repository                   { return nil }
func (u casTestUnitOfWork) Models() modeldomain.Repository                         { return nil }
func (u casTestUnitOfWork) Syncers() syncerdomain.Repository                       { return nil }
func (u casTestUnitOfWork) Webhooks() webhookdomain.Repository                     { return nil }
func (u casTestUnitOfWork) Ldaps() ldapdomain.Repository                           { return nil }
func (u casTestUnitOfWork) Forms() formdomain.Repository                           { return nil }
func (u casTestUnitOfWork) Orders() orderdomain.Repository                         { return nil }
func (u casTestUnitOfWork) Payments() paymentdomain.Repository                     { return nil }
func (u casTestUnitOfWork) Plans() plandomain.Repository                           { return nil }

func (u casTestUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository { return nil }
func (u casTestUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u casTestUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository      { return nil }
func (u casTestUnitOfWork) Pricings() pricingdomain.Repository                   { return nil }
func (u casTestUnitOfWork) Products() productdomain.Repository                   { return nil }
func (u casTestUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u casTestUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u casTestUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u casTestUnitOfWork) Consents() consentdomain.Repository                   { return nil }
func (u casTestUnitOfWork) Gateway() gatewaydomain.Repository                    { return nil }
func (u casTestUnitOfWork) Groups() groupsdomain.Repository                      { return nil }
func (u casTestUnitOfWork) Invitations() invitationsdomain.Repository            { return nil }
func (u casTestUnitOfWork) MfaChallenges() mfachallengedomain.Repository         { return nil }
func (u casTestUnitOfWork) Organizations() orgdomain.Repository                  { return u.orgRepo }
func (u casTestUnitOfWork) Permissions() permissiondomain.Repository             { return nil }
func (u casTestUnitOfWork) Providers() providerdomain.Repository                 { return nil }
func (u casTestUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u casTestUnitOfWork) Sessions() sessiondomain.Repository                   { return u.sessRepo }
func (u casTestUnitOfWork) Tokens() tokendomain.Repository                       { return nil }
func (u casTestUnitOfWork) Users() userdomain.Repository                         { return u.userRepo }
func (u casTestUnitOfWork) Records() recordsdomain.Repository                    { return nil }
func (u casTestUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u casTestUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u casTestUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u casTestUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u casTestUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u casTestUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u casTestUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u casTestUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository   { return nil }
func (u casTestUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	return nil
}
func (u casTestUnitOfWork) Notifications() notificationsdomain.Repository    { return nil }
func (u casTestUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository { return nil }
func (u casTestUnitOfWork) Rules() ruledomain.Repository                     { return nil }
func (u casTestUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository     { return nil }
func (u casTestUnitOfWork) Keys() keysdomain.Repository                      { return nil }
func (u casTestUnitOfWork) Transactions() transactiondomain.Repository       { return nil }
func (u casTestUnitOfWork) WebhookEvents() webhookeventdomain.Repository     { return nil }
func (u casTestUnitOfWork) QRCodeSessions() qrcodesessions.Repository        { return nil }
func (u casTestUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository      { return nil }

func (u casTestUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u casTestUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u casTestUnitOfWork) CASTickets() castickets.Repository           { return u.casTicketRepo }
func (u casTestUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u casTestUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return u.lockoutAttempt }

func (u casTestUnitOfWork) IDV() idvdomain.Repository { return nil }

func (u casTestUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u casTestUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u casTestUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u casTestUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u casTestUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u casTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u casTestUnitOfWork) IGA() igadomain.Repository { return nil }

type casTestTicketRepository struct {
	tickets map[string]*castickets.Ticket // keyed by ticketID
}

func (r *casTestTicketRepository) GetByTicketID(ctx context.Context, ticketID string) (*castickets.Ticket, error) {
	ticket, ok := r.tickets[ticketID]
	if !ok {
		return nil, apperrors.NewNotFound("ticket not found")
	}
	return ticket, nil
}

func (r *casTestTicketRepository) Create(ctx context.Context, ticket *castickets.Ticket) error {
	if r.tickets == nil {
		r.tickets = make(map[string]*castickets.Ticket)
	}
	r.tickets[ticket.TicketID] = ticket
	return nil
}

func (r *casTestTicketRepository) DeleteByTicketID(ctx context.Context, ticketID string) (*castickets.Ticket, error) {
	ticket, ok := r.tickets[ticketID]
	if !ok {
		return nil, apperrors.NewNotFound("ticket not found")
	}
	delete(r.tickets, ticketID)
	return ticket, nil
}

func (r *casTestTicketRepository) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

func (r *casTestTicketRepository) GetByPGTIOU(ctx context.Context, pgtIOU string) (*castickets.Ticket, error) {
	for _, t := range r.tickets {
		if t.PGTIOU == pgtIOU {
			return t, nil
		}
	}
	return nil, apperrors.NewNotFound("pgtIou not found")
}

func (r *casTestTicketRepository) MarkPGTIOUUsed(ctx context.Context, pgtIOU string) (string, error) {
	for _, t := range r.tickets {
		if t.PGTIOU == pgtIOU && !t.PGTIOUUsed {
			t.PGTIOUUsed = true
			return t.TicketID, nil
		}
	}
	return "", apperrors.NewNotFound("pgtIou invalid or already used")
}

// ---------------------------------------------------------------------------
// CAS tests
// ---------------------------------------------------------------------------

func TestCASLogin_MissingUsername(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.CASLogin(context.Background(), CASLoginCommand{
		OrganizationSlug: "test-org",
		Username:         "",
		Password:         "password",
		Service:          "https://app.example.com",
	})
	if err == nil {
		t.Fatal("expected error for missing username")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestCASLogin_MissingPassword(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.CASLogin(context.Background(), CASLoginCommand{
		OrganizationSlug: "test-org",
		Username:         "testuser",
		Password:         "",
		Service:          "https://app.example.com",
	})
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestCASLogin_MissingService(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.CASLogin(context.Background(), CASLoginCommand{
		OrganizationSlug: "test-org",
		Username:         "testuser",
		Password:         "password",
		Service:          "",
	})
	if err == nil {
		t.Fatal("expected error for missing service")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("expected invalid_argument, got %s", appErr.Code)
	}
}

func TestCASLogin_InvalidCredentials(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := casTestUnitOfWork{
		orgRepo: &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{org.Slug: org}},
		userRepo: &loginTestUserRepository{
			users:      map[string]*userdomain.User{user.ID: user},
			usersByOrg: map[string]map[string]*userdomain.User{org.ID: {user.Username: user}},
		},
		appRepo:        &loginTestAppRepository{apps: map[string]*applications.Application{app.ID: app}},
		casTicketRepo:  &casTestTicketRepository{},
		sessRepo:       &loginTestSessionRepository{},
		lockoutAttempt: &loginTestLockoutAttemptRepository{},
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
		UOW:           uow,
	}

	result, err := svc.CASLogin(context.Background(), CASLoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "WrongPassword",
		Service:          "https://app.example.com",
	})
	if err == nil {
		t.Fatal("expected error for invalid credentials")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnauthorized {
		t.Errorf("expected unauthorized, got %s", appErr.Code)
	}
}

func TestCASLogin_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	casTicketRepo := &casTestTicketRepository{}
	uow := casTestUnitOfWork{
		orgRepo: &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{org.Slug: org}},
		userRepo: &loginTestUserRepository{
			users:      map[string]*userdomain.User{user.ID: user},
			usersByOrg: map[string]map[string]*userdomain.User{org.ID: {user.Username: user}},
		},
		appRepo:        &loginTestAppRepository{apps: map[string]*applications.Application{app.ID: app}},
		casTicketRepo:  casTicketRepo,
		sessRepo:       &loginTestSessionRepository{},
		lockoutAttempt: &loginTestLockoutAttemptRepository{},
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
		UOW:           uow,
	}

	result, err := svc.CASLogin(context.Background(), CASLoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		Service:          "https://app.example.com",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Ticket == "" {
		t.Error("expected non-empty ticket")
	}
	if !strings.HasPrefix(result.Ticket, "ST-") {
		t.Errorf("expected ticket to start with ST-, got %s", result.Ticket)
	}
	if result.Message != "success" {
		t.Errorf("expected success message, got %s", result.Message)
	}
}

func TestCASServiceValidate_MissingTicket(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.CASServiceValidate(context.Background(), CASServiceValidateCommand{
		OrganizationSlug: "test-org",
		ApplicationSlug:  "test-app",
		Ticket:           "",
		Service:          "https://app.example.com",
	})
	if err == nil {
		t.Fatal("expected error for missing ticket")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestCASServiceValidate_MissingService(t *testing.T) {
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
	}

	result, err := svc.CASServiceValidate(context.Background(), CASServiceValidateCommand{
		OrganizationSlug: "test-org",
		ApplicationSlug:  "test-app",
		Ticket:           "ST-123",
		Service:          "",
	})
	if err == nil {
		t.Fatal("expected error for missing service")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestCASServiceValidate_InvalidTicket(t *testing.T) {
	casTicketRepo := &casTestTicketRepository{tickets: map[string]*castickets.Ticket{}}
	uow := casTestUnitOfWork{
		orgRepo:        &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{}},
		userRepo:       &loginTestUserRepository{users: map[string]*userdomain.User{}, usersByOrg: map[string]map[string]*userdomain.User{}},
		appRepo:        &loginTestAppRepository{apps: map[string]*applications.Application{}},
		casTicketRepo:  casTicketRepo,
		sessRepo:       &loginTestSessionRepository{},
		lockoutAttempt: &loginTestLockoutAttemptRepository{},
	}
	svc := Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
		UOW:           uow,
	}

	result, err := svc.CASServiceValidate(context.Background(), CASServiceValidateCommand{
		OrganizationSlug: "test-org",
		ApplicationSlug:  "test-app",
		Ticket:           "ST-nonexistent",
		Service:          "https://app.example.com",
	})
	if err == nil {
		t.Fatal("expected error for non-existent ticket")
	}
	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestCAS_GenerateTicketID(t *testing.T) {
	ticketID := casGenerateTicketID("ST")
	if !strings.HasPrefix(ticketID, "ST-") {
		t.Errorf("expected ST- prefix, got %s", ticketID)
	}
}

func TestCAS_BuildXMLSuccessResponse(t *testing.T) {
	data := BuildCASXMLSuccessResponse("testuser", "user-1", map[string]string{"email": "test@example.com"})
	if len(data) == 0 {
		t.Fatal("expected non-empty XML response")
	}
	dataStr := string(data)
	if !strings.Contains(dataStr, "testuser") {
		t.Errorf("expected testuser in response, got %s", dataStr)
	}
	if !strings.Contains(dataStr, "authenticationSuccess") {
		t.Error("expected authenticationSuccess in response")
	}
}

func TestCAS_BuildXMLFailureResponse(t *testing.T) {
	data := BuildCASXMLFailureResponse("INVALID_TICKET", "ticket not recognized")
	if len(data) == 0 {
		t.Fatal("expected non-empty XML response")
	}
	dataStr := string(data)
	if !strings.Contains(dataStr, "INVALID_TICKET") {
		t.Errorf("expected INVALID_TICKET in response, got %s", dataStr)
	}
	if !strings.Contains(dataStr, "authenticationFailure") {
		t.Error("expected authenticationFailure in response")
	}
}
