// Package sqlguard 提供 runtime 级 SQL 注入防御校验。
//
// 设计意图：registry 登记的 L2/L3 动态 SQL 点（ORDER BY whitelist、
// extra 拼接、IN 占位符等）虽然当前是安全的，但未来维护者可能绕过
// whitelist / 内部常量路径，把 DTO 字段直接拼进 SQL 模板。
//
// 校验函数放在动态片段的**拼接出口**，如果返回值不满足合法格式，
// 根据 GULIANGIAM_SQL_GUARD_MODE：
//
//	panic  — 生产严格模式，直接终止请求
//	log    — 默认，只记 warning 日志，观察期用
//	off    — 跳过校验
//
// 与 P2 AST 工具互补：AST 是编译时门禁，这里是运行时兜底。
package sqlguard

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"

	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
)

// Mode 定义 guard 的行为。
type Mode string

const (
	ModePanic Mode = "panic"
	ModeLog   Mode = "log"
	ModeOff   Mode = "off"
)

// 当前模式（首次访问时从 env 读取一次）。
var (
	currentMode Mode
	modeOnce    sync.Once
)

func mode() Mode {
	modeOnce.Do(func() {
		switch platformconfig.SQLGuardMode() {
		case "panic":
			currentMode = ModePanic
		case "off":
			currentMode = ModeOff
		default:
			currentMode = ModeLog
		}
	})
	return currentMode
}

// Handle 按模式处理校验失败：panic / 记日志 / 什么都不做。
// 所有校验函数内部统一调用它，避免在每个调用点重复写 if-mode 分支。
func Handle(violation, clause string) {
	switch mode() {
	case ModePanic:
		panic(fmt.Sprintf("sqlguard violation: %s (clause=%q)", violation, clause))
	case ModeLog:
		slog.Warn("sqlguard violation",
			"violation", violation,
			"clause", clause,
		)
	case ModeOff:
		// 显式关闭：不做任何处理
	}
}

// ── 注入关键词黑名单 ──────────────────────────────────────────────────────────

// dangerousTokens 是 SQL 注入常见的终止符、注释标记、高阶 SQL 关键词。
//
// 重要：每个"词型"关键词必须带边界后缀（空格或 (），避免 substring 误伤
// 合法列名——例如 created_at 里的 "CREATE"、updated_at 里的 "UPDATE"、
// execution 里的 "EXEC"。
var dangerousTokens = []string{
	// SQL DDL / DML 关键词（带空格或 ( 边界）
	"SELECT ", "INSERT ", "UPDATE ", "DELETE ", "DROP ", "CREATE ", "ALTER ",
	"UNION ",
	// 执行类（带 ( 边界）
	"EXEC(", "EXECUTE(", "OPENROWSET(", "INTO OUTFILE ", "LOAD_FILE(",
	"BENCHMARK(", "SLEEP(", "PG_SLEEP(", "GEN_RANDOM_UUID(",
	// 逻辑短路
	" OR 1=", " AND 1=", "1=1", "1=2",
	// 注释 / 终止符
	"--", "/*", "*/", ";", "'", "||", "'||",
	// SELECT 终止符
	" WHERE ",
}

func hasDangerousToken(s string) (string, bool) {
	upper := strings.ToUpper(s)
	for _, tok := range dangerousTokens {
		if strings.Contains(upper, tok) {
			return tok, true
		}
	}
	return "", false
}

// ── 4 个校验函数 ──────────────────────────────────────────────────────────────

// ValidOrderByClause ORDER BY 子句必须是 whitelist 列 + ASC|DESC + tiebreaker 的封闭格式。
//
// 合法: "ORDER BY created_at DESC, id ASC"
// 非法: "ORDER BY id; DROP TABLE users"（注入关键词）
func ValidOrderByClause(clause string) error {
	if tok, bad := hasDangerousToken(clause); bad {
		Handle("ORDER BY clause contains dangerous token: "+tok, clause)
		return fmt.Errorf("ORDER BY clause rejected: dangerous token %q", tok)
	}
	if !orderByFormatRe.MatchString(strings.TrimSpace(clause)) {
		Handle("ORDER BY clause violates expected format", clause)
		return fmt.Errorf("ORDER BY clause rejected: unexpected format")
	}
	return nil
}

// ValidColumnIdentifier 动态列名（whitelist map 的 value）必须是合法标识符。
//
// 合法: "u.username", "physical_column"
// 非法: "u.username WHERE 1=1"
func ValidColumnIdentifier(col string) error {
	if tok, bad := hasDangerousToken(col); bad {
		Handle("column identifier contains dangerous token: "+tok, col)
		return fmt.Errorf("column identifier rejected: dangerous token %q", tok)
	}
	if !columnIdentRe.MatchString(strings.TrimSpace(col)) {
		Handle("column identifier violates format", col)
		return fmt.Errorf("column identifier rejected: unexpected format")
	}
	return nil
}

// ValidAndClause extra 里追加的 "AND col = $N" 必须是硬编码占位符模式。
//
// 合法: " AND status = $5"
// 非法: " AND status = 'admin'"（字符串字面量直接拼进来）
func ValidAndClause(clause string) error {
	if tok, bad := hasDangerousToken(clause); bad {
		// $1/$2/$3... 里的 $ 不应该被 hasDangerousToken 命中
		Handle("AND clause contains dangerous token: "+tok, clause)
		return fmt.Errorf("AND clause rejected: dangerous token %q", tok)
	}
	if !andClauseRe.MatchString(strings.TrimSpace(clause)) {
		Handle("AND clause violates expected format", clause)
		return fmt.Errorf("AND clause rejected: unexpected format")
	}
	return nil
}

// ValidINPlaceholders IN (...) 占位符部分必须是 $1,$2,$3... 的纯占位符列表。
//
// 合法: "$1,$2,$3"
// 非法: "$1,'abc',$3"
func ValidINPlaceholders(ph string) error {
	trimmed := strings.TrimSpace(ph)
	if !inPlaceholderRe.MatchString(trimmed) {
		Handle("IN placeholder list violates expected format", ph)
		return fmt.Errorf("IN placeholder list rejected: unexpected format")
	}
	return nil
}

// ── compiled regexes ──────────────────────────────────────────────────────────

// ORDER BY 格式: 一个或多个 "<ident> (ASC|DESC)" 片段用 ", " 连接
var orderByFormatRe = regexp.MustCompile(
	`^ORDER BY \w+(\.\w+)* (?:ASC|DESC)(?:, \w+(\.\w+)* (?:ASC|DESC))*$`,
)

// 列标识符: [a-zA-Z_][a-zA-Z0-9_.]* 允许点号限定（如 u.username）
var columnIdentRe = regexp.MustCompile(
	`^[a-zA-Z_][a-zA-Z0-9_.]*$`,
)

// AND 条件片段: " AND <ident> = $<num>"
var andClauseRe = regexp.MustCompile(
	`^AND [a-zA-Z_][a-zA-Z0-9_.]* = \$\d+$`,
)

// IN 占位符: $1 或 $1,$2 或 $1,$2,$3 ...
var inPlaceholderRe = regexp.MustCompile(
	`^\$\d+(?:,\$\d+)*$`,
)

// INTERNAL TEST ONLY: 包内测试可以直接重置 modeOnce 切换模式。
// 生产代码不应调 sync.Once 重置。
