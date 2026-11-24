package kerberostickets

const (
	queryGetByTicketID = `
SELECT id, ticket_id, user_id, username, service, realm, expires_at, created_at
FROM "guliangiam".kerberos_tickets
WHERE ticket_id = $1`

	queryCreate = `
INSERT INTO "guliangiam".kerberos_tickets
    (id, ticket_id, user_id, username, service, realm, expires_at, created_at)
VALUES ($1::uuid, $2, NULLIF($3, '')::uuid, $4, $5, $6, $7, $8)`

	queryDeleteByTicketID = `
DELETE FROM "guliangiam".kerberos_tickets
WHERE ticket_id = $1`

	queryDeleteExpired = `
DELETE FROM "guliangiam".kerberos_tickets
WHERE expires_at < NOW()`
)
