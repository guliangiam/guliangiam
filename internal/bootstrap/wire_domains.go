// wire_domains.go — 领域 service 装配：所有 NewXXXService + adapter 注入 + aigateway 系列 + scope/kerberos.
// 从 runtime.go NewRuntime L408-550 抽离，保持 bootstrap 包级 private function。
//
// 注意: 包含 authService 构造 + 所有 adapter 注入（social/ldap/faceid + verification/MFA/token family + kerberos）。
// wireRbacServices / wireSessionCleanup / wireCommerceWorkers / wireAuthSecurity 调用留在 NewRuntime 骨架。
package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	appadapters "github.com/guliangiam/guliangiam/internal/app/adapters"
	appagents "github.com/guliangiam/guliangiam/internal/app/agents"
	appaigateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	appapplications "github.com/guliangiam/guliangiam/internal/app/applications"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	appcaptchaproviders "github.com/guliangiam/guliangiam/internal/app/captchaproviders"
	appcerts "github.com/guliangiam/guliangiam/internal/app/certs"
	appcoupons "github.com/guliangiam/guliangiam/internal/app/coupons"
	appdashboard "github.com/guliangiam/guliangiam/internal/app/dashboard"
	appentries "github.com/guliangiam/guliangiam/internal/app/entries"
	appfaceidproviders "github.com/guliangiam/guliangiam/internal/app/faceidproviders"
	appforms "github.com/guliangiam/guliangiam/internal/app/forms"
	appgateway "github.com/guliangiam/guliangiam/internal/app/gateway"
	appgroups "github.com/guliangiam/guliangiam/internal/app/groups"
	appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	appldaps "github.com/guliangiam/guliangiam/internal/app/ldaps"
	appmcpservers "github.com/guliangiam/guliangiam/internal/app/mcpservers"
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
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
	appusers "github.com/guliangiam/guliangiam/internal/app/users"
	appwebhooks "github.com/guliangiam/guliangiam/internal/app/webhooks"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	infraadaptertest "github.com/guliangiam/guliangiam/internal/infra/adaptertest"
	infrajwksuri "github.com/guliangiam/guliangiam/internal/infra/jwksuri"
	postgresorganizations "github.com/guliangiam/guliangiam/internal/infra/postgres/organizations"
	postgresregions "github.com/guliangiam/guliangiam/internal/infra/postgres/regions"

	auditdomain "github.com/guliangiam/guliangiam/internal/domain/authorization_audit"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	"github.com/guliangiam/guliangiam/internal/infra/cache"
	"github.com/guliangiam/guliangiam/internal/infra/casbin"
	"github.com/guliangiam/guliangiam/internal/infra/cert"
	infrakerberos "github.com/guliangiam/guliangiam/internal/infra/kerberos"
	infraldap "github.com/guliangiam/guliangiam/internal/infra/ldap"
	mysqlauditrepo "github.com/guliangiam/guliangiam/internal/infra/mysql/authorization_audit"
	mysqlcasbin "github.com/guliangiam/guliangiam/internal/infra/mysql/casbin"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	auditrepo "github.com/guliangiam/guliangiam/internal/infra/postgres/authorization_audit"
	infradashboard "github.com/guliangiam/guliangiam/internal/infra/postgres/dashboard"
	"github.com/guliangiam/guliangiam/internal/infra/storage"
	infrasyncer "github.com/guliangiam/guliangiam/internal/infra/syncer"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// wireDomains 构造所有领域层 service + adapter 注入 + aigateway 系列 + scope/kerberos。
//
// 参数:
//   - pool          *pgxpool.Pool     — Dashboard/AIGateway 等直接依赖 pgx 连接
//   - unitOfWork    appuow.UnitOfWork — 所有 NewXXXService 的 UoW 依赖
//   - outboxRepo    *infraoutbox.Repository — authService.Outbox + aigatewayService 审计事件
//   - workersCtx    context.Context    — aigatewayProbeWorker.Start(ctx)
//
// 返回:
//   - AppServices     — 所有 domain service 实例（不含 verification/enforcer）
//   - domainInfraDeps — Casbin/Storage/PrincipalResolver/ScopeAuthorizer/AIGatewayProbe
//
// clock/uuid 由 Runtime 自行构造并复用给后续 wire_*.go，不再作为本函数返回值。
type domainInfraDeps struct {
	Casbin            *casbin.Manager
	Storage           storage.Provider
	PrincipalResolver scopeauthorizer.PrincipalTypeResolver
	ScopeAuthorizer   scopeauthorizer.ScopeAuthorizer
	ProbeWorker       *appaigateway.HealthProbeWorker
}

func wireDomains(
	mysqlDB *sql.DB,
	driver string,
	pool *pgxpool.Pool,
	unitOfWork appuow.UnitOfWork,
	outboxRepo domainoutbox.Repository,
	workersCtx context.Context,
	sessionCache cache.SessionCache,
) (AppServices, domainInfraDeps) {
	clockProvider := clock.SystemClock{}
	uuidGenerator := platformuuid.V7Generator{}

	authService := appauth.NewServiceWithDependencies(clockProvider, uuidGenerator, unitOfWork)
	// Remote client JWKS (jwks_uri) is fetched through the SSRF-safe client.
	authService.ClientJWKSFetcher = infrajwksuri.NewFetcher(nil)

	// Wire social login adapters: connect the app layer to the IDP infrastructure
	// without creating a direct import dependency (app layer must not import infra).
	// The SAML adapter receives the replay store so consumed Response IDs are
	// rejected across replicas (Redis) or within the process (fallback).
	samlReplayStore := newSAMLReplayStore(sessionCache)
	authService.SAMLReplayStore = samlReplayStore
	// Client assertion `jti` replay protection shares the same Redis/process-local
	// store: a jti consumed on one replica must be rejected on every replica.
	authService.ClientAssertionReplayStore = samlReplayStore
	appauth.SetSocialLoginAdapters(exchangeSocialCodeWithIDP(samlReplayStore), getSocialRedirectURLWithIDP)

	// Wire LDAP login adapter: connect the app layer to the LDAP client infrastructure.
	appauth.SetLdapLoginAdapter(ldapBindAndSearch)

	// Wire Face ID adapters: connect the app layer to the Face ID verifier infrastructure.
	appauth.SetFaceIdAdapters(verifyFaceIdLiveness, compareFaceIdWithTemplate)

	certService := appcerts.NewService(clockProvider, uuidGenerator, unitOfWork)
	captchaProviderService := appcaptchaproviders.NewService(clockProvider, uuidGenerator, unitOfWork)
	notificationProviderService := appnotificationproviders.NewService(clockProvider, uuidGenerator, unitOfWork)
	notificationService := appnotifications.NewService(clockProvider, uuidGenerator, unitOfWork)
	userNotificationUOW, ok := unitOfWork.(appuow.UserNotificationsUoW)
	if !ok {
		panic("unit of work does not implement UserNotificationsUoW")
	}
	userNotificationService := appusernotifications.NewService(clockProvider, uuidGenerator, userNotificationUOW)
	// 多渠道路由：解析用户邮箱/手机/渠道绑定，供 Publish 自动扇出。
	userNotificationService.Contacts = notificationContactResolver{uow: userNotificationUOW}
	authService.Notifications = newAuthInboxNotifier(userNotificationService)
	// If ACME is enabled, create the ACME renewer and attach it to the cert service.
	if cert.ACMEEnabled() {
		acmeEmail := envOrEmpty("ACME_EMAIL")
		acmeStaging := envOrEmpty("ACME_STAGING") == valueTrue
		acmeRenewer := cert.NewACMERenewer(acmeEmail, acmeStaging)
		certService = appcerts.NewServiceWithACME(clockProvider, uuidGenerator, unitOfWork, acmeRenewer)
	}
	casbinManager := casbin.NewManager(newCasbinPolicyStore(pool, mysqlDB, driver))
	adapterService := appadapters.NewService(clockProvider, uuidGenerator, unitOfWork, casbinManager)
	// BE-B-16：数据库连接探测下沉 infra/adaptertest（DSN 转义 + SSRF 防护 +
	// mssql 驱动边界）。不注入时 TestConnection 只会报 "not configured"。
	adapterService.ConnectionTester = infraadaptertest.New()
	modelService := appmodels.NewService(clockProvider, uuidGenerator, unitOfWork)
	syncerService := appsyncers.NewServiceWithFactory(clockProvider, uuidGenerator, unitOfWork, infrasyncer.NewFactoryAdapter())
	// BE-B-21（DB-061）：多副本下同一 syncer 的并发同步必须跨进程互斥。
	// MySQL 分支 pool=nil 时保留进程内守卫（OPEN-API-36/37 边界）。
	if pool != nil {
		syncerService = syncerService.WithLockProvider(infrapg.NewAdvisoryLockProvider(pool))
	}
	webhookService := appwebhooks.NewService(clockProvider, uuidGenerator, unitOfWork)

	authService.Outbox = outboxRepo

	ldapService := appldaps.NewService(clockProvider, uuidGenerator, unitOfWork)
	ldapService.LDAPClient = infraldap.NewClient()
	ldapService.LDAPSyncer = infraldap.NewSyncer()
	agentService := appagents.NewService(clockProvider, uuidGenerator, unitOfWork)
	formService := appforms.NewService(clockProvider, uuidGenerator, unitOfWork)
	applicationService := appapplications.NewService(clockProvider, uuidGenerator, unitOfWork)
	gatewayService := appgateway.NewService(clockProvider, uuidGenerator, unitOfWork)
	groupService := appgroups.NewService(clockProvider, uuidGenerator, unitOfWork)
	invitationService := appinvitations.NewService(clockProvider, uuidGenerator, unitOfWork)
	organizationService := apporganizations.NewService(clockProvider, uuidGenerator, unitOfWork)
	if driver != driverMySQL && pool != nil {
		organizationService.RegionLister = postgresregions.NewRepository(pool)
		organizationService.PolicyInheritance = postgresorganizations.NewRepository(pool)
	}

	permissionService := apppermissions.NewService(clockProvider, uuidGenerator, unitOfWork, casbinManager)
	// 权限/策略写成功后清除账户权限缓存（DEF-16）。
	permissionService.CacheInvalidator = &authService

	orderService := apporders.NewService(clockProvider, uuidGenerator, unitOfWork)

	// Wire payment gateway adapters: connect the app layer to the payment
	// infrastructure without creating a direct import dependency (app layer must not import infra).
	apporders.SetPaymentAdapters(createPaymentWithGateway, queryPaymentWithGateway)

	paymentService := apppayments.NewService(clockProvider, uuidGenerator, unitOfWork)
	planService := appplans.NewService(clockProvider, uuidGenerator, unitOfWork)
	platformAdminService := appplatformadmins.NewService(clockProvider, uuidGenerator, unitOfWork, slog.Default())
	pricingService := apppricings.NewService(clockProvider, uuidGenerator, unitOfWork)
	productService := appproducts.NewService(clockProvider, uuidGenerator, unitOfWork)
	providerService := appproviders.NewService(clockProvider, uuidGenerator, unitOfWork)
	recordService := apprecords.NewService(clockProvider, uuidGenerator, unitOfWork)

	storageProvider := buildStorageProvider()

	resourceService := appresources.NewService(clockProvider, uuidGenerator, unitOfWork, storageProvider,
		storage.NewProviderResolver(unitOfWork.Providers()))
	roleService := approles.NewService(clockProvider, uuidGenerator, unitOfWork, casbinManager)
	sessionService := appsessions.NewService(clockProvider, unitOfWork)

	// Phase C-Step 1/2/4 + Phase D-Step 3: ScopeAuthorizer + PrincipalTypeResolver + AuditWriter
	// （Service 层 + handlers + infra 都依赖 — 必须在所有 Service.NewService 之前构造）
	principalResolver := scopeauthorizer.NewCachedDBResolver(unitOfWork.PlatformAdmins(), time.Now)
	// OPEN-API-55: deactivation invalidates the principal cache and revokes sessions.
	platformAdminService.PrincipalCache = principalResolver
	var authorizationAuditRepo auditdomain.AuditRepository
	switch {
	case driver == driverMySQL && mysqlDB != nil:
		authorizationAuditRepo = mysqlauditrepo.NewRepository(mysqlDB)
	case pool != nil:
		authorizationAuditRepo = auditrepo.NewRepository(pool)
	}
	auditWriter := scopeauthorizer.NewAuditWriter(authorizationAuditRepo, slog.Default())
	// worker/system 旁路集中审计（OPEN-API-42 ③ 裁决）：所有后台 worker 共享同一个
	// TenantFilterSystem 进程级作用域，逐任务写 records 会产生海量噪声；改为在启动时
	// 记录一次「system 作用域建立」事件（actor=system，来源/目标=built-in），并保留
	// worker 的结构化 slog 日志用于任务级追溯。
	builtInOrg := uuid.MustParse("01a0ea76-0c00-7000-8000-000000000001")
	auditWriter.Append(workersCtx, scopeauthorizer.AuditRecord{
		ActorID:      uuid.Nil,
		ActorType:    scopeauthorizer.PrincipalSystem,
		SourceTenant: builtInOrg,
		TargetTenant: builtInOrg,
		ResourceType: "worker",
		Action:       "worker.system_scope",
		RiskLevel:    scopeauthorizer.RiskMedium,
		Decision:     "allow",
		Reason:       "worker_context_uses_tenant_filter_system",
	})
	scopeAuthorizer := scopeauthorizer.NewAllowIfScopedScopeAuthorizer(
		unitOfWork.PlatformAdmins(),
		unitOfWork.PlatformAdminScopes(),
		unitOfWork.CrossTenantOps(),
		slog.Default(), auditWriter,
	)

	subscriptionService := appsubscriptions.NewService(clockProvider, uuidGenerator, unitOfWork)
	ticketService := apptickets.NewService(clockProvider, uuidGenerator, unitOfWork)
	ticketService.Notifier = newTicketInboxNotifier(unitOfWork, userNotificationService)
	tokenService := apptokens.NewService(clockProvider, uuidGenerator, unitOfWork)
	// OAuth refresh_token grant: auth service delegates rotation to app/tokens.
	authService.RefreshTokens = newRefreshTokenExchanger(tokenService)
	userService := appusers.NewService(clockProvider, uuidGenerator, unitOfWork, scopeAuthorizer)
	// 管理员重置口令后向被重置用户发送安全通知（收件箱 + 偏好驱动的外部渠道）。
	userService.Notifier = newUsersInboxNotifier(userNotificationService)
	// Face ID enrollment: the adapter only runs the provider liveness check and
	// mints an enrollment id; the users service seals the captured image into
	// user_face_templates (encrypted reference), never plaintext resource storage.
	userService.FaceIDEnrollment = faceIDEnrollmentAdapter{
		UOW:   unitOfWork,
		Clock: clockProvider,
		UUID:  uuidGenerator,
	}
	// User role bindings are persisted in user_role_bindings, but authorization
	// evaluates Casbin `g(user, role, org)` rows; project every binding write
	// into the policy store (grant and revoke).
	userService.RolePolicies = casbinManager
	// Wire verification code adapters: connect the auth layer to the verifications repository.
	appauth.WireVerificationAdapters(func() verificationsdomain.Repository {
		return unitOfWork.Verifications()
	})
	// Wire MFA setup verification to use the verifications repository.
	appusers.WireMfaVerification(func() verificationsdomain.Repository {
		return unitOfWork.Verifications()
	})
	// Wire token family revocation for OAuth 2.1 token reuse detection.
	appauth.WireTokenFamilyRevocation(func() appuow.UnitOfWork {
		return unitOfWork
	})
	// Wire Kerberos SPNEGO authenticator for cryptographic token verification.
	kerberosRealm := envOrEmpty("KRB5_REALM")
	kerberosKeytab := envOrEmpty("KRB5_KEYTAB_PATH")
	kerberosServiceName := envOrEmpty("KRB5_SERVICE_NAME")
	if kerberosServiceName == "" {
		kerberosServiceName = "HTTP"
	}
	kerberosAuth := infrakerberos.NewAuthenticator(kerberosRealm, kerberosKeytab, kerberosServiceName)
	if kerberosAuth.IsReady() {
		appauth.WireKerberosAuthenticator(func(authHeader string) (string, string, error) {
			principal, err := kerberosAuth.ValidateSPNEGO(authHeader)
			if err != nil {
				return "", "", err
			}
			return principal.Username, principal.Realm, nil
		})
	}
	couponService := appcoupons.NewService(clockProvider, uuidGenerator, unitOfWork)
	entryService := appentries.NewService(clockProvider, uuidGenerator, unitOfWork)
	mcpServerService := appmcpservers.NewService(clockProvider, uuidGenerator, unitOfWork)
	radiusAccountingService := appradiusaccountings.NewService(clockProvider, uuidGenerator, unitOfWork)
	// MySQL 分支 pool=nil：dashboard 仓储只有 PostgreSQL 实现，直接注入会在
	// 首个请求时 nil-pointer panic。按 driver 选择「能力不可用」实现（503）。
	dashboardStatsRepo := appdashboard.StatsRepository(infradashboard.NewRepository(pool))
	if pool == nil {
		dashboardStatsRepo = unavailableDashboardRepository{}
	}
	dashboardService := appdashboard.NewService(dashboardStatsRepo)
	b2bService := appb2b.NewService(clockProvider, uuidGenerator, unitOfWork)
	faceIdProviderService := appfaceidproviders.NewService(clockProvider, uuidGenerator, unitOfWork)
	ruleService := apprules.NewService(clockProvider, uuidGenerator, unitOfWork)
	radiusConfigService := appradiusconfigs.NewService(clockProvider, uuidGenerator, unitOfWork)
	transactionService := apptransactions.NewService(clockProvider, uuidGenerator, unitOfWork)
	keyService := appkeys.NewService(clockProvider, uuidGenerator, unitOfWork)
	searchService := appsearch.NewService(unitOfWork)
	aigatewayService := NewAIGatewayService(pool, wireQuotaMeter(), outboxRepo)
	// Wire ToolExecutor: adapter wraps mcpservers.Service's MCPProxy.CallTool
	// into domain.ToolExecutor Port (Clean Architecture — composition root only
	// knows both, app layer remains unaware of mcpservers).
	aigatewayService.ToolExecutor = &mcpToolExecutor{mcp: mcpServerService.MCP}
	aigatewayAdminService := NewAIGatewayAdminService(pool)
	aigatewayProbeWorker := NewHealthProbeWorker(infrapg.NewInstrumentedDBTX(pool), outboxRepo, slog.Default())
	aigatewayProbeWorker.Start(workersCtx)

	return AppServices{
		CertService:                 certService,
		CaptchaProviderService:      captchaProviderService,
		NotificationProviderService: notificationProviderService,
		NotificationService:         notificationService,
		UserNotificationService:     userNotificationService,
		AdapterService:              adapterService,
		ModelService:                modelService,
		SyncerService:               syncerService,
		WebhookService:              webhookService,
		LdapService:                 ldapService,
		AgentService:                agentService,
		FormService:                 formService,
		AuthService:                 authService,
		ApplicationService:          applicationService,
		GatewayService:              gatewayService,
		GroupService:                groupService,
		InvitationService:           invitationService,
		OrganizationService:         organizationService,
		PermissionService:           permissionService,
		OrderService:                orderService,
		PaymentService:              paymentService,
		PlanService:                 planService,
		PlatformAdminService:        platformAdminService,
		PricingService:              pricingService,
		ProductService:              productService,
		ProviderService:             providerService,
		RecordService:               recordService,
		ResourceService:             resourceService,
		RoleService:                 roleService,
		SessionService:              sessionService,
		SubscriptionService:         subscriptionService,
		TicketService:               ticketService,
		TokenService:                tokenService,
		UserService:                 userService,
		CouponService:               couponService,
		EntryService:                entryService,
		McpServerService:            mcpServerService,
		RadiusAccountingService:     radiusAccountingService,
		DashboardService:            dashboardService,
		B2BService:                  b2bService,
		FaceIdProviderService:       faceIdProviderService,
		RuleService:                 ruleService,
		RadiusConfigService:         radiusConfigService,
		TransactionService:          transactionService,
		KeyService:                  keyService,
		SearchService:               searchService,
		AIGateway:                   aigatewayService,
		AIGatewayAdmin:              aigatewayAdminService,
	}, domainInfraDeps{
		Casbin:            casbinManager,
		Storage:           storageProvider,
		PrincipalResolver: principalResolver,
		ScopeAuthorizer:   scopeAuthorizer,
		ProbeWorker:       aigatewayProbeWorker,
	}
}

// newCasbinPolicyStore 按 driver 选择 Casbin 策略存储：MySQL 走方言实现，
// 其余（PostgreSQL 族）走 pgx 适配器。Manager 只依赖 PolicyStore 端口。
func newCasbinPolicyStore(pool *pgxpool.Pool, mysqlDB *sql.DB, driver string) casbin.PolicyStore {
	if driver == driverMySQL && mysqlDB != nil {
		return mysqlcasbin.NewStore(mysqlDB)
	}
	return casbin.NewAdapter(pool)
}
