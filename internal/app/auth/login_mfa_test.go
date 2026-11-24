package auth

import (
	"context"
	"strings"
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

type loginMfaTestUnitOfWork struct {
	challengeRepo *loginMfaTestChallengeRepository
}

func (u loginMfaTestUnitOfWork) Outbox() domainoutbox.Repository       { return nil }
func (u loginMfaTestUnitOfWork) Applications() applications.Repository { return nil }
func (u loginMfaTestUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository {
	return nil
}
func (u loginMfaTestUnitOfWork) Certs() certdomain.Repository         { return nil }
func (u loginMfaTestUnitOfWork) Adapters() adapterdomain.Repository   { return nil }
func (u loginMfaTestUnitOfWork) Agents() agentdomain.Repository       { return nil }
func (u loginMfaTestUnitOfWork) Enforcers() enforcerdomain.Repository { return nil }
func (u loginMfaTestUnitOfWork) Models() modeldomain.Repository       { return nil }
func (u loginMfaTestUnitOfWork) Syncers() syncerdomain.Repository     { return nil }
func (u loginMfaTestUnitOfWork) Webhooks() webhookdomain.Repository   { return nil }
func (u loginMfaTestUnitOfWork) Ldaps() ldapdomain.Repository         { return nil }
func (u loginMfaTestUnitOfWork) Forms() formdomain.Repository         { return nil }
func (u loginMfaTestUnitOfWork) Orders() orderdomain.Repository       { return nil }
func (u loginMfaTestUnitOfWork) Payments() paymentdomain.Repository   { return nil }
func (u loginMfaTestUnitOfWork) Plans() plandomain.Repository         { return nil }

func (u loginMfaTestUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository { return nil }
func (u loginMfaTestUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u loginMfaTestUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository      { return nil }
func (u loginMfaTestUnitOfWork) Pricings() pricingdomain.Repository                   { return nil }
func (u loginMfaTestUnitOfWork) Products() productdomain.Repository                   { return nil }
func (u loginMfaTestUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u loginMfaTestUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u loginMfaTestUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u loginMfaTestUnitOfWork) Consents() consentdomain.Repository                   { return nil }
func (u loginMfaTestUnitOfWork) Gateway() gatewaydomain.Repository                    { return nil }
func (u loginMfaTestUnitOfWork) Groups() groupsdomain.Repository                      { return nil }
func (u loginMfaTestUnitOfWork) Invitations() invitationsdomain.Repository            { return nil }
func (u loginMfaTestUnitOfWork) MfaChallenges() mfachallengedomain.Repository         { return u.challengeRepo }
func (u loginMfaTestUnitOfWork) Organizations() orgdomain.Repository                  { return nil }
func (u loginMfaTestUnitOfWork) Permissions() permissiondomain.Repository             { return nil }
func (u loginMfaTestUnitOfWork) Providers() providerdomain.Repository                 { return nil }
func (u loginMfaTestUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u loginMfaTestUnitOfWork) Sessions() sessiondomain.Repository                   { return nil }
func (u loginMfaTestUnitOfWork) Tokens() tokendomain.Repository                       { return nil }
func (u loginMfaTestUnitOfWork) Users() userdomain.Repository                         { return nil }
func (u loginMfaTestUnitOfWork) Records() recordsdomain.Repository                    { return nil }
func (u loginMfaTestUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u loginMfaTestUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u loginMfaTestUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u loginMfaTestUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u loginMfaTestUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u loginMfaTestUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u loginMfaTestUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u loginMfaTestUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository   { return nil }
func (u loginMfaTestUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	return nil
}
func (u loginMfaTestUnitOfWork) Notifications() notificationsdomain.Repository    { return nil }
func (u loginMfaTestUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository { return nil }
func (u loginMfaTestUnitOfWork) Rules() ruledomain.Repository                     { return nil }
func (u loginMfaTestUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository     { return nil }
func (u loginMfaTestUnitOfWork) Keys() keysdomain.Repository                      { return nil }
func (u loginMfaTestUnitOfWork) Transactions() transactiondomain.Repository       { return nil }
func (u loginMfaTestUnitOfWork) WebhookEvents() webhookeventdomain.Repository     { return nil }
func (u loginMfaTestUnitOfWork) QRCodeSessions() qrcodesessions.Repository        { return nil }
func (u loginMfaTestUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository      { return nil }

func (u loginMfaTestUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u loginMfaTestUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u loginMfaTestUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u loginMfaTestUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u loginMfaTestUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }

func (u loginMfaTestUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u loginMfaTestUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u loginMfaTestUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u loginMfaTestUnitOfWork) IDV() idvdomain.Repository { return nil }

func (u loginMfaTestUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u loginMfaTestUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u loginMfaTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u loginMfaTestUnitOfWork) IGA() igadomain.Repository { return nil }

type loginMfaTestChallengeRepository struct {
	items map[string]*mfachallengedomain.Challenge
}

func (r *loginMfaTestChallengeRepository) GetByID(ctx context.Context, id string) (*mfachallengedomain.Challenge, error) {
	challenge, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("login challenge not found")
	}
	return challenge, nil
}

func (r *loginMfaTestChallengeRepository) Create(ctx context.Context, challenge *mfachallengedomain.Challenge) error {
	r.items[challenge.ID] = challenge
	return nil
}

func (r *loginMfaTestChallengeRepository) Update(ctx context.Context, challenge *mfachallengedomain.Challenge) error {
	if _, ok := r.items[challenge.ID]; !ok {
		return apperrors.NewNotFound("login challenge not found")
	}
	r.items[challenge.ID] = challenge
	return nil
}

func (r *loginMfaTestChallengeRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return apperrors.NewNotFound("login challenge not found")
	}
	delete(r.items, id)
	return nil
}

func TestVerifyLoginMfaWithEmailPasscode(t *testing.T) {
	originalVerifyEmailCode := verifyEmailCode
	defer func() {
		verifyEmailCode = originalVerifyEmailCode
	}()

	called := false
	verifyEmailCode = func(ctx context.Context, dest string, passcode string) error {
		called = true
		if dest != "jane@example.com" {
			t.Fatalf("verifyEmailCode dest = %q, want %q", dest, "jane@example.com")
		}
		if passcode != "123456" {
			t.Fatalf("verifyEmailCode passcode = %q, want %q", passcode, "123456")
		}
		return nil
	}

	user := &userdomain.User{
		Profile: userdomain.UserProfile{Email: "jane@example.com"},
		MfaMethods: []userdomain.UserMfaMethod{{
			ID:         "mfa-email-1",
			MethodType: "email",
			Enabled:    true,
			Verified:   true,
		}},
	}

	command := LoginCommand{MfaType: "email", Passcode: "123456"}
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	result, err := verifyLoginMfa(context.Background(), nil, command, nil, user, nil, enabledVerifiedMfaMethods(user), now, nil)
	if err != nil {
		t.Fatalf("verifyLoginMfa() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("expected email verification to complete login, got %+v", result)
	}
	if !called {
		t.Fatalf("expected verifyEmailCode to be called")
	}
	if user.MfaMethods[0].LastUsedAt == nil || !user.MfaMethods[0].LastUsedAt.Equal(now) {
		t.Fatalf("expected email MFA method last used timestamp to be updated, got %+v", user.MfaMethods[0].LastUsedAt)
	}
}

func TestVerifyLoginMfaWithSmsPasscode(t *testing.T) {
	originalVerifySmsCode := verifySmsCode
	defer func() {
		verifySmsCode = originalVerifySmsCode
	}()

	called := false
	verifySmsCode = func(ctx context.Context, phone string, countryCode string, passcode string) error {
		called = true
		if phone != "13800138000" {
			t.Fatalf("verifySmsCode phone = %q, want %q", phone, "13800138000")
		}
		if countryCode != "86" {
			t.Fatalf("verifySmsCode countryCode = %q, want %q", countryCode, "86")
		}
		if passcode != "654321" {
			t.Fatalf("verifySmsCode passcode = %q, want %q", passcode, "654321")
		}
		return nil
	}

	user := &userdomain.User{
		Profile: userdomain.UserProfile{Phone: "13800138000", CountryCode: "86"},
		MfaMethods: []userdomain.UserMfaMethod{{
			ID:         "mfa-sms-1",
			MethodType: "sms",
			Enabled:    true,
			Verified:   true,
		}},
	}

	command := LoginCommand{MfaType: "sms", Passcode: "654321"}
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	result, err := verifyLoginMfa(context.Background(), nil, command, nil, user, nil, enabledVerifiedMfaMethods(user), now, nil)
	if err != nil {
		t.Fatalf("verifyLoginMfa() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("expected sms verification to complete login, got %+v", result)
	}
	if !called {
		t.Fatalf("expected verifySmsCode to be called")
	}
	if user.MfaMethods[0].LastUsedAt == nil || !user.MfaMethods[0].LastUsedAt.Equal(now) {
		t.Fatalf("expected sms MFA method last used timestamp to be updated, got %+v", user.MfaMethods[0].LastUsedAt)
	}
}

func TestVerifyLoginMfaWithRadiusPasscode(t *testing.T) {
	originalResolveRadiusProvider := resolveRadiusProvider
	originalVerifyRadiusCredentials := verifyRadiusCredentials
	defer func() {
		resolveRadiusProvider = originalResolveRadiusProvider
		verifyRadiusCredentials = originalVerifyRadiusCredentials
	}()

	calledResolve := false
	resolveRadiusProvider = func(ctx context.Context, tx appuow.UnitOfWork, providerID string) (radiusProviderConfig, error) {
		calledResolve = true
		if providerID != "provider-radius-1" {
			t.Fatalf("resolveRadiusProvider providerID = %q, want %q", providerID, "provider-radius-1")
		}
		return radiusProviderConfig{host: "radius.internal", port: 1812, sharedSecret: "secret-1"}, nil
	}

	calledVerify := false
	verifyRadiusCredentials = func(ctx context.Context, config radiusProviderConfig, username string, password string) error {
		calledVerify = true
		if config.host != "radius.internal" || config.port != 1812 || config.sharedSecret != "secret-1" {
			t.Fatalf("unexpected radius config: %+v", config)
		}
		if username != "jane-radius" {
			t.Fatalf("verifyRadiusCredentials username = %q, want %q", username, "jane-radius")
		}
		if password != "radius-pass" {
			t.Fatalf("verifyRadiusCredentials password = %q, want %q", password, "radius-pass")
		}
		return nil
	}

	user := &userdomain.User{
		MfaRadiusUsername: "jane-radius",
		MfaRadiusProvider: "provider-radius-1",
		MfaMethods: []userdomain.UserMfaMethod{{
			ID:         "mfa-radius-1",
			MethodType: "radius",
			Enabled:    true,
			Verified:   true,
		}},
	}

	command := LoginCommand{MfaType: "radius", Passcode: "radius-pass"}
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	result, err := verifyLoginMfa(context.Background(), nil, command, nil, user, nil, enabledVerifiedMfaMethods(user), now, nil)
	if err != nil {
		t.Fatalf("verifyLoginMfa() error = %v", err)
	}
	if !result.Completed {
		t.Fatalf("expected radius verification to complete login, got %+v", result)
	}
	if !calledResolve {
		t.Fatalf("expected resolveRadiusProvider to be called")
	}
	if !calledVerify {
		t.Fatalf("expected verifyRadiusCredentials to be called")
	}
	if user.MfaMethods[0].LastUsedAt == nil || !user.MfaMethods[0].LastUsedAt.Equal(now) {
		t.Fatalf("expected radius MFA method last used timestamp to be updated, got %+v", user.MfaMethods[0].LastUsedAt)
	}
}

func TestVerifyLoginMfaWithPush(t *testing.T) {
	originalResolvePushProvider := resolvePushProvider
	originalSendPushAuthentication := sendPushAuthentication
	defer func() {
		resolvePushProvider = originalResolvePushProvider
		sendPushAuthentication = originalSendPushAuthentication
	}()

	calledResolve := false
	resolvePushProvider = func(ctx context.Context, tx appuow.UnitOfWork, providerID string) (pushProviderConfig, error) {
		calledResolve = true
		if providerID != "provider-push-1" {
			t.Fatalf("resolvePushProvider providerID = %q, want %q", providerID, "provider-push-1")
		}
		return pushProviderConfig{providerType: "Pushbullet", clientSecret: "push-secret"}, nil
	}

	calledSend := false
	sendPushAuthentication = func(ctx context.Context, config pushProviderConfig, receiver string, title string, message string) error {
		calledSend = true
		if config.providerType != "Pushbullet" || config.clientSecret != "push-secret" {
			t.Fatalf("unexpected push config: %+v", config)
		}
		if receiver != "device-token-1" {
			t.Fatalf("sendPushAuthentication receiver = %q, want %q", receiver, "device-token-1")
		}
		if title != "MFA Verification Request" {
			t.Fatalf("sendPushAuthentication title = %q, want %q", title, "MFA Verification Request")
		}
		if message == "" {
			t.Fatalf("expected push message to be populated")
		}
		if !strings.Contains(message, "sign-in") {
			t.Fatalf("expected push message to contain action type %q, got %q", "sign-in", message)
		}
		return nil
	}

	user := &userdomain.User{
		MfaPushReceiver: "device-token-1",
		MfaPushProvider: "provider-push-1",
		MfaMethods: []userdomain.UserMfaMethod{{
			ID:         "mfa-push-1",
			MethodType: "push",
			Enabled:    true,
			Verified:   true,
		}},
	}
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	challenge := &mfachallengedomain.Challenge{
		ID:        "challenge-push-1",
		State:     mfachallengedomain.StatePending,
		ExpiresAt: now.Add(10 * time.Minute),
	}
	challengeRepo := &loginMfaTestChallengeRepository{items: map[string]*mfachallengedomain.Challenge{
		challenge.ID: challenge,
	}}
	tx := loginMfaTestUnitOfWork{challengeRepo: challengeRepo}

	command := LoginCommand{MfaType: "push"}
	result, err := verifyLoginMfa(context.Background(), tx, command, nil, user, challenge, enabledVerifiedMfaMethods(user), now, nil)
	if err != nil {
		t.Fatalf("verifyLoginMfa() error = %v", err)
	}
	if result.Completed {
		t.Fatalf("expected initial push verification to remain pending, got %+v", result)
	}
	if result.Response.Status != LoginStatusMfaRequired || result.Response.ChallengeState != mfachallengedomain.StatePushPending {
		t.Fatalf("unexpected push verification response: %+v", result.Response)
	}
	if !calledResolve {
		t.Fatalf("expected resolvePushProvider to be called")
	}
	if !calledSend {
		t.Fatalf("expected sendPushAuthentication to be called")
	}
	if challenge.State != mfachallengedomain.StatePushPending {
		t.Fatalf("expected challenge state to become push_pending, got %q", challenge.State)
	}
	if user.MfaMethods[0].LastUsedAt != nil {
		t.Fatalf("expected push MFA method last used timestamp to remain unset before approval, got %+v", user.MfaMethods[0].LastUsedAt)
	}
}
