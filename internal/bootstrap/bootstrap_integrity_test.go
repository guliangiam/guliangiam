package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureBootstrapLogs 把默认 slog 重定向到内存缓冲，用于断言启动裁决日志。
// 这些用例不调用 t.Parallel：slog.SetDefault 是进程级状态。
func captureBootstrapLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return buf
}

// stubIntegrityVerifier 替换完整性校验入口并注册恢复。
func stubIntegrityVerifier(t *testing.T, verify func(context.Context, *pgxpool.Pool) (*BootstrapIntegrityReport, error)) {
	t.Helper()
	prev := integrityVerifier
	integrityVerifier = verify
	t.Cleanup(func() { integrityVerifier = prev })
}

// SEC-BOOT-INTEGRITY：默认严格 —— 校验不通过必须拒绝启动，而不是 WARN 后继续。
func TestEnforceBootstrapIntegrity_StrictFailsOnGaps(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BOOTSTRAP_INTEGRITY_STRICT", "true")
	logs := captureBootstrapLogs(t)

	stubIntegrityVerifier(t, func(context.Context, *pgxpool.Pool) (*BootstrapIntegrityReport, error) {
		return &BootstrapIntegrityReport{
			StructureApplied: 14,
			DataApplied:      1,
			ChecksPassed:     false,
			Failures: []string{
				"data_ledger: expected versions 1..2 applied, got 1 applied and missing [2]",
				"seed_table users: expected at least 1 row(s), got 0",
				"admin_credential: expected password_hash for admin@built-in, got admin credential row not found",
			},
		}, nil
	})

	err := enforceBootstrapIntegrity(context.Background(), nil)
	require.Error(t, err, "strict mode must fail startup when the integrity check reports gaps")
	assert.Contains(t, err.Error(), "bootstrap integrity check failed")
	assert.Contains(t, err.Error(), "data_ledger")
	assert.Contains(t, err.Error(), "seed_table users")
	assert.Contains(t, err.Error(), "admin_credential")

	out := logs.String()
	assert.Contains(t, out, "bootstrap integrity check failed; refusing to start")
	assert.Contains(t, out, "data_ledger: expected versions 1..2 applied")
	assert.Contains(t, out, `"level":"ERROR"`)
}

// 校验本身无法执行（读账本/事务失败）同样不能让服务带着未自证的基线继续。
func TestEnforceBootstrapIntegrity_VerificationErrorFails(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BOOTSTRAP_INTEGRITY_STRICT", "true")
	logs := captureBootstrapLogs(t)

	stubIntegrityVerifier(t, func(context.Context, *pgxpool.Pool) (*BootstrapIntegrityReport, error) {
		return nil, errors.New("read migration ledger: connection refused")
	})

	err := enforceBootstrapIntegrity(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bootstrap integrity verification could not run")
	assert.Contains(t, logs.String(), "connection refused")
}

// 显式开关关闭（仅非生产）：不执行校验、启动继续，并打 WARN 声明被显式跳过。
func TestEnforceBootstrapIntegrity_ExplicitSkipContinuesWithWarn(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BOOTSTRAP_INTEGRITY_STRICT", "false")
	logs := captureBootstrapLogs(t)

	called := false
	stubIntegrityVerifier(t, func(context.Context, *pgxpool.Pool) (*BootstrapIntegrityReport, error) {
		called = true
		return &BootstrapIntegrityReport{ChecksPassed: false, Failures: []string{"should not run"}}, nil
	})

	require.NoError(t, enforceBootstrapIntegrity(context.Background(), nil))
	assert.False(t, called, "the verifier must not run when the switch is explicitly off")

	out := logs.String()
	assert.Contains(t, out, "bootstrap integrity check explicitly skipped")
	assert.Contains(t, out, `"switch":"BOOTSTRAP_INTEGRITY_STRICT=false"`)
	assert.Contains(t, out, `"level":"WARN"`)
}

// 生产环境禁止关闭：开关本身是配置错误，必须在监听器创建前失败。
func TestEnforceBootstrapIntegrity_ProductionRejectsSkip(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("BOOTSTRAP_INTEGRITY_STRICT", "false")
	logs := captureBootstrapLogs(t)

	stubIntegrityVerifier(t, func(context.Context, *pgxpool.Pool) (*BootstrapIntegrityReport, error) {
		t.Fatal("verifier must not run when the switch is invalid")
		return nil, nil
	})

	err := enforceBootstrapIntegrity(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BOOTSTRAP_INTEGRITY_STRICT=false is forbidden when APP_ENV=production")
	assert.Contains(t, logs.String(), "bootstrap integrity switch is invalid; refusing to start")
}

// 拼错的开关不能静默退化成默认值：值无法解析即配置错误。
func TestEnforceBootstrapIntegrity_InvalidSwitchValueFails(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BOOTSTRAP_INTEGRITY_STRICT", "strict")
	captureBootstrapLogs(t)

	stubIntegrityVerifier(t, func(context.Context, *pgxpool.Pool) (*BootstrapIntegrityReport, error) {
		t.Fatal("verifier must not run when the switch is unparseable")
		return nil, nil
	})

	err := enforceBootstrapIntegrity(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `BOOTSTRAP_INTEGRITY_STRICT must be true or false, got "strict"`)
}

// 空串按未设置处理：默认严格不能因为「变量存在但为空」而放行。
func TestEnforceBootstrapIntegrity_EmptySwitchDefaultsToStrict(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("BOOTSTRAP_INTEGRITY_STRICT", "")
	captureBootstrapLogs(t)

	stubIntegrityVerifier(t, func(context.Context, *pgxpool.Pool) (*BootstrapIntegrityReport, error) {
		return &BootstrapIntegrityReport{ChecksPassed: false, Failures: []string{"ledger gap"}}, nil
	})

	require.Error(t, enforceBootstrapIntegrity(context.Background(), nil))
}
