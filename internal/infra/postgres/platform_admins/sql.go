package platformadmins

// SQL 常量 — 所有查询走 TenantFilterSystem ctx（RLS: is_super_admin()）
// 不做 set_tenant_context（依赖 UOW.WithinTransaction 注入）

const (
	// platform_admins
	queryGetByID = `
		SELECT id, user_id, created_by, is_active, created_at, updated_at, revision
		FROM guliangiam.platform_admins WHERE id = $1`

	queryGetByUserID = `
		SELECT id, user_id, created_by, is_active, created_at, updated_at, revision
		FROM guliangiam.platform_admins WHERE user_id = $1`

	queryList = `
		SELECT id, user_id, created_by, is_active, created_at, updated_at, revision
		FROM guliangiam.platform_admins
		WHERE ($1::bool IS NULL OR is_active = $1)
		  AND ($2 = '' OR user_id::text LIKE '%' || $2 || '%')
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4`

	queryCount = `
		SELECT COUNT(*) FROM guliangiam.platform_admins
		WHERE ($1::bool IS NULL OR is_active = $1)
		  AND ($2 = '' OR user_id::text LIKE '%' || $2 || '%')`

	queryInsert = `
		INSERT INTO guliangiam.platform_admins
			(id, user_id, created_by, is_active, created_at, updated_at, revision)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	queryUpdate = `
		UPDATE guliangiam.platform_admins
		SET created_by = $1, is_active = $2, updated_at = $3, revision = revision + 1
		WHERE id = $4 AND revision = $5
		RETURNING revision`

	querySetActive = `
		UPDATE guliangiam.platform_admins SET is_active = $1, updated_at = now() WHERE id = $2`

	queryDelete = `
		DELETE FROM guliangiam.platform_admins WHERE id = $1`

	// platform_admin_scopes
	//
	// scopeValidity 是 scope 有效性的**唯一权威源**：
	//   未过期 && (is_bootstrap || approved_by 非空)
	// 列表查询（ResolveAdminScope 消费）与 Gate 7（HasActiveScopeForTenant）都必须
	// 拼接本常量，禁止各自另写判定。表别名统一为 s，常量才可直接复用。
	scopeValidity = `s.expires_at > now() AND (s.is_bootstrap OR s.approved_by IS NOT NULL)`

	queryGetScopeByID = `
		SELECT id, platform_admin_id, scope_type, target_tenants, approved_by, approved_at, is_bootstrap, expires_at, created_at
		FROM guliangiam.platform_admin_scopes WHERE id = $1`

	queryListScopesByAdmin = `
		SELECT id, platform_admin_id, scope_type, target_tenants, approved_by, approved_at, is_bootstrap, expires_at, created_at
		FROM guliangiam.platform_admin_scopes s
		WHERE s.platform_admin_id = $1
		  AND ($2 = false OR ` + scopeValidity + `)
		ORDER BY s.expires_at DESC`

	queryInsertScope = `
		INSERT INTO guliangiam.platform_admin_scopes
			(id, platform_admin_id, scope_type, target_tenants, approved_by, approved_at, is_bootstrap, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	queryApproveScope = `
		UPDATE guliangiam.platform_admin_scopes
		SET approved_by = $1, approved_at = now()
		WHERE id = $2 AND approved_by IS NULL`

	queryDeleteScope = `
		DELETE FROM guliangiam.platform_admin_scopes WHERE id = $1`

	queryHasActiveScopeForTenant = `
		SELECT EXISTS (
			SELECT 1 FROM guliangiam.platform_admin_scopes s
			WHERE s.platform_admin_id = $1
			  AND ` + scopeValidity + `
			  AND (
				  s.scope_type = 'all'
				  OR (s.scope_type = 'targeted' AND $2::uuid = ANY(s.target_tenants))
			  )
		)`
)
