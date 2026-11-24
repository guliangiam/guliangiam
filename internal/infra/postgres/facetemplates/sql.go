package facetemplates

// template_ref 保存绑定 (table, column, user_id) 的 v2 信封密文：人脸底图绝不
// 以明文落库。AAD 以 user_id（UNIQUE，一行一用户）为锚点而非行 id，使重采原位
// 更新（ON CONFLICT DO UPDATE）前后 AAD 保持稳定，密文可被同一绑定解封。
// SELECT 列顺序与 scanOne 的 Scan 参数严格一致。

const queryEnroll = `
INSERT INTO guliangiam.user_face_templates
  (id, organization_id, user_id, template_ref, provider, provider_ref,
   status, revoked_at, created_at, updated_at, revision)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6,
        'active', NULL, $7, $8, 1)
ON CONFLICT (user_id) DO UPDATE SET
  template_ref = EXCLUDED.template_ref,
  provider = EXCLUDED.provider,
  provider_ref = EXCLUDED.provider_ref,
  status = 'active',
  revoked_at = NULL,
  updated_at = EXCLUDED.updated_at,
  revision = guliangiam.user_face_templates.revision + 1
`

const queryGetActiveByUser = `
SELECT id, organization_id, user_id, template_ref, provider, provider_ref,
       status, revoked_at, created_at, updated_at
FROM guliangiam.user_face_templates
WHERE organization_id = $1::uuid AND user_id = $2::uuid AND status = 'active'
`

// revoked_at 是 timestamp（无时区）而 updated_at 是 timestamptz；若两者共用
// 同一个无类型参数 $3，PostgreSQL 在 PREPARE 阶段无法推断 $3 的类型
// （inconsistent types deduced: timestamp versus timestamptz），pgx 默认
// 预编译模式下整条语句直接 500。显式把 $3 锚定为 timestamptz，赋值给
// revoked_at 时由列类型隐式收窄，消除推断冲突。
const queryRevokeByUser = `
UPDATE guliangiam.user_face_templates
SET status = 'revoked', revoked_at = $3::timestamptz, updated_at = $3::timestamptz,
    revision = revision + 1
WHERE organization_id = $1::uuid AND user_id = $2::uuid AND status = 'active'
`
