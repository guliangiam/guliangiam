package bootstrap

import (
	"os"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/config"
)

// unsetEnv 把变量置为真正的「未设置」（t.Setenv 会留一个空值，语义不同）。
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if previous, ok := os.LookupEnv(key); ok {
		t.Cleanup(func() { _ = os.Setenv(key, previous) })
	} else {
		t.Cleanup(func() { _ = os.Unsetenv(key) })
	}
	_ = os.Unsetenv(key)
}

// TestMigrationGatesDefaultOff 验证所有环境默认关闭 migration+seed：
// 裸启动不得重放数据库，只有命令行 -migrate/-seed 或显式环境变量才开启。
func TestMigrationGatesDefaultOff(t *testing.T) {
	unsetEnv(t, "AUTO_MIGRATE")
	unsetEnv(t, "AUTO_SEED")

	for _, appEnv := range []string{"development", "test", "staging", "production", ""} {
		t.Setenv("APP_ENV", appEnv)
		autoMigrate, autoSeed := migrationGates()
		if autoMigrate {
			t.Errorf("APP_ENV=%q should default auto_migrate=false; got true", appEnv)
		}
		if autoSeed {
			t.Errorf("APP_ENV=%q should default auto_seed=false; got true", appEnv)
		}
	}
}

// TestMigrationGatesExplicitOverridesDefault 验证显式 AUTO_MIGRATE / AUTO_SEED
// 总是覆盖默认关闭。
func TestMigrationGatesExplicitOverridesDefault(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	t.Setenv("AUTO_MIGRATE", "false")
	t.Setenv("AUTO_SEED", "0")
	autoMigrate, autoSeed := migrationGates()
	if autoMigrate || autoSeed {
		t.Fatalf("explicit false must stay off: got auto_migrate=%v auto_seed=%v", autoMigrate, autoSeed)
	}

	t.Setenv("APP_ENV", "production")
	t.Setenv("AUTO_MIGRATE", "true")
	t.Setenv("AUTO_SEED", "yes")
	autoMigrate, autoSeed = migrationGates()
	if !autoMigrate || !autoSeed {
		t.Fatalf("explicit true must override production default: got auto_migrate=%v auto_seed=%v", autoMigrate, autoSeed)
	}

	// 解析 gate 不应写入环境变量（进程可区分显式 vs 默认选择）
	t.Setenv("APP_ENV", "development")
	unsetEnv(t, "AUTO_MIGRATE")
	unsetEnv(t, "AUTO_SEED")
	_, _ = migrationGates()
	if _, set := os.LookupEnv("AUTO_MIGRATE"); set {
		t.Fatal("resolving gates must not write AUTO_MIGRATE")
	}
	if _, set := os.LookupEnv("AUTO_SEED"); set {
		t.Fatal("resolving gates must not write AUTO_SEED")
	}
}

// TestConfigAndBootstrapGatesAgree 确保 config 层和 bootstrap 层对同一环境
// 变量的解读完全一致。
func TestConfigAndBootstrapGatesAgree(t *testing.T) {
	for _, tc := range []struct {
		appEnv      string
		autoMigrate string
		autoSeed    string
	}{
		{"development", "", ""},
		{"production", "", ""},
		{"development", "true", "1"},
		{"production", "false", "no"},
		{"staging", "on", "yes"},
	} {
		t.Setenv("APP_ENV", tc.appEnv)
		if tc.autoMigrate != "" {
			t.Setenv("AUTO_MIGRATE", tc.autoMigrate)
		} else {
			unsetEnv(t, "AUTO_MIGRATE")
		}
		if tc.autoSeed != "" {
			t.Setenv("AUTO_SEED", tc.autoSeed)
		} else {
			unsetEnv(t, "AUTO_SEED")
		}

		bm, bs := migrationGates()
		cm, cs := config.AutoMigrate(), config.AutoSeed()
		if bm != cm {
			t.Errorf("APP_ENV=%q AUTO_MIGRATE=%q: bootstrap autoMigrate=%v != config=%v",
				tc.appEnv, tc.autoMigrate, bm, cm)
		}
		if bs != cs {
			t.Errorf("APP_ENV=%q AUTO_SEED=%q: bootstrap autoSeed=%v != config=%v",
				tc.appEnv, tc.autoSeed, bs, cs)
		}
	}
}
