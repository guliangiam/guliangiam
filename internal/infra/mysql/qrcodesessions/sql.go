package qrcodesessions

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid                          → ?
//   "guliangiam".table                 → `table`
//   now() → NOW() 保持（MySQL 也是 NOW()）

const btQRSessions = "`qr_code_sessions`"

const queryGetByCodeID = `SELECT id, code_id, code, organization_id, application_id, expires_at, status, user_id, username, ip_address, user_agent, created_at, updated_at, revision FROM ` + btQRSessions + ` WHERE code_id = ?`

const queryGetByCode = `SELECT id, code_id, code, organization_id, application_id, expires_at, status, user_id, username, ip_address, user_agent, created_at, updated_at, revision FROM ` + btQRSessions + ` WHERE code = ?`

const queryCreate = `INSERT INTO ` + btQRSessions + ` (id, code_id, code, organization_id, application_id, expires_at, status, user_id, username, ip_address, user_agent, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

const queryUpdateStatus = `UPDATE ` + btQRSessions + ` SET status = ?, user_id = ?, username = ?, ip_address = ?, user_agent = ?, updated_at = ?, revision = revision + 1 WHERE code_id = ? AND revision = ?`

const queryDeleteByCodeID = `DELETE FROM ` + btQRSessions + ` WHERE code_id = ?`

const queryDeleteExpired = `DELETE FROM ` + btQRSessions + ` WHERE expires_at < NOW()`
