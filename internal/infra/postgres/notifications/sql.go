package notifications

const (
	queryInsertNotification = `
INSERT INTO guliangiam.notifications (id, organization_id, provider_id, user_notification_id, channel, provider_kind, recipient, subject, content, template_id, variables, status, error, sent_at, retry_count, created_at, updated_at)
VALUES ($1, $2::uuid, $3::uuid, NULLIF($4, '')::uuid, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`

	queryGetNotificationByID = `
SELECT id, organization_id, provider_id, COALESCE(user_notification_id::text, ''), COALESCE(channel, ''), COALESCE(provider_kind, ''), recipient, subject, content, template_id, variables, status, error, sent_at, retry_count, created_at, updated_at
FROM guliangiam.notifications
WHERE id = $1`

	queryGetNotificationByIDForOrganization = `
SELECT id, organization_id, provider_id, COALESCE(user_notification_id::text, ''), COALESCE(channel, ''), COALESCE(provider_kind, ''), recipient, subject, content, template_id, variables, status, error, sent_at, retry_count, created_at, updated_at
FROM guliangiam.notifications
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($2)`

	queryListNotifications = `
SELECT id, organization_id, provider_id, COALESCE(user_notification_id::text, ''), COALESCE(channel, ''), COALESCE(provider_kind, ''), recipient, subject, content, template_id, variables, status, error, sent_at, retry_count, created_at, updated_at
FROM guliangiam.notifications
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR status = $2)
  AND ($3 = '' OR subject ILIKE '%' || $3 || '%' OR recipient ILIKE '%' || $3 || '%')
ORDER BY created_at DESC
LIMIT $4 OFFSET $5`

	queryCountNotifications = `
SELECT COUNT(*)
FROM guliangiam.notifications
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR status = $2)
  AND ($3 = '' OR subject ILIKE '%' || $3 || '%' OR recipient ILIKE '%' || $3 || '%')`

	queryUpdateNotification = `
UPDATE guliangiam.notifications
SET provider_id = $2::uuid, recipient = $3, subject = $4, content = $5, template_id = $6,
    variables = $7, status = $8, error = $9, sent_at = $10, retry_count = $11, updated_at = $12
WHERE id = $1`

	queryDeleteNotification = `
DELETE FROM guliangiam.notifications WHERE id = $1`

	queryMarkSent = `
UPDATE guliangiam.notifications
SET status = 'sent', sent_at = $2, updated_at = $2
WHERE id = $1`

	queryMarkSentForOrganization = `
UPDATE guliangiam.notifications
SET status = 'sent', sent_at = $2, updated_at = $2
WHERE id = $1 AND organization_id = guliangiam.resolve_org_id($3)`

	queryMarkFailed = `
UPDATE guliangiam.notifications
SET status = 'failed', error = $2, updated_at = $3
WHERE id = $1`

	queryIncrementRetryCount = `
UPDATE guliangiam.notifications
SET retry_count = retry_count + 1, updated_at = $2
WHERE id = $1`

	queryListPendingRetries = `
SELECT id, organization_id, provider_id, COALESCE(user_notification_id::text, ''), COALESCE(channel, ''), COALESCE(provider_kind, ''), recipient, subject, content, template_id, variables, status, error, sent_at, retry_count, created_at, updated_at
FROM guliangiam.notifications
WHERE status = 'pending' AND retry_count < 3
ORDER BY created_at ASC
LIMIT $1`
)
