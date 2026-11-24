package auth

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"

	"github.com/pquerna/otp/totp"

	"github.com/guliangiam/guliangiam/cred"
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
// Fake implementations for login tests
// ---------------------------------------------------------------------------

// fakeClock returns a fixed time for deterministic tests.
type fakeClock struct {
	fixedTime time.Time
}

func (f fakeClock) Now() time.Time { return f.fixedTime }

// fakeUUIDGenerator returns a fixed UUID string for deterministic tests.
type fakeUUIDGenerator struct {
	fixedID string
}

func (f fakeUUIDGenerator) NewString() (string, error) { return f.fixedID, nil }

// loginTestUnitOfWork is a mock UnitOfWork that holds in-memory repositories
// for the domain objects needed by the Login flow.
type loginTestUnitOfWork struct {
	orgRepo        *loginTestOrgRepository
	userRepo       *loginTestUserRepository
	appRepo        *loginTestAppRepository
	sessRepo       *loginTestSessionRepository
	lockoutAttempt *loginTestLockoutAttemptRepository
	challengeRepo  *loginTestChallengeRepository
	// qrRepo/tokenRepo are only wired by the QR Code login tests; nil means
	// "this flow does not touch QR sessions / tokens" (unchanged behaviour).
	qrRepo    qrcodesessions.Repository
	tokenRepo tokendomain.Repository
	verifRepo verificationsdomain.Repository
	// webAuthnRepo is only wired by the WebAuthn login tests (nil = this flow
	// does not touch passkey credentials).
	webAuthnRepo webauthndomain.Repository
	// faceidRepo is only wired by the Face ID login tests.
	faceidRepo faceidsessions.Repository
	// workloadRepo is nil in legacy login tests and explicitly supplied by WLI tests.
	workloadRepo workloaddomain.Repository
	// trustedDeviceRepo is only wired by trusted-device login tests (nil keeps
	// the legacy "repository unavailable → fall back to MFA" behaviour).
	trustedDeviceRepo trusteddevicesdomain.Repository
	// recordsRepo is only wired when a test asserts an audit side effect.
	recordsRepo recordsdomain.Repository
	// outboxRepo is only wired when a test asserts an outbox side effect.
	outboxRepo domainoutbox.Repository
	// captchaRepo is only wired by the login captcha-gate tests (nil keeps the
	// "no provider configured → fail-open" behaviour for every other flow).
	captchaRepo captchaproviderdomain.Repository
}

func (u loginTestUnitOfWork) Outbox() domainoutbox.Repository { return u.outboxRepo }
func (u loginTestUnitOfWork) Applications() applications.Repository {
	return u.appRepo
}
func (u loginTestUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository {
	return nil
}
func (u loginTestUnitOfWork) Certs() certdomain.Repository         { return nil }
func (u loginTestUnitOfWork) Adapters() adapterdomain.Repository   { return nil }
func (u loginTestUnitOfWork) Agents() agentdomain.Repository       { return nil }
func (u loginTestUnitOfWork) Enforcers() enforcerdomain.Repository { return nil }
func (u loginTestUnitOfWork) Models() modeldomain.Repository       { return nil }
func (u loginTestUnitOfWork) Syncers() syncerdomain.Repository     { return nil }
func (u loginTestUnitOfWork) Webhooks() webhookdomain.Repository   { return nil }
func (u loginTestUnitOfWork) Ldaps() ldapdomain.Repository         { return nil }
func (u loginTestUnitOfWork) Forms() formdomain.Repository         { return nil }
func (u loginTestUnitOfWork) Orders() orderdomain.Repository       { return nil }
func (u loginTestUnitOfWork) Payments() paymentdomain.Repository   { return nil }
func (u loginTestUnitOfWork) Plans() plandomain.Repository         { return nil }

func (u loginTestUnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository { return nil }
func (u loginTestUnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return nil
}
func (u loginTestUnitOfWork) CrossTenantOps() crosstenantops.TicketRepository      { return nil }
func (u loginTestUnitOfWork) Pricings() pricingdomain.Repository                   { return nil }
func (u loginTestUnitOfWork) Products() productdomain.Repository                   { return nil }
func (u loginTestUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u loginTestUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u loginTestUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u loginTestUnitOfWork) Consents() consentdomain.Repository                   { return nil }
func (u loginTestUnitOfWork) Gateway() gatewaydomain.Repository                    { return nil }
func (u loginTestUnitOfWork) Groups() groupsdomain.Repository                      { return nil }
func (u loginTestUnitOfWork) Invitations() invitationsdomain.Repository            { return nil }
func (u loginTestUnitOfWork) MfaChallenges() mfachallengedomain.Repository         { return u.challengeRepo }
func (u loginTestUnitOfWork) Organizations() orgdomain.Repository                  { return u.orgRepo }
func (u loginTestUnitOfWork) Permissions() permissiondomain.Repository             { return nil }
func (u loginTestUnitOfWork) Providers() providerdomain.Repository                 { return nil }
func (u loginTestUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u loginTestUnitOfWork) Sessions() sessiondomain.Repository                   { return u.sessRepo }
func (u loginTestUnitOfWork) Tokens() tokendomain.Repository                       { return u.tokenRepo }
func (u loginTestUnitOfWork) Users() userdomain.Repository                         { return u.userRepo }
func (u loginTestUnitOfWork) Records() recordsdomain.Repository                    { return u.recordsRepo }
func (u loginTestUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u loginTestUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u loginTestUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u loginTestUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u loginTestUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u loginTestUnitOfWork) Verifications() verificationsdomain.Repository        { return u.verifRepo }
func (u loginTestUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return u.webAuthnRepo }
func (u loginTestUnitOfWork) CaptchaProviders() captchaproviderdomain.Repository {
	return u.captchaRepo
}
func (u loginTestUnitOfWork) NotificationProviders() notificationproviderdomain.Repository {
	return nil
}
func (u loginTestUnitOfWork) Notifications() notificationsdomain.Repository    { return nil }
func (u loginTestUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository { return nil }
func (u loginTestUnitOfWork) Rules() ruledomain.Repository                     { return nil }
func (u loginTestUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository     { return nil }
func (u loginTestUnitOfWork) Keys() keysdomain.Repository                      { return nil }
func (u loginTestUnitOfWork) Transactions() transactiondomain.Repository       { return nil }
func (u loginTestUnitOfWork) WebhookEvents() webhookeventdomain.Repository     { return nil }
func (u loginTestUnitOfWork) QRCodeSessions() qrcodesessions.Repository        { return u.qrRepo }
func (u loginTestUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository      { return nil }

// fakeWeb3IdentitiesRepo — 最小 fake，所有方法返回 ErrNotFound。
// V8 Layer 2 地址关联是可选功能（没有 web3_identity_mapping 也能登录），
// 测试不需要覆盖这条路径 — 真实代码已用 errors.Is 容错。
type fakeWeb3IdentitiesRepo struct{}

func (f *fakeWeb3IdentitiesRepo) Create(_ context.Context, _ *web3identity.Mapping) error {
	return nil
}
func (f *fakeWeb3IdentitiesRepo) GetByCredentialID(_ context.Context, _ string) (*web3identity.Mapping, error) {
	return nil, web3identity.ErrNotFound
}
func (f *fakeWeb3IdentitiesRepo) GetByWalletAddress(_ context.Context, _ string) (*web3identity.Mapping, error) {
	return nil, web3identity.ErrNotFound
}
func (f *fakeWeb3IdentitiesRepo) TouchLastUsedAt(_ context.Context, _ string, _ time.Time) error {
	return nil
}

func (u loginTestUnitOfWork) Web3Identities() web3identity.Repository {
	return &fakeWeb3IdentitiesRepo{}
}
func (u loginTestUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return u.faceidRepo }
func (u loginTestUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u loginTestUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u loginTestUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return u.lockoutAttempt }

func (u loginTestUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u loginTestUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u loginTestUnitOfWork) Workload() workloaddomain.Repository { return u.workloadRepo }

func (u loginTestUnitOfWork) IDV() idvdomain.Repository { return nil }

func (u loginTestUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u loginTestUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u loginTestUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u loginTestUnitOfWork) IGA() igadomain.Repository { return nil }

// TrustedDevices lets trusted-device tests inject a fake repository; nil keeps
// the legacy behaviour (Login falls back to the normal MFA flow).
func (u loginTestUnitOfWork) TrustedDevices() trusteddevicesdomain.Repository {
	return u.trustedDeviceRepo
}

// loginTestOrgRepository is an in-memory organization repository for tests.
type loginTestOrgRepository struct {
	orgs map[string]*orgdomain.Organization // keyed by slug
}

func (r *loginTestOrgRepository) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	for _, org := range r.orgs {
		if org.ID == id {
			return org, nil
		}
	}
	return nil, apperrors.NewNotFound("organization not found")
}

func (r *loginTestOrgRepository) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	org, ok := r.orgs[slug]
	if !ok {
		return nil, apperrors.NewNotFound("organization not found")
	}
	return org, nil
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *loginTestOrgRepository) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *loginTestOrgRepository) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, nil
}
func (r *loginTestOrgRepository) Create(ctx context.Context, organization *orgdomain.Organization) error {
	return nil
}
func (r *loginTestOrgRepository) Update(ctx context.Context, organization *orgdomain.Organization) error {
	return nil
}
func (r *loginTestOrgRepository) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	return nil
}
func (r *loginTestOrgRepository) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	return nil
}
func (r *loginTestOrgRepository) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	return nil
}

func (r *loginTestOrgRepository) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}

func (r *loginTestOrgRepository) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

// loginTestUserRepository is an in-memory user repository for tests.
type loginTestUserRepository struct {
	users      map[string]*userdomain.User            // keyed by id
	usersByOrg map[string]map[string]*userdomain.User // orgID -> username -> user
	created    []*userdomain.User                     // 每次 Create 的入参，按顺序
	// locked 记录 LockByID 的入参；callLog 非空时同时记录调用顺序。
	locked []string
	// updatedCredentials 记录每次 UpdateCredentials 的入参，供算法升级/改密断言。
	updatedCredentials []*userdomain.UserCredentials
	callLog            *loginTestCallLog
	// revisionGuard 打开后，Update 复刻生产仓储的乐观锁约定：入参 revision
	// 必须等于载入时的 revision+1（SQL 用 Revision-1 作为 WHERE 值）。
	revisionGuard   bool
	loadedRevisions map[string]int64
}

func (r *loginTestUserRepository) recordLoadedRevision(user *userdomain.User) {
	if !r.revisionGuard || user == nil {
		return
	}
	if r.loadedRevisions == nil {
		r.loadedRevisions = make(map[string]int64)
	}
	r.loadedRevisions[user.ID] = user.Revision
}

// LockByID 复刻仓储的行级锁语义：只记录调用（假仓储没有事务，锁本身不生效）。
func (r *loginTestUserRepository) LockByID(ctx context.Context, id string) error {
	r.locked = append(r.locked, id)
	r.callLog.add("users.LockByID")
	return nil
}

func (r *loginTestUserRepository) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, apperrors.NewNotFound("user not found")
	}
	r.recordLoadedRevision(user)
	return user, nil
}

func (r *loginTestUserRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*userdomain.User, error) {
	user, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return user, nil
}

func (r *loginTestUserRepository) GetByUsername(ctx context.Context, organizationID, username string) (*userdomain.User, error) {
	orgUsers, ok := r.usersByOrg[organizationID]
	if !ok {
		return nil, apperrors.NewNotFound("user not found")
	}
	user, ok := orgUsers[username]
	if !ok {
		return nil, apperrors.NewNotFound("user not found")
	}
	r.recordLoadedRevision(user)
	return user, nil
}

// GetByLoginIdentifier mirrors the production resolution order (username exact
// > email case-insensitive > phone exact) and fails closed when more than one
// distinct user matches, so login tests exercise the same contract as the SQL
// repositories.
func (r *loginTestUserRepository) GetByLoginIdentifier(ctx context.Context, organizationID, identifier string) (*userdomain.User, error) {
	identifier = strings.TrimSpace(identifier)
	orgUsers, ok := r.usersByOrg[organizationID]
	if !ok {
		return nil, apperrors.NewNotFound("user not found")
	}
	if user, ok := orgUsers[identifier]; ok {
		r.recordLoadedRevision(user)
		return user, nil
	}

	matches := make([]*userdomain.User, 0, 1)
	for _, user := range orgUsers {
		if user.Profile.Email != "" && strings.EqualFold(identifier, user.Profile.Email) {
			matches = append(matches, user)
			continue
		}
		if user.Profile.Phone != "" && identifier == user.Profile.Phone {
			matches = append(matches, user)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *loginTestUserRepository) List(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	return nil, 0, nil
}
func (r *loginTestUserRepository) Create(ctx context.Context, user *userdomain.User) error {
	if r.users == nil {
		r.users = make(map[string]*userdomain.User)
	}
	if r.usersByOrg == nil {
		r.usersByOrg = make(map[string]map[string]*userdomain.User)
	}
	r.created = append(r.created, user)
	r.users[user.ID] = user
	if r.usersByOrg[user.OrganizationID] == nil {
		r.usersByOrg[user.OrganizationID] = make(map[string]*userdomain.User)
	}
	r.usersByOrg[user.OrganizationID][user.Username] = user
	return nil
}
func (r *loginTestUserRepository) Update(ctx context.Context, user *userdomain.User) error {
	if !r.revisionGuard {
		return nil
	}
	loaded, ok := r.loadedRevisions[user.ID]
	if !ok {
		return apperrors.NewNotFound("user not found")
	}
	if user.Revision-1 != loaded {
		return apperrors.NewConflict("user revision conflict")
	}
	if stored, ok := r.users[user.ID]; ok {
		stored.Revision = user.Revision
	}
	// 成功更新后，下一次 Update 的基准就是本次写入的 revision（与 SQL 的
	// WHERE revision = $n 语义一致）。
	r.loadedRevisions[user.ID] = user.Revision
	return nil
}
func (r *loginTestUserRepository) UpdateForOrganization(ctx context.Context, user *userdomain.User, organizationID string) error {
	return r.Update(ctx, user)
}
func (r *loginTestUserRepository) UpdateProfile(ctx context.Context, profile *userdomain.UserProfile) error {
	if user, ok := r.users[profile.UserID]; ok {
		user.Profile = *profile
	}
	return nil
}

func (r *loginTestUserRepository) SetContactVerifiedForOrganization(ctx context.Context, userID, organizationID string, emailVerified, phoneVerified bool) error {
	user, ok := r.users[userID]
	if !ok || user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	user.Profile.EmailVerified = emailVerified
	user.Profile.PhoneVerified = phoneVerified
	return nil
}

func (r *loginTestUserRepository) UpdateCollectionsForOrganization(ctx context.Context, userID, organizationID string, collections userdomain.UserCollections) error {
	user, ok := r.users[userID]
	if !ok || user.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	user.Addresses = collections.Addresses
	user.ManagedAccounts = collections.ManagedAccounts
	user.FaceIds = collections.FaceIds
	user.Cart = collections.Cart
	user.MfaItems = collections.MfaItems
	user.MfaAccounts = collections.MfaAccounts
	user.ApplicationScopes = collections.ApplicationScopes
	return nil
}
func (r *loginTestUserRepository) UpdateCredentials(ctx context.Context, credentials *userdomain.UserCredentials) error {
	r.updatedCredentials = append(r.updatedCredentials, credentials)
	// 复刻生产仓储的全行 upsert：整行覆盖，包括 access_key/access_secret_hash。
	if user, ok := r.users[credentials.UserID]; ok {
		user.Credentials = *credentials
	}
	return nil
}

// UpdatePasswordCredential 复刻定向更新：只覆盖口令列，保留访问密钥。
func (r *loginTestUserRepository) UpdatePasswordCredential(ctx context.Context, credentials *userdomain.UserCredentials) error {
	r.updatedCredentials = append(r.updatedCredentials, credentials)
	if user, ok := r.users[credentials.UserID]; ok {
		user.Credentials.HasPassword = credentials.HasPassword
		user.Credentials.PasswordHash = credentials.PasswordHash
		user.Credentials.PasswordAlgo = credentials.PasswordAlgo
		user.Credentials.PasswordUpdatedAt = credentials.PasswordUpdatedAt
		user.Credentials.NeedUpdatePassword = credentials.NeedUpdatePassword
	}
	return nil
}
func (r *loginTestUserRepository) ReplaceIdentities(ctx context.Context, userID string, identities []userdomain.UserIdentity) error {
	return nil
}
func (r *loginTestUserRepository) CreateIdentity(ctx context.Context, identity *userdomain.UserIdentity) error {
	return nil
}
func (r *loginTestUserRepository) DeleteIdentity(ctx context.Context, userID, identityID string) error {
	return nil
}
func (r *loginTestUserRepository) ReplaceMfaMethods(ctx context.Context, userID string, methods []userdomain.UserMfaMethod) error {
	return nil
}
func (r *loginTestUserRepository) ReplaceRoleBindings(ctx context.Context, userID string, bindings []userdomain.UserRoleBinding) error {
	return nil
}
func (r *loginTestUserRepository) ListRoleNames(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}
func (r *loginTestUserRepository) ReplaceGroupMemberships(ctx context.Context, userID string, memberships []userdomain.UserGroupMembership) error {
	return nil
}
func (r *loginTestUserRepository) Delete(ctx context.Context, id, organizationID string) error {
	return nil
}
func (r *loginTestUserRepository) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error) {
	return nil, nil
}
func (r *loginTestUserRepository) CreatePasswordHistory(ctx context.Context, entry *userdomain.PasswordHistoryEntry) error {
	return nil
}
func (r *loginTestUserRepository) DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error {
	return nil
}
func (r *loginTestUserRepository) Count(ctx context.Context, organizationID string) (int64, error) {
	return 0, nil
}

func (r *loginTestUserRepository) GetByProviderIdentity(ctx context.Context, providerID, subject string) (*userdomain.User, error) {
	return nil, apperrors.NewNotFound("user not found")
}

func (r *loginTestUserRepository) HasDeletedWithUsername(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

// loginTestAppRepository is an in-memory application repository for tests.
type loginTestAppRepository struct {
	apps map[string]*applications.Application // keyed by id
	// jwksURIs models the app_client_jwks_uris side table (OPEN-API-61).
	jwksURIs map[string]string
	// byClientID is populated only by tests that exercise the OAuth client
	// lookup paths. It is separate from `apps` so existing tests that expect
	// GetByClientID to fail keep failing.
	byClientID map[string]*applications.Application
}

func (r *loginTestAppRepository) GetByID(ctx context.Context, id string) (*applications.Application, error) {
	app, ok := r.apps[id]
	if !ok {
		return nil, apperrors.NewNotFound("application not found")
	}
	return app, nil
}
func (r *loginTestAppRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*applications.Application, error) {
	app, ok := r.apps[id]
	if !ok || app.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("application not found")
	}
	return app, nil
}
func (r *loginTestAppRepository) GetByClientID(ctx context.Context, clientID string) (*applications.Application, error) {
	if app, ok := r.byClientID[clientID]; ok {
		return app, nil
	}
	return nil, apperrors.NewNotFound("application not found")
}
func (r *loginTestAppRepository) GetByOrganizationAndSlug(ctx context.Context, organizationID, slug string) (*applications.Application, error) {
	for _, app := range r.apps {
		if app.OrganizationID == organizationID && app.Slug == slug {
			return app, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}
func (r *loginTestAppRepository) List(ctx context.Context, query applications.ListQuery) ([]*applications.Application, int64, error) {
	return nil, 0, nil
}
func (r *loginTestAppRepository) Create(ctx context.Context, application *applications.Application) error {
	return nil
}
func (r *loginTestAppRepository) Delete(ctx context.Context, id, organizationID string) error {
	return nil
}
func (r *loginTestAppRepository) Update(ctx context.Context, application *applications.Application) error {
	return nil
}
func (r *loginTestAppRepository) ReplaceProviderBindings(ctx context.Context, applicationID string, bindings []applications.ApplicationProviderBinding) error {
	return nil
}
func (r *loginTestAppRepository) HasDeletedWithOrgAndSlug(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (r *loginTestAppRepository) FindHostConflicts(_ context.Context, _, _ string, _ []string) ([]applications.HostConflict, error) {
	return nil, nil
}

func (r *loginTestAppRepository) SetRegistrationAccessToken(_ context.Context, _, _ string) error {
	return nil
}

func (r *loginTestAppRepository) GetRegistrationAccessTokenHash(_ context.Context, _ string) (string, error) {
	return "", nil
}

// loginTestCallLog records the order of cross-repository calls a test cares
// about (e.g. "the user row is locked before the session count is read").
type loginTestCallLog struct {
	calls []string
}

func (l *loginTestCallLog) add(call string) {
	if l == nil {
		return
	}
	l.calls = append(l.calls, call)
}

// loginTestSessionRepository is an in-memory session repository for tests.
type loginTestSessionRepository struct {
	sessions map[string]*sessiondomain.Session // keyed by id
	revoked  []string                          // ids passed to Revoke, in order
	// revokedByUserAndApp 记录排他登录调用（userID, applicationID, excludeSessionID）。
	revokedByUserAndApp [][3]string
	// callLog 非空时记录 List 调用，用于断言「先锁用户行、再读会话数」。
	callLog *loginTestCallLog
}

func (r *loginTestSessionRepository) GetByID(ctx context.Context, id string) (*sessiondomain.Session, error) {
	return nil, apperrors.NewNotFound("session not found")
}
func (r *loginTestSessionRepository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*sessiondomain.Session, error) {
	return nil, apperrors.NewNotFound("session not found")
}
func (r *loginTestSessionRepository) List(ctx context.Context, query sessiondomain.ListQuery) ([]*sessiondomain.Session, int64, error) {
	// 只有并发会话上限/异常登录检测会读这条路径；两个调用点都按 UserID 过滤，
	// 因此这里实现成最小可用版本（不模拟分页），但排序必须与仓储一致：
	// created_at DESC（最新在前），enforceSessionLimit 依赖「最旧的排在末尾」。
	r.callLog.add("sessions.List")
	out := make([]*sessiondomain.Session, 0, len(r.sessions))
	for _, sess := range r.sessions {
		if query.UserID != "" && sess.UserID != query.UserID {
			continue
		}
		if query.ActiveOnly != nil && *query.ActiveOnly && sess.SessionState != "active" {
			continue
		}
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, int64(len(out)), nil
}
func (r *loginTestSessionRepository) Create(ctx context.Context, session *sessiondomain.Session) error {
	if r.sessions == nil {
		r.sessions = make(map[string]*sessiondomain.Session)
	}
	r.sessions[session.ID] = session
	return nil
}
func (r *loginTestSessionRepository) Update(ctx context.Context, session *sessiondomain.Session) error {
	return nil
}
func (r *loginTestSessionRepository) Touch(ctx context.Context, id string, touchedAt time.Time) error {
	return nil
}
func (r *loginTestSessionRepository) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	r.revoked = append(r.revoked, id)
	return nil
}
func (r *loginTestSessionRepository) RevokeForOrganization(ctx context.Context, id, organizationID string, revokedAt time.Time) error {
	return r.Revoke(ctx, id, revokedAt)
}
func (r *loginTestSessionRepository) RevokeByUserExcept(ctx context.Context, userID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (r *loginTestSessionRepository) RevokeByUserAndApp(ctx context.Context, userID, applicationID, excludeSessionID string, revokedAt time.Time) (int64, error) {
	r.revokedByUserAndApp = append(r.revokedByUserAndApp, [3]string{userID, applicationID, excludeSessionID})
	return 0, nil
}
func (r *loginTestSessionRepository) Delete(ctx context.Context, id string) error { return nil }
func (r *loginTestSessionRepository) DeleteForOrganization(ctx context.Context, id, organizationID string) error {
	return nil
}
func (r *loginTestSessionRepository) CleanupExpired(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *loginTestSessionRepository) RevokeByOrganization(ctx context.Context, organizationID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}
func (r *loginTestSessionRepository) RevokeByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

// loginTestLockoutAttemptRepository is an in-memory lockout attempt repository for tests.
type loginTestLockoutAttemptRepository struct {
	attempts map[string]*lockoutattempts.Attempt // keyed by userID
}

func (r *loginTestLockoutAttemptRepository) GetByUserID(ctx context.Context, userID string) (*lockoutattempts.Attempt, error) {
	if r == nil || r.attempts == nil {
		return nil, nil
	}
	attempt, ok := r.attempts[userID]
	if !ok {
		return nil, nil
	}
	return attempt, nil
}
func (r *loginTestLockoutAttemptRepository) Create(ctx context.Context, attempt *lockoutattempts.Attempt) error {
	if r.attempts == nil {
		r.attempts = make(map[string]*lockoutattempts.Attempt)
	}
	r.attempts[attempt.UserID] = attempt
	return nil
}
func (r *loginTestLockoutAttemptRepository) Update(ctx context.Context, attempt *lockoutattempts.Attempt) error {
	if r.attempts == nil {
		r.attempts = make(map[string]*lockoutattempts.Attempt)
	}
	r.attempts[attempt.UserID] = attempt
	return nil
}
func (r *loginTestLockoutAttemptRepository) DeleteByUserID(ctx context.Context, userID string) error {
	if r.attempts != nil {
		delete(r.attempts, userID)
	}
	return nil
}
func (r *loginTestLockoutAttemptRepository) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

// RecordFailure mirrors the lockout state transition for in-memory tests.
func (r *loginTestLockoutAttemptRepository) RecordFailure(ctx context.Context, userID string, maxAttempts, lockDurationMinutes int, now time.Time) (*lockoutattempts.Attempt, error) {
	if r.attempts == nil {
		r.attempts = make(map[string]*lockoutattempts.Attempt)
	}

	attempt, ok := r.attempts[userID]
	if !ok {
		attempt = &lockoutattempts.Attempt{
			UserID:          userID,
			CreatedAt:       now,
			WindowStartedAt: now,
			UpdatedAt:       now,
		}
		r.attempts[userID] = attempt
	}
	if attempt.WindowStartedAt.IsZero() {
		attempt.WindowStartedAt = attempt.CreatedAt
		if attempt.WindowStartedAt.IsZero() {
			attempt.WindowStartedAt = now
		}
	}
	attempt.RecordFailure(now, maxAttempts, time.Duration(lockDurationMinutes)*time.Minute)
	attempt.UpdatedAt = now
	return attempt, nil
}

// loginTestChallengeRepository is an in-memory MFA challenge repository for tests.
type loginTestChallengeRepository struct {
	items map[string]*mfachallengedomain.Challenge
}

func (r *loginTestChallengeRepository) GetByID(ctx context.Context, id string) (*mfachallengedomain.Challenge, error) {
	if r.items == nil {
		return nil, apperrors.NewNotFound("challenge not found")
	}
	challenge, ok := r.items[id]
	if !ok {
		return nil, apperrors.NewNotFound("challenge not found")
	}
	return challenge, nil
}

func (r *loginTestChallengeRepository) Create(ctx context.Context, challenge *mfachallengedomain.Challenge) error {
	if r.items == nil {
		r.items = make(map[string]*mfachallengedomain.Challenge)
	}
	r.items[challenge.ID] = challenge
	return nil
}

func (r *loginTestChallengeRepository) Update(ctx context.Context, challenge *mfachallengedomain.Challenge) error {
	if r.items == nil {
		r.items = make(map[string]*mfachallengedomain.Challenge)
	}
	r.items[challenge.ID] = challenge
	return nil
}

func (r *loginTestChallengeRepository) Delete(ctx context.Context, id string) error {
	if r.items != nil {
		delete(r.items, id)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

const (
	testOrgID  = "019org-0000-0000-0000-000000000001"
	testUserID = "019usr-0000-0000-0000-000000000001"
	testAppID  = "01a00000-0000-7000-8000-0000000000a1"
	testSessID = "019ses-0000-0000-0000-000000000001"

	testOrgSlug     = "test-org"
	testUsername    = "testuser"
	testPassword    = "TestP@ss123" // not a common password
	testAppName     = "test-app"
	testAppClientID = "test-client-id"
)

// newTestService creates a Service with fake dependencies and the given UOW.
func newTestService(uow loginTestUnitOfWork) Service {
	return Service{
		Clock:         fakeClock{fixedTime: time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)},
		UUIDGenerator: fakeUUIDGenerator{fixedID: testSessID},
		UOW:           uow,
		// 用例里的断言都以这个来源签名（signWebAuthnAssertion），因此它必须在允许清单里；
		// 需要验证「缺配置 fail-closed」的用例会自行把它清空。
		WebAuthnAllowedOrigins: []string{"https://login.example.com"},
	}
}

// testPasswordHash is the argon2id hash of testPassword. It is computed once
// so the many login fixtures do not pay the KDF cost per test.
var testPasswordHash = mustHashTestPassword()

func mustHashTestPassword() string {
	manager := cred.GetCredManager(userdomain.PasswordAlgorithmArgon2ID)
	if manager == nil {
		panic("argon2id credential manager unavailable")
	}
	hash := manager.GetHashedPassword(testPassword, "")
	if hash == "" {
		panic("failed to hash test password")
	}
	return hash
}

// newTestUser creates a user with the given overrides. Credentials match the
// production write contract: a local password is always argon2id.
func newTestUser(overrides ...func(*userdomain.User)) *userdomain.User {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	u := &userdomain.User{
		ID:             testUserID,
		OrganizationID: testOrgID,
		Username:       testUsername,
		Status:         "active",
		IsForbidden:    false,
		CreatedAt:      now,
		UpdatedAt:      now,
		Profile: userdomain.UserProfile{
			UserID:      testUserID,
			DisplayName: "Test User",
			Email:       "testuser@example.com",
		},
		Credentials: userdomain.UserCredentials{
			UserID:       testUserID,
			HasPassword:  true,
			PasswordHash: testPasswordHash,
			PasswordAlgo: userdomain.PasswordAlgorithmArgon2ID,
		},
	}
	for _, fn := range overrides {
		fn(u)
	}
	return u
}

func newTestOrg(overrides ...func(*orgdomain.Organization)) *orgdomain.Organization {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	org := &orgdomain.Organization{
		ID:        testOrgID,
		Slug:      testOrgSlug,
		UpdatedAt: now,
		// 测试固定 8 位口令以覆盖算法/历史路径；生产默认下限由
		// domain.DefaultPasswordMinLength（14）提供（DEF-11）。
		Security: orgdomain.OrganizationSecurityPolicy{
			OrganizationID: testOrgID,
			PasswordPolicy: map[string]any{"minLength": 8},
		},
	}
	for _, fn := range overrides {
		fn(org)
	}
	return org
}

func newTestApp(overrides ...func(*applications.Application)) *applications.Application {
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	app := &applications.Application{
		ID:                  testAppID,
		OrganizationID:      testOrgID,
		Slug:                testAppName,
		ClientID:            testAppClientID,
		EnableSigninSession: true,
		UpdatedAt:           now,
	}
	for _, fn := range overrides {
		fn(app)
	}
	return app
}

func newTestUow(org *orgdomain.Organization, user *userdomain.User, app *applications.Application) loginTestUnitOfWork {
	orgRepo := &loginTestOrgRepository{orgs: map[string]*orgdomain.Organization{org.Slug: org}}
	userRepo := &loginTestUserRepository{
		users: map[string]*userdomain.User{user.ID: user},
		usersByOrg: map[string]map[string]*userdomain.User{
			org.ID: {user.Username: user},
		},
	}
	appRepo := &loginTestAppRepository{apps: map[string]*applications.Application{app.ID: app}}
	sessRepo := &loginTestSessionRepository{}

	return loginTestUnitOfWork{
		orgRepo:        orgRepo,
		userRepo:       userRepo,
		appRepo:        appRepo,
		sessRepo:       sessRepo,
		lockoutAttempt: &loginTestLockoutAttemptRepository{},
		challengeRepo:  &loginTestChallengeRepository{},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestPasswordLogin_Success(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatalf("Login() result = nil, want non-nil")
	}
	if result.SessionID != testSessID {
		t.Fatalf("Login() SessionID = %q, want %q", result.SessionID, testSessID)
	}
	if result.Response.Status != LoginStatusSuccess {
		t.Fatalf("Login() Response.Status = %q, want %q", result.Response.Status, LoginStatusSuccess)
	}
	if result.Response.Account == nil {
		t.Fatalf("Login() Response.Account = nil, want non-nil")
	}
	if result.Response.Account.Name != testUsername {
		t.Fatalf("Login() Response.Account.Name = %q, want %q", result.Response.Account.Name, testUsername)
	}
	// account.id is the stable user primary key used by clients for
	// requester ownership checks; it must never be the username.
	if result.Response.Account.ID != testUserID {
		t.Fatalf("Login() Response.Account.ID = %q, want %q", result.Response.Account.ID, testUserID)
	}

	// Verify session was created in the repository.
	sess, ok := uow.sessRepo.sessions[testSessID]
	if !ok {
		t.Fatalf("expected session with id %q to be created", testSessID)
	}
	if sess.UserID != testUserID {
		t.Fatalf("session UserID = %q, want %q", sess.UserID, testUserID)
	}
	if sess.OrganizationID != testOrgID {
		t.Fatalf("session OrganizationID = %q, want %q", sess.OrganizationID, testOrgID)
	}
	if sess.ApplicationID != testAppID {
		t.Fatalf("session ApplicationID = %q, want %q", sess.ApplicationID, testAppID)
	}
	if sess.SessionState != "active" {
		t.Fatalf("session SessionState = %q, want %q", sess.SessionState, "active")
	}
}

func TestPasswordLogin_WrongPassword(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         "WrongPassword!",
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error for wrong password")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil for wrong password", result)
	}

	// The Login method returns CodeUnauthorized for credential errors.
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

func TestPasswordLogin_UserNotFound(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	// Request a non-existent user.
	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         "nonexistent",
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error for non-existent user")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil for non-existent user", result)
	}

	// The Login method returns CodeUnauthorized to avoid user enumeration.
	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

func TestPasswordLogin_ForbiddenUser(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.IsForbidden = true
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error for forbidden user")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil for forbidden user", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

func TestPasswordLogin_DisabledSignin(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp(func(a *applications.Application) {
		a.DisableSignin = true
	})
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error when application has signin disabled")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil when application has signin disabled", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

// ---------------------------------------------------------------------------
// Additional login test scenarios
// ---------------------------------------------------------------------------

func TestPasswordLogin_AccountLocked(t *testing.T) {
	org := newTestOrg()
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)

	// Simulate an existing lockout entry for the user.
	now := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC)
	lockedUntil := now.Add(30 * time.Minute)
	uow.lockoutAttempt.attempts = map[string]*lockoutattempts.Attempt{
		testUserID: {
			ID:          "lockout-1",
			UserID:      testUserID,
			Attempts:    5,
			LockedUntil: &lockedUntil,
			CreatedAt:   now,
			UpdatedAt:   now,
		},
	}

	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error for locked account")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil for locked account", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
	if typed.Message != "invalid username or password" {
		t.Fatalf("Login() error message = %q, want the generic credential error", typed.Message)
	}
}

func TestPasswordLogin_MfaRequired(t *testing.T) {
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.Security = orgdomain.OrganizationSecurityPolicy{
			MfaPolicy:      map[string]any{"required": true},
			OrganizationID: testOrgID,
		}
	})
	user := newTestUser(func(u *userdomain.User) {
		u.MfaMethods = []userdomain.UserMfaMethod{
			{
				ID:         "mfa-totp-1",
				MethodType: "totp",
				Enabled:    true,
				Verified:   true,
			},
		}
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil for MFA required flow", err)
	}
	if result == nil {
		t.Fatalf("Login() result = nil, want non-nil")
	}
	if result.Response.Status != LoginStatusMfaRequired {
		t.Fatalf("Login() Response.Status = %q, want %q", result.Response.Status, LoginStatusMfaRequired)
	}
	if result.Response.ChallengeID == "" {
		t.Fatalf("Login() ChallengeID = empty, want non-empty challenge ID")
	}
	if len(result.Response.AvailableMfaMethods) == 0 {
		t.Fatalf("Login() AvailableMfaMethods = empty, want at least one method")
	}
}

func TestPasswordLogin_DisabledUser(t *testing.T) {
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Status = "suspended"
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error for disabled/suspended user")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil for disabled user", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

func TestPasswordLogin_PasswordExpired(t *testing.T) {
	// The max-age decision is reported on the login response only. Login must
	// never persist it into need_update_password, otherwise every later request
	// would hit the flag branch and password_expired would be unreachable.
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.Security = orgdomain.OrganizationSecurityPolicy{
			PasswordPolicy: map[string]any{
				"passwordMaxAgeDays": 30,
			},
			OrganizationID: testOrgID,
		}
	})
	passwordUpdatedAt := time.Date(2026, 5, 15, 10, 0, 0, 0, time.UTC).Add(-60 * 24 * time.Hour)
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.PasswordUpdatedAt = &passwordUpdatedAt
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if result.Response.PasswordExpired != true {
		t.Fatalf("login response PasswordExpired = %v, want true", result.Response.PasswordExpired)
	}
	if result.Response.NeedUpdatePassword {
		t.Fatalf("login response NeedUpdatePassword = true, want false for a max-age-only result")
	}
	if len(uow.userRepo.updatedCredentials) != 0 {
		t.Fatalf("UpdateCredentials calls = %d, want 0 (max-age must not be persisted)", len(uow.userRepo.updatedCredentials))
	}
	if user.Credentials.NeedUpdatePassword {
		t.Fatalf("in-memory credentials NeedUpdatePassword = true after max-age login, want false")
	}
}

func TestPasswordLogin_AdminForcedFlagNotRewritten(t *testing.T) {
	// An administrator-forced flag is already stored; login must report it but
	// must not rewrite the credential row.
	org := newTestOrg()
	user := newTestUser(func(u *userdomain.User) {
		u.Credentials.NeedUpdatePassword = true
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil", err)
	}
	if !result.Response.NeedUpdatePassword {
		t.Fatalf("login response NeedUpdatePassword = false, want true")
	}
	if !user.Credentials.NeedUpdatePassword {
		t.Fatalf("stored NeedUpdatePassword = false, want the admin flag unchanged")
	}
	if len(uow.userRepo.updatedCredentials) != 0 {
		t.Fatalf("UpdateCredentials calls = %d, want 0", len(uow.userRepo.updatedCredentials))
	}
}

func TestPasswordLogin_OrgDisabledSignin(t *testing.T) {
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.DisableSignin = true
	})
	user := newTestUser()
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err == nil {
		t.Fatalf("Login() error = nil, want non-nil error when organization has signin disabled")
	}
	if result != nil {
		t.Fatalf("Login() result = %+v, want nil when organization has signin disabled", result)
	}

	typed, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("Login() error type = %T, want *apperrors.Error", err)
	}
	if typed.Code != apperrors.CodeUnauthorized {
		t.Fatalf("Login() error code = %q, want %q", typed.Code, apperrors.CodeUnauthorized)
	}
}

func TestPasswordLogin_MfaSetupRequired(t *testing.T) {
	org := newTestOrg(func(o *orgdomain.Organization) {
		o.Security = orgdomain.OrganizationSecurityPolicy{
			MfaPolicy:      map[string]any{"required": true},
			OrganizationID: testOrgID,
		}
	})
	// User has no MFA methods configured.
	user := newTestUser(func(u *userdomain.User) {
		u.MfaMethods = nil
	})
	app := newTestApp()
	uow := newTestUow(org, user, app)
	svc := newTestService(uow)

	result, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() error = %v, want nil for MFA setup required flow", err)
	}
	if result == nil {
		t.Fatalf("Login() result = nil, want non-nil")
	}
	if result.Response.Status != LoginStatusMfaSetup {
		t.Fatalf("Login() Response.Status = %q, want %q", result.Response.Status, LoginStatusMfaSetup)
	}
	if result.Response.ChallengeID == "" {
		t.Fatalf("Login() ChallengeID = empty, want non-empty challenge ID")
	}
	if len(result.Response.RequiredMfa) == 0 {
		t.Fatalf("Login() RequiredMfa = empty, want at least one required MFA type")
	}
}

// TestMfaLoginSuccessAdvancesUserRevision guards the optimistic-lock contract of
// the MFA success path: the production repository guards on Revision-1, so the
// use case must bump the aggregate before Update. Without the bump every
// successful MFA login ended in a 409 revision conflict (found in L4).
func TestMfaLoginSuccessAdvancesUserRevision(t *testing.T) {
	const secret = "JBSWY3DPEHPK3PXP"
	user := newTestUser(func(u *userdomain.User) {
		u.Revision = 3
		u.TotpSecret = secret
		u.PreferredMfaType = mfaTypeTotp
		u.MfaMethods = []userdomain.UserMfaMethod{{
			ID:         "mfa-totp",
			UserID:     testUserID,
			MethodType: mfaTypeTotp,
			Enabled:    true,
			Verified:   true,
		}}
	})
	uow := newTestUow(newTestOrg(), user, newTestApp())
	uow.userRepo.revisionGuard = true
	svc := newTestService(uow)

	first, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		Password:         testPassword,
		ApplicationID:    testAppID,
	})
	if err != nil {
		t.Fatalf("Login() password step error = %v", err)
	}
	if first.Response.ChallengeID == "" {
		t.Fatalf("Login() password step ChallengeID = empty, want a pending MFA challenge")
	}

	code, err := totp.GenerateCode(secret, svc.Clock.Now())
	if err != nil {
		t.Fatalf("totp.GenerateCode() error = %v", err)
	}
	second, err := svc.Login(context.Background(), LoginCommand{
		OrganizationSlug: testOrgSlug,
		Username:         testUsername,
		ApplicationID:    testAppID,
		ChallengeID:      first.Response.ChallengeID,
		MfaType:          mfaTypeTotp,
		Passcode:         code,
	})
	if err != nil {
		t.Fatalf("Login() MFA step error = %v, want success", err)
	}
	if second.Response.Account == nil {
		t.Fatalf("Login() MFA step Account = nil, want authenticated account")
	}
	// Two aggregate writes happen on a successful MFA login: the MFA branch
	// (3 -> 4) and the post-login session bookkeeping (4 -> 5).
	if user.Revision != 5 {
		t.Fatalf("user.Revision = %d, want 5 (loaded 3, MFA update, post-login update)", user.Revision)
	}
}
