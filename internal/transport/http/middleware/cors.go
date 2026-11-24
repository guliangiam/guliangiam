package middleware

import (
	"net/http"
	"strconv"
	"strings"

	envcfg "github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ratelimit"
)

// CORS 跨域中间件（《GuliangIAM 开发规范手册》4.16）。
//
// 为什么必须有：控制台是浏览器端 SPA，跨域是默认形态。缺了这层，浏览器会拦掉
// 每一个响应，而 `Access-Control-Expose-Headers` 缺失还会让前端读不到
// `RateLimit-*`/`Retry-After`/`X-Request-Id`——限流反馈与问题定位在前端全部失效，
// 表现为「被限流了但控制台看不到任何线索」。
//
// 安全红线（任何一条都不允许放宽）：
//   - `Access-Control-Allow-Origin` 只回显白名单中与请求 Origin **精确匹配**的单个来源，
//     绝不回显任意 Origin、绝不使用裸 `*`；
//   - `Access-Control-Allow-Headers` 取服务端固定清单，**不反射** `Access-Control-Request-Headers`；
//   - 凭证模式下更不允许通配；`*` 与 `Allow-Credentials: true` 同用会被浏览器拒绝，
//     本身就是配置缺陷。
//
// 允许来源白名单（`CORS_ALLOWED_ORIGINS`，逗号分隔）支持两种形态：
//   - 精确来源：`https://console.example.com`、`http://localhost:7101`
//   - HTTPS 子域通配：`https://*.example.com`（父域后缀精确匹配，`evilexample.com`
//     因缺少点边界不匹配）
const (
	// EnvCORSAllowedOrigins 是允许来源白名单的环境变量名（环境变量注册表 D.4）。
	EnvCORSAllowedOrigins = "CORS_ALLOWED_ORIGINS"
	// EnvCORSAllowedMethods 是预检允许方法白名单的环境变量名（环境变量注册表 D.4）。
	EnvCORSAllowedMethods = "CORS_ALLOWED_METHODS"

	// corsDefaultMaxAgeSeconds 是预检结果缓存秒数。
	// 86400（24h）是 Chrome / Firefox 上限，最大化节省预检次数。
	corsDefaultMaxAgeSeconds = 86400
)

// corsDefaultAllowedMethods 是 `CORS_ALLOWED_METHODS` 未设置时的默认值（环境变量注册表 D.4）。
var corsDefaultAllowedMethods = []string{
	http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete, http.MethodOptions,
}

// corsDefaultAllowedHeaders 是服务端固定允许的请求头清单（4.16.1 第 3 条）。
// 这里是**清单**而非反射来源：客户端宣称什么就放行什么是 CSRF 的经典温床。
var corsDefaultAllowedHeaders = []string{
	"Authorization",
	"Content-Type",
	"Accept",
	"Accept-Language",
	"Idempotency-Key",
	"If-Match",
	"If-None-Match",
	"X-Request-Id",
	"X-CSRF-Token",
}

// corsExposedHeaders 是跨域可读的响应头（4.16.1 第 4 条）。
// `RateLimit-*` 部分直接取自 ratelimit 包的字段常量，避免文档清单与实现漂移。
//
// `ETag` 必须在内：控制台默认跨域部署（见下 `CORS_ALLOWED_ORIGINS` 说明），而
// 42.3.11 要求前端轮询携带 `If-None-Match`——读不到 `ETag` 就无从携带，
// 条件请求会在跨域形态下退化为「无条件轮询 + 每次全量响应」。
//
// `Location` 必须在内：创建端点的 201 用它指向新资源（route.WriteCreated），
// 浏览器不暴露该头时控制台只能靠响应体里的 id 自行拼接路径。
var corsExposedHeaders = []string{
	ratelimit.HeaderLimit,
	ratelimit.HeaderRemaining,
	ratelimit.HeaderReset,
	ratelimit.HeaderPolicy,
	ratelimit.HeaderRetryAfter,
	"ETag",
	"Location",
	"X-Request-Id",
}

// CORSConfig 是 CORS 中间件配置。零值即「同源不放行」，这是安全的默认值。
type CORSConfig struct {
	// AllowedOrigins 是允许来源模式清单（精确来源或 `https://*.父域`）。
	AllowedOrigins []string
	// AllowedMethods 是预检允许的 HTTP 方法。
	AllowedMethods []string
	// AllowCredentials 决定是否下发 `Access-Control-Allow-Credentials: true`。
	// 控制台使用 Cookie 会话，因此启用；启用时任何来源都必须是精确白名单条目。
	AllowCredentials bool
	// MaxAgeSeconds 是预检结果缓存秒数，0 表示使用默认值 86400（Chrome / Firefox 上限）。
	MaxAgeSeconds int
}

// CORSConfigFromEnv 从环境变量装配 CORS 配置（环境变量注册表 D.4）。
// 未设置 `CORS_ALLOWED_ORIGINS` 时返回「不启用跨域」的配置：默认同源不放行，
// 宁可前端在联调期报跨域错误，也不允许生产环境因漏配而静默放行任意来源。
func CORSConfigFromEnv() CORSConfig {
	cfg := CORSConfig{
		AllowedOrigins:   splitAndTrim(envcfg.CORSAllowedOrigins()),
		AllowedMethods:   splitAndTrim(envcfg.CORSAllowedMethods()),
		MaxAgeSeconds:    envcfg.CORSMaxAgeSeconds(),
		AllowCredentials: true,
	}
	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = corsDefaultAllowedMethods
	}
	return cfg
}

// Enabled 报告白名单是否非空。空白名单时中间件是空操作，仅补 `Vary: Origin`。
func (c CORSConfig) Enabled() bool { return len(c.AllowedOrigins) > 0 }

// Validate 在启动阶段拒绝非法来源模式。非法白名单必须在进程启动时就失败，
// 而不是等到第一个跨域请求时静默不放行——后者的排查成本远高于启动失败。
func (c CORSConfig) Validate() error {
	for _, pattern := range c.AllowedOrigins {
		if reason := invalidOriginReason(pattern); reason != "" {
			return apperrors.NewInvalidArgument("CORS 配置非法（" + EnvCORSAllowedOrigins + "）：" + reason)
		}
	}
	return nil
}

// invalidOriginReason 返回空串表示合法。返回具体原因是必要的：只说「来源非法」，
// 运维需要逐个白名单条目二分排查，而这一眼就能定位。
func invalidOriginReason(pattern string) string {
	switch {
	case pattern == "*":
		return "禁止裸通配来源 `*`：无法与凭证模式共存，且等于放行任意站点"
	case !strings.HasPrefix(pattern, "https://") && !strings.HasPrefix(pattern, "http://"):
		return "来源必须带 scheme（" + pattern + "）"
	case strings.HasPrefix(pattern, "http://*"):
		return "禁止 http 通配来源：通配子域仅允许 https（" + pattern + "）"
	}
	rest, ok := strings.CutPrefix(pattern, "https://*.")
	if !ok {
		if strings.Contains(pattern, "*") {
			return "通配符只能出现在 https://*. 形式的子域位置（" + pattern + "）"
		}
		return ""
	}
	if rest == "" || strings.ContainsAny(rest, "*/") {
		return "非法通配来源（" + pattern + "）"
	}
	return ""
}

// matchOrigin 返回 origin 是否命中白名单。
//
// 精确条目按字符串相等比较；通配条目要求前缀为「https://」加**非空**的子域部分，
// 且其后紧跟 `.父域`。点边界是关键：`https://*.example.com` 不匹配
// `https://evilexample.com`（没有点）也不匹配 `https://example.com`（子域为空）。
func (c CORSConfig) matchOrigin(origin string) bool {
	for _, pattern := range c.AllowedOrigins {
		if suffix, ok := strings.CutPrefix(pattern, "https://*."); ok {
			rest, ok := strings.CutPrefix(origin, "https://")
			if !ok {
				continue
			}
			sub, ok := strings.CutSuffix(rest, "."+suffix)
			if !ok || sub == "" {
				continue
			}
			return true
		}
		if pattern == origin {
			return true
		}
	}
	return false
}

// CORS 返回配置好的跨域中间件。
//
// 预检（OPTIONS + `Access-Control-Request-Method`）由本中间件**终结**，不再下发到
// 业务链：预检不应触发鉴权、限流计数或审计写入，否则一次真实请求会被记两次。
// 实际请求只补响应头后继续。
func CORS(config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}

			// 白名单存在与否都必须声明 `Vary: Origin`：响应体会随 Origin 变化，
			// 漏掉会让共享缓存把一个站点的响应喂给另一个站点。
			appendVary(w, "Origin")
			if !config.matchOrigin(origin) {
				// 未命中不回显任何 CORS 头，浏览器自行拒绝；服务端不因此报错，
				// 以免把一个前端配置问题升级成后端 4xx。
				next.ServeHTTP(w, r)
				return
			}

			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			if config.AllowCredentials {
				h.Set("Access-Control-Allow-Credentials", "true")
			}
			if len(corsExposedHeaders) > 0 {
				h.Set("Access-Control-Expose-Headers", strings.Join(corsExposedHeaders, ", "))
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				appendVary(w, "Access-Control-Request-Method", "Access-Control-Request-Headers")
				h.Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				h.Set("Access-Control-Allow-Headers", strings.Join(corsDefaultAllowedHeaders, ", "))
				h.Set("Access-Control-Max-Age", strconv.Itoa(corsMaxAge(config.MaxAgeSeconds)))
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func corsMaxAge(configured int) int {
	if configured > 0 {
		return configured
	}
	return corsDefaultMaxAgeSeconds
}

// appendVary 把字段并入单个 `Vary` 头。重复字段会让头部膨胀（每个请求都可能
// 追加一次），同一字段重复出现也违反头部语义。
func appendVary(w http.ResponseWriter, fields ...string) {
	existing := splitAndTrim(w.Header().Get("Vary"))
	seen := make(map[string]bool, len(existing))
	for _, field := range existing {
		seen[field] = true
	}
	for _, field := range fields {
		if !seen[field] {
			seen[field] = true
			existing = append(existing, field)
		}
	}
	w.Header().Set("Vary", strings.Join(existing, ", "))
}

func splitAndTrim(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
