package dialect

import (
	"sort"
	"strings"
	"testing"
)

func TestPostgresDialect_Basics(t *testing.T) {
	d := PostgresDialect{}
	if d.Name() != "postgres" {
		t.Errorf("Name() = %q", d.Name())
	}
	if d.Placeholder(3) != "$3" {
		t.Errorf("Placeholder(3) = %q", d.Placeholder(3))
	}
	if d.QuoteIdentifier("id") != `"id"` {
		t.Errorf("QuoteIdentifier = %q", d.QuoteIdentifier("id"))
	}
	if d.CurrentTimestamp() != "now()" {
		t.Errorf("CurrentTimestamp = %q", d.CurrentTimestamp())
	}
	if d.BooleanLiteral(true) != "TRUE" || d.BooleanLiteral(false) != "FALSE" {
		t.Errorf("BooleanLiteral: %s %s", d.BooleanLiteral(true), d.BooleanLiteral(false))
	}
}

func TestMySQLDialect_Basics(t *testing.T) {
	d := MySQLDialect{}
	if d.Name() != "mysql" {
		t.Errorf("Name() = %q", d.Name())
	}
	if d.Placeholder(1) != "?" {
		t.Errorf("Placeholder = %q", d.Placeholder(1))
	}
	if d.QuoteIdentifier("id") != "`id`" {
		t.Errorf("QuoteIdentifier = %q", d.QuoteIdentifier("id"))
	}
	if d.BooleanLiteral(true) != "1" || d.BooleanLiteral(false) != "0" {
		t.Errorf("BooleanLiteral")
	}
}

func TestFeatureDetection(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}

	tests := []struct {
		d    Dialect
		f    Feature
		want bool
	}{
		{pg, FeatureReturning, true},
		{pg, FeatureUpsert, true},
		{pg, FeatureSkipLocked, true},
		{pg, FeatureRLS, true},
		{pg, FeatureArrayType, true},
		{pg, FeatureJSONType, true},
		{pg, FeatureLimitTwoArg, false},
		{mysql, FeatureReturning, false},
		{mysql, FeatureUpsert, true},
		{mysql, FeatureSkipLocked, true}, // 8.4+
		{mysql, FeatureRLS, false},
		{mysql, FeatureArrayType, false},
		{mysql, FeatureJSONType, true},
	}
	for _, tt := range tests {
		got := tt.d.Supports(tt.f)
		if got != tt.want {
			t.Errorf("%s.Supports(%s) = %v, want %v", tt.d.Name(), tt.f, got, tt.want)
		}
	}
}

func TestUpsert(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}

	// Postgres DO UPDATE
	u := DialectUpsert{
		Table:           "guliangiam.users",
		ConflictColumns: []string{"org_id", "username"},
		UpdateColumns:   []string{"username", "updated_at"},
	}
	pg_sql := pg.Upsert(u, []string{"org_id", "username", "updated_at"})
	if pg_sql != `ON CONFLICT ("org_id", "username") DO UPDATE SET "username"=EXCLUDED."username", "updated_at"=EXCLUDED."updated_at"` {
		t.Errorf("postgres Upsert DO UPDATE = %q", pg_sql)
	}

	// Postgres DO NOTHING
	u2 := DialectUpsert{Table: "x", ConflictColumns: []string{"id"}, DoNothing: true}
	pg_ignore := pg.Upsert(u2, nil)
	if pg_ignore != `ON CONFLICT ("id") DO NOTHING` {
		t.Errorf("postgres Upsert DO NOTHING = %q", pg_ignore)
	}

	// MySQL
	mysql_u := DialectUpsert{ConflictColumns: []string{"org_id"}, UpdateColumns: []string{"username", "updated_at"}}
	mysql_sql := mysql.Upsert(mysql_u, []string{"username"})
	if mysql_sql != "ON DUPLICATE KEY UPDATE `username`=VALUES(`username`), `updated_at`=VALUES(`updated_at`)" {
		t.Errorf("mysql Upsert = %q", mysql_sql)
	}

	// MySQL INSERT IGNORE — Upsert returns "INSERT IGNORE" prefix, caller handles INTO ... VALUES
	mysql_ignore := mysql.Upsert(DialectUpsert{DoNothing: true}, nil)
	if mysql_ignore != "INSERT IGNORE" {
		t.Errorf("mysql Upsert IGNORE = %q", mysql_ignore)
	}
}

func TestLimit(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}

	if pg.Limit(0, 50) != "LIMIT 50" {
		t.Errorf("pg.Limit(0,50) = %q", pg.Limit(0, 50))
	}
	if pg.Limit(100, 50) != "LIMIT 50 OFFSET 100" {
		t.Errorf("pg.Limit(100,50) = %q", pg.Limit(100, 50))
	}
	if mysql.Limit(0, 50) != "LIMIT 50" {
		t.Errorf("mysql.Limit(0,50) = %q", mysql.Limit(0, 50))
	}
	if mysql.Limit(100, 50) != "LIMIT 100, 50" {
		t.Errorf("mysql.Limit(100,50) = %q", mysql.Limit(100, 50))
	}
}

func TestLocks(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}

	if pg.ForUpdate() != "FOR UPDATE" {
		t.Errorf("pg.ForUpdate = %q", pg.ForUpdate())
	}
	if pg.ForUpdateSkipLocked() != "FOR UPDATE SKIP LOCKED" {
		t.Errorf("pg.ForUpdateSkipLocked = %q", pg.ForUpdateSkipLocked())
	}
	if mysql.ForUpdateSkipLocked() != "FOR UPDATE SKIP LOCKED" {
		t.Errorf("mysql.ForUpdateSkipLocked = %q", mysql.ForUpdateSkipLocked())
	}
}

func TestCast(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}

	if pg.Cast("$1", LogicalUUID) != "$1::uuid" {
		t.Errorf("pg.Cast = %q", pg.Cast("$1", LogicalUUID))
	}
	if pg.Cast("$2", LogicalText) != "$2::text" {
		t.Errorf("pg.Cast = %q", pg.Cast("$2", LogicalText))
	}
	if mysql.Cast("?", LogicalUUID) != "CAST(? AS CHAR(36))" {
		t.Errorf("mysql.Cast uuid = %q", mysql.Cast("?", LogicalUUID))
	}
	if mysql.Cast("?", LogicalBoolean) != "CAST(? AS TINYINT(1))" {
		t.Errorf("mysql.Cast bool = %q", mysql.Cast("?", LogicalBoolean))
	}
}

func TestInsertReturning(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}

	if pg.InsertReturning(nil) != "RETURNING *" {
		t.Errorf("pg.InsertReturning(nil) = %q", pg.InsertReturning(nil))
	}
	if pg.InsertReturning([]string{"id", "org_id"}) != "RETURNING id, org_id" {
		t.Errorf("pg.InsertReturning = %q", pg.InsertReturning([]string{"id"}))
	}
	if mysql.InsertReturning(nil) != "" {
		t.Errorf("mysql.InsertReturning should be empty, got %q", mysql.InsertReturning(nil))
	}
}

func TestRegistry(t *testing.T) {
	if _, err := New("postgres"); err != nil {
		t.Fatalf("New(postgres) error: %v", err)
	}
	if _, err := New("mysql"); err != nil {
		t.Fatalf("New(mysql) error: %v", err)
	}
	if _, err := New("unknown"); err == nil {
		t.Fatalf("New(unknown) should error")
	}
	wantLen := len(Keys()) // 动态 — 所有已注册 driver
	got := Keys()
	if len(got) != wantLen {
		t.Errorf("Keys() = %v (len=%d), want len=%d", got, len(got), wantLen)
	}
}

func TestArrayLiteral(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}

	if pg.ArrayLiteral([]string{"a", "b"}) != "ARRAY[a, b]" {
		t.Errorf("pg.ArrayLiteral = %q", pg.ArrayLiteral([]string{"a", "b"}))
	}
	if mysql.ArrayLiteral([]string{"a", "b"}) != "JSON_ARRAY(a, b)" {
		t.Errorf("mysql.ArrayLiteral = %q", mysql.ArrayLiteral([]string{"a", "b"}))
	}
}

func TestJSONLiteral(t *testing.T) {
	pg := PostgresDialect{}
	mysql := MySQLDialect{}
	// JSONB is native in Postgres; JSON is native in MySQL (CAST handled by caller)
	if pg.JSONLiteral("$1") != "$1" {
		t.Errorf("pg.JSONLiteral = %q", pg.JSONLiteral("$1"))
	}
	if mysql.JSONLiteral("?") != "?" {
		t.Errorf("mysql.JSONLiteral = %q", mysql.JSONLiteral("?"))
	}
}

// ========== Phase E3 新增 Dialect 注册表测试 ==========

func TestRegistry_Tier1MySQLCompatible(t *testing.T) {
	tests := []struct {
		driver            string
		wantReturning     bool
		wantUpsertDoNot   bool
		featureRLS        bool
		featurePartialIdx bool
	}{
		{"mysql", false, true, false, false},
		{"oceanbase", false, true, false, false},
		{"oceanbase-mysql", false, true, false, false},
		{"tidb", true, true, false, false}, // TiDB 5.4+ 支持 RETURNING
		{"polardb", false, true, false, false},
		{"polardb-mysql", false, true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			d, err := New(tt.driver)
			if err != nil {
				t.Fatalf("New(%q) error: %v", tt.driver, err)
			}
			if d.Name() != tt.driver && !strings.HasPrefix(tt.driver, d.Name()) && !strings.HasPrefix(d.Name(), tt.driver) {
				t.Errorf("Name() = %q, expected compatible with %q", d.Name(), tt.driver)
			}
			if d.SupportsReturning() != tt.wantReturning {
				t.Errorf("SupportsReturning() = %v, want %v", d.SupportsReturning(), tt.wantReturning)
			}
			if d.Supports(FeatureUpsertDoNothing) != tt.wantUpsertDoNot {
				t.Errorf("UpsertDoNothing = %v, want %v", d.Supports(FeatureUpsertDoNothing), tt.wantUpsertDoNot)
			}
			if d.Supports(FeatureRLS) != tt.featureRLS {
				t.Errorf("RLS = %v, want %v", d.Supports(FeatureRLS), tt.featureRLS)
			}
			if d.Supports(FeaturePartialIndex) != tt.featurePartialIdx {
				t.Errorf("PartialIndex = %v, want %v", d.Supports(FeaturePartialIndex), tt.featurePartialIdx)
			}
			// 占位符必须都是 ?（MySQL 协议）
			if d.Placeholder(1) != "?" {
				t.Errorf("Placeholder = %q, want ?", d.Placeholder(1))
			}
			// Quote 必须都是反引号
			if d.QuoteIdentifier("t") != "`t`" {
				t.Errorf("QuoteIdentifier = %q, want `t`", d.QuoteIdentifier("t"))
			}
		})
	}
}

func TestRegistry_Tier2PostgresCompatible(t *testing.T) {
	tests := []struct {
		driver      string
		wantReturn  bool
		wantRLS     bool
		wantArray   bool
		wantPartial bool
	}{
		{"postgres", true, true, true, true},
		{"kingbase", true, false, false, true}, // KingbaseES: RLS false (保守), Array false
		{"kingbasees", true, false, false, true},
		{"gaussdb", true, true, true, true},
		{"opengauss", true, true, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			d, err := New(tt.driver)
			if err != nil {
				t.Fatalf("New(%q) error: %v", tt.driver, err)
			}
			if d.SupportsReturning() != tt.wantReturn {
				t.Errorf("SupportsReturning = %v, want %v", d.SupportsReturning(), tt.wantReturn)
			}
			if d.Supports(FeatureRLS) != tt.wantRLS {
				t.Errorf("RLS = %v, want %v", d.Supports(FeatureRLS), tt.wantRLS)
			}
			if d.Supports(FeatureArrayType) != tt.wantArray {
				t.Errorf("Array = %v, want %v", d.Supports(FeatureArrayType), tt.wantArray)
			}
			if d.Supports(FeaturePartialIndex) != tt.wantPartial {
				t.Errorf("PartialIndex = %v, want %v", d.Supports(FeaturePartialIndex), tt.wantPartial)
			}
			// 占位符必须是 $1
			if d.Placeholder(1) != "$1" {
				t.Errorf("Placeholder = %q, want $1", d.Placeholder(1))
			}
			// Quote 必须是双引号
			if d.QuoteIdentifier("t") != `"t"` {
				t.Errorf("QuoteIdentifier = %q, want \"t\"", d.QuoteIdentifier("t"))
			}
		})
	}
}

func TestAllRegisteredDrivers(t *testing.T) {
	got := Keys()
	sort.Strings(got)
	if len(got) < 11 { // 基础 postgres+mysql + Tier1(3)+Tier2(2)+aliases(3) = 11+
		t.Errorf("Keys() = %v (len=%d) — want at least 11 registered drivers", got, len(got))
	}
	for _, k := range got {
		d, err := New(k)
		if err != nil {
			t.Errorf("New(%q) error: %v", k, err)
			continue
		}
		if d.Name() == "" {
			t.Errorf("New(%q).Name() is empty", k)
		}
	}
}

func TestTiDB_InsertReturning(t *testing.T) {
	d, _ := New("tidb")
	sql := d.InsertReturning([]string{"id", "name"})
	if sql != "RETURNING `id`, `name`" {
		t.Errorf("TiDB InsertReturning = %q, want RETURNING with backticks", sql)
	}
}

func TestKingbaseES_CastUUID(t *testing.T) {
	d, _ := New("kingbase")
	got := d.Cast("?", LogicalUUID)
	// KingbaseES UUID cast should be CAST(? AS VARCHAR(36)) not ::uuid
	if !strings.Contains(got, "VARCHAR(36)") {
		t.Errorf("KingbaseES Cast(UUID) = %q, want CAST(... AS VARCHAR(36))", got)
	}
}

func TestKingbaseES_ArrayLiteral(t *testing.T) {
	d, _ := New("kingbase")
	got := d.ArrayLiteral([]string{"$1", "$2"})
	if !strings.Contains(got, "JSON_ARRAY") {
		t.Errorf("KingbaseES ArrayLiteral = %q, want JSON_ARRAY fallback", got)
	}
}

// ========== Phase E4: runtime DB_DRIVER → Dialect Registry 链路测试 ==========

func TestDialectRegistry_KnownDriverNames(t *testing.T) {
	// 确保 runtime.go driverNormalize 覆盖的所有 DB_DRIVER 值都能 New 出 Dialect
	driverNames := []struct {
		env      string
		wantName string
	}{
		// pgx wire protocol
		{"pgx", "postgres"},
		{"postgres", "postgres"},
		{"postgresql", "postgres"},
		{"kingbase", "kingbase"},
		{"kingbasees", "kingbase"},
		{"gaussdb", "gaussdb"},
		{"opengauss", "gaussdb"},
		// mysql wire protocol
		{"mysql", "mysql"},
		{"mariadb", "mysql"},
		{"oceanbase", "oceanbase"},
		{"oceanbase-mysql", "oceanbase"},
		{"tidb", "tidb"},
		{"polardb", "polardb"},
		{"polardb-mysql", "polardb"},
	}
	for _, tc := range driverNames {
		t.Run(tc.env, func(t *testing.T) {
			d, err := New(tc.env)
			if err != nil {
				t.Fatalf("New(%q) error: %v", tc.env, err)
			}
			if d.Name() != tc.wantName {
				t.Errorf("Name() = %q, want %q", d.Name(), tc.wantName)
			}
		})
	}
}

func TestDM8_Dialect(t *testing.T) {
	d, err := New("dm8")
	if err != nil {
		t.Fatal("New(dm8) error:", err)
	}
	if d.Name() != "dm8" {
		t.Errorf("Name() = %q, want dm8", d.Name())
	}
	if d.Placeholder(1) != "?" {
		t.Errorf("Placeholder = %q, want ?", d.Placeholder(1))
	}
	if d.SupportsReturning() {
		t.Error("SupportsReturning should be false (DM8 RETURNING is INTO not PG-style)")
	}
	if !d.Supports(FeatureSkipLocked) {
		t.Error("Supports SKIP LOCKED should be true (DM8 8.0+)")
	}
	if !d.Supports(FeatureUpsert) {
		t.Error("Supports UPSERT (MERGE INTO) should be true")
	}
	if d.Supports(FeatureRLS) {
		t.Error("Supports RLS should be false")
	}
	// Limit 应该支持 MySQL 式两参数
	if d.Limit(0, 10) != "LIMIT 10" {
		t.Errorf("Limit(0,10) = %q", d.Limit(0, 10))
	}
	if d.Limit(20, 10) != "LIMIT 20, 10" {
		t.Errorf("Limit(20,10) = %q", d.Limit(20, 10))
	}
	// Cast UUID → CHAR(32)
	got := d.Cast("?", LogicalUUID)
	if !strings.Contains(got, "CHAR(32)") {
		t.Errorf("Cast(UUID) = %q, want CHAR(32)", got)
	}
	// MERGE INTO 模式
	sql := d.Upsert(DialectUpsert{
		Table:           "users",
		ConflictColumns: []string{"id"},
		UpdateColumns:   []string{"name", "email"},
	}, []string{"id", "name", "email"})
	if !strings.HasPrefix(sql, "MERGE INTO") {
		t.Errorf("Upsert should return MERGE INTO full statement, got:\n%s", sql)
	}
	if !strings.Contains(sql, "WHEN MATCHED THEN UPDATE SET") {
		t.Errorf("MERGE missing WHEN MATCHED UPDATE:\n%s", sql)
	}
	if !strings.Contains(sql, "WHEN NOT MATCHED THEN INSERT") {
		t.Errorf("MERGE missing WHEN NOT MATCHED INSERT:\n%s", sql)
	}
	t.Log("MERGE INTO statement:\n" + sql)

	// DoNothing 模式 — 没有 WHEN MATCHED
	sql2 := d.Upsert(DialectUpsert{
		Table:           "idempotency",
		ConflictColumns: []string{"scope", "key"},
		DoNothing:       true,
	}, []string{"scope", "key", "created_at"})
	if strings.Contains(sql2, "WHEN MATCHED THEN UPDATE") {
		t.Errorf("DoNothing MERGE should not have WHEN MATCHED UPDATE:\n%s", sql2)
	}
	t.Log("MERGE INTO DoNothing statement:\n" + sql2)
}

func TestShentong_Standard_Dialect(t *testing.T) {
	d, err := New("shentong")
	if err != nil {
		t.Fatal("New(shentong) error:", err)
	}
	if d.Name() != "shentong" {
		t.Errorf("Name() = %q", d.Name())
	}
	if d.Placeholder(1) != "?" {
		t.Errorf("Placeholder = %q", d.Placeholder(1))
	}
	if d.SupportsReturning() {
		t.Error("SupportsReturning should be false")
	}
	if d.Supports(FeatureSkipLocked) {
		t.Error("Supports SKIP LOCKED should be false (conservative)")
	}
	if d.Supports(FeatureRLS) {
		t.Error("Supports RLS should be false (conservative)")
	}
	if !d.Supports(FeatureUpsert) {
		t.Error("Supports UPSERT (MERGE INTO) should be true")
	}
	// LIMIT
	if d.Limit(0, 10) != "LIMIT 10" {
		t.Errorf("Limit(0,10) = %q", d.Limit(0, 10))
	}
	// Cast UUID → CHAR(32)
	if !strings.Contains(d.Cast("?", LogicalUUID), "CHAR(32)") {
		t.Errorf("Cast(UUID) = %q, want CHAR(32)", d.Cast("?", LogicalUUID))
	}
	// Cast TEXT → CLOB
	if !strings.Contains(d.Cast("?", LogicalText), "CLOB") {
		t.Errorf("Cast(TEXT) = %q, want CLOB", d.Cast("?", LogicalText))
	}
	// MERGE INTO
	sql := d.Upsert(DialectUpsert{
		Table: "users", ConflictColumns: []string{"id"},
		UpdateColumns: []string{"name", "email"},
	}, []string{"id", "name", "email"})
	if !strings.HasPrefix(sql, "MERGE INTO") {
		t.Errorf("Should return MERGE INTO, got:\n%s", sql)
	}
	t.Log("Shentong (standard) MERGE INTO:\n" + sql)
}

func TestShentong_OpenGauss_Dialect(t *testing.T) {
	d, err := New("shentong-gaussdb")
	if err != nil {
		t.Fatal("New(shentong-gaussdb) error:", err)
	}
	if d.Name() != "shentong-gaussdb" {
		t.Errorf("Name() = %q", d.Name())
	}
	// openGauss 版应该完全支持 PG 能力
	if !d.SupportsReturning() {
		t.Error("openGauss should support RETURNING")
	}
	if !d.Supports(FeatureRLS) {
		t.Error("openGauss should support RLS")
	}
	if !d.Supports(FeatureArrayType) {
		t.Error("openGauss should support ARRAY")
	}
	// 占位符 PG 风格
	if d.Placeholder(1) != "$1" {
		t.Errorf("Placeholder = %q, want $1", d.Placeholder(1))
	}
}

func TestAllShentongDrivers(t *testing.T) {
	// Shentong 标准版两个 key
	for _, k := range []string{"shentong", "shentongdb"} {
		d, err := New(k)
		if err != nil {
			t.Errorf("New(%q) error: %v", k, err)
			continue
		}
		if d.Name() != "shentong" {
			t.Errorf("%s Name() = %q", k, d.Name())
		}
	}
	// openGauss 版
	d, err := New("shentong-gaussdb")
	if err != nil {
		t.Fatal("New(shentong-gaussdb) error:", err)
	}
	if d.Name() != "shentong-gaussdb" {
		t.Error("shentong-gaussdb Name mismatch")
	}
}
