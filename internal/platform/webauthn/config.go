package webauthn

import (
	"strings"

	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
)

// defaultDevelopmentConsoleOrigin 是仓库内 web/vite.config.ts 固定的本地控制台端口。
// 开发态后端通常监听 :8000、控制台监听 :7101，两者端口不同即不同 origin；若只放行
// ISSUER_URL，管理员在「账户安全」里注册 Passkey 会在 origin 校验处失败。这里仅在
// APP_ENV=development 下追加该来源；production/staging 不受影响（必须显式配置
// CORS_ALLOWED_ORIGINS）。
const defaultDevelopmentConsoleOrigin = "http://localhost:7101"

// IsDevelopment 报告当前部署是否为开发态（APP_ENV=development）。
func IsDevelopment() bool {
	return platformconfig.IsDevelopment()
}

// WithDevelopmentOrigins 是开发期来源补全的唯一入口：开发态追加本地控制台来源，
// 其他环境原样返回。
//
// 管理员面（代注册）与自服务/登录面共用这一处默认值，避免两边各写一套后出现
// 「管理员能注册、登录不能用」的割裂。生产路径不注入任何开发来源。
func WithDevelopmentOrigins(origins []string) []string {
	if !IsDevelopment() {
		return origins
	}
	result := make([]string, 0, len(origins)+1)
	result = append(result, origins...)
	return append(result, defaultDevelopmentConsoleOrigin)
}

// ConfiguredAllowedOrigins 返回部署级 WebAuthn origin 白名单：CORS 白名单
// （跨域部署时配置）∪ ISSUER_URL 的来源（同源部署时白名单通常为空）∪
// 开发态本地控制台来源。
//
// 与 auth 服务的 webAuthnAllowedOrigins 读取同一组环境变量
// （CORS_ALLOWED_ORIGINS / ISSUER_URL），不接受请求头推导。
func ConfiguredAllowedOrigins() []string {
	origins := make([]string, 0)
	for _, part := range strings.Split(platformconfig.CORSAllowedOrigins(), ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	if issuerOrigin := OriginOfURL(platformconfig.ConfiguredIssuerURL()); issuerOrigin != "" {
		origins = append(origins, issuerOrigin)
	}
	return WithDevelopmentOrigins(origins)
}

// ConfiguredRelyingPartyID 从配置推导 RP ID（所有来源主机的公共后缀）。
func ConfiguredRelyingPartyID() (string, error) {
	return RelyingPartyIDFor(ConfiguredAllowedOrigins())
}

// DeployRequiresUV 返回部署级 user-verification 开关。它只能收紧要求，
// 组织策略可以进一步收紧，但不能放宽。
func DeployRequiresUV() bool {
	return platformconfig.WebAuthnRequireUV()
}
