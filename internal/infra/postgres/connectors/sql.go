package connectors

// guliangiam.connectors — SCIM 出站连接器配置。
//
// 所有语句都带显式 organization_id 谓词；RLS 是第二道防线，不是唯一防线。
// bearer_token_sealed 只接受信封密文，明文由仓储在内存中封/解。
const (
	queryConnectorGet = `
SELECT id, organization_id, name, scim_base_url, bearer_token_sealed, is_enabled,
       timeout_ms, max_rate_per_min, revision, created_at, updated_at
FROM guliangiam.connectors
WHERE id = $1::uuid AND organization_id = $2::uuid`

	queryConnectorList = `
SELECT id, organization_id, name, scim_base_url, bearer_token_sealed, is_enabled,
       timeout_ms, max_rate_per_min, revision, created_at, updated_at
FROM guliangiam.connectors
WHERE organization_id = $1::uuid
ORDER BY name ASC, id ASC`

	queryConnectorInsert = `
INSERT INTO guliangiam.connectors (
	id, organization_id, name, scim_base_url, bearer_token_sealed, is_enabled,
	timeout_ms, max_rate_per_min, revision, created_at, updated_at
) VALUES (
	$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11
)`

	// revision 由调用方读到的当前版本作为乐观锁守卫，写路径自增 1。
	queryConnectorUpdate = `
UPDATE guliangiam.connectors
SET name = $1,
    scim_base_url = $2,
    bearer_token_sealed = $3,
    is_enabled = $4,
    timeout_ms = $5,
    max_rate_per_min = $6,
    updated_at = $7,
    revision = revision + 1
WHERE id = $8::uuid
  AND organization_id = $9::uuid
  AND revision = $10
RETURNING revision`

	queryConnectorDelete = `
DELETE FROM guliangiam.connectors
WHERE id = $1::uuid AND organization_id = $2::uuid`
)
