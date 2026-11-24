package idv

// session_token 沿用历史列名，但库内保存的是绑定 (table, column, id) 的 v2
// 信封密文（A-6.2）；NULL/空串表示“未配置”，仓储在 INSERT 前封装、读取时解封。
// UPDATE 不写 session_token，读到明文后不会回写，因此不会覆盖信封。
//
// id_number/raw_result 保留明文列作为升级窗口的回退源；同表的
// id_number_sealed/raw_result_sealed 由仓储双写为 v2 信封（A-6.3C-1），
// 读取优先用 sealed。SELECT 顺序与 scanOne 的 Scan 参数严格一致。

// ─── guliangiam.identity_verifications ─────────────────────────────────────

const queryGetByID = `
SELECT id, organization_id, user_id, provider, status,
       redirect_url, session_token, provider_ref,
       biz_seq, action, product_code, subject_ref,
       first_name, last_name, date_of_birth, nationality,
       id_type, id_number, id_number_sealed, confidence, reason,
       raw_result, raw_result_sealed, initiated_at, completed_at, created_at, updated_at
FROM guliangiam.identity_verifications
WHERE id = $1::uuid AND organization_id = $2::uuid
`

const queryGetByProviderRef = `
SELECT id, organization_id, user_id, provider, status,
       redirect_url, session_token, provider_ref,
       biz_seq, action, product_code, subject_ref,
       first_name, last_name, date_of_birth, nationality,
       id_type, id_number, id_number_sealed, confidence, reason,
       raw_result, raw_result_sealed, initiated_at, completed_at, created_at, updated_at
FROM guliangiam.identity_verifications
WHERE provider = $1 AND provider_ref = $2 AND organization_id IS NOT NULL
`

const queryList = `
SELECT id, organization_id, user_id, provider, status,
       redirect_url, session_token, provider_ref,
       biz_seq, action, product_code, subject_ref,
       first_name, last_name, date_of_birth, nationality,
       id_type, id_number, id_number_sealed, confidence, reason,
       raw_result, raw_result_sealed, initiated_at, completed_at, created_at, updated_at
FROM guliangiam.identity_verifications
WHERE organization_id = $1::uuid
  AND ($2 = '' OR user_id = $2::uuid)
  AND ($3 = '' OR provider = $3)
  AND ($4::text IS NULL OR status = $4)
ORDER BY initiated_at DESC
LIMIT $5 OFFSET $6
`

const queryCount = `
SELECT count(*) FROM guliangiam.identity_verifications
WHERE organization_id = $1::uuid
  AND ($2 = '' OR user_id = $2::uuid)
  AND ($3 = '' OR provider = $3)
  AND ($4::text IS NULL OR status = $4)
`

const queryInsert = `
INSERT INTO guliangiam.identity_verifications
  (id, organization_id, user_id, provider, status,
   redirect_url, session_token, provider_ref,
   first_name, last_name, date_of_birth, nationality,
   id_type, id_number, id_number_sealed, confidence, reason,
   raw_result, raw_result_sealed, initiated_at, completed_at, created_at, updated_at,
   biz_seq, action, product_code, subject_ref)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5,
        $6, $7, $8,
        $9, $10, $11, $12,
        $13, $14, $15, $16, $17,
        $18, $19, $20, $21, $22, $23,
        $24, $25, $26, $27)
`

const queryUpdate = `
UPDATE guliangiam.identity_verifications
SET status = $1,
    first_name = $2,
    last_name = $3,
    date_of_birth = $4,
    nationality = $5,
    id_type = $6,
    id_number = $7,
    id_number_sealed = $8,
    confidence = $9,
    reason = $10,
    raw_result = $11,
    raw_result_sealed = $12,
    completed_at = $13,
    provider_ref = COALESCE($14, provider_ref),
    updated_at = $15,
    subject_ref = COALESCE($18, subject_ref)
WHERE id = $16::uuid AND organization_id = $17::uuid
`

const queryDelete = `
DELETE FROM guliangiam.identity_verifications
WHERE id = $1::uuid AND organization_id = $2::uuid
`
