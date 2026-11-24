package kerberostickets

// SQL 方言转换规则（PG → MySQL）：
//   $1::uuid / NULLIF($3, '')::uuid  → ? / NULLIF(?, '')
//   "guliangiam".table               → `table`
//   $N                               → ?
//   NOW()                            → NOW()（MySQL 也支持）

const btKerberosTickets = "`kerberos_tickets`"

// GetByTicketID: 按 Kerberos ticket_id 查找。
const queryGetByTicketID = `
SELECT id, ticket_id, user_id, username, service, realm, expires_at, created_at
FROM ` + btKerberosTickets + `
WHERE ticket_id = ?`

// Create: 插入 Kerberos ticket。
// PG 版 NULLIF($3, ”)::uuid — MySQL 直接 NULLIF(?, ”)，因为 user_id 在 MySQL 是可空 CHAR/UUID 类型。
const queryCreate = `
INSERT INTO ` + btKerberosTickets + `
    (id, ticket_id, user_id, username, service, realm, expires_at, created_at)
VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`

// DeleteByTicketID: 按 ticket_id 删除单条。
const queryDeleteByTicketID = `
DELETE FROM ` + btKerberosTickets + `
WHERE ticket_id = ?`

// DeleteExpired: 清理所有过期票据，返回受影响行数。
const queryDeleteExpired = `
DELETE FROM ` + btKerberosTickets + `
WHERE expires_at < NOW()`
