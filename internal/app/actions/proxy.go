package actions

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// OutboundProxy is the egress gatekeeper between a sandboxed action and
// the outside world. The WASM host layer (Iteration 2) and the Dispatcher
// both route outbound HTTP through this interface so host filtering lives
// in exactly one place.
type OutboundProxy interface {
	// Allowed returns true iff the given host passes the whitelist
	// (suffix-match, case-insensitive). An empty host is never allowed.
	Allowed(host string) bool

	// GET fetches the given URL. Returns ErrProxyDenied if the host is
	// not whitelisted. The ctx deadline is respected by the underlying
	// http.Client.
	Get(ctx context.Context, rawURL string) ([]byte, error)

	// POST sends body to the given URL. Same host-policy semantics as Get.
	Post(ctx context.Context, rawURL string, body []byte) ([]byte, error)
}

// maxProxyResponseBytes 限制动作沙箱出站响应大小，防止异常上游耗尽内存。
const maxProxyResponseBytes int64 = 1 << 20 // 1 MiB

// ErrProxyDenied is returned when the sandbox tries to reach a host that
// isn't in the configured whitelist. Dispatcher logs this as a technical
// failure rather than a business deny.
var ErrProxyDenied = errors.New("outbound host denied by proxy whitelist")

// NoopProxy disables all egress. Used by default so sandboxed actions
// cannot "phone home" unless an ops-configured WhitelistProxy is wired in.
type NoopProxy struct{}

// Allowed always returns false — nothing is permitted.
func (NoopProxy) Allowed(string) bool { return false }

// Get returns ErrProxyDenied.
func (NoopProxy) Get(_ context.Context, _ string) ([]byte, error) { return nil, ErrProxyDenied }

// Post returns ErrProxyDenied.
func (NoopProxy) Post(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, ErrProxyDenied
}

var _ OutboundProxy = NoopProxy{}

// WhitelistProxy enforces a static host suffix list on all outbound calls.
// A host "api.example.com" matches an entry "example.com" (and
// ".example.com"). The whitelist is case-insensitive.
type WhitelistProxy struct {
	allowed []string
	client  *http.Client
}

// NewWhitelistProxy constructs a proxy with the given allowed host suffixes.
// An empty slice means "no egress" — same behaviour as NoopProxy. The
// client argument may be nil and defaults to a 10s timeout http.Client.
func NewWhitelistProxy(allowedHosts []string, client *http.Client) *WhitelistProxy {
	// normalise entries: lowercase, strip leading dot (we re-add it in
	// the suffix check so we don't accidentally match "example.com"
	// against "notexample.com").
	norm := make([]string, 0, len(allowedHosts))
	for _, h := range allowedHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.TrimPrefix(h, ".")
		if h != "" {
			norm = append(norm, h)
		}
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	// 复制调用方 client，避免修改共享实例；逐跳复检重定向目标（BE-D-9）：
	// 白名单主机 302 到内网/非白名单主机时立即拒绝，而不是跟随到内网。
	clientCopy := *client
	p := &WhitelistProxy{allowed: norm, client: &clientCopy}
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		host := req.URL.Hostname()
		if host == "" || !p.Allowed(host) {
			return ErrProxyDenied
		}
		return nil
	}
	return p
}

// Allowed performs suffix-match on host. Subdomains of whitelisted entries
// pass: "api.example.com" ∈ ["example.com"] → true.
func (p *WhitelistProxy) Allowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, w := range p.allowed {
		if host == w {
			return true
		}
		// suffix match with boundary — avoids "example.com" matching
		// "notexample.com"
		if strings.HasSuffix(host, "."+w) {
			return true
		}
	}
	return false
}

func (p *WhitelistProxy) checkHost(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return apperrors.WrapInternal(err, "invalid url")
	}
	host := parsed.Hostname()
	if host == "" {
		return apperrors.New(apperrors.CodeInvalidArgument, "invalid url: missing host")
	}
	if !p.Allowed(host) {
		return ErrProxyDenied
	}
	return nil
}

// Get implements OutboundProxy.
func (p *WhitelistProxy) Get(ctx context.Context, rawURL string) ([]byte, error) {
	if err := p.checkHost(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	return p.do(req)
}

// Post implements OutboundProxy. Content-Length is auto-set from body.
func (p *WhitelistProxy) Post(ctx context.Context, rawURL string, body []byte) ([]byte, error) {
	if err := p.checkHost(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return p.do(req)
}

func (p *WhitelistProxy) do(req *http.Request) ([]byte, error) {
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxProxyResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxProxyResponseBytes {
		return nil, apperrors.NewUnavailablef("upstream response body exceeds %d bytes", maxProxyResponseBytes)
	}
	if resp.StatusCode >= 400 {
		return data, apperrors.Newf(apperrors.CodeUnavailable, "upstream status %d", resp.StatusCode)
	}
	return data, nil
}

var _ OutboundProxy = (*WhitelistProxy)(nil)
