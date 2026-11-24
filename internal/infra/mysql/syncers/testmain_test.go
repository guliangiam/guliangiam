package syncers

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
