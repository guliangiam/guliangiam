package dialect

import "fmt"

// PostgresDialect is the primary database dialect for GuliangIAM.
// PostgreSQL 18+ supports all features Phase E1 targets.
type PostgresDialect struct{}

func init() { Register("postgres", func() Dialect { return PostgresDialect{} }) }

func (PostgresDialect) Name() string                       { return "postgres" }
func (PostgresDialect) Placeholder(ordinal int) string     { return fmt.Sprintf("$%d", ordinal) }
func (PostgresDialect) QuoteIdentifier(name string) string { return `"` + name + `"` }
func (PostgresDialect) CurrentTimestamp() string           { return "now()" }
func (PostgresDialect) SupportsReturning() bool            { return true }

func (PostgresDialect) BooleanLiteral(v bool) string {
	if v {
		return boolLiteralTrue
	}
	return boolLiteralFalse
}

// Supports reports capability for Postgres — true for every phase-E1 target.
func (PostgresDialect) Supports(feature Feature) bool {
	switch feature {
	case FeatureReturning, FeatureUpsert, FeatureUpsertDoNothing,
		FeatureSkipLocked, FeaturePartialIndex, FeatureRLS,
		FeatureArrayType, FeatureJSONType, FeatureLimitOffset,
		FeatureBooleanKeyword:
		return true
	case FeatureLimitTwoArg:
		return false // PG 只支持单参 LIMIT
	default:
		return false
	}
}

// Upsert emits ON CONFLICT (...) DO UPDATE/DO NOTHING.
func (PostgresDialect) Upsert(u DialectUpsert, columns []string) string {
	// Build the (col1, col2, ...) conflict column list
	if len(u.ConflictColumns) == 0 {
		return ""
	}
	conflict := ""
	for i, c := range u.ConflictColumns {
		if i > 0 {
			conflict += ", "
		}
		conflict += (PostgresDialect{}).QuoteIdentifier(c)
	}

	if u.DoNothing || len(u.UpdateColumns) == 0 {
		return fmt.Sprintf("ON CONFLICT (%s) DO NOTHING", conflict)
	}

	// DO UPDATE SET a=EXCLUDED.a, b=EXCLUDED.b, ...
	set := ""
	for i, c := range u.UpdateColumns {
		if i > 0 {
			set += ", "
		}
		set += fmt.Sprintf("%s=EXCLUDED.%s", (PostgresDialect{}).QuoteIdentifier(c), (PostgresDialect{}).QuoteIdentifier(c))
	}
	return fmt.Sprintf("ON CONFLICT (%s) DO UPDATE SET %s", conflict, set)
}

// InsertReturning produces "RETURNING col1, col2, ..." or "RETURNING *".
func (PostgresDialect) InsertReturning(columns []string) string {
	if len(columns) == 0 {
		return "RETURNING *"
	}
	out := "RETURNING "
	for i, c := range columns {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

// Limit emits "LIMIT n OFFSET m" (Postgres style).
func (PostgresDialect) Limit(offset, limit int) string {
	if limit <= 0 {
		return ""
	}
	if offset > 0 {
		return fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	}
	return fmt.Sprintf("LIMIT %d", limit)
}

func (PostgresDialect) ForUpdate() string           { return forUpdateClause }
func (PostgresDialect) ForUpdateSkipLocked() string { return forUpdateSkipLockedClause }

// Cast produces "expr::logicalType" — Postgres shorthand is canonical.
func (PostgresDialect) Cast(expr, logicalType string) string {
	return fmt.Sprintf("%s::%s", expr, logicalType)
}

func (PostgresDialect) JSONLiteral(expr string) string {
	// JSONB is native in Postgres; no wrapper needed.
	return expr
}

func (PostgresDialect) ArrayLiteral(items []string) string {
	out := "ARRAY["
	for i, it := range items {
		if i > 0 {
			out += ", "
		}
		out += it
	}
	out += "]"
	return out
}
