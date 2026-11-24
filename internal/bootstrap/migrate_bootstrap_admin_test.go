package bootstrap

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/db"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestBootstrapAdminCredentialHash_PreservesExistingCredential(t *testing.T) {
	existing := "existing-argon2id-hash"
	hash, needed, err := bootstrapAdminCredentialHash(&existing, "")
	if err != nil {
		t.Fatalf("existing credential must not require a bootstrap password: %v", err)
	}
	if needed || hash != "" {
		t.Fatalf("existing credential must not be replaced, needed=%v hash=%q", needed, hash)
	}
}

func TestValidateBootstrapAdminPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "valid", password: "Str0ng!Pass12"},
		{name: "empty", password: "", wantErr: true},
		{name: "known admin password", password: "admin", wantErr: true},
		{name: "too short", password: "Ab1!", wantErr: true},
		{name: "missing uppercase", password: "alllowercase1!xx", wantErr: true},
		{name: "missing lowercase", password: "ALLUPPERCASE1!XX", wantErr: true},
		{name: "missing digit", password: "NoDigitsHere!Xx", wantErr: true},
		{name: "missing special", password: "NoSpecial123Xx", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBootstrapAdminPassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateBootstrapAdminPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var appErr *apperrors.Error
				if !strings.Contains(err.Error(), "BOOTSTRAP_ADMIN_PASSWORD") {
					t.Fatalf("error must identify the environment variable, got %v", err)
				}
				if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeInvalidArgument {
					t.Fatalf("error = %v, want invalid_argument", err)
				}
			}
		})
	}
}

func TestBootstrapAdminCredentialHash_GeneratesArgon2idForMissingCredential(t *testing.T) {
	var emptyHash string
	hash, needed, err := bootstrapAdminCredentialHash(&emptyHash, "Str0ng!Pass12")
	if err != nil {
		t.Fatalf("bootstrapAdminCredentialHash() error = %v", err)
	}
	if !needed {
		t.Fatal("missing credential must require initialization")
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("hash prefix = %q, want argon2id", hash)
	}
	if strings.Contains(hash, "Str0ng!Pass12") {
		t.Fatal("hash must not contain the plaintext password")
	}
}

func TestEnsureBootstrapAdminCredential_SeedDisabled(t *testing.T) {
	t.Setenv("AUTO_SEED", "false")
	if err := EnsureBootstrapAdminCredential(context.Background(), nil); err != nil {
		t.Fatalf("disabled seed must not touch the database: %v", err)
	}
}

// 2026-10-29 开发期 squash 后结构链只保留 V1 基线：原 V10–V12 新增的表/列已并入
// 生成式基线，因此断言基线内容而不是历史版本号。
func TestV1BaselineCarriesLifecycleRegistryAndTrustSchema(t *testing.T) {
	var baseline string
	for _, migration := range Migrations() {
		if migration.Kind == "structure" && migration.Version == 1 {
			baseline = migration.SQL
			break
		}
	}
	if baseline == "" {
		t.Fatal("V1 structure baseline is not registered")
	}
	for _, fragment := range []string{
		"guliangiam.ticket_messages", "is_privileged",
		"organization_domains", "authorization_chain_state", "cross_tenant_operations",
		"authorization_audit_records", "pay_channel_configs", "pay_callback_receipts",
		"pay_refunds", "pay_attempts", "pay_recon_batches", "pay_recon_items",
		"llm_metering_events", "ldap_sync_tasks",
		"access_token_ttl_seconds", "ck_service_accounts_access_token_ttl",
	} {
		if !strings.Contains(baseline, fragment) {
			t.Fatalf("V1 baseline is missing %q", fragment)
		}
	}
}

func TestMigrationsDeclareContiguousStructureChainAndSingleDataBaseline(t *testing.T) {
	migrations := Migrations()
	if len(migrations) != 23 {
		t.Fatalf("chain must declare structure V1..V21 + data V1/V2, got %d", len(migrations))
	}
	want := []struct {
		kind    db.MigrationKind
		version int64
	}{
		{db.KindStructure, 1},
		{db.KindStructure, 2},
		{db.KindStructure, 3},
		{db.KindStructure, 4},
		{db.KindStructure, 5},
		{db.KindStructure, 6},
		{db.KindStructure, 7},
		{db.KindStructure, 8},
		{db.KindStructure, 9},
		{db.KindStructure, 10},
		{db.KindStructure, 11},
		{db.KindStructure, 12},
		{db.KindStructure, 13},
		{db.KindStructure, 14},
		{db.KindStructure, 15},
		{db.KindStructure, 16},
		{db.KindStructure, 17},
		{db.KindStructure, 18},
		{db.KindStructure, 19},
		{db.KindStructure, 20},
		{db.KindStructure, 21},
		{db.KindData, 1},
		{db.KindData, 2},
	}
	for i, expected := range want {
		if migrations[i].Kind != expected.kind || migrations[i].Version != expected.version {
			t.Fatalf("migration[%d] = %s/V%d, want %s/V%d", i, migrations[i].Kind,
				migrations[i].Version, expected.kind, expected.version)
		}
	}
	// 结构链必须从 V1 起连续；V6 增加 access_secret 宽限期列与护照/驾照个人信息列，
	// V7/V8/V9 依次扩展 ck_providers_kind 闭集（tencent_idv、SMS 批次、Email 批次），
	// V10 再扩展 ck_providers_kind（Captcha 批次 tencent_captcha），
	// V11 扩展 identity 闭集（OPEN-API-75 P5 onfido/veriff/persona）。
	// V12 扩展 notification 闭集（OPEN-API-75 P6 ntfy/serverchan/pagerduty/opsgenie/gotify）。
	// V13 扩展 identity 闭集 Storage 家族（OPEN-API-75 P7 cloudflare_r2/backblaze_b2/digitalocean_spaces）。
	// V14 扩展 payment 闭集（OPEN-API-75 P8 square/checkout）。
	// V15 为 idempotency_keys 增加 content_type 列（BE-E-12 回放还原 Content-Type）。
	// V16 为 identity_verifications 增加 biz_seq/action/product_code/subject_ref 列
	// 与 uq_idv_biz_seq 唯一约束，并新建 user_face_templates 加密底图表（FaceAction 闭环）。
	// V20 扩展四个 pay 渠道 CHECK 闭集（square/checkout），打通渠道配置→激活→扣款链。
	// V21 新增个人通知中心（user_notifications/replies/preferences/channel_bindings）
	// 与 notifications(user_notification_id/channel/provider_kind)、验证码应用/提供商绑定列。
	var structureVersions []int64
	for _, migration := range migrations {
		if migration.Kind == db.KindStructure {
			structureVersions = append(structureVersions, migration.Version)
		}
	}
	if len(structureVersions) == 0 {
		t.Fatal("structure chain must not be empty")
	}
	for i, version := range structureVersions {
		if version != int64(i+1) {
			t.Fatalf("structure chain must be contiguous V1..V%d, got %v", len(structureVersions), structureVersions)
		}
	}
}

// structureBaseline 返回折叠后的 V1 结构基线 SQL。
func structureBaseline(t *testing.T) string {
	t.Helper()
	for _, migration := range Migrations() {
		if migration.Kind == db.KindStructure && migration.Version == 1 {
			return migration.SQL
		}
	}
	t.Fatal("V1 structure baseline is not registered")
	return ""
}

// V1 折叠基线必须覆盖原 V2/V5 对 providers.kind 闭集的全部厂商取值。
func TestFoldedBaselineCoversProviderKindClosedSet(t *testing.T) {
	baseline := structureBaseline(t)
	for _, fragment := range []string{
		"ck_providers_kind", "'alibaba_sms'", "'twilio_sms'", "'sendgrid'",
		"'aws_s3'", "'recaptcha'", "'totp'",
	} {
		if !strings.Contains(baseline, fragment) {
			t.Fatalf("folded V1 baseline is missing %q", fragment)
		}
	}
}

// V1 折叠基线必须保留原 V4 的凭据列扩宽：provider 凭据列为 TEXT，容纳 envelope 密文。
func TestFoldedBaselineCarriesWidenedProviderCredentialColumns(t *testing.T) {
	baseline := structureBaseline(t)
	for _, fragment := range []string{
		"client_secret TEXT NOT NULL",
		"client_secret2 TEXT NOT NULL",
		"password TEXT NOT NULL",
		"app_secret TEXT NOT NULL",
		"access_token TEXT NOT NULL",
	} {
		if !strings.Contains(baseline, fragment) {
			t.Fatalf("folded V1 baseline is missing %q", fragment)
		}
	}
}

// V1 折叠基线必须保留原 V3 的微信支付规范渠道键，同时保留 wxpay 历史别名。
func TestFoldedBaselineAddsCanonicalWeChatPayChannel(t *testing.T) {
	baseline := structureBaseline(t)
	for _, fragment := range []string{
		"ck_receipts_channel", "ck_pay_channel_configs_channel",
		"'wechat_pay'", "'wxpay'",
	} {
		if !strings.Contains(baseline, fragment) {
			t.Fatalf("folded V1 baseline is missing %q", fragment)
		}
	}
}

// V1 折叠基线必须保留原 V6 的密码派生算法列与闭集约束；存储哈希不新增国密选项。
func TestFoldedBaselineCarriesPasswordDeriveAlgorithm(t *testing.T) {
	baseline := structureBaseline(t)
	for _, fragment := range []string{
		"password_derive_algorithm VARCHAR(16) NOT NULL DEFAULT 'hmac_sha256'",
		"ck_organization_security_policies_password_derive_algorithm",
		"'hmac_sm3'",
	} {
		if !strings.Contains(baseline, fragment) {
			t.Fatalf("folded V1 baseline is missing %q", fragment)
		}
	}
	if strings.Contains(baseline, "password_hash_algorithm") {
		t.Fatal("V1 must not declare a password_hash_algorithm column")
	}
}

// 结构迁移必须同时存在于 schema_sql/ 与 Migrations()：只生成 SQL 文件而漏登记
// go:embed 变量或 migrations 条目时，cmd/migrate 与 -migrate 都不会应用它，
// 声明式 Schema 已经前移而运行期约束永远不更新。此用例把两者交叉比对。
func TestStructureSchemaFilesAreRegistered(t *testing.T) {
	entries, err := os.ReadDir("schema_sql")
	if err != nil {
		t.Fatalf("read schema_sql: %v", err)
	}
	registered := make(map[int64]string)
	for _, migration := range Migrations() {
		if migration.Kind == db.KindStructure {
			registered[migration.Version] = migration.Name
		}
	}
	matched := make(map[int64]bool)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "V") || !strings.HasSuffix(name, ".sql") {
			continue
		}
		parts := strings.SplitN(strings.TrimSuffix(name, ".sql"), "__", 2)
		if len(parts) != 2 {
			t.Fatalf("schema file %q does not follow V{version}__{name}.sql", name)
		}
		version, err := strconv.ParseInt(strings.TrimPrefix(parts[0], "V"), 10, 64)
		if err != nil {
			t.Fatalf("schema file %q has a non-numeric version: %v", name, err)
		}
		registeredName, ok := registered[version]
		if !ok {
			t.Fatalf("schema file %q is not registered in Migrations()", name)
		}
		if registeredName != parts[1] {
			t.Fatalf("Migrations() V%d name = %q, want %q", version, registeredName, parts[1])
		}
		matched[version] = true
	}
	if len(matched) != len(registered) {
		t.Fatalf("Migrations() declares %d structure versions, schema_sql/ has %d", len(registered), len(matched))
	}
}
