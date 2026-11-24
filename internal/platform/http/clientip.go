package http

import (
	"net"
	"net/http"
	"strings"

	"github.com/guliangiam/guliangiam/internal/platform/config"
)

// ClientIP 从 HTTP request 提取客户端真实 IP。
//
// 安全默认：只有请求来自 TRUSTED_PROXY_CIDRS 中的反向代理时，才采信
// X-Forwarded-For / X-Real-IP——否则这两个头由客户端任意伪造，可用于绕过
// IP 白名单/限流或伪造会话绑定。未配置任何可信代理时一律使用 RemoteAddr。
//
// 可信代理场景下，从右向左剥离 X-Forwarded-For 中的可信代理；
// 无 XFF 时才使用 X-Real-IP，最后回退 RemoteAddr。
func ClientIP(r *http.Request) string {
	remote := remoteIP(r)
	if !isTrustedProxy(remote) {
		return remote
	}

	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		ips := make([]string, 0, len(parts))
		for _, part := range parts {
			ip := strings.TrimSpace(part)
			// BE-E-16: skip malformed entries instead of discarding the whole
			// header — a forged bad entry must not collapse every client
			// behind the proxy into the proxy's own rate-limit bucket.
			if net.ParseIP(ip) == nil {
				continue
			}
			ips = append(ips, ip)
		}
		client := remote
		for i := len(ips) - 1; i >= 0 && isTrustedProxy(client); i-- {
			client = ips[i]
		}
		return client
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); net.ParseIP(xri) != nil {
		return xri
	}
	return remote
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

// isTrustedProxy reports whether remoteIP belongs to a configured trusted
// proxy CIDR. Matching lives in config.IsTrustedProxyIP so the issuer and
// client-IP paths cannot drift.
func isTrustedProxy(remoteIP string) bool {
	return config.IsTrustedProxyIP(remoteIP)
}
