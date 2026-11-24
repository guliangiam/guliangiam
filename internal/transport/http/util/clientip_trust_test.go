package util

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClientIP_UntrustedRemoteIgnoresForwardedHeaders(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "203.0.113.0/24")
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4"}, "X-Real-Ip": []string{"5.6.7.8"}},
		RemoteAddr: "198.51.100.10:1234",
	}
	assert.Equal(t, "198.51.100.10", ClientIP(r),
		"forwarded headers must be ignored unless the peer is a trusted proxy")
}

func TestClientIP_TrustedRemoteUsesForwardedHeaders(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "203.0.113.0/24")
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4"}},
		RemoteAddr: "203.0.113.5:1234",
	}
	assert.Equal(t, "1.2.3.4", ClientIP(r))
}

func TestClientIP_MalformedTrustedProxyConfigFailsClosed(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "not-a-cidr")
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4"}},
		RemoteAddr: "203.0.113.5:1234",
	}
	assert.Equal(t, "203.0.113.5", ClientIP(r))
}

func TestClientIP_DefaultTrustsNoProxy(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4"}},
		RemoteAddr: "203.0.113.5:1234",
	}
	assert.Equal(t, "203.0.113.5", ClientIP(r))
}

func TestClientIP_TrustedProxyStripsTrustedHopsFromRight(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "203.0.113.0/24")
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4, 198.51.100.8, 203.0.113.6"}},
		RemoteAddr: "203.0.113.5:1234",
	}
	assert.Equal(t, "198.51.100.8", ClientIP(r),
		"a client-supplied leftmost entry must not replace the rightmost untrusted hop")
}

func TestClientIP_TrustedProxySkipsMalformedForwardedEntry(t *testing.T) {
	// BE-E-16 裁决：坏条目跳过而非整体回退——伪造坏条目不得把所有客户端
	// 折叠进代理自身的限流桶。可信代理正常追加的链尾仍是最右可信剥离锚点。
	t.Setenv("TRUSTED_PROXY_CIDRS", "203.0.113.0/24")
	r := &http.Request{
		Header:     http.Header{"X-Forwarded-For": []string{"1.2.3.4, not-an-ip"}},
		RemoteAddr: "203.0.113.5:1234",
	}
	assert.Equal(t, "1.2.3.4", ClientIP(r))
}

func TestClientIP_TrustedProxyValidatesRealIP(t *testing.T) {
	t.Setenv("TRUSTED_PROXY_CIDRS", "203.0.113.0/24")
	r := &http.Request{
		Header:     http.Header{"X-Real-Ip": []string{"not-an-ip"}},
		RemoteAddr: "203.0.113.5:1234",
	}
	assert.Equal(t, "203.0.113.5", ClientIP(r))
}
