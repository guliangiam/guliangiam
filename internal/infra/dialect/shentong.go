package dialect

import "fmt"

// ShentongDialect implements the Dialect interface for Shentong (神舟通用数据库).
//
// Shentong has TWO product lines — both Oracle-compatible at the SQL core:
//
//   1. 神通数据库（openGauss 版） → PostgresDialect embedding, registered as "shentong-gaussdb".
//      openGauss wire protocol + PG syntax — see ShentongGaussDialect below.
//
//   2. 神通数据库（标准版，自研内核）→ Independent Oracle-compatible implementation here.
//      Oracle wire protocol via ODBC/JDBC/OLEDB drivers.
//
// This file covers BOTH dialects — ShentongDialect (标准版) and ShentongGaussDialect (openGauss 版).
// ADR-023 Tier 3 expects Shentong to be independent; openGauss variant is a thin PG wrapper.
//
// —— 标准版 Gap 分析（基于 Oracle 兼容 + 迁移工具文档，未完全官方验证的地方标注 conservative default）——
//
// | Feature        | Shentong 标准版行为           | Gap Level | Source                    |
// |----------------|-------------------------------|-----------|---------------------------|
// | Placeholder    | ? (JDBC standard)             | OK        | Oracle JDBC 兼容          |
// | QuoteIdent     | "col" (double quotes)         | OK        | Oracle-style              |
// | UPSERT         | MERGE INTO (Oracle-style)     | OK        | Oracle 兼容核心语法       |
// | RETURNING      | INTO <out-var>                | Supports=false | Oracle INTO 非 PG-style  |
// | LIMIT          | LIMIT n / LIMIT offset,n      | OK ?      | 兼容层支持；保守 true     |
// | SKIP LOCKED    | FOR UPDATE NO WAIT / ?        | Supports=false | 保守 false，等版本验证 |
// | RLS            | ?                             | Supports=false | 保守 false             |
// | PartialIdx     | 不支持 → 生成列替代            | Supports=false | ADR-023 决策 6        |
// | JSON           | VARCHAR2/CLOB — 无原生 JSON   | JSONType=false | 保守 false             |
// | UUID           | CHAR(32) (Oracle CHAR default) | OK       | —                         |
// | TEXT           | CLOB / LONG                   | OK        | Oracle TEXT = LONG/CLOB   |
//
// Driver: Shentong Go drivers — ODBC + ODBC driver manager, or native stgo driver.
// We register "shentong" + "shentongdb" as driver keys.

// ──────────────────────────────────────────────────────────────────────
// ShentongDialect — 标准版（自研内核，Oracle 兼容）
// ──────────────────────────────────────────────────────────────────────

type ShentongDialect struct{}

func init() {
	Register("shentong", func() Dialect { return ShentongDialect{} })
	Register("shentongdb", func() Dialect { return ShentongDialect{} })
}

func (ShentongDialect) Name() string { return "shentong" }

func (ShentongDialect) Placeholder(ordinal int) string { return "?" }

// Oracle-style double-quote identifiers.
func (ShentongDialect) QuoteIdentifier(name string) string { return `"` + name + `"` }

func (ShentongDialect) CurrentTimestamp() string { return "SYSDATE" }

func (ShentongDialect) BooleanLiteral(v bool) string {
	if v {
		return boolLiteralTrue
	}
	return boolLiteralFalse
}

func (ShentongDialect) SupportsReturning() bool { return false }

func (ShentongDialect) Supports(feature Feature) bool {
	switch feature {
	case FeatureUpsert, FeatureUpsertDoNothing,
		FeatureLimitTwoArg, FeatureLimitOffset,
		FeatureBooleanKeyword:
		return true
	// 以下保守 false — 待 Shentong 各版本验证后调整
	case FeatureReturning, FeatureSkipLocked,
		FeatureRLS, FeaturePartialIndex,
		FeatureJSONType, FeatureArrayType:
		return false
	default:
		return false
	}
}

// Upsert — Oracle-style MERGE INTO.
// Shentong 标准版是 Oracle 兼容，UPSERT 统一 MERGE INTO 改写（同 DM8）。
// 返回完整 MERGE 语句（调用方需检测前缀跳过 INSERT 前缀）。
//
// 注：Shentong MERGE USING 子句 — Oracle 用 SELECT ... FROM DUAL，这里沿用 DM8 模板。
// 若 Shentong 不支持 FROM DUAL，调用方未来可改成 USING (SELECT 1 AS dummy)。
func (ShentongDialect) Upsert(u DialectUpsert, columns []string) string {
	if u.Table == "" {
		return ""
	}
	s := ShentongDialect{}
	table := s.QuoteIdentifier(u.Table)

	if len(u.ConflictColumns) == 0 {
		if len(columns) == 0 {
			return ""
		}
		u.ConflictColumns = []string{columns[0]}
	}

	var sql string
	sql += fmt.Sprintf("MERGE INTO %s t USING (", table)
	sql += "SELECT "
	for i, c := range columns {
		if i > 0 {
			sql += ", "
		}
		sql += fmt.Sprintf("? AS %s", s.QuoteIdentifier(c))
	}
	sql += " FROM DUAL) s ON ("
	for i, cc := range u.ConflictColumns {
		if i > 0 {
			sql += " AND "
		}
		sql += fmt.Sprintf("t.%s = s.%s", s.QuoteIdentifier(cc), s.QuoteIdentifier(cc))
	}
	sql += ")"
	if !u.DoNothing {
		sql += " WHEN MATCHED THEN UPDATE SET "
		for i, c := range u.UpdateColumns {
			if i > 0 {
				sql += ", "
			}
			sql += fmt.Sprintf("t.%s = s.%s", s.QuoteIdentifier(c), s.QuoteIdentifier(c))
		}
	}
	sql += " WHEN NOT MATCHED THEN INSERT ("
	for i, c := range columns {
		if i > 0 {
			sql += ", "
		}
		sql += s.QuoteIdentifier(c)
	}
	sql += ") VALUES ("
	for i, c := range columns {
		if i > 0 {
			sql += ", "
		}
		sql += fmt.Sprintf("s.%s", s.QuoteIdentifier(c))
	}
	sql += ")"
	return sql
}

func (ShentongDialect) InsertReturning([]string) string { return "" }

// LIMIT — 保守支持 MySQL-style LIMIT（Shentong 有 MySQL 兼容层）。
// 若实际部署版本不支持 LIMIT，改用 ROWNUM LIMIT 模式：
//
//	SELECT * FROM (SELECT ... FROM t WHERE ROWNUM <= 20) WHERE ROWNUM > 10
//
// 由调用方检测 feature flag 并降级——Upsert 链路走 LIMIT（同 DM8）。
func (ShentongDialect) Limit(offset, limit int) string {
	if limit <= 0 {
		return ""
	}
	if offset > 0 {
		return fmt.Sprintf("LIMIT %d, %d", offset, limit)
	}
	return fmt.Sprintf("LIMIT %d", limit)
}

func (ShentongDialect) ForUpdate() string { return forUpdateClause }

// SKIP LOCKED 保守 false — Shentong 标准版 Oracle 兼容层默认只支持 NOWAIT / WAIT n。
// 若某版本支持 FOR UPDATE SKIP LOCKED，调整 Supports(FeatureSkipLocked)=true 即可。
func (ShentongDialect) ForUpdateSkipLocked() string { return "" }

// Cast — Oracle physical type mapping (CHAR(32) / CLOB / NUMBER / ...).
// Shentong 标准版走 Oracle 物理类型（非 DM8 的 CHAR(32) vs CHAR(32) 一致）。
func (ShentongDialect) Cast(expr, logicalType string) string {
	phys := map[string]string{
		LogicalUUID:      "CHAR(32)", // Oracle 默认 CHAR(32) 无分隔符格式
		LogicalText:      physicalTypeCLOB,
		LogicalVarchar:   "VARCHAR2(255)",
		LogicalBigInt:    "NUMBER(19)", // Oracle NUMBER 无 INTEGER 别名的老版本
		LogicalInteger:   "NUMBER(10)",
		LogicalBoolean:   "NUMBER(1)",
		LogicalTimestamp: "TIMESTAMP",
		LogicalJSON:      physicalTypeCLOB, // 无原生 JSON
		LogicalBlob:      "BLOB",
	}
	t, ok := phys[logicalType]
	if !ok {
		t = logicalType
	}
	return fmt.Sprintf("CAST(%s AS %s)", expr, t)
}

func (ShentongDialect) JSONLiteral(expr string) string { return expr }

func (ShentongDialect) ArrayLiteral(items []string) string {
	out := jsonArrayFuncOpen
	for i, it := range items {
		if i > 0 {
			out += ", "
		}
		out += it
	}
	out += ")"
	return out
}

// ──────────────────────────────────────────────────────────────────────
// ShentongGaussDialect — openGauss 版（PG wire protocol）
// ──────────────────────────────────────────────────────────────────────
// openGauss 是 PG 12~14 的增强分支，Shentong openGauss 版完整继承 PG wire + PG SQL 语法。
// 和 GaussDB Dialect 完全同构——直接嵌入 PostgresDialect。

type ShentongGaussDialect struct{ PostgresDialect }

func init() {
	Register("shentong-gaussdb", func() Dialect { return ShentongGaussDialect{} })
}

func (ShentongGaussDialect) Name() string { return "shentong-gaussdb" }

// openGauss 版 Feature 矩阵 — 与 Postgres/GaussDB 一致
func (ShentongGaussDialect) Supports(feature Feature) bool {
	switch feature {
	case FeatureReturning, FeatureUpsert, FeatureUpsertDoNothing,
		FeatureSkipLocked, FeaturePartialIndex, FeatureRLS,
		FeatureArrayType, FeatureJSONType, FeatureLimitOffset,
		FeatureLimitTwoArg, FeatureBooleanKeyword:
		return true
	default:
		return false
	}
}
