package webauthn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 来源规范化与全等比对。这里的边界值都来自真实 clientDataJSON：浏览器给的是
// scheme://host[:port]，而配置里常见的是带路径或带默认端口的写法。

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"plain https", "https://console.example.com", "https://console.example.com"},
		{"host is lowercased", "https://Console.Example.COM", "https://console.example.com"},
		{"default https port is dropped", "https://console.example.com:443", "https://console.example.com"},
		{"default http port is dropped", "http://localhost:80", "http://localhost"},
		{"non-default port is kept", "http://localhost:7101", "http://localhost:7101"},
		{"path and trailing slash are ignored", "https://console.example.com/login/", "https://console.example.com"},
		{"whitespace is trimmed", "  https://console.example.com  ", "https://console.example.com"},
		{"wildcard is refused", "https://*.example.com", ""},
		{"garbage is refused", "not a url", ""},
		{"empty is refused", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeOrigin(tt.raw))
		})
	}
}

func TestOriginOfURL(t *testing.T) {
	assert.Equal(t, "https://iam.example.com", OriginOfURL("https://iam.example.com/realms/acme"))
	assert.Equal(t, "http://localhost:7101", OriginOfURL("http://localhost:7101"))
	assert.Equal(t, "", OriginOfURL(""))
	assert.Equal(t, "", OriginOfURL("acme"))
}

func TestMatchOrigin(t *testing.T) {
	allowed := []string{"https://console.example.com", "http://localhost:7101"}

	assert.True(t, MatchOrigin("https://console.example.com", allowed))
	assert.True(t, MatchOrigin("https://console.example.com:443", allowed), "默认端口归一后应命中")
	assert.True(t, MatchOrigin("http://localhost:7101/login", allowed), "来源里的路径应被忽略")

	assert.False(t, MatchOrigin("https://evil.example.com", allowed))
	assert.False(t, MatchOrigin("https://console.example.com.evil.net", allowed), "后缀伪装不得命中")
	assert.False(t, MatchOrigin("http://console.example.com", allowed), "协议不同即不同来源")
	assert.False(t, MatchOrigin("https://console.example.com:8443", allowed), "非默认端口即不同来源")
	assert.False(t, MatchOrigin("", allowed))
	assert.False(t, MatchOrigin("https://console.example.com", nil), "空清单永不匹配")
	assert.False(t, MatchOrigin("https://a.example.com", []string{"https://*.example.com"}), "通配符条目不参与匹配")
}
