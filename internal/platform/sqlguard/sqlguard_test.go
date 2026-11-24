package sqlguard

import (
	"os"
	"sync"
	"testing"
)

// resetMode 直接重置 modeOnce — internal test 特权访问 unexported。
func resetMode() {
	modeOnce = sync.Once{}
}

// ── ValidOrderByClause ────────────────────────────────────────────────────────

func TestValidOrderByClause_AcceptsWhitelistFormat(t *testing.T) {
	cases := []string{
		"ORDER BY created_at DESC, id ASC",
		"ORDER BY updated_at ASC",
		"ORDER BY a.b DESC, c.d ASC",
		"ORDER BY action DESC, resource_type ASC",
	}
	for _, c := range cases {
		if err := ValidOrderByClause(c); err != nil {
			t.Errorf("should accept %q but got %v", c, err)
		}
	}
}

func TestValidOrderByClause_RejectsInjectionKeyword(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	cases := []string{
		"ORDER BY id; DROP TABLE users",
		"ORDER BY id; DELETE FROM users",
		"ORDER BY id WHERE 1=1",
		"ORDER BY id UNION SELECT 1,2,3",
		"ORDER BY id-- OR 1=1",
		"ORDER BY id /* comment */",
		"ORDER BY id' OR '1'='1",
		"ORDER BY id || 'x'",
		"ORDER BY id BENCHMARK(1000000,1)",
		"ORDER BY id SLEEP(5)",
		"ORDER BY id PG_SLEEP(5)",
	}
	for _, c := range cases {
		if err := ValidOrderByClause(c); err == nil {
			t.Errorf("should reject %q", c)
		}
	}
}

func TestValidOrderByClause_RejectsBadFormat(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	cases := []string{
		"ORDER BY ",
		"ORDER BY created_at",     // 缺 ASC|DESC
		"ORDER BY created_at ZZZ", // 非法方向
		"created_at DESC",         // 缺 ORDER BY
	}
	for _, c := range cases {
		if err := ValidOrderByClause(c); err == nil {
			t.Errorf("should reject bad format %q", c)
		}
	}
}

// ── ValidColumnIdentifier ─────────────────────────────────────────────────────

func TestValidColumnIdentifier_AcceptsLegalNames(t *testing.T) {
	cases := []string{
		"username",
		"u.username",
		"physical_column",
		"guliangiam.users.id",
		"_private_col",
	}
	for _, c := range cases {
		if err := ValidColumnIdentifier(c); err != nil {
			t.Errorf("should accept %q but got %v", c, err)
		}
	}
}

func TestValidColumnIdentifier_RejectsInjection(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	cases := []string{
		"u.username WHERE 1=1",
		"u.username; DROP TABLE users",
		"u.username UNION SELECT",
		"u.username -- OR 1=1",
		"u.username'",
	}
	for _, c := range cases {
		if err := ValidColumnIdentifier(c); err == nil {
			t.Errorf("should reject %q", c)
		}
	}
}

func TestValidColumnIdentifier_RejectsBadFormat(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	cases := []string{
		"",
		"123col",   // 数字开头
		"col name", // 空格
		"col@name", // 非法字符
	}
	for _, c := range cases {
		if err := ValidColumnIdentifier(c); err == nil {
			t.Errorf("should reject bad format %q", c)
		}
	}
}

// ── ValidAndClause ────────────────────────────────────────────────────────────

func TestValidAndClause_AcceptsParamPlaceholder(t *testing.T) {
	cases := []string{
		" AND status = $5",
		" AND user_id = $2",
		" AND u.created_at = $7",
	}
	for _, c := range cases {
		if err := ValidAndClause(c); err != nil {
			t.Errorf("should accept %q but got %v", c, err)
		}
	}
}

func TestValidAndClause_RejectsStringLiteral(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	cases := []string{
		" AND status = 'admin'",
		" AND role = 'superuser' OR 1=1",
		" AND username = x'61646D696E'",
	}
	for _, c := range cases {
		if err := ValidAndClause(c); err == nil {
			t.Errorf("should reject %q", c)
		}
	}
}

func TestValidAndClause_RejectsBadFormat(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	cases := []string{
		"",
		"status = $5",     // 缺 AND
		" AND status = ?", // 不是 $N
		" AND status",     // 不全
	}
	for _, c := range cases {
		if err := ValidAndClause(c); err == nil {
			t.Errorf("should reject bad format %q", c)
		}
	}
}

// ── ValidINPlaceholders ───────────────────────────────────────────────────────

func TestValidINPlaceholders_AcceptsPlaceholderList(t *testing.T) {
	cases := []string{
		"$1",
		"$1,$2",
		"$1,$2,$3",
		"$10,$11,$12,$13",
	}
	for _, c := range cases {
		if err := ValidINPlaceholders(c); err != nil {
			t.Errorf("should accept %q but got %v", c, err)
		}
	}
}

func TestValidINPlaceholders_RejectsLiterals(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	cases := []string{
		"$1,'abc',$3",
		"'abc','def'",
		"$1,foo,$3",
		"1,2,3",
	}
	for _, c := range cases {
		if err := ValidINPlaceholders(c); err == nil {
			t.Errorf("should reject %q", c)
		}
	}
}

// ── Handle() 三种模式 ──────────────────────────────────────────────────────────

func TestHandle_PanicMode(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "panic")
	resetMode()

	defer func() {
		if r := recover(); r == nil {
			t.Error("Handle() should panic in panic mode")
		}
	}()

	Handle("test violation", "ORDER BY bad; DROP")
}

func TestHandle_LogMode_NoPanic(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "log")
	resetMode()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Handle() should NOT panic in log mode, got %v", r)
		}
	}()

	Handle("test violation", "ORDER BY bad; DROP")
}

func TestHandle_OffMode_NoOp(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Handle() should be no-op in off mode, got %v", r)
		}
	}()

	Handle("anything", "anything")
}

// ── 边界: 空输入 + 误伤检查 ─────────────────────────────────────────────────────

func TestValidOrderByClause_EmptyInput(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()
	if err := ValidOrderByClause(""); err == nil {
		t.Error("should reject empty ORDER BY")
	}
}

func TestValidINPlaceholders_EmptyInput(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()
	if err := ValidINPlaceholders(""); err == nil {
		t.Error("should reject empty IN placeholder")
	}
}

// 列名 "format" 不应该被当成 FORMAT 注入关键词误伤
func TestNoKeywordFalsePositive(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()
	if err := ValidColumnIdentifier("format"); err != nil {
		t.Errorf("column 'format' should be legal, got %v", err)
	}
}

// created_at 不能因为包含子串 CREATE 被误伤
// updated_at 不能因为包含子串 UPDATE 被误伤
func TestNoSubstringFalsePositive(t *testing.T) {
	os.Setenv("GULIANGIAM_SQL_GUARD_MODE", "off")
	resetMode()

	if err := ValidColumnIdentifier("created_at"); err != nil {
		t.Errorf("column 'created_at' should be legal (not contain 'CREATE '), got %v", err)
	}
	if err := ValidColumnIdentifier("updated_at"); err != nil {
		t.Errorf("column 'updated_at' should be legal (not contain 'UPDATE '), got %v", err)
	}
	if err := ValidColumnIdentifier("execution_logs"); err != nil {
		t.Errorf("column 'execution_logs' should be legal (not contain 'EXEC('), got %v", err)
	}
	if err := ValidColumnIdentifier("order_execution"); err != nil {
		t.Errorf("column 'order_execution' should be legal, got %v", err)
	}
	if err := ValidAndClause(" AND created_at = $3"); err != nil {
		t.Errorf("AND clause with created_at should be legal, got %v", err)
	}
	if err := ValidOrderByClause("ORDER BY created_at DESC, id ASC"); err != nil {
		t.Errorf("ORDER BY with created_at should be legal, got %v", err)
	}
}
