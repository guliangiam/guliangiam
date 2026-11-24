package webhooks

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// TestMain 配置固定测试加密 key（DB-058 禁止回退到明文存储）。
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
