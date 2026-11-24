package tokens

import (
	"context"
	"encoding/json"
	"errors"
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
	domain "github.com/guliangiam/guliangiam/internal/domain/tokens"
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
}

func (g *fakeUUIDGenerator) NewString() (string, error) {
	g.callNum++
	return g.fixedID, nil
}

// ---------------------------------------------------------------------------
// Mock token repository
// ---------------------------------------------------------------------------

type mockTokenRepo struct {
	tokens        []*domain.Token
	refreshTokens []*domain.RefreshToken
	// revokedFamilies records RevokeByFamilyID arguments for reuse-detection assertions.
	revokedFamilies []string
}

func (r *mockTokenRepo) GetByID(ctx context.Context, id string) (*domain.Token, error) {
	for _, t := range r.tokens {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, apperrors.NewNotFound("token not found")
}

func (r *mockTokenRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Token, error) {
	for _, t := range r.tokens {
		if t.ID == id && t.OrganizationID == organizationID {
			return t, nil
		}
	}
	return nil, apperrors.NewNotFound("token not found")
}

func (r *mockTokenRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Token, error) {
	for _, t := range r.tokens {
		if t.TokenHash == tokenHash {
			return t, nil
		}
	}
	return nil, apperrors.NewNotFound("token not found")
}

func (r *mockTokenRepo) GetByDeviceSecretHash(ctx context.Context, deviceSecretHash string) (*domain.Token, error) {
	return nil, apperrors.NewNotFound("not found")
}

func (r *mockTokenRepo) GetRefreshByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshToken, error) {
	for _, rt := range r.refreshTokens {
		if rt.RefreshTokenHash == tokenHash {
			return rt, nil
		}
	}
	return nil, apperrors.NewNotFound("refresh token not found")
}

func (r *mockTokenRepo) List(ctx context.Context, query domain.ListQuery) ([]*domain.Token, int64, error) {
	return r.tokens, int64(len(r.tokens)), nil
}

func (r *mockTokenRepo) Create(ctx context.Context, token *domain.Token) error {
	r.tokens = append(r.tokens, token)
	return nil
}

func (r *mockTokenRepo) UpdateDeviceSecret(ctx context.Context, tokenID, deviceSecretHash string, expiresIn int) error {
	return nil
}
func (r *mockTokenRepo) CreateRefresh(ctx context.Context, token *domain.RefreshToken) error {
	r.refreshTokens = append(r.refreshTokens, token)
	return nil
}

func (r *mockTokenRepo) Revoke(ctx context.Context, id string, revokedAt time.Time) error {
	for _, t := range r.tokens {
		if t.ID == id {
			t.RevokedAt = &revokedAt
			return nil
		}
	}
	return apperrors.NewNotFound("token not found")
}

func (r *mockTokenRepo) RevokeRefresh(ctx context.Context, id string, revokedAt time.Time) error {
	for _, rt := range r.refreshTokens {
		if rt.ID == id {
			rt.RevokedAt = &revokedAt
			return nil
		}
	}
	return apperrors.NewNotFound("refresh token not found")
}

func (r *mockTokenRepo) RevokeByQuery(ctx context.Context, query domain.RevokeQuery, revokedAt time.Time) (int64, error) {
	var count int64
	for _, t := range r.tokens {
		if t.RevokedAt != nil {
			continue
		}
		match := true
		if query.OrganizationID != "" && t.OrganizationID != query.OrganizationID {
			match = false
		}
		if query.UserID != "" && t.UserID != query.UserID {
			match = false
		}
		if query.ApplicationID != "" && t.ApplicationID != query.ApplicationID {
			match = false
		}
		if match {
			t.RevokedAt = &revokedAt
			count++
		}
	}
	return count, nil
}

func (r *mockTokenRepo) RevokeByFamilyID(ctx context.Context, familyID string, revokedAt time.Time) (int64, error) {
	r.revokedFamilies = append(r.revokedFamilies, familyID)
	return 0, nil
}

func (r *mockTokenRepo) RevokeRefreshByUser(ctx context.Context, userID string, revokedAt time.Time) (int64, error) {
	var count int64
	for _, rt := range r.refreshTokens {
		if rt.UserID == userID && rt.RevokedAt == nil {
			rt.RevokedAt = &revokedAt
			count++
		}
	}
	return count, nil
}

func (r *mockTokenRepo) RevokeByAuthorizationCodeID(ctx context.Context, authorizationCodeID string, revokedAt time.Time) (int64, error) {
	return 0, nil
}

func (r *mockTokenRepo) Delete(ctx context.Context, id string) error {
	for i, t := range r.tokens {
		if t.ID == id {
			r.tokens = append(r.tokens[:i], r.tokens[i+1:]...)
			return nil
		}
	}
	return apperrors.NewNotFound("token not found")
}

// ---------------------------------------------------------------------------
// Mock org/user/app repos (needed for CreateToken/RefreshAccessToken)
// ---------------------------------------------------------------------------

type mockOrgRepo struct {
	orgs []*orgdomain.Organization
}

func (r *mockOrgRepo) GetByID(ctx context.Context, id string) (*orgdomain.Organization, error) {
	for _, o := range r.orgs {
		if o.ID == id {
			return o, nil
		}
	}
	return nil, apperrors.NewNotFound("organization not found")
}
func (r *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return nil, apperrors.NewNotFound("not found")
}

// GetBySlugPublic delegates to GetBySlug (tests don't need RLS bypass).
func (r *mockOrgRepo) GetBySlugPublic(ctx context.Context, slug string) (*orgdomain.Organization, error) {
	return r.GetBySlug(ctx, slug)
}

func (r *mockOrgRepo) List(ctx context.Context, query orgdomain.ListQuery) ([]*orgdomain.Organization, int64, error) {
	return nil, 0, nil
}
func (r *mockOrgRepo) Create(ctx context.Context, org *orgdomain.Organization) error { return nil }
func (r *mockOrgRepo) Update(ctx context.Context, org *orgdomain.Organization) error { return nil }
func (r *mockOrgRepo) UpdateSettings(ctx context.Context, settings *orgdomain.OrganizationSettings) error {
	return nil
}
func (r *mockOrgRepo) UpdateTheme(ctx context.Context, theme *orgdomain.OrganizationThemeSettings) error {
	return nil
}
func (r *mockOrgRepo) UpdateSecurityPolicy(ctx context.Context, policy *orgdomain.OrganizationSecurityPolicy) error {
	return nil
}

func (r *mockOrgRepo) Delete(_ context.Context, _ string) error {
	return apperrors.NewUnimplemented("not implemented")
}

func (r *mockOrgRepo) HasDeletedWithSlug(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type mockUserRepo struct {
	users []*userdomain.User
}

func (r *mockUserRepo) GetByID(ctx context.Context, id string) (*userdomain.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, apperrors.NewNotFound("user not found")
}

func (r *mockUserRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*userdomain.User, error) {
	user, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user.OrganizationID != organizationID {
		return nil, apperrors.NewNotFound("user not found")
	}
	return user, nil
}
func (r *mockUserRepo) GetByUsername(ctx context.Context, organizationID, username string) (*userdomain.User, error) {
	return nil, apperrors.NewNotFound("not found")
}

// GetByLoginIdentifier 仅是密码登录入口使用的读路径，令牌用例不经过登录，占位即可。
func (r *mockUserRepo) GetByLoginIdentifier(ctx context.Context, organizationID, identifier string) (*userdomain.User, error) {
	return nil, apperrors.NewNotFound("not found")
}

// LockByID 是会话记账用的行级锁；本测试的用例不经过并发会话策略，占位即可。
func (r *mockUserRepo) LockByID(context.Context, string) error { return nil }
func (r *mockUserRepo) List(ctx context.Context, query userdomain.ListQuery) ([]*userdomain.User, int64, error) {
	return nil, 0, nil
}
func (r *mockUserRepo) Create(ctx context.Context, user *userdomain.User) error { return nil }
func (r *mockUserRepo) Update(ctx context.Context, user *userdomain.User) error { return nil }

func (r *mockUserRepo) UpdateForOrganization(ctx context.Context, user *userdomain.User, organizationID string) error {
	existing, err := r.GetByID(ctx, user.ID)
	if err != nil {
		return err
	}
	if existing.OrganizationID != organizationID {
		return apperrors.NewNotFound("user not found")
	}
	for i, item := range r.users {
		if item.ID == user.ID {
			r.users[i] = user
			break
		}
	}
	return nil
}
func (r *mockUserRepo) UpdateProfile(ctx context.Context, profile *userdomain.UserProfile) error {
	return nil
}
func (r *mockUserRepo) SetContactVerifiedForOrganization(context.Context, string, string, bool, bool) error {
	return nil
}
func (r *mockUserRepo) UpdateCollectionsForOrganization(context.Context, string, string, userdomain.UserCollections) error {
	return nil
}
func (r *mockUserRepo) UpdateCredentials(ctx context.Context, credentials *userdomain.UserCredentials) error {
	return nil
}

func (r *mockUserRepo) UpdatePasswordCredential(ctx context.Context, credentials *userdomain.UserCredentials) error {
	return nil
}
func (r *mockUserRepo) ReplaceIdentities(ctx context.Context, userID string, identities []userdomain.UserIdentity) error {
	return nil
}
func (r *mockUserRepo) CreateIdentity(ctx context.Context, identity *userdomain.UserIdentity) error {
	return nil
}
func (r *mockUserRepo) DeleteIdentity(ctx context.Context, userID, identityID string) error {
	return nil
}
func (r *mockUserRepo) ReplaceMfaMethods(ctx context.Context, userID string, methods []userdomain.UserMfaMethod) error {
	return nil
}
func (r *mockUserRepo) ReplaceRoleBindings(ctx context.Context, userID string, bindings []userdomain.UserRoleBinding) error {
	return nil
}

func (r *mockUserRepo) ListRoleNames(ctx context.Context, userID string) ([]string, error) {
	return nil, nil
}
func (r *mockUserRepo) ReplaceGroupMemberships(ctx context.Context, userID string, memberships []userdomain.UserGroupMembership) error {
	return nil
}
func (r *mockUserRepo) Delete(ctx context.Context, id, organizationID string) error { return nil }
func (r *mockUserRepo) GetPasswordHistory(ctx context.Context, userID string, limit int) ([]userdomain.PasswordHistoryEntry, error) {
	return nil, nil
}
func (r *mockUserRepo) CreatePasswordHistory(ctx context.Context, entry *userdomain.PasswordHistoryEntry) error {
	return nil
}
func (r *mockUserRepo) DeletePasswordHistoryBeyond(ctx context.Context, userID string, keepCount int) error {
	return nil
}
func (r *mockUserRepo) GetByProviderIdentity(ctx context.Context, providerID, subject string) (*userdomain.User, error) {
	return nil, apperrors.NewNotFound("not found")
}
func (r *mockUserRepo) Count(ctx context.Context, organizationID string) (int64, error) {
	return int64(len(r.users)), nil
}
func (r *mockUserRepo) HasDeletedWithUsername(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

type mockAppRepo struct {
	apps []*applications.Application
}

func (r *mockAppRepo) GetByID(ctx context.Context, id string) (*applications.Application, error) {
	for _, a := range r.apps {
		if a.ID == id {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}
func (r *mockAppRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*applications.Application, error) {
	for _, a := range r.apps {
		if a.ID == id && a.OrganizationID == organizationID {
			return a, nil
		}
	}
	return nil, apperrors.NewNotFound("application not found")
}
func (r *mockAppRepo) GetByClientID(ctx context.Context, clientID string) (*applications.Application, error) {
	return nil, apperrors.NewNotFound("not found")
}
func (r *mockAppRepo) GetByOrganizationAndSlug(ctx context.Context, organizationID, slug string) (*applications.Application, error) {
	return nil, apperrors.NewNotFound("not found")
}
func (r *mockAppRepo) ReplaceProviderBindings(ctx context.Context, applicationID string, bindings []applications.ApplicationProviderBinding) error {
	return nil
}
func (r *mockAppRepo) List(ctx context.Context, query applications.ListQuery) ([]*applications.Application, int64, error) {
	return nil, 0, nil
}
func (r *mockAppRepo) Create(ctx context.Context, app *applications.Application) error { return nil }
func (r *mockAppRepo) Update(ctx context.Context, app *applications.Application) error { return nil }
func (r *mockAppRepo) Delete(ctx context.Context, id, organizationID string) error {
	return nil
}
func (r *mockAppRepo) HasDeletedWithOrgAndSlug(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}

func (r *mockAppRepo) FindHostConflicts(_ context.Context, _, _ string, _ []string) ([]applications.HostConflict, error) {
	return nil, nil
}

func (r *mockAppRepo) SetRegistrationAccessToken(_ context.Context, _, _ string) error {
	return nil
}

func (r *mockAppRepo) GetRegistrationAccessTokenHash(_ context.Context, _ string) (string, error) {
	return "", nil
}

// ---------------------------------------------------------------------------
// Test UOW
// ---------------------------------------------------------------------------

type mockRecordsRepo struct {
	records     []*recordsdomain.Record
	createCalls int
}

func (r *mockRecordsRepo) Create(_ context.Context, record *recordsdomain.Record) error {
	r.records = append(r.records, record)
	r.createCalls++
	return nil
}
func (r *mockRecordsRepo) List(context.Context, recordsdomain.ListQuery) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}
func (r *mockRecordsRepo) Search(context.Context, recordsdomain.SearchCriteria) ([]*recordsdomain.Record, int64, error) {
	return nil, 0, nil
}
func (r *mockRecordsRepo) LoadLastChainEntry(context.Context, string) (seq int64, hash []byte, err error) {
	return 0, nil, nil
}
func (r *mockRecordsRepo) LoadAnchorSeq(context.Context, string, string) (int64, error) {
	return 0, nil
}
func (r *mockRecordsRepo) InsertAnchor(context.Context, *recordsdomain.ChainAnchor) error {
	return nil
}
func (r *mockRecordsRepo) ListChainEntries(context.Context, string) ([]recordsdomain.ChainEntry, error) {
	return nil, nil
}

type mockOutboxRepo struct {
	events       []*domainoutbox.OutboxEvent
	enqueueCalls int
}

func (r *mockOutboxRepo) Enqueue(_ context.Context, event *domainoutbox.OutboxEvent) error {
	r.events = append(r.events, event)
	r.enqueueCalls++
	return nil
}
func (r *mockOutboxRepo) EnqueueBatch(context.Context, []*domainoutbox.OutboxEvent) error { return nil }

type testUnitOfWork struct {
	tokenRepo   *mockTokenRepo
	recordsRepo *mockRecordsRepo
	outboxRepo  *mockOutboxRepo
	orgRepo     *mockOrgRepo
	userRepo    *mockUserRepo
	appRepo     *mockAppRepo
}

func (u testUnitOfWork) Outbox() domainoutbox.Repository {
	if u.outboxRepo != nil {
		return u.outboxRepo
	}
	return nil
}

func (u testUnitOfWork) Applications() applications.Repository                        { return u.appRepo }
func (u testUnitOfWork) AuthorizationCodes() authorizationcodedomain.Repository       { return nil }
func (u testUnitOfWork) Consents() consentdomain.Repository                           { return nil }
func (u testUnitOfWork) Gateway() gatewaydomain.Repository                            { return nil }
func (u testUnitOfWork) Groups() groupsdomain.Repository                              { return nil }
func (u testUnitOfWork) Invitations() invitationsdomain.Repository                    { return nil }
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
func (u testUnitOfWork) Records() recordsdomain.Repository {
	if u.recordsRepo != nil {
		return u.recordsRepo
	}
	return nil
}
func (u testUnitOfWork) Resources() resourcedomain.Repository                 { return nil }
func (u testUnitOfWork) Roles() roledomain.Repository                         { return nil }
func (u testUnitOfWork) Syncers() syncerdomain.Repository                     { return nil }
func (u testUnitOfWork) Sessions() sessiondomain.Repository                   { return nil }
func (u testUnitOfWork) Subscriptions() subscriptiondomain.Repository         { return nil }
func (u testUnitOfWork) Tickets() ticketdomain.Repository                     { return nil }
func (u testUnitOfWork) Tokens() domain.Repository                            { return u.tokenRepo }
func (u testUnitOfWork) Webhooks() webhookdomain.Repository                   { return nil }
func (u testUnitOfWork) Users() userdomain.Repository                         { return u.userRepo }
func (u testUnitOfWork) Coupons() coupondomain.Repository                     { return nil }
func (u testUnitOfWork) CouponUsages() coupondomain.CouponUsageRepository     { return nil }
func (u testUnitOfWork) Entries() entrydomain.Repository                      { return nil }
func (u testUnitOfWork) McpServers() mcpserverdomain.Repository               { return nil }
func (u testUnitOfWork) RadiusAccountings() radiusaccountingdomain.Repository { return nil }
func (u testUnitOfWork) Verifications() verificationsdomain.Repository        { return nil }
func (u testUnitOfWork) WebAuthnCredentials() webauthndomain.Repository       { return nil }
func (u testUnitOfWork) FaceIdProviders() faceidproviderdomain.Repository     { return nil }
func (u testUnitOfWork) RadiusConfigs() radiusconfigdomain.Repository         { return nil }
func (u testUnitOfWork) Rules() ruledomain.Repository                         { return nil }
func (u testUnitOfWork) Transactions() transactiondomain.Repository           { return nil }
func (u testUnitOfWork) WebhookEvents() webhookeventdomain.Repository         { return nil }
func (u testUnitOfWork) Keys() keysdomain.Repository                          { return nil }
func (u testUnitOfWork) QRCodeSessions() qrcodesessions.Repository            { return nil }
func (u testUnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository          { return nil }

func (u testUnitOfWork) Web3Identities() web3identity.Repository     { return nil }
func (u testUnitOfWork) FaceIDSessions() faceidsessions.Repository   { return nil }
func (u testUnitOfWork) CASTickets() castickets.Repository           { return nil }
func (u testUnitOfWork) KerberosTickets() kerberostickets.Repository { return nil }
func (u testUnitOfWork) LockoutAttempts() lockoutattempts.Repository { return nil }

func (u testUnitOfWork) Rebac() rebac.Repository     { return nil }
func (u testUnitOfWork) Risk() riskdomain.Repository { return nil }

func (u testUnitOfWork) IDV() idvdomain.Repository { return nil }

func (u testUnitOfWork) Actions() actionsdomain.Repository { return nil }

func (u testUnitOfWork) Workload() workloaddomain.Repository { return nil }

func (u testUnitOfWork) Metering() meteringdomain.Repository { return nil }

func (u testUnitOfWork) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}
func (u testUnitOfWork) IGA() igadomain.Repository { return nil }

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestService_ValidateCreateTokenCommand(t *testing.T) {
	tests := []struct {
		name    string
		command CreateTokenCommand
		wantErr bool
	}{
		{
			name: "valid command",
			command: CreateTokenCommand{
				OrganizationID:   "org-1",
				UserID:           "user-1",
				ApplicationID:    "app-1",
				Scopes:           []string{"read", "write"},
				ExpiresInSeconds: 3600,
			},
			wantErr: false,
		},
		{
			name: "missing organizationId",
			command: CreateTokenCommand{
				UserID:           "user-1",
				ApplicationID:    "app-1",
				Scopes:           []string{"read"},
				ExpiresInSeconds: 3600,
			},
			wantErr: true,
		},
		{
			name: "missing userId",
			command: CreateTokenCommand{
				OrganizationID:   "org-1",
				ApplicationID:    "app-1",
				Scopes:           []string{"read"},
				ExpiresInSeconds: 3600,
			},
			wantErr: true,
		},
		{
			name: "missing applicationId",
			command: CreateTokenCommand{
				OrganizationID:   "org-1",
				UserID:           "user-1",
				Scopes:           []string{"read"},
				ExpiresInSeconds: 3600,
			},
			wantErr: true,
		},
		{
			name: "expiresInSeconds zero",
			command: CreateTokenCommand{
				OrganizationID:   "org-1",
				UserID:           "user-1",
				ApplicationID:    "app-1",
				Scopes:           []string{"read"},
				ExpiresInSeconds: 0,
			},
			wantErr: true,
		},
		{
			name: "empty scopes",
			command: CreateTokenCommand{
				OrganizationID:   "org-1",
				UserID:           "user-1",
				ApplicationID:    "app-1",
				Scopes:           []string{},
				ExpiresInSeconds: 3600,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateTokenCommand(tt.command)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateCreateTokenCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_RevokeToken(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{ID: "tok-1", UserID: "user-1", OrganizationID: "org-1"},
		},
	}
	recordsRepo := &mockRecordsRepo{}
	outboxRepo := &mockOutboxRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{
		tokenRepo:   repo,
		recordsRepo: recordsRepo,
		outboxRepo:  outboxRepo,
	}}

	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{ID: "tok-1", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("RevokeToken() error = %v", err)
	}
	if repo.tokens[0].RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}

	// § 审计同事务
	if recordsRepo.createCalls != 1 {
		t.Errorf("audit Records.Create calls = %d, want 1", recordsRepo.createCalls)
	}
	if len(recordsRepo.records) > 0 && recordsRepo.records[0].Action != "token.revoked" {
		t.Errorf("audit Action = %q, want %q", recordsRepo.records[0].Action, "token.revoked")
	}
	if outboxRepo.enqueueCalls != 1 {
		t.Errorf("outbox Enqueue calls = %d, want 1", outboxRepo.enqueueCalls)
	}
	if len(outboxRepo.events) > 0 && outboxRepo.events[0].EventType != "token.revoked" {
		t.Errorf("outbox EventType = %q, want %q", outboxRepo.events[0].EventType, "token.revoked")
	}
}

func TestService_RevokeToken_EmptyID(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{}}

	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{ID: "", OrganizationID: "org-1"})
	if err == nil {
		t.Error("expected error for empty token ID, got nil")
	}
}

func requireAppErrorCode(t *testing.T, err error, code apperrors.Code) {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %T", err)
	}
	if appErr.Code != code {
		t.Errorf("error code = %q, want %q", appErr.Code, code)
	}
}

func TestService_RevokeToken_EmptyOrganization(t *testing.T) {
	repo := &mockTokenRepo{
		tokens: []*domain.Token{{ID: "tok-1", OrganizationID: "org-1"}},
	}
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{tokenRepo: repo}}

	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{ID: "tok-1"})
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
}

func TestService_RevokeToken_ForeignOrganizationNotFound(t *testing.T) {
	repo := &mockTokenRepo{
		tokens: []*domain.Token{{ID: "tok-1", OrganizationID: "org-2"}},
	}
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{tokenRepo: repo}}

	err := svc.RevokeToken(context.Background(), RevokeTokenCommand{ID: "tok-1", OrganizationID: "org-1"})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
	if repo.tokens[0].RevokedAt != nil {
		t.Error("foreign token must not be revoked")
	}
}

func TestService_RevokeTokens(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{ID: "tok-1", UserID: "user-1", OrganizationID: "org-1", ApplicationID: "app-1"},
			{ID: "tok-2", UserID: "user-1", OrganizationID: "org-1", ApplicationID: "app-1"},
			{ID: "tok-3", UserID: "user-2", OrganizationID: "org-1", ApplicationID: "app-1"},
		},
	}
	recordsRepo := &mockRecordsRepo{}
	outboxRepo := &mockOutboxRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{
		tokenRepo:   repo,
		recordsRepo: recordsRepo,
		outboxRepo:  outboxRepo,
	}}

	result, err := svc.RevokeTokens(context.Background(), RevokeTokensCommand{
		OrganizationID: "org-1",
		UserID:         "user-1",
	})
	if err != nil {
		t.Fatalf("RevokeTokens() error = %v", err)
	}
	if result.RevokedCount != 2 {
		t.Errorf("RevokedCount = %d, want 2", result.RevokedCount)
	}

	// § 审计同事务
	if recordsRepo.createCalls != 1 {
		t.Errorf("audit Records.Create calls = %d, want 1", recordsRepo.createCalls)
	}
	if len(recordsRepo.records) > 0 && recordsRepo.records[0].Action != "tokens.revoked.batch" {
		t.Errorf("audit Action = %q, want %q", recordsRepo.records[0].Action, "tokens.revoked.batch")
	}
	if outboxRepo.enqueueCalls != 1 {
		t.Errorf("outbox Enqueue calls = %d, want 1", outboxRepo.enqueueCalls)
	}
	if len(outboxRepo.events) > 0 && outboxRepo.events[0].EventType != "tokens.revoked.batch" {
		t.Errorf("outbox EventType = %q, want %q", outboxRepo.events[0].EventType, "tokens.revoked.batch")
	}
}

func TestService_RevokeTokens_NoFilter(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{}}

	_, err := svc.RevokeTokens(context.Background(), RevokeTokensCommand{})
	if err == nil {
		t.Error("expected error when no filter provided, got nil")
	}
}

func TestService_IntrospectToken_ActiveToken(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_pat_test123")
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{
				ID:        "tok-1",
				UserID:    "user-1",
				Username:  "testuser",
				TokenHash: tokenHash,
				Scopes:    []string{"read", "write"},
				Audience:  []string{"app-1"},
				ExpiresAt: now.Add(1 * time.Hour),
				RevokedAt: nil,
				CreatedAt: now.Add(-1 * time.Hour),
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{tokenRepo: repo}}

	result, err := svc.IntrospectToken(context.Background(), IntrospectTokenCommand{
		Token: "gl_pat_test123",
	})
	if err != nil {
		t.Fatalf("IntrospectToken() error = %v", err)
	}
	if !result.Active {
		t.Error("expected Active = true")
	}
	if result.TokenType != "access_token" {
		t.Errorf("TokenType = %q, want %q", result.TokenType, "access_token")
	}
	if result.Username != "testuser" {
		t.Errorf("Username = %q, want %q", result.Username, "testuser")
	}
	if result.Scope != "read write" {
		t.Errorf("Scope = %q, want %q", result.Scope, "read write")
	}
}

func TestService_IntrospectToken_ExpiredToken(t *testing.T) {
	now := time.Now()
	tokenHash := hashTokenValue("gl_pat_expired")
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{
				ID:        "tok-1",
				TokenHash: tokenHash,
				ExpiresAt: now.Add(-1 * time.Hour),
				RevokedAt: nil,
				CreatedAt: now.Add(-2 * time.Hour),
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{tokenRepo: repo}}

	result, err := svc.IntrospectToken(context.Background(), IntrospectTokenCommand{
		Token: "gl_pat_expired",
	})
	if err != nil {
		t.Fatalf("IntrospectToken() error = %v", err)
	}
	if result.Active {
		t.Error("expected Active = false for expired token")
	}
}

func TestService_IntrospectToken_RevokedToken(t *testing.T) {
	now := time.Now()
	revokedAt := now.Add(-1 * time.Hour)
	tokenHash := hashTokenValue("gl_pat_revoked")
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{
				ID:        "tok-1",
				TokenHash: tokenHash,
				ExpiresAt: now.Add(1 * time.Hour),
				RevokedAt: &revokedAt,
				CreatedAt: now.Add(-2 * time.Hour),
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{tokenRepo: repo}}

	result, err := svc.IntrospectToken(context.Background(), IntrospectTokenCommand{
		Token: "gl_pat_revoked",
	})
	if err != nil {
		t.Fatalf("IntrospectToken() error = %v", err)
	}
	if result.Active {
		t.Error("expected Active = false for revoked token")
	}
}

func TestService_IntrospectToken_EmptyToken(t *testing.T) {
	svc := Service{Clock: fakeClock{}, UOW: testUnitOfWork{}}

	_, err := svc.IntrospectToken(context.Background(), IntrospectTokenCommand{
		Token: "",
	})
	if err == nil {
		t.Error("expected error for empty token, got nil")
	}
}

func TestService_IntrospectToken_NotFound(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{tokenRepo: repo}}

	result, err := svc.IntrospectToken(context.Background(), IntrospectTokenCommand{
		Token: "gl_pat_nonexistent",
	})
	if err != nil {
		t.Fatalf("IntrospectToken() error = %v", err)
	}
	if result.Active {
		t.Error("expected Active = false for not-found token")
	}
}

func TestService_IntrospectToken_RefreshToken(t *testing.T) {
	now := time.Now()
	refreshTokenHash := hashTokenValue("gl_prt_refresh1")
	repo := &mockTokenRepo{
		refreshTokens: []*domain.RefreshToken{
			{
				ID:               "rt-1",
				UserID:           "user-1",
				Username:         "testuser",
				RefreshTokenHash: refreshTokenHash,
				Scopes:           []string{"read"},
				Audience:         []string{"app-1"},
				ExpiresAt:        now.Add(1 * time.Hour),
				RevokedAt:        nil,
				CreatedAt:        now.Add(-1 * time.Hour),
			},
		},
	}
	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{tokenRepo: repo}}

	result, err := svc.IntrospectToken(context.Background(), IntrospectTokenCommand{
		Token:         "gl_prt_refresh1",
		TokenTypeHint: "refresh_token",
	})
	if err != nil {
		t.Fatalf("IntrospectToken() error = %v", err)
	}
	if !result.Active {
		t.Error("expected Active = true for valid refresh token")
	}
	if result.TokenType != "refresh_token" {
		t.Errorf("TokenType = %q, want %q", result.TokenType, "refresh_token")
	}
}

func TestService_GetToken(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{ID: "tok-1", UserID: "user-1", Username: "testuser", OrganizationID: "org-1", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{tokenRepo: repo}}

	detail, err := svc.GetToken(context.Background(), GetTokenQuery{ID: "tok-1", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}
	if detail.ID != "tok-1" {
		t.Errorf("ID = %q, want %q", detail.ID, "tok-1")
	}
}

func TestService_DeleteToken(t *testing.T) {
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{ID: "tok-1", UserID: "user-1", OrganizationID: "org-1"},
		},
	}
	svc := Service{UOW: testUnitOfWork{tokenRepo: repo}}

	err := svc.DeleteToken(context.Background(), DeleteTokenCommand{ID: "tok-1", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("DeleteToken() error = %v", err)
	}
	if len(repo.tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(repo.tokens))
	}
}

func TestService_DeleteToken_EmptyOrganization(t *testing.T) {
	repo := &mockTokenRepo{
		tokens: []*domain.Token{{ID: "tok-1", OrganizationID: "org-1"}},
	}
	svc := Service{UOW: testUnitOfWork{tokenRepo: repo}}

	err := svc.DeleteToken(context.Background(), DeleteTokenCommand{ID: "tok-1"})
	requireAppErrorCode(t, err, apperrors.CodeInvalidArgument)
	if len(repo.tokens) != 1 {
		t.Errorf("token must not be deleted without a trusted tenant")
	}
}

func TestService_GetToken_ForeignOrganizationNotFound(t *testing.T) {
	repo := &mockTokenRepo{
		tokens: []*domain.Token{{ID: "tok-1", OrganizationID: "org-2"}},
	}
	svc := Service{UOW: testUnitOfWork{tokenRepo: repo}}

	_, err := svc.GetToken(context.Background(), GetTokenQuery{ID: "tok-1", OrganizationID: "org-1"})
	requireAppErrorCode(t, err, apperrors.CodeNotFound)
}

func TestService_ListTokens(t *testing.T) {
	now := time.Now()
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{ID: "tok-1", UserID: "user-1", OrganizationID: "org-1", CreatedAt: now, UpdatedAt: now},
			{ID: "tok-2", UserID: "user-2", OrganizationID: "org-1", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{tokenRepo: repo}}

	items, total, err := svc.ListTokens(context.Background(), ListTokensQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListTokens() error = %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("len(items) = %d, want 2", len(items))
	}
}

// BE-D-19: 列表/详情读接口不得回显原始 tokenHash（无盐 SHA-256 + createdAt
// 构成离线猜解 oracle），只暴露 tokenHashSet + tokenHashMasked。
func TestService_ListTokens_MasksTokenHash(t *testing.T) {
	now := time.Now()
	rawHash := hashTokenValue("gl_pat_masking_secret")
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{ID: "tok-1", UserID: "user-1", OrganizationID: "org-1", TokenHash: rawHash, CreatedAt: now, UpdatedAt: now},
			{ID: "tok-2", UserID: "user-2", OrganizationID: "org-1", CreatedAt: now, UpdatedAt: now},
		},
	}
	svc := Service{UOW: testUnitOfWork{tokenRepo: repo}}

	items, _, err := svc.ListTokens(context.Background(), ListTokensQuery{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListTokens() error = %v", err)
	}

	body, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(body), rawHash) {
		t.Fatalf("list response must not contain the raw token hash")
	}
	if strings.Contains(string(body), `"tokenHash"`) {
		t.Fatalf("list response must not expose a tokenHash field")
	}
	if !items[0].TokenHashSet || items[0].TokenHashMasked != "******" {
		t.Errorf("item with hash: set = %v, masked = %q; want true, ******", items[0].TokenHashSet, items[0].TokenHashMasked)
	}
	if items[1].TokenHashSet || items[1].TokenHashMasked != "" {
		t.Errorf("item without hash: set = %v, masked = %q; want false, empty", items[1].TokenHashSet, items[1].TokenHashMasked)
	}
	if !strings.Contains(string(body), `"tokenHashSet":true`) || !strings.Contains(string(body), `"tokenHashMasked":"******"`) {
		t.Errorf("list response must carry tokenHashSet/tokenHashMasked, got %s", body)
	}
}

func TestService_GetToken_MasksTokenHash(t *testing.T) {
	rawHash := hashTokenValue("gl_pat_detail_secret")
	repo := &mockTokenRepo{
		tokens: []*domain.Token{
			{ID: "tok-1", UserID: "user-1", OrganizationID: "org-1", TokenHash: rawHash},
		},
	}
	svc := Service{UOW: testUnitOfWork{tokenRepo: repo}}

	detail, err := svc.GetToken(context.Background(), GetTokenQuery{ID: "tok-1", OrganizationID: "org-1"})
	if err != nil {
		t.Fatalf("GetToken() error = %v", err)
	}

	body, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(body), rawHash) || strings.Contains(string(body), `"tokenHash"`) {
		t.Fatalf("detail response must not expose the raw token hash, got %s", body)
	}
	if !detail.TokenHashSet || detail.TokenHashMasked != "******" {
		t.Errorf("detail: set = %v, masked = %q; want true, ******", detail.TokenHashSet, detail.TokenHashMasked)
	}
}

func Test_normalizeStringList(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		output []string
	}{
		{
			name:   "empty input",
			input:  []string{},
			output: []string{},
		},
		{
			name:   "trims whitespace",
			input:  []string{"  read  ", " write "},
			output: []string{"read", "write"},
		},
		{
			name:   "removes empty",
			input:  []string{"read", "", "  ", "write"},
			output: []string{"read", "write"},
		},
		{
			name:   "deduplicates",
			input:  []string{"read", "read", "write"},
			output: []string{"read", "write"},
		},
		{
			name:   "nil input",
			input:  nil,
			output: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeStringList(tt.input)
			if len(result) != len(tt.output) {
				t.Errorf("len = %d, want %d", len(result), len(tt.output))
				return
			}
			for i := range result {
				if result[i] != tt.output[i] {
					t.Errorf("item[%d] = %q, want %q", i, result[i], tt.output[i])
				}
			}
		})
	}
}

func Test_hashTokenValue(t *testing.T) {
	hash1 := hashTokenValue("test-token")
	hash2 := hashTokenValue("test-token")
	hash3 := hashTokenValue("different-token")

	if hash1 != hash2 {
		t.Error("same input should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("different inputs should produce different hashes")
	}
	if len(hash1) != 64 {
		t.Errorf("SHA-256 hex length = %d, want 64", len(hash1))
	}
}
