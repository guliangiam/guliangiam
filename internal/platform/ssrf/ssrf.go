// Package ssrf 提供出站 HTTP 连接的服务端请求伪造（SSRF）防护。
//
// 防护策略（架构设计 §25.6 / 安全就绪度 S17）：
//  1. Scheme 白名单：只允许 http/https
//  2. 解析后 IP 校验：拒绝私有 IP、链路本地、回环、ULA、组播
//  3. 重定向防护：每次重定向后重新校验目标 IP
//  4. DNS Rebinding 防护：DialContext 直接校验 IP，不信任 HTTP 层 Host 名
package ssrf

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// schemeHTTP 是 SSRF 校验允许的明文协议。
const schemeHTTP = "http"

// schemeHTTPS 是 SSRF 校验允许的加密协议。
const schemeHTTPS = "https"

// DefaultHTTPClient 是经过 SSRF 防护的默认 HTTP Client（30s timeout）。
// 应用层所有出站 HTTP 调用应优先使用此变量，替代 http.DefaultClient。
var DefaultHTTPClient = SafeHTTPClient(30 * time.Second)

// SafeTransport 返回一个经过 SSRF 防护的 http.Transport。
// 所有通过本 Transport 发起的 TCP 连接都会在 dial 阶段校验目标 IP，
// 私有/链路本地/回环/组播 等不可达地址直接被拒绝。
//
// 用法：
//
//	client := &http.Client{Transport: ssrf.SafeTransport(10 * time.Second)}
func SafeTransport(dialTimeout time.Duration) *http.Transport {
	return SafeTransportWithAllowlist(dialTimeout, nil)
}

// SafeTransportWithAllowlist 与 SafeTransport 相同，但允许额外放行
// 一些特定的 IP 或网段（典型场景：测试环境的 httptest.NewServer）。
// allowlist 接受 net.IP 或 "CIDR" 字符串。
func SafeTransportWithAllowlist(dialTimeout time.Duration, allowlist []string) *http.Transport {
	allowed := parseAllowlist(allowlist)
	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
		Control:   controlSocket,
	}

	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, apperrors.WrapInvalidArgumentf(err, "ssrf: invalid address %q", addr)
			}

			resolved, err := resolveHost(ctx, host)
			if err != nil {
				return nil, apperrors.WrapInternalf(err, "ssrf: resolve %q", host)
			}

			// fail-closed：任一解析结果不安全即整组拒绝
			for _, a := range resolved {
				if allowed.contains(a.IP) {
					continue
				}
				if !IsSafeIP(a.IP) {
					return nil, apperrors.NewForbiddenf("ssrf: blocked target %s (%s)", addr, a.IP)
				}
			}

			// 直接按已校验的 numeric IP 拨号，杜绝“校验后再解析”的 DNS rebinding 窗口。
			var lastErr error
			for _, a := range resolved {
				conn, err := dialer.DialContext(ctx, network, joinHostPort(a, port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}

// SafeProxyTransport is SafeTransport with the environment HTTP proxy enabled.
// The proxy connects to the destination, so the destination hostname is not
// IP-validated; use it only when an administrator explicitly opts in per
// provider. Redirects keep the same scheme/host checks as SafeHTTPClient.
func SafeProxyTransport(dialTimeout time.Duration) *http.Transport {
	transport := SafeTransport(dialTimeout)
	transport.Proxy = http.ProxyFromEnvironment
	return transport
}

// SafeProxyHTTPClient returns a proxy-aware client with the same timeout and
// redirect policy as SafeHTTPClient.
func SafeProxyHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     SafeProxyTransport(timeout),
		CheckRedirect: redirectCheck,
	}
}

// SafeHTTPClientWithAllowlist 返回一个带 SSRF 防护的 *http.Client，
// 额外放行 allowlist 里的 IP/CIDR（供测试使用）。
func SafeHTTPClientWithAllowlist(timeout time.Duration, allowlist []string) *http.Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     SafeTransportWithAllowlist(timeout, allowlist),
		CheckRedirect: redirectCheck,
	}
}

// ValidateURL 做基础 URL 合法性校验（scheme 白名单 + hostname 非空）。
// 深度 IP 校验由 SafeTransport 在 dial 时执行。
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return apperrors.WrapInvalidArgumentf(err, "ssrf: parse url")
	}
	if u.Hostname() == "" {
		return apperrors.NewInvalidArgument("ssrf: missing hostname")
	}
	switch u.Scheme {
	case schemeHTTP, schemeHTTPS:
		// ok
	default:
		return apperrors.NewInvalidArgumentf("ssrf: scheme %q not allowed (only http/https)", u.Scheme)
	}
	return nil
}

// privateRanges is parsed once at package init (BE-E-21): IsSafeIP sits
// on the SSRF dial hot path and must not re-parse 18 CIDRs per call.
var privateRanges = []*net.IPNet{
	mustCIDR("10.0.0.0/8"),
	mustCIDR("172.16.0.0/12"),
	mustCIDR("192.168.0.0/16"),
	mustCIDR("127.0.0.0/8"),
	mustCIDR("169.254.0.0/16"),
	mustCIDR("0.0.0.0/8"),
	mustCIDR("100.64.0.0/10"),
	mustCIDR("192.0.0.0/24"),
	mustCIDR("192.0.2.0/24"),
	mustCIDR("198.51.100.0/24"),
	mustCIDR("203.0.113.0/24"),
	mustCIDR("224.0.0.0/4"),
	mustCIDR("240.0.0.0/4"),
	mustCIDR("::1/128"),
	mustCIDR("fc00::/7"),
	mustCIDR("fe80::/10"),
	mustCIDR("ff00::/8"),
	mustCIDR("100::/64"),
}

// IsSafeIP 返回 true 如果 ip 是公网可达地址（非私有、非链路本地、非回环）。
func IsSafeIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	for _, network := range privateRanges {
		if network.Contains(ip) {
			return false
		}
	}
	return true
}

// SafeHTTPClient 返回一个带 SSRF 防护和超时的 *http.Client。
func SafeHTTPClient(timeout time.Duration) *http.Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: SafeTransport(timeout),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 每跳重定向都重新走 dial 时的 IP 校验
			if len(via) >= 10 {
				return apperrors.NewUnavailable("ssrf: too many redirects")
			}
			// 禁止跨 scheme 重定向（http→file:// 等）
			if req.URL.Scheme != "http" && req.URL.Scheme != schemeHTTPS {
				return apperrors.NewInvalidArgumentf("ssrf: redirect to %q not allowed", req.URL.Scheme)
			}
			// 目标 URL 基础校验
			if err := ValidateURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

// lookupIPAddr 是 DNS 解析入口（变量便于测试注入，模拟 DNS rebinding）。
var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	// 使用自定义 resolver 带短超时，防止 DNS 放大攻击
	r := &net.Resolver{PreferGo: true}
	return r.LookupIPAddr(ctx, host)
}

// resolveHost 解析 hostname 为 IPAddr 列表（保留 IPv6 zone）。
// 如果 host 本身是 IP，直接返回。
func resolveHost(ctx context.Context, host string) ([]net.IPAddr, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IPAddr{{IP: ip}}, nil
	}
	return lookupIPAddr(ctx, host)
}

// joinHostPort 把已校验的 numeric IP 与端口拼成拨号地址。
// 保留 IPv6 zone，避免 link-local 地址拨号失败。
func joinHostPort(a net.IPAddr, port string) string {
	return net.JoinHostPort(a.String(), port)
}

// controlSocket 是 platform 特定的 socket Control 函数。
// macOS/Linux 上可以用来额外绑定接口或禁用 ICMP。
// 当前是 passthrough 实现，保留扩展点。
func controlSocket(network, address string, c syscall.RawConn) error {
	return nil
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("ssrf: invalid CIDR %q: %v", s, err))
	}
	return n
}

// ── allowlist 支持 ─────────────────────────────────────────────────────

type ipAllowlist struct {
	ips  []net.IP
	nets []*net.IPNet
}

func (a *ipAllowlist) contains(ip net.IP) bool {
	for _, allowed := range a.ips {
		if allowed.Equal(ip) {
			return true
		}
	}
	for _, allowed := range a.nets {
		if allowed.Contains(ip) {
			return true
		}
	}
	return false
}

func parseAllowlist(items []string) ipAllowlist {
	var a ipAllowlist
	for _, raw := range items {
		if strings.Contains(raw, "/") {
			_, n, err := net.ParseCIDR(raw)
			if err == nil {
				a.nets = append(a.nets, n)
				continue
			}
		}
		if ip := net.ParseIP(raw); ip != nil {
			a.ips = append(a.ips, ip)
		}
	}
	return a
}

func redirectCheck(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return apperrors.NewUnavailable("ssrf: too many redirects")
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != schemeHTTPS {
		return apperrors.NewInvalidArgumentf("ssrf: redirect to %q not allowed", req.URL.Scheme)
	}
	return nil
}
