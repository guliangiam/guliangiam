package bootstrap

import (
	"context"
	_ "embed"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/guliangiam/guliangiam/cred"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	"github.com/guliangiam/guliangiam/internal/platform/db"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema_sql/V1__guliangiam_schema_init.sql
var schemaDDL string

//go:embed schema_sql/V2__pay_recon_items_pay_recon_items.sql
var schemaDDLV2 string

//go:embed schema_sql/V3__app_saml_config_custom_attribute_definitions.sql
var schemaDDLV3 string

//go:embed schema_sql/V4__dsar_requests.sql
var schemaDDLV4 string

//go:embed schema_sql/V5__lockout_attempts.sql
var schemaDDLV5 string

//go:embed schema_sql/V6__user_credentials_user_profiles.sql
var schemaDDLV6 string

//go:embed schema_sql/V7__ck_providers_kind.sql
var schemaDDLV7 string

//go:embed schema_sql/V8__ck_providers_kind.sql
var schemaDDLV8 string

//go:embed schema_sql/V9__ck_providers_kind.sql
var schemaDDLV9 string

//go:embed schema_sql/V10__ck_providers_kind.sql
var schemaDDLV10 string

//go:embed schema_sql/V11__ck_providers_kind.sql
var schemaDDLV11 string

//go:embed schema_sql/V12__ck_providers_kind.sql
var schemaDDLV12 string

//go:embed schema_sql/V13__ck_providers_kind.sql
var schemaDDLV13 string

//go:embed schema_sql/V14__ck_providers_kind.sql
var schemaDDLV14 string

//go:embed schema_sql/V15__idempotency_keys.sql
var schemaDDLV15 string

//go:embed schema_sql/V16__identity_verifications_identity_verifications.sql
var schemaDDLV16 string

//go:embed schema_sql/V17__app_form_config.sql
var schemaDDLV17 string

//go:embed schema_sql/V18__app_form_config.sql
var schemaDDLV18 string

//go:embed schema_sql/V19__user_profiles.sql
var schemaDDLV19 string

//go:embed schema_sql/V20__ck_pay_attempts_channel_ck_pay_channel_configs_cha.sql
var schemaDDLV20 string

//go:embed schema_sql/V21__notifications_verifications.sql
var schemaDDLV21 string

//go:embed seed_sql/V1__guliangiam_seed_init.sql
var seedDML string

//go:embed seed_sql/V2__admin_profile_enrich.sql
var dataEnrichV2 string

// SeedBootstrapAdminHash 是种子内置的开发引导管理员 Argon2id 哈希。
//
// 明文只记录在 AGENTS.md / 部署运维文档的「开发引导口令」小节，禁止写入日志。
// 生产环境检测到该哨兵时 EnsureBootstrapAdminCredential 会拒绝播种/迁移，除非
// 通过 BOOTSTRAP_ADMIN_PASSWORD 显式替换。种子 SQL 中的字面量必须与本常量逐字一致
// （回归用例 TestSeedBootstrapCredential...）。
const SeedBootstrapAdminHash = "$argon2id$v=19$m=65536,t=1,p=8$tDFXTrRgMV3QvcVxL92Eig$kPprEni7O5SEvxJhFrLbwcoPjb/gq3/91hf0DGo0d9Y"

var legacySeedBootstrapAdminHashes = []string{
	"$argon2id$v=19$m=65536,t=1,p=8$f3KzhpjoIJbdOrQBJaMK3g$I+ZHFaPrprYHVeec437F+rbvKbszsmLYeQQpznrBpg4",
	"$argon2id$v=19$m=65536,t=1,p=8$MbrPfzJ9H6AQbFkeFn8G1Q$29qnfvGTjLyQ/oHlS+2z4Xe8ti8C3obGdh5VI9o8WtY",
}

// migrations declares the embedded migration set in the order the schema was
// versioned. Structure lives in `internal/bootstrap/schema_sql/` and seed in
// `internal/bootstrap/seed_sql/` (SHA-256 checked by
// `scripts/check-migrations.sh`). `db/seeds/` is the source of truth mirrored
// into `internal/bootstrap/seed_sql/` for go:embed; edit db/seeds/* and run
// `scripts/sync-seed-mirror.sh` to keep both copies byte-identical.
//
// 结构迁移链（version 自 1 起连续）:
//
//	V1: guliangiam_schema_init — 声明式 Schema 的冻结全量基线（= history/active/V1）。
//	    `contracts/schema/*.yaml` 是唯一结构源，V1 SQL 由 `gen_schema.py` 从活动快照渲染，
//	    不存在手写冻结文件；db/postgresql 与 go:embed 副本逐字节一致。
//
// 2026-11-04 开发期迁移折叠：此前 V2–V14 的增量（provider/支付闭集扩展、密文列扩宽、
// 组织策略 password_derive_algorithm、B2B 域名续验预算、connectors、凭据 *_sealed、
// tenant_identities / tenant_instances / outbox_dead_letters.organization_id 与补 RLS）
// 全部并入单一 V1 基线（145 表）。这些增量均为纯 DDL、无数据迁移语义，因此不保留 V2。
// 2026-11-09 同理折叠 app_saml_config 列宽扩展（saml_metadata → TEXT、
// saml_metadata_url → VARCHAR(2048)），不单独新增 V2 增量。
// 后续结构变更仍按 ADR-022 走「改声明 → 生成 V{N}」流程：
// `python3 scripts/gen_schema.py --dialect postgresql --version N`。
// 生成 V{N} 后必须在下方 migrations 列表登记对应 go:embed 变量，check-migrations 会校验。
// V5 增加 lockout_attempts 的 15 分钟观察窗起点与重复锁定退避级别。
// V7 ck_providers_kind 将 providers.kind 闭集扩展 tencent_idv（腾讯云实人认证适配器）。
//
// 数据迁移链（version 自 1 起独立于结构链，kind=data 内按 version 升序；
// 所有 structure 先跑完，再跑全部 data，由 sort.SliceStable 按 kind 优先度保证）:
//
//	V1: guliangiam_seed_init — 基线种子：内置组织/应用、管理员与 platform admin、
//	    Casbin 模型/Enforcer/内置角色策略、内置与目录 provider、app 配置子表、
//	    验证码/通知明细、组织安全策略与 settings、历史枚举归一化。
//
//	V2: admin_profile_enrich — 内置管理员 profile 字段补全（COALESCE 幂等），
//	    让前端首次登录时所有 Tab/表单都有真实演示数据。依赖 data V1 已注入 admin
//	    用户行，必须在所有 structure（含 V6 user_profiles 新列）应用之后执行。
//
// 2026-10-29 开发期 squash：原 V1–V8 合并为单一 V1 种子，幂等规则不变
// （全部 INSERT 使用无仲裁键的 ON CONFLICT DO NOTHING）。
var migrations = []db.Migration{
	{Kind: db.KindStructure, Version: 1, Name: "guliangiam_schema_init", SQL: schemaDDL},
	{Kind: db.KindStructure, Version: 2, Name: "pay_recon_items_pay_recon_items", SQL: schemaDDLV2},
	{Kind: db.KindStructure, Version: 3, Name: "app_saml_config_custom_attribute_definitions", SQL: schemaDDLV3},
	{Kind: db.KindStructure, Version: 4, Name: "dsar_requests", SQL: schemaDDLV4},
	{Kind: db.KindStructure, Version: 5, Name: "lockout_attempts", SQL: schemaDDLV5},
	{Kind: db.KindStructure, Version: 6, Name: "user_credentials_user_profiles", SQL: schemaDDLV6},
	{Kind: db.KindStructure, Version: 7, Name: "ck_providers_kind", SQL: schemaDDLV7},
	{Kind: db.KindStructure, Version: 8, Name: "ck_providers_kind", SQL: schemaDDLV8},
	{Kind: db.KindStructure, Version: 9, Name: "ck_providers_kind", SQL: schemaDDLV9},
	{Kind: db.KindStructure, Version: 10, Name: "ck_providers_kind", SQL: schemaDDLV10},
	{Kind: db.KindStructure, Version: 11, Name: "ck_providers_kind", SQL: schemaDDLV11},
	{Kind: db.KindStructure, Version: 12, Name: "ck_providers_kind", SQL: schemaDDLV12},
	{Kind: db.KindStructure, Version: 13, Name: "ck_providers_kind", SQL: schemaDDLV13},
	{Kind: db.KindStructure, Version: 14, Name: "ck_providers_kind", SQL: schemaDDLV14},
	{Kind: db.KindStructure, Version: 15, Name: "idempotency_keys", SQL: schemaDDLV15},
	{Kind: db.KindStructure, Version: 16, Name: "identity_verifications_identity_verifications", SQL: schemaDDLV16},
	{Kind: db.KindStructure, Version: 17, Name: "app_form_config", SQL: schemaDDLV17},
	{Kind: db.KindStructure, Version: 18, Name: "app_form_config", SQL: schemaDDLV18},
	{Kind: db.KindStructure, Version: 19, Name: "user_profiles", SQL: schemaDDLV19},
	{Kind: db.KindStructure, Version: 20, Name: "ck_pay_attempts_channel_ck_pay_channel_configs_cha", SQL: schemaDDLV20},
	{Kind: db.KindStructure, Version: 21, Name: "notifications_verifications", SQL: schemaDDLV21},
	{Kind: db.KindData, Version: 1, Name: "guliangiam_seed_init", SQL: seedDML},
	{Kind: db.KindData, Version: 2, Name: "admin_profile_enrich", SQL: dataEnrichV2},
}

// Migrations returns the embedded migration set. It is the single declaration
// point for both the server path (migrateAndSeed) and the operator path
// (cmd/migrate), so the two can never disagree about what the image contains.
func Migrations() []db.Migration {
	out := make([]db.Migration, len(migrations))
	copy(out, migrations)
	return out
}

// migrationGates returns the two opt-in switches.
//
// It deliberately does not write the environment. The former "non-production
// defaults to on" behaviour called os.Setenv, which made the process
// indistinguishable from an operator who had chosen auto-migration - and on a
// database whose seed chain had drifted it turned every local boot into an
// aborted replay instead of a served request.
func migrationGates() (autoMigrate, autoSeed bool) {
	return config.AutoMigrate(), config.AutoSeed()
}

// appliedCount 返回账本行数；账本尚不存在时视为 0（首次播种的正常状态）。
func appliedCount(ctx context.Context, pool *pgxpool.Pool) int {
	rows, err := db.Applied(ctx, pool)
	if err != nil {
		return 0
	}
	return len(rows)
}

func migrateAndSeed(ctx context.Context, pool *pgxpool.Pool) error {
	// Gate 语义：命令行 -migrate/-seed 与环境变量在 RunServer 中已裁决并写回；
	// 未显式开启时两个开关都为 false，启动只校验结构存在，不重放迁移、不导入
	// 种子。生产同样 opt-in，由 deploy pipeline 显式设置。
	autoMigrate, autoSeed := migrationGates()
	if !autoMigrate && !autoSeed {
		slog.Info("database bootstrap skipped",
			"reason", "no -migrate/-seed flag and AUTO_MIGRATE/AUTO_SEED not enabled",
			"schema_check", "presence only",
		)
	} else {
		slog.Info("database bootstrap start", "migrate", autoMigrate, "seed", autoSeed)
	}

	started := time.Now()
	ledgerBefore := appliedCount(ctx, pool)
	if err := db.Migrate(ctx, pool, migrations); err != nil {
		return err
	}
	if err := EnsureBootstrapAdminCredential(ctx, pool); err != nil {
		return err
	}
	if autoMigrate || autoSeed {
		appliedNow := appliedCount(ctx, pool) - ledgerBefore
		if appliedNow < 0 {
			appliedNow = 0
		}
		slog.Info("database bootstrap finished",
			"migrate", autoMigrate, "seed", autoSeed,
			"applied_now", appliedNow,
			"elapsed_ms", time.Since(started).Milliseconds(),
		)
	}

	// 完整性校验：账本连续性 + 种子表非空 + admin credential 完整性。
	// SEC-BOOT-INTEGRITY：默认严格，任何检查项不通过都拒绝启动——服务不得
	// 带着未自证的基线对外提供服务（AGENTS.md §4.3）。唯一的继续入口是显式
	// BOOTSTRAP_INTEGRITY_STRICT=false（仅非生产；生产设置该值为 false 直接
	// 失败）。校验不修复、不重建任何东西。
	if err := enforceBootstrapIntegrity(ctx, pool); err != nil {
		return err
	}

	return nil
}

// integrityVerifier 是完整性校验的可替换入口；生产恒为 VerifyBootstrapIntegrity。
// 测试用它注入「校验失败」的结论，无需真实数据库即可覆盖启动裁决。
var integrityVerifier = VerifyBootstrapIntegrity

// enforceBootstrapIntegrity 按 BOOTSTRAP_INTEGRITY_STRICT 裁决启动是否可以继续。
//
//   - 严格（默认）：校验无法执行或任一检查项不通过 → 返回错误，调用链
//     NewRuntime → NewHTTPServer → runServer 以非 0 退出码终止，监听器不会创建。
//   - 显式关闭（仅非生产）：完全不执行校验，打 WARN 声明「完整性校验被显式跳过」。
func enforceBootstrapIntegrity(ctx context.Context, pool *pgxpool.Pool) error {
	strict, err := config.BootstrapIntegrityStrict()
	if err != nil {
		slog.Error("bootstrap integrity switch is invalid; refusing to start", "error", err)
		return apperrors.WrapUnavailable(err, "invalid bootstrap integrity switch")
	}
	if !strict {
		slog.Warn("bootstrap integrity check explicitly skipped; the process starts without proving the database baseline",
			"switch", "BOOTSTRAP_INTEGRITY_STRICT=false",
			"app_env", os.Getenv("APP_ENV"),
			"consequence", "ledger gaps, empty seed tables, or a missing admin credential will not stop startup",
		)
		return nil
	}

	report, err := integrityVerifier(ctx, pool)
	if err != nil {
		slog.Error("bootstrap integrity verification could not run; refusing to start", "error", err)
		return apperrors.WrapUnavailable(err, "bootstrap integrity verification could not run")
	}
	LogIntegrityReport(report)
	if !report.ChecksPassed {
		slog.Error("bootstrap integrity check failed; refusing to start",
			"failures", report.Failures,
			"remediation", "run `go run ./cmd/migrate up --seed`, or set BOOTSTRAP_INTEGRITY_STRICT=false for a non-production database",
		)
		return apperrors.NewUnavailablef(
			"bootstrap integrity check failed; refusing to serve an unverified baseline: %s",
			strings.Join(report.Failures, "; "))
	}
	return nil
}

// EnsureBootstrapAdminCredential 在首次播种后初始化内置管理员凭据。
//
// 凭据优先级：BOOTSTRAP_ADMIN_PASSWORD > 数据链 V1 的开发哨兵哈希。
// 用户自定义的非空哈希永不覆盖；空哈希必须由环境显式提供安全口令；
// 生产环境检测到开发哨兵时拒绝执行（除非环境口令替换它）。
func EnsureBootstrapAdminCredential(ctx context.Context, pool *pgxpool.Pool) error {
	if !config.AutoSeed() {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return apperrors.WrapInternal(err, "begin bootstrap admin credential transaction")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT set_config('app.current_role', 'super_admin', true)"); err != nil {
		return apperrors.WrapInternal(err, "set bootstrap admin tenant context")
	}

	var userID string
	var existingHash *string
	err = tx.QueryRow(ctx, `
		SELECT u.id, c.password_hash
		FROM guliangiam.users u
		JOIN guliangiam.organizations o ON o.id = u.organization_id
		LEFT JOIN guliangiam.user_credentials c ON c.user_id = u.id
		WHERE u.username = 'admin' AND o.slug = 'built-in'
		  AND u.deleted_at IS NULL AND o.deleted_at IS NULL
		FOR UPDATE OF u`).Scan(&userID, &existingHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperrors.NewNotFound("built-in admin not found; apply the seed chain first")
	}
	if err != nil {
		return apperrors.WrapInternal(err, "read built-in admin credential")
	}

	hash, needed, err := bootstrapAdminCredentialHash(existingHash, os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"))
	if err != nil {
		return err
	}
	if !needed {
		return tx.Commit(ctx)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO guliangiam.user_credentials
			(user_id, password_hash, password_algo, password_updated_at, need_update_password)
		VALUES ($1, $2, 'argon2id', now(), true)
		ON CONFLICT (user_id) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			password_algo = EXCLUDED.password_algo,
			password_updated_at = EXCLUDED.password_updated_at,
			need_update_password = EXCLUDED.need_update_password
		WHERE guliangiam.user_credentials.password_hash IS NULL
		   OR btrim(guliangiam.user_credentials.password_hash) = ''
		   OR guliangiam.user_credentials.password_hash = ANY($3::text[])`,
		userID, hash, bootstrapAdminSeedHashes()); err != nil {
		return apperrors.WrapInternal(err, "initialize built-in admin credential")
	}

	// Post-condition: re-scan to guarantee the row now carries a non-empty hash.
	var finalHash *string
	if err := tx.QueryRow(ctx, `
		SELECT password_hash FROM guliangiam.user_credentials WHERE user_id = $1
	`, userID).Scan(&finalHash); err != nil {
		return apperrors.WrapInternal(err, "verify built-in admin credential persistence")
	}
	if finalHash == nil || strings.TrimSpace(*finalHash) == "" {
		return apperrors.NewInternal("built-in admin credential still empty after EnsureBootstrapAdminCredential; the password may have been locked by a parallel process")
	}
	return tx.Commit(ctx)
}

func bootstrapAdminCredentialHash(existingHash *string, password string) (hashOut string, overridden bool, err error) {
	trimmed := ""
	if existingHash != nil {
		trimmed = strings.TrimSpace(*existingHash)
	}

	// 用户自定义凭据永不被种子重放覆盖。
	if trimmed != "" && !isBootstrapAdminSeedHash(trimmed) {
		return "", false, nil
	}

	// 哨兵/空哈希：优先使用环境显式口令。
	if strings.TrimSpace(password) == "" {
		if trimmed == SeedBootstrapAdminHash {
			if config.IsProduction() {
				return "", false, apperrors.NewUnavailable(
					"production seed contains the development bootstrap credential; " +
						"set BOOTSTRAP_ADMIN_PASSWORD to replace it before starting")
			}
			slog.Warn("development bootstrap credential is active; set BOOTSTRAP_ADMIN_PASSWORD to rotate it")
			return "", false, nil
		}
		if trimmed != "" {
			if config.IsProduction() {
				return "", false, apperrors.NewUnavailable(
					"production seed contains a legacy development bootstrap credential; " +
						"set BOOTSTRAP_ADMIN_PASSWORD to replace it before starting")
			}
			slog.Warn("legacy development bootstrap credential was migrated to the current sentinel")
			return SeedBootstrapAdminHash, true, nil
		}
		return "", false, validateBootstrapAdminPassword(password)
	}

	if err := validateBootstrapAdminPassword(password); err != nil {
		return "", false, err
	}
	manager := cred.GetCredManager("argon2id")
	if manager == nil {
		return "", false, apperrors.NewInternal("argon2id credential manager unavailable")
	}
	hash := manager.GetHashedPassword(password, "")
	if hash == "" {
		return "", false, apperrors.NewInternal("hash bootstrap admin password")
	}
	return hash, true, nil
}

func isBootstrapAdminSeedHash(hash string) bool {
	if hash == SeedBootstrapAdminHash {
		return true
	}
	for _, legacyHash := range legacySeedBootstrapAdminHashes {
		if hash == legacyHash {
			return true
		}
	}
	return false
}

func bootstrapAdminSeedHashes() []string {
	hashes := make([]string, 1, len(legacySeedBootstrapAdminHashes)+1)
	hashes[0] = SeedBootstrapAdminHash
	return append(hashes, legacySeedBootstrapAdminHashes...)
}

func validateBootstrapAdminPassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < 12 || length > 128 {
		return apperrors.NewInvalidArgument("BOOTSTRAP_ADMIN_PASSWORD must contain 12-128 characters")
	}
	common := map[string]struct{}{
		"admin": {}, "admin123": {}, "password": {}, "password123": {},
		"12345678": {}, "qwerty123": {}, "changeme": {}, "change-me": {},
	}
	if _, found := common[strings.ToLower(password)]; found {
		return apperrors.NewInvalidArgument("BOOTSTRAP_ADMIN_PASSWORD must not be a common password")
	}

	var upper, lower, digit, special bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			special = true
		}
	}
	if !upper || !lower || !digit || !special {
		return apperrors.NewInvalidArgument("BOOTSTRAP_ADMIN_PASSWORD must include uppercase, lowercase, digit, and special characters")
	}
	return nil
}
