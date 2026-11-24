package dialect

// PolarDBDialect implements the Dialect interface for Alibaba Cloud PolarDB MySQL-compatible mode.
// PolarDB MySQL mode is a fully wire-compatible MySQL 8.0+ clone with Aurora-like storage.
//
// Known differences from MySQL 8.4 LTS:
//
//	❌ RETURNING not supported (use LAST_INSERT_ID())
//	✅ UPSERT (ON DUPLICATE KEY UPDATE / INSERT IGNORE)
//	✅ FOR UPDATE SKIP LOCKED
//	❌ RLS / partial unique index (same as MySQL)
//
// Driver: github.com/go-sql-driver/mysql (PolarDB MySQL mode is wire-compatible).
type PolarDBDialect struct{ MySQLDialect }

func init() { Register("polardb", func() Dialect { return PolarDBDialect{} }) }
func init() { Register("polardb-mysql", func() Dialect { return PolarDBDialect{} }) }

func (PolarDBDialect) Name() string { return "polardb" }
