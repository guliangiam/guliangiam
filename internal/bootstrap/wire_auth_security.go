// wire_auth_security.go — Auth/Security infrastructure: WAF + JWKS/JWT + authService 字段赋值 + IDV + enforcer + messaging/captcha/notification.
// 从 runtime.go NewRuntime L618-702 抽离，保持 bootstrap 包级 private function。
//
// 注意: 接收 *appauth.Service 指针，在函数内部完成 authService 所有安全相关字段赋值
// （JWKSManager, TokenIssuer, DPoP, SAMLValidator, GeoLookup, CasbinPolicyManager, CaptchaFactory, ...）。
package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appenforcers "github.com/guliangiam/guliangiam/internal/app/enforcers"
	appnotifications "github.com/guliangiam/guliangiam/internal/app/notifications"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appverifications "github.com/guliangiam/guliangiam/internal/app/verifications"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"

	infracache "github.com/guliangiam/guliangiam/internal/infra/cache"
	"github.com/guliangiam/guliangiam/internal/infra/captcha"
	"github.com/guliangiam/guliangiam/internal/infra/casbin"
	infrafederation "github.com/guliangiam/guliangiam/internal/infra/federation"
	"github.com/guliangiam/guliangiam/internal/infra/hibp"
	"github.com/guliangiam/guliangiam/internal/infra/jwt"
	"github.com/guliangiam/guliangiam/internal/infra/messaging"
	mysqlinfradpop "github.com/guliangiam/guliangiam/internal/infra/mysql/dpop"
	"github.com/guliangiam/guliangiam/internal/infra/notification"
	infradpop "github.com/guliangiam/guliangiam/internal/infra/postgres/dpop"
	infrasaml "github.com/guliangiam/guliangiam/internal/infra/saml"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformdpop "github.com/guliangiam/guliangiam/internal/platform/dpop"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformgeo "github.com/guliangiam/guliangiam/internal/platform/geo"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
	httpmiddleware "github.com/guliangiam/guliangiam/internal/transport/http/middleware"

	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	ssfoutbound "github.com/guliangiam/guliangiam/internal/outbound/ssf"
)

// wireAuthSecurity 构造 Auth 安全基础设施 + 修改 authService 安全字段 + 返回 Messaging/Captcha/Notification 全家桶。
//
// 参数:
//   - authService    *appauth.Service — 指针，函数内直接完成所有安全字段赋值
//   - ssfSignerHolder *ssfoutbound.SignerHolder — 需要 Set(jwksProvider)
//   - corsConfig     CORS 白名单（WebAuthnAllowedOrigins 复用）
//   - casbinManager  Casbin 管理器（authService.CasbinPolicyManager + enforcerService）
//   - driver/pool/mysqlDB — DPoP Replay Store 双基线选择
//   - unitOfWork/clock/uuid — verificationService 构造
//
// authSecurityDeps 聚合认证/安全中间件、Issuer、JWKS 与通知依赖。
type authSecurityDeps struct {
	WAF                *httpmiddleware.WAFMiddleware
	IPRules            *httpmiddleware.IPRulesMiddleware
	JWTIssuer          *jwt.Issuer
	JWKS               domainauth.JWKSProvider
	Enforcer           appenforcers.Service
	Verification       appverifications.Service
	Captcha            *captcha.Factory
	Notification       *notification.Factory
	NotificationWorker *appnotifications.Worker
	Sender             *messaging.SenderFactory
	RemoteJWKS         *platformjwks.RemoteJWKSCache
	ProviderCache      *appproviders.MessagingProviderCache
}

// wireAuthSecurity 构造认证/安全基础设施并聚合为 authSecurityDeps。
// 字段:
//   - WAF                → Middleware.WAFMiddleware
//   - JWTIssuer          → Infra.JWTIssuer + Handlers
//   - JWKS               → Infra.JWKSManager + wellknownHandler
//   - Enforcer           → Services.EnforcerService
//   - Verification       → Services.VerificationService
//   - Captcha            → Infra.CaptchaFactory
//   - Notification       → Infra.NotificationFactory
//   - NotificationWorker → Workers.NotificationWorker（已 Start）
//   - Sender             → Infra.SenderFactory
//   - RemoteJWKS         → 联邦信任远程 JWKS 缓存
func wireAuthSecurity(
	ctx context.Context,
	authService *appauth.Service,
	ssfSignerHolder *ssfoutbound.SignerHolder,
	corsConfig httpmiddleware.CORSConfig,
	casbinManager *casbin.Manager,
	driver string,
	pool *pgxpool.Pool,
	mysqlDB *sql.DB,
	unitOfWork appuow.UnitOfWork,
	clockProvider clock.Clock,
	uuidGenerator platformuuid.Generator,
	sessionCache infracache.SessionCache,
) (authSecurityDeps, error) {
	// ── WAF middleware (fail-close) ─────────────────────────────────────
	// 规则仓储走 UoW（PostgreSQL / MySQL 双实现），不再直接用 pgxpool：
	// MySQL 分支的 pool 为 nil，直接构造会在启动加载 WAF 规则时 panic。
	ruleRepo := unitOfWork.Rules()
	ruleQuerySvc := newRuleQueryService(ruleRepo)
	wafMiddleware, err := httpmiddleware.NewWAFMiddleware(ruleQuerySvc, slog.Default())
	if err != nil {
		var emptyEnforcer appenforcers.Service
		var emptyVerif appverifications.Service
		return authSecurityDeps{Enforcer: emptyEnforcer, Verification: emptyVerif},
			apperrors.WrapInternal(err, "WAF middleware initialization failed")
	}
	// BE-E-04：IP 黑白名单与 WAF 共用同一规则查询适配器；中间件自身 fail-open
	// （规则查询出错放行），无启用规则时零影响。
	ipRulesMiddleware := httpmiddleware.NewIPRulesMiddleware(ruleQuerySvc)

	// ── JWKS + JWT Issuer ────────────────────────────────────────────────
	jwksManager, err := newJWKSManager(envOrEmpty("JWT_SIGNING_KEYS_FILE"))
	if err != nil {
		var emptyEnforcer appenforcers.Service
		var emptyVerif appverifications.Service
		return authSecurityDeps{Enforcer: emptyEnforcer, Verification: emptyVerif},
			apperrors.WrapInternal(err, "JWKS initialization failed")
	}
	jwtIssuer := jwt.NewIssuer(jwksManager)
	jwksProvider := jwt.NewJWKSProviderAdapter(jwksManager)

	// ── authService 安全字段赋值 ──────────────────────────────────────────
	authService.JWKSManager = jwksProvider
	ssfSignerHolder.Set(jwksProvider)
	// RFC 8705 tls_client_auth 的证书链校验根：与服务端 TLS 使用同一 CA bundle。
	// 文件为空时返回 nil，应用层据此对该方式 fail-closed。
	clientCAPool, err := platformmtls.LoadCertPoolFromFile(config.TLSClientCAFile())
	if err != nil {
		var emptyEnforcer appenforcers.Service
		var emptyVerif appverifications.Service
		return authSecurityDeps{Enforcer: emptyEnforcer, Verification: emptyVerif},
			apperrors.WrapInternal(err, "TLS client CA bundle initialization failed")
	}
	authService.TLSClientCAs = clientCAPool
	// ID token signing goes through the same issuer that validates them.
	// Without this assignment IssueIDToken always failed with "token issuer is not configured".
	authService.TokenIssuer = jwtIssuer
	// The issuer a token declares must equal what discovery reports.
	authService.IssuerURL = config.ConfiguredIssuerURL()
	// WebAuthn 断言 origin 校验复用 CORS 白名单（跨域部署），同源部署时由 IssuerURL 兜底。
	authService.WebAuthnAllowedOrigins = corsConfig.AllowedOrigins
	// WebAuthn UV 策略（默认关闭；只做 UP 的存量认证器因此不受影响）。
	authService.WebAuthnRequireUV = config.WebAuthnRequireUV()
	// DPoP proof validation needs state (the replay cache) on top of stateless checks.
	// Driver selection aligns with DB_DRIVER env: pgx → PG store, mysql → MySQL store.
	var replayStore platformdpop.ReplayStore
	switch driver {
	case driverMySQL:
		replayStore = mysqlinfradpop.NewMySQLReplayStore(mysqlDB)
	default:
		replayStore = infradpop.NewPostgresReplayStore(pool)
	}
	authService.DPoP = platformdpop.NewDPoPValidatorWithReplayStore(replayStore)
	authService.DPoP.Nonces = newDPoPNonceStore(sessionCache)
	// 泄露口令检查（AUTH-009 / OPEN-API-85）：开关默认关；开启后仅 SHA-1
	// 前 5 位出域（k-匿名），依赖故障 fail-open 并在审计标记 unverified。
	if config.PasswordBreachCheckEnabled() {
		authService.BreachChecker = newBreachChecker(sessionCache)
		slog.Info("password breach check enabled", "base_url", config.PasswordBreachCheckBaseURL())
	}
	authService.SAMLValidator = infrasaml.NewValidator()
	// RelayState signing stays on the server: the frontend must never hold the
	// key (SEC-A01). The subkey is derived from the mandatory encryption key with
	// domain separation, so no extra deployment secret is required.
	relayStateSigner, err := infrasaml.NewRelayStateSigner(config.EncryptionKeyCurrent())
	if err != nil {
		var emptyEnforcer appenforcers.Service
		var emptyVerif appverifications.Service
		return authSecurityDeps{Enforcer: emptyEnforcer, Verification: emptyVerif},
			apperrors.WrapInternal(err, "SAML relay state signer initialization failed")
	}
	authService.RelayStateSigner = relayStateSigner
	// IP geolocation service wired to auth service
	geoLookup := platformgeo.NewLookupService(envOrEmpty("IP2REGION_XDB_PATH"), slog.Default())
	authService.GeoLookup = geoLookup
	// Casbin manager for permissions_query.go to resolve real p-policies (role inheritance included).
	authService.CasbinPolicyManager = casbinManager

	// Federation Trust uses a distinct remote-JWKS cache and an SSRF-safe client.
	federationJWKSCache := configureFederation(authService)

	// ── Enforcer service ──────────────────────────────────────────────────
	enforcerService := appenforcers.NewService(clockProvider, uuidGenerator, unitOfWork, casbinManager)
	// 策略写成功后清除账户权限缓存（DEF-16）。
	enforcerService.CacheInvalidator = authService

	// ── Messaging + Captcha + Notification ───────────────────────────────
	senderFactory, emailProvider, smsProvider := registerMessagingSenders()
	messagingResolver, providerCache := newMessagingResolver(driver, pool, senderFactory, emailProvider, smsProvider)
	verificationService := appverifications.NewServiceWithMessaging(
		clockProvider, uuidGenerator, unitOfWork, messagingResolver,
	)
	// Login one-time-code channel shares the verification service so expiry,
	// one-time consumption and recipient/per-IP rate limits stay identical.
	appauth.WireLoginVerificationConsumer(verificationService.ConsumeVerificationCodeWithClientIP)
	// 事务内码匹配（重置/注册/联系方式绑定）与服务侧消费共用同一失败预算
	// （「失败 3 次即失效该码」，B06）。
	appauth.WireVerificationAttemptLimiter(verificationService.RateLimiter)

	captchaFactory := registerCaptchaProviders()
	authService.CaptchaFactory = captchaFactory
	// Gate verification-code delivery on the same human check as signup, so a
	// configured captcha provider also caps SMS/mail sends to verified humans.
	verificationService.CaptchaVerifier = authService.EnforceCaptcha

	notificationFactory, notificationWorker := registerNotificationProviders(
		senderFactory, emailProvider, smsProvider, unitOfWork,
	)
	notificationWorker.Start(ctx)

	return authSecurityDeps{
		WAF:                wafMiddleware,
		IPRules:            ipRulesMiddleware,
		JWTIssuer:          jwtIssuer,
		JWKS:               jwksProvider,
		Enforcer:           enforcerService,
		Verification:       verificationService,
		Captcha:            captchaFactory,
		Notification:       notificationFactory,
		NotificationWorker: notificationWorker,
		Sender:             senderFactory,
		RemoteJWKS:         federationJWKSCache,
		ProviderCache:      providerCache,
	}, nil
}

func configureFederation(authService *appauth.Service) *platformjwks.RemoteJWKSCache {
	cache := platformjwks.NewRemoteJWKSCache(
		5*time.Minute, 30*time.Minute, ssrf.SafeHTTPClient(10*time.Second),
	)
	// ConfigureFederation installs both the verifier and the UOW/system-context
	// issuer resolver; assigning only the verifier leaves federation disabled.
	authService.ConfigureFederation(infrafederation.NewVerifier(cache))
	return cache
}

// newDPoPNonceStore reuses the session-cache Redis connection when one exists
// so nonces issued by any replica verify on any other (OPEN-API-86); otherwise
// it falls back to process-local state with an explicit single-instance warning
// (mirrors newSAMLReplayStore). Request-level Redis errors fail closed in
// RedisNonceStore.Verify.
func newDPoPNonceStore(sessionCache infracache.SessionCache) platformdpop.NonceStore {
	if provider, ok := sessionCache.(interface{ RedisClient() *redis.Client }); ok {
		if client := provider.RedisClient(); client != nil {
			return platformdpop.NewRedisNonceStore(client)
		}
	}
	slog.Warn("DPoP nonce store uses process-local state",
		"reason", "session cache is not Redis-backed; multi-replica nonce recognition is unavailable")
	return platformdpop.NewLocalNonceStore()
}

// newBreachChecker builds the HIBP k-anonymity range checker (AUTH-009).
// Range responses are shared through the session-cache Redis when available
// (mirrors newDPoPNonceStore); otherwise a process-local cache is used. The
// cache only holds public hash-suffix lists keyed by 5-char prefix — no
// user-derived secret material.
func newBreachChecker(sessionCache infracache.SessionCache) *hibp.RangeChecker {
	var cache hibp.RangeCache
	if provider, ok := sessionCache.(interface{ RedisClient() *redis.Client }); ok {
		if client := provider.RedisClient(); client != nil {
			cache = hibp.NewRedisRangeCache(client)
		}
	}
	if cache == nil {
		slog.Warn("password breach range cache uses process-local state",
			"reason", "session cache is not Redis-backed; range responses are not shared across replicas")
		cache = hibp.NewLocalRangeCache()
	}
	return hibp.NewRangeChecker(config.PasswordBreachCheckBaseURL(), 2*time.Second, cache)
}
