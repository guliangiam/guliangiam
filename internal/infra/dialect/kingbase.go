package dialect

// KingbaseESDialect implements the Dialect interface for KingbaseES (人大金仓).
// KingbaseES V8/V9 is PostgreSQL 9.6~12 compatible with added Oracle-mode extensions.
//
// Feature summary:
//
//	✅ RETURNING, UPSERT (ON CONFLICT), SKIP LOCKED, partial unique index — all PG-style
//	⚠️  RLS — KingbaseES has RLS but compatibility varies by version; marked false to force
//	         application-level TenantFilter as on MySQL until verified per-version.
//	❌  native ARRAY[] — KS has intarray extension but not bundled; fallback to JSON_ARRAY
//
// Driver: same as Postgres — github.com/jackc/pgx/v5.
type KingbaseESDialect struct{ PostgresDialect }

func init() { Register("kingbase", func() Dialect { return KingbaseESDialect{} }) }
func init() { Register("kingbasees", func() Dialect { return KingbaseESDialect{} }) }

func (KingbaseESDialect) Name() string { return "kingbase" }

func (KingbaseESDialect) Supports(feature Feature) bool {
	switch feature {
	case FeatureReturning, FeatureUpsert, FeatureUpsertDoNothing,
		FeatureSkipLocked, FeaturePartialIndex,
		FeatureLimitOffset, FeatureLimitTwoArg,
		FeatureJSONType, FeatureBooleanKeyword:
		return true
	// RLS: conservative false — wait per-version verification
	// ArrayType: no native ARRAY[] without ext
	case FeatureRLS, FeatureArrayType:
		return false
	default:
		return false
	}
}

// Cast: KingbaseES ::uuid may not be registered as a built-in cast in every image;
// fall back to a CAST form that always works.
func (KingbaseESDialect) Cast(expr, logicalType string) string {
	// For UUID, KingbaseES CHAR(36) is safer than ::uuid across images
	if logicalType == LogicalUUID {
		return "CAST(" + expr + " AS VARCHAR(36))"
	}
	return (PostgresDialect{}).Cast(expr, logicalType)
}

// ArrayLiteral: no native ARRAY[] — use JSON_ARRAY like MySQL-compat dialects.
func (KingbaseESDialect) ArrayLiteral(items []string) string {
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
