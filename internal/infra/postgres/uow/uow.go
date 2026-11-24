package uow

import (
	"context"
	customattributes "github.com/guliangiam/guliangiam/internal/domain/customattributes"
	customattributerepo "github.com/guliangiam/guliangiam/internal/infra/postgres/customattributes"
	"log/slog"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
	"github.com/guliangiam/guliangiam/internal/domain/adapters"
	"github.com/guliangiam/guliangiam/internal/domain/agents"
	aidomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	"github.com/guliangiam/guliangiam/internal/domain/b2b"
	"github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/domain/castickets"
	"github.com/guliangiam/guliangiam/internal/domain/certs"
	"github.com/guliangiam/guliangiam/internal/domain/consents"
	"github.com/guliangiam/guliangiam/internal/domain/coupons"
	crosstenantops "github.com/guliangiam/guliangiam/internal/domain/cross_tenant_operations"
	"github.com/guliangiam/guliangiam/internal/domain/deviceauthcodes"
	"github.com/guliangiam/guliangiam/internal/domain/enforcers"
	"github.com/guliangiam/guliangiam/internal/domain/entries"
	"github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/domain/faceidsessions"
	"github.com/guliangiam/guliangiam/internal/domain/forms"
	"github.com/guliangiam/guliangiam/internal/domain/gateway"
	"github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/guliangiam/guliangiam/internal/domain/idv"
	igadomain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	"github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	"github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	meteringdomain "github.com/guliangiam/guliangiam/internal/domain/metering"
	"github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	"github.com/guliangiam/guliangiam/internal/domain/models"
	"github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/domain/notifications"
	"github.com/guliangiam/guliangiam/internal/domain/orders"
	"github.com/guliangiam/guliangiam/internal/domain/organizations"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	"github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/domain/plans"
	platformadmins "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	"github.com/guliangiam/guliangiam/internal/domain/pricings"
	privacyDomain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	"github.com/guliangiam/guliangiam/internal/domain/products"
	"github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	"github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	"github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	"github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/domain/records"
	remembermedomain "github.com/guliangiam/guliangiam/internal/domain/rememberme"
	"github.com/guliangiam/guliangiam/internal/domain/resources"
	"github.com/guliangiam/guliangiam/internal/domain/risk"
	"github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/guliangiam/guliangiam/internal/domain/rules"
	"github.com/guliangiam/guliangiam/internal/domain/sessions"
	"github.com/guliangiam/guliangiam/internal/domain/subscriptions"
	"github.com/guliangiam/guliangiam/internal/domain/syncers"
	"github.com/guliangiam/guliangiam/internal/domain/tenants"
	"github.com/guliangiam/guliangiam/internal/domain/tickets"
	"github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/domain/transactions"
	trusteddevices "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	"github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/domain/web3identity"
	"github.com/guliangiam/guliangiam/internal/domain/webauthn"
	"github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	"github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	cacheinfra "github.com/guliangiam/guliangiam/internal/infra/cache"
	pgtenant "github.com/guliangiam/guliangiam/internal/infra/postgres"
	actionsrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/actions"
	adapterrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/adapters"
	agentrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/agents"
	aigatewayrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/aigateway"
	applicationrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/applications"
	authorizationcoderepo "github.com/guliangiam/guliangiam/internal/infra/postgres/authorizationcodes"
	b2brepo "github.com/guliangiam/guliangiam/internal/infra/postgres/b2b"
	captchaproviderrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/captchaproviders"
	casticketrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/castickets"
	certrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/certs"
	consentrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/consents"
	couponrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/coupons"
	crosstenantopsrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/cross_tenant_operations"
	deviceauthcoderepo "github.com/guliangiam/guliangiam/internal/infra/postgres/deviceauthcodes"
	enforcerrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/enforcers"
	entryrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/entries"
	faceidproviderrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/faceidproviders"
	faceidsessionrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/faceidsessions"
	facetemplaterepo "github.com/guliangiam/guliangiam/internal/infra/postgres/facetemplates"
	formrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/forms"
	gatewayrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/gateway"
	groupsrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/groups"
	idvrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/idv"
	igarepo "github.com/guliangiam/guliangiam/internal/infra/postgres/iga"
	invitationrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/invitations"
	kerberosticketrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/kerberostickets"
	keyrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/keys"
	ldaprepo "github.com/guliangiam/guliangiam/internal/infra/postgres/ldaps"
	lockoutattemptrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/lockoutattempts"
	mcpserverrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/mcpservers"
	meteringrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/metering"
	mfachallengerepo "github.com/guliangiam/guliangiam/internal/infra/postgres/mfachallenges"
	modelrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/models"
	notificationproviderrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/notificationproviders"
	notificationrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/notifications"
	orderrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/orders"
	organizationrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/organizations"
	outboxrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/outbox"
	paymentrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/payments"
	permissionrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/permissions"
	planrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/plans"
	platformadminrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/platform_admins"
	pricingrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/pricings"
	privacyRepo "github.com/guliangiam/guliangiam/internal/infra/postgres/privacy"
	productrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/products"
	providerrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/providers"
	qrcodesessionrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/qrcodesessions"
	radiusaccountingrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/radiusaccountings"
	radiusconfigrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/radiusconfigs"
	rebacrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/rebac"
	recordsrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/records"
	remembermerepo "github.com/guliangiam/guliangiam/internal/infra/postgres/rememberme"
	resourcerepo "github.com/guliangiam/guliangiam/internal/infra/postgres/resources"
	riskrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/risk"
	rolerepo "github.com/guliangiam/guliangiam/internal/infra/postgres/roles"
	rulerepo "github.com/guliangiam/guliangiam/internal/infra/postgres/rules"
	sessionrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/sessions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	subscriptionrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/subscriptions"
	syncerrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/syncers"
	tenantsrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/tenants"
	ticketrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/tickets"
	tokenrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/tokens"
	transactionrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/transactions"
	trusteddevicerepo "github.com/guliangiam/guliangiam/internal/infra/postgres/trusteddevices"
	usernotificationrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/usernotifications"
	userrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/users"
	verificationrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/verifications"
	web3identityrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/web3identity"
	webauthnrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/webauthn"
	webhookeventrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/webhookevents"
	webhookrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/webhooks"
	workloadrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/workload"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type beginner interface {
	shared.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

type UnitOfWork struct {
	db   beginner
	pool *pgxpool.Pool // optional — for Repositories that need pool.Acquire (RLS bypass)
}

// New constructs a UOW bound to db. An optional *pgxpool.Pool may be passed
// for Repositories that need pool-level RLS-bypass queries. Variadic so
// existing call sites compile unchanged.
func New(db beginner, pools ...*pgxpool.Pool) UnitOfWork {
	u := UnitOfWork{db: db}
	if len(pools) > 0 {
		u.pool = pools[0]
	}
	return u
}

// HasPublicPool reports whether this unit of work carries the pool used by
// public-discovery repositories (by-slug/resolve). It exists so regressions in
// transaction-handle propagation are observable.
func (u UnitOfWork) HasPublicPool() bool { return u.pool != nil }

func (u UnitOfWork) CustomAttributeDefinitions() customattributes.Repository {
	return customattributerepo.NewRepository(u.db)
}

func (u UnitOfWork) Applications() applications.Repository {
	return applicationrepo.NewRepository(u.db)
}

func (u UnitOfWork) AuthorizationCodes() authorizationcodes.Repository {
	return authorizationcoderepo.NewRepository(u.db)
}

func (u UnitOfWork) Certs() certs.Repository {
	return certrepo.NewRepository(u.db)
}

func (u UnitOfWork) Adapters() adapters.Repository {
	return adapterrepo.NewRepository(u.db)
}

func (u UnitOfWork) Agents() agents.Repository {
	return agentrepo.NewRepository(u.db)
}

func (u UnitOfWork) Enforcers() enforcers.Repository {
	return enforcerrepo.NewRepository(u.db)
}

func (u UnitOfWork) Forms() forms.Repository {
	return formrepo.NewRepository(u.db)
}

func (u UnitOfWork) Ldaps() ldaps.Repository {
	return ldaprepo.NewRepository(u.db)
}

func (u UnitOfWork) Models() models.Repository {
	return modelrepo.NewRepository(u.db)
}

// AIProviders exposes the AI-gateway provider aggregate so the unified
// provider service can read/write FamilyAI registrations in the same tx.
func (u UnitOfWork) AIProviders() aidomain.ProvidersRepo {
	return aigatewayrepo.NewProvidersRepository(u.db)
}

func (u UnitOfWork) Orders() orders.Repository {
	return orderrepo.NewRepository(u.db)
}

func (u UnitOfWork) Payments() payments.Repository {
	return paymentrepo.NewRepository(u.db)
}

// Refunds 暴露 tx 绑定的退款仓储（BE-C-09 退款编排接线）。具体类型附带
// LockPaymentForRefund（SELECT FOR UPDATE），bootstrap 经能力断言取用。
func (u UnitOfWork) Refunds() payments.RefundsRepo {
	return paymentrepo.NewRefundsRepository(u.db)
}

func (u UnitOfWork) Plans() plans.Repository {
	return planrepo.NewRepository(u.db)
}

func (u UnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository {
	return platformadminrepo.NewRepository(u.db)
}

func (u UnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return platformadminrepo.NewScopeRepository(u.db)
}

func (u UnitOfWork) CrossTenantOps() crosstenantops.TicketRepository {
	return crosstenantopsrepo.NewRepository(u.db)
}

func (u UnitOfWork) Pricings() pricings.Repository {
	return pricingrepo.NewRepository(u.db)
}

func (u UnitOfWork) Products() products.Repository {
	return productrepo.NewRepository(u.db)
}

func (u UnitOfWork) Consents() consents.Repository {
	return consentrepo.NewRepository(u.db)
}

func (u UnitOfWork) Gateway() gateway.Repository {
	return gatewayrepo.NewRepository(u.db)
}

func (u UnitOfWork) Groups() groups.Repository {
	return groupsrepo.NewRepository(u.db)
}

func (u UnitOfWork) Invitations() invitations.Repository {
	return invitationrepo.NewRepository(u.db)
}

func (u UnitOfWork) MfaChallenges() mfachallenges.Repository {
	return mfachallengerepo.NewRepository(u.db)
}

func (u UnitOfWork) Organizations() organizations.Repository {
	return organizationrepo.NewRepository(u.db, u.pool)
}

// Tenants returns the tenant identity + topology provisioning port bound to
// the current transaction, so CreateOrganization can write both rows atomically.
func (u UnitOfWork) Tenants() tenants.Repository {
	return tenantsrepo.NewRepository(u.db)
}

func (u UnitOfWork) Permissions() permissions.Repository {
	return permissionrepo.NewRepository(u.db)
}

func (u UnitOfWork) Providers() providers.Repository {
	return providerrepo.NewRepository(u.db)
}

func (u UnitOfWork) Roles() roles.Repository {
	return rolerepo.NewRepository(u.db)
}

func (u UnitOfWork) Syncers() syncers.Repository {
	return syncerrepo.NewRepository(u.db)
}

func (u UnitOfWork) Sessions() sessions.Repository {
	raw := sessionrepo.NewRepository(u.db)
	// Process-level optional cache — set via cache.Factory() in bootstrap.
	// If none is registered, returns raw postgres repo (zero overhead).
	if cache := sessionCache(); cache != nil {
		return sessionrepo.NewCachedRepository(raw, cache)
	}
	return raw
}

// CachedSession implements appuow.SessionCacheUoW. The authentication read path
// uses it to serve a warm session without opening a transaction: RLS is only
// needed on the cache-miss fallback, which still goes through withinTransaction.
// It mirrors CachedRepository's freshness check (not expired, not revoked) and
// never mutates the shared cached pointer.
func (u UnitOfWork) CachedSession(ctx context.Context, id string) (*sessions.Session, bool) {
	cache := sessionCache()
	if cache == nil || id == "" {
		return nil, false
	}
	cached, ok := cache.Get(ctx, id)
	if !ok || cached == nil {
		return nil, false
	}
	if cached.IsExpired(time.Now()) || cached.IsRevoked() {
		return nil, false
	}
	return cached, true
}

// sessionCache returns the registered process-wide session cache, or nil.
// Set via SetSessionCache() at bootstrap.
var (
	sessionCacheMu sync.RWMutex
	sessionCache_  cacheinfra.SessionCache
)

// SetSessionCache registers a process-level session cache.
func SetSessionCache(c cacheinfra.SessionCache) {
	sessionCacheMu.Lock()
	defer sessionCacheMu.Unlock()
	sessionCache_ = c
}

func sessionCache() cacheinfra.SessionCache {
	sessionCacheMu.RLock()
	defer sessionCacheMu.RUnlock()
	return sessionCache_
}

func (u UnitOfWork) Subscriptions() subscriptions.Repository {
	return subscriptionrepo.NewRepository(u.db)
}

func (u UnitOfWork) Tickets() tickets.Repository {
	return ticketrepo.NewRepository(u.db)
}

func (u UnitOfWork) Tokens() tokens.Repository {
	return tokenrepo.NewRepository(u.db)
}

func (u UnitOfWork) Webhooks() webhooks.Repository {
	return webhookrepo.NewRepository(u.db)
}

func (u UnitOfWork) Users() users.Repository {
	return userrepo.NewRepository(u.db)
}

func (u UnitOfWork) Records() records.Repository {
	return recordsrepo.NewRepository(u.db)
}

func (u UnitOfWork) Resources() resources.Repository {
	return resourcerepo.NewRepository(u.db)
}

func (u UnitOfWork) Coupons() coupons.Repository {
	return couponrepo.NewRepository(u.db)
}

func (u UnitOfWork) CouponUsages() coupons.CouponUsageRepository {
	return couponrepo.NewCouponUsageRepository(u.db)
}

func (u UnitOfWork) Entries() entries.Repository {
	return entryrepo.NewRepository(u.db)
}

func (u UnitOfWork) Rebac() rebac.Repository {
	return rebacrepo.NewRepository(u.db)
}

// B2BDomains 返回 B2B 域名治理子仓库（infra/postgres/b2b 真实实现）。
func (u UnitOfWork) B2BDomains() b2b.Repository {
	return b2brepo.NewRepository(u.db)
}

func (u UnitOfWork) Privacy() privacyDomain.DsarRepository {
	return privacyRepo.NewRepository(u.db)
}

func (u UnitOfWork) PrivacyConsents() privacyDomain.ConsentRepository {
	return privacyRepo.NewConsentRepository(u.db)
}

func (u UnitOfWork) Risk() risk.Repository {
	return riskrepo.NewRepository(u.db)
}

func (u UnitOfWork) IDV() idv.Repository {
	return idvrepo.NewRepository(u.db)
}

func (u UnitOfWork) Actions() actionsdomain.Repository {
	return actionsrepo.NewRepository(u.db)
}

func (u UnitOfWork) Metering() meteringdomain.Repository {
	return meteringrepo.NewRepository(u.db)
}

func (u UnitOfWork) Workload() workloaddomain.Repository {
	return workloadrepo.NewRepository(u.db)
}

func (u UnitOfWork) IGA() igadomain.Repository {
	return igarepo.NewRepository(u.db)
}

func (u UnitOfWork) McpServers() mcpservers.Repository {
	return mcpserverrepo.NewRepository(u.db)
}

func (u UnitOfWork) RadiusAccountings() radiusaccountings.Repository {
	return radiusaccountingrepo.NewRepository(u.db)
}

func (u UnitOfWork) Verifications() verifications.Repository {
	return verificationrepo.NewRepository(u.db)
}

func (u UnitOfWork) WebAuthnCredentials() webauthn.Repository {
	return webauthnrepo.NewRepository(u.db)
}

// TrustedDevices is intentionally exposed only on the concrete UoW (through a
// narrow app-side type assertion) to avoid expanding the shared domain UoW
// interfaces and every fake that implements them.
func (u UnitOfWork) TrustedDevices() trusteddevices.Repository {
	return trusteddevicerepo.NewRepository(u.db)
}

// RememberMe is intentionally exposed only on the concrete UoW (through a
// narrow app-side type assertion) so the shared UoW interfaces and their fakes
// stay unchanged.
func (u UnitOfWork) RememberMe() remembermedomain.Repository {
	return remembermerepo.NewRepository(u.db)
}

func (u UnitOfWork) CaptchaProviders() captchaproviders.Repository {
	return captchaproviderrepo.NewRepository(u.db)
}

func (u UnitOfWork) FaceIdProviders() faceidproviders.Repository {
	return faceidproviderrepo.NewRepository(u.db)
}

// FaceTemplates is intentionally exposed only on the concrete UoW (through a
// narrow app-side type assertion) so the shared domain UoW interfaces and
// their fakes stay unchanged. It stores the encrypted FaceID login底图.
func (u UnitOfWork) FaceTemplates() users.FaceTemplateRepository {
	return facetemplaterepo.NewRepository(u.db)
}

func (u UnitOfWork) NotificationProviders() notificationproviders.Repository {
	return notificationproviderrepo.NewRepository(u.db)
}

func (u UnitOfWork) Notifications() notifications.Repository {
	return notificationrepo.NewRepository(u.db)
}

func (u UnitOfWork) UserNotifications() usernotifications.Repository {
	return usernotificationrepo.NewRepository(u.db)
}

func (u UnitOfWork) RadiusConfigs() radiusconfigs.Repository {
	return radiusconfigrepo.NewRepository(u.db)
}

func (u UnitOfWork) Rules() rules.Repository {
	return rulerepo.NewRepository(u.db)
}

func (u UnitOfWork) Transactions() transactions.Repository {
	return transactionrepo.NewRepository(u.db)
}

func (u UnitOfWork) Outbox() domainoutbox.Repository {
	return outboxrepo.NewRepository(u.db, nil)
}

func (u UnitOfWork) WebhookEvents() webhookevents.Repository {
	return webhookeventrepo.NewRepository(u.db)
}

func (u UnitOfWork) Keys() keys.Repository {
	return keyrepo.NewRepository(u.db)
}

func (u UnitOfWork) QRCodeSessions() qrcodesessions.Repository {
	return qrcodesessionrepo.NewRepository(u.db)
}

func (u UnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository {
	return deviceauthcoderepo.NewRepository(u.db)
}

func (u UnitOfWork) Web3Identities() web3identity.Repository {
	return web3identityrepo.NewRepository(u.db)
}

func (u UnitOfWork) FaceIDSessions() faceidsessions.Repository {
	return faceidsessionrepo.NewRepository(u.db)
}

func (u UnitOfWork) CASTickets() castickets.Repository {
	return casticketrepo.NewRepository(u.db)
}

func (u UnitOfWork) KerberosTickets() kerberostickets.Repository {
	return kerberosticketrepo.NewRepository(u.db)
}

func (u UnitOfWork) LockoutAttempts() lockoutattempts.Repository {
	return lockoutattemptrepo.NewRepository(u.db)
}

func (u UnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	tx, err := u.db.Begin(ctx)
	if err != nil {
		return err
	}

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	// RLS 租户上下文注入（ADR-016）：在事务内设置 app.current_organization_id GUC，
	// 使 FORCE ROW LEVEL SECURITY 策略生效。未设置租户上下文时策略 fail-closed。
	if err := setTenantContext(ctx, tx); err != nil {
		return err
	}

	// Preserve the pool so public-discovery repositories keep using the
	// RLS-bypassing public projection inside the transaction instead of
	// falling back to the full (credential-bearing) organization query.
	txUOW := New(tx, u.pool)
	if err := fn(ctx, txUOW); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	committed = true
	return nil
}

// setTenantContext 在事务内设置 RLS 租户上下文。
// 从 context 中提取 TenantScope，解析组织标识（UUID 或 slug）为 UUID，
// 调用 guliangiam.set_tenant_context(uuid, role)。
//
// 关键约束：guliangiam.set_tenant_context 的第一个参数是 UUID 类型，
// pgx 驱动会对传入值做 ::uuid cast。如果传入的是 organization slug
// （如 "built-in"），cast 会在 SQL 执行前就触发：
//
//	ERROR: invalid input syntax for type uuid: "built-in"
//
// 因此 resolve 必须在此函数内完成，且 super_admin/system 模式
// （is_super_admin() 会放行所有租户隔离）直接传 nil::uuid，
// 不需要也不应该尝试 slug → UUID 解析。
func setTenantContext(ctx context.Context, tx pgx.Tx) error {
	scope := pgtenant.TenantScopeFromContext(ctx)

	role := platformdb.RLSRoleMember
	isPrivileged := scope.Mode == pgtenant.TenantFilterSuperAdmin || scope.Mode == pgtenant.TenantFilterSystem
	if isPrivileged {
		// super_admin 模式下 RLS is_super_admin() = true 会放行所有租户隔离
		// 直接设 SESSION=false 的 config 值，让整个事务内 role=super_admin, org=''
		// 事务内必须用 LOCAL(true)，不能用 SESSION(false)
		// SESSION 级会污染 pgx pool 连接，后续 member 请求继承错误值
		if _, err := tx.Exec(ctx,
			`SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`,
		); err != nil {
			return apperrors.WrapInternal(err, "set super_admin role")
		}
		return nil
	}

	// =====================================================
	// 普通 member 模式：slug → UUID resolve 后再设 tenant context
	// =====================================================
	orgID := scope.OrganizationID
	if (orgID == "" || !isValidUUID(orgID)) && scope.OrganizationName != "" {
		orgID = scope.OrganizationName
	}

	if orgID != "" && !isValidUUID(orgID) {
		// slug → UUID 解析（必须在 super_admin role 下做，否则 RLS 拦截）
		// 事务内用 LOCAL(true) 避免污染 pool
		if _, err := tx.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', true)`); err != nil {
			return err
		}
		var resolved string
		if err := tx.QueryRow(ctx,
			`SELECT id::text FROM guliangiam.organizations WHERE slug = $1 LIMIT 1`,
			orgID,
		).Scan(&resolved); err != nil {
			slog.WarnContext(ctx, "setTenantContext: slug resolve failed, using nil org",
				"slug", orgID, "error", err)
			// resolve 失败 → 不设 org，让 RLS 用 NULL（fail-closed）
			orgID = ""
		} else {
			orgID = resolved
		}
	}

	// 用 set_tenant_context(p_organization_id::uuid, p_role) — 如果 orgID
	// 是空串 → 传 nil → 函数内部 COALESCE(NULL::text, '') = '' → current_organization_id() = NULL
	var orgParam any
	if orgID != "" {
		orgParam = orgID
	}

	if _, err := tx.Exec(ctx,
		`SELECT guliangiam.set_tenant_context($1::uuid, $2)`,
		orgParam, role,
	); err != nil {
		return apperrors.WrapInternal(err, "set tenant context")
	}
	return nil
}

// isValidUUID 校验字符串是否为 UUID 格式（不依赖外部库）。
func isValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

var _ appuow.UnitOfWork = UnitOfWork{}
