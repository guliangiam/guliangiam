package idp

import (
	"net/http"
	"time"

	"github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// providerHTTPClient builds the outbound client for a provider row. Nil keeps
// the SSRF-guarded package default, so only opted-in providers pay the extra
// client/allocation cost.
func providerHTTPClient(p *providers.Provider) *http.Client {
	if p == nil || (!p.EnableProxy && len(p.HttpHeaders) == 0) {
		return nil
	}
	base := ssrf.DefaultHTTPClient
	if p.EnableProxy {
		base = ssrf.SafeProxyHTTPClient(30 * time.Second)
	}
	if len(p.HttpHeaders) == 0 {
		return base
	}
	clone := *base
	transport := clone.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clone.Transport = headerInjector{base: transport, headers: p.HttpHeaders}
	return &clone
}

// headerInjector adds the provider's configured HTTP headers to every outbound
// request without mutating the caller's request object.
type headerInjector struct {
	base    http.RoundTripper
	headers map[string]string
}

func (h headerInjector) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for key, value := range h.headers {
		clone.Header.Set(key, value)
	}
	return h.base.RoundTrip(clone)
}
