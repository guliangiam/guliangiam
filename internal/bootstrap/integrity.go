package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/platform/db"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BootstrapIntegrityReport 汇总完整性校验结果，供启动日志输出和后续消费。
//
// Failures 是逐项的人类可读结论，每条都带「检查项 + 期望值 + 实际值」，
// 既用于启动日志，也直接进入 fail-fast 的返回错误——拒绝启动时运维必须
// 能一眼看出是哪一项基线没自证通过。
type BootstrapIntegrityReport struct {
	StructureApplied     int64    `json:"structure_applied"`
	DataApplied          int64    `json:"data_applied"`
	MissingTables        []string `json:"missing_tables,omitempty"`
	EmptySeedTables      []string `json:"empty_seed_tables,omitempty"`
	AdminCredentialOK    bool     `json:"admin_credential_ok"`
	AdminCredentialError string   `json:"admin_credential_error,omitempty"`
	Failures             []string `json:"failures,omitempty"`
	ChecksPassed         bool     `json:"checks_passed"`
}

// fail 记录一项不通过并把整体结论置为失败。任何一项失败都不允许被后续
// 通过项覆盖（ChecksPassed 一旦为 false 就保持 false）。
func (r *BootstrapIntegrityReport) fail(format string, args ...any) {
	r.ChecksPassed = false
	r.Failures = append(r.Failures, fmt.Sprintf(format, args...))
}

// VerifyBootstrapIntegrity 在 migrate+seed 完成后自动校验启动基线完整性。
//
// 所有 RLS-bypass 的查询在**同一个事务**内执行——pgx pool 每次
// Exec/QueryRow 可能从池里拿到不同连接，而 set_config 是会话级别的，
// 必须在同一连接上先 bypass 再查询才能读到种子数据。
//
// 校验项：
//  1. 账本连续：结构链 1..N 连续 + 数据链 1..M 连续；缺版本视为不完整
//  2. 种子表非空：V1 基线种子覆盖的核心表（含 captcha/notification_providers）应有数据
//  3. admin credential：存在且 password_hash 非空
//
// 本函数只负责给出结论，不做任何修复/重建：任一检查项不通过或校验本身
// 无法执行（事务打不开、RLS bypass 失败、查询报错）都写入 Failures 并
// 置 ChecksPassed=false。是否因此拒绝启动由调用方 enforceBootstrapIntegrity
// 按 BOOTSTRAP_INTEGRITY_STRICT 裁决（默认严格）。
func VerifyBootstrapIntegrity(ctx context.Context, pool *pgxpool.Pool) (*BootstrapIntegrityReport, error) {
	report := &BootstrapIntegrityReport{ChecksPassed: true}

	// --- 1. 账本连续性 ---
	applied, err := db.Applied(ctx, pool)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "read migration ledger for integrity check")
	}

	var structVers, dataVers []int64
	for _, a := range applied {
		if a.Kind == db.KindStructure {
			structVers = append(structVers, a.Version)
		} else {
			dataVers = append(dataVers, a.Version)
		}
	}
	report.StructureApplied = int64(len(structVers))
	report.DataApplied = int64(len(dataVers))

	// 期望的最新版本从 migrations 声明推导：任何 squash 或新增迁移都会自动
	// 同步，避免这里的硬编码版本号与实际镜像漂移（squash 后曾残留 V14/V7）。
	expected := map[db.MigrationKind]int64{}
	for _, migration := range migrations {
		if migration.Version > expected[migration.Kind] {
			expected[migration.Kind] = migration.Version
		}
	}
	expectedStructureMax := expected[db.KindStructure]
	expectedDataMax := expected[db.KindData]

	var missingStructure, missingData []int64
	for v := int64(1); v <= expectedStructureMax; v++ {
		if !contains(structVers, v) {
			missingStructure = append(missingStructure, v)
		}
	}
	if len(missingStructure) > 0 {
		report.fail("structure_ledger: expected versions 1..%d applied, got %d applied and missing %v",
			expectedStructureMax, len(structVers), missingStructure)
		slog.Warn("integrity: structure ledger gap detected",
			"check", "structure_ledger",
			"missing_versions", missingStructure,
			"applied_structure", len(structVers),
			"expected_max", expectedStructureMax)
	}
	for v := int64(1); v <= expectedDataMax; v++ {
		if !contains(dataVers, v) {
			missingData = append(missingData, v)
		}
	}
	if len(missingData) > 0 {
		report.fail("data_ledger: expected versions 1..%d applied, got %d applied and missing %v",
			expectedDataMax, len(dataVers), missingData)
		slog.Warn("integrity: data ledger gap detected",
			"check", "data_ledger",
			"missing_versions", missingData,
			"applied_data", len(dataVers),
			"expected_max", expectedDataMax)
	}

	// --- 2+3. 种子表非空 + admin credential（同一个事务里 RLS bypass） ---
	tx, err := pool.Begin(ctx)
	if err != nil {
		// 校验无法执行 = 基线无法自证，同样按不通过处理（而不是静默放过）。
		report.AdminCredentialError = err.Error()
		report.fail("seed_tables: expected a verification transaction, got begin error: %v", err)
		slog.Warn("integrity: failed to begin verification transaction", "check", "seed_tables", "error", err)
		return report, nil
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		SELECT set_config('app.current_organization_id', '', true);
		SELECT set_config('app.current_role', 'super_admin', true);
	`); err != nil {
		report.AdminCredentialError = err.Error()
		report.fail("seed_tables: expected RLS bypass to succeed, got error: %v", err)
		slog.Warn("integrity: RLS bypass failed", "check", "seed_tables", "error", err)
		return report, nil
	}

	// V1 种子覆盖的核心表（不含 adapters/encryption_keys，它们非种子范围）
	coreSeedTables := []struct {
		name    string
		minRows int64
	}{
		{"organizations", 1},
		{"users", 1},
		{"applications", 1},
		{"roles", 1},
		{"casbin_rules", 1},
		{"enforcers", 1},
		{"models", 1},
		{"providers", 1},
		{"captcha_providers", 1},
		{"notification_providers", 1},
	}

	for _, t := range coreSeedTables {
		var count int64
		if err := tx.QueryRow(ctx,
			"SELECT COUNT(*) FROM guliangiam."+t.name,
		).Scan(&count); err != nil {
			// 表不存在（或不可读）→ Schema 未完整应用
			report.MissingTables = append(report.MissingTables, t.name)
			report.fail("seed_table %s: expected table present, got query error: %v", t.name, err)
			slog.Warn("integrity: table missing", "check", "seed_table", "table", t.name, "error", err)
			continue
		}
		if count < t.minRows {
			report.EmptySeedTables = append(report.EmptySeedTables,
				fmt.Sprintf("%s (got %d, want ≥%d)", t.name, count, t.minRows))
			report.fail("seed_table %s: expected at least %d row(s), got %d", t.name, t.minRows, count)
			slog.Warn("integrity: seed table too empty",
				"check", "seed_table", "table", t.name, "count", count, "min_rows", t.minRows)
		}
	}

	// Admin credential
	var hash *string
	err = tx.QueryRow(ctx, `
		SELECT c.password_hash
		FROM guliangiam.user_credentials c
		JOIN guliangiam.users u ON u.id = c.user_id
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		WHERE u.username = 'admin' AND o.slug = 'built-in'
		  AND u.deleted_at IS NULL AND o.deleted_at IS NULL
	`).Scan(&hash)

	switch {
	case err != nil:
		if err == pgx.ErrNoRows {
			report.AdminCredentialError = "admin credential row not found"
		} else {
			report.AdminCredentialError = err.Error()
		}
		report.fail("admin_credential: expected password_hash for admin@built-in, got %s", report.AdminCredentialError)
		slog.Warn("integrity: admin credential lookup failed",
			"check", "admin_credential", "expected", "password_hash for admin@built-in", "error", err)
	case hash == nil || *hash == "":
		report.AdminCredentialError = "password_hash is empty"
		report.fail("admin_credential: expected non-empty password_hash for admin@built-in, got empty")
		slog.Warn("integrity: admin credential password_hash is empty",
			"check", "admin_credential", "expected", "non-empty password_hash for admin@built-in", "got", "empty",
			"hint", "set BOOTSTRAP_ADMIN_PASSWORD and restart, or run: AUTO_SEED=true go run ./cmd/migrate up --seed")
	default:
		report.AdminCredentialOK = true
	}

	return report, nil
}

func contains(vs []int64, v int64) bool {
	for _, x := range vs {
		if x == v {
			return true
		}
	}
	return false
}

// LogIntegrityReport 在启动日志中汇总完整性校验结果。
// 不通过时逐项打出 Failures（检查项 + 期望值 + 实际值）与修复提示；
// 是否继续启动由 enforceBootstrapIntegrity 裁决，本函数只负责记录事实。
func LogIntegrityReport(report *BootstrapIntegrityReport) {
	if report == nil {
		return
	}
	if report.ChecksPassed {
		slog.Info("bootstrap integrity check PASSED",
			"structure_versions", report.StructureApplied,
			"data_versions", report.DataApplied,
			"admin_credential", report.AdminCredentialOK,
		)
		return
	}

	fields := []any{
		"structure_versions", report.StructureApplied,
		"data_versions", report.DataApplied,
		"admin_credential", report.AdminCredentialOK,
		"failures", report.Failures,
	}
	if len(report.MissingTables) > 0 {
		fields = append(fields, "missing_tables", report.MissingTables)
	}
	if len(report.EmptySeedTables) > 0 {
		fields = append(fields, "empty_seed_tables", report.EmptySeedTables)
	}
	if report.AdminCredentialError != "" {
		fields = append(fields, "admin_credential_error", report.AdminCredentialError)
	}

	slog.Warn("bootstrap integrity check had gaps — DB may be incomplete", fields...)

	// 生产环境：给出明确修复提示
	if config.IsProduction() {
		slog.Warn("remediation: verify deploy pipeline ran `migrate up --seed` before starting server")
	} else {
		slog.Warn("remediation: run `go run ./cmd/migrate up --seed` to (re)apply structure + seed chain")
	}
}
