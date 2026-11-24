package uow

import (
	"context"

	actions "github.com/guliangiam/guliangiam/internal/domain/actions"
	"github.com/guliangiam/guliangiam/internal/domain/adapters"
	"github.com/guliangiam/guliangiam/internal/domain/agents"
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
	iga "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/guliangiam/guliangiam/internal/domain/kerberostickets"
	"github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/domain/ldaps"
	"github.com/guliangiam/guliangiam/internal/domain/lockoutattempts"
	"github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	metering "github.com/guliangiam/guliangiam/internal/domain/metering"
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
	"github.com/guliangiam/guliangiam/internal/domain/products"
	"github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/domain/qrcodesessions"
	"github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	"github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	"github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/domain/records"
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
	"github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/domain/web3identity"
	"github.com/guliangiam/guliangiam/internal/domain/webauthn"
	"github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	"github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workload "github.com/guliangiam/guliangiam/internal/domain/workload"
)

type IdentityUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Users() users.Repository
	Organizations() organizations.Repository
	Groups() groups.Repository
	Roles() roles.Repository
	Permissions() permissions.Repository
	Invitations() invitations.Repository
	WebAuthnCredentials() webauthn.Repository
}

// PlatformAdminUoW 平台管理员 + 作用域 + 跨租户 ticket CRUD。
// 对应 V5 platform_admins + platform_admin_scopes + cross_tenant_operations 表。
// 所有操作 RLS 要求 is_super_admin() — 走 TenantFilterSystem ctx。
type PlatformAdminUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	PlatformAdmins() platformadmins.PlatformAdminRepository
	PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository
	CrossTenantOps() crosstenantops.TicketRepository
}

type AuthUoW interface {
	IdentityUoW
	AuthnUoW
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Applications() applications.Repository
	Providers() providers.Repository
	Certs() certs.Repository
	Tokens() tokens.Repository
	Sessions() sessions.Repository
	Consents() consents.Repository
	Verifications() verifications.Repository
	Keys() keys.Repository
	CaptchaProviders() captchaproviders.Repository
	Records() records.Repository
	FaceIdProviders() faceidproviders.Repository
}

type AuthnUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	AuthorizationCodes() authorizationcodes.Repository
	DeviceAuthCodes() deviceauthcodes.Repository
	QRCodeSessions() qrcodesessions.Repository
	Web3Identities() web3identity.Repository
	FaceIDSessions() faceidsessions.Repository
	CASTickets() castickets.Repository
	KerberosTickets() kerberostickets.Repository
	LockoutAttempts() lockoutattempts.Repository
	MfaChallenges() mfachallenges.Repository
}

type AuditUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Records() records.Repository
	Entries() entries.Repository
	WebhookEvents() webhookevents.Repository
	Transactions() transactions.Repository
	Rules() rules.Repository
}

type CommerceUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Orders() orders.Repository
	Payments() payments.Repository
	Plans() plans.Repository
	Pricings() pricings.Repository
	Products() products.Repository
	Subscriptions() subscriptions.Repository
	Coupons() coupons.Repository
	CouponUsages() coupons.CouponUsageRepository
	Tickets() tickets.Repository
}

type InfraUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Outbox() domainoutbox.Repository
	Adapters() adapters.Repository
	Agents() agents.Repository
	Enforcers() enforcers.Repository
	Forms() forms.Repository
	Models() models.Repository
	Syncers() syncers.Repository
	Webhooks() webhooks.Repository
	Ldaps() ldaps.Repository
	Gateway() gateway.Repository
	CaptchaProviders() captchaproviders.Repository
	NotificationProviders() notificationproviders.Repository
	Notifications() notifications.Repository
	FaceIdProviders() faceidproviders.Repository
	RadiusConfigs() radiusconfigs.Repository
	RadiusAccountings() radiusaccountings.Repository
	McpServers() mcpservers.Repository
	Resources() resources.Repository
}

type AuthzUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Rebac() rebac.Repository
}

type RiskUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Risk() risk.Repository
}

type IDVUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	IDV() idv.Repository
}

type ActionsUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Actions() actions.Repository
}

type MeteringUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Metering() metering.Repository
}

type WorkloadUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Workload() workload.Repository
}

type IGAUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	IGA() iga.Repository
	Sessions() sessions.Repository
}

// TxRunner 是最小化事务执行接口。域UoW Service 可通过类型断言获取
// 实际 UoW 实现（postgresuow/mysqluow）均实现此接口。
type TxRunner interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
}

// EnsureTxRunner 从任意域UoW接口提取事务执行器。
// 如果底层实现未提供事务能力则返回 (nil, false)。
func EnsureTxRunner(uow any) (TxRunner, bool) {
	if tx, ok := uow.(TxRunner); ok {
		return tx, true
	}
	return nil, false
}

type UnitOfWork interface {
	IdentityUoW
	AuthUoW
	AuthnUoW
	AuthzUoW
	RiskUoW
	IDVUoW
	ActionsUoW
	MeteringUoW
	WorkloadUoW
	IGAUoW
	AuditUoW
	CommerceUoW
	InfraUoW
	PlatformAdminUoW
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
}

// TenantProvisioningUoW 是租户开通/退役的窄能力接口。
// 不并入 UnitOfWork：绝大多数域 UoW 不需要租户开通能力，强行加入会迫使
// 所有 fake/mock 实现 Tenants()。organizations 服务通过类型断言获取：
//
//	var tenantTx appuow.TenantProvisioningUoW
//	if tx, ok := someUoW.(appuow.TenantProvisioningUoW); ok { tenantTx = tx }
type TenantProvisioningUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	Tenants() tenants.Repository
	// Enforcers 用于开通租户时写入默认 enforcer（DEF-34）：没有 enforcer 行时，
	// 该组织用户的任何授权判定都会在 ResolveEnforcerUUID 处 500。
	Enforcers() enforcers.Repository
}

// SessionCacheUoW 是鉴权读路径的可选能力：缓存命中时直接返回会话，
// 调用方无需开启事务（RLS 只需要在未命中回源时设置租户上下文）。
// 只被实现该能力的 UoW 使用，其它实现自动回退到原有事务路径：
//
//	var cacheUoW appuow.SessionCacheUoW
//	if c, ok := uow.(appuow.SessionCacheUoW); ok { ... }
type SessionCacheUoW interface {
	// CachedSession returns a cached session only when it is present and still
	// usable (not expired, not revoked). It never touches the database.
	CachedSession(ctx context.Context, id string) (*sessions.Session, bool)
}

// UserNotificationsUoW 是个人通知中心的窄能力接口 —— 不嵌入 InfraUoW/UnitOfWork。
// 原因同 B2BUoW：个人收件箱只有 usernotifications 服务使用，强制所有 fake UoW
// 实现 UserNotifications() 会造成 30+ service_test.go 批量修改。
// infra/postgres/uow 与 infra/mysql/uow 的 UnitOfWork struct 已实现该方法，
// 组合根与 app 服务通过类型断言取得此能力：
//
//	var tx appuow.UserNotificationsUoW
//	if v, ok := someUoW.(appuow.UserNotificationsUoW); ok { tx = v }
type UserNotificationsUoW interface {
	UnitOfWork
	UserNotifications() usernotifications.Repository
}

// B2BUoW 是独立接口 —— 不嵌入 UnitOfWork 组合接口。
// 原因：B2B 域（domain/b2b）还没有 app 层调用方，强制所有 fake UoW
// 实现 B2BDomains() 会导致 30+ service_test.go 批量修改。
// infra/postgres/uow 和 infra/mysql/uow 的 UnitOfWork struct 同时实现
// UnitOfWork + B2BUoW —— app/b2b/service.go 未来通过类型断言获取：
//
//	var b2bTx B2BUoW
//	if tx, ok := someUoW.(B2BUoW); ok { b2bTx = tx }
type B2BUoW interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx UnitOfWork) error) error
	B2BDomains() b2b.Repository
}
