package bootstrap

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	actionsapp "github.com/guliangiam/guliangiam/internal/app/actions"
	appadapters "github.com/guliangiam/guliangiam/internal/app/adapters"
	appagents "github.com/guliangiam/guliangiam/internal/app/agents"
	appaigateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appauditdenial "github.com/guliangiam/guliangiam/internal/app/auditdenial"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	appcaptchaproviders "github.com/guliangiam/guliangiam/internal/app/captchaproviders"
	appcerts "github.com/guliangiam/guliangiam/internal/app/certs"
	appcoupons "github.com/guliangiam/guliangiam/internal/app/coupons"
	appdashboard "github.com/guliangiam/guliangiam/internal/app/dashboard"
	appenforcers "github.com/guliangiam/guliangiam/internal/app/enforcers"
	appentries "github.com/guliangiam/guliangiam/internal/app/entries"
	appfaceidproviders "github.com/guliangiam/guliangiam/internal/app/faceidproviders"
	appforms "github.com/guliangiam/guliangiam/internal/app/forms"
	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
	appiga "github.com/guliangiam/guliangiam/internal/app/iga"
	appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	appldaps "github.com/guliangiam/guliangiam/internal/app/ldaps"
	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	appmcpservers "github.com/guliangiam/guliangiam/internal/app/mcpservers"
	appmetering "github.com/guliangiam/guliangiam/internal/app/metering"
	appmodels "github.com/guliangiam/guliangiam/internal/app/models"
	appnotificationproviders "github.com/guliangiam/guliangiam/internal/app/notificationproviders"
	appnotifications "github.com/guliangiam/guliangiam/internal/app/notifications"
	apporders "github.com/guliangiam/guliangiam/internal/app/orders"
	apporganizations "github.com/guliangiam/guliangiam/internal/app/organizations"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
	appplans "github.com/guliangiam/guliangiam/internal/app/plans"
	appplatformadmins "github.com/guliangiam/guliangiam/internal/app/platform_admins"
	apppricings "github.com/guliangiam/guliangiam/internal/app/pricings"
	appproducts "github.com/guliangiam/guliangiam/internal/app/products"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	appradiusaccountings "github.com/guliangiam/guliangiam/internal/app/radiusaccountings"
	appradiusconfigs "github.com/guliangiam/guliangiam/internal/app/radiusconfigs"
	apprebac "github.com/guliangiam/guliangiam/internal/app/rebac"
	appreconciliation "github.com/guliangiam/guliangiam/internal/app/reconciliation"
	apprecords "github.com/guliangiam/guliangiam/internal/app/records"
	appresources "github.com/guliangiam/guliangiam/internal/app/resources"
	approles "github.com/guliangiam/guliangiam/internal/app/roles"
	apprules "github.com/guliangiam/guliangiam/internal/app/rules"
	appsearch "github.com/guliangiam/guliangiam/internal/app/search"
	appsessions "github.com/guliangiam/guliangiam/internal/app/sessions"
	appsubscriptions "github.com/guliangiam/guliangiam/internal/app/subscriptions"
	appsyncers "github.com/guliangiam/guliangiam/internal/app/syncers"
	apptickets "github.com/guliangiam/guliangiam/internal/app/tickets"
	apptokens "github.com/guliangiam/guliangiam/internal/app/tokens"
	apptransactions "github.com/guliangiam/guliangiam/internal/app/transactions"
	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"
	appwebhooks "github.com/guliangiam/guliangiam/internal/app/webhooks"
	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	domainidem "github.com/guliangiam/guliangiam/internal/domain/idempotency"
	domainradius "github.com/guliangiam/guliangiam/internal/domain/radius"
	"github.com/guliangiam/guliangiam/internal/infra/captcha"
	"github.com/guliangiam/guliangiam/internal/infra/casbin"
	"github.com/guliangiam/guliangiam/internal/infra/jwt"
	"github.com/guliangiam/guliangiam/internal/infra/messaging"
	"github.com/guliangiam/guliangiam/internal/infra/notification"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	pgpartitioning "github.com/guliangiam/guliangiam/internal/infra/postgres/partitioning"
	postgresregions "github.com/guliangiam/guliangiam/internal/infra/postgres/regions"
	postgresscim "github.com/guliangiam/guliangiam/internal/infra/postgres/scim"
	infraradius "github.com/guliangiam/guliangiam/internal/infra/radius"
	"github.com/guliangiam/guliangiam/internal/infra/storage"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformi18n "github.com/guliangiam/guliangiam/internal/platform/i18n"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	platformpartitioning "github.com/guliangiam/guliangiam/internal/platform/partitioning"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	actionstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/actions"
	adaptertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/adapters"
	agenttransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/agents"
	aigatewaytrans "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/aigateway"
	applicationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/applications"
	authtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	b2btransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/b2b"
	batchtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/batch"
	captchaprovidertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/captchaproviders"
	castransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/cas"
	certtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/certs"
	coupontransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/coupons"
	dashboardtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dashboard"
	dcrtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dcr"
	enforcertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/enforcers"
	entrytransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/entries"
	faceidprovidertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/faceidproviders"
	formtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/forms"
	gatewaytransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/gateway"
	groupstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/groups"
	i18ntransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/i18n"
	workloadtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/identitypools"
	idvtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/idv"
	igatransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/iga"
	invitationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/invitations"
	kerberustransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/kerberos"
	keystransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/keys"
	ldaptransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldaps"
	ldapsynctransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldapsync"
	mcpservertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/mcpservers"
	messagetransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/messages"
	meteringtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/metering"
	modeltransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/models"
	notificationprovidertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notificationproviders"
	notificationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notifications"
	ordertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/orders"
	organizationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/organizations"
	paymenttransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/payments"
	permissiontransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/permissions"
	plantransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/plans"
	platformadmintransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/platformadmins"
	pricingtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/pricings"
	privacytransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/privacy"
	producttransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/products"
	providertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/providers"
	radiustransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radius"
	radiusaccountingtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusaccountings"
	radiusconfigtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusconfigs"
	rebactransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rebac"
	reconciliationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/reconciliation"
	recordtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/records"
	resourcetransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/resources"
	risktransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/risk"
	roletransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/roles"
	ruletransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rules"
	samltransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/saml"
	scimv1transport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/scim"
	searchtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/search"
	sessiontransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/sessions"
	subscriptiontransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/subscriptions"
	syncertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/syncers"
	tickettransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tickets"
	tokentransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tokens"
	transactionstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/transactions"
	usernotificationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/usernotifications"
	usertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/users"
	verificationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/verifications"
	webauthntransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webauthn"
	webhookeventstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhookevents"
	webhooktransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhooks"
	wellknowntransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/wellknown"
	httpmiddleware "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	"github.com/redis/go-redis/v9"
)

// DBConfig holds database connection pools and driver metadata.
type DBConfig struct {
	Pool      *pgxpool.Pool
	Driver    string  // "pgx" (default) or "mysql" — 启动时从 DB_DRIVER 读取
	MySQLPool *sql.DB // MySQL 路径下持有 *sql.DB；PG 路径下为 nil
}

// AppServices holds all application-layer service instances.
type AppServices struct {
	CertService                 appcerts.Service
	CaptchaProviderService      appcaptchaproviders.Service
	NotificationProviderService appnotificationproviders.Service
	NotificationService         appnotifications.Service
	UserNotificationService     appusernotifications.Service
	AdapterService              appadapters.Service
	EnforcerService             appenforcers.Service
	ModelService                appmodels.Service
	SyncerService               appsyncers.Service
	WebhookService              appwebhooks.Service
	LdapService                 appldaps.Service
	LdapSyncService             *appldapsync.Service
	AgentService                appagents.Service
	FormService                 appforms.Service
	AuthService                 appauth.Service
	ApplicationService          appapplications.Service
	GatewayService              appgateway.Service
	GroupService                appgroups.Service
	InvitationService           appinvitations.Service
	OrganizationService         apporganizations.Service
	PermissionService           apppermissions.Service
	OrderService                apporders.Service
	PaymentService              apppayments.Service
	PlanService                 appplans.Service
	PricingService              apppricings.Service
	ProductService              appproducts.Service
	PlatformAdminService        appplatformadmins.Service
	ProviderService             appproviders.Service
	RecordService               apprecords.Service
	ResourceService             appresources.Service
	RoleService                 approles.Service
	SessionService              appsessions.Service
	SubscriptionService         appsubscriptions.Service
	TicketService               apptickets.Service
	TokenService                apptokens.Service
	ReconciliationService       appreconciliation.Service
	UserService                 appusers.Service
	CouponService               appcoupons.Service
	EntryService                appentries.Service
	McpServerService            appmcpservers.Service
	RadiusAccountingService     appradiusaccountings.Service
	VerificationService         appverifications.Service
	DashboardService            appdashboard.Service
	B2BService                  appb2b.Service
	FaceIdProviderService       appfaceidproviders.Service
	RuleService                 apprules.Service
	RadiusConfigService         appradiusconfigs.Service
	TransactionService          apptransactions.Service
	KeyService                  appkeys.Service
	SearchService               appsearch.Service
	AIGateway                   *appaigateway.GatewayService
	AIGatewayAdmin              *appaigateway.AdminService
}

// TransportHandlers holds all HTTP transport handler instances.
type TransportHandlers struct {
	CertHandler                 certtransport.Handler
	CaptchaProviderHandler      captchaprovidertransport.Handler
	NotificationProviderHandler notificationprovidertransport.Handler
	NotificationHandler         notificationtransport.Handler
	UserNotificationHandler     usernotificationtransport.Handler
	AdapterHandler              adaptertransport.Handler
	EnforcerHandler             enforcertransport.Handler
	ModelHandler                modeltransport.Handler
	SyncerHandler               syncertransport.Handler
	WebhookHandler              webhooktransport.Handler
	MessageHandler              messagetransport.Handler
	LdapHandler                 ldaptransport.Handler
	LdapSyncHandler             *ldapsynctransport.Handler
	AgentHandler                agenttransport.Handler
	FormHandler                 formtransport.Handler
	AuthHandler                 authtransport.Handler
	DCRHandler                  dcrtransport.Handler
	ApplicationHandler          applicationtransport.Handler
	GatewayHandler              gatewaytransport.Handler
	GroupHandler                groupstransport.Handler
	InvitationHandler           invitationtransport.Handler
	OrganizationHandler         organizationtransport.Handler
	PermissionHandler           permissiontransport.Handler
	OrderHandler                ordertransport.Handler
	PaymentHandler              paymenttransport.Handler
	PaymentNotifyHandler        paymenttransport.NotifyHandler
	ReconHandler                paymenttransport.ReconHandler
	ChannelConfigHandler        paymenttransport.ChannelConfigHandler
	ChannelCatalogHandler       paymenttransport.ChannelCatalogHandler
	AttemptsHandler             paymenttransport.AttemptsHandler
	RefundsHandler              paymenttransport.RefundsHandler
	CallbackReceiptsHandler     paymenttransport.CallbackReceiptsHandler
	PlanHandler                 plantransport.Handler
	PlatformAdminHandler        platformadmintransport.Handler
	PricingHandler              pricingtransport.Handler
	ProductHandler              producttransport.Handler
	ProviderHandler             providertransport.Handler
	RecordHandler               recordtransport.Handler
	RebacHandler                rebactransport.Handler
	RiskHandler                 risktransport.Handler
	PrivacyHandler              privacytransport.Handler
	IDVHandler                  idvtransport.Handler
	ActionsHandler              actionstransport.Handler
	MeteringHandler             meteringtransport.Handler
	WorkloadHandler             workloadtransport.Handler
	IGAHandler                  igatransport.Handler
	WebhookEventHandler         webhookeventstransport.Handler
	ResourceHandler             resourcetransport.Handler
	RoleHandler                 roletransport.Handler
	SessionHandler              sessiontransport.Handler
	SubscriptionHandler         subscriptiontransport.Handler
	TicketHandler               tickettransport.Handler
	TokenHandler                tokentransport.Handler
	UserHandler                 usertransport.Handler
	BatchHandler                batchtransport.Handler
	CouponHandler               coupontransport.Handler
	EntryHandler                entrytransport.Handler
	McpServerHandler            mcpservertransport.Handler
	RadiusAccountingHandler     radiusaccountingtransport.Handler
	VerificationHandler         verificationtransport.Handler
	DashboardHandler            dashboardtransport.Handler
	B2BHandler                  b2btransport.Handler
	FaceIdProviderHandler       faceidprovidertransport.Handler
	RuleHandler                 ruletransport.Handler
	RadiusConfigHandler         radiusconfigtransport.Handler
	TransactionHandler          transactionstransport.Handler
	KeyHandler                  keystransport.Handler
	SearchHandler               searchtransport.Handler
	WellknownHandler            wellknowntransport.Handler
	CASHandler                  *castransport.Handler
	SAMLHandler                 *samltransport.Handler
	WebAuthnHandler             *webauthntransport.Handler
	KerberosHandler             *kerberustransport.Handler
	RADIUSHandler               *radiustransport.Handler
	I18nHandler                 *i18ntransport.Handler
	SCIMV1Handler               *scimv1transport.Handler
	ReconciliationHandler       *reconciliationtransport.Handler
	AIGatewayAdminHandler       aigatewaytrans.AdminHandler
}

// InfraConfig holds infrastructure-level singletons.
type InfraConfig struct {
	StorageProvider     storage.Provider
	JWKSManager         domainauth.JWKSProvider
	CasbinManager       *casbin.Manager
	RebacEngine         *apprebac.Engine
	JWTIssuer           *jwt.Issuer
	SenderFactory       *messaging.SenderFactory
	CaptchaFactory      *captcha.Factory
	NotificationFactory *notification.Factory
	I18nTranslator      *platformi18n.Translator
	// Phase C-Step 1/2 新增：跨租户作用域授权基础设施
	ScopeAuthorizer   scopeauthorizer.ScopeAuthorizer
	PrincipalResolver scopeauthorizer.PrincipalTypeResolver
}

// MiddlewareConfig holds HTTP middleware configuration.
type MiddlewareConfig struct {
	WAFMiddleware     *httpmiddleware.WAFMiddleware
	IPRulesMiddleware *httpmiddleware.IPRulesMiddleware
	ServiceIPLimiter  *httpmiddleware.ServiceIPLimiter
	// CORSConfig 是跨域白名单（手册 4.16）。它是环境派生配置，在 Runtime 装配阶段
	// 校验一次即可——非法白名单必须让进程启动失败，而不是等第一个跨域请求静默不放行。
	CORSConfig httpmiddleware.CORSConfig
	// OrganizationSlugResolver 让 TenantResolutionMiddleware 把平台管理员显式指定
	// 的非 UUID target（组织 slug）解析成组织 UUID，再交给 ScopeAuthorizer 校验。
	// 由组合根注入组织仓储，transport 层不直接依赖 domain/infra。
	OrganizationSlugResolver httpmiddleware.OrganizationSlugResolver
	// RegionResolver 解析组织所在 region；RegionResidencyMode 决定跨 region
	// 租户访问是观察（observe，默认）还是拒绝（enforce）。
	RegionResolver      httpmiddleware.RegionResolver
	RegionResidencyMode httpmiddleware.RegionResidencyMode
	// PasswordGate 强制改密门禁（off/warn/enforce，默认 off）。密码状态按
	// session 缓存 30s，只有 enforce 模式会返回 403 password_expired。
	PasswordGate *authtransport.PasswordGate
	// DenialAudit is the best-effort L1/L2 rejection recorder.
	DenialAudit *appauditdenial.Recorder
}

// BackgroundWorkers holds background worker instances.
type BackgroundWorkers struct {
	OutboxWorker        *platformoutbox.Worker
	NotificationWorker  *appnotifications.Worker
	IdempotencyRepo     domainidem.Repository
	AIGatewayProbe      *appaigateway.HealthProbeWorker
	MeteringAggregate   *appmetering.AggregateWorker
	AuditChain          *platformaudit.ChainWorker
	IGAExpire           *appiga.ExpireWorker
	ProviderConsistency *appproviders.ConsistencyWorker
	ExpiryReminder      *appusernotifications.ExpiryReminderWorker
}

// LifecycleConfig holds lifecycle management components.
type LifecycleConfig struct {
	SessionCleanupStopCh chan struct{}
	ShutdownTelemetry    func(context.Context) error
	// SessionCacheCloser closes the Redis/memory session cache on shutdown.
	// Only set when the underlying cache implements io.Closer (RedisSessionCache does).
	// Memory cache has nothing to close — this will be nil in that case.
	SessionCacheCloser io.Closer
	// WorkersCancel cancels background worker goroutines on shutdown.
	// Used by: AIGatewayProbe, MeteringAggregate (ctx-based workers).
	WorkersCancel context.CancelFunc
	// FederationJWKSCache stops the remote JWKS background refresh loop.
	FederationJWKSCache interface{ Close() }
}

// Runtime is the top-level application runtime, composing sub-structures by responsibility.
type Runtime struct {
	DB         DBConfig
	Services   AppServices
	Handlers   TransportHandlers
	Infra      InfraConfig
	Middleware MiddlewareConfig
	Workers    BackgroundWorkers
	Lifecycle  LifecycleConfig
}

func NewRuntime(ctx context.Context, pool *pgxpool.Pool, mysqlDB *sql.DB, driver, dialectHint string) (runtime *Runtime, err error) {
	// --- Fail-Fast: migrateAndSeed 是 pgx 专用（PgMigrate 用 pgconn 类型）。
	// MySQL 路径暂时跳过迁移——双基线 schema 应由 DDL 脚本保证一致。
	if driver != driverMySQL {
		if err := migrateAndSeed(ctx, pool); err != nil {
			return nil, err
		}
		// 启动期自检：seed 创建的 built-in 管理员必须保有有效 all-scope。
		// 该记录损坏时平台跨租户可见性会静默消失（P1 症状）；生产 fail-fast，
		// 非生产只告警。MySQL 路径不适用（既有不支持边界，见 OPEN-API-37）。
		if err := verifyBootstrapAdminAllScope(
			ctx,
			queryBuiltinPlatformAdmin(pool),
			listBootstrapAdminScopes(pool),
			config.IsProduction(),
		); err != nil {
			return nil, err
		}
	}

	corsConfig := httpmiddleware.CORSConfigFromEnv()
	if err := corsConfig.Validate(); err != nil {
		return nil, err
	}

	// SSRF allowlist 注册表校验：非法 CIDR / 空描述 / 含 metadata 段 → 启动失败。
	if err := ssrf.ValidateRegistry(); err != nil {
		return nil, err
	}

	shutdownTelemetry := initTelemetry()

	// workersCtx 绑定所有后台 worker goroutines（AIGatewayProbe, MeteringAggregate 等）。
	// Close() 时 workersCancel() → 所有 ctx 监听者收到 Done → 优雅退出。
	// defer 在 error return 时立即 cancel（无 goroutine listen 也无害）；
	// 成功 return 时 workersCancel 交给 Runtime.Lifecycle.WorkersCancel 持有。
	//
	// ⚠️ TenantFilterSystem 注入：
	//   所有后台 Worker 入口统一用 TenantFilterSystem 包装 ctx → setTenantContext
	//   识别后映射到 super_admin role → RLS is_super_admin() = true 放行所有租户隔离。
	//   AfterConnect 改 fail-closed（Phase D1）后，Worker 依赖此注入保持 RLS 放行。
	systemCtx := infrapg.WithTenantScope(ctx, "", "", infrapg.TenantFilterSystem)
	// 关键：worker 生命周期必须与「启动期 10s 超时 ctx」解耦（DEF-46）。
	// NewHTTPServer 用 context.WithTimeout(...,10s) 创建启动 ctx，函数返回即 cancel；
	// 若 workersCtx 由它派生，所有 Worker 会在启动完成后立刻收到 Done：
	// outbox 永不在 5s 后轮询、webhook/notification 只跑首轮——表现为「队列永不投递」。
	// WithoutCancel 保留租户作用域值，仅切断取消传播；真正关闭由 Runtime.Close -> workersCancel 负责。
	workersCtx, workersCancel := context.WithCancel(context.WithoutCancel(systemCtx))
	defer func() {
		if err != nil {
			workersCancel()
		}
	}()

	// ── Session Cache (ADR-010) + Rate Limit Redis (ADR-019) ────────────────
	// 抽离到 wire_cache.go — 两个都 fail-open，不会 panic。
	sessionCache, sessionCacheCloser := wireCacheDeps()

	// ── DB Driver + Dialect + UoW 构造 ─────────────────────────────────────
	// 抽离到 wire_db.go — 返回 (unitOfWork, error)。
	unitOfWork, err := wireDeps(pool, mysqlDB, driver, dialectHint, sessionCache)
	if err != nil {
		return nil, err
	}

	// ── 分区容量探测（ADR-011 / OPEN-API-40）────────────────────────────
	// 只读探测：真实查询 pg_total_relation_size/pg_indexes_size，按声明策略
	// 评估 ADR-011 出口条件；不修改 schema，不启用分区。满足条件时输出告警级日志，
	// 供运维按表启动迁移评估。
	if pool != nil {
		partitioningAdvisor := platformpartitioning.NewAdvisor(
			platformpartitioning.NewStubStrategy(),
			pgpartitioning.NewProber(pool),
			slog.Default(),
		)
		for _, decision := range partitioningAdvisor.Evaluate(systemCtx) {
			if decision.ShouldPartition {
				slog.Warn("partitioning exit conditions met", "table", decision.Table, "reason", decision.Reason,
					"total_bytes", decision.Size.TotalBytes, "index_bytes", decision.Size.IndexBytes)
				continue
			}
			slog.Debug("partitioning advisory", "table", decision.Table, "reason", decision.Reason,
				"total_bytes", decision.Size.TotalBytes, "index_bytes", decision.Size.IndexBytes)
		}
	}

	// ── Outbox (ADR-008) + Idempotency (ADR-018) ─────────────────────────
	// 抽离到 wire_outbox.go — outboxWorker Start() 在函数内部。
	// outboxRepo 作为 domainoutbox.Repository 传给 AI Gateway 等模块写入审计事件。
	outbox := wireOutboxDeps(workersCtx, pool, mysqlDB, driver, unitOfWork)
	outboxWorker := outbox.OutboxWorker
	idempotencyRepo := outbox.Idempotency
	outboxRepo := outbox.OutboxRepo
	ssfSignerHolder := outbox.SSFSigner
	auditChainWorker := outbox.AuditChainWorker

	// ── 领域 service 装配（wire_domains.go）──
	// 返回完整 AppServices；clock/uuid 在本函数构造并供后续 wire_*.go 复用
	services, infraDeps := wireDomains(mysqlDB, driver, pool, unitOfWork, outboxRepo, workersCtx, sessionCache)
	casbinManager := infraDeps.Casbin
	storageProvider := infraDeps.Storage
	principalResolver := infraDeps.PrincipalResolver
	scopeAuthorizer := infraDeps.ScopeAuthorizer
	aigatewayProbeWorker := infraDeps.ProbeWorker
	clockProvider := clock.SystemClock{}
	uuidGenerator := platformuuid.V7Generator{}

	// ── ReBAC services（已在 wire_rbac_services.go）──
	rbacSvc := wireRbacServices(unitOfWork, clockProvider, uuidGenerator)
	rebacRepo := rbacSvc.RebacRepo
	rebacEngine := rbacSvc.RebacEngine
	rebacService := rbacSvc.RebacService
	riskService := rbacSvc.Risk
	actionsService := rbacSvc.Actions
	meteringService := rbacSvc.Metering
	igaService := rbacSvc.IGA
	workloadService := rbacSvc.Workload
	webhookeventsService := rbacSvc.WebhookEvents
	idvStoreService := rbacSvc.Idv
	wireIDV(&services, &idvStoreService)
	igaExpireWorker := startIGAExpireWorker(workersCtx, unitOfWork)
	// 到期提醒：邀请/凭据/应用客户端证书临期时写入管理员个人收件箱并按偏好扇出。
	expiryReminderWorker := startExpiryReminderWorker(workersCtx, unitOfWork, services.UserNotificationService)

	// ── Risk → auth pre-session evaluation gate（AC-RISK-001）────────────
	// risk.Service.EvaluateLogin satisfies domain/risk.LoginRiskEvaluator;
	// auth.Service skips the gate when RiskEvaluator is nil (backward-compat).
	services.AuthService.RiskEvaluator = &riskService

	// ── ADR-029 同步扩展点 + ADR-030 JML（BE-D-11）─────────────────────
	// WASM 出站默认 NoopProxy（fail-closed 禁出站）；运维需要出站时换
	// WhitelistProxy。JML 仅在 users 仓储具备角色绑定能力时注入
	// （postgres；MySQL 镜像跳过，OPEN-API-37）。
	services.AuthService.Actions = actionsapp.NewDispatcher(
		actionsService, actionsapp.NewWASMExecutor(actionsapp.NoopProxy{}), nil)
	services.UserService.JML = jmlLifecycleFor(unitOfWork, clockProvider)

	// ── Risk governance → actual session kill + SSF outbox（AC-RISK-002）
	// risk.Service.TransitionGovernance(action=revoke) cascades to
	// sessions.Service.RevokeSession when SessionRevoker is wired up.
	// sessions.Service.RevokeSession writes its own SSF session-revoked event
	// + audit in its own transaction — same-tenant RLS is enforced there.
	riskService.SessionRevoker = &services.SessionService

	// ── Session Cleanup（已在 wire_session_cleanup.go）──
	sessionCleanupStopCh := startSessionCleanup(services.AuthService, services.SessionService, idempotencyRepo)

	// ── Commerce domain workers + ldapSync（已在 wire_commerce_workers.go）──
	commerce := wireCommerceWorkers(pool, unitOfWork, clockProvider, uuidGenerator, workersCtx)
	meteringAggregateWorker := commerce.MeteringAggregate
	paymentsReconService := commerce.PaymentsRecon
	paymentsChannelConfigService := commerce.PaymentsChannelConfig
	paymentsAttemptsRepo := commerce.PaymentsAttempts
	paymentsRefundsRepo := commerce.PaymentsRefunds
	paymentsCallbackReceiptsRepo := commerce.PaymentsCallbackReceipts
	ldapSyncService := commerce.LDAPSync
	providerConsistencyWorker := commerce.ProviderConsistency

	// ── Auth/Security infrastructure（已在 wire_auth_security.go）─────────
	authDeps, err := wireAuthSecurity(
		workersCtx,
		&services.AuthService, ssfSignerHolder, corsConfig, casbinManager,
		driver, pool, mysqlDB, unitOfWork, clockProvider, uuidGenerator, sessionCache,
	)
	if err != nil {
		return nil, err
	}
	wafMiddleware := authDeps.WAF
	ipRulesMiddleware := authDeps.IPRules
	jwtIssuer := authDeps.JWTIssuer
	jwksProvider := authDeps.JWKS
	enforcerService := authDeps.Enforcer
	verificationService := authDeps.Verification
	// 用户详情页的「当前联系方式验证」复用同一条真实发送/校验链路；
	// 适配器放在 bootstrap，避免 app/users 直接 import app/verifications。
	services.UserService.ContactVerification = newContactVerificationAdapter(&verificationService)
	captchaFactory := authDeps.Captcha
	notificationFactory := authDeps.Notification
	notificationWorker := authDeps.NotificationWorker
	senderFactory := authDeps.Sender
	federationJWKSCache := authDeps.RemoteJWKS
	// Provider writes invalidate the messaging resolution cache in this process;
	// other instances converge through its TTL.
	if authDeps.ProviderCache != nil && services.ProviderService.Unified != nil {
		services.ProviderService.Unified.CacheInvalidator = authDeps.ProviderCache
	}

	// ── Transport handlers（wire_transport.go）──
	handlers := wireTransport(
		pool, services, &services.AuthService,
		jwtIssuer, jwksProvider, notificationFactory,
		// NewServer 返回 *infra.Server，其方法集现在与 domainradius.Server 接口完全对齐
		// （SetAuthHandler/Start/Stop 签名一致），加一层 adapter 让函数类型协变通过。
		func(addr, secret string) domainradius.Server { return infraradius.NewServer(addr, secret) },
		enforcerService, verificationService,
		ldapSyncService,
		paymentsReconService, paymentsChannelConfigService,
		paymentsAttemptsRepo, paymentsRefundsRepo, paymentsCallbackReceiptsRepo,
		rebacRepo, rebacEngine, rebacService, uuidGenerator,
		riskService, actionsService, meteringService, igaService, workloadService, webhookeventsService, idvStoreService,
		scopeAuthorizer, unitOfWork,
	)

	denialAudit := appauditdenial.NewRecorder(unitOfWork, clockProvider, uuidGenerator)
	if provider, ok := sessionCache.(interface{ RedisClient() *redis.Client }); ok {
		denialAudit.Deduper = appauditdenial.NewRedisDeduper(provider.RedisClient(), denialAudit.Deduper)
	}

	return &Runtime{
		DB: DBConfig{
			Pool:      pool,
			Driver:    driver,
			MySQLPool: mysqlDB,
		},
		Services: services,
		Handlers: handlers,
		Infra: InfraConfig{
			StorageProvider:     storageProvider,
			JWKSManager:         jwksProvider,
			CasbinManager:       casbinManager,
			RebacEngine:         rebacEngine,
			JWTIssuer:           jwtIssuer,
			SenderFactory:       senderFactory,
			CaptchaFactory:      captchaFactory,
			NotificationFactory: notificationFactory,
			I18nTranslator:      platformi18n.NewTranslator(),
			PrincipalResolver:   principalResolver,
			ScopeAuthorizer:     scopeAuthorizer,
		},
		Middleware: MiddlewareConfig{
			WAFMiddleware:     wafMiddleware,
			IPRulesMiddleware: ipRulesMiddleware,
			// 服务级 per-IP 限流必须走 LoadConfigFromEnv：直接传 DefaultServiceIPLimitConfigs
			// 会让 RATE_LIMIT_PER_IP_REQUESTS/WINDOW 这两个已文档化的旋钮永远不生效。
			ServiceIPLimiter:    newServiceIPLimiterFromEnv(),
			CORSConfig:          corsConfig,
			RegionResolver:      regionResolverFromPool(pool),
			RegionResidencyMode: httpmiddleware.NormalizeRegionResidencyMode(os.Getenv("REGION_RESIDENCY_MODE")),
			OrganizationSlugResolver: func(ctx context.Context, slug string) (uuid.UUID, error) {
				// GetBySlugPublic 是既有的组织仓储查询：transaction-local
				// super_admin 仅用于匿名 by-slug/resolve；此处只取 ID，
				// 不读取任何凭据字段。
				organization, err := unitOfWork.Organizations().GetBySlugPublic(ctx, slug)
				if err != nil {
					return uuid.Nil, err
				}
				return uuid.Parse(organization.ID)
			},
			PasswordGate: authtransport.NewPasswordGate(
				&services.AuthService,
				&services.AuthService,
				config.PasswordChangeEnforcement(),
				clock.SystemClock{},
			),
			DenialAudit: denialAudit,
		},
		Workers: BackgroundWorkers{
			OutboxWorker:        outboxWorker,
			NotificationWorker:  notificationWorker,
			IdempotencyRepo:     idempotencyRepo,
			AIGatewayProbe:      aigatewayProbeWorker,
			MeteringAggregate:   meteringAggregateWorker,
			AuditChain:          auditChainWorker,
			IGAExpire:           igaExpireWorker,
			ProviderConsistency: providerConsistencyWorker,
			ExpiryReminder:      expiryReminderWorker,
		},
		Lifecycle: LifecycleConfig{
			SessionCleanupStopCh: sessionCleanupStopCh,
			ShutdownTelemetry:    shutdownTelemetry,
			SessionCacheCloser:   sessionCacheCloser,
			WorkersCancel:        workersCancel,
			FederationJWKSCache:  federationJWKSCache,
		},
	}, nil
}

// driverNormalize: 把 DB_DRIVER env 值规范化为 runtime 内部的 driver 字符串。
// 返回 (normalized, wireProtocol) — normalized 是内部 driver（pgx/mysql），wireProtocol
// 决定走哪个 newRuntimeXxx 底层函数。

// mcpToolExecutor adapts app/mcpservers.MCPProxy to domain/aigateway.ToolExecutor.
// Lives here (composition root) only — app/aigateway layer stays unaware of
// mcpservers, depending purely on the ToolExecutor Port interface.
type mcpToolExecutor struct {
	mcp appmcpservers.MCPProxy
}

// ExecuteTool routes the tool_use call through MCPProxy.CallTool using the
// MCP server target from the binding. binding may be nil (skipped in that case).
func (e *mcpToolExecutor) ExecuteTool(ctx context.Context, binding *aigatewaydomain.MCPBinding, _ string, input map[string]any) (any, error) {
	if binding == nil {
		return nil, nil
	}
	result, err := e.mcp.CallTool(ctx, binding.McpServerID, binding.ToolName, input)
	if err != nil {
		// Hand the gateway a typed, fixed-message error: its tool_result block
		// is relayed to the model, so upstream URL/query/header/body text must
		// not survive. errors.Is/As still reach the original cause.
		return nil, appmcpservers.SanitizeToolError(err)
	}
	return result, nil
}

// buildReconciliationService wires the PostgreSQL report store. MySQL runtime
// has no pool; the service stays available but reads fail closed with 503.
func buildReconciliationService(pool *pgxpool.Pool) appreconciliation.Service {
	if pool == nil {
		return appreconciliation.NewService(nil)
	}
	return appreconciliation.NewService(postgresscim.NewReconciliationReportRepository(pool))
}

// newServiceIPLimiterFromEnv 按 RATE_LIMIT_PER_IP_* 构建服务级 per-IP 限流器。
//
// 未设置环境变量时 LoadConfigFromEnv 的默认值与 DefaultServiceIPLimitConfigs
// 一致（登录 10/min、注册 5/min、验证码 3/min、邮件/短信 1/h），因此默认行为不变。
func newServiceIPLimiterFromEnv() *httpmiddleware.ServiceIPLimiter {
	envConfig := httpmiddleware.LoadConfigFromEnv()
	return httpmiddleware.NewServiceIPLimiter(envConfig.Configs, envConfig.CleanupInterval)
}

// regionResolverFromPool builds the cross-region resolver from the runtime
// PostgreSQL pool. MySQL runtime is deployment-blocked (OPEN-API-37), so a nil
// pool means "not verifiable" and the middleware fails closed only in enforce.
func regionResolverFromPool(pool *pgxpool.Pool) httpmiddleware.RegionResolver {
	if pool == nil {
		return nil
	}
	return postgresregions.NewRepository(pool)
}
