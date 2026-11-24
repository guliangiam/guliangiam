package gateway

import "testing"

func TestTlsSkipVerifyOptIn(t *testing.T) {
	// 默认（无 Metadata / 未声明 / 非 true）必须严格校验证书
	for name, metadata := range map[string]map[string]any{
		"nil metadata":   nil,
		"empty metadata": {},
		"explicit false": {"insecureSkipTlsVerify": false},
		"wrong type":     {"insecureSkipTlsVerify": "true"},
	} {
		if tlsSkipVerifyOptIn(metadata) {
			t.Errorf("%s: TLS verification must be enabled by default", name)
		}
	}

	// 仅显式 true 才允许跳过（自签内网端点场景）
	if !tlsSkipVerifyOptIn(map[string]any{"insecureSkipTlsVerify": true}) {
		t.Error("explicit insecureSkipTlsVerify=true must opt in")
	}
}
