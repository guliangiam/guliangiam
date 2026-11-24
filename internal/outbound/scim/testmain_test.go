package scim

import (
	"os"
	"testing"
)

// The handler/listing tests use httptest loopback servers; opt in explicitly
// (production default keeps the SSRF-safe transport, see client_ssrf_test.go).
func TestMain(m *testing.M) {
	os.Setenv("SCIM_ALLOW_PRIVATE_TARGETS", "true")
	os.Exit(m.Run())
}
