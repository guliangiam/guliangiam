package usernotifications

const notificationColumns = `id, organization_id, user_id, category, event_type, title, content, link,
	source_type, source_id::text, allow_reply, is_read, read_at, deleted_at, created_at, updated_at, revision`

const (
	queryListNotifications = `
SELECT ` + notificationColumns + `
FROM guliangiam.user_notifications
WHERE organization_id = guliangiam.resolve_org_id($1)
  AND user_id = $2::uuid
  AND deleted_at IS NULL
  AND ($3 = '' OR category = $3)
  AND ($4::boolean IS NULL OR is_read = $4)
  AND ($5 = '' OR title ILIKE '%' || $5 || '%' OR content ILIKE '%' || $5 || '%')
ORDER BY created_at DESC
LIMIT $6 OFFSET $7`

	queryCountNotifications = `
SELECT COUNT(*)
FROM guliangiam.user_notifications
WHERE organization_id = guliangiam.resolve_org_id($1)
  AND user_id = $2::uuid
  AND deleted_at IS NULL
  AND ($3 = '' OR category = $3)
  AND ($4::boolean IS NULL OR is_read = $4)
  AND ($5 = '' OR title ILIKE '%' || $5 || '%' OR content ILIKE '%' || $5 || '%')`

	queryGetNotificationForUser = `
SELECT ` + notificationColumns + `
FROM guliangiam.user_notifications
WHERE id = $1::uuid
  AND organization_id = guliangiam.resolve_org_id($2)
  AND user_id = $3::uuid
  AND deleted_at IS NULL`

	queryUnreadCount = `
SELECT COUNT(*)
FROM guliangiam.user_notifications
WHERE organization_id = guliangiam.resolve_org_id($1)
  AND user_id = $2::uuid
  AND is_read = FALSE
  AND deleted_at IS NULL`

	queryInsertNotification = `
INSERT INTO guliangiam.user_notifications (
	id, organization_id, user_id, category, event_type, title, content, link,
	source_type, source_id, allow_reply, is_read, read_at, created_at, updated_at, revision
) VALUES (
	$1::uuid, guliangiam.resolve_org_id($2), $3::uuid, $4, $5, $6, $7, $8,
	$9, $10::uuid, $11, $12, $13, $14, $15, 1
)`

	queryExistsBySource = `
SELECT EXISTS (
	SELECT 1
	FROM guliangiam.user_notifications
	WHERE organization_id = guliangiam.resolve_org_id($1)
	  AND user_id = $2::uuid
	  AND source_type = $3
	  AND source_id = $4::uuid
	  AND deleted_at IS NULL
)`

	queryMarkRead = `
UPDATE guliangiam.user_notifications
SET is_read = $4,
    read_at = CASE WHEN $4 THEN COALESCE(read_at, $5) ELSE NULL END,
    updated_at = $5,
    revision = revision + 1
WHERE id = $1::uuid
  AND organization_id = guliangiam.resolve_org_id($2)
  AND user_id = $3::uuid
  AND deleted_at IS NULL`

	queryMarkAllRead = `
UPDATE guliangiam.user_notifications
SET is_read = TRUE,
    read_at = COALESCE(read_at, $4),
    updated_at = $4,
    revision = revision + 1
WHERE organization_id = guliangiam.resolve_org_id($1)
  AND user_id = $2::uuid
  AND is_read = FALSE
  AND deleted_at IS NULL
  AND ($3 = '' OR category = $3)`

	querySoftDelete = `
UPDATE guliangiam.user_notifications
SET deleted_at = $4, updated_at = $4, revision = revision + 1
WHERE id = $1::uuid
  AND organization_id = guliangiam.resolve_org_id($2)
  AND user_id = $3::uuid
  AND deleted_at IS NULL`

	queryBatchSoftDelete = `
UPDATE guliangiam.user_notifications
SET deleted_at = $4, updated_at = $4, revision = revision + 1
WHERE organization_id = guliangiam.resolve_org_id($2)
  AND user_id = $3::uuid
  AND deleted_at IS NULL
  AND id::text = ANY($1)`

	queryListReplies = `
SELECT id, notification_id, organization_id, user_id, content, created_at, updated_at, revision
FROM guliangiam.user_notification_replies
WHERE notification_id = $1::uuid
  AND organization_id = guliangiam.resolve_org_id($2)
  AND user_id = $3::uuid
ORDER BY created_at ASC`

	queryInsertReply = `
INSERT INTO guliangiam.user_notification_replies (
	id, notification_id, organization_id, user_id, content, created_at, updated_at, revision
) VALUES ($1::uuid, $2::uuid, guliangiam.resolve_org_id($3), $4::uuid, $5, $6, $7, 1)`

	queryListPreferences = `
SELECT id, organization_id, user_id, category, channel, enabled, created_at, updated_at, revision
FROM guliangiam.notification_preferences
WHERE organization_id = guliangiam.resolve_org_id($1)
  AND user_id = $2::uuid
ORDER BY category, channel`

	queryUpsertPreference = `
INSERT INTO guliangiam.notification_preferences (
	id, organization_id, user_id, category, channel, enabled, created_at, updated_at, revision
) VALUES ($1::uuid, guliangiam.resolve_org_id($2), $3::uuid, $4, $5, $6, $7, $8, 1)
ON CONFLICT (user_id, category, channel) DO UPDATE
SET enabled = EXCLUDED.enabled,
    updated_at = EXCLUDED.updated_at,
    revision = guliangiam.notification_preferences.revision + 1`

	queryListChannelBindings = `
SELECT id, organization_id, user_id, channel, external_id, verified, created_at, updated_at, revision
FROM guliangiam.notification_channel_bindings
WHERE organization_id = guliangiam.resolve_org_id($1)
  AND user_id = $2::uuid
ORDER BY channel, created_at`

	queryUpsertChannelBinding = `
INSERT INTO guliangiam.notification_channel_bindings (
	id, organization_id, user_id, channel, external_id, verified, created_at, updated_at, revision
) VALUES ($1::uuid, guliangiam.resolve_org_id($2), $3::uuid, $4, $5, $6, $7, $8, 1)
ON CONFLICT (user_id, channel, external_id) DO UPDATE
SET verified = EXCLUDED.verified,
    updated_at = EXCLUDED.updated_at,
    revision = guliangiam.notification_channel_bindings.revision + 1`

	queryDeleteChannelBinding = `
DELETE FROM guliangiam.notification_channel_bindings
WHERE id = $1::uuid
  AND organization_id = guliangiam.resolve_org_id($2)
  AND user_id = $3::uuid`
)
