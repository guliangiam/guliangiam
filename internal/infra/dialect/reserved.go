package dialect

// Placeholder for future dialects (SQLite / MSSQL / KingbaseES / GaussDB / DM / OceanBase / TiDB / PolarDB).
// Phase E1 only wires up postgres + mysql. New dialects just need to:
//   1. Implement the Dialect interface (dialect.go)
//   2. Call Register("driver", factory) in init()
// No other file changes required.
