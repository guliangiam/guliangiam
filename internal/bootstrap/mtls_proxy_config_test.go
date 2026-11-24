package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateMTLSProxyConfig(t *testing.T) {
	tests := []struct {
		name    string
		header  string
		cidrs   string
		wantErr bool
	}{
		{"disabled", "", "", false},
		{"header without trusted proxies", "X-Forwarded-Client-Cert", "", true},
		{"header with trusted proxies", "X-Forwarded-Client-Cert", "10.0.0.0/8", false},
		{"invalid header name", "X Forwarded Client Cert", "10.0.0.0/8", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("MTLS_PROXY_CLIENT_CERT_HEADER", tt.header)
			t.Setenv("TRUSTED_PROXY_CIDRS", tt.cidrs)
			err := validateMTLSProxyConfig()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
