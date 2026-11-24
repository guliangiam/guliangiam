package providers

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// TestMain configures the dedicated test encryption key: provider credentials
// are envelope-encrypted on write and fail closed on plaintext reads.
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
