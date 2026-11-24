package castickets

const queryGetByTicketID = `SELECT id, ticket_id, ticket_type, user_id, username, service, expires_at, created_at, pgt_iou, pgt_iou_expires_at, pgt_iou_used FROM guliangiam.cas_tickets WHERE ticket_id = $1`

const queryCreate = `INSERT INTO guliangiam.cas_tickets (id, ticket_id, ticket_type, user_id, username, service, expires_at, created_at, pgt_iou, pgt_iou_expires_at, pgt_iou_used) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

const queryDeleteByTicketID = `DELETE FROM guliangiam.cas_tickets WHERE ticket_id = $1 RETURNING id, ticket_id, ticket_type, user_id, username, service, expires_at, created_at, pgt_iou, pgt_iou_expires_at, pgt_iou_used`

const queryDeleteExpired = `DELETE FROM guliangiam.cas_tickets WHERE expires_at < now()`

const queryGetByPGTIOU = `SELECT id, ticket_id, ticket_type, user_id, username, service, expires_at, created_at, pgt_iou, pgt_iou_expires_at, pgt_iou_used FROM guliangiam.cas_tickets WHERE pgt_iou = $1`

const queryMarkPGTIOUUsed = `UPDATE guliangiam.cas_tickets SET pgt_iou_used = TRUE WHERE pgt_iou = $1 AND pgt_iou_used = FALSE AND pgt_iou_expires_at > now() RETURNING ticket_id`
