package mcpservers

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// TestMain 配置固定测试加密 key：token 的读写都走信封加密。
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
