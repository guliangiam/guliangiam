package connectors

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// Configure the public test KEK before any envelope encryption happens.
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
