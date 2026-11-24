package users

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
// Fakes for service_test.go (separate from crud_test.go to avoid conflicts)
// ---------------------------------------------------------------------------

type svcFakeClock struct {
	fixedTime time.Time
}

func (c svcFakeClock) Now() time.Time { return c.fixedTime }

type svcFakeUUID struct {
	fixedID string
}

func (g svcFakeUUID) NewString() (string, error) { return g.fixedID, nil }

type svcFakeUUIDErr struct{}

func (g svcFakeUUIDErr) NewString() (string, error) {
	return "", apperrors.NewInternal("uuid generation failed")
}

// svcMockUserRepo implements userdomain.Repository with configurable function fields.
type svcMockUserRepo struct {
	getByIDFn                          func(ctx context.Context, id string) (*userdomain.User, error)
	getByIDForOrganizationFn           func(ctx context.Context, id, organizationID string) (*userdomain.User, error)
	getByUsernameFn                    func(ctx context.Context, organizationID, username string) (*userdomain.User, error)
	listFn                             func(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error)
	createFn                           func(ctx context.Context, user *userdomain.User) error
	updateFn                           func(ctx context.Context, user *userdomain.User) error
	updateForOrganizationFn            func(ctx context.Context, user *userdomain.User, organizationID string) error
	updateCollectionsForOrganizationFn func(ctx context.Context, userID, organizationID string, collections userdomain.UserCollections) error
	updateProfileFn                    func(ctx context.Context, profile *userdomain.UserProfile) error
	updateCredentialsFn                func(ctx context.Context, credentials *userdomain.UserCredentials) error
	replaceIdentitiesFn                func(ctx context.Context, userID string, identities []userdomain.UserIdentity) error
	createIdentityFn                   func(ctx context.Context, identity *userdomain.UserIdentity) error
	deleteIdentityFn                   func(ctx context.Context, userID, identityID string) error
	replaceMfaMethodsFn                func(ctx context.Context, userID string, methods []userdomain.UserMfaMethod) error
	replaceRoleBindingsFn              func(ctx context.Context, userID string, bindings []userdomain.UserRoleBinding) error
	listRoleNamesFn                    func(ctx context.Context, userID string) ([]string, error)
	replaceGroupMembershipsFn          func(ctx context.Context, userID string, memberships []userdomain.UserGroupMembership) error
	deleteFn                           func(ctx context.Context, id, organizationID string) error
	countFn                            func(ctx context.Context, organizationID string) (int64, error)
	getByProviderIdentityFn            func(ctx context.Context, providerID, subject string) (*userdomain.User, error)
}

func (r *svcMockUserRepo) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *svcMockUserRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*userdomain.User, error) {
	if r.getByIDForOrganizationFn != nil {
		return r.getByIDForOrganizationFn(ctx, id, organizationID)
	}
	return r.GetByID(ctx, id)
}

func (r *svcMockUserRepo) GetByUsername(ctx context.Context, organizationID, username string) (*userdomain.User, error) {
	if r.getByUsernameFn != nil {
		return r.getByUsernameFn(ctx, organizationID, username)
	}
	return nil, apperrors.NewNotFound("user not found")
}

// GetByLoginIdentifier 只在密码登录入口使用；用户服务用例委托用户名读路径即可。
func (r *svcMockUserRepo) GetByLoginIdentifier(ctx context.Context, organizationID, identifier string) (*userdomain.User, error) {
	return r.GetByUsername(ctx, organizationID, identifier)
}

// LockByID 是会话记账用的行级锁；本测试不经过并发会话策略，占位即可。
func (r *svcMockUserRepo) LockByID(context.Context, string) error { return nil }

func (r *svcMockUserRepo) List(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	if r.listFn != nil {
		return r.listFn(ctx, query)
	}
	return nil, 0, nil
}

func (r *svcMockUserRepo) Create(ctx context.Context, user *userdomain.User) error {
	if r.createFn != nil {
		return r.createFn(ctx, user)
	}
	return nil
}

func (r *svcMockUserRepo) Update(ctx context.Context, user *userdomain.User) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, user)
	}
	return nil
}

func (r *svcMockUserRepo) UpdateForOrganization(ctx context.Context, user *userdomain.User, organizationID string) error {
	if r.updateForOrganizationFn != nil {
		return r.updateForOrganizationFn(ctx, user, organizationID)
	}
	return r.Update(ctx, user)
}

func (r *svcMockUserRepo) UpdateProfile(ctx context.Context, profile *userdomain.UserProfile) error {
	if r.updateProfileFn != nil {
		return r.updateProfileFn(ctx, profile)
	}
	return nil
}

func (r *svcMockUserRepo) SetContactVerifiedForOrganization(context.Context, string, string, bool, bool) error {
	return nil
}

func (r *svcMockUserRepo) UpdateCollectionsForOrganization(ctx context.Context, userID, organizationID string, collections userdomain.UserCollections) error {
	if r.updateCollectionsForOrganizationFn != nil {
		return r.updateCollectionsForOrganizationFn(ctx, userID, organizationID, collections)
	}
	return nil
}

func (r *svcMockUserRepo) UpdateCredentials(ctx context.Context, credentials *userdomain.UserCredentials) error {
	if r.updateCredentialsFn != nil {
		return r.updateCredentialsFn(ctx, credentials)
	}
	return nil
}

func (r *svcMockUserRepo) UpdatePasswordCredential(ctx context.Context, credentials *userdomain.UserCredentials) error {
	return nil
}

func (r *svcMockUserRepo) ReplaceIdentities(ctx context.Context, userID string, identities []userdomain.UserIdentity) error {
	if r.replaceIdentitiesFn != nil {
		return r.replaceIdentitiesFn(ctx, userID, identities)
	}
	return nil
}

func (r *svcMockUserRepo) CreateIdentity(ctx context.Context, identity *userdomain.UserIdentity) error {
	if r.createIdentityFn != nil {
		return r.createIdentityFn(ctx, identity)
	}
	return nil
}

func (r *svcMockUserRepo) DeleteIdentity(ctx context.Context, userID, identityID string) error {
	if r.deleteIdentityFn != nil {
		return r.deleteIdentityFn(ctx, userID, identityID)
	}
	return nil
}

func (r *svcMockUserRepo) ReplaceMfaMethods(ctx context.Context, userID string, methods []userdomain.UserMfaMethod) error {
	if r.replaceMfaMethodsFn != nil {
		return r.replaceMfaMethodsFn(ctx, userID, methods)
	}
	return nil
}

func (r *svcMockUserRepo) ReplaceRoleBindings(ctx context.Context, userID string, bindings []userdomain.UserRoleBinding) error {
	if r.replaceRoleBindingsFn != nil {
		return r.replaceRoleBindingsFn(ctx, userID, bindings)
	}
	return nil
}

func (r *svcMockUserRepo) ListRoleNames(ctx context.Context, userID string) ([]string, error) {
	if r.listRoleNamesFn != nil {
		return r.listRoleNamesFn(ctx, userID)
	}
	return nil, nil
}

func (r *svcMockUserRepo) ReplaceGroupMemberships(ctx context.Context, userID string, memberships []userdomain.UserGroupMembership) error {
	if r.replaceGroupMembershipsFn != nil {
		return r.replaceGroupMembershipsFn(ctx, userID, memberships)
	}
	return nil
}

func (r *svcMockUserRepo) Delete(ctx context.Context, id, organizationID string) error {
	if r.deleteFn != nil {
		return r.deleteFn(ctx, id, organizationID)
	}
	return nil
}

func (r *svcMockUserRepo) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error) {
	return nil, nil
}

func (r *svcMockUserRepo) CreatePasswordHistory(ctx context.Context, entry *userdomain.PasswordHistoryEntry) error {
	return nil
}

func (r *svcMockUserRepo) DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error {
	return nil
}

func (r *svcMockUserRepo) Count(ctx context.Context, organizationID string) (int64, error) {
	if r.countFn != nil {
		return r.countFn(ctx, organizationID)
	}
	return 0, nil
}

func (r *svcMockUserRepo) GetByProviderIdentity(ctx context.Context, providerID, subject string) (*userdomain.User, error) {
	if r.getByProviderIdentityFn != nil {
		return r.getByProviderIdentityFn(ctx, providerID, subject)
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *svcMockUserRepo) HasDeletedWithUsername(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

// svcMockOrgRepo implements orgdomain.Repository with configurable function fields.
type svcMockOrgRepo struct {
	getByIDFn func(ctx context.Context, id string) (*orgdomain.Organization, error)
}

func (r *svcMockOrgRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *svcMockOrgRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return nil, apperrors.NewNotFound("organization not found")
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *svcMockOrgRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *svcMockOrgRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, nil
}

func (r *svcMockOrgRepo) Create(ctx context.Context, organization *orgdomain.Organization) error {
	return nil
}
func (r *svcMockOrgRepo) Update(ctx context.Context, organization *orgdomain.Organization) error {
	return nil
}
func (r *svcMockOrgRepo) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	return nil
}
func (r *svcMockOrgRepo) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	return nil
}
func (r *svcMockOrgRepo) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	return nil
}
func (r *svcMockOrgRepo) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}
func (r *svcMockOrgRepo) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// svcAnyRoleRepo / svcAnyGroupRepo satisfy the scoped lookups the replace
// use cases perform before writing role bindings / group memberships.
type svcAnyRoleRepo struct {
	roledomain.Repository
}

func (svcAnyRoleRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*roledomain.Role, error) {
	return &roledomain.Role{ID: id, OrganizationID: organizationID}, nil
}

type svcAnyGroupRepo struct {
	groupsdomain.Repository
}

func (svcAnyGroupRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*groupsdomain.Group, error) {
	return &groupsdomain.Group{ID: id, OrganizationID: organizationID}, nil
}

// svcTestUnitOfWork implements appuow.UnitOfWork
type svcTestUnitOfWork struct {
	users         *svcMockUserRepo
	organizations *svcMockOrgRepo
	roles         roledomain.Repository
	groups        groupsdomain.Repository
	records       recordsdomain.Repository
}

func (u svcTestUnitOfWork) Outbox() domainoutbox.Repository                        { return nil }
func (u svcTestUnitOfWork) Applications() applications.Repository                  { return nil }
func (u svcTestUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository { return nil }
func (u svcTestUnitOfWork) Certs() certdomain.Repository                           { return nil }
func (u svcTestUnitOfWork) Adapters() adapterdomain.Repository                     { return nil }
func (u svcTestUnitOfWork) Agents() agentdomain.Repository                         { return nil }
func (u svcTestUnitOfWork) Enforcers() enforcerdomain.Repository                   { return nil }
func (u svcTestUnitOfWork) Models() modeldomain.Repository                         { return nil }
func (u svcTestUnitOfWork) Syncers() syncerdomain.Repository                       { return nil }
func (u svcTestUnitOfWork) Webhooks() webhookdomain.Repository                     { return nil }
func (u svcTestUnitOfWork) Ldaps() ldapdomain.Repository                           { return nil }
func (u svcTestUnitOfWork) Forms() formdomain.Repository                           { return nil }
func (u svcTestUnitOfWork) Orders() orderdomain.Repository                         { return nil }
func (u svcTestUnitOfWork) Payments() paymentdomain.Repository                     { return nil }
func (u svcTestUnitOfWork) Plans() plandomain.Repository                           { return nil }

// DeleteUser 走 assertCanManagePlatformAdmin；零值 stub 返回 (nil, nil)，
// 即目标非平台管理员，放行。
func (u svcTestUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository {
	return stubPlatformAdminRepo{}
}
func (u svcTestUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u svcTestUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository { return nil }
func (u svcTestUnitOfWork) Pricings() pricingdomain.Repository              { return nil }
func (u svcTestUnitOfWork) Products() productdomain.Repository              { return nil }
func (u svcTestUnitOfWork) Subscriptions() subscriptiondomain.Repository    { return nil }
func (u svcTestUnitOfWork) Tickets() ticketdomain.Repository                { return nil }
func (u svcTestUnitOfWork) Resources() resourcedomain.Repository            { return nil }
func (u svcTestUnitOfWork) Consents() consentdomain.Repository              { return nil }
func (u svcTestUnitOfWork) Gateway() gatewaydomain.Repository               { return nil }
func (u svcTestUnitOfWork) Groups() groupsdomain.Repository                 { return u.groups }
func (u svcTestUnitOfWork) Invitations() invitationsdomain.Repository       { return nil }
func (u svcTestUnitOfWork) MfaChallenges() mfachallengedomain.Repository    { return nil }
func (u svcTestUnitOfWork) Organizations() orgdomain.Repository {
	if u.organizations == nil {
		// Use cases that only read the organization for derived metadata (e.g.
		// the MFA collections mirror) must not panic in tests that never set up
		// an organization fake. A benign built-in org keeps those paths green;
		// tests that need specific organization data still override the field.
		return &svcMockOrgRepo{
			getByIDFn: func(_ context.Context, id string) (*orgdomain.Organization, error) {
				return &orgdomain.Organization{ID: id, Slug: "built-in", DisplayName: "Built In"}, nil
			},
		}
	}
	return u.organizations
}
func (u svcTestUnitOfWork) Permissions() permissiondomain.Repository             { return nil }
func (u svcTestUnitOfWork) Providers() providerdomain.Repository                 { return nil }
func (u svcTestUnitOfWork) Roles() roledomain.Repository                         { return u.roles }
func (u svcTestUnitOfWork) Sessions() sessiondomain.Repository                   { return nil }
func (u svcTestUnitOfWork) Tokens() tokendomain.Repository                       { return nil }
func (u svcTestUnitOfWork) Users() userdomain.Repository                         { return u.users }
func (u svcTestUnitOfWork) Records() recordsdomain.Repository                    { return u.records }
func (u svcTestUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u svcTestUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u svcTestUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u svcTestUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u svcTestUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u svcTestUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u svcTestUnitOfWork) WebAuthnCredentials() webauthndomain.Repository {
	// GetUser 回填 webauthnCredentialCount；零值 waMockRepo 返回空列表。
	return &waMockRepo{}
}
func (u svcTestUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u svcTestUnitOfWork) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u svcTestUnitOfWork) Notifications() notificationsdomain.Repository                { return nil }
func (u svcTestUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository             { return nil }
func (u svcTestUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository                 { return nil }
func (u svcTestUnitOfWork) Keys() keysdomain.Repository                                  { return nil }
func (u svcTestUnitOfWork) Transactions() transactiondomain.Repository                   { return nil }
func (u svcTestUnitOfWork) WebhookEvents() webhookeventdomain.Repository                 { return nil }
func (u svcTestUnitOfWork) QRCodeSessions() qrcodesessions.Repository                    { return nil }
func (u svcTestUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository                  { return nil }

func (u svcTestUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u svcTestUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u svcTestUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u svcTestUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u svcTestUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }
func (u svcTestUnitOfWork) Rules() ruledomain.Repository                { return nil }

func (u svcTestUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u svcTestUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u svcTestUnitOfWork) IDV() idvdomain.Repository { return nil }

func (u svcTestUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u svcTestUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u svcTestUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u svcTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u svcTestUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests for Service.NewUser
// ---------------------------------------------------------------------------

func TestService_NewUser_UUIDError(t *testing.T) {
	svc := Service{
		Clock: svcFakeClock{fixedTime: time.Now()},
		UUID:  svcFakeUUIDErr{},
	}

	_, err := svc.NewUser(CreateUserCommand{
		OrganizationID: "org-1",
		Username:       "testuser",
		UserType:       "normal",
		Status:         "active",
	})
	if err == nil {
		t.Fatal("NewUser() expected error when UUID fails, got nil")
	}
}

func TestService_NewUser_AllFields(t *testing.T) {
	fixedNow := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	fixedID := "019test-user-id-0001"

	svc := Service{
		Clock: svcFakeClock{fixedTime: fixedNow},
		UUID:  svcFakeUUID{fixedID: fixedID},
	}

	user, err := svc.NewUser(CreateUserCommand{
		OrganizationID:      "org-1",
		Username:            "alice",
		ExternalID:          "ext-001",
		UserType:            "admin",
		Status:              "active",
		IsAdmin:             true,
		SignupApplicationID: "app-1",
		RegisterSource:      "web",
	})
	if err != nil {
		t.Fatalf("NewUser() unexpected error: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"ID", user.ID, fixedID},
		{"OrganizationID", user.OrganizationID, "org-1"},
		{"Username", user.Username, "alice"},
		{"ExternalID", user.ExternalID, "ext-001"},
		{"UserType", user.UserType, "admin"},
		{"Status", user.Status, "active"},
		{"SignupApplicationID", user.SignupApplicationID, "app-1"},
		{"RegisterSource", user.RegisterSource, "web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}

	if !user.IsAdmin {
		t.Error("IsAdmin = false, want true")
	}
	if !user.CreatedAt.Equal(fixedNow) {
		t.Errorf("CreatedAt = %v, want %v", user.CreatedAt, fixedNow)
	}
	if !user.UpdatedAt.Equal(fixedNow) {
		t.Errorf("UpdatedAt = %v, want %v", user.UpdatedAt, fixedNow)
	}
	if user.Revision != 1 {
		t.Errorf("Revision = %d, want 1", user.Revision)
	}
}

// ---------------------------------------------------------------------------
// Tests for Service.GetUserCount
// ---------------------------------------------------------------------------

func TestService_GetUserCount_Success(t *testing.T) {
	userRepo := &svcMockUserRepo{
		countFn: func(ctx context.Context, organizationID string) (int64, error) {
			if organizationID == "org-1" {
				return 42, nil
			}
			return 0, nil
		},
	}

	uow := svcTestUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	count, err := svc.GetUserCount(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("GetUserCount() error = %v", err)
	}
	if count != 42 {
		t.Errorf("count = %d, want 42", count)
	}
}

func TestService_GetUserCount_RepositoryError(t *testing.T) {
	userRepo := &svcMockUserRepo{
		countFn: func(ctx context.Context, organizationID string) (int64, error) {
			return 0, apperrors.NewInternal("db connection lost")
		},
	}

	uow := svcTestUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	_, err := svc.GetUserCount(context.Background(), "org-1")
	if err == nil {
		t.Fatal("GetUserCount() expected error, got nil")
	}
}

func TestService_GetUserCount_EmptyOrg(t *testing.T) {
	userRepo := &svcMockUserRepo{
		countFn: func(ctx context.Context, organizationID string) (int64, error) {
			return 0, nil
		},
	}

	uow := svcTestUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	count, err := svc.GetUserCount(context.Background(), "")
	if err != nil {
		t.Fatalf("GetUserCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// Tests for Service.ListUsers
// ---------------------------------------------------------------------------

func TestService_ListUsers_RepositoryError(t *testing.T) {
	userRepo := &svcMockUserRepo{
		listFn: func(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
			return nil, 0, apperrors.NewInternal("database error")
		},
	}

	uow := svcTestUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	_, _, err := svc.ListUsers(context.Background(), ListUsersQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: "org-1",
	})
	if err == nil {
		t.Fatal("ListUsers() expected error, got nil")
	}
}

func TestService_ListUsers_SingleUser(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	user := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-1",
		Username:       "alice",
		UserType:       "normal",
		Status:         "active",
		Profile: userdomain.UserProfile{
			UserID:      "user-1",
			DisplayName: "Alice",
			Email:       "alice@example.com",
		},
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}

	userRepo := &svcMockUserRepo{
		listFn: func(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
			return []*userdomain.User{user}, 1, nil
		},
	}

	uow := svcTestUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	items, total, err := svc.ListUsers(context.Background(), ListUsersQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: "org-1",
	})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].ID != "user-1" {
		t.Errorf("items[0].ID = %q, want %q", items[0].ID, "user-1")
	}
	if items[0].Username != "alice" {
		t.Errorf("items[0].Username = %q, want %q", items[0].Username, "alice")
	}
}

// ---------------------------------------------------------------------------
// Tests for Service.GetUser
// ---------------------------------------------------------------------------

func TestService_GetUser_OrgNotFound(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	testUser := &userdomain.User{
		ID:             "user-1",
		OrganizationID: "org-nonexistent",
		Username:       "alice",
		UserType:       "normal",
		Status:         "active",
		Profile:        userdomain.UserProfile{UserID: "user-1", DisplayName: "Alice"},
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	userRepo := &svcMockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return testUser, nil
		},
	}

	orgRepo := &svcMockOrgRepo{} // returns NotFound by default

	uow := svcTestUnitOfWork{users: userRepo, organizations: orgRepo}
	svc := Service{UOW: uow}

	_, err := svc.GetUser(context.Background(), "user-1", "org-1")
	if err == nil {
		t.Fatal("GetUser() expected error when org not found, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests for buildEnforceRvals equivalent (toListItem / toDetail)
// ---------------------------------------------------------------------------

func TestService_NewUser_EmptyFields(t *testing.T) {
	fixedNow := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)

	svc := Service{
		Clock: svcFakeClock{fixedTime: fixedNow},
		UUID:  svcFakeUUID{fixedID: "test-id"},
	}

	user, err := svc.NewUser(CreateUserCommand{
		OrganizationID: "org-1",
		Username:       "",
		UserType:       "",
		Status:         "",
	})
	if err != nil {
		t.Fatalf("NewUser() unexpected error: %v", err)
	}
	if user.Username != "" {
		t.Errorf("Username = %q, want empty", user.Username)
	}
	if user.UserType != "" {
		t.Errorf("UserType = %q, want empty", user.UserType)
	}
	if user.Status != "" {
		t.Errorf("Status = %q, want empty", user.Status)
	}
	if user.IsAdmin {
		t.Error("IsAdmin should default to false")
	}
}
