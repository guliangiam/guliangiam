package tickets

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::text                → ?
//   "guliangiam".table                 → `table`
//   ILIKE '%' || $2 || '%'             → LIKE CONCAT('%', ?, '%')
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   $3::text IS NULL OR status = $3    → ? = '' OR status = ?（传两次）

const btTickets = "`tickets`"

// queryGetByID: id(1), organization_id(2)
const queryGetByID = `
SELECT id, organization_id, name, display_name, type, status, priority, assignee, creator,
       description, attachment, resolved_at, created_at, updated_at, revision
FROM ` + btTickets + `
WHERE id = ? AND organization_id = ?`

// queryListTickets
//
//	org: CASE WHEN ?='' THEN TRUE ELSE organization_id=? END             → 2
//	keyword: ?='' OR name LIKE CONCAT('%',?, '%') OR display_name LIKE CONCAT('%',?, '%') → 3
//	status: ?='' OR status=?                                             → 2
//	assignee: ?='' OR assignee=?                                         → 2
//	creator: ?='' OR creator=?                                           → 2
//	priority: ?='' OR priority=?                                         → 2
//	type: ?='' OR type=?                                                 → 2
//	limit, offset                                                        → 2
//	总计：17
const queryListTickets = `
SELECT id, organization_id, name, display_name, type, status, priority, assignee, creator,
       description, attachment, resolved_at, created_at, updated_at, revision
FROM ` + btTickets + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)
  AND (? = '' OR assignee = ?)
  AND (? = '' OR creator = ?)
  AND (? = '' OR priority = ?)
  AND (? = '' OR type = ?)
ORDER BY created_at DESC
LIMIT ? OFFSET ?`

// queryCountTickets: org×2 + keyword×3 + status×2 + assignee×2 + creator×2 + priority×2 + type×2 = 15
const queryCountTickets = `
SELECT COUNT(*)
FROM ` + btTickets + `
WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END)
  AND (? = '' OR name LIKE CONCAT('%', ?, '%') OR display_name LIKE CONCAT('%', ?, '%'))
  AND (? = '' OR status = ?)
  AND (? = '' OR assignee = ?)
  AND (? = '' OR creator = ?)
  AND (? = '' OR priority = ?)
  AND (? = '' OR type = ?)`

// queryInsertTicket: 15 个占位符
const queryInsertTicket = `
INSERT INTO ` + btTickets + ` (id, organization_id, name, display_name, type, status, priority, assignee, creator, description, attachment, resolved_at, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateTicket: display_name(1), type(1), status(1), priority(1), assignee(1),
//
//	description(1), attachment(1), resolved_at(1), updated_at(1), revision(1),
//	id(1), organization_id(1) → 12
const queryUpdateTicket = `
UPDATE ` + btTickets + `
SET display_name = ?, type = ?, status = ?, priority = ?, assignee = ?,
    description = ?, attachment = ?, resolved_at = ?, updated_at = ?, revision = ?
WHERE id = ? AND organization_id = ?`

// queryDeleteTicket: id(1), organization_id(2)
const queryDeleteTicket = `
DELETE FROM ` + btTickets + `
WHERE id = ? AND organization_id = ?`
