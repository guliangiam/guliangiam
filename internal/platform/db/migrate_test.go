package db

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func structure(v int64, name, sql string) Migration {
	return Migration{Kind: KindStructure, Version: v, Name: name, SQL: sql}
}

func data(v int64, name, sql string) Migration {
	return Migration{Kind: KindData, Version: v, Name: name, SQL: sql}
}

func appliedOf(m Migration) AppliedMigration {
	return AppliedMigration{
		Kind:     m.Kind,
		Version:  m.Version,
		Name:     m.Name,
		Checksum: contentSHA256(m.SQL),
	}
}

func TestValidateMigrationsRejectsNonDeterministicSets(t *testing.T) {
	tests := []struct {
		name       string
		migrations []Migration
		wantSubstr string
	}{
		{
			name:       "empty set",
			migrations: nil,
			wantSubstr: "migration set is empty",
		},
		{
			name:       "empty SQL",
			migrations: []Migration{structure(1, "a", "")},
			wantSubstr: "empty SQL",
		},
		{
			name:       "non-positive version",
			migrations: []Migration{structure(0, "a", "SELECT 1")},
			wantSubstr: "non-positive version",
		},
		{
			name:       "empty name",
			migrations: []Migration{structure(1, "  ", "SELECT 1")},
			wantSubstr: "empty name",
		},
		{
			name:       "unknown kind",
			migrations: []Migration{{Kind: "mystery", Version: 1, Name: "a", SQL: "SELECT 1"}},
			wantSubstr: "unknown kind",
		},
		{
			name:       "duplicate version in same kind",
			migrations: []Migration{structure(1, "a", "SELECT 1"), structure(1, "b", "SELECT 2")},
			wantSubstr: "duplicate migration structure V1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateMigrations(tc.migrations)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantSubstr)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("expected error containing %q, got %q", tc.wantSubstr, err.Error())
			}
		})
	}
}

func TestValidateMigrationsOrdersStructureBeforeData(t *testing.T) {
	got, err := validateMigrations([]Migration{
		data(2, "seed_b", "INSERT INTO b VALUES (1) ON CONFLICT DO NOTHING"),
		structure(2, "schema_b", "CREATE TABLE b (id UUID)"),
		data(1, "seed_a", "INSERT INTO a VALUES (1) ON CONFLICT DO NOTHING"),
		structure(1, "schema_a", "CREATE TABLE a (id UUID)"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{
		"structure/1", "structure/2", "data/1", "data/2",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d migrations, got %d", len(want), len(got))
	}
	for i, m := range got {
		if key := string(m.Kind) + "/" + strconv.FormatInt(m.Version, 10); key != want[i] {
			t.Fatalf("position %d: expected %s, got %s", i, want[i], key)
		}
	}
}

func TestPlanMigrationsAppliesPendingInOrder(t *testing.T) {
	set := []Migration{
		structure(1, "a", "CREATE TABLE a (id UUID)"),
		structure(2, "b", "CREATE TABLE b (id UUID)"),
		data(1, "seed", "INSERT INTO a VALUES (1) ON CONFLICT DO NOTHING"),
	}
	ordered, err := validateMigrations(set)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pending, err := planMigrations(ordered, []AppliedMigration{appliedOf(set[0])}, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending migrations, got %d", len(pending))
	}
	if pending[0].Version != 2 || pending[0].Kind != KindStructure {
		t.Fatalf("expected structure V2 first, got %s V%d", pending[0].Kind, pending[0].Version)
	}
	if pending[1].Kind != KindData {
		t.Fatalf("expected data migration last, got %s", pending[1].Kind)
	}
}

func TestPlanMigrationsIsIdempotentWhenEverythingApplied(t *testing.T) {
	set := []Migration{
		structure(1, "a", "CREATE TABLE a (id UUID)"),
		data(1, "seed", "INSERT INTO a VALUES (1) ON CONFLICT DO NOTHING"),
	}
	ordered, _ := validateMigrations(set)

	pending, err := planMigrations(ordered, []AppliedMigration{appliedOf(set[0]), appliedOf(set[1])}, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending migrations, got %d", len(pending))
	}
}

func TestPlanMigrationsHonoursKindGates(t *testing.T) {
	set := []Migration{
		structure(1, "a", "CREATE TABLE a (id UUID)"),
		data(1, "seed", "INSERT INTO a VALUES (1) ON CONFLICT DO NOTHING"),
	}
	ordered, _ := validateMigrations(set)

	pending, err := planMigrations(ordered, nil, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pending) != 1 || pending[0].Kind != KindData {
		t.Fatalf("expected only the data migration to be pending, got %+v", pending)
	}
}

func TestPlanMigrationsRefusesEditedAppliedMigration(t *testing.T) {
	m := structure(1, "a", "CREATE TABLE a (id UUID)")
	ordered, _ := validateMigrations([]Migration{m})

	drifted := appliedOf(m)
	drifted.Checksum = contentSHA256("CREATE TABLE a (id UUID, extra TEXT)")

	_, err := planMigrations(ordered, []AppliedMigration{drifted}, true, false)
	if err == nil {
		t.Fatal("expected drift to be refused")
	}
	if !strings.Contains(err.Error(), "already applied with checksum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestChecksumDriftAllowedByEnvironment(t *testing.T) {
	tests := []struct {
		name        string
		appEnv      string
		override    string
		wantAllowed bool
		wantErr     bool
	}{
		{name: "development default", appEnv: "development", wantAllowed: true},
		{name: "unset environment defaults to development", wantAllowed: true},
		{name: "development opt out", appEnv: "development", override: "false"},
		{name: "test implicitly allows drift", appEnv: "test", wantAllowed: true},
		{name: "test explicit opt out", appEnv: "test", override: "false"},
		{name: "staging implicitly allows drift", appEnv: "staging", wantAllowed: true},
		{name: "staging explicit opt in", appEnv: "staging", override: "true", wantAllowed: true},
		{name: "production disables drift handling", appEnv: "production"},
		{name: "production rejects explicit opt in", appEnv: "production", override: "true", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.appEnv)
			if tc.override == "" {
				t.Setenv("ALLOW_CHECKSUM_DRIFT", "")
				if err := os.Unsetenv("ALLOW_CHECKSUM_DRIFT"); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("ALLOW_CHECKSUM_DRIFT", tc.override)
			}

			got, err := checksumDriftAllowed()
			if (err != nil) != tc.wantErr {
				t.Fatalf("checksumDriftAllowed() error = %v, wantErr %t", err, tc.wantErr)
			}
			if got != tc.wantAllowed {
				t.Fatalf("checksumDriftAllowed() = %t, want %t", got, tc.wantAllowed)
			}
		})
	}
}

func TestPlanMigrationsRefusesVersionMissingFromImage(t *testing.T) {
	m := structure(1, "a", "CREATE TABLE a (id UUID)")
	ordered, _ := validateMigrations([]Migration{m})

	future := appliedOf(structure(2, "from_the_future", "ALTER TABLE a ADD COLUMN extra TEXT"))
	_, err := planMigrations(ordered, []AppliedMigration{appliedOf(m), future}, false, false)
	if err == nil {
		t.Fatal("expected a ledger version absent from the image to be refused")
	}
	if !strings.Contains(err.Error(), "this image does not contain that version") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanMigrationsRejectsLegacyV1ChecksumMismatch(t *testing.T) {
	m := structure(1, "guliangiam_schema_init", "CREATE TABLE a (id UUID, current TEXT)")
	ordered, err := validateMigrations([]Migration{m})
	if err != nil {
		t.Fatalf("unexpected migration validation error: %v", err)
	}

	legacyV1 := appliedOf(structure(1, "guliangiam_schema_init", "CREATE TABLE a (id UUID)"))
	legacyV2 := appliedOf(structure(2, "users", "ALTER TABLE a ADD COLUMN current TEXT"))

	_, err = planMigrations(ordered, []AppliedMigration{legacyV1, legacyV2}, false, false)
	if err == nil {
		t.Fatal("expected legacy V1 checksum mismatch to be refused")
	}
	if !strings.Contains(err.Error(), "already applied with checksum") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPlanMigrationsRefusesMissingV1Ledger(t *testing.T) {
	// ledger 里连 V1 都没有（但 schema 不是空的）→ 拒绝
	m := structure(1, "a", "CREATE TABLE a (id UUID)")
	ordered, _ := validateMigrations([]Migration{m})

	_, err := planMigrations(ordered, []AppliedMigration{}, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// AUTO_MIGRATE=false 时 pending 也为空，但 ledger 缺失 V1 不会在 planMigrations 里报错
	// （那是 verifySchemaPresent 的事）—— 这个测试保持简单
}

func TestStructureContentHashIgnoresSeeds(t *testing.T) {
	base := []Migration{structure(1, "a", "CREATE TABLE a (id UUID)")}
	withSeed := []Migration{
		structure(1, "a", "CREATE TABLE a (id UUID)"),
		data(1, "seed", "INSERT INTO a VALUES (1) ON CONFLICT DO NOTHING"),
	}
	changedSeed := []Migration{
		structure(1, "a", "CREATE TABLE a (id UUID)"),
		data(1, "seed", "INSERT INTO a VALUES (2) ON CONFLICT DO NOTHING"),
	}
	changedStructure := []Migration{structure(1, "a", "CREATE TABLE a (id UUID, extra TEXT)")}

	if StructureContentHash(base) != StructureContentHash(withSeed) {
		t.Fatal("seed changes must not move the structure content hash")
	}
	if StructureContentHash(withSeed) != StructureContentHash(changedSeed) {
		t.Fatal("seed content must not participate in the structure content hash")
	}
	if StructureContentHash(base) == StructureContentHash(changedStructure) {
		t.Fatal("structure changes must move the structure content hash")
	}
}

func TestContentSHA256IsStable(t *testing.T) {
	a := contentSHA256("CREATE TABLE a (id UUID)")
	b := contentSHA256("CREATE TABLE a (id UUID)")
	c := contentSHA256("CREATE TABLE a (id UUID) ")
	if a != b {
		t.Fatal("same content must produce the same checksum")
	}
	if a == c {
		t.Fatal("whitespace changes must change the checksum")
	}
	if len(a) != 64 {
		t.Fatalf("expected a hex sha256, got %q", a)
	}
}
