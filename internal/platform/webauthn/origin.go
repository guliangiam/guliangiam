package webauthn

import (
	"net/url"
	"strings"
)

// 来源（origin）校验：《开发设计》31.3.2(a) 要求「origin 全等」，31.2 缺陷表登记了
// 「collectedClientData 解析了 Origin 字段但 VerifyAssertion 全函数无比对」。
//
// 为什么必须比对：type 与 challenge 只能证明「浏览器为这次挑战签了名」，不能证明
// 「签名发生在我们的站点上」。攻击者可以把 challenge 拿到自己的页面里诱导用户完成
// 断言，再把结果交给 finish——没有 origin 校验时这条路径完全可行。

// NormalizeOrigin 把来源折算成可比较形态：scheme/host 小写、默认端口省略、忽略路径与
// 末尾斜杠。无法解析成「scheme://host」时返回空串（空串永不匹配）。
//
// 含通配符的条目（CORS 白名单允许 `https://*.父域`）在这里一律返回空串：WebAuthn 断言的
// 作用域由 RP ID 决定，来源若允许任意子域，等于把「哪个站点能替用户签名」放宽到整个父域。
func NormalizeOrigin(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.Contains(trimmed, "*") {
		return ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Host)

	// 默认端口省略：https://a.com:443 与 https://a.com 是同一个来源。
	if (scheme == "https" && strings.HasSuffix(host, ":443")) ||
		(scheme == "http" && strings.HasSuffix(host, ":80")) {
		host = host[:strings.LastIndex(host, ":")]
	}

	return scheme + "://" + host
}

// OriginOfURL 把配置里的 URL（如 ISSUER_URL）折算成来源，供同源部署的允许清单使用；
// 空串或不可解析时返回空串。
func OriginOfURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return NormalizeOrigin(parsed.Scheme + "://" + parsed.Host)
}

// MatchOrigin 报告 origin 是否命中 allowed 中的某一条（全等比对；两侧都先规范化）。
func MatchOrigin(origin string, allowed []string) bool {
	normalized := NormalizeOrigin(origin)
	if normalized == "" {
		return false
	}
	for _, candidate := range allowed {
		if NormalizeOrigin(candidate) == normalized {
			return true
		}
	}
	return false
}
