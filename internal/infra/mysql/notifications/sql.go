package notifications

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::timestamptz  → ?
//   guliangiam.table            → `table`
//   guliangiam.resolve_org_id($1) → 直接 organization_id = ?
//   ILIKE '%' || $3 || '%'      → LIKE CONCAT('%', ?, '%')
//   now()                       → NOW()
//   jsonb (variables)           → TEXT (JSON 字符串)

const btNotifications = "`notifications`"

// notificationSelectBase: 17 列
const notificationSelectBase = `
SELECT id, organization_id, provider_id, COALESCE(user_notification_id, ''), COALESCE(channel, ''), COALESCE(provider_kind, ''), recipient, subject, content,
       template_id, variables, status, error, sent_at, retry_count,
       created_at, updated_at
FROM ` + btNotifications

// queryGetNotificationByID: id(1)
const queryGetNotificationByID = notificationSelectBase + `
WHERE id = ?`

// queryGetNotificationByIDForOrganization: id(1), organization_id(2)
const queryGetNotificationByIDForOrganization = notificationSelectBase + `
WHERE id = ? AND organization_id = ?`

// queryListNotifications: org_id×2(2), status×2(2), keyword×3(3), limit, offset(2) = 9
//
//	CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END
//	AND (? = '' OR status = ?)
//	AND (? = '' OR subject LIKE CONCAT('%', ?, '%') OR recipient LIKE CONCAT('%', ?, '%'))
const queryListNotifications = notificationSelectBase + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR status = ?)
  AND (? = '' OR subject LIKE CONCAT('%', ?, '%') OR recipient LIKE CONCAT('%', ?, '%'))
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountNotifications: 去掉 LIMIT/OFFSET，占位符数 = 7
const queryCountNotifications = `
SELECT COUNT(*)
FROM ` + btNotifications + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR status = ?)
  AND (? = '' OR subject LIKE CONCAT('%', ?, '%') OR recipient LIKE CONCAT('%', ?, '%'))`

// queryInsertNotification: 17 个占位符
const queryInsertNotification = `
INSERT INTO ` + btNotifications + ` (id, organization_id, provider_id, user_notification_id, channel, provider_kind, recipient, subject, content, template_id, variables, status, error, sent_at, retry_count, created_at, updated_at)
VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateNotification: 12 个占位符
const queryUpdateNotification = `
UPDATE ` + btNotifications + `
SET provider_id = ?, recipient = ?, subject = ?, content = ?, template_id = ?,
    variables = ?, status = ?, error = ?, sent_at = ?, retry_count = ?, updated_at = ?
WHERE id = ?`

// queryDeleteNotification: id(1)
const queryDeleteNotification = `
DELETE FROM ` + btNotifications + ` WHERE id = ?`

// queryMarkSent: id(1), now(2)
const queryMarkSent = `
UPDATE ` + btNotifications + `
SET status = 'sent', sent_at = ?, updated_at = ?
WHERE id = ?`

// queryMarkSentForOrganization: sent_at(1), updated_at(2), id(3), organization_id(4)
const queryMarkSentForOrganization = `
UPDATE ` + btNotifications + `
SET status = 'sent', sent_at = ?, updated_at = ?
WHERE id = ? AND organization_id = ?`

// queryMarkFailed: id(1), errMsg(2), now(3)
const queryMarkFailed = `
UPDATE ` + btNotifications + `
SET status = 'failed', error = ?, updated_at = ?
WHERE id = ?`

// queryIncrementRetryCount: id(1), now(2)
const queryIncrementRetryCount = `
UPDATE ` + btNotifications + `
SET retry_count = retry_count + 1, updated_at = ?
WHERE id = ?`

// queryListPendingRetries: limit(1)
const queryListPendingRetries = notificationSelectBase + `
WHERE status = 'pending' AND retry_count < 3
ORDER BY created_at ASC
LIMIT ?`
