package crosstenantops

// SQL 常量 — 所有查询走 TenantFilterSystem ctx（RLS: is_super_admin()）

const (
	// HasApprovedTicket — AllowIfScoped ScopeAuthorizer Gate 2/8 热路径
	// actor_id 语义：users 表的 id（V5 schema 注释）
	queryHasApprovedTicket = `
		SELECT EXISTS (
			SELECT 1 FROM guliangiam.cross_tenant_operations t
			WHERE t.actor_id = $1
			  AND ($2::uuid IS NULL OR t.target_tenant = $2::uuid)
			  AND ($3::text = '' OR t.resource_type = $3)
			  AND ($4::text = '' OR t.action = $4)
			  AND t.status = 'approved'
			  AND t.expires_at > now()
		)`

	queryGetByID = `
		SELECT id, scope_id, actor_id, source_tenant, target_tenant,
		       resource_type, action, risk_level, status,
		       approved_by, approved_at, expires_at, used_at,
		       trace_id, reason, created_at
		FROM guliangiam.cross_tenant_operations WHERE id = $1`

	queryInsert = `
		INSERT INTO guliangiam.cross_tenant_operations
			(id, scope_id, actor_id, source_tenant, target_tenant,
			 resource_type, action, risk_level, status,
			 approved_by, approved_at, expires_at, used_at,
			 trace_id, reason, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	queryApprove = `
		UPDATE guliangiam.cross_tenant_operations
		SET status = 'approved', approved_by = $1, approved_at = now()
		WHERE id = $2 AND status = 'pending'`

	queryReject = `
		UPDATE guliangiam.cross_tenant_operations
		SET status = 'rejected', reason = $1
		WHERE id = $2 AND status = 'pending'`

	queryMarkUsed = `
		UPDATE guliangiam.cross_tenant_operations
		SET status = 'used', used_at = now()
		WHERE id = $1 AND status = 'approved'`

	queryListByActor = `
		SELECT id, scope_id, actor_id, source_tenant, target_tenant,
		       resource_type, action, risk_level, status,
		       approved_by, approved_at, expires_at, used_at,
		       trace_id, reason, created_at
		FROM guliangiam.cross_tenant_operations
		WHERE actor_id = $1
		  AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC`

	queryListByScope = `
		SELECT id, scope_id, actor_id, source_tenant, target_tenant,
		       resource_type, action, risk_level, status,
		       approved_by, approved_at, expires_at, used_at,
		       trace_id, reason, created_at
		FROM guliangiam.cross_tenant_operations
		WHERE scope_id = $1
		ORDER BY created_at DESC`
)
