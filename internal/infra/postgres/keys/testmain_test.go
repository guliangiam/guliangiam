package keys

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// TestMain configures the dedicated test encryption key: keys.private_key is
// sealed with the same fail-closed envelope used in production.
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
