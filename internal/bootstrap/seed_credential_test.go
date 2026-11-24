package bootstrap

import (
	"strings"
	"testing"

	"github.com/guliangiam/guliangiam/cred"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// testBootstrapPasswordFixture 是显式合成的测试夹具口令，不是任何环境的真实凭据。
const testBootstrapPasswordFixture = "fixture-only-NotARealSecret-1A!"

// The development bootstrap credential must stay usable end-to-end: the seed
// SQL carries exactly the Go sentinel, and the sentinel verifies the password
// documented in AGENTS.md. A drift between the two makes the seed unusable.
func TestSeedBootstrapCredentialHashMatchesDocumentedPassword(t *testing.T) {
	ok, err := cred.VerifyArgon2id("Guliang@2026", SeedBootstrapAdminHash)
	if err != nil {
		t.Fatalf("verify sentinel hash: %v", err)
	}
	if !ok {
		t.Fatal("documented development password does not match the seed sentinel hash")
	}
	if !strings.Contains(seedDML, SeedBootstrapAdminHash) {
		t.Fatal("seed SQL does not contain SeedBootstrapAdminHash; run scripts/sync-seed-mirror.sh after editing db/seeds")
	}
	if !strings.Contains(seedDML, "need_update_password") {
		t.Fatal("seed SQL must mark the bootstrap credential as pending a forced password change")
	}
}

func TestSeedContainsCoreFixtures(t *testing.T) {
	required := []string{
		"built-in", "app-built-in", "admin", "platform_admins", "platform_admin_scopes",
		"built-in-administrators", "built-in-operators", "built-in-admin-all", "built-in-viewer-read",
		"organization_auth_config", "organization_billing", "organization_branding", "organization_theme_settings",
		"built-in-demo-product", "built-in-demo-monthly", "built-in-demo-pricing", "WELCOME10",
		"built-in-llm-disabled", "gpt-4o-mini", "built-in-chat-alias",
		"provider_configs", "provider_secrets", "_provider_registry_v1",
	}
	for _, needle := range required {
		if !strings.Contains(seedDML, needle) {
			t.Errorf("seed SQL is missing required fixture marker %q", needle)
		}
	}
}

// platformAdminScopeInsert 截取种子中 platform_admin_scopes 的 INSERT 语句。
func platformAdminScopeInsert(t *testing.T) string {
	t.Helper()
	const marker = "INSERT INTO guliangiam.platform_admin_scopes"
	start := strings.Index(seedDML, marker)
	if start < 0 {
		t.Fatal("seed SQL has no platform_admin_scopes INSERT")
	}
	rest := seedDML[start:]
	end := strings.Index(rest, ";")
	if end < 0 {
		t.Fatal("platform_admin_scopes INSERT is not terminated")
	}
	return rest[:end]
}

// 种子里的内置管理员必须带 “all + is_bootstrap=true + 无审批人” 的 scope 行；
// 这正是 scopeValidity（未过期 && (is_bootstrap || approved_by 非空)）的唯一免审批来源。
// 取值一旦漂移（例如补上 approved_by、把 is_bootstrap 置 false 或换成 targeted），
// 平台管理员会重新看不见组织列表。
func TestSeedPlatformAdminAllScopeShape(t *testing.T) {
	stmt := platformAdminScopeInsert(t)

	required := []string{
		"'all'",
		"ARRAY[]::uuid[]",
		"now() + INTERVAL '1 year'",
		"is_bootstrap",
		// approved_by, approved_at 必须为 NULL，is_bootstrap 必须为 true（种子免审批行）。
		"'all', ARRAY[]::uuid[], now() + INTERVAL '1 year', NULL, NULL, true, now()",
	}
	for _, needle := range required {
		if !strings.Contains(stmt, needle) {
			t.Errorf("platform_admin_scopes seed row does not contain %q; statement: %s", needle, stmt)
		}
	}
}

func TestBootstrapAdminCredentialHashPrecedence(t *testing.T) {
	t.Setenv("APP_ENV", "development")

	tests := []struct {
		name         string
		existing     *string
		password     string
		wantHash     bool
		wantSentinel bool
		wantErr      bool
		wantUnavail  bool
	}{
		{
			name:     "empty hash requires an explicit password",
			existing: nil,
			wantErr:  true,
		},
		{
			name:     "empty hash accepts the environment password",
			existing: nil,
			password: testBootstrapPasswordFixture,
			wantHash: true,
		},
		{
			name:     "sentinel stays usable without a password in development",
			existing: strPtr(SeedBootstrapAdminHash),
		},
		{
			name:         "legacy sentinel migrates to the documented sentinel",
			existing:     strPtr(legacySeedBootstrapAdminHashes[0]),
			wantHash:     true,
			wantSentinel: true,
		},
		{
			name:     "sentinel is replaced by the environment password",
			existing: strPtr(SeedBootstrapAdminHash),
			password: testBootstrapPasswordFixture,
			wantHash: true,
		},
		{
			name:     "custom credential is never overwritten",
			existing: strPtr("$argon2id$v=19$m=65536,t=1,p=8$custom$hash"),
			password: testBootstrapPasswordFixture,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hash, needed, err := bootstrapAdminCredentialHash(tc.existing, tc.password)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if needed != tc.wantHash {
				t.Fatalf("needed = %v, want %v", needed, tc.wantHash)
			}
			if tc.wantHash && hash == "" {
				t.Fatal("expected a non-empty hash")
			}
			if tc.wantSentinel && hash != SeedBootstrapAdminHash {
				t.Fatal("legacy sentinel was not migrated to the documented development sentinel")
			}
		})
	}
}

func TestBootstrapAdminCredentialHashRejectsSentinelInProduction(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	for _, hash := range append([]string{SeedBootstrapAdminHash}, legacySeedBootstrapAdminHashes...) {
		_, needed, err := bootstrapAdminCredentialHash(strPtr(hash), "")
		if err == nil {
			t.Fatal("production must reject a development sentinel without an environment password")
		}
		if needed {
			t.Fatal("a rejected credential must not be written")
		}
		if got := apperrors.FromError(err).HTTPStatus(); got != 503 {
			t.Fatalf("status = %d, want 503", got)
		}
	}
}

func strPtr(v string) *string { return &v }
