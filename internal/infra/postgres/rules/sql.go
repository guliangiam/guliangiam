package rules

const (
	// queryGetRuleByID — 占位符：id(1)、organization_id(2)。
	queryGetRuleByID = `
SELECT id, organization_id, name, scope, rule_type, conditions, action, priority, is_enabled, created_at, updated_at, revision
FROM guliangiam.rules
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`

	queryListRules = `
SELECT id, organization_id, name, scope, rule_type, conditions, action, priority, is_enabled, created_at, updated_at, revision
FROM guliangiam.rules
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR scope = $2)
  AND ($3 = '' OR rule_type = $3)
  AND ($4::boolean IS NULL OR is_enabled = $4)
ORDER BY priority ASC, created_at DESC
LIMIT $5 OFFSET $6`

	queryCountRules = `
SELECT COUNT(*)
FROM guliangiam.rules
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR scope = $2)
  AND ($3 = '' OR rule_type = $3)
  AND ($4::boolean IS NULL OR is_enabled = $4)`

	queryInsertRule = `
INSERT INTO guliangiam.rules (id, organization_id, name, scope, rule_type, conditions, action, priority, is_enabled, created_at, updated_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::jsonb, $7, $8, $9, $10, $11, $12)`

	// queryUpdateRule — 乐观锁 + 租户谓词。organization_id 取自实体（$11），
	// 调用方必须先做 scoped GetByID，因此 0 行语义仍是 revision 冲突。
	queryUpdateRule = `
UPDATE guliangiam.rules
SET name = $1,
    scope = $2,
    rule_type = $3,
    conditions = $4::jsonb,
    action = $5,
    priority = $6,
    is_enabled = $7,
    updated_at = $8,
    revision = revision + 1
WHERE id = $10::uuid AND revision = $9 AND organization_id = guliangiam.resolve_org_id($11) RETURNING revision`

	queryDeleteRule = `
DELETE FROM guliangiam.rules
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2)`
)
