package applications

import (
	"net/url"
	"strings"
)

// NormalizeReverseProxyHost 把 domain / otherDomains / upstreamHost 收敛成可比较的
// 「小写 host[:port]」：去 scheme、去 path/query/fragment、去末尾点，保留端口。
//
// 它只服务于冲突比较（主域名 vs otherDomains vs upstreamHost），不做格式校验：
// 无法解析的输入退回 trim + 小写，由写侧校验负责拒绝。
func NormalizeReverseProxyHost(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	// 绝对 URL 取 authority；其余按裸 host[:port] 处理。
	candidate := trimmed
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		candidate = parsed.Host
	} else if index := strings.IndexAny(trimmed, "/?#"); index >= 0 {
		candidate = trimmed[:index]
	}
	return trimHostTrailingDot(strings.ToLower(candidate))
}

// ReverseProxyHostOnly 去掉 host[:port] 的端口。自环判定不关心端口：
// 指向同一主机的任意端口仍然是「代理指向自己」。
func ReverseProxyHostOnly(normalized string) string {
	if normalized == "" {
		return ""
	}
	if strings.HasPrefix(normalized, "[") {
		if end := strings.LastIndex(normalized, "]"); end > 0 {
			return normalized[:end+1]
		}
		return normalized
	}
	// 未加方括号的 IPv6 字面量含多个冒号，按原样返回。
	if strings.Count(normalized, ":") > 1 {
		return normalized
	}
	if index := strings.LastIndex(normalized, ":"); index >= 0 {
		return normalized[:index]
	}
	return normalized
}

// trimHostTrailingDot 去掉 host 末尾的点（FQDN 写法），端口保持不动。
func trimHostTrailingDot(value string) string {
	host, port := value, ""
	if index := strings.LastIndex(value, ":"); index >= 0 && !strings.Contains(value[index+1:], ":") && !strings.HasPrefix(value, "[") {
		host, port = value[:index], value[index:]
	}
	return strings.TrimSuffix(host, ".") + port
}
