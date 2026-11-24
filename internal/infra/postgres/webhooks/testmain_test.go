package webhooks

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// TestMain configures the dedicated test encryption key: DB-058 forbids
// falling back to plaintext storage for confidential columns.
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
