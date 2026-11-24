package uow

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mysqlactions "github.com/guliangiam/guliangiam/internal/infra/mysql/actions"
	mysqladapters "github.com/guliangiam/guliangiam/internal/infra/mysql/adapters"
	mysqlagents "github.com/guliangiam/guliangiam/internal/infra/mysql/agents"
	mysqlapplications "github.com/guliangiam/guliangiam/internal/infra/mysql/applications"
	mysqlauthorizationcodes "github.com/guliangiam/guliangiam/internal/infra/mysql/authorizationcodes"
	mysqlb2b "github.com/guliangiam/guliangiam/internal/infra/mysql/b2b"
	mysqlcaptchaproviders "github.com/guliangiam/guliangiam/internal/infra/mysql/captchaproviders"
	mysqlcastickets "github.com/guliangiam/guliangiam/internal/infra/mysql/castickets"
	mysqlcerts "github.com/guliangiam/guliangiam/internal/infra/mysql/certs"
	mysqlconsents "github.com/guliangiam/guliangiam/internal/infra/mysql/consents"
	mysqlcoupons "github.com/guliangiam/guliangiam/internal/infra/mysql/coupons"
	mysqldeviceauthcodes "github.com/guliangiam/guliangiam/internal/infra/mysql/deviceauthcodes"
	mysqlenforcers "github.com/guliangiam/guliangiam/internal/infra/mysql/enforcers"
	mysqlentries "github.com/guliangiam/guliangiam/internal/infra/mysql/entries"
	mysqlfaceidproviders "github.com/guliangiam/guliangiam/internal/infra/mysql/faceidproviders"
	mysqlfaceidsessions "github.com/guliangiam/guliangiam/internal/infra/mysql/faceidsessions"
	mysqlfacetemplates "github.com/guliangiam/guliangiam/internal/infra/mysql/facetemplates"
	mysqlforms "github.com/guliangiam/guliangiam/internal/infra/mysql/forms"
	mysqlgateway "github.com/guliangiam/guliangiam/internal/infra/mysql/gateway"
	mysqlgroups "github.com/guliangiam/guliangiam/internal/infra/mysql/groups"
	mysqlidv "github.com/guliangiam/guliangiam/internal/infra/mysql/idv"
	mysqliga "github.com/guliangiam/guliangiam/internal/infra/mysql/iga"
	mysqlinvitations "github.com/guliangiam/guliangiam/internal/infra/mysql/invitations"
	mysqlkerberostickets "github.com/guliangiam/guliangiam/internal/infra/mysql/kerberostickets"
	mysqlkeys "github.com/guliangiam/guliangiam/internal/infra/mysql/keys"
	mysqlldaps "github.com/guliangiam/guliangiam/internal/infra/mysql/ldaps"
	mysqllockoutattempts "github.com/guliangiam/guliangiam/internal/infra/mysql/lockoutattempts"
	mysqlmcpservers "github.com/guliangiam/guliangiam/internal/infra/mysql/mcpservers"
	mysqlmetering "github.com/guliangiam/guliangiam/internal/infra/mysql/metering"
	mysqlmfachallenges "github.com/guliangiam/guliangiam/internal/infra/mysql/mfachallenges"
	mysqlmodels "github.com/guliangiam/guliangiam/internal/infra/mysql/models"
	mysqlnotificationproviders "github.com/guliangiam/guliangiam/internal/infra/mysql/notificationproviders"
	mysqlnotifications "github.com/guliangiam/guliangiam/internal/infra/mysql/notifications"
	mysqlorders "github.com/guliangiam/guliangiam/internal/infra/mysql/orders"
	mysqlorganizations "github.com/guliangiam/guliangiam/internal/infra/mysql/organizations"
	mysqloutbox "github.com/guliangiam/guliangiam/internal/infra/mysql/outbox"
	mysqlpayments "github.com/guliangiam/guliangiam/internal/infra/mysql/payments"
	mysqlpermissions "github.com/guliangiam/guliangiam/internal/infra/mysql/permissions"
	mysqlplans "github.com/guliangiam/guliangiam/internal/infra/mysql/plans"
	mysqlpricings "github.com/guliangiam/guliangiam/internal/infra/mysql/pricings"
	mysqlproducts "github.com/guliangiam/guliangiam/internal/infra/mysql/products"
	mysqlproviders "github.com/guliangiam/guliangiam/internal/infra/mysql/providers"
	mysqlqrcodesessions "github.com/guliangiam/guliangiam/internal/infra/mysql/qrcodesessions"
	mysqlradiusaccountings "github.com/guliangiam/guliangiam/internal/infra/mysql/radiusaccountings"
	mysqlradiusconfigs "github.com/guliangiam/guliangiam/internal/infra/mysql/radiusconfigs"
	mysqlrebac "github.com/guliangiam/guliangiam/internal/infra/mysql/rebac"
	mysqlrecords "github.com/guliangiam/guliangiam/internal/infra/mysql/records"
	mysqlresources "github.com/guliangiam/guliangiam/internal/infra/mysql/resources"
	mysqlrisk "github.com/guliangiam/guliangiam/internal/infra/mysql/risk"
	mysqlroles "github.com/guliangiam/guliangiam/internal/infra/mysql/roles"
	mysqlrules "github.com/guliangiam/guliangiam/internal/infra/mysql/rules"
	mysqlsessions "github.com/guliangiam/guliangiam/internal/infra/mysql/sessions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	mysqlsubscriptions "github.com/guliangiam/guliangiam/internal/infra/mysql/subscriptions"
	mysqlsyncers "github.com/guliangiam/guliangiam/internal/infra/mysql/syncers"
	mysqltickets "github.com/guliangiam/guliangiam/internal/infra/mysql/tickets"
	mysqltokens "github.com/guliangiam/guliangiam/internal/infra/mysql/tokens"
	mysqltransactions "github.com/guliangiam/guliangiam/internal/infra/mysql/transactions"
	mysqltrusteddevices "github.com/guliangiam/guliangiam/internal/infra/mysql/trusteddevices"
	mysqlusernotifications "github.com/guliangiam/guliangiam/internal/infra/mysql/usernotifications"
	mysqlusers "github.com/guliangiam/guliangiam/internal/infra/mysql/users"
	mysqlverifications "github.com/guliangiam/guliangiam/internal/infra/mysql/verifications"
	mysqlweb3identity "github.com/guliangiam/guliangiam/internal/infra/mysql/web3identity"
	mysqlwebauthncredentials "github.com/guliangiam/guliangiam/internal/infra/mysql/webauthncredentials"
	mysqlwebhookevents "github.com/guliangiam/guliangiam/internal/infra/mysql/webhookevents"
	mysqlwebhooks "github.com/guliangiam/guliangiam/internal/infra/mysql/webhooks"
	mysqlworkload "github.com/guliangiam/guliangiam/internal/infra/mysql/workload"

	actionsdomain "github.com/guliangiam/guliangiam/internal/domain/actions"
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
	tenantidentities "github.com/guliangiam/guliangiam/internal/domain/tenant_identities"
	tenantinstances "github.com/guliangiam/guliangiam/internal/domain/tenant_instances"
	"github.com/guliangiam/guliangiam/internal/domain/tenants"
	"github.com/guliangiam/guliangiam/internal/domain/tickets"
	"github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/domain/transactions"
	"github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	"github.com/guliangiam/guliangiam/internal/domain/usernotifications"
	"github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/domain/web3identity"
	"github.com/guliangiam/guliangiam/internal/domain/webauthn"
	"github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	"github.com/guliangiam/guliangiam/internal/domain/webhooks"
	workloaddomain "github.com/guliangiam/guliangiam/internal/domain/workload"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// UnitOfWork 是 MySQL 8.4+ 实现的 appuow.UnitOfWork。
// 共享 domain.Repository 接口，底层用 mysqlshared.DBTX（*sql.DB 或 *sql.Tx）。
// 实现策略：Rebac() + Sessions() + WithinTransaction() 完整，其他 61 个方法 panic 严格失败。
type UnitOfWork struct{ db mysqlshared.DBTX }

// New 构造顶层 UnitOfWork，接受 *sql.DB（实现 Beginner 接口，可开启事务）。
func New(db *sql.DB) UnitOfWork { return UnitOfWork{db: db} }

// newWithDBTX 接受 DBTX（*sql.DB 或 *sql.Tx）构造 UnitOfWork。
// 事务内通过 WithinTransaction 调用，让所有 repo 在同一个 tx 上操作。
func newWithDBTX(db mysqlshared.DBTX) UnitOfWork { return UnitOfWork{db: db} }

func (u UnitOfWork) Applications() applications.Repository {
	return mysqlapplications.NewRepository(u.db)
}

func (u UnitOfWork) AuthorizationCodes() authorizationcodes.Repository {
	return mysqlauthorizationcodes.NewRepository(u.db)
}

func (u UnitOfWork) Certs() certs.Repository {
	return mysqlcerts.NewRepository(u.db)
}

func (u UnitOfWork) Adapters() adapters.Repository {
	return mysqladapters.NewRepository(u.db)
}

func (u UnitOfWork) Agents() agents.Repository {
	return mysqlagents.NewRepository(u.db)
}

func (u UnitOfWork) Enforcers() enforcers.Repository {
	return mysqlenforcers.NewRepository(u.db)
}

func (u UnitOfWork) Forms() forms.Repository {
	return mysqlforms.NewRepository(u.db)
}

func (u UnitOfWork) Ldaps() ldaps.Repository {
	return mysqlldaps.NewRepository(u.db)
}

func (u UnitOfWork) Models() models.Repository {
	return mysqlmodels.NewRepository(u.db)
}

func (u UnitOfWork) Orders() orders.Repository {
	return mysqlorders.NewRepository(u.db)
}

func (u UnitOfWork) Payments() payments.Repository {
	return mysqlpayments.NewRepository(u.db)
}

// Refunds 暴露 tx 绑定的退款仓储（BE-C-09，与 postgres 镜像同形）。
func (u UnitOfWork) Refunds() payments.RefundsRepo {
	return mysqlpayments.NewRefundsRepository(u.db)
}

func (u UnitOfWork) Plans() plans.Repository {
	return mysqlplans.NewRepository(u.db)
}

// PlatformAdmins / PlatformAdminScopes / CrossTenantOps — mysql 后端暂不实现。
// platform_admins + platform_admin_scopes 表依赖 Postgres RLS + SECURITY DEFINER
// 函数（is_super_admin() / current_platform_admin_scopes()），
// mysql 后端需要迁移至其他授权机制。返回 notImplemented repo 防止 panic。

func (u UnitOfWork) PlatformAdmins() platformadmins.PlatformAdminRepository {
	return notImplementedPlatformAdminRepo{}
}

func (u UnitOfWork) PlatformAdminScopes() platformadmins.PlatformAdminScopeRepository {
	return notImplementedPlatformAdminScopeRepo{}
}

func (u UnitOfWork) CrossTenantOps() crosstenantops.TicketRepository {
	return notImplementedCrossTenantOpsRepo{}
}

func (u UnitOfWork) Pricings() pricings.Repository {
	return mysqlpricings.NewRepository(u.db)
}

func (u UnitOfWork) Products() products.Repository {
	return mysqlproducts.NewRepository(u.db)
}

func (u UnitOfWork) Consents() consents.Repository {
	return mysqlconsents.NewRepository(u.db)
}

func (u UnitOfWork) Gateway() gateway.Repository {
	return mysqlgateway.NewRepository(u.db)
}

func (u UnitOfWork) Groups() groups.Repository {
	return mysqlgroups.NewRepository(u.db)
}

func (u UnitOfWork) Invitations() invitations.Repository {
	return mysqlinvitations.NewRepository(u.db)
}

func (u UnitOfWork) MfaChallenges() mfachallenges.Repository {
	return mysqlmfachallenges.NewRepository(u.db)
}

func (u UnitOfWork) Organizations() organizations.Repository {
	return mysqlorganizations.NewRepository(u.db)
}

// Tenants — MySQL 无 RLS，租户身份/拓扑写入未实现（OPEN-API-37）。
// 返回 graceful unimplemented，不宣称 MySQL 可用。
func (u UnitOfWork) Tenants() tenants.Repository {
	return notImplementedTenantsRepo{}
}

func (u UnitOfWork) Permissions() permissions.Repository {
	return mysqlpermissions.NewRepository(u.db)
}

func (u UnitOfWork) Providers() providers.Repository {
	return mysqlproviders.NewRepository(u.db)
}

func (u UnitOfWork) Roles() roles.Repository {
	return mysqlroles.NewRepository(u.db)
}

func (u UnitOfWork) Syncers() syncers.Repository {
	return mysqlsyncers.NewRepository(u.db)
}

func (u UnitOfWork) Sessions() sessions.Repository {
	return mysqlsessions.NewRepository(u.db)
}

func (u UnitOfWork) Subscriptions() subscriptions.Repository {
	return mysqlsubscriptions.NewRepository(u.db)
}

func (u UnitOfWork) Tickets() tickets.Repository {
	return mysqltickets.NewRepository(u.db)
}

func (u UnitOfWork) Tokens() tokens.Repository {
	return mysqltokens.NewRepository(u.db)
}

func (u UnitOfWork) Webhooks() webhooks.Repository {
	return mysqlwebhooks.NewRepository(u.db)
}

func (u UnitOfWork) Users() users.Repository { return mysqlusers.NewRepository(u.db) }

// TrustedDevices 与 Postgres 版一致：只暴露在具体 UoW 上，app 层用窄类型断言访问，
// 不向共享 domain UoW 接口扩散可选仓储。
func (u UnitOfWork) TrustedDevices() trusteddevices.Repository {
	return mysqltrusteddevices.NewRepository(u.db)
}

// FaceTemplates 与 Postgres 版一致：只暴露在具体 UoW 上，app 层用窄类型断言访问。
func (u UnitOfWork) FaceTemplates() users.FaceTemplateRepository {
	return mysqlfacetemplates.NewRepository(u.db)
}

func (u UnitOfWork) Records() records.Repository {
	return mysqlrecords.NewRepository(u.db)
}

func (u UnitOfWork) Resources() resources.Repository {
	return mysqlresources.NewRepository(u.db)
}

func (u UnitOfWork) Coupons() coupons.Repository {
	return mysqlcoupons.NewRepository(u.db)
}

func (u UnitOfWork) CouponUsages() coupons.CouponUsageRepository {
	return mysqlcoupons.NewUsageRepository(u.db)
}

func (u UnitOfWork) Entries() entries.Repository {
	return mysqlentries.NewRepository(u.db)
}

func (u UnitOfWork) Rebac() rebac.Repository { return mysqlrebac.NewRepositoryWithDBTX(u.db) }

// B2BDomains 返回 B2B 域名治理子仓库（PostgreSQL / MySQL 双方言实现一致）。
func (u UnitOfWork) B2BDomains() b2b.Repository { return mysqlb2b.NewRepository(u.db) }

// ── MySQL 不支持的 RLS 依赖仓储（graceful unimplemented，不 panic） ──────

// notImplementedTenantsRepo — 租户身份/拓扑开通依赖 Postgres 事务语义与 RLS，
// MySQL 驱动下未实现（OPEN-API-37）。
type notImplementedTenantsRepo struct{}

func (notImplementedTenantsRepo) notImpl() error {
	return apperrors.NewUnimplemented("TenantsRepository 在 MySQL 驱动上尚未实现（依赖 Postgres 事务与 RLS，见 OPEN-API-37）")
}

func (notImplementedTenantsRepo) CreateIdentity(ctx context.Context, identity *tenantidentities.TenantIdentity) error {
	return notImplementedTenantsRepo{}.notImpl()
}

func (notImplementedTenantsRepo) GetIdentity(ctx context.Context, organizationID string) (*tenantidentities.TenantIdentity, error) {
	return nil, notImplementedTenantsRepo{}.notImpl()
}

func (notImplementedTenantsRepo) DecommissionIdentity(ctx context.Context, organizationID string, at time.Time) error {
	return notImplementedTenantsRepo{}.notImpl()
}

func (notImplementedTenantsRepo) CreatePrimaryTopology(ctx context.Context, instance *tenantinstances.TenantInstance) error {
	return notImplementedTenantsRepo{}.notImpl()
}

func (notImplementedTenantsRepo) GetPrimaryTopology(ctx context.Context, organizationID string) (*tenantinstances.TenantInstance, error) {
	return nil, notImplementedTenantsRepo{}.notImpl()
}

func (notImplementedTenantsRepo) DecommissionTopologies(ctx context.Context, organizationID string, at time.Time) error {
	return notImplementedTenantsRepo{}.notImpl()
}

func (notImplementedTenantsRepo) PrimaryRegion(ctx context.Context) (*tenantinstances.Region, error) {
	return nil, notImplementedTenantsRepo{}.notImpl()
}

// PlatformAdminRepository — MySQL 无 RLS，平台管理员 CRUD 未实现。
type notImplementedPlatformAdminRepo struct{}

func (notImplementedPlatformAdminRepo) notImpl() error {
	return apperrors.NewUnimplemented("PlatformAdminRepository 在 MySQL 驱动上尚未实现（依赖 Postgres RLS）")
}

func (notImplementedPlatformAdminRepo) GetByID(ctx context.Context, id string) (*platformadmins.PlatformAdmin, error) {
	return nil, notImplementedPlatformAdminRepo{}.notImpl()
}
func (notImplementedPlatformAdminRepo) GetByUserID(ctx context.Context, userID string) (*platformadmins.PlatformAdmin, error) {
	return nil, notImplementedPlatformAdminRepo{}.notImpl()
}
func (notImplementedPlatformAdminRepo) List(ctx context.Context, opts platformadmins.ListOptions) ([]*platformadmins.PlatformAdmin, int64, error) {
	return nil, 0, notImplementedPlatformAdminRepo{}.notImpl()
}
func (notImplementedPlatformAdminRepo) Create(ctx context.Context, admin *platformadmins.PlatformAdmin) error {
	return notImplementedPlatformAdminRepo{}.notImpl()
}
func (notImplementedPlatformAdminRepo) Update(ctx context.Context, admin *platformadmins.PlatformAdmin) error {
	return notImplementedPlatformAdminRepo{}.notImpl()
}
func (notImplementedPlatformAdminRepo) SetActive(ctx context.Context, id string, active bool) error {
	return notImplementedPlatformAdminRepo{}.notImpl()
}
func (notImplementedPlatformAdminRepo) Delete(ctx context.Context, id string) error {
	return notImplementedPlatformAdminRepo{}.notImpl()
}

// PlatformAdminScopeRepository — MySQL 无 RLS + SECURITY DEFINER。
type notImplementedPlatformAdminScopeRepo struct{}

func (notImplementedPlatformAdminScopeRepo) notImpl() error {
	return apperrors.NewUnimplemented("PlatformAdminScopeRepository 在 MySQL 驱动上尚未实现（依赖 Postgres RLS + SECURITY DEFINER）")
}

func (notImplementedPlatformAdminScopeRepo) GetByID(ctx context.Context, id string) (*platformadmins.PlatformAdminScope, error) {
	return nil, notImplementedPlatformAdminScopeRepo{}.notImpl()
}
func (notImplementedPlatformAdminScopeRepo) ListByPlatformAdmin(ctx context.Context, adminID string, opts platformadmins.ScopeListOptions) ([]*platformadmins.PlatformAdminScope, error) {
	return nil, notImplementedPlatformAdminScopeRepo{}.notImpl()
}
func (notImplementedPlatformAdminScopeRepo) Create(ctx context.Context, scope *platformadmins.PlatformAdminScope) error {
	return notImplementedPlatformAdminScopeRepo{}.notImpl()
}
func (notImplementedPlatformAdminScopeRepo) Approve(ctx context.Context, scopeID, approverID string) error {
	return notImplementedPlatformAdminScopeRepo{}.notImpl()
}
func (notImplementedPlatformAdminScopeRepo) Delete(ctx context.Context, id string) error {
	return notImplementedPlatformAdminScopeRepo{}.notImpl()
}
func (notImplementedPlatformAdminScopeRepo) HasActiveScopeForTenant(ctx context.Context, adminID, tenantID string) (bool, error) {
	return false, notImplementedPlatformAdminScopeRepo{}.notImpl()
}

// TicketRepository — 跨租户操作票依赖 RLS。
type notImplementedCrossTenantOpsRepo struct{}

func (notImplementedCrossTenantOpsRepo) notImpl() error {
	return apperrors.NewUnimplemented("CrossTenantOps TicketRepository 在 MySQL 驱动上尚未实现（依赖 Postgres RLS）")
}

func (notImplementedCrossTenantOpsRepo) HasApprovedTicket(ctx context.Context, actorUserID, targetTenant, resourceType, action string) (bool, error) {
	return false, notImplementedCrossTenantOpsRepo{}.notImpl()
}
func (notImplementedCrossTenantOpsRepo) GetByID(ctx context.Context, id string) (*crosstenantops.Ticket, error) {
	return nil, notImplementedCrossTenantOpsRepo{}.notImpl()
}
func (notImplementedCrossTenantOpsRepo) Create(ctx context.Context, ticket *crosstenantops.Ticket) error {
	return notImplementedCrossTenantOpsRepo{}.notImpl()
}
func (notImplementedCrossTenantOpsRepo) Approve(ctx context.Context, ticketID, approverPlatformAdminID string) error {
	return notImplementedCrossTenantOpsRepo{}.notImpl()
}
func (notImplementedCrossTenantOpsRepo) Reject(ctx context.Context, ticketID, reason string) error {
	return notImplementedCrossTenantOpsRepo{}.notImpl()
}
func (notImplementedCrossTenantOpsRepo) MarkUsed(ctx context.Context, ticketID string) error {
	return notImplementedCrossTenantOpsRepo{}.notImpl()
}
func (notImplementedCrossTenantOpsRepo) ListByActor(ctx context.Context, actorUserID, status string) ([]*crosstenantops.Ticket, error) {
	return nil, notImplementedCrossTenantOpsRepo{}.notImpl()
}
func (notImplementedCrossTenantOpsRepo) ListByScope(ctx context.Context, scopeID string) ([]*crosstenantops.Ticket, error) {
	return nil, notImplementedCrossTenantOpsRepo{}.notImpl()
}

func (u UnitOfWork) Risk() risk.Repository {
	return mysqlrisk.NewRepository(u.db)
}

func (u UnitOfWork) IDV() idv.Repository {
	return mysqlidv.NewRepository(u.db)
}

func (u UnitOfWork) Actions() actionsdomain.Repository {
	return mysqlactions.NewRepository(u.db)
}

func (u UnitOfWork) Metering() meteringdomain.Repository {
	return mysqlmetering.NewRepository(u.db)
}

func (u UnitOfWork) Workload() workloaddomain.Repository {
	return mysqlworkload.NewRepository(u.db)
}

func (u UnitOfWork) IGA() igadomain.Repository {
	return mysqliga.NewRepository(u.db)
}

func (u UnitOfWork) McpServers() mcpservers.Repository {
	return mysqlmcpservers.NewRepository(u.db)
}

func (u UnitOfWork) RadiusAccountings() radiusaccountings.Repository {
	return mysqlradiusaccountings.NewRepository(u.db)
}

func (u UnitOfWork) Verifications() verifications.Repository {
	return mysqlverifications.NewRepository(u.db)
}

func (u UnitOfWork) WebAuthnCredentials() webauthn.Repository {
	return mysqlwebauthncredentials.NewRepository(u.db)
}

func (u UnitOfWork) CaptchaProviders() captchaproviders.Repository {
	return mysqlcaptchaproviders.NewRepository(u.db)
}

func (u UnitOfWork) FaceIdProviders() faceidproviders.Repository {
	return mysqlfaceidproviders.NewRepository(u.db)
}

func (u UnitOfWork) NotificationProviders() notificationproviders.Repository {
	return mysqlnotificationproviders.NewRepository(u.db)
}

func (u UnitOfWork) Notifications() notifications.Repository {
	return mysqlnotifications.NewRepository(u.db)
}

func (u UnitOfWork) UserNotifications() usernotifications.Repository {
	return mysqlusernotifications.NewRepository(u.db)
}

func (u UnitOfWork) RadiusConfigs() radiusconfigs.Repository {
	return mysqlradiusconfigs.NewRepository(u.db)
}

func (u UnitOfWork) Rules() rules.Repository {
	return mysqlrules.NewRepository(u.db)
}

func (u UnitOfWork) Transactions() transactions.Repository {
	return mysqltransactions.NewRepository(u.db)
}

func (u UnitOfWork) Outbox() domainoutbox.Repository {
	return mysqloutbox.NewRepository(u.db)
}

func (u UnitOfWork) WebhookEvents() webhookevents.Repository {
	return mysqlwebhookevents.NewRepository(u.db)
}

func (u UnitOfWork) Keys() keys.Repository {
	return mysqlkeys.NewRepository(u.db)
}

func (u UnitOfWork) QRCodeSessions() qrcodesessions.Repository {
	return mysqlqrcodesessions.NewRepository(u.db)
}

func (u UnitOfWork) DeviceAuthCodes() deviceauthcodes.Repository {
	return mysqldeviceauthcodes.NewRepository(u.db)
}

func (u UnitOfWork) Web3Identities() web3identity.Repository {
	return mysqlweb3identity.NewRepository(u.db)
}

func (u UnitOfWork) FaceIDSessions() faceidsessions.Repository {
	return mysqlfaceidsessions.NewRepository(u.db)
}

func (u UnitOfWork) CASTickets() castickets.Repository {
	return mysqlcastickets.NewRepository(u.db)
}

func (u UnitOfWork) KerberosTickets() kerberostickets.Repository {
	return mysqlkerberostickets.NewRepository(u.db)
}

func (u UnitOfWork) LockoutAttempts() lockoutattempts.Repository {
	return mysqllockoutattempts.NewRepository(u.db)
}

// savepointSeq 生成事务内唯一的 savepoint 名；嵌套调用可能在同一 tx 上并发排队。
var savepointSeq atomic.Uint64

// WithinTransaction 开启一个真实的 MySQL 事务，fn 内收到的 UoW 所有 repo 都在同一个 tx 上操作。
//
// 嵌套调用（fn 内再次调用 WithinTransaction，例如 app 服务组合调用另一个服务）不再
// 返回 Unimplemented：MySQL 不支持嵌套事务，但支持 SAVEPOINT。内层调用在外层 tx 上
// 建立 savepoint，成功则 RELEASE，失败则 ROLLBACK TO，语义与 PostgreSQL 的嵌套
// 事务块一致——外层仍然决定最终提交/回滚，内层失败只回滚自身写入。
func (u UnitOfWork) WithinTransaction(ctx context.Context, fn func(context.Context, appuow.UnitOfWork) error) error {
	if beginner, ok := u.db.(mysqlshared.Beginner); ok {
		tx, err := beginner.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		committed := false
		defer func() {
			if !committed {
				_ = tx.Rollback()
			}
		}()

		if err := fn(ctx, newWithDBTX(tx)); err != nil {
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	}

	// 已在事务内：用 savepoint 模拟嵌套事务带来的独立回滚边界。
	dbtx := u.db
	if dbtx == nil {
		return apperrors.NewUnimplemented("mysqluow: WithinTransaction requires a transaction-capable handle")
	}

	savepoint := fmt.Sprintf("uow_sp_%d", savepointSeq.Add(1))
	if _, err := dbtx.ExecContext(ctx, "SAVEPOINT "+savepoint); err != nil {
		return err
	}
	if err := fn(ctx, newWithDBTX(dbtx)); err != nil {
		if _, rollbackErr := dbtx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+savepoint); rollbackErr != nil {
			return fmt.Errorf("%w (savepoint rollback failed: %v)", err, rollbackErr)
		}
		return err
	}
	if _, err := dbtx.ExecContext(ctx, "RELEASE SAVEPOINT "+savepoint); err != nil {
		return err
	}
	return nil
}
