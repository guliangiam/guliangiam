package users

import (
	"context"
	"errors"
	"testing"
	"time"

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
	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
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
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
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
	trusteddevicesdomain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
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
}

func (g fakeUUIDGenerator) NewString() (string, error) { return g.fixedID, nil }

// ---------------------------------------------------------------------------
// Mock repositories
// ---------------------------------------------------------------------------

// mockUserRepo implements userdomain.Repository with configurable function fields.
type mockUserRepo struct {
	getByIDFn                     func(ctx context.Context, id string) (*userdomain.User, error)
	getByIDForOrganizationFn      func(ctx context.Context, id, organizationID string) (*userdomain.User, error)
	getByUsernameFn               func(ctx context.Context, organizationID, username string) (*userdomain.User, error)
	listFn                        func(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error)
	createFn                      func(ctx context.Context, user *userdomain.User) error
	updateFn                      func(ctx context.Context, user *userdomain.User) error
	updateForOrganizationFn       func(ctx context.Context, user *userdomain.User, organizationID string) error
	updateProfileFn               func(ctx context.Context, profile *userdomain.UserProfile) error
	updateCredentialsFn           func(ctx context.Context, credentials *userdomain.UserCredentials) error
	updatePasswordCredentialFn    func(ctx context.Context, credentials *userdomain.UserCredentials) error
	replaceIdentitiesFn           func(ctx context.Context, userID string, identities []userdomain.UserIdentity) error
	createIdentityFn              func(ctx context.Context, identity *userdomain.UserIdentity) error
	deleteIdentityFn              func(ctx context.Context, userID, identityID string) error
	replaceMfaMethodsFn           func(ctx context.Context, userID string, methods []userdomain.UserMfaMethod) error
	replaceRoleBindingsFn         func(ctx context.Context, userID string, bindings []userdomain.UserRoleBinding) error
	replaceGroupMembershipsFn     func(ctx context.Context, userID string, memberships []userdomain.UserGroupMembership) error
	deleteFn                      func(ctx context.Context, id, organizationID string) error
	getPasswordHistoryFn          func(ctx context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error)
	createPasswordHistoryFn       func(ctx context.Context, entry *userdomain.PasswordHistoryEntry) error
	deletePasswordHistoryBeyondFn func(ctx context.Context, userID string, keepCount int) error
	getByProviderIdentityFn       func(ctx context.Context, providerID, subject string) (*userdomain.User, error)
	updateCollectionsFn           func(ctx context.Context, userID, organizationID string, collections userdomain.UserCollections) error
}

func (r *mockUserRepo) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFound("user not found")
}

// GetByIDForOrganization falls back to getByIDFn for legacy fixtures that do
// not model tenant ownership; tests that pin scoping set the dedicated field.
func (r *mockUserRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*userdomain.User, error) {
	if r.getByIDForOrganizationFn != nil {
		return r.getByIDForOrganizationFn(ctx, id, organizationID)
	}
	return r.GetByID(ctx, id)
}

// LockByID 是会话记账用的行级锁；本测试不经过并发会话策略，占位即可。
func (r *mockUserRepo) LockByID(context.Context, string) error { return nil }

func (r *mockUserRepo) GetByUsername(ctx context.Context, organizationID, username string) (*userdomain.User, error) {
	if r.getByUsernameFn != nil {
		return r.getByUsernameFn(ctx, organizationID, username)
	}
	return nil, apperrors.NewNotFound("user not found")
}

// GetByLoginIdentifier 只在密码登录入口使用；用户管理用例委托用户名读路径即可。
func (r *mockUserRepo) GetByLoginIdentifier(ctx context.Context, organizationID, identifier string) (*userdomain.User, error) {
	return r.GetByUsername(ctx, organizationID, identifier)
}

func (r *mockUserRepo) List(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	if r.listFn != nil {
		return r.listFn(ctx, query)
	}
	return nil, 0, nil
}

func (r *mockUserRepo) Create(ctx context.Context, user *userdomain.User) error {
	if r.createFn != nil {
		return r.createFn(ctx, user)
	}
	return nil
}

func (r *mockUserRepo) Update(ctx context.Context, user *userdomain.User) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, user)
	}
	return nil
}

func (r *mockUserRepo) UpdateForOrganization(ctx context.Context, user *userdomain.User, organizationID string) error {
	if r.updateForOrganizationFn != nil {
		return r.updateForOrganizationFn(ctx, user, organizationID)
	}
	return r.Update(ctx, user)
}

func (r *mockUserRepo) UpdateProfile(ctx context.Context, profile *userdomain.UserProfile) error {
	if r.updateProfileFn != nil {
		return r.updateProfileFn(ctx, profile)
	}
	return nil
}

func (r *mockUserRepo) SetContactVerifiedForOrganization(context.Context, string, string, bool, bool) error {
	return nil
}

func (r *mockUserRepo) UpdateCollectionsForOrganization(ctx context.Context, userID, organizationID string, collections userdomain.UserCollections) error {
	if r.updateCollectionsFn != nil {
		return r.updateCollectionsFn(ctx, userID, organizationID, collections)
	}
	return nil
}

func (r *mockUserRepo) UpdateCredentials(ctx context.Context, credentials *userdomain.UserCredentials) error {
	if r.updateCredentialsFn != nil {
		return r.updateCredentialsFn(ctx, credentials)
	}
	return nil
}

func (r *mockUserRepo) UpdatePasswordCredential(ctx context.Context, credentials *userdomain.UserCredentials) error {
	if r.updatePasswordCredentialFn != nil {
		return r.updatePasswordCredentialFn(ctx, credentials)
	}
	return nil
}

func (r *mockUserRepo) ReplaceIdentities(ctx context.Context, userID string, identities []userdomain.UserIdentity) error {
	if r.replaceIdentitiesFn != nil {
		return r.replaceIdentitiesFn(ctx, userID, identities)
	}
	return nil
}

func (r *mockUserRepo) CreateIdentity(ctx context.Context, identity *userdomain.UserIdentity) error {
	if r.createIdentityFn != nil {
		return r.createIdentityFn(ctx, identity)
	}
	return nil
}

func (r *mockUserRepo) DeleteIdentity(ctx context.Context, userID, identityID string) error {
	if r.deleteIdentityFn != nil {
		return r.deleteIdentityFn(ctx, userID, identityID)
	}
	return nil
}

func (r *mockUserRepo) ReplaceMfaMethods(ctx context.Context, userID string, methods []userdomain.UserMfaMethod) error {
	if r.replaceMfaMethodsFn != nil {
		return r.replaceMfaMethodsFn(ctx, userID, methods)
	}
	return nil
}

func (r *mockUserRepo) ReplaceRoleBindings(ctx context.Context, userID string, bindings []userdomain.UserRoleBinding) error {
	if r.replaceRoleBindingsFn != nil {
		return r.replaceRoleBindingsFn(ctx, userID, bindings)
	}
	return nil
}

func (r *mockUserRepo) ListRoleNames(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}

func (r *mockUserRepo) ReplaceGroupMemberships(ctx context.Context, userID string, memberships []userdomain.UserGroupMembership) error {
	if r.replaceGroupMembershipsFn != nil {
		return r.replaceGroupMembershipsFn(ctx, userID, memberships)
	}
	return nil
}

func (r *mockUserRepo) Delete(ctx context.Context, id, organizationID string) error {
	if r.deleteFn != nil {
		return r.deleteFn(ctx, id, organizationID)
	}
	return nil
}

func (r *mockUserRepo) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error) {
	if r.getPasswordHistoryFn != nil {
		return r.getPasswordHistoryFn(ctx, userID, limit)
	}
	return nil, nil
}

func (r *mockUserRepo) CreatePasswordHistory(ctx context.Context, entry *userdomain.PasswordHistoryEntry) error {
	if r.createPasswordHistoryFn != nil {
		return r.createPasswordHistoryFn(ctx, entry)
	}
	return nil
}

func (r *mockUserRepo) DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error {
	if r.deletePasswordHistoryBeyondFn != nil {
		return r.deletePasswordHistoryBeyondFn(ctx, userID, keepCount)
	}
	return nil
}

func (r *mockUserRepo) Count(ctx context.Context, organizationID string) (int64, error) {
	return 0, nil
}

func (r *mockUserRepo) GetByProviderIdentity(ctx context.Context, providerID, subject string) (*userdomain.User, error) {
	if r.getByProviderIdentityFn != nil {
		return r.getByProviderIdentityFn(ctx, providerID, subject)
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *mockUserRepo) HasDeletedWithUsername(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}
func (r *mockUserRepo) IGA() igadomain.Repository { return nil }

// mockOrgRepo implements orgdomain.Repository with configurable function fields.
type mockOrgRepo struct {
	getByIDFn              func(ctx context.Context, id string) (*orgdomain.Organization, error)
	getBySlugFn            func(ctx context.Context, slug string) (*orgdomain.Organization, error)
	listFn                 func(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error)
	createFn               func(ctx context.Context, organization *orgdomain.Organization) error
	updateFn               func(ctx context.Context, organization *orgdomain.Organization) error
	updateSettingsFn       func(ctx context.Context, settings *orgdomain.OrganizationSettings) error
	updateThemeFn          func(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error
	updateSecurityPolicyFn func(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error
}

func (r *mockOrgRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	if r.getBySlugFn != nil {
		return r.getBySlugFn(ctx, slug)
	}
	return nil, apperrors.NewNotFound("organization not found")
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *mockOrgRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *mockOrgRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	if r.listFn != nil {
		return r.listFn(ctx, query)
	}
	return nil, 0, nil
}

func (r *mockOrgRepo) Create(ctx context.Context, organization *orgdomain.Organization) error {
	if r.createFn != nil {
		return r.createFn(ctx, organization)
	}
	return nil
}

func (r *mockOrgRepo) Update(ctx context.Context, organization *orgdomain.Organization) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, organization)
	}
	return nil
}

func (r *mockOrgRepo) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	if r.updateSettingsFn != nil {
		return r.updateSettingsFn(ctx, settings)
	}
	return nil
}

func (r *mockOrgRepo) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	if r.updateThemeFn != nil {
		return r.updateThemeFn(ctx, theme)
	}
	return nil
}

func (r *mockOrgRepo) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	if r.updateSecurityPolicyFn != nil {
		return r.updateSecurityPolicyFn(ctx, policy)
	}
	return nil
}

func (r *mockOrgRepo) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}

func (r *mockOrgRepo) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// mockSessionRepo implements sessiondomain.Repository with configurable function fields.
type mockSessionRepo struct {
	getByIDFn            func(ctx context.Context, id string) (*sessiondomain.Session, error)
	listFn               func(ctx context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error)
	createFn             func(ctx context.Context, session *sessiondomain.Session) error
	updateFn             func(ctx context.Context, session *sessiondomain.Session) error
	touchFn              func(ctx context.Context, id string, touchedAt time.Time) error
	revokeFn             func(ctx context.Context, id string, revokedAt time.Time) error
	revokeByUserExceptFn func(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error)
	revokeByUserFn       func(ctx context.Context, userID string, revokedAt time.Time) (int64, error)
	revokeByUserAndAppFn func(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error)
	deleteFn             func(ctx context.Context, id string) error
}

func (r *mockSessionRepo) GetByID(ctx context.Context, id string) (*sessiondomain.Session, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFound("session not found")
}

func (r *mockSessionRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*sessiondomain.Session, error) {
	return r.GetByID(ctx, id)
}

func (r *mockSessionRepo) List(ctx context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	if r.listFn != nil {
		return r.listFn(ctx, query)
	}
	return nil, 0, nil
}

func (r *mockSessionRepo) Create(ctx context.Context, session *sessiondomain.Session) error {
	if r.createFn != nil {
		return r.createFn(ctx, session)
	}
	return nil
}

func (r *mockSessionRepo) Update(ctx context.Context, session *sessiondomain.Session) error {
	if r.updateFn != nil {
		return r.updateFn(ctx, session)
	}
	return nil
}

func (r *mockSessionRepo) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	if r.touchFn != nil {
		return r.touchFn(ctx, id, touchedAt)
	}
	return nil
}

func (r *mockSessionRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	if r.revokeFn != nil {
		return r.revokeFn(ctx, id, revokedAt)
	}
	return nil
}

func (r *mockSessionRepo) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	return r.Revoke(ctx, id, revokedAt)
}

func (r *mockSessionRepo) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	if r.revokeByUserExceptFn != nil {
		return r.revokeByUserExceptFn(ctx, userID, excludeSessionID, revokedAt)
	}
	return 0, nil
}

func (r *mockSessionRepo) RevokeByUserAndApp(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	if r.revokeByUserAndAppFn != nil {
		return r.revokeByUserAndAppFn(ctx, userID, applicationID, excludeSessionID, revokedAt)
	}
	return 0, nil
}

func (r *mockSessionRepo) Delete(ctx context.Context, id string) error {
	if r.deleteFn != nil {
		return r.deleteFn(ctx, id)
	}
	return nil
}

func (r *mockSessionRepo) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	return r.Delete(ctx, id)
}

func (r *mockSessionRepo) CleanupExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *mockSessionRepo) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (r *mockSessionRepo) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	if r.revokeByUserFn != nil {
		return r.revokeByUserFn(ctx, userID, revokedAt)
	}
	return 0, nil
}

// mockTokenRepo implements tokendomain.Repository with configurable function fields.
type mockTokenRepo struct {
	getByIDFn               func(ctx context.Context, id string) (*tokendomain.Token, error)
	getByTokenHashFn        func(ctx context.Context, tokenHash string) (*tokendomain.Token, error)
	getByDeviceSecretHashFn func(ctx context.Context, deviceSecretHash string) (*tokendomain.Token, error)
	getRefreshByTokenHashFn func(ctx context.Context, tokenHash string) (*tokendomain.RefreshToken, error)
	listFn                  func(ctx context.Context, query tokendomain.ListQuery) ([]*tokendomain.Token, int64, error)
	createFn                func(ctx context.Context, token *tokendomain.Token) error
	createRefreshFn         func(ctx context.Context, token *tokendomain.RefreshToken) error
	revokeFn                func(ctx context.Context, id string, revokedAt time.Time) error
	revokeRefreshFn         func(ctx context.Context, id string, revokedAt time.Time) error
	revokeByQueryFn         func(ctx context.Context, query tokendomain.RevokeQuery, revokedAt time.Time) (int64, error)
	revokeByFamilyIDFn      func(ctx context.Context, familyID string, revokedAt time.Time) (int64, error)
	revokeRefreshByUserFn   func(ctx context.Context, userID string, revokedAt time.Time) (int64, error)
	deleteFn                func(ctx context.Context, id string) error
}

func (r *mockTokenRepo) GetByID(ctx context.Context, id string) (*tokendomain.Token, error) {
	if r.getByIDFn != nil {
		return r.getByIDFn(ctx, id)
	}
	return nil, apperrors.NewNotFound("token not found")
}

func (r *mockTokenRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*tokendomain.Token, error) {
	token, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if token.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("token not found")
	}
	return token, nil
}

func (r *mockTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*tokendomain.Token, error) {
	if r.getByTokenHashFn != nil {
		return r.getByTokenHashFn(ctx, tokenHash)
	}
	return nil, apperrors.NewNotFound("token not found")
}

func (r *mockTokenRepo) GetByDeviceSecretHash(ctx context.Context, deviceSecretHash string) (*tokendomain.Token, error) {
	if r.getByDeviceSecretHashFn != nil {
		return r.getByDeviceSecretHashFn(ctx, deviceSecretHash)
	}
	return nil, apperrors.NewNotFound("token not found")
}

func (r *mockTokenRepo) GetRefreshByTokenHash(ctx context.Context, tokenHash string) (*tokendomain.RefreshToken, error) {
	if r.getRefreshByTokenHashFn != nil {
		return r.getRefreshByTokenHashFn(ctx, tokenHash)
	}
	return nil, apperrors.NewNotFound("refresh token not found")
}

func (r *mockTokenRepo) List(ctx context.Context, query tokendomain.ListQuery) ([]*tokendomain.Token, int64, error) {
	if r.listFn != nil {
		return r.listFn(ctx, query)
	}
	return nil, 0, nil
}

func (r *mockTokenRepo) Create(ctx context.Context, token *tokendomain.Token) error {
	if r.createFn != nil {
		return r.createFn(ctx, token)
	}
	return nil
}

func (r *mockTokenRepo) UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error {
	return nil
}
func (r *mockTokenRepo) CreateRefresh(ctx context.Context, token *tokendomain.RefreshToken) error {
	if r.createRefreshFn != nil {
		return r.createRefreshFn(ctx, token)
	}
	return nil
}

func (r *mockTokenRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	if r.revokeFn != nil {
		return r.revokeFn(ctx, id, revokedAt)
	}
	return nil
}

func (r *mockTokenRepo) RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error {
	if r.revokeRefreshFn != nil {
		return r.revokeRefreshFn(ctx, id, revokedAt)
	}
	return nil
}

func (r *mockTokenRepo) RevokeByQuery(ctx context.Context, query tokendomain.RevokeQuery, revokedAt time.Time) (int64, error) {
	if r.revokeByQueryFn != nil {
		return r.revokeByQueryFn(ctx, query, revokedAt)
	}
	return 0, nil
}

func (r *mockTokenRepo) RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	if r.revokeByFamilyIDFn != nil {
		return r.revokeByFamilyIDFn(ctx, familyID, revokedAt)
	}
	return 0, nil
}

func (r *mockTokenRepo) RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	if r.revokeRefreshByUserFn != nil {
		return r.revokeRefreshByUserFn(ctx, userID, revokedAt)
	}
	return 0, nil
}

func (r *mockTokenRepo) RevokeByAuthorizationCodeID(ctx context.Context, authorizationCodeID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *mockTokenRepo) Delete(ctx context.Context, id string) error {
	if r.deleteFn != nil {
		return r.deleteFn(ctx, id)
	}
	return nil
}
func (r *mockTokenRepo) IGA() igadomain.Repository { return nil }

// fakeRecordsRepo records audit rows written inside the transaction. The
// embedded interface keeps the unimplemented audit-chain methods out of scope:
// only Create is exercised by the delete-user path.
type fakeRecordsRepo struct {
	recordsdomain.Repository

	created []*recordsdomain.Record
}

func (r *fakeRecordsRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.created = append(r.created, record)
	return nil
}

// ---------------------------------------------------------------------------
// Mock UnitOfWork
// ---------------------------------------------------------------------------

type testUnitOfWork struct {
	users              *mockUserRepo
	organizations      *mockOrgRepo
	sessions           *mockSessionRepo
	tokens             *mockTokenRepo
	apps               applications.Repository
	records            recordsdomain.Repository
	roles              roledomain.Repository
	groups             groupsdomain.Repository
	platformAdmins     platformadmins.PlatformAdminRepository
	platformAdminScope platformadmins.PlatformAdminScopeRepository
	trustedDevices     trusteddevicesdomain.Repository
	faceTemplates      userdomain.FaceTemplateRepository
	outbox             domainoutbox.Repository
	webauthn           webauthndomain.Repository
}

func (u testUnitOfWork) Applications() applications.Repository                  { return u.apps }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository { return nil }
func (u testUnitOfWork) Certs() certdomain.Repository                           { return nil }
func (u testUnitOfWork) Adapters() adapterdomain.Repository                     { return nil }
func (u testUnitOfWork) Agents() agentdomain.Repository                         { return nil }
func (u testUnitOfWork) Enforcers() enforcerdomain.Repository                   { return nil }
func (u testUnitOfWork) Models() modeldomain.Repository                         { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                       { return nil }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository                     { return nil }
func (u testUnitOfWork) Ldaps() ldapdomain.Repository                           { return nil }
func (u testUnitOfWork) Forms() formdomain.Repository                           { return nil }
func (u testUnitOfWork) Orders() orderdomain.Repository                         { return nil }
func (u testUnitOfWork) Payments() paymentdomain.Repository                     { return nil }
func (u testUnitOfWork) Plans() plandomain.Repository                           { return nil }

func (u testUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository {
	if u.platformAdmins != nil {
		return u.platformAdmins
	}
	// DeleteUser 走 assertCanManagePlatformAdmin；零值 stub 返回 (nil, nil)，
	// 即目标非平台管理员，放行。
	return stubPlatformAdminRepo{}
}
func (u testUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return u.platformAdminScope
}
func (u testUnitOfWork) TrustedDevices() trusteddevicesdomain.Repository      { return u.trustedDevices }
func (u testUnitOfWork) FaceTemplates() userdomain.FaceTemplateRepository     { return u.faceTemplates }
func (u testUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository      { return nil }
func (u testUnitOfWork) Pricings() pricingdomain.Repository                   { return nil }
func (u testUnitOfWork) Products() productdomain.Repository                   { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u testUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u testUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                   { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                    { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                      { return u.groups }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository            { return nil }
func (u testUnitOfWork) MfaChallenges() mfachallengedomain.Repository         { return nil }
func (u testUnitOfWork) Organizations() orgdomain.Repository                  { return u.organizations }
func (u testUnitOfWork) Permissions() permissiondomain.Repository             { return nil }
func (u testUnitOfWork) Providers() providerdomain.Repository                 { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                         { return u.roles }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                   { return u.sessions }
func (u testUnitOfWork) Tokens() tokendomain.Repository                       { return u.tokens }
func (u testUnitOfWork) Users() userdomain.Repository                         { return u.users }
func (u testUnitOfWork) Records() recordsdomain.Repository                    { return u.records }
func (u testUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u testUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u testUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository {
	// GetUser 回填 webauthnCredentialCount；零值 waMockRepo 返回空列表。
	if u.webauthn != nil {
		return u.webauthn
	}
	return &waMockRepo{}
}
func (u testUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository           { return nil }
func (u testUnitOfWork) NotificationProviders() notificationproviderdomain.Repository { return nil }
func (u testUnitOfWork) Notifications() notificationsdomain.Repository                { return nil }
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository             { return nil }
func (u testUnitOfWork) Rules() ruledomain.Repository                                 { return nil }
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository                 { return nil }
func (u testUnitOfWork) Keys() keysdomain.Repository                                  { return nil }
func (u testUnitOfWork) Transactions() transactiondomain.Repository                   { return nil }
func (u testUnitOfWork) WebhookEvents() webhookeventdomain.Repository                 { return nil }
func (u testUnitOfWork) QRCodeSessions() qrcodesessions.Repository                    { return nil }
func (u testUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository                  { return nil }

func (u testUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u testUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u testUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u testUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u testUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }
func (u testUnitOfWork) IDV() idvdomain.Repository   { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) Outbox() domainoutbox.Repository { return u.outbox }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

const (
	testUserID         = "019abcde-f000-7000-8000-000000000001"
	testOrganizationID = "019abcde-f000-7000-8000-000000000010"
	testUsername       = "testuser"
	testDisplayName    = "Test User"
	testEmail          = "testuser@example.com"
)

func newTestUser() *userdomain.User {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	return &userdomain.User{
		ID:             testUserID,
		OrganizationID: testOrganizationID,
		Username:       testUsername,
		UserType:       "normal",
		Status:         "active",
		IsAdmin:        false,
		IsForbidden:    false,
		IsVerified:     true,
		Profile: userdomain.UserProfile{
			UserID:      testUserID,
			DisplayName: testDisplayName,
			Email:       testEmail,
		},
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}
}

func newTestOrganization() *orgdomain.Organization {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	return &orgdomain.Organization{
		ID:          testOrganizationID,
		Slug:        "test-org",
		DisplayName: "Test Organization",
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
		Revision:    1,
	}
}

// ---------------------------------------------------------------------------
// Test cases
// ---------------------------------------------------------------------------

func TestCreateUser_Success(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	fixedUUID := "019newid-f000-7000-8000-000000000099"

	recordsRepo := &fakeRecordsRepo{}
	var createdUser *userdomain.User
	var updatedProfile *userdomain.UserProfile
	_ = updatedProfile // mockUserRepo 定义里仍引用，保留但不再断言

	userRepo := &mockUserRepo{
		createFn: func(ctx context.Context, user *userdomain.User) error {
			createdUser = user
			return nil
		},
		updateProfileFn: func(ctx context.Context, profile *userdomain.UserProfile) error {
			updatedProfile = profile
			return nil
		},
	}

	uow := testUnitOfWork{users: userRepo, records: recordsRepo}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: fixedUUID},
		UOW:   uow,
	}

	const actorID = "019operator-0000-7000-8000-000000000001"
	cmd := CreateUserWithProfileCommand{
		CreateUserCommand: CreateUserCommand{
			OrganizationID: testOrganizationID,
			ActorID:        actorID,
			Username:       "newuser",
			UserType:       "normal",
			Status:         "active",
		},
		Profile: CreateUserProfileCommand{
			DisplayName: "New User",
			Email:       "newuser@example.com",
			Homepage:    "https://example.com",
			Education:   "University",
			Language:    "en",
			Affiliation: "Example Org",
			Title:       "Engineer",
		},
		Karma:   42,
		Ranking: 7,
	}

	user, err := svc.CreateUser(context.Background(), cmd)
	if err != nil {
		t.Fatalf("CreateUser() returned unexpected error: %v", err)
	}

	// Verify returned user fields
	if user.ID != fixedUUID {
		t.Errorf("user.ID = %q, want %q", user.ID, fixedUUID)
	}
	if user.OrganizationID != testOrganizationID {
		t.Errorf("user.OrganizationID = %q, want %q", user.OrganizationID, testOrganizationID)
	}
	if user.Username != "newuser" {
		t.Errorf("user.Username = %q, want %q", user.Username, "newuser")
	}
	if user.UserType != "normal" {
		t.Errorf("user.UserType = %q, want %q", user.UserType, "normal")
	}
	if user.Status != "active" {
		t.Errorf("user.Status = %q, want %q", user.Status, "active")
	}
	if !user.CreatedAt.Equal(fixedNow) {
		t.Errorf("user.CreatedAt = %v, want %v", user.CreatedAt, fixedNow)
	}
	if !user.UpdatedAt.Equal(fixedNow) {
		t.Errorf("user.UpdatedAt = %v, want %v", user.UpdatedAt, fixedNow)
	}
	if user.Revision != 1 {
		t.Errorf("user.Revision = %d, want %d", user.Revision, 1)
	}

	// Verify repository was called with the user
	if createdUser == nil {
		t.Fatal("expected Create to be called on the repository, but createdUser is nil")
	}
	if createdUser.ID != fixedUUID {
		t.Errorf("createdUser.ID = %q, want %q", createdUser.ID, fixedUUID)
	}

	// Profile 内嵌在 user 里，由 repository.Create 内部处理
	if createdUser.Profile.DisplayName != "New User" {
		t.Errorf("createdUser.Profile.DisplayName = %q, want %q", createdUser.Profile.DisplayName, "New User")
	}
	if createdUser.Profile.Email != "newuser@example.com" {
		t.Errorf("createdUser.Profile.Email = %q, want %q", createdUser.Profile.Email, "newuser@example.com")
	}
	if createdUser.Profile.Properties["homepage"] != "https://example.com" ||
		createdUser.Profile.Properties["education"] != "University" ||
		createdUser.Profile.Properties["language"] != "en" ||
		createdUser.Profile.Properties["affiliation"] != "Example Org" ||
		createdUser.Profile.Properties["title"] != "Engineer" ||
		createdUser.Profile.Properties["karma"] != 42 ||
		createdUser.Profile.Properties["ranking"] != 7 {
		t.Errorf("createdUser.Profile.Properties = %#v, want all extended profile values", createdUser.Profile.Properties)
	}

	// Verify the audit row carries the operator as actor.
	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	if record := recordsRepo.created[0]; record.Action != "user.created" {
		t.Errorf("audit action = %q, want %q", record.Action, "user.created")
	} else if record.ActorID != actorID {
		t.Errorf("audit actorID = %q, want %q", record.ActorID, actorID)
	}
}

// TestCreateUser_SystemAction_ActorIDEmpty pins the control case: a system
// path with no actor must leave Record.ActorID empty (the records repository
// maps "" to NULL), never a placeholder UUID.
func TestCreateUser_SystemAction_ActorIDEmpty(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	fixedUUID := "019sysid-f000-7000-8000-000000000098"

	userRepo := &mockUserRepo{
		createFn: func(ctx context.Context, user *userdomain.User) error { return nil },
	}
	recordsRepo := &fakeRecordsRepo{}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: fixedUUID},
		UOW:   testUnitOfWork{users: userRepo, records: recordsRepo},
	}

	_, err := svc.CreateUser(context.Background(), CreateUserWithProfileCommand{
		CreateUserCommand: CreateUserCommand{
			OrganizationID: testOrganizationID,
			Username:       "systemuser",
			UserType:       "normal",
			Status:         "active",
		},
		Profile: CreateUserProfileCommand{DisplayName: "System User", Email: "system@example.com"},
	})
	if err != nil {
		t.Fatalf("CreateUser() returned unexpected error: %v", err)
	}

	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	if record := recordsRepo.created[0]; record.ActorID != "" {
		t.Errorf("audit actorID = %q, want empty for a system action", record.ActorID)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	fixedUUID := "019dup-f000-7000-8000-000000000099"

	userRepo := &mockUserRepo{
		createFn: func(ctx context.Context, user *userdomain.User) error {
			return apperrors.NewConflict("username already exists")
		},
	}

	uow := testUnitOfWork{users: userRepo}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: fixedUUID},
		UOW:   uow,
	}

	cmd := CreateUserWithProfileCommand{
		CreateUserCommand: CreateUserCommand{
			OrganizationID: testOrganizationID,
			Username:       "duplicateuser",
			UserType:       "normal",
			Status:         "active",
		},
		Profile: CreateUserProfileCommand{
			DisplayName: "Duplicate User",
		},
	}

	user, err := svc.CreateUser(context.Background(), cmd)
	if err == nil {
		t.Fatal("CreateUser() expected error for duplicate username, got nil")
	}

	// Verify the error is a conflict error
	var appErr *apperrors.Error
	if !errorAs(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeConflict {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeConflict)
	}

	if user != nil {
		t.Errorf("expected nil user on error, got %+v", user)
	}
}

func TestGetUser_Success(t *testing.T) {
	testUser := newTestUser()
	testOrg := newTestOrganization()

	userRepo := &mockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			if id != testUserID {
				return nil, apperrors.NewNotFound("user not found")
			}
			return testUser, nil
		},
	}

	orgRepo := &mockOrgRepo{
		getByIDFn: func(ctx context.Context, id string) (*orgdomain.Organization, error) {
			if id != testOrganizationID {
				return nil, apperrors.NewNotFound("organization not found")
			}
			return testOrg, nil
		},
	}

	uow := testUnitOfWork{users: userRepo, organizations: orgRepo}

	svc := Service{UOW: uow}

	detail, err := svc.GetUser(context.Background(), testUserID, testOrganizationID)
	if err != nil {
		t.Fatalf("GetUser() returned unexpected error: %v", err)
	}

	if detail.ID != testUserID {
		t.Errorf("detail.ID = %q, want %q", detail.ID, testUserID)
	}
	if detail.Username != testUsername {
		t.Errorf("detail.Username = %q, want %q", detail.Username, testUsername)
	}
	if detail.OrganizationID != testOrganizationID {
		t.Errorf("detail.OrganizationID = %q, want %q", detail.OrganizationID, testOrganizationID)
	}
	if detail.DisplayName != testDisplayName {
		t.Errorf("detail.DisplayName = %q, want %q", detail.DisplayName, testDisplayName)
	}
	if detail.Email != testEmail {
		t.Errorf("detail.Email = %q, want %q", detail.Email, testEmail)
	}
	if detail.UserType != "normal" {
		t.Errorf("detail.UserType = %q, want %q", detail.UserType, "normal")
	}
	if detail.Status != "active" {
		t.Errorf("detail.Status = %q, want %q", detail.Status, "active")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	userRepo := &mockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("user not found")
		},
	}

	uow := testUnitOfWork{users: userRepo}

	svc := Service{UOW: uow}

	detail, err := svc.GetUser(context.Background(), "nonexistent-id", testOrganizationID)
	if err == nil {
		t.Fatal("GetUser() expected error for non-existent user, got nil")
	}

	var appErr *apperrors.Error
	if !errorAs(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeNotFound {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeNotFound)
	}

	if detail != nil {
		t.Errorf("expected nil detail on error, got %+v", detail)
	}
}

func TestListUsers_Success(t *testing.T) {
	user1 := newTestUser()
	user2 := &userdomain.User{
		ID:             "019abcde-f000-7000-8000-000000000002",
		OrganizationID: testOrganizationID,
		Username:       "anotheruser",
		UserType:       "normal",
		Status:         "active",
		Profile: userdomain.UserProfile{
			UserID:      "019abcde-f000-7000-8000-000000000002",
			DisplayName: "Another User",
		},
		CreatedAt: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC),
		Revision:  1,
	}

	userRepo := &mockUserRepo{
		listFn: func(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
			if query.OrganizationID != testOrganizationID {
				return nil, 0, nil
			}
			return []*userdomain.User{user1, user2}, 2, nil
		},
	}

	uow := testUnitOfWork{users: userRepo}

	svc := Service{UOW: uow}

	items, total, err := svc.ListUsers(context.Background(), ListUsersQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: testOrganizationID,
	})
	if err != nil {
		t.Fatalf("ListUsers() returned unexpected error: %v", err)
	}

	if total != 2 {
		t.Errorf("total = %d, want %d", total, 2)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want %d", len(items), 2)
	}

	// Verify first item
	if items[0].ID != testUserID {
		t.Errorf("items[0].ID = %q, want %q", items[0].ID, testUserID)
	}
	if items[0].Username != testUsername {
		t.Errorf("items[0].Username = %q, want %q", items[0].Username, testUsername)
	}
	if items[0].DisplayName != testDisplayName {
		t.Errorf("items[0].DisplayName = %q, want %q", items[0].DisplayName, testDisplayName)
	}

	// Verify second item
	if items[1].ID != "019abcde-f000-7000-8000-000000000002" {
		t.Errorf("items[1].ID = %q, want %q", items[1].ID, "019abcde-f000-7000-8000-000000000002")
	}
	if items[1].Username != "anotheruser" {
		t.Errorf("items[1].Username = %q, want %q", items[1].Username, "anotheruser")
	}
	if items[1].DisplayName != "Another User" {
		t.Errorf("items[1].DisplayName = %q, want %q", items[1].DisplayName, "Another User")
	}
}

func TestUpdateUser_Success(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	testUser := newTestUser()

	var updatedUser *userdomain.User
	var updatedProfile *userdomain.UserProfile
	userRepo := &mockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			if id != testUserID {
				return nil, apperrors.NewNotFound("user not found")
			}
			// Return a copy so the original is not mutated before the test checks
			copy := *testUser
			return &copy, nil
		},
		updateFn: func(ctx context.Context, user *userdomain.User) error {
			updatedUser = user
			return nil
		},
		updateProfileFn: func(_ context.Context, profile *userdomain.UserProfile) error {
			updatedProfile = profile
			return nil
		},
	}

	uow := testUnitOfWork{users: userRepo}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW:   uow,
	}

	newStatus := "disabled"
	isAdmin := true
	tag := "vip"
	karma, ranking := 42, 7
	language, title := "en", "Engineer"

	err := svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		Status:         &newStatus,
		IsAdmin:        &isAdmin,
		Tag:            &tag,
		Karma:          &karma,
		Ranking:        &ranking,
		Language:       &language,
		Title:          &title,
		Revision:       1,
	})
	if err != nil {
		t.Fatalf("PatchUser() returned unexpected error: %v", err)
	}

	// Verify the updated fields were applied
	if updatedUser == nil {
		t.Fatal("expected Update to be called on the repository, but updatedUser is nil")
	}
	if updatedUser.Status != "disabled" {
		t.Errorf("updatedUser.Status = %q, want %q", updatedUser.Status, "disabled")
	}
	if updatedUser.IsAdmin != true {
		t.Errorf("updatedUser.IsAdmin = %v, want %v", updatedUser.IsAdmin, true)
	}
	if updatedUser.Tag != "vip" {
		t.Errorf("updatedUser.Tag = %q, want %q", updatedUser.Tag, "vip")
	}
	if !updatedUser.UpdatedAt.Equal(fixedNow) {
		t.Errorf("updatedUser.UpdatedAt = %v, want %v", updatedUser.UpdatedAt, fixedNow)
	}
	if updatedUser.Revision != 2 {
		t.Errorf("updatedUser.Revision = %d, want %d", updatedUser.Revision, 2)
	}
	if updatedProfile == nil {
		t.Fatal("expected UpdateProfile to be called for extended profile values")
	}
	if updatedProfile.Properties["karma"] != 42 ||
		updatedProfile.Properties["ranking"] != 7 ||
		updatedProfile.Properties["language"] != "en" ||
		updatedProfile.Properties["title"] != "Engineer" {
		t.Errorf("updatedProfile.Properties = %#v, want updated extended values", updatedProfile.Properties)
	}
}

func TestDeleteUser_Success(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)
	testUser := newTestUser()
	testOrg := newTestOrganization()

	var updatedUser *userdomain.User
	sessionsRevoked := false
	tokensRevoked := false

	userRepo := &mockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			if id != testUserID {
				return nil, apperrors.NewNotFound("user not found")
			}
			copy := *testUser
			return &copy, nil
		},
		updateFn: func(ctx context.Context, user *userdomain.User) error {
			updatedUser = user
			return nil
		},
	}

	orgRepo := &mockOrgRepo{
		getByIDFn: func(ctx context.Context, id string) (*orgdomain.Organization, error) {
			if id != testOrganizationID {
				return nil, apperrors.NewNotFound("organization not found")
			}
			return testOrg, nil
		},
	}

	sessionRepo := &mockSessionRepo{
		revokeByUserExceptFn: func(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
			sessionsRevoked = true
			if userID != testUserID {
				t.Errorf("revokeByUserExcept userID = %q, want %q", userID, testUserID)
			}
			if excludeSessionID != "" {
				t.Errorf("revokeByUserExcept excludeSessionID = %q, want empty for delete-user", excludeSessionID)
			}
			return 3, nil
		},
	}
	recordsRepo := &fakeRecordsRepo{}

	tokenRepo := &mockTokenRepo{
		revokeByQueryFn: func(ctx context.Context, query tokendomain.RevokeQuery, revokedAt time.Time) (int64, error) {
			tokensRevoked = true
			if query.UserID != testUserID {
				t.Errorf("revokeByQuery UserID = %q, want %q", query.UserID, testUserID)
			}
			return 5, nil
		},
	}

	uow := testUnitOfWork{
		users:         userRepo,
		organizations: orgRepo,
		sessions:      sessionRepo,
		tokens:        tokenRepo,
		records:       recordsRepo,
	}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW:   uow,
	}

	const actorID = "019operator-0000-7000-8000-000000000001"
	err := svc.DeleteUser(context.Background(), DeleteUserCommand{ID: testUserID, OrganizationID: testOrganizationID, ActorID: actorID})
	if err != nil {
		t.Fatalf("DeleteUser() returned unexpected error: %v", err)
	}

	// Verify soft-delete: DeletedAt should be set
	if updatedUser == nil {
		t.Fatal("expected Update to be called on the repository, but updatedUser is nil")
	}
	if updatedUser.DeletedAt == nil {
		t.Error("expected DeletedAt to be set for soft-deleted user, got nil")
	} else if !updatedUser.DeletedAt.Equal(fixedNow) {
		t.Errorf("updatedUser.DeletedAt = %v, want %v", *updatedUser.DeletedAt, fixedNow)
	}
	// C11③：is_deleted 必须随软删同步（MySQL 镜像无 PG 触发器）。
	if !updatedUser.IsDeleted {
		t.Error("expected IsDeleted to be true for soft-deleted user")
	}
	if !updatedUser.UpdatedAt.Equal(fixedNow) {
		t.Errorf("updatedUser.UpdatedAt = %v, want %v", updatedUser.UpdatedAt, fixedNow)
	}
	if updatedUser.Revision != 2 {
		t.Errorf("updatedUser.Revision = %d, want %d", updatedUser.Revision, 2)
	}

	// Verify sessions and tokens were revoked
	if !sessionsRevoked {
		t.Error("expected RevokeByUserExcept to be called on sessions repository")
	}
	if !tokensRevoked {
		t.Error("expected RevokeByQuery to be called on tokens repository")
	}

	// Verify the audit row was embedded in the same transaction.
	if len(recordsRepo.created) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(recordsRepo.created))
	}
	record := recordsRepo.created[0]
	if record.Action != "user.deleted" {
		t.Errorf("audit action = %q, want %q", record.Action, "user.deleted")
	}
	if record.ResourceType != "User" || record.ResourceID != testUserID {
		t.Errorf("audit resource = %q/%q, want User/%q", record.ResourceType, record.ResourceID, testUserID)
	}
	if record.OrganizationID != testOrganizationID {
		t.Errorf("audit organizationID = %q, want %q", record.OrganizationID, testOrganizationID)
	}
	if record.ActorID != actorID {
		t.Errorf("audit actorID = %q, want %q", record.ActorID, actorID)
	}
}

// ---------------------------------------------------------------------------
// Additional test cases
// ---------------------------------------------------------------------------

func TestCreateUser_InvalidFields(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	fixedUUID := "019newid-f000-7000-8000-000000000099"

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: fixedUUID},
		UOW:   testUnitOfWork{users: &mockUserRepo{}},
	}

	tests := []struct {
		name     string
		command  CreateUserWithProfileCommand
		wantCode apperrors.Code
	}{
		{
			name:     "missing organizationId",
			wantCode: apperrors.CodeInvalidArgument,
			command: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					Username: "testuser",
					UserType: "normal",
					Status:   "active",
				},
			},
		},
		{
			name:     "missing username",
			wantCode: apperrors.CodeInvalidArgument,
			command: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: testOrganizationID,
					UserType:       "normal",
					Status:         "active",
				},
			},
		},
		{
			name:     "missing userType",
			wantCode: apperrors.CodeInvalidArgument,
			command: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: testOrganizationID,
					Username:       "testuser",
					Status:         "active",
				},
			},
		},
		{
			name:     "missing status",
			wantCode: apperrors.CodeInvalidArgument,
			command: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: testOrganizationID,
					Username:       "testuser",
					UserType:       "normal",
				},
			},
		},
		{
			name:     "invalid email format",
			wantCode: apperrors.CodeUnprocessableEntity,
			command: CreateUserWithProfileCommand{
				CreateUserCommand: CreateUserCommand{
					OrganizationID: testOrganizationID,
					Username:       "testuser",
					UserType:       "normal",
					Status:         "active",
				},
				Profile: CreateUserProfileCommand{
					DisplayName: "Test User",
					Email:       "not-an-email",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateUser(context.Background(), tt.command)
			if err == nil {
				t.Fatalf("CreateUser() expected error for %s, got nil", tt.name)
			}
			assertUsersAppError(t, err, tt.wantCode)
		})
	}
}

func TestUpdateUser_RevisionMismatch(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	testUser := newTestUser()

	userRepo := &mockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			if id != testUserID {
				return nil, apperrors.NewNotFound("user not found")
			}
			copy := *testUser
			copy.Revision = 5 // current revision is 5
			return &copy, nil
		},
	}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW:   uow,
	}

	newStatus := "disabled"
	err := svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		Status:         &newStatus,
		Revision:       3, // stale revision
	})
	if err == nil {
		t.Fatal("PatchUser() expected error for revision mismatch, got nil")
	}

	var appErr *apperrors.Error
	if !errorAs(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodePreconditionFailed {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodePreconditionFailed)
	}
	// 规范 4.10：412 响应体指明当前 revision，客户端重取后可直接重试。
	if appErr.Meta["revision"] != int64(5) {
		t.Errorf("meta revision = %v, want current 5", appErr.Meta["revision"])
	}
}

// TestPatchUser_MissingRevisionRejected pins 规范 4.10 / R2-C05: a patch
// without an optimistic-lock precondition is 428, not last-write-wins, and
// leaves the stored user untouched.
func TestPatchUser_MissingRevisionRejected(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	testUser := newTestUser()

	updated := false
	userRepo := &mockUserRepo{
		getByIDForOrganizationFn: func(ctx context.Context, id, organizationID string) (*userdomain.User, error) {
			copy := *testUser
			return &copy, nil
		},
		updateForOrganizationFn: func(ctx context.Context, user *userdomain.User, organizationID string) error {
			updated = true
			return nil
		},
	}

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW:   testUnitOfWork{users: userRepo},
	}

	newStatus := "disabled"
	err := svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             testUserID,
		Status:         &newStatus,
	})
	var appErr *apperrors.Error
	if !errorAs(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %v", err)
	}
	if appErr.Code != apperrors.CodePreconditionRequired {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodePreconditionRequired)
	}
	if appErr.HTTPStatus() != 428 {
		t.Errorf("http status = %d, want 428", appErr.HTTPStatus())
	}
	if updated {
		t.Error("PatchUser() must not write when the precondition is missing")
	}
}

func TestUpdateUser_NotFound(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)

	userRepo := &mockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("user not found")
		},
	}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW:   uow,
	}

	newStatus := "disabled"
	err := svc.PatchUser(context.Background(), PatchUserCommand{
		OrganizationID: testOrganizationID,
		ID:             "nonexistent-id",
		Status:         &newStatus,
		Revision:       1,
	})
	if err == nil {
		t.Fatal("PatchUser() expected error for non-existent user, got nil")
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 14, 0, 0, 0, time.UTC)

	userRepo := &mockUserRepo{
		getByIDFn: func(ctx context.Context, id string) (*userdomain.User, error) {
			return nil, apperrors.NewNotFound("user not found")
		},
	}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UOW:   uow,
	}

	err := svc.DeleteUser(context.Background(), DeleteUserCommand{ID: "nonexistent-id", OrganizationID: testOrganizationID})
	if err == nil {
		t.Fatal("DeleteUser() expected error for non-existent user, got nil")
	}

	var appErr *apperrors.Error
	if !errorAs(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeNotFound {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeNotFound)
	}
}

func TestDeleteUser_EmptyID(t *testing.T) {
	svc := Service{UOW: testUnitOfWork{}}

	err := svc.DeleteUser(context.Background(), DeleteUserCommand{ID: "", OrganizationID: testOrganizationID})
	if err == nil {
		t.Fatal("DeleteUser() expected error for empty ID, got nil")
	}

	var appErr *apperrors.Error
	if !errorAs(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != apperrors.CodeInvalidArgument {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeInvalidArgument)
	}
}

func TestListUsers_Empty(t *testing.T) {
	userRepo := &mockUserRepo{
		listFn: func(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
			return []*userdomain.User{}, 0, nil
		},
	}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	items, total, err := svc.ListUsers(context.Background(), ListUsersQuery{
		Page:           1,
		PageSize:       50,
		OrganizationID: testOrganizationID,
	})
	if err != nil {
		t.Fatalf("ListUsers() returned unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
}

func TestGetUserCount_Success(t *testing.T) {
	userRepo := &mockUserRepo{}

	uow := testUnitOfWork{users: userRepo}
	svc := Service{UOW: uow}

	// The default mockUserRepo.Count returns 0, nil
	count, err := svc.GetUserCount(context.Background(), testOrganizationID)
	if err != nil {
		t.Fatalf("GetUserCount() returned unexpected error: %v", err)
	}
	// Default mock returns 0
	if count != 0 {
		t.Errorf("count = %d, want 0 (default mock)", count)
	}
}

func TestNewUser_Success(t *testing.T) {
	fixedNow := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)

	svc := Service{
		Clock: fakeClock{fixedTime: fixedNow},
		UUID:  fakeUUIDGenerator{fixedID: "test-uuid-123"},
	}

	user, err := svc.NewUser(CreateUserCommand{
		OrganizationID: testOrganizationID,
		Username:       "testuser",
		UserType:       "normal",
		Status:         "active",
	})
	if err != nil {
		t.Fatalf("NewUser() unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("NewUser() returned nil user")
	}
	if user.ID != "test-uuid-123" {
		t.Errorf("user.ID = %q, want %q", user.ID, "test-uuid-123")
	}
	if user.Username != "testuser" {
		t.Errorf("user.Username = %q, want %q", user.Username, "testuser")
	}
}

// errorAs is a helper that wraps errors.As for consistent usage in tests.
func errorAs(err error, target any) bool {
	return errors.As(err, target)
}
