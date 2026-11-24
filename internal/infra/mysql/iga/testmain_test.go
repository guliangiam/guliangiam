package iga

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/crypto/cryptotest"
)

// TestMain 配置固定测试加密 key。
func TestMain(m *testing.M) {
	cryptotest.Configure()
	os.Exit(m.Run())
}
