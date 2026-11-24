package adapters

const (
	queryGetAdapterByID = `
SELECT id, organization_id, name, display_name, description, type, server, database, table_name, username, password,
	owner, use_same_db, database_type, host, port, db_user, db_password, database_name, policy, is_enabled,
	created_at, updated_at, revision
FROM "guliangiam".adapters WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListAdapters = `
SELECT id, organization_id, name, display_name, description, type, server, database, table_name, username, password,
	owner, use_same_db, database_type, host, port, db_user, db_password, database_name, policy, is_enabled,
	created_at, updated_at, revision
FROM "guliangiam".adapters
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
  AND ($4 = '' OR database_type = $4)
ORDER BY created_at DESC LIMIT $5 OFFSET $6`

	queryCountAdapters = `
SELECT COUNT(*) FROM "guliangiam".adapters
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR name ILIKE '%' || $2 || '%' OR display_name ILIKE '%' || $2 || '%')
  AND ($3::boolean IS NULL OR is_enabled = $3)
  AND ($4 = '' OR database_type = $4)`

	queryInsertAdapter = `
INSERT INTO "guliangiam".adapters (
	id, organization_id, name, display_name, description, type, server, database, table_name, username, password,
	owner, use_same_db, database_type, host, port, db_user, db_password, database_name, policy, is_enabled,
	created_at, updated_at, revision
) VALUES (
	$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11,
	$12, $13, $14, $15, $16, $17, $18, $19, $20, $21,
	$22, $23, $24
)`

	// queryUpdateAdapter — 乐观锁 + 租户谓词。organization_id 取自实体（$22），
	// 调用方必须先做 scoped GetByID，因此 0 行语义仍是 revision 冲突。
	queryUpdateAdapter = `
UPDATE "guliangiam".adapters
SET display_name=$1, description=$2, type=$3, server=$4, database=$5, table_name=$6, username=$7, password=$8,
    owner=$9, use_same_db=$10, database_type=$11, host=$12, port=$13, db_user=$14, db_password=$15, database_name=$16, policy=$17, is_enabled=$18,
    updated_at=$19, revision=revision+1
WHERE id=$21 AND revision=$20 AND organization_id=$22::uuid RETURNING revision`

	queryDeleteAdapter = `DELETE FROM "guliangiam".adapters WHERE id=$1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
