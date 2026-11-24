package bootstrap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	platformaudit "github.com/guliangiam/guliangiam/internal/platform/auditdenial"
	config "github.com/guliangiam/guliangiam/internal/platform/config"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	mw "github.com/guliangiam/guliangiam/internal/platform/middleware"
	httpmw "github.com/guliangiam/guliangiam/internal/transport/http/middleware"
	httputil "github.com/guliangiam/guliangiam/internal/transport/http/util"

	rebacpostgres "github.com/guliangiam/guliangiam/internal/infra/postgres/rebac"
	actionstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/actions"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/adapters"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/agents"
	aigatewaytrans "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/aigateway"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/applications"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	b2btransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/b2b"
	batchtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/batch"
	captchaproviders "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/captchaproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/cas"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/certs"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/coupons"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dashboard"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/dcr"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/enforcers"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/entries"
	faceidproviders "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/forms"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/gateway"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/groups"
	i18ntransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/i18n"
	workloadtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/identitypools"
	idvtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/idv"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/iga"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/invitations"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/kerberos"
	keystransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/keys"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldaps"
	ldapsynctransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/ldapsync"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/mcpservers"
	messagetransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/messages"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/metering"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/models"
	notificationproviders "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notificationproviders"
	notifications "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/notifications"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/orders"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/organizations"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/payments"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/permissions"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/plans"
	platformadmintransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/platformadmins"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/pricings"
	privacytransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/privacy"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/products"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/providers"
	radiustransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radius"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusaccountings"
	radiusconfigs "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/radiusconfigs"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rebac"
	reconciliationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/reconciliation"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/records"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/resources"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/risk"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/roles"
	rules "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/rules"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/saml"
	scimv1transport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/scim"
	searchtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/search"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/sessions"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/subscriptions"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/syncers"
	systemtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/system"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tickets"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/tokens"
	transactionstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/transactions"
	usernotifications "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/usernotifications"
	userstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/users"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/verifications"
	webauthntransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webauthn"
	webhookeventstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhookevents"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/webhooks"
	"github.com/guliangiam/guliangiam/internal/transport/http/api/v1/wellknown"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// Request-body hard walls (手册 4.17.2). The default applies to every request;
// only the paths below may be larger than 1 MiB. `http.MaxBytesReader` is the
// only enforcement point, so a handler that declares a bigger multipart limit
// without a matching entry here still sees 1 MiB — which is exactly the
// users-import defect this table fixes.
const (
	defaultRequestBodyLimit int64 = 1 << 20  // 1 MiB
	otlpRequestBodyLimit    int64 = 64 << 20 // 64 MiB
)

// requestBodyLimit resolves the cap for a request path. The users Excel/CSV
// import endpoints, the group/role/permission Excel imports and the enforcer
// policy Excel import all parse multipart bodies and must follow the same
// single-file cap as every other upload (STORAGE_UPLOAD_MAX_BYTES, 32 MiB
// default), rather than silently inheriting the 1 MiB default.
func requestBodyLimit(path string) int64 {
	// The enforcer route carries the enforcer id in the path, so the table
	// cannot match it exactly; the suffix is unique to that import route.
	if strings.HasPrefix(path, "/api/v1/enforcers/") && strings.HasSuffix(path, "/upload-policies") {
		return config.UploadMaxBytes()
	}
	switch path {
	case "/api/v1/upload-resource",
		"/api/v1/users/upload",
		"/api/v1/users/upload/preview",
		"/api/v1/users/upload-csv",
		"/api/v1/groups/upload",
		"/api/v1/roles/upload",
		"/api/v1/permissions/upload":
		return config.UploadMaxBytes()
	case "/api/v1/entries/logs",
		"/api/v1/entries/metrics",
		"/api/v1/entries/traces":
		return otlpRequestBodyLimit
	default:
		return defaultRequestBodyLimit
	}
}

func requestBodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, requestBodyLimit(r.URL.Path))
		next.ServeHTTP(w, r)
	})
}

func NewChiRouter(runtime *Runtime) chi.Router {
	validateStartupRoutes()
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	// 保留原始 RemoteAddr；ClientIP 仅在 socket peer 属于可信代理时解析转发头。
	// CORS 必须排在可观测性、限流与安全头之前（手册 4.16）：
	//   - 预检请求由本中间件终结，不计入限流额度、不产生 span、不写审计，
	//     否则浏览器每次真实请求都会先消耗一份配额；
	//   - 被限流/被 WAF 拒绝的响应也必须带跨域头，否则浏览器只会报「跨域失败」，
	//     把 429 的退避指引（Retry-After）连同真实原因一起藏掉。
	router.Use(httpmw.CORS(runtime.Middleware.CORSConfig))
	router.Use(httpmw.SentryMiddleware())
	// Panic recovery writes the same error envelope as route.WriteError (or the
	// protocol-face shape for OAuth/SCIM paths) and records requestId/traceId.
	router.Use(httpmw.Recoverer())
	router.Use(mw.TracingMiddleware)
	router.Use(mw.PrometheusMiddleware)
	// 统一访问日志（《可观测性》§1.3）：放在 Prometheus 之后，使 RED 指标与
	// 日志行描述同一次请求；探针端点由中间件自身跳过。
	router.Use(mw.AccessLogMiddleware)
	router.Use(mw.RateLimit(mw.DefaultAPIRateLimit))
	router.Use(runtime.Middleware.ServiceIPLimiter.Middleware)
	// 请求体大小保护（1MB 默认，白名单路径放宽）
	// WAF 层已有 12.5MB access log buffer，这是应用层第一道硬墙。
	// chi/v5 没有 BodyLimit，用 http.MaxBytesReader 实现。
	router.Use(requestBodyLimitMiddleware)

	router.Use(httpmw.SecurityHeaders(httpmw.DefaultSecurityHeadersConfig()))
	router.Use(runtime.Middleware.WAFMiddleware.Middleware)
	// BE-E-04：IP 黑白名单（规则类型 IP，Allow 优先于 Block）。合成 Runtime 的
	// 测试可能不装配该中间件，nil 时跳过（与 DenialAudit 同模式）。
	if runtime.Middleware.IPRulesMiddleware != nil {
		router.Use(runtime.Middleware.IPRulesMiddleware.Middleware)
	}

	// Stamps the API face (手册 4.1.3) plus the request/trace ids onto every
	// request context. The surface is derived from the path by
	// route.SurfaceFor, the same classifier the surface contract gate holds every
	// route declaration to, so metrics, audit rows and OpenAPI tags cannot drift
	// apart from the contract.
	router.Use(route.RequestMetadataMiddleware())
	// Carries the observed request origin (client IP via the trusted-proxy
	// verdict, user agent) plus the ids above to the platform-level context, so
	// app/audit layers can stamp audit records without importing transport.
	router.Use(httpmw.RequestMetadataMiddleware())

	RegisterPublicRoutes(router, runtime)

	// The management, authentication and protocol faces all sit behind the same
	// session/authorization chain: the protocol handlers intentionally keep the
	// exact chain they had before the faces were made explicit, so this refactor
	// changes no runtime behaviour. Giving the protocol face its own chain (the
	// standard clients authenticate themselves) is a separate, deliberate change
	// tracked in 《架构设计》5 / 6.1.
	protected := router.Group(nil)
	protected.Use(httpmw.CompressionMiddleware(httpmw.DefaultCompressionConfig()))
	protected.Use(httpmw.ETagCacheMiddleware(httpmw.DefaultETagConfig()))
	protected.Use(httpmw.PoolGuardMiddleware(runtime.DB.Pool))
	// Pagination bounds (手册 4.7.1) are enforced once, here, so every list
	// endpoint — present and future — rejects page>MaxPage or pageSize>100 with
	// 422 instead of passing an overflowing offset to the database. Read the
	// parameters before the session chain because the answer does not depend on
	// who is calling.
	protected.Use(httpmw.PaginationMiddleware())
	protected.Use(auth.SessionContextMiddleware(runtime.Handlers.AuthHandler, runtime.Infra.JWTIssuer, runtime.Infra.PrincipalResolver))
	// 路径 UUID 参数校验：非法 id 统一 400（DEF-44），避免 pgx cast 错误被包装成 500。
	protected.Use(httpmw.UUIDParamGuard)
	// Denial audit: L1 callers use the recorder from context; L2 only fills the
	// 403 gap that cannot be classified by domain code.
	if runtime.Middleware.DenialAudit != nil {
		protected.Use(platformaudit.WithRecorderMiddleware(runtime.Middleware.DenialAudit))
		protected.Use(platformaudit.AuthorizationDenialMiddleware(func(r *http.Request) (platformaudit.Event, bool) {
			session, ok := auth.SessionContextFromContext(r.Context())
			if !ok || session.OrganizationID == "" {
				return platformaudit.Event{}, false
			}
			meta := route.RequestMetadataFrom(r.Context())
			return platformaudit.Event{
				OrganizationID: session.OrganizationID,
				ActorID:        session.UserID,
				IP:             httputil.ClientIP(r),
				UserAgent:      strings.TrimSpace(r.UserAgent()),
				Surface:        string(meta.Surface),
				RequestID:      meta.RequestID,
				TraceID:        meta.TraceID,
			}, true
		}))
	}
	// CSRF runs after session resolution so only a successfully validated header
	// JWT can use the bearer exemption. Raw Authorization/X-Access-Token headers
	// and cookie-backed sessions still require the double-submit token.
	//
	// Client-authenticated protocol endpoints are exempt by exact path: an OAuth2
	// client has no session cookie, so it can never present a double-submit
	// token. The predicate is the same one the PEP uses (single source of truth).
	protected.Use(mw.CSRFTokenMiddlewareWithExemption(func(r *http.Request) bool {
		return auth.IsClientAuthenticatedProtocolEndpoint(r.Method, r.URL.Path)
	}, auth.HasValidatedBearerSession))
	// Phase B-D4: TenantResolutionMiddleware — 解析显式 target_tenant（跨租户操作时）
	// 必须在 SessionContextMiddleware 之后（依赖 session），在 APIAuthorizationMiddleware 之前
	// （Casbin 需要知道 target_tenant 是否是 session 自己的 org）。
	// 无显式 target_tenant 时 pass-through — 本租户操作由 TenantFilterMiddleware 处理。
	// 非 UUID 的显式 target（组织 slug）由组合根注入的组织仓储解析成 UUID，
	// 再交给 ScopeAuthorizer；解析失败一律 deny。
	protected.Use(httpmw.TenantResolutionMiddlewareWithRegionGuard(runtime.Middleware.OrganizationSlugResolver, runtime.Middleware.RegionResolver, runtime.Middleware.RegionResidencyMode))
	// Idempotency (ADR-018): must run after SessionContextMiddleware so we can
	// derive the scope from session.OrganizationSlug + session.UserID (BE-E-01:
	// the user dimension stops same-tenant users from replaying each other's
	// stored responses), but before APIAuthorizationMiddleware — same-key
	// replays should short-circuit before Casbin overhead.
	protected.Use(mw.IdempotencyMiddleware(runtime.Workers.IdempotencyRepo, func(ctx context.Context) (string, string) {
		if sess, ok := auth.SessionContextFromContext(ctx); ok {
			return sess.OrganizationSlug, sess.UserID
		}
		return "", ""
	}))
	var nsResolver auth.NamespaceResolver
	if resolver := rebacpostgres.NewNamespaceResolver(runtime.DB.Pool); resolver != nil {
		nsResolver = resolver
	}
	// 强制改密门禁：默认 enforce（未设置时）；warn 只计数+结构化日志；enforce 对非豁免路由分别返回
	// 403 password_change_required（管理员强制）或 password_expired（超过有效期）。
	//
	// 位置约束：必须在 SessionContext（认证）、TenantResolution（租户解析）、
	// 路由级限流之后，但必须在 APIAuthorization 之前。若排在授权之后，被强制
	// 改密的普通用户访问 landing 接口时先被 Casbin 以 insufficient permissions
	// 拒绝，password_change_required 永远不会到达客户端，用户无法自助解锁。
	// 门禁自身所需的 RLS 租户作用域由 PasswordGate 用会话组织钉住（见 password_gate.go）。
	if runtime.Middleware.PasswordGate != nil {
		protected.Use(runtime.Middleware.PasswordGate.Middleware)
	}
	// 显式权限点钉选：POST /users/{id}/set-password 的方法别名是 users:write，
	// 但口令重置必须由独立权限点 users:password:reset 授权（决策 #3）。
	protected.Use(auth.APIAuthorizationMiddleware(
		runtime.Infra.CasbinManager,
		runtime.Infra.RebacEngine,
		nsResolver,
		auth.PermissionConfig{
			Method:     "POST",
			Pattern:    "/api/v1/users/{id}/set-password",
			Permission: "users:password:reset",
		},
		auth.PermissionConfig{Method: "POST", Pattern: "/api/v1/auth/step-up/begin", Permission: "auth:step_up"},
		auth.PermissionConfig{Method: "POST", Pattern: "/api/v1/auth/step-up/finish", Permission: "auth:step_up"},
		auth.PermissionConfig{Method: "POST", Pattern: "/api/v1/auth/trusted-devices/begin", Permission: "auth:trusted_device"},
		auth.PermissionConfig{Method: "POST", Pattern: "/api/v1/auth/trusted-devices", Permission: "auth:trusted_device"},
		auth.PermissionConfig{Method: "GET", Pattern: "/api/v1/auth/trusted-devices", Permission: "auth:trusted_device"},
		auth.PermissionConfig{Method: "DELETE", Pattern: "/api/v1/auth/trusted-devices/{deviceId}", Permission: "auth:trusted_device"},
		// DEF-02：erase-review 嵌套路径运行期首段推导为 privacy，与声明的
		// privacy:erase 不一致；显式钉选后 Casbin 按专属权限点裁决（最小权限）。
		auth.PermissionConfig{Method: "POST", Pattern: "/api/v1/privacy/dsar/{id}/erase-review", Permission: "privacy:erase"},
	))
	// Phase C-Step 2: ScopeAuthorizerMiddleware — 跨租户作用域授权。
	// 必须在 Casbin 之后（资源级权限先放行），TenantFilterMiddleware 之前（RLS 前决定是否放行）。
	protected.Use(httpmw.ScopeAuthorizerMiddleware(runtime.Infra.ScopeAuthorizer))
	protected.Use(httpmw.TenantFilterMiddleware(httpmw.TenantFilterConfig{StrictMode: false}))
	protected.Use(auth.PrivilegedAuditMiddleware(runtime.Handlers.IGAHandler.Service))

	// Protocol face: endpoints defined by an external standard, spread over the
	// six prefix families of 手册 4.1.3 (/oauth2/*, /.well-known/*,
	// /cas/{org}/{app}/*, /scim/v2/*, /api/v1/saml/* and the protocol endpoints
	// inside /api/v1/auth/*, the last of which the auth package registers
	// itself).
	protocol := protected.Group(nil)
	protocol.Use(route.WithSurface(route.SurfaceProtocol))
	RegisterProtocolRoutes(protocol, runtime)

	// Authentication face: the page and session endpoints under /api/v1/auth/*.
	// The auth tree also carries the protocol endpoints of prefix family (5)
	// (authorize, tokens:*, device/*, oauth2/*, cas/*, saml/*, kerberos/*); the
	// auth package splits those into their own group internally and pins each
	// group's surface and rate limit.
	authentication := protected.Group(nil)
	auth.RegisterChiRoutes(authentication, runtime.Handlers.AuthHandler)

	management := protected.Group(nil)
	management.Use(route.WithSurface(route.SurfaceManagement))
	RegisterManagementRoutes(management, runtime)

	// Ops endpoints sit off protected but with their own token-based auth.
	// MetricsTokenMiddleware handles the gate — see RegisterOpsRoutes for why not PEP.

	// AI Gateway data plane — public group, own API Key auth (no session middleware)
	aiHandler := aigatewaytrans.NewHandler(runtime.Services.AIGateway)
	aigatewaytrans.RegisterChiRoutes(router, aiHandler)

	RegisterOpsRoutes(router, runtime)

	// Walk the assembled route tree and reject anonymously reachable routes that
	// are only covered by a whitelist prefix instead of an explicit declaration.
	validateStartupPublicWhitelist(router)

	return router
}

// RegisterProtocolRoutes wires the protocol face: endpoints defined by an
// external standard (OpenID Connect, OAuth 2.0, CAS, SCIM, SAML) rather than by
// this project's own REST conventions (手册 4.1.3).
func RegisterProtocolRoutes(router chi.Router, runtime *Runtime) {
	wellknown.RegisterChiRoutes(router, runtime.Handlers.WellknownHandler)
	dcr.RegisterChiRoutes(router, runtime.Handlers.DCRHandler)
	cas.RegisterChiRoutes(router, runtime.Handlers.CASHandler)
	saml.RegisterChiRoutes(router, runtime.Handlers.SAMLHandler)
	kerberos.RegisterChiRoutes(router, runtime.Handlers.KerberosHandler)
	webauthntransport.RegisterChiRoutes(router, runtime.Handlers.WebAuthnHandler)
	router.Mount("/", scimv1transport.ChiRouter(runtime.Handlers.SCIMV1Handler, runtime.Services.ApplicationService))
}

// healthzHandler 披露进程级健康：CRYPTO_PROFILE=gm 且必需能力缺失时，
// HealthReport 状态为 unavailable，这里映射为 503，避免编排层把
// “进程活着但算法不可用”当成健康。
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
	status := platformcrypto.HealthReport()
	code := http.StatusOK
	if status.Status == platformcrypto.HealthStatusUnavailable {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(status)
}

func RegisterPublicRoutes(router chi.Router, runtime *Runtime) {
	i18ntransport.RegisterChiRoutes(router, runtime.Handlers.I18nHandler)

	// SAML SP browser ACS: the external IdP posts SAMLResponse/RelayState here
	// before any session exists. It must stay on the root public group so
	// session/CSRF middleware never intercepts the cross-site form POST.
	saml.RegisterPublicChiRoutes(router, runtime.Handlers.SAMLHandler)

	// ── Liveness probe（公开，PEP 白名单） ─────────────────────────────────────
	// Kubernetes spec: /healthz is the canonical liveness endpoint.
	// 同 /livez 别名，方便不同编排器。
	livenessHandler := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
	// /healthz 额外披露加密能力位：CRYPTO_PROFILE=gm 且必需能力缺失时返回 503，
	// 避免编排层把“进程活着但算法不可用”当成健康。/livez 保持纯进程存活语义。
	router.Get("/healthz", healthzHandler)
	router.Get("/livez", livenessHandler)

	// CSRF 豁免安全论证：
	//   1. 此端点在 public 组（无 SessionContextMiddleware），请求不带 __Host-session cookie
	//   2. CSRF 攻击的前提是受害者浏览器自动携带 cookie；此端点无 cookie 攻击面
	//   3. 客户端是浏览器 PerformanceObserver，无鉴权
	router.Post("/api/v1/analytics/web-vitals", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"accepted"}`))
	})

	uploadsDir := config.StorageLocalBaseDir()
	if uploadsDir == "" {
		uploadsDir = defaultUploadsDir
	}
	absDir, _ := filepath.Abs(uploadsDir)
	if info, err := os.Stat(absDir); err == nil && info.IsDir() {
		fileServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir(absDir)))
		serveUploads := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 活跃内容（SVG/HTML/JS 等）禁止内联渲染，命中
			// STORAGE_ATTACHMENT_EXTENSIONS 时固定下发附件（手册 4.17.2）。
			if config.IsAttachmentExtension(filepath.Ext(r.URL.Path)) {
				name := path.Base(r.URL.Path)
				w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))
			}
			fileServer.ServeHTTP(w, r)
		})
		// 静态代理只有读语义，按方法注册 GET/HEAD：Handle 的全方法兜底会把
		// POST/PUT/DELETE 等一并变成匿名可达路由，违反启动白名单门禁的
		// 逐条登记契约（rootPublicRoutes 只登记 GET/HEAD）。FileServer 对
		// 其他方法本就返回 405，行为不变，只是把拒绝点移到路由层。
		router.Get("/uploads/*", serveUploads)
		router.Head("/uploads/*", serveUploads)
	}
}

// RegisterManagementRoutes wires the management face: the console and
// administrator APIs (手册 4.1.3).
func RegisterManagementRoutes(router chi.Router, runtime *Runtime) {
	if runtime.Handlers.ReconciliationHandler != nil {
		reconciliationtransport.RegisterChiRoutes(router, runtime.Handlers.ReconciliationHandler)
	}
	certs.RegisterChiRoutes(router, runtime.Handlers.CertHandler)
	captchaproviders.RegisterChiRoutes(router, runtime.Handlers.CaptchaProviderHandler)
	notificationproviders.RegisterChiRoutes(router, runtime.Handlers.NotificationProviderHandler)
	notifications.RegisterChiRoutes(router, runtime.Handlers.NotificationHandler)
	usernotifications.RegisterChiRoutes(router, runtime.Handlers.UserNotificationHandler)
	adapters.RegisterChiRoutes(router, runtime.Handlers.AdapterHandler)
	enforcers.RegisterChiRoutes(router, runtime.Handlers.EnforcerHandler)
	models.RegisterChiRoutes(router, runtime.Handlers.ModelHandler)
	syncers.RegisterChiRoutes(router, runtime.Handlers.SyncerHandler)
	webhooks.RegisterChiRoutes(router, runtime.Handlers.WebhookHandler)
	messagetransport.RegisterChiRoutes(router, runtime.Handlers.MessageHandler)
	ldaps.RegisterChiRoutes(router, runtime.Handlers.LdapHandler)
	ldapsynctransport.RegisterRoutes(router, runtime.Handlers.LdapSyncHandler)
	agents.RegisterChiRoutes(router, runtime.Handlers.AgentHandler)
	forms.RegisterChiRoutes(router, runtime.Handlers.FormHandler)
	applications.RegisterChiRoutes(router, runtime.Handlers.ApplicationHandler)
	gateway.RegisterChiRoutes(router, runtime.Handlers.GatewayHandler)
	groups.RegisterChiRoutes(router, runtime.Handlers.GroupHandler)
	invitations.RegisterChiRoutes(router, runtime.Handlers.InvitationHandler)
	organizations.RegisterChiRoutes(router, runtime.Handlers.OrganizationHandler)
	permissions.RegisterChiRoutes(router, runtime.Handlers.PermissionHandler)
	orders.RegisterChiRoutes(router, runtime.Handlers.OrderHandler)
	payments.RegisterChiRoutes(router, runtime.Handlers.PaymentHandler)
	payments.RegisterNotifyChiRoutes(router, runtime.Handlers.PaymentNotifyHandler)
	payments.RegisterReconChiRoutes(router, runtime.Handlers.ReconHandler)
	payments.RegisterChannelConfigChiRoutes(router, runtime.Handlers.ChannelConfigHandler)
	payments.RegisterChannelCatalogChiRoutes(router, runtime.Handlers.ChannelCatalogHandler)
	payments.RegisterAttemptsChiRoutes(router, runtime.Handlers.AttemptsHandler)
	payments.RegisterRefundsChiRoutes(router, runtime.Handlers.RefundsHandler)
	payments.RegisterCallbackReceiptsChiRoutes(router, runtime.Handlers.CallbackReceiptsHandler)
	plans.RegisterChiRoutes(router, runtime.Handlers.PlanHandler)
	platformadmintransport.RegisterChiRoutes(router, runtime.Handlers.PlatformAdminHandler)
	pricings.RegisterChiRoutes(router, runtime.Handlers.PricingHandler)
	products.RegisterChiRoutes(router, runtime.Handlers.ProductHandler)
	providers.RegisterChiRoutes(router, runtime.Handlers.ProviderHandler)
	records.RegisterChiRoutes(router, runtime.Handlers.RecordHandler)
	resources.RegisterChiRoutes(router, runtime.Handlers.ResourceHandler)
	roles.RegisterChiRoutes(router, runtime.Handlers.RoleHandler)
	sessions.RegisterChiRoutes(router, runtime.Handlers.SessionHandler)
	subscriptions.RegisterChiRoutes(router, runtime.Handlers.SubscriptionHandler)
	tickets.RegisterChiRoutes(router, runtime.Handlers.TicketHandler)
	tokens.RegisterChiRoutes(router, runtime.Handlers.TokenHandler)
	userstransport.RegisterChiRoutes(router, runtime.Handlers.UserHandler)
	batchtransport.RegisterChiRoutes(router, runtime.Handlers.BatchHandler)
	coupons.RegisterChiRoutes(router, runtime.Handlers.CouponHandler)
	dashboard.RegisterChiRoutes(router, runtime.Handlers.DashboardHandler)
	b2btransport.RegisterChiRoutes(router, runtime.Handlers.B2BHandler)
	entries.RegisterChiRoutes(router, runtime.Handlers.EntryHandler)
	mcpservers.RegisterChiRoutes(router, runtime.Handlers.McpServerHandler)
	radiusaccountings.RegisterChiRoutes(router, runtime.Handlers.RadiusAccountingHandler)
	verifications.RegisterChiRoutes(router, runtime.Handlers.VerificationHandler)
	faceidproviders.RegisterChiRoutes(router, runtime.Handlers.FaceIdProviderHandler)
	rules.RegisterChiRoutes(router, runtime.Handlers.RuleHandler)
	radiusconfigs.RegisterChiRoutes(router, runtime.Handlers.RadiusConfigHandler)
	transactionstransport.RegisterChiRoutes(router, runtime.Handlers.TransactionHandler)
	keystransport.RegisterChiRoutes(router, runtime.Handlers.KeyHandler)
	searchtransport.RegisterChiRoutes(router, runtime.Handlers.SearchHandler)
	rebac.RegisterChiRoutes(router, runtime.Handlers.RebacHandler)
	workloadtransport.RegisterChiRoutes(router, runtime.Handlers.WorkloadHandler)
	metering.RegisterChiRoutes(router, runtime.Handlers.MeteringHandler)
	iga.RegisterChiRoutes(router, runtime.Handlers.IGAHandler)
	risk.RegisterChiRoutes(router, runtime.Handlers.RiskHandler)
	privacytransport.RegisterChiRoutes(router, runtime.Handlers.PrivacyHandler)
	idvtransport.RegisterChiRoutes(router, runtime.Handlers.IDVHandler)
	idvtransport.RegisterCallbackChiRoutes(router, runtime.Handlers.IDVHandler)
	webhookeventstransport.RegisterChiRoutes(router, runtime.Handlers.WebhookEventHandler)
	actionstransport.RegisterChiRoutes(router, runtime.Handlers.ActionsHandler)

	providers.RegisterGlobalChiRoutes(router, runtime.Handlers.ProviderHandler)
	certs.RegisterGlobalChiRoutes(router, runtime.Handlers.CertHandler)
	keystransport.RegisterGlobalChiRoutes(router, runtime.Handlers.KeyHandler)
	userstransport.RegisterGlobalChiRoutes(router, runtime.Handlers.UserHandler)

	radiustransport.RegisterChiRoutes(router, runtime.Handlers.RADIUSHandler)

	// AI Gateway 管理面 CRUD（Round 5）。挂在 management face，
	// 自动走 session + Casbin middleware。resourceFromPath 会从
	// `/api/v1/aigateway/*` 提取 resource="aigateway"。
	aigatewaytrans.RegisterAdminChiRoutes(router, runtime.Handlers.AIGatewayAdminHandler)
}

// RegisterOpsRoutes wires instance-level ops endpoints (metrics / readiness).
//
// 为什么不用 PEP？/metrics 和 /readyz 是实例级端点，不是租户级。
// PEP 的 Casbin model 是 domain-based RBAC (matcher: r.org == p.org)，
// 让真实租户用户访问会全部 403（domain mismatch）。
//
// 因此 ops 端点单独用 MetricsTokenMiddleware 保护 —
// 需要 Header: X-Metrics-Token 或 Authorization: Bearer <METRICS_SCRAPE_TOKEN env>
func RegisterOpsRoutes(router chi.Router, runtime *Runtime) {
	token := config.MetricsScrapeToken()
	ops := router.Group(nil)
	ops.Use(mw.MetricsTokenMiddleware(token))

	// Prometheus scrape endpoint
	ops.With(middleware.NoCache).Get("/metrics", promhttp.Handler().ServeHTTP)

	// Readiness probe — requires the active database driver to be alive (same token-based auth).
	ops.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		switch {
		case runtime.DB.Pool != nil:
			if err := runtime.DB.Pool.Ping(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"degraded: db ping failed"}`))
				return
			}
		case runtime.DB.MySQLPool != nil:
			if err := runtime.DB.MySQLPool.PingContext(r.Context()); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{"status":"degraded: mysql ping failed"}`))
				return
			}
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"degraded: db unavailable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// pprof endpoints — same token-protected ops group
	ops.Get("/debug/pprof/", http.HandlerFunc(pprof.Index))
	ops.Get("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	ops.Get("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	ops.Get("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	ops.Get("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))

	// System-level routes under ops — handler 带 DB pool
	systemtransport.RegisterChiRoutes(ops, runtime.DB.Pool)
}
