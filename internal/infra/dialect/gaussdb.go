package dialect

// GaussDBDialect implements the Dialect interface for Huawei GaussDB.
// GaussDB (now openGauss open-source fork) is the closest PostgreSQL-compatible
// commercial fork; it supports nearly every PG 12~14 feature including RLS.
//
// Feature summary:
//
//	✅ RETURNING, UPSERT (ON CONFLICT), SKIP LOCKED, partial unique index, RLS — all true
//	✅ native ARRAY[] — intarray-style constructs available
//
// Driver: same as Postgres — github.com/jackc/pgx/v5 (PG wire protocol).
type GaussDBDialect struct{ PostgresDialect }

func init() { Register("gaussdb", func() Dialect { return GaussDBDialect{} }) }
func init() { Register("opengauss", func() Dialect { return GaussDBDialect{} }) }

func (GaussDBDialect) Name() string { return "gaussdb" }

// GaussDB matches Postgres feature set almost exactly — defer to parent.
// Override only if feature gaps are discovered in real GaussDB 8.x testing.
