package dpop

// qInsertJTI records a jti for DPoP replay detection.
// MySQL equivalent of PG's INSERT ... ON CONFLICT DO NOTHING —
// ON DUPLICATE KEY UPDATE jti = jti is a no-op on collision,
// but MySQL's LastInsertId returns 0 on update, so we use RowsAffected
// to determine whether this was the first insert (1 row) or a replay (0).
const qInsertJTI = "INSERT INTO guliangiam.dpop_replay (jti) VALUES (?) ON DUPLICATE KEY UPDATE jti = jti"
