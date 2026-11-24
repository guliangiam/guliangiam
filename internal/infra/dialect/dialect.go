// Package dialect defines the SQL dialect abstraction for multi-database
// support. Every Dialect implements the same interface; repositories call
// Dialect methods to generate dialect-specific SQL fragments instead of
// hard-coding PostgreSQL or MySQL syntax directly.
//
// Design rules (ADR-023, FR-1):
//  1. Only Dialect implementations may contain dialect-specific SQL syntax
//     (ON CONFLICT, ON DUPLICATE KEY, RETURNING, SKIP LOCKED, $1::uuid, ...).
//  2. Repository SQL may only use the constrained subset documented in
//     docs/manual/database-design.md §3.3.1; everything else goes through Dialect.
//  3. New database support = one file + one Register call. No other file
//     should need changing.
package dialect

import (
	"errors"
	"fmt"
	"strings"
)

// Feature enumerates dialect capabilities for runtime detection.
// Use Supports(Feature) to gate dialect-specific code paths (fail-fast for
// required features, degrade gracefully for optional ones).
type Feature string

const (
	FeatureReturning       Feature = "returning"
	FeatureUpsert          Feature = "upsert"            // ON CONFLICT / ON DUPLICATE KEY
	FeatureUpsertDoNothing Feature = "upsert_do_nothing" // DO NOTHING / IGNORE
	FeatureSkipLocked      Feature = "skip_locked"       // FOR UPDATE SKIP LOCKED
	FeaturePartialIndex    Feature = "partial_index"     // WHERE clause on UNIQUE INDEX
	FeatureRLS             Feature = "rls"               // Row Level Security
	FeatureArrayType       Feature = "array_type"        // TEXT[] / UUID[]
	FeatureJSONType        Feature = "json_type"         // JSONB / JSON column type
	FeatureLimitOffset     Feature = "limit_offset"      // LIMIT n OFFSET m style
	FeatureLimitTwoArg     Feature = "limit_two_arg"     // LIMIT m, n style
	FeatureBooleanKeyword  Feature = "boolean_keyword"   // TRUE/FALSE vs 1/0
)

// DialectUpsert models a unified UPSERT request.
// The Dialect implementation translates this struct into the target SQL:
//   - Postgres: INSERT INTO <Table> (Cols...) VALUES (...) ON CONFLICT (<ConflictCols>) [DO NOTHING | DO UPDATE SET <UpdateCols>=EXCLUDED.<UpdateCols>...]
//   - MySQL:    INSERT INTO <Table> (Cols...) VALUES (...) ON DUPLICATE KEY UPDATE <UpdateCols>=VALUES(<UpdateCols>), ...
//
// DoNothing=true produces the "no-op on conflict" variant (Postgres DO NOTHING,
// MySQL INSERT IGNORE). When DoNothing=true, UpdateColumns is ignored.
type DialectUpsert struct {
	Table           string   // schema-qualified table, e.g. "guliangiam.users"
	ConflictColumns []string // columns that trigger conflict (Postgres uses these; MySQL uses PK/UK implicitly)
	UpdateColumns   []string // columns to update on conflict (ignored when DoNothing=true)
	DoNothing       bool     // true → no-op on conflict
}

// Dialect abstracts all database-specific SQL variations.
// Only Dialect implementations (not business code) may contain dialect-specific
// SQL syntax. Every method returns a dialect-appropriate SQL fragment.
type Dialect interface {
	// Name returns the dialect identifier ("postgres", "mysql", ...).
	Name() string

	// Placeholder returns the parameterized placeholder for ordinal position
	// (1-indexed). Postgres uses $1, $2, ...; MySQL uses ?.
	Placeholder(ordinal int) string

	// QuoteIdentifier wraps a table/column name.
	QuoteIdentifier(name string) string

	// CurrentTimestamp returns the current timestamp SQL expression.
	CurrentTimestamp() string

	// BooleanLiteral returns the boolean literal for value.
	BooleanLiteral(v bool) string

	// SupportsReturning is retained as the legacy single-feature probe.
	// Prefer Supports(FeatureReturning) for new code.
	SupportsReturning() bool

	// Supports returns true when the dialect has the given capability.
	// See Feature constants for the full list.
	Supports(feature Feature) bool

	// Upsert generates a full INSERT ... ON CONFLICT / ON DUPLICATE KEY statement
	// including the VALUES clause position marker. The caller provides the
	// column list and VALUES fragment; this method produces the conflict +
	// update/ignore tail.
	//
	// Example output:
	//   Postgres: "... VALUES (?, ?, ?) ON CONFLICT (org_id, slug) DO UPDATE SET slug=EXCLUDED.slug WHERE slug IS NOT NULL"
	//   MySQL:    "... VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE slug=VALUES(slug)"
	Upsert(u DialectUpsert, columns []string) string

	// InsertReturning generates the RETURNING clause (Postgres) or equivalent
	// (MySQL's LAST_INSERT_ID() is not a direct equivalent; MySQL implementations
	// return an empty string and the caller must handle identity retrieval).
	// For multi-row returns, call InsertReturning with all needed column names.
	InsertReturning(columns []string) string

	// Limit generates pagination SQL.
	//   Postgres/LimitOffset style: "LIMIT <limit> OFFSET <offset>"
	//   MySQL/LimitTwoArg style:    "LIMIT <offset>, <limit>"  (when offset > 0)
	// When limit <= 0 no LIMIT clause is generated.
	Limit(offset, limit int) string

	// ForUpdate appends "FOR UPDATE" (or dialect equivalent) for row locking.
	ForUpdate() string

	// ForUpdateSkipLocked appends "FOR UPDATE SKIP LOCKED".
	// Returns an empty string if dialect does not support this feature.
	ForUpdateSkipLocked() string

	// Cast produces a dialect-typed cast: expr CAST to targetType.
	//   Postgres: "$1::uuid"
	//   MySQL:    "CAST($1 AS CHAR)"  (type mapping handled by caller)
	//
	// targetType is the *logical* type keyword ("uuid", "text", "bigint", ...);
	// the Dialect maps it to the physical SQL type. Use LogicalType constants.
	Cast(expr, logicalType string) string

	// JSONLiteral wraps a JSON expression for the dialect.
	//   Postgres: expr (JSONB is a native type, no wrapper needed)
	//   MySQL:    JSON_EXTRACT(expr, '$')  or CAST(expr AS JSON) — simplified here
	JSONLiteral(expr string) string

	// ArrayLiteral constructs an array literal from individual items.
	//   Postgres: ARRAY[item1, item2, ...]
	//   MySQL:    JSON_ARRAY(item1, item2, ...)  (approximation; MySQL has no native array)
	ArrayLiteral(items []string) string
}

// LogicalType constants for Dialect.Cast — use these instead of physical SQL types.
// The Dialect maps each logical type to its dialect-specific physical type.
const (
	LogicalUUID      = "uuid"
	LogicalText      = "text"
	LogicalVarchar   = "varchar"
	LogicalBigInt    = "bigint"
	LogicalInteger   = "integer"
	LogicalBoolean   = "boolean"
	LogicalTimestamp = "timestamptz"
	LogicalJSON      = "json"
	LogicalBlob      = "blob"
)

// ---- registry ----

// driverRegistry is populated by init() in dialect-specific implementation files.
var driverRegistry = map[string]func() Dialect{}

// Register adds a Dialect factory to the global registry.
// Called by each dialect's implementation file (in init()).
// Panics on duplicate driver key — misconfiguration should fail fast.
func Register(driver string, factory func() Dialect) {
	if _, exists := driverRegistry[driver]; exists {
		panic(fmt.Sprintf("dialect.Register: duplicate driver %q", driver))
	}
	driverRegistry[driver] = factory
}

// New constructs a Dialect for the given driver identifier.
// Unknown drivers return an error — the caller decides whether to fail-fast
// (pass through a fatal log entry) or degrade gracefully.
func New(driver string) (Dialect, error) {
	if factory, ok := driverRegistry[driver]; ok {
		return factory(), nil
	}
	return nil, errors.New("dialect: unknown driver " + driver + " (registered: " + strings.Join(Keys(), ", ") + ")")
}

// Keys returns the list of registered drivers, useful for logging and
// testing that the expected dialects are wired up.
func Keys() []string {
	keys := make([]string, 0, len(driverRegistry))
	for k := range driverRegistry {
		keys = append(keys, k)
	}
	return keys
}

// Alias registrations — 为运行时 driver 名（DB_DRIVER env 值）注册 Dialect。
// 这些别名与 PostgresDialect{} / MySQLDialect{} 是同一个实现实例。
func init() {
	// Runtime 把 "pgx"/"postgres"/"postgresql" 都 normalize 为 "postgres"
	Register("pgx", func() Dialect { return PostgresDialect{} })
	Register("postgresql", func() Dialect { return PostgresDialect{} })
	// Runtime 把 "mariadb" normalize 为 "mysql"
	Register("mariadb", func() Dialect { return MySQLDialect{} })
}
