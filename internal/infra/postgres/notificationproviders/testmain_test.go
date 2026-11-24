package notificationproviders

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// TestMain configures the dedicated test encryption key: the access token is
// envelope-encrypted on write and fails closed on plaintext reads.
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
