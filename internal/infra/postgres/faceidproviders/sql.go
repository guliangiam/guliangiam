package faceidproviders

const (
	// access_key_secret 沿用历史列名，但库内保存的是绑定 (table, column, id)
	// 的 v2 信封密文（A-6.1）；空串表示“未配置”，仓储在写入前用 EncryptColumn
	// 封装，读取时用 DecryptEnvelopeColumn 解封。
	// queryGetFaceIdProviderByID — 占位符：id(1)、organization_id(2)。
	queryGetFaceIdProviderByID = `
SELECT id, organization_id, name, provider_type, access_key_id, access_key_secret, endpoint, config, is_enabled, created_at, updated_at, revision
FROM guliangiam.face_id_providers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListFaceIdProviders = `
SELECT id, organization_id, name, provider_type, access_key_id, access_key_secret, endpoint, config, is_enabled, created_at, updated_at, revision
FROM guliangiam.face_id_providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2::boolean IS NULL OR is_enabled = $2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4`

	queryCountFaceIdProviders = `
SELECT COUNT(*)
FROM guliangiam.face_id_providers
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2::boolean IS NULL OR is_enabled = $2)`

	queryInsertFaceIdProvider = `
INSERT INTO guliangiam.face_id_providers (id, organization_id, name, provider_type, access_key_id, access_key_secret, endpoint, config, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, $12)`

	// queryUpdateFaceIdProvider — SET 8 + revision 守卫 + id + organization_id = $11。
	// 调用方（app 层）已先按租户读取实体并执行 Revision++，因此返回 0 行只可能
	// 是同一租户内的 revision 冲突；跨租户 id 在读取阶段就已 NotFound。
	queryUpdateFaceIdProvider = `
UPDATE guliangiam.face_id_providers
SET name = $1,
    provider_type = $2,
    access_key_id = $3,
    access_key_secret = $4,
    endpoint = $5,
    config = $6::jsonb,
    is_enabled = $7,
    updated_at = $8,
    revision = revision + 1
WHERE id = $10::uuid AND revision = $9 AND organization_id = guliangiam.resolve_org_id($11) RETURNING revision`

	// queryDeleteFaceIdProvider — 占位符：id(1)、organization_id(2)。
	queryDeleteFaceIdProvider = `
DELETE FROM guliangiam.face_id_providers
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
