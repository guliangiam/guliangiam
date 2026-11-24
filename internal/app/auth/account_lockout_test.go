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
// Lockout test UoW
// ---------------------------------------------------------------------------

type lockoutTestUnitOfWork struct {
	orgRepo        *loginTestOrgRepository
	lockoutAttempt *loginTestLockoutAttemptRepository
}

func (u lockoutTestUnitOfWork) Applications() applications.Repository                  { return nil }
func (u lockoutTestUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository { return nil }
func (u lockoutTestUnitOfWork) Certs() certdomain.Repository                           { return nil }
func (u lockoutTestUnitOfWork) Adapters() adapterdomain.Repository                     { return nil }
func (u lockoutTestUnitOfWork) Agents() agentdomain.Repository                         { return nil }
func (u lockoutTestUnitOfWork) Enforcers() enforcerdomain.Repository                   { return nil }
func (u lockoutTestUnitOfWork) Models() modeldomain.Repository                         { return nil }
func (u lockoutTestUnitOfWork) Syncers() syncerdomain.Repository                       { return nil }
func (u lockoutTestUnitOfWork) Webhooks() webhookdomain.Repository                     { return nil }
func (u lockoutTestUnitOfWork) Ldaps() ldapdomain.Repository                           { return nil }
func (u lockoutTestUnitOfWork) Forms() formdomain.Repository                           { return nil }
func (u lockoutTestUnitOfWork) Orders() orderdomain.Repository                         { return nil }
func (u lockoutTestUnitOfWork) Payments() paymentdomain.Repository                     { return nil }
func (u lockoutTestUnitOfWork) Plans() plandomain.Repository                           { return nil }

func (u lockoutTestUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository { return nil }
func (u lockoutTestUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u lockoutTestUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository      { return nil }
func (u lockoutTestUnitOfWork) Pricings() pricingdomain.Repository                   { return nil }
func (u lockoutTestUnitOfWork) Products() productdomain.Repository                   { return nil }
func (u lockoutTestUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u lockoutTestUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u lockoutTestUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u lockoutTestUnitOfWork) Consents() consentdomain.Repository                   { return nil }
func (u lockoutTestUnitOfWork) Gateway() gatewaydomain.Repository                    { return nil }
func (u lockoutTestUnitOfWork) Groups() groupsdomain.Repository                      { return nil }
func (u lockoutTestUnitOfWork) Invitations() invitationsdomain.Repository            { return nil }
func (u lockoutTestUnitOfWork) MfaChallenges() mfachallengedomain.Repository         { return nil }
func (u lockoutTestUnitOfWork) Organizations() orgdomain.Repository                  { return u.orgRepo }
func (u lockoutTestUnitOfWork) Permissions() permissiondomain.Repository             { return nil }
func (u lockoutTestUnitOfWork) Providers() providerdomain.Repository                 { return nil }
func (u lockoutTestUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u lockoutTestUnitOfWork) Sessions() sessiondomain.Repository                   { return nil }
func (u lockoutTestUnitOfWork) Tokens() tokendomain.Repository                       { return nil }
func (u lockoutTestUnitOfWork) Users() userdomain.Repository                         { return nil }
func (u lockoutTestUnitOfWork) Records() recordsdomain.Repository                    { return nil }
func (u lockoutTestUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u lockoutTestUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u lockoutTestUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u lockoutTestUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u lockoutTestUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u lockoutTestUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u lockoutTestUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u lockoutTestUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository   { return nil }
func (u lockoutTestUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	return nil
}
func (u lockoutTestUnitOfWork) Notifications() notificationsdomain.Repository    { return nil }
func (u lockoutTestUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository { return nil }
func (u lockoutTestUnitOfWork) Rules() ruledomain.Repository                     { return nil }
func (u lockoutTestUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository     { return nil }
func (u lockoutTestUnitOfWork) Keys() keysdomain.Repository                      { return nil }
func (u lockoutTestUnitOfWork) Transactions() transactiondomain.Repository       { return nil }
func (u lockoutTestUnitOfWork) WebhookEvents() webhookeventdomain.Repository     { return nil }
func (u lockoutTestUnitOfWork) QRCodeSessions() qrcodesessions.Repository        { return nil }
func (u lockoutTestUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository      { return nil }

func (u lockoutTestUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u lockoutTestUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u lockoutTestUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u lockoutTestUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u lockoutTestUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return u.lockoutAttempt }
func (u lockoutTestUnitOfWork) IDV() idvdomain.Repository                   { return nil }

func (u lockoutTestUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u lockoutTestUnitOfWork) Rebac() rebac.Repository { return nil }

func (u lockoutTestUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u lockoutTestUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u lockoutTestUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u lockoutTestUnitOfWork) Outbox() domainoutbox.Repository { return nil }

func (u lockoutTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u lockoutTestUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Lockout policy tests (existing)
// ---------------------------------------------------------------------------

func TestDefaultLockoutPolicy(t *testing.T) {
	policy := defaultLockoutPolicy()
	if policy.MaxAttempts != 5 {
		t.Errorf("expected MaxAttempts=5, got %d", policy.MaxAttempts)
	}
	if policy.LockoutDurationMinutes != 30 {
		t.Errorf("expected LockoutDurationMinutes=30, got %d", policy.LockoutDurationMinutes)
	}
}

func TestResolveLockoutPolicy_NilOrg(t *testing.T) {
	policy := resolveLockoutPolicy(nil)
	if policy.MaxAttempts != 5 {
		t.Errorf("expected default MaxAttempts=5, got %d", policy.MaxAttempts)
	}
}

func TestResolveLockoutPolicy_EmptyLockoutPolicy(t *testing.T) {
	org := &orgdomain.Organization{
		Security: orgdomain.OrganizationSecurityPolicy{},
	}
	policy := resolveLockoutPolicy(org)
	if policy.MaxAttempts != 5 {
		t.Errorf("expected default MaxAttempts=5, got %d", policy.MaxAttempts)
	}
}

func TestResolveLockoutPolicy_CustomValues(t *testing.T) {
	tests := []struct {
		name                    string
		lockoutPolicy           map[string]interface{}
		wantMaxAttempts         int
		wantLockoutDurationMins int
	}{
		{
			"custom int values",
			map[string]interface{}{"maxAttempts": 3, "lockoutDurationMinutes": 15},
			3, 15,
		},
		{
			"custom float values",
			map[string]interface{}{"maxAttempts": float64(10), "lockoutDurationMinutes": float64(60)},
			10, 60,
		},
		{
			"only maxAttempts",
			map[string]interface{}{"maxAttempts": 10},
			10, 30,
		},
		{
			"zero maxAttempts ignored",
			map[string]interface{}{"maxAttempts": 0},
			5, 30,
		},
		{
			"negative lockout ignored",
			map[string]interface{}{"lockoutDurationMinutes": -5},
			5, 30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org := &orgdomain.Organization{
				Security: orgdomain.OrganizationSecurityPolicy{
					LockoutPolicy: tt.lockoutPolicy,
				},
			}
			policy := resolveLockoutPolicy(org)
			if policy.MaxAttempts != tt.wantMaxAttempts {
				t.Errorf("MaxAttempts = %d, want %d", policy.MaxAttempts, tt.wantMaxAttempts)
			}
			if policy.LockoutDurationMinutes != tt.wantLockoutDurationMins {
				t.Errorf("LockoutDurationMinutes = %d, want %d", policy.LockoutDurationMinutes, tt.wantLockoutDurationMins)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Additional lockout test scenarios
// ---------------------------------------------------------------------------

func TestLockout_ConsecutiveFailuresReachThreshold(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	org := &orgdomain.Organization{
		ID:   "org-1",
		Slug: "test-org",
		Security: orgdomain.OrganizationSecurityPolicy{
			LockoutPolicy:  map[string]interface{}{"maxAttempts": 3, "lockoutDurationMinutes": 15},
			OrganizationID: "org-1",
		},
	}
	lockoutRepo := &loginTestLockoutAttemptRepository{}
	orgRepo := &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{org.Slug: org}}
	uow := lockoutTestUnitOfWork{orgRepo: orgRepo, lockoutAttempt: lockoutRepo}

	userID := "user-lockout-1"
	policy := resolveLockoutPolicy(org)

	// Simulate 3 consecutive failed login attempts.
	for i := 0; i < 3; i++ {
		_, err := recordFailureAtTimeWithUOW(context.Background(), uow, userID, policy, now)
		if err != nil {
			t.Fatalf("recordFailure attempt %d: error = %v", i+1, err)
		}
	}

	// After 3 failures, the account should be locked.
	attempt, _ := lockoutRepo.GetByUserID(context.Background(), userID)
	if attempt == nil {
		t.Fatal("expected lockout attempt record to exist")
	}
	if attempt.Attempts < 3 {
		t.Errorf("expected at least 3 attempts, got %d", attempt.Attempts)
	}
	if attempt.LockedUntil == nil {
		t.Fatal("expected account to be locked (LockedUntil should be set)")
	}
}

func TestLockout_LockedAccountRejectsLogin(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(30 * time.Minute)
	userID := "user-locked-1"

	lockoutRepo := &loginTestLockoutAttemptRepository{
		attempts: map[string]*lockoutattempts.Attempt{
			userID: {
				ID:          "lockout-locked",
				UserID:      userID,
				Attempts:    5,
				LockedUntil: &lockedUntil,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	uow := lockoutTestUnitOfWork{lockoutAttempt: lockoutRepo}

	err := checkAccountLockoutAtTimeWithUOW(context.Background(), uow, userID, now)
	if err == nil {
		t.Fatal("expected error for locked account")
	}
	appErr := apperrors.FromError(err)
	if appErr.Code != apperrors.CodeUnauthorized {
		t.Errorf("expected the generic credential error, got %s", appErr.Code)
	}
	if appErr.Message != "invalid username or password" {
		t.Errorf("lockout message = %q, want the generic credential message", appErr.Message)
	}
}

func TestLockout_ExpiredLockoutAutoUnlocks(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 30, 0, 0, time.UTC)
	// Lockout expired 5 minutes ago.
	lockedUntil := now.Add(-5 * time.Minute)
	userID := "user-expired-lock-1"

	lockoutRepo := &loginTestLockoutAttemptRepository{
		attempts: map[string]*lockoutattempts.Attempt{
			userID: {
				ID:          "lockout-expired",
				UserID:      userID,
				Attempts:    5,
				LockedUntil: &lockedUntil,
				CreatedAt:   now.Add(-35 * time.Minute),
				UpdatedAt:   now.Add(-35 * time.Minute),
			},
		},
	}
	uow := lockoutTestUnitOfWork{lockoutAttempt: lockoutRepo}

	err := checkAccountLockoutAtTimeWithUOW(context.Background(), uow, userID, now)
	if err != nil {
		t.Fatalf("expected no error for expired lockout, got %v", err)
	}
}

func TestLockout_SuccessfulLoginResetsFailures(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	userID := "user-reset-1"

	lockoutRepo := &loginTestLockoutAttemptRepository{
		attempts: map[string]*lockoutattempts.Attempt{
			userID: {
				ID:          "lockout-reset",
				UserID:      userID,
				Attempts:    3,
				LockedUntil: nil,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	uow := lockoutTestUnitOfWork{lockoutAttempt: lockoutRepo}

	// Reset login failures (simulating successful login).
	err := resetLoginFailuresWithTx(context.Background(), uow, userID)
	if err != nil {
		t.Fatalf("expected no error on reset, got %v", err)
	}

	// Verify the lockout entry was deleted.
	attempt, _ := lockoutRepo.GetByUserID(context.Background(), userID)
	if attempt != nil {
		t.Fatal("expected lockout entry to be deleted after reset")
	}
}

func TestLockout_NoLockoutEntryMeansNotLocked(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	userID := "user-no-lockout"

	lockoutRepo := &loginTestLockoutAttemptRepository{}
	uow := lockoutTestUnitOfWork{lockoutAttempt: lockoutRepo}

	err := checkAccountLockoutAtTimeWithUOW(context.Background(), uow, userID, now)
	if err != nil {
		t.Fatalf("expected no error for user without lockout entry, got %v", err)
	}
}

func TestLockout_RecordFailureCreatesNewEntry(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	org := &orgdomain.Organization{
		ID:   "org-1",
		Slug: "test-org",
		Security: orgdomain.OrganizationSecurityPolicy{
			LockoutPolicy:  map[string]interface{}{"maxAttempts": 5, "lockoutDurationMinutes": 30},
			OrganizationID: "org-1",
		},
	}
	lockoutRepo := &loginTestLockoutAttemptRepository{}
	orgRepo := &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{org.Slug: org}}
	uow := lockoutTestUnitOfWork{orgRepo: orgRepo, lockoutAttempt: lockoutRepo}

	userID := "user-new-failure"
	policy := resolveLockoutPolicy(org)

	_, err := recordFailureAtTimeWithUOW(context.Background(), uow, userID, policy, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	attempt, _ := lockoutRepo.GetByUserID(context.Background(), userID)
	if attempt == nil {
		t.Fatal("expected lockout attempt record to be created")
	}
	if attempt.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempt.Attempts)
	}
}

func TestLockout_RecordFailureDoesNotIncrementWhenLocked(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(30 * time.Minute)
	userID := "user-already-locked"

	org := &orgdomain.Organization{
		ID:   "org-1",
		Slug: "test-org",
		Security: orgdomain.OrganizationSecurityPolicy{
			LockoutPolicy:  map[string]interface{}{"maxAttempts": 5, "lockoutDurationMinutes": 30},
			OrganizationID: "org-1",
		},
	}
	lockoutRepo := &loginTestLockoutAttemptRepository{
		attempts: map[string]*lockoutattempts.Attempt{
			userID: {
				ID:          "lockout-existing",
				UserID:      userID,
				Attempts:    5,
				LockedUntil: &lockedUntil,
				CreatedAt:   now.Add(-30 * time.Minute),
				UpdatedAt:   now.Add(-30 * time.Minute),
			},
		},
	}
	orgRepo := &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{org.Slug: org}}
	uow := lockoutTestUnitOfWork{orgRepo: orgRepo, lockoutAttempt: lockoutRepo}

	policy := resolveLockoutPolicy(org)
	_, err := recordFailureAtTimeWithUOW(context.Background(), uow, userID, policy, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Attempts should remain at 5 (not incremented while locked).
	attempt, _ := lockoutRepo.GetByUserID(context.Background(), userID)
	if attempt.Attempts != 5 {
		t.Errorf("expected 5 attempts (not incremented while locked), got %d", attempt.Attempts)
	}
}

func TestLockout_CheckAccountLockout_ServiceMethod(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	userID := "user-svc-lockout"

	lockoutRepo := &loginTestLockoutAttemptRepository{}
	uow := lockoutTestUnitOfWork{lockoutAttempt: lockoutRepo}
	svc := Service{
		Clock:         fakeClock{fixedTime: now},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
		UOW:           uow,
	}

	// No lockout entry — should not be locked.
	err := svc.CheckAccountLockout(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error for user without lockout, got %v", err)
	}
}

func TestLockout_ResetLoginFailures_ServiceMethod(t *testing.T) {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	userID := "user-svc-reset"

	lockoutRepo := &loginTestLockoutAttemptRepository{
		attempts: map[string]*lockoutattempts.Attempt{
			userID: {
				ID:        "lockout-svc-reset",
				UserID:    userID,
				Attempts:  2,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	uow := lockoutTestUnitOfWork{lockoutAttempt: lockoutRepo}
	svc := Service{
		Clock:         fakeClock{fixedTime: now},
		UUIDGenerator: fakeUUIDGenerator{fixedID: "test-id"},
		UOW:           uow,
	}

	err := svc.ResetLoginFailures(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	attempt, _ := lockoutRepo.GetByUserID(context.Background(), userID)
	if attempt != nil {
		t.Fatal("expected lockout entry to be deleted after reset")
	}
}
