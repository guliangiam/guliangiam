package rules

// SQL 方言转换规则（PG → MySQL）：
//   $N::uuid / $N::jsonb              → ?
//   "guliangiam".table / guliangiam.table → `table`
//   $4::boolean IS NULL OR col=$4      → ? IS NULL OR col = ?
//   guliangiam.resolve_org_id($1)      → 直接 organization_id = ?
//   RETURNING revision                 → UPDATE + SELECT 两步

const btRules = "`rules`"

// queryGetRuleByID 占位符：id(1)、organization_id(2)
const queryGetRuleByID = `SELECT id, organization_id, name, scope, rule_type, conditions, action, priority, is_enabled, created_at, updated_at, revision FROM ` + btRules + ` WHERE id = ? AND organization_id = ?`

// queryListRules 占位符计数：
//
//	org_id: CASE WHEN ?='' → organization_id=?            → 2
//	scope: ?='' OR scope=?                                 → 2
//	rule_type: ?='' OR rule_type=?                         → 2
//	is_enabled: ? IS NULL OR is_enabled=?                  → 2
//	limit, offset                                          → 2
//	总计：10
const queryListRules = `SELECT id, organization_id, name, scope, rule_type, conditions, action, priority, is_enabled, created_at, updated_at, revision FROM ` + btRules + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR scope = ?) AND (? = '' OR rule_type = ?) AND (? IS NULL OR is_enabled = ?) ORDER BY priority ASC, created_at DESC LIMIT ? OFFSET ?`

// queryCountRules: org_id×2 + scope×2 + rule_type×2 + is_enabled×2 = 8
const queryCountRules = `SELECT COUNT(*) FROM ` + btRules + ` WHERE (CASE WHEN ? = '' THEN TRUE ELSE organization_id = ? END) AND (? = '' OR scope = ?) AND (? = '' OR rule_type = ?) AND (? IS NULL OR is_enabled = ?)`

const queryInsertRule = `INSERT INTO ` + btRules + ` (id, organization_id, name, scope, rule_type, conditions, action, priority, is_enabled, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// queryUpdateRule: 乐观锁更新，revision 原子 +1。WHERE id=? AND revision=? AND organization_id=?
const queryUpdateRule = `UPDATE ` + btRules + ` SET name = ?, scope = ?, rule_type = ?, conditions = ?, action = ?, priority = ?, is_enabled = ?, updated_at = ?, revision = revision + 1 WHERE id = ? AND revision = ? AND organization_id = ?`

// querySelectRuleRevision: UPDATE 后读新 revision（organization_id 谓词防止
// MySQL（无 RLS）在 id 复用时读到其他租户的行）
const querySelectRuleRevision = `SELECT revision FROM ` + btRules + ` WHERE id = ? AND organization_id = ?`

const queryDeleteRule = `DELETE FROM ` + btRules + ` WHERE id = ? AND organization_id = ?`
