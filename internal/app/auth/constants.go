package auth

import userdomain "github.com/guliangiam/guliangiam/internal/domain/users"

// OAuth/OIDC 参数名、协议取值与跨文件复用的词元集中定义。
// 这些字符串属于对接协议或权限契约的一部分：修改会直接影响客户端、Casbin 策略
// 与前端 action 前缀，必须与契约文档同步修改。
const (
	// OIDC scopes（authorize / discovery）
	scopeOpenID        = "openid"
	scopeProfile       = "profile"
	scopeEmail         = "email"
	scopePhone         = "phone"
	scopeOfflineAccess = "offline_access"

	// OAuth 参数与取值
	paramGrantType         = "grantType"
	paramIssued            = "issued"
	paramLoginMethod       = "loginMethod"
	fieldCode              = "code"
	oauthRefreshToken      = "refresh_token"
	grantAuthorizationCode = "authorization_code"
	tokenTypeBearer        = "Bearer"
	tokenTypeJWT           = "JWT"

	// 联系方式 / 验证通道
	contactEmail = "email"
	contactPhone = "phone"
	contactSMS   = "sms"
	contactPush  = "push"

	// 账号与登录方式
	statusActive        = "active"
	userTypeNormal      = "normal"
	loginMethodPassword = "password"
	loginMethodMFA      = "mfa"
	loginMethodLDAP     = "ldap"
	loginMethodFido2    = "fido2"
	loginMethodSocial   = "social"
	hashArgon2ID        = userdomain.PasswordAlgorithmArgon2ID

	// 其他复用键
	keyApp                   = "app"
	fieldDisplayName         = "displayName"
	fieldAvatarURL           = "avatarUrl"
	verificationPurposeReset = "reset"
	casTicketProxyGranting   = "proxy_granting"
)

// Casbin 资源 slug（policy 的 obj 字段）。前端 action 前缀由 resourceToActionKey 映射。
const (
	resourceUsers                 = "users"
	resourceOrganizations         = "organizations"
	resourceDomains               = "domains"
	resourceApplications          = "applications"
	resourceProviders             = "providers"
	resourceGroups                = "groups"
	resourceRoles                 = "roles"
	resourcePermissions           = "permissions"
	resourceSessions              = "sessions"
	resourceTokens                = "tokens"
	resourceRecords               = "records"
	resourceWebhooks              = "webhooks"
	resourceCerts                 = "certs"
	resourceKeys                  = "keys"
	resourceModels                = "models"
	resourceAdapters              = "adapters"
	resourceEnforcers             = "enforcers"
	resourceSyncers               = "syncers"
	resourceResources             = "resources"
	resourceInvitations           = "invitations"
	resourceForms                 = "forms"
	resourceEntries               = "entries"
	resourceMCPServers            = "mcp-servers"
	resourcePolicies              = "policies"
	resourceGateway               = "gateway"
	resourcePlans                 = "plans"
	resourcePricings              = "pricings"
	resourceProducts              = "products"
	resourceOrders                = "orders"
	resourcePayments              = "payments"
	resourceCoupons               = "coupons"
	resourceSubscriptions         = "subscriptions"
	resourceTransactions          = "transactions"
	resourceAIGatewayProviders    = "aigateway-providers"
	resourceAIGatewayModels       = "aigateway-models"
	resourceAIGatewayAliases      = "aigateway-aliases"
	resourceAIGatewayRoutes       = "aigateway-routes"
	resourceAIGatewayAPIKeys      = "aigateway-api-keys"
	resourceAIGatewayQuotas       = "aigateway-quotas"
	resourceAIGatewayPrices       = "aigateway-prices"
	resourceCaptchaProviders      = "captcha-providers"
	resourceNotificationProviders = "notification-providers"
	resourceFaceIDProviders       = "face-id-providers"
	resourceRadiusConfigs         = "radius-configs"
	resourceServers               = "servers"
	resourceSites                 = "sites"
	resourceAgents                = "agents"
	resourceTickets               = "tickets"
	resourceLDAPs                 = "ldaps"
	resourceRules                 = "rules"
	resourceIGA                   = "iga"
	// Governance surfaces share the same hyphen-free slug for Casbin obj and
	// frontend action prefix: actions / rebac / risk / metering.
	resourceActions  = "actions"
	resourceReBAC    = "rebac"
	resourceRisk     = "risk"
	resourceMetering = "metering"
	// Workload identity pools: Casbin resource and frontend action prefix share
	// the same hyphenated slug, so the derived action is "identity-pools:read".
	resourceIdentityPools = "identity-pools"
)

// 路径首段资源：Casbin obj 与前端 action 前缀同名（resourceFromPath 不改名）。
const (
	resourcePayRecon          = "pay-recon"
	resourcePayReconItems     = "pay-recon-items"
	resourcePayChannelConfigs = "pay-channel-configs"
	resourceI18n              = "i18n"
)
