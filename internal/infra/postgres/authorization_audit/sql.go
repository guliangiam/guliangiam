package auditrepo

// SQL 常量 — 所有查询走 TenantFilterSystem ctx（RLS: is_super_admin() 才能读写）

const (
	// queryChainStateInit 保证全新库首条审计写入前链头存在；并发初始化不覆盖既有 hash。
	queryChainStateInit = `
		INSERT INTO guliangiam.authorization_chain_state (id, last_hash, updated_at)
		VALUES (1, NULL, now()) ON CONFLICT (id) DO NOTHING`

	// queryChainStateLock 锁单行 id=1 读 last_hash（哈希链事务 Step 1）
	queryChainStateLock = `
		SELECT last_hash FROM guliangiam.authorization_chain_state WHERE id = 1 FOR UPDATE`

	// queryInsertAuditRecord 写入一条审计记录（哈希链事务 Step 3）
	queryInsertAuditRecord = `
		INSERT INTO guliangiam.authorization_audit_records (
			id, actor_id, actor_type, source_tenant, target_tenant,
			resource_type, action, risk_level, scope_id, ticket_id,
			decision, reason, prev_hash, hash, trace_id, created_at
		) VALUES (
			gen_random_uuid(), $1, $2, $3::uuid, $4::uuid,
			$5, $6, $7, NULL, NULL,
			$8, $9, $10, $11::text, NULL, $12
		)`

	// queryUpdateChainState 更新链头 last_hash（哈希链事务 Step 4）
	queryUpdateChainState = `
		UPDATE guliangiam.authorization_chain_state
		SET last_hash = $1::text, updated_at = now()
		WHERE id = 1`

	// queryListForValidate 拉审计链用于验证完整性
	queryListForValidate = `
		SELECT actor_id, actor_type, source_tenant, target_tenant,
		       resource_type, action, risk_level, decision, reason,
		       prev_hash, hash, created_at
		FROM guliangiam.authorization_audit_records
		ORDER BY created_at DESC
		LIMIT $1`
)
