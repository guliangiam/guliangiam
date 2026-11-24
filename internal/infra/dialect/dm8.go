package dialect

import "fmt"

// DM8Dialect implements the Dialect interface for Dameng DM8 (达梦数据库).
// DM8 is a domestic OLTP database with ORACLE-compatible core plus MySQL-compatible
// layer. Key behaviors vs Postgres/MySQL:
//
//	✅ LIMIT n / LIMIT offset,count / LIMIT n OFFSET offset  — MySQL-compatible
//	✅ FOR UPDATE SKIP LOCKED                                  — since DM8.0
//	✅ PLACEHOLDER ? / QuoteIdentifier "col"                   — hybrid
//	❌ RETURNING (into object, not PG-style SELECT return)
//	❌ ON CONFLICT / ON DUPLICATE KEY UPDATE                   — use MERGE INTO
//	❌ RLS / partial unique index / native array / JSONB
//	❌ INSERT IGNORE                                           — use MERGE WHEN NOT MATCHED THEN INSERT
//
// Upsert strategy: DM8 has neither ON CONFLICT (PG) nor ON DUPLICATE (MySQL).
// Every upsert must be rendered as MERGE INTO <t> USING (SELECT ? AS col FROM DUAL [UNION ALL ...])
// ON (<pk_col> = <pk_col>) WHEN MATCHED THEN UPDATE SET ... WHEN NOT MATCHED THEN INSERT (...) VALUES (...).
// The Upsert method returns the FULL MERGE statement (not a fragment) — callers must
// detect this (strings.HasPrefix("MERGE INTO")) and skip their INSERT INTO prefix.
//
// Driver: DM8 has a Go driver "github.com/godror/godror" (ORACLE wire) or a native DM driver
// "github.com/r3nic1e/go-dm8". We register under "dm8" — infra/dm8/ repository (future) picks driver.
type DM8Dialect struct{}

func init() {
	Register("dm8", func() Dialect { return DM8Dialect{} })
	Register("dameng", func() Dialect { return DM8Dialect{} })
}

func (DM8Dialect) Name() string { return "dm8" }

// DM8 uses Oracle-style ? placeholders (not :name, not $1).
func (DM8Dialect) Placeholder(ordinal int) string { return "?" }

// DM8 supports double-quote identifiers — same as Postgres.
func (DM8Dialect) QuoteIdentifier(name string) string { return `"` + name + `"` }

func (DM8Dialect) CurrentTimestamp() string { return "SYSDATE" }

func (DM8Dialect) BooleanLiteral(v bool) string {
	if v {
		return boolLiteralTrue
	}
	return boolLiteralFalse
}

// Supports reports DM8 capability.
func (DM8Dialect) Supports(feature Feature) bool {
	switch feature {
	case FeatureUpsert, FeatureUpsertDoNothing, // via MERGE INTO
		FeatureSkipLocked,
		FeatureLimitTwoArg, FeatureLimitOffset,
		FeatureBooleanKeyword:
		return true
	case FeatureReturning, // INTO object, not PG-style
		FeatureRLS, FeaturePartialIndex,
		FeatureArrayType, FeatureJSONType:
		return false
	default:
		return false
	}
}

// Upsert returns a full MERGE INTO statement for DM8.
//
// Unlike Postgres (ON CONFLICT ...) and MySQL (ON DUPLICATE KEY UPDATE ...) which
// return a clause fragment appended after INSERT INTO, DM8 has no upsert clause.
// The caller must detect "MERGE INTO" prefix and skip their INSERT prefix.
//
// DoNothing → MERGE INTO ... WHEN NOT MATCHED THEN INSERT (skip WHEN MATCHED branch).
func (DM8Dialect) Upsert(u DialectUpsert, columns []string) string {
	if u.Table == "" {
		return ""
	}
	t := DM8Dialect{}
	table := t.QuoteIdentifier(u.Table)

	if len(u.ConflictColumns) == 0 {
		// Fallback: assume first column in columns is the PK for MERGE ON condition.
		if len(columns) == 0 {
			return ""
		}
		u.ConflictColumns = []string{columns[0]}
	}

	// MERGE INTO "guliangiam.users" t USING (...) s ON (...)
	var sql string
	sql += fmt.Sprintf("MERGE INTO %s t USING (", table)

	// USING clause — SELECT col1 AS col1, col2 AS col2 FROM DUAL UNION ALL ...
	// We emit one placeholder per column; caller provides VALUES in same order.
	sql += "SELECT "
	for i, c := range columns {
		if i > 0 {
			sql += ", "
		}
		sql += fmt.Sprintf("? AS %s", t.QuoteIdentifier(c))
	}
	sql += " FROM DUAL) s ON ("

	// ON condition — all ConflictColumns
	for i, cc := range u.ConflictColumns {
		if i > 0 {
			sql += " AND "
		}
		sql += fmt.Sprintf("t.%s = s.%s", t.QuoteIdentifier(cc), t.QuoteIdentifier(cc))
	}
	sql += ")"

	if !u.DoNothing {
		// WHEN MATCHED THEN UPDATE SET t.col = s.col, ...
		sql += " WHEN MATCHED THEN UPDATE SET "
		for i, c := range u.UpdateColumns {
			if i > 0 {
				sql += ", "
			}
			sql += fmt.Sprintf("t.%s = s.%s", t.QuoteIdentifier(c), t.QuoteIdentifier(c))
		}
	}

	// WHEN NOT MATCHED THEN INSERT (...) VALUES (...)
	sql += " WHEN NOT MATCHED THEN INSERT ("
	for i, c := range columns {
		if i > 0 {
			sql += ", "
		}
		sql += t.QuoteIdentifier(c)
	}
	sql += ") VALUES ("
	for i, c := range columns {
		if i > 0 {
			sql += ", "
		}
		sql += fmt.Sprintf("s.%s", t.QuoteIdentifier(c))
	}
	sql += ")"

	return sql
}

// InsertReturning — DM8 RETURNING clause is INTO <var>, not a result set.
// Caller must use RETURNING ... INTO + oracle-style OUT parameters (godror driver).
// We return empty here to signal "not supported as a SELECT return clause".
func (DM8Dialect) InsertReturning([]string) string { return "" }

// Limit — DM8 fully supports MySQL-compatible LIMIT.
func (DM8Dialect) Limit(offset, limit int) string {
	if limit <= 0 {
		return ""
	}
	if offset > 0 {
		return fmt.Sprintf("LIMIT %d, %d", offset, limit)
	}
	return fmt.Sprintf("LIMIT %d", limit)
}

func (DM8Dialect) ForUpdate() string           { return forUpdateClause }
func (DM8Dialect) ForUpdateSkipLocked() string { return forUpdateSkipLockedClause }

// Cast — DM8 physical type mapping. No native UUID / JSON type.
func (DM8Dialect) Cast(expr, logicalType string) string {
	phys := map[string]string{
		LogicalUUID:      "CHAR(32)", // DM8 CHAR default uppercase storage
		LogicalText:      physicalTypeCLOB,
		LogicalVarchar:   "VARCHAR(255)",
		LogicalBigInt:    "BIGINT",
		LogicalInteger:   physicalTypeINT,
		LogicalBoolean:   physicalTypeINT, // DM8 has BIT but INT is more portable
		LogicalTimestamp: "TIMESTAMP",
		LogicalJSON:      physicalTypeCLOB, // no native JSON; store as text
		LogicalBlob:      "BLOB",
	}
	t, ok := phys[logicalType]
	if !ok {
		t = logicalType
	}
	return fmt.Sprintf("CAST(%s AS %s)", expr, t)
}

// JSONLiteral — DM8 has no JSON type; store as CLOB text.
func (DM8Dialect) JSONLiteral(expr string) string { return expr }

// ArrayLiteral — no native array; JSON_ARRAY approximation like MySQL.
func (DM8Dialect) ArrayLiteral(items []string) string {
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

func (DM8Dialect) SupportsReturning() bool { return false }
