package auth

import (
	"net/http"
	"strings"
)

// IsTrulyPublicEndpoint is the exported façade of isTrulyPublicRequest for
// path-only contract tests. It answers for GET, the method used by pre-login
// discovery reads.
func IsTrulyPublicEndpoint(path string) bool {
	return isTrulyPublicRequest(http.MethodGet, path)
}

// IsPublicRequest is the method-aware façade used by startup route validation.
func IsPublicRequest(method, path string) bool {
	return isTrulyPublicRequest(method, path)
}

// clientAuthenticatedProtocolPaths are the protocol endpoints that
// authenticate the client itself (RFC 6749 §2.3, RFC 7009, RFC 7523, RFC 7662,
// RFC 8693): the caller presents a client secret or a signed assertion — a
// browser session is never part of the contract. Revocation and introspection
// take a client credential on the same terms as the token endpoints, which is
// why they belong to this list rather than to the session-bound /oauth2/ prefix.
//
// The list is exact and POST-only on purpose. A prefix turns every future
// addition into an implicit anonymous exemption; an exact list keeps each new
// endpoint a deliberate security decision.
var clientAuthenticatedProtocolPaths = map[string]struct{}{
	"/api/v1/auth/tokens/exchange":           {},
	"/api/v1/auth/tokens/jwt-bearer":         {},
	"/api/v1/auth/tokens/token-exchange":     {},
	"/api/v1/auth/tokens/introspect":         {},
	"/api/v1/auth/tokens/client-credentials": {},
	"/api/v1/auth/oauth2/revoke":             {},
	"/api/v1/auth/oauth2/introspect":         {},
	// DEF-40/42：RFC 8628 设备流与 RFC 7591 DCR 的调用方是非浏览器客户端，
	// 没有会话/CSRF cookie 概念；此前只过匿名门、未过 CSRF 门 → 403 不可达。
	"/api/v1/auth/device/authorize": {},
	"/api/v1/auth/device/token":     {},
	"/oauth2/register":              {},
}

// IsClientAuthenticatedProtocolEndpoint reports whether the request targets a
// protocol endpoint whose caller authenticates as a client. It is the single
// source of truth shared by the PEP anonymous gate and the CSRF middleware
// (bootstrap injects it as the exemption predicate), so the two never drift.
func IsClientAuthenticatedProtocolEndpoint(method, path string) bool {
	if method != http.MethodPost {
		return false
	}
	_, ok := clientAuthenticatedProtocolPaths[path]
	return ok
}

// captchaProviderAnonymousMethods is intentionally empty: no request to
// /api/v1/captcha-providers is part of the anonymous login surface. The login
// UI reads captcha configuration from /api/v1/auth/captcha-status and
// /api/v1/auth/captcha (both below); captcha-provider CRUD is authenticated
// tenant administration. Keeping the decision keyed by method + exact path
// namespace means a future read-only exception must be added deliberately.
var captchaProviderAnonymousMethods = map[string]struct{}{}

// isTrulyPublicRequest is the runtime anonymous-gate decision. The captcha
// provider namespace is denied for every method before the path-only switch,
// so GET/PATCH/DELETE can no longer ride the old prefix. All other entries
// keep their existing semantics (protocol and login-initiation endpoints are
// public by design and span multiple methods).
func isTrulyPublicRequest(method, path string) bool {
	if path == "/api/v1/captcha-providers" ||
		strings.HasPrefix(path, "/api/v1/captcha-providers/") {
		_, anonymous := captchaProviderAnonymousMethods[method]
		return anonymous
	}
	// Client-authenticated protocol endpoints are anonymous to the PEP: the
	// handler authenticates the client itself (secret / assertion), so requiring
	// a session would make the standard OAuth2 call unreachable (discovery
	// publishes /tokens/exchange, /oauth2/revoke and /oauth2/introspect).
	// Gate 2a放行不等于不做认证 —— 认证责任在 handler，且这里只匹配 POST +
	// 精确七条，文件顶部 clientAuthenticatedProtocolPaths 是唯一名单。
	if IsClientAuthenticatedProtocolEndpoint(method, path) {
		return true
	}
	// RFC 6749 §4.1.1 front-channel authorize: the browser may hit this before
	// login. The handler itself redirects anonymous callers to the console
	// login page and can never issue a code without a session; the POST variant
	// on the same path stays session-required (console SPA).
	if method == http.MethodGet && path == "/api/v1/auth/authorize" {
		return true
	}
	// Login challenges are deliberately absent: AC-PERM-004 lists
	// /api/v1/auth/login-challenges/* as management-surface entries that must
	// not hit any anonymous exemption. Push MFA initiate/verify are also
	// session-required (isAuthRequiringSession): the password-login second
	// factor completes through /api/v1/auth/login with a challengeId, so no
	// anonymous poll depends on these endpoints.
	return isTrulyPublicEndpoint(path)
}

// parseLoginChallengePath matches real path segments of
// /api/v1/auth/login-challenges/<challengeId>[/approve|/reject], never
// placeholder literals. All three shapes are session-required; approve/reject
// additionally enforce object ownership in the application layer.
func parseLoginChallengePath(path string) (challengeID string, ok bool) {
	const prefix = "/api/v1/auth/login-challenges/"
	rest, found := strings.CutPrefix(path, prefix)
	if !found {
		return "", false
	}
	segments := strings.Split(rest, "/")
	if len(segments) == 0 || !isRealPathSegment(segments[0]) {
		return "", false
	}
	switch len(segments) {
	case 1:
		return segments[0], true
	case 2:
		if segments[1] == "approve" || segments[1] == "reject" {
			return segments[0], true
		}
	}
	return "", false
}

// isRealPathSegment rejects empty segments and route-pattern placeholders such
// as "{challengeId}". Only values a chi router can actually deliver count.
func isRealPathSegment(segment string) bool {
	return segment != "" &&
		!strings.Contains(segment, "{") &&
		!strings.Contains(segment, "}")
}

const idvCallbackPrefix = "/api/v1/idv/callback/"

// isIDVCallbackPath matches exactly one provider segment after the callback
// prefix. It deliberately does not reuse a prefix test: that shape would also
// admit /api/v1/idv/callback/<provider>/<anything>. The route template
// "{provider}" is accepted because routes.go patterns are fed to the public
// whitelist contract test verbatim.
func isIDVCallbackPath(path string) bool {
	provider, found := strings.CutPrefix(path, idvCallbackPrefix)
	if !found {
		return false
	}
	if provider == "{provider}" {
		return true
	}
	return isRealPathSegment(provider) && !strings.Contains(provider, "/")
}

func isLoginChallengePath(path string) bool {
	_, ok := parseLoginChallengePath(path)
	return ok
}

// PublicRuleKind classifies how a path matched the anonymous whitelist.
type PublicRuleKind int

const (
	// PublicRuleExact means the path is enumerated one-by-one (method-agnostic
	// path list or method+path list). It is an explicit decision.
	PublicRuleExact PublicRuleKind = iota
	// PublicRulePattern means the path is enumerated by a narrow template rule
	// (single real segment), still an explicit decision.
	PublicRulePattern
	// PublicRuleFamily means the path is covered by a protocol/namespace prefix
	// family. Startup validation requires such a route to declare itself public
	// or protocol-surface, so a new endpoint cannot inherit anonymity silently.
	PublicRuleFamily
	// PublicRuleNone means the path is not covered by any explicit public rule.
	// Callers must branch on this kind instead of testing the rule string for
	// emptiness, so "not public" cannot be mistaken for an explicit decision.
	PublicRuleNone
)

// PublicRule returns the whitelist rule that makes path anonymous, together
// with its kind. It mirrors isTrulyPublicEndpoint and is consumed by the
// startup whitelist cross-check.
func PublicRule(path string) (string, PublicRuleKind) {
	// Client-authenticated protocol endpoints: exact method+path list.
	if IsClientAuthenticatedProtocolEndpoint(http.MethodPost, path) ||
		IsClientAuthenticatedProtocolEndpoint(http.MethodGet, path) {
		return "exact:" + path, PublicRuleExact
	}
	// IDV callback: exactly one provider segment, never a deep prefix.
	if isIDVCallbackPath(path) {
		return "pattern:" + idvCallbackPrefix + "{provider}", PublicRulePattern
	}
	// QR code polling is a narrow template rule, not a namespace family.
	if strings.HasPrefix(path, "/api/v1/auth/qrcode/") && strings.HasSuffix(path, "/status") {
		return "pattern:/api/v1/auth/qrcode/{codeId}/status", PublicRulePattern
	}
	// Captcha: the three login-surface endpoints are exact; anything else that
	// shares the prefix is a family match and must self-declare.
	if strings.HasPrefix(path, "/api/v1/auth/captcha") {
		switch path {
		case "/api/v1/auth/captcha", "/api/v1/auth/captcha-status", "/api/v1/auth/captcha/verify":
			return "exact:" + path, PublicRuleExact
		}
		return "prefix:/api/v1/auth/captcha", PublicRuleFamily
	}
	// Login-initiation endpoints are enumerated one-by-one.
	if isAuthLoginFlowPublic(path) {
		return "exact:" + path, PublicRuleExact
	}
	switch {
	case path == "/healthz":
		return "exact:/healthz", PublicRuleExact
	case strings.HasPrefix(path, "/.well-known/"),
		strings.HasPrefix(path, "/openapi/"),
		strings.HasPrefix(path, "/uploads/"):
		return "prefix:" + firstMatchingPrefix(path, "/.well-known/", "/openapi/", "/uploads/"), PublicRuleFamily
	case strings.HasPrefix(path, "/cas/"),
		strings.HasPrefix(path, "/oauth2/"),
		strings.HasPrefix(path, "/kerberos/"),
		strings.HasPrefix(path, "/scim/v2/"),
		strings.HasPrefix(path, "/api/v1/saml/"),
		strings.HasPrefix(path, "/api/v1/auth/saml/"):
		return "prefix:" + firstMatchingPrefix(path, "/cas/", "/oauth2/", "/kerberos/", "/scim/v2/", "/api/v1/saml/", "/api/v1/auth/saml/"), PublicRuleFamily
	case strings.HasPrefix(path, "/api/v1/dcr/"):
		return "prefix:/api/v1/dcr/", PublicRuleFamily
	case path == "/api/v1/applications/resolve":
		return "exact:/api/v1/applications/resolve", PublicRuleExact
	case strings.HasPrefix(path, "/api/v1/applications/by-client-id/"):
		return "prefix:/api/v1/applications/by-client-id/", PublicRuleFamily
	case path == "/api/v1/organizations/resolve":
		return "exact:/api/v1/organizations/resolve", PublicRuleExact
	case strings.HasPrefix(path, "/api/v1/organizations/by-slug/"):
		return "prefix:/api/v1/organizations/by-slug/", PublicRuleFamily
	case path == "/api/v1/invitations/verify":
		return "exact:/api/v1/invitations/verify", PublicRuleExact
	case path == "/api/v1/system/health":
		return "exact:/api/v1/system/health", PublicRuleExact
	case strings.HasPrefix(path, "/api/v1/payments/notify/"):
		return "prefix:/api/v1/payments/notify/", PublicRuleFamily
	}
	return "", PublicRuleNone
}

func firstMatchingPrefix(path string, prefixes ...string) string {
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return prefix
		}
	}
	return ""
}

// isTrulyPublicEndpoint returns true for endpoints that require NO authentication.
// These are endpoints where a caller without any session or token may legitimately
// interact (discovery, login initiation, protocol handshake).
//
// 安全纪律（架构 §8.8 fail-close）：
//   - 每一条都必须说明"为什么完全不需要认证"
//   - 禁止用通配前缀覆盖 auth 端点（auth 端点拆到 isAuthRequiringSession）
//   - 新增条目 = 安全决策 — 必须附威胁模型分析
func isTrulyPublicEndpoint(path string) bool {
	switch {
	// Infrastructure — 机器可读 metadata，无业务敏感数据
	case strings.HasPrefix(path, "/.well-known/"),
		path == "/healthz",
		strings.HasPrefix(path, "/openapi/"),
		strings.HasPrefix(path, "/uploads/"):
		return true

	// Protocol endpoints — 各自实现认证/授权，不走 PEP
	// （OIDC/CAS/OAuth2/SAML/Kerberos/SCIM 握手本身是获取 session 的入口）
	case strings.HasPrefix(path, "/cas/"),
		strings.HasPrefix(path, "/oauth2/"),
		strings.HasPrefix(path, "/kerberos/"),
		strings.HasPrefix(path, "/scim/v2/"),
		strings.HasPrefix(path, "/api/v1/saml/"),
		strings.HasPrefix(path, "/api/v1/auth/saml/"):
		return true

	// Auth 登录流端点 — 本身就是获取 session 的入口，完全公开
	// 规则："不携带 session 也能发起的认证握手"
	case isAuthLoginFlowPublic(path):
		return true

	// Dynamic Client Registration (OAuth2 RFC 7591) — 公开注册端点
	case strings.HasPrefix(path, "/api/v1/dcr/"):
		return true

	// Application resolution — pre-login discovery，必须公开
	case path == "/api/v1/applications/resolve",
		strings.HasPrefix(path, "/api/v1/applications/by-client-id/"):
		return true

	// Organization resolution — pre-login discovery（品牌配置、默认应用、主题），必须公开
	case path == "/api/v1/organizations/resolve",
		strings.HasPrefix(path, "/api/v1/organizations/by-slug/"):
		return true

	// Captcha providers are intentionally absent: the login surface uses
	// /api/v1/auth/captcha-status and /api/v1/auth/captcha, while provider
	// CRUD is authenticated administration. See isTrulyPublicRequest.

	// Invitation verification — 公开链接
	case path == "/api/v1/invitations/verify":
		return true

	// System health — 运维探测
	case path == "/api/v1/system/health":
		return true

	// Payment provider callback — 第三方通知用签名验证，不走 PEP
	case strings.HasPrefix(path, "/api/v1/payments/notify/"):
		return true

	// Identity provider callback — 第三方 IDV 通知用 webhook 签名验证，不走 PEP。
	// 威胁模型：回调来自无法持有会话的外部服务；因此以原始字节 + 租户
	// WebhookSecret 的 HMAC 验签作为唯一认证，失败一律 4xx，未配置 secret 直接拒绝。
	// 精确匹配单段 provider，不用前缀通配：前缀会同时放行
	// /api/v1/idv/callback/<provider>/<subpath>，并把公开边界与受保护的
	// /api/v1/idv/verifications 资源混在一起。
	case isIDVCallbackPath(path):
		return true
	}
	return false
}

// isAuthLoginFlowPublic returns true for /api/v1/auth/* endpoints that are
// part of the login/authentication initiation flow — callers without a session
// may legitimately interact with them (they are the entry point to obtain a session).
//
// 安全边界：此函数**只**覆盖 auth 包里 "发起认证" 的端点。任何 auth 端点不在此列表
// 也不在 isAuthRequiringSession 列表里 → 走正常 Casbin 流程（如果没有 policy → 403）。
func isAuthLoginFlowPublic(path string) bool {
	if !strings.HasPrefix(path, "/api/v1/auth/") {
		return false
	}
	switch {
	// 基础登录
	case path == "/api/v1/auth/login",
		path == "/api/v1/auth/signup",
		path == "/api/v1/auth/native-sso-complete",
		path == "/api/v1/auth/contact",
		path == "/api/v1/auth/reset-password",
		// 找回密码新版路径：与 legacy /reset-password 同为匿名握手入口
		// （双轨并存，routes.go 同步声明 Permission="public"）。不在白名单
		// 时匿名找回会被 Gate 拦成 401，前端找回页只调这条路径（R2-B01）。
		path == "/api/v1/auth/password/reset",

		// 验证码发送/校验 — 注册、忘记密码、绑定邮箱/手机等场景在登录前
		// 也要调用：routes.go 声明 Permission="public"，与 login/signup/reset-password
		// 同属"发起认证"流程入口。发送和校验均不持 session 即可安全调用，且
		// 有 per-recipient + per-IP 限流保护。
		path == "/api/v1/auth/verification-code/send",
		path == "/api/v1/auth/verification-code/verify",

		// Social login (OAuth/SAML callback)
		path == "/api/v1/auth/social/callback",
		path == "/api/v1/auth/social/login",
		path == "/api/v1/auth/social/authorize",

		// LDAP login
		path == "/api/v1/auth/login/ldap",

		// QR code login (initiation + polling + scan — confirm/exchange need session).
		//
		// 轮询端点必须在两种形状上都命中：本白名单同时被运行期（真实 URL，如
		// /api/v1/auth/qrcode/<id>/status）与合约测试（routes.go 的模式串
		// /api/v1/auth/qrcode/{codeId}/status）消费。此前只写了模式串形状，真实路径
		// 既不在公开白名单、也不在 isAuthRequiringSession，匿名 PC 轮询会落进
		// Gate 3 拿到 401——扫码登录在创建二维码之后就走不下去。
		path == "/api/v1/auth/qrcode",
		path == "/api/v1/auth/qrcode/scan",
		strings.HasPrefix(path, "/api/v1/auth/qrcode/") && strings.HasSuffix(path, "/status"),
		// exchange 也属登录握手：会话在 exchange 才签发（对齐 /faceid/complete），
		// 要求调用方先有会话等于要求「先登录才能登录」。
		path == "/api/v1/auth/qrcode/exchange",

		// Face ID login：三步握手全程匿名，会话在 complete 才签发（对齐
		// qrcode exchange 与 webauthn finish）。complete 要求会话等于「先登录
		// 才能登录」；一次性 challenge 绑定由 faceid→verify 服务端保证。
		path == "/api/v1/auth/faceid",
		path == "/api/v1/auth/faceid/verify",
		path == "/api/v1/auth/faceid/complete",

		// WebAuthn（Passkey）登录：begin/finish 是同一次断言握手的两个半边，
		// 会话在 finish 才签发（对齐 /faceid/complete 与扫码 exchange）。
		// 绑定关系由服务端一次性 challenge（begin 生成、finish 取出即删）保证，
		// 因此匿名调用不构成「任意 username 直接换会话」。
		// registration/* 不在公开清单：注册凭证需要已认证会话（见 isAuthRequiringSession）。
		path == "/api/v1/auth/webauthn/authentication/begin",
		path == "/api/v1/auth/webauthn/authentication/finish",

		// DPoP nonce issuance is anonymous by RFC 9449; the nonce alone grants nothing.
		path == "/api/v1/auth/dpop/nonce",

		// RFC 8628 device flow: the device has no browser session when it starts
		// the flow or polls for the token. User verification remains session-bound.
		path == "/api/v1/auth/device/authorize",
		path == "/api/v1/auth/device/token",

		// Remember-me exchange consumes a single-use rotating token, not a session.
		path == "/api/v1/auth/remember-me/exchange",

		// Web3 login
		path == "/api/v1/auth/web3",
		path == "/api/v1/auth/web3/verify",

		// Login challenges: the GET poll is decided by isTrulyPublicRequest
		// (method + concrete segment); approve/reject require a session.

		// Captcha — 与 PublicRule 相同的三条 exact；其余 /api/v1/auth/captcha*
		// 形状归 Family，须路由自声明，运行期不再整前缀匿名（BE-10）。
		path == "/api/v1/auth/captcha",
		path == "/api/v1/auth/captcha-status",
		path == "/api/v1/auth/captcha/verify",

		// Application login page aggregation
		path == "/api/v1/auth/app-login":
		return true
	}
	return false
}

// isAuthRequiringSession returns true for auth endpoints that need a valid session
// but SKIP Casbin PEP enforcement. These are endpoints inside /api/v1/auth/* that
// are NOT "truly public" — they operate on the authenticated user's own session.
//
// 为什么跳过 Casbin？auth 端点内部已经在做更细粒度的认证流程（状态机、MFA、
// session rotation），额外套一层静态 RBAC 反而会与状态机决策冲突。
// 但 **session context 必须存在** — 没有就是 401（fail-close）。
//
// 安全边界：此函数只匹配 /api/v1/auth/ 下需要 session 的端点。
// 任何其他 /api/v1/auth/* 端点如果不在此列表里 → 走正常 Casbin 流程。
func isAuthRequiringSession(path string) bool {
	// Personal notification center (self-service inbox): the endpoints operate
	// only on the caller's own rows and are reachable by every signed-in user,
	// so they need a session but must not go through Casbin (ordinary members
	// carry no notifications:* policy). Ownership is enforced in the
	// application layer by deriving organization/user from the trusted session.
	if strings.HasPrefix(path, "/api/v1/me/notifications") ||
		strings.HasPrefix(path, "/api/v1/me/notification-preferences") ||
		strings.HasPrefix(path, "/api/v1/me/notification-channel-bindings") {
		return true
	}

	// 快速排除非 auth 端点
	if !strings.HasPrefix(path, "/api/v1/auth/") {
		return false
	}

	// 需要 session 但跳过 Casbin 的 auth 端点
	// 规则："端点行为依赖调用者自身的 session 状态"
	switch {
	case path == "/api/v1/auth/account",
		path == "/api/v1/auth/permissions",
		path == "/api/v1/auth/password/check",
		path == "/api/v1/auth/password/change",
		path == "/api/v1/auth/verify-code",
		path == "/api/v1/auth/exit-impersonate",
		path == "/api/v1/auth/authorize",
		path == "/api/v1/auth/sso-logout",
		path == "/api/v1/auth/frontchannel-logout",

		// Consent management — 操作当前用户的授权
		strings.HasPrefix(path, "/api/v1/auth/consents/"),

		// Account self-service face — 普通成员管理本人 MFA / 第三方身份 / 授权同意。
		// 身份只从可信会话取（users.chi_selfservice.go），应用层做归属校验；这里
		// 强制要求 session 存在但跳过 Casbin（无需 users:* 管理权限）。
		// 注意：mfa-setup/* 与 mfa/push/* 均为精确匹配，互不串扰。
		path == "/api/v1/auth/mfa-setup/initiate",
		path == "/api/v1/auth/mfa-setup/verify",
		path == "/api/v1/auth/mfa-setup/enable",
		path == "/api/v1/auth/mfa-methods",
		path == "/api/v1/auth/preferred-mfa",
		path == "/api/v1/auth/consents",
		strings.HasPrefix(path, "/api/v1/auth/identities/"),

		// Account self-service profile / real-name / FaceID — 本人档案部分合并、
		// 实名认证发起与底图采集/吊销。精确匹配：faceids（复数）是会话自助面，
		// 公开登录握手 faceid（单数）仍走 isAuthLoginFlowPublic，互不串扰。
		path == "/api/v1/auth/profile",
		path == "/api/v1/auth/verify-identification",
		path == "/api/v1/auth/faceids",

		// Token endpoints are deliberately absent: /api/v1/auth/tokens/* is the
		// client-authenticated protocol surface
		// (IsClientAuthenticatedProtocolEndpoint).
		// Token list/revoke for the current session live on the management face
		// (/api/v1/tokens), not here.

		// Login challenges — AC-PERM-004: no anonymous exemption. Both the
		// read-only poll and approve/reject require a session but skip Casbin
		// (object-level ownership is enforced in the application layer).
		// 真实段解析而不是 "contains + suffix"：字面量模式串与其他前缀不命中。
		isLoginChallengePath(path),

		// MFA push enable/disable — 注册在案的唯一 enable/disable 形状。
		// 精确路径取代 Contains+suffix（BE-11）：不再命中任意
		// /api/v1/auth/<seg>/mfa/<seg>/enable 深路径。
		path == "/api/v1/auth/mfa/push/enable",
		path == "/api/v1/auth/mfa/push/disable",

		// Push MFA initiate/verify — 操作当前用户自己的推送与挑战（发送推送、
		// 轮询本人 challenge）。登录第二因子走 /api/v1/auth/login + challengeId，
		// 不依赖这两条端点；因此它们必须持有会话，对象归属在应用层校验。
		path == "/api/v1/auth/mfa/push/initiate",
		path == "/api/v1/auth/mfa/push/verify",

		// QR code confirm (after scan) — 需要 session 完成登录；
		// exchange 不在此列：它属登录握手，会话由它签发。
		// 精确路径（BE-11）：路由注册为 /api/v1/auth/qrcode/confirm，
		// Contains 会误放行任意深路径形状。
		path == "/api/v1/auth/qrcode/confirm",

		// WebAuthn 凭证注册（自服务绑定 passkey）— 需要 session 但跳过 Casbin：
		// 操作对象是调用者自己的凭证，且 finish 会在同一事务内轮换当前会话。
		// 认证面（authentication/*）不在此列：它是登录握手，必须匿名可达。
		strings.HasPrefix(path, "/api/v1/auth/webauthn/registration/"),
		strings.HasPrefix(path, "/api/v1/auth/webauthn/credentials"),

		// Protocol-facing auth endpoints that need a session. The two
		// client-authenticated protocol endpoints under /oauth2/ (revoke per
		// RFC 7009, introspect per RFC 7662) are not session-bound; asking the
		// shared predicate keeps that decision in one place instead of copying
		// the two path literals here.
		strings.HasPrefix(path, "/api/v1/auth/oauth2/") && !IsClientAuthenticatedProtocolEndpoint(http.MethodPost, path),
		strings.HasPrefix(path, "/api/v1/auth/cas/"),
		strings.HasPrefix(path, "/api/v1/auth/kerberos/"),
		path == "/api/v1/auth/device/verify",
		path == "/api/v1/auth/device/cancel":
		return true
	}
	return false
}
