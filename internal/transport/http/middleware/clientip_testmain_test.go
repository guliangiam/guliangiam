package middleware

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("TRUSTED_PROXY_CIDRS", "10.0.0.0/8,192.168.0.0/16")
	os.Exit(m.Run())
}
