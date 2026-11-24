package dialect

// TiDBDialect implements the Dialect interface for PingCAP TiDB.
// TiDB is wire-compatible with MySQL 8.0+ and supports RETURNING since TiDB 5.4.
//
// Known differences from MySQL 8.4 LTS:
//
//	✅ RETURNING clause supported (TiDB 5.4+)
//	✅ UPSERT (ON DUPLICATE KEY UPDATE / INSERT IGNORE)
//	✅ FOR UPDATE SKIP LOCKED
//	❌ RLS / partial unique index (same as MySQL)
//
// Driver: github.com/go-sql-driver/mysql (TiDB is wire-compatible).
type TiDBDialect struct{ MySQLDialect }

func init() { Register("tidb", func() Dialect { return TiDBDialect{} }) }

func (TiDBDialect) Name() string { return "tidb" }

// TiDB supports RETURNING since 5.4 — override Supports and SupportsReturning.
func (TiDBDialect) SupportsReturning() bool { return true }

func (TiDBDialect) Supports(feature Feature) bool {
	switch feature {
	case FeatureReturning, FeatureUpsert, FeatureUpsertDoNothing,
		FeatureSkipLocked, FeatureLimitTwoArg, FeatureLimitOffset,
		FeatureJSONType:
		return true
	case FeaturePartialIndex, FeatureRLS, FeatureArrayType, FeatureBooleanKeyword:
		return false
	default:
		return false
	}
}

// TiDB emits RETURNING as MySQL-compatible RETURNING — same semantics as Postgres
// but TiDB accepts the MySQL wire protocol variant.
func (TiDBDialect) InsertReturning(cols []string) string {
	if len(cols) == 0 {
		return ""
	}
	quoted := ""
	for i, c := range cols {
		if i > 0 {
			quoted += ", "
		}
		quoted += (TiDBDialect{}).QuoteIdentifier(c)
	}
	return "RETURNING " + quoted
}
