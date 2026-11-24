// wire_transport.go — TransportHandlers 构造：所有 HTTP handler 初始化 + wellknown/cas/saml/webauthn/kerberos 组装。
// 从 runtime.go NewRuntime L579-597（handler 组装）+ L657-733（TransportHandlers 构造）抽离。
// 接收所有依赖后返回完整的 TransportHandlers 结构体。
package bootstrap

import (
	"github.com/jackc/pgx/v5/pgxpool"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appenforcers "github.com/guliangiam/guliangiam/internal/app/enforcers"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appplatformadmins "github.com/guliangiam/guliangiam/internal/app/platform_admins"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"

	actionsapp "github.com/guliangiam/guliangiam/internal/app/actions"
	idvapp "github.com/guliangiam/guliangiam/internal/app/idv"
	igaapp "github.com/guliangiam/guliangiam/internal/app/iga"
	meteringapp "github.com/guliangiam/guliangiam/internal/app/metering"
	apprebac "github.com/guliangiam/guliangiam/internal/app/rebac"
	riskapp "github.com/guliangiam/guliangiam/internal/app/risk"
	appwebhookevents "github.com/guliangiam/guliangiam/internal/app/webhookevents"
	workloadapp "github.com/guliangiam/guliangiam/internal/app/workload"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	domainnotifications "github.com/guliangiam/guliangiam/internal/domain/notifications"
	domainradius "github.com/guliangiam/guliangiam/internal/domain/radius"

	appprivacy "github.com/guliangiam/guliangiam/internal/app/privacy"
	infrapayment "github.com/guliangiam/guliangiam/internal/infra/payment"
	infrapgpayments "github.com/guliangiam/guliangiam/internal/infra/postgres/payments"
	privacytransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/privacy"

	appmessages "github.com/guliangiam/guliangiam/internal/app/messages"
	appradius "github.com/guliangiam/guliangiam/internal/app/radius"

	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/i18n"
	"github.com/guliangiam/guliangiam/internal/platform/scopeauthorizer"
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

	"github.com/guliangiam/guliangiam/internal/infra/jwt"
)

// wireTransport 构造所有 HTTP transport handler + 组装 wellknown/cas/saml/webauthn/kerberos handler。
// 接收完整的 AppServices + wireAuthSecurity 产物 + wireCommerceWorkers 产物 + wireRbacServices 产物。
func wireTransport(
	pool *pgxpool.Pool,
	services AppServices,
	authServicePtr *appauth.Service,
	jwtIssuer *jwt.Issuer,
	jwksProvider domainauth.JWKSProvider,
	notificationFactory domainnotifications.NotificationFactory,
	radiusFactory domainradius.ServerFactory,
	enforcerService appenforcers.Service,
	verificationService appverifications.Service,
	ldapSyncService *appldapsync.Service,
	paymentsReconService apppayments.ReconService,
	paymentsChannelConfigService apppayments.ChannelConfigService,
	paymentsAttemptsRepo infrapgpayments.AttemptsRepository,
	paymentsRefundsRepo infrapgpayments.RefundsRepository,
	paymentsCallbackReceiptsRepo infrapgpayments.CallbackReceiptsRepository,
	rebacRepo apprebac.Repository,
	rebacEngine *apprebac.Engine,
	rebacService *apprebac.Service,
	uuidGenerator platformuuid.Generator,
	riskService riskapp.Service,
	actionsService actionsapp.Service,
	meteringService meteringapp.Service,
	igaService igaapp.Service,
	workloadService workloadapp.Service,
	webhookeventsService appwebhookevents.Service,
	idvStoreService idvapp.Service,
	scopeAuthorizer scopeauthorizer.ScopeAuthorizer,
	unitOfWork appuow.UnitOfWork,
) TransportHandlers {
	wellknownHandler := wellknowntransport.Handler{
		JWKSManager:        appkeys.NewJWKSViewProvider(jwksProvider),
		UserInfoFetch:      wellknowntransport.UserInfoAdapter{Service: authServicePtr},
		SAMLMetadataFetch:  wellknowntransport.SAMLMetadataAdapter{Service: authServicePtr},
		OIDCDiscoveryFetch: wellknowntransport.OIDCDiscoveryAdapter{Service: authServicePtr},
		KeyService:         &services.KeyService,
	}
	casHandler := &castransport.Handler{
		AuthService: authServicePtr,
	}
	samlHandler := &samltransport.Handler{
		AuthService: authServicePtr,
	}
	webauthnHandler := &webauthntransport.Handler{
		AuthService: authServicePtr,
	}
	kerberosHandler := &kerberustransport.Handler{
		AuthService: authServicePtr,
	}

	// Charge 编排运行期接线（OPEN-API-71）：渠道激活校验 + 状态门禁 + 渠道调用。
	paymentGatewayRegistry := InitGatewayRegistry()
	paymentChargeOrchestrator := apppayments.NewChargeOrchestrator(
		clock.SystemClock{}, platformuuid.V7Generator{},
		infrapgpayments.NewRepository(pool),
		paymentGatewayRegistry,
		channelConfigResolver{repo: infrapgpayments.NewChannelConfigsRepository(pool)},
		chargeRunner{uow: unitOfWork},
	)
	paymentHandler := paymenttransport.NewHandler(services.PaymentService, jwtIssuer)
	paymentHandler.Charge = paymentChargeOrchestrator
	// Refund 编排运行期接线（BE-C-09）：事务边界 + tx 绑定仓储/行锁由
	// refundExecutor 提供，与 charge 共用同一渠道注册表。
	paymentHandler.Refund = refundExecutor{uow: unitOfWork, registry: paymentGatewayRegistry}

	// Transport 层只依赖 app 层接口，不直接碰 domain/infra。
	// BE-B-12：providerID → 实体 → ProviderType 两级解析需要租户 scoped 的
	// provider 仓储；不注入 lookup 时 SendNotification 的 providerID 路径不可用。
	messageService := appmessages.NewServiceWithProviderLookup(notificationFactory, unitOfWork.NotificationProviders())
	radiusService := appradius.NewService(radiusFactory)
	// BE-B-13：RADIUS 计费报文必须落库。注入带租户解析的计费回调
	//（System 作用域解析 orgSlug → 严格租户作用域写入 radius_accountings）。
	radiusHandler := radiustransport.NewHandler(unitOfWork, radiusService)
	radiusHandler.AccountingHandler = newRadiusAccountingHandler(unitOfWork, services.RadiusAccountingService)

	return TransportHandlers{
		CertHandler:                 certtransport.NewHandler(services.CertService),
		CaptchaProviderHandler:      captchaprovidertransport.NewHandler(services.CaptchaProviderService),
		NotificationProviderHandler: notificationprovidertransport.NewHandler(services.NotificationProviderService),
		NotificationHandler:         notificationtransport.NewHandler(services.NotificationService, services.UserNotificationService),
		UserNotificationHandler:     usernotificationtransport.NewHandler(services.UserNotificationService),
		AdapterHandler:              adaptertransport.NewHandler(services.AdapterService),
		EnforcerHandler:             enforcertransport.NewHandler(enforcerService),
		ModelHandler:                modeltransport.NewHandler(services.ModelService),
		SyncerHandler:               syncertransport.NewHandler(services.SyncerService),
		WebhookHandler:              webhooktransport.NewHandler(services.WebhookService),
		MessageHandler:              messagetransport.NewHandler(messageService),
		LdapHandler:                 ldaptransport.NewHandler(services.LdapService),
		LdapSyncHandler:             ldapsynctransport.NewHandler(ldapSyncService),
		AgentHandler:                agenttransport.NewHandler(services.AgentService),
		FormHandler:                 formtransport.NewHandler(services.FormService),
		AuthHandler:                 authtransport.NewHandler(authServicePtr, jwtIssuer),
		DCRHandler:                  dcrtransport.NewHandler(services.ApplicationService),
		ApplicationHandler:          applicationtransport.NewHandler(services.ApplicationService, jwtIssuer),
		GatewayHandler:              gatewaytransport.NewHandler(services.GatewayService),
		GroupHandler:                groupstransport.NewHandler(services.GroupService),
		InvitationHandler:           invitationtransport.NewHandler(services.InvitationService),
		OrganizationHandler:         organizationtransport.NewHandler(services.OrganizationService, services.ApplicationService),
		PermissionHandler:           permissiontransport.NewHandler(services.PermissionService),
		OrderHandler:                ordertransport.NewHandler(services.OrderService),
		PaymentHandler:              paymentHandler,
		PaymentNotifyHandler:        paymenttransport.NewNotifyHandler(services.OrderService, loadPaymentNotifyConfig()),
		ReconHandler:                paymenttransport.NewReconHandler(paymentsReconService),
		ChannelConfigHandler:        paymenttransport.NewChannelConfigHandler(paymentsChannelConfigService),
		ChannelCatalogHandler:       paymenttransport.NewChannelCatalogHandler(apppayments.NewChannelCatalogService(infrapayment.ChannelCatalog{})),
		AttemptsHandler:             paymenttransport.NewAttemptsHandler(apppayments.NewHistoryService(paymentsAttemptsRepo, paymentsRefundsRepo)),
		RefundsHandler:              paymenttransport.NewRefundsHandler(apppayments.NewHistoryService(paymentsAttemptsRepo, paymentsRefundsRepo)),
		CallbackReceiptsHandler:     paymenttransport.NewCallbackReceiptsHandler(apppayments.NewPaymentAuditService(paymentsCallbackReceiptsRepo)),
		PlanHandler:                 plantransport.NewHandler(services.PlanService),
		// BE-A-05：平台管理员管理面接线。DTOAdapter 把 domain 实体投影为 app DTO，
		// transport 窄接口不 import domain；审计身份由 handler 从可信会话解析。
		PlatformAdminHandler:    platformadmintransport.NewHandler(appplatformadmins.NewDTOAdapter(services.PlatformAdminService)),
		PricingHandler:          pricingtransport.NewHandler(services.PricingService),
		ProductHandler:          producttransport.NewHandler(services.ProductService),
		ProviderHandler:         providertransport.NewHandler(services.ProviderService),
		RecordHandler:           recordtransport.NewHandler(services.RecordService),
		RebacHandler:            rebactransport.NewHandler(rebacRepo, rebacEngine, rebacService),
		RiskHandler:             risktransport.Handler{Service: riskService},
		PrivacyHandler:          privacytransport.Handler{Service: appprivacy.NewService(clock.SystemClock{}, platformuuid.V7Generator{}, newPrivacyRunner(unitOfWork)), Consent: appprivacy.NewConsentService(clock.SystemClock{}, platformuuid.V7Generator{}, newPrivacyRunner(unitOfWork))},
		IDVHandler:              idvtransport.Handler{Service: idvStoreService},
		ActionsHandler:          actionstransport.Handler{Service: actionsService},
		MeteringHandler:         meteringtransport.Handler{Service: meteringService},
		WorkloadHandler:         workloadtransport.Handler{Service: workloadService},
		IGAHandler:              igatransport.Handler{Service: igaService},
		WebhookEventHandler:     webhookeventstransport.Handler{Service: webhookeventsService},
		ResourceHandler:         resourcetransport.NewHandler(services.ResourceService),
		RoleHandler:             roletransport.NewHandler(services.RoleService),
		SessionHandler:          sessiontransport.NewHandler(services.SessionService),
		SubscriptionHandler:     subscriptiontransport.NewHandler(services.SubscriptionService),
		TicketHandler:           tickettransport.NewHandler(services.TicketService),
		TokenHandler:            tokentransport.NewHandler(services.TokenService),
		UserHandler:             usertransport.NewHandler(services.UserService, services.OrderService, services.PaymentService, services.SessionService, scopeAuthorizer, authServicePtr),
		BatchHandler:            batchtransport.NewHandler(nil),
		CouponHandler:           coupontransport.NewHandler(services.CouponService),
		EntryHandler:            entrytransport.NewHandler(services.EntryService),
		McpServerHandler:        mcpservertransport.NewHandler(services.McpServerService),
		RadiusAccountingHandler: radiusaccountingtransport.NewHandler(services.RadiusAccountingService),
		VerificationHandler:     verificationtransport.NewHandler(verificationService),
		DashboardHandler: dashboardtransport.Handler{
			DashboardService: services.DashboardService,
		},
		B2BHandler:            b2btransport.Handler{Service: services.B2BService},
		FaceIdProviderHandler: faceidprovidertransport.NewHandler(services.FaceIdProviderService),
		RuleHandler:           ruletransport.NewHandler(services.RuleService),
		RadiusConfigHandler:   radiusconfigtransport.NewHandler(services.RadiusConfigService),
		TransactionHandler:    transactionstransport.NewHandler(services.TransactionService),
		KeyHandler:            keystransport.NewHandler(services.KeyService),
		SearchHandler:         searchtransport.NewHandler(services.SearchService),
		WellknownHandler:      wellknownHandler,
		CASHandler:            casHandler,
		SAMLHandler:           samlHandler,
		WebAuthnHandler:       webauthnHandler,
		KerberosHandler:       kerberosHandler,
		RADIUSHandler:         radiusHandler,
		I18nHandler:           i18ntransport.NewHandler(i18n.NewTranslator()),
		SCIMV1Handler:         scimv1transport.NewHandler(newSCIMService(uuidGenerator, unitOfWork)),
		ReconciliationHandler: reconciliationtransport.NewHandler(buildReconciliationService(pool)),
		AIGatewayAdminHandler: aigatewaytrans.NewAdminHandler(services.AIGatewayAdmin),
	}
}

// loadPaymentNotifyConfig collects the payment-callback verification material
// from the environment. Every channel is fail-closed: an unset variable leaves
// that channel unconfigured and its callbacks are rejected with 503 rather
// than accepted without verification.
func loadPaymentNotifyConfig() paymenttransport.NotifyConfig {
	return paymenttransport.NotifyConfig{
		AlipayPublicKey:       envOrEmpty("ALIPAY_PUBLIC_KEY"),
		WeChatAPIV3Key:        envOrEmpty("WECHAT_PAY_API_V3_KEY"),
		WeChatPlatformCertPEM: envOrEmpty("WECHAT_PAY_PLATFORM_CERTS"),
		StripeWebhookSecret:   envOrEmpty("STRIPE_WEBHOOK_SECRET"),
	}
}
