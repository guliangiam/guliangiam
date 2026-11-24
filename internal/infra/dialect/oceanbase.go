package dialect

// OceanBaseDialect implements the Dialect interface for OceanBase in MySQL-compatible mode.
// OceanBase MySQL mode speaks MySQL wire protocol and supports nearly all MySQL 8.x syntax.
//
// Known differences from MySQL 8.4 LTS:
//
//	❌ RETURNING clause not supported (use LAST_INSERT_ID())
//	✅ UPSERT (ON DUPLICATE KEY UPDATE / INSERT IGNORE)
//	✅ FOR UPDATE SKIP LOCKED
//	❌ RLS / partial unique index (same as MySQL)
//
// Driver: github.com/go-sql-driver/mysql (OceanBase MySQL mode is wire-compatible).
type OceanBaseDialect struct{ MySQLDialect }

func init() { Register("oceanbase", func() Dialect { return OceanBaseDialect{} }) }

func (OceanBaseDialect) Name() string { return "oceanbase" }

// TiDB is the reference driver name OceanBase deployments often advertise.
// Register both aliases so DB_DRIVER accepts either.
func init() { Register("oceanbase-mysql", func() Dialect { return OceanBaseDialect{} }) }
