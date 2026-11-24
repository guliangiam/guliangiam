package bootstrap

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/casbin"
	mysqlcasbin "github.com/guliangiam/guliangiam/internal/infra/mysql/casbin"
)

// 组合根按 driver 选择 Casbin 策略存储：MySQL 走方言实现，其余走 pgx 适配器。
func TestNewCasbinPolicyStore_PicksDialectByDriver(t *testing.T) {
	store := newCasbinPolicyStore(nil, &sql.DB{}, "mysql")
	_, ok := store.(mysqlcasbin.Store)
	require.True(t, ok, "mysql driver 必须使用 MySQL 方言实现")

	store = newCasbinPolicyStore(nil, nil, "pgx")
	_, ok = store.(casbin.Adapter)
	assert.True(t, ok, "PostgreSQL 族必须使用 pgx 适配器")
}
