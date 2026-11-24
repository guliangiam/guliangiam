package dialect

import "fmt"

// MySQLDialect implements the Dialect interface for MySQL 8.4 LTS+ / MariaDB 12.3+.
//
// Feature summary:
//
//	✅ UPSERT (ON DUPLICATE KEY UPDATE)
//	✅ INSERT IGNORE (DoNothing variant)
//	✅ FOR UPDATE (row lock)
//	✅ FOR UPDATE SKIP LOCKED (8.4 LTS+ / OceanBase / TiDB)
//	❌ RETURNING — caller must retrieve identity via LAST_INSERT_ID() separately
//	❌ RLS / partial unique index — application-level TenantFilter instead
//	❌ native array / JSONB — approximated with JSON_ARRAY / CAST(... AS JSON)
//
// LogicalType → MySQL physical type mapping is in Cast():
//
//	uuid       → CHAR(36)     (MySQL has no native UUID type; CHAR(36) stores UUIDv7 strings)
//	text       → LONGTEXT
//	bigint     → BIGINT
//	integer    → INT
//	timestamptz → DATETIME(6)  (MySQL has no timezone-aware timestamp column; application handles)
//	json       → JSON
//	blob       → LONGBLOB
type MySQLDialect struct{}

func init() { Register("mysql", func() Dialect { return MySQLDialect{} }) }

func (MySQLDialect) Name() string                       { return "mysql" }
func (MySQLDialect) Placeholder(int) string             { return "?" }
func (MySQLDialect) QuoteIdentifier(name string) string { return "`" + name + "`" }
func (MySQLDialect) CurrentTimestamp() string           { return "NOW(6)" }
func (MySQLDialect) SupportsReturning() bool            { return false }

func (MySQLDialect) BooleanLiteral(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (MySQLDialect) Supports(feature Feature) bool {
	switch feature {
	case FeatureUpsert, FeatureUpsertDoNothing,
		FeatureSkipLocked, FeatureLimitTwoArg, FeatureLimitOffset:
		return true
	case FeatureReturning, FeaturePartialIndex, FeatureRLS,
		FeatureArrayType, FeatureBooleanKeyword:
		return false
	case FeatureJSONType:
		return true // JSON column type supported
	default:
		return false
	}
}

// Upsert emits ON DUPLICATE KEY UPDATE ... / INSERT IGNORE.
// MySQL uses PK/UK implicitly — ConflictColumns are documented but not used in SQL.
func (MySQLDialect) Upsert(u DialectUpsert, columns []string) string {
	if u.DoNothing {
		// INSERT IGNORE silently skips duplicates. Table goes at caller site.
		return "INSERT IGNORE"
	}
	if len(u.UpdateColumns) == 0 {
		return ""
	}
	set := ""
	for i, c := range u.UpdateColumns {
		if i > 0 {
			set += ", "
		}
		quoted := (MySQLDialect{}).QuoteIdentifier(c)
		set += fmt.Sprintf("%s=VALUES(%s)", quoted, quoted)
	}
	return fmt.Sprintf("ON DUPLICATE KEY UPDATE %s", set)
}

// InsertReturning is empty for MySQL — caller must use LAST_INSERT_ID() after exec.
func (MySQLDialect) InsertReturning([]string) string { return "" }

// Limit emits MySQL's two-arg LIMIT: LIMIT offset, count (style 2 when offset > 0).
func (MySQLDialect) Limit(offset, limit int) string {
	if limit <= 0 {
		return ""
	}
	if offset > 0 {
		return fmt.Sprintf("LIMIT %d, %d", offset, limit)
	}
	return fmt.Sprintf("LIMIT %d", limit)
}

func (MySQLDialect) ForUpdate() string           { return forUpdateClause }
func (MySQLDialect) ForUpdateSkipLocked() string { return forUpdateSkipLockedClause }

// Cast uses CAST(expr AS <physical>) — maps LogicalType to MySQL physical type.
func (MySQLDialect) Cast(expr, logicalType string) string {
	phys := map[string]string{
		LogicalUUID:      "CHAR(36)",
		LogicalText:      "LONGTEXT",
		LogicalVarchar:   "CHAR(255)",
		LogicalBigInt:    "BIGINT",
		LogicalInteger:   physicalTypeINT,
		LogicalBoolean:   "TINYINT(1)",
		LogicalTimestamp: "DATETIME(6)",
		LogicalJSON:      "JSON",
		LogicalBlob:      "LONGBLOB",
	}
	t, ok := phys[logicalType]
	if !ok {
		t = logicalType
	}
	return fmt.Sprintf("CAST(%s AS %s)", expr, t)
}

// JSONLiteral: MySQL JSON type accepts raw strings as long as the column is
// JSON; no wrapper needed at SQL fragment level.
func (MySQLDialect) JSONLiteral(expr string) string { return expr }

// ArrayLiteral: MySQL has no native array. JSON_ARRAY is an approximation —
// caller is responsible for ensuring the target column is JSON-compatible.
func (MySQLDialect) ArrayLiteral(items []string) string {
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
