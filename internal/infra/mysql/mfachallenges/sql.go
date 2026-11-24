package mfachallenges

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::timestamptz  → ?
//   guliangiam.table            → `table`
//   RETURNING revision          → MySQL 用 UPDATE + SELECT 两步

const btLoginChallenges = "`login_challenges`"

// queryGetChallengeByID: id(1) - 12 列
const queryGetChallengeByID = `
SELECT id, organization_id, user_id, application_id, state, method_type, metadata,
       expires_at, verified_at, created_at, updated_at, revision
FROM ` + btLoginChallenges + `
WHERE id = ?`

// queryInsertChallenge: 12 个占位符
const queryInsertChallenge = `
INSERT INTO ` + btLoginChallenges + ` (id, organization_id, user_id, application_id, state, method_type, metadata, expires_at, verified_at, created_at, updated_at, revision)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateChallenge: PG RETURNING revision 改写
// MySQL UPDATE 不支持 RETURNING — 先 UPDATE 检查 affected，再单独 SELECT
// 参数：state, method_type, metadata, expires_at, verified_at, updated_at, old_revision, id
const queryUpdateChallenge = `
UPDATE ` + btLoginChallenges + `
SET state = ?, method_type = ?, metadata = ?, expires_at = ?, verified_at = ?, updated_at = ?, revision = revision + 1
WHERE id = ? AND revision = ?`

// querySelectRevisionAfterUpdate: UPDATE 之后拿新 revision
const querySelectRevisionAfterUpdate = `
SELECT revision FROM ` + btLoginChallenges + ` WHERE id = ?`

// queryDeleteChallenge: id(1)
const queryDeleteChallenge = `
DELETE FROM ` + btLoginChallenges + ` WHERE id = ?`
