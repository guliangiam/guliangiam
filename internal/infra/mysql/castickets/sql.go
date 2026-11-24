package castickets

// SQL 方言转换规则（PG → MySQL）：
//   $N                               → ?
//   "guliangiam".table               → `table`
//   now()                            → NOW()
//   DELETE ... RETURNING             → MySQL 用 DELETE 单独执行
//   UPDATE ... RETURNING             → MySQL UPDATE（判 affected rows）+ SELECT
//   TRUE/FALSE                       → true/false（MySQL 可直接用）

const btCasTickets = "`cas_tickets`"

// queryGetByTicketID: ticket_id(1)
const queryGetByTicketID = `
SELECT id, ticket_id, ticket_type, user_id, username, service, expires_at, created_at,
       pgt_iou, pgt_iou_expires_at, pgt_iou_used
FROM ` + btCasTickets + `
WHERE ticket_id = ?`

// queryGetByPGTIOU: pgt_iou(1)
const queryGetByPGTIOU = `
SELECT id, ticket_id, ticket_type, user_id, username, service, expires_at, created_at,
       pgt_iou, pgt_iou_expires_at, pgt_iou_used
FROM ` + btCasTickets + `
WHERE pgt_iou = ?`

// queryCreate: 11 个占位符
const queryCreate = `
INSERT INTO ` + btCasTickets + ` (id, ticket_id, ticket_type, user_id, username, service, expires_at, created_at, pgt_iou, pgt_iou_expires_at, pgt_iou_used) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryDeleteByTicketID: ticket_id(1)
const queryDeleteByTicketID = `
DELETE FROM ` + btCasTickets + `
WHERE ticket_id = ?`

// queryDeleteExpired: 无参数
const queryDeleteExpired = `
DELETE FROM ` + btCasTickets + `
WHERE expires_at < NOW()`

// queryMarkPGTIOUUsed: pgt_iou(1)
// WHERE 保证只处理未使用 + 未过期的 → 0 affected 说明无效
// PG 版是 UPDATE ... RETURNING ticket_id，MySQL 版拆成 UPDATE + SELECT
const queryMarkPGTIOUUsed = `
UPDATE ` + btCasTickets + `
SET pgt_iou_used = true
WHERE pgt_iou = ?
  AND pgt_iou_used = false
  AND pgt_iou_expires_at > NOW()`

// querySelectPGTTicketID: pgt_iou(1) → UPDATE 之后拿回 ticket_id
const querySelectPGTTicketID = `
SELECT ticket_id FROM ` + btCasTickets + ` WHERE pgt_iou = ?`
