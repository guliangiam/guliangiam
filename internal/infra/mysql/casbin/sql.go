package casbin

// casbin_rules 的读写语句。表名与列名与 db/mysql/V1 生成 DDL 一致（无 schema 前缀）。
const (
	qLoadPolicy = "SELECT ptype, v0, v1, v2, v3, v4, v5 FROM `casbin_rules` " +
		"WHERE enforcer_id = ? AND (v5 IS NULL OR v5 NOT LIKE '%\"source\":\"iga_jit\"%') " +
		"ORDER BY created_at, id"

	qInsertRule = "INSERT INTO `casbin_rules` (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5) " +
		"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"

	qDeleteRule = "DELETE FROM `casbin_rules` WHERE enforcer_id = ? AND ptype = ? AND v0 = ? AND v1 = ? " +
		"AND (? = '' OR v2 = ?) AND (? = '' OR v3 = ?) AND (? = '' OR v4 = ?) AND (? = '' OR v5 = ?)"

	qDeleteGroupingsForUser = "DELETE FROM `casbin_rules` WHERE enforcer_id = ? AND ptype = 'g' AND v0 = ?"

	// qDeleteAllRules 用于 SavePolicy 的全量重写。
	qDeleteAllRules = "DELETE FROM `casbin_rules` WHERE enforcer_id = ?"

	// qDeleteFilteredBase 是 RemoveFilteredPolicy 的固定前缀，随后按字段拼条件。
	qDeleteFilteredBase = "DELETE FROM `casbin_rules` WHERE enforcer_id = ? AND ptype = ?"

	// 唯一键是 (enforcer_id, ptype, v0..v5) 的前缀索引；重复插入保持原行。
	qInsertGrouping = "INSERT INTO `casbin_rules` (id, enforcer_id, ptype, v0, v1, v2) " +
		"VALUES (?, ?, 'g', ?, ?, ?) ON DUPLICATE KEY UPDATE id = id"

	qSelectJITCandidates = "SELECT v0, v1, v2, v3, v4, v5 FROM `casbin_rules` " +
		"WHERE enforcer_id = ? AND ptype = 'p' AND v0 = ? AND v1 = ? " +
		"AND (v2 = '*' OR v2 = ?) AND (v3 = '*' OR v3 = ?) " +
		"AND v5 LIKE '%\"source\":\"iga_jit\"%'"

	qResolveEnforcer = "SELECT id FROM `enforcers` WHERE organization_id = ? AND is_enabled = 1 " +
		"ORDER BY created_at, id LIMIT 1"
)
