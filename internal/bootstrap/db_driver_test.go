package bootstrap

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMaskMySQLDSN 验证 MySQL DSN 密码脱敏逻辑（纯字符串处理，无 DB 依赖）。
func TestMaskMySQLDSN(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		expect string
	}{
		{"标准 tcp 格式", "guliang:secret@tcp(127.0.0.1:3306)/iam", "guliang:****@tcp(127.0.0.1:3306)/iam"},
		{"unix socket 格式", "guliang:pass123@unix(/var/run/mysqld.sock)/iam", "guliang:****@unix(/var/run/mysqld.sock)/iam"},
		{"无密码", "guliang@tcp(localhost)/iam", "guliang@tcp(localhost)/iam"},
		{"完全无 @", "guliang", "guliang"},
		{"空串", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := maskMySQLDSN(tc.input)
			assert.Equal(t, tc.expect, out)
		})
	}
}

// TestDriverSelectionStrictFail 验证未知 DB_DRIVER 值严格失败（无静默降级）。
func TestDriverSelectionStrictFail(t *testing.T) {
	t.Setenv("DB_DRIVER", "oracle")

	_, err := NewRuntimeFromEnv(context.Background())
	assert.Error(t, err, "unknown DB_DRIVER should fail strictly")
	assert.Contains(t, err.Error(), "unknown DB_DRIVER")
}

// TestDriverMySQLMissingDSN 验证 DB_DRIVER=mysql 但无 MYSQL_DSN → 严格失败。
func TestDriverMySQLMissingDSN(t *testing.T) {
	t.Setenv("DB_DRIVER", "mysql")
	t.Setenv("MYSQL_DSN", "")
	t.Setenv("MYSQL_URL", "")

	_, err := NewRuntimeFromEnv(context.Background())
	assert.Error(t, err, "no MYSQL_DSN should fail strictly")
	assert.Contains(t, err.Error(), "MYSQL_DSN")
}

// TestDriverDefaultPgxDBNil 验证 DB_DRIVER 为空时默认 pgx（严格要求 PG DSN）。
func TestDriverDefaultPgxDBNil(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("DATABASE_URL", "")

	_, err := NewRuntimeFromEnv(context.Background())
	assert.Error(t, err, "no DSN should fail strictly in default pgx mode")
}

// 确保 os 被引用（go vet 警告）
var _ = os.Getenv
