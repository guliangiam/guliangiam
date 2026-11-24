package util

import (
	"os"
	"testing"
)

// The client-IP tests exercise reverse-proxy forwarding; make their simulated
// proxy addresses explicitly trusted (the production default trusts none).
func TestMain(m *testing.M) {
	os.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.168.0.0/16,127.0.0.0/8")
	os.Exit(m.Run())
}
