-- 一次性存量数据修正：为种子管理员的内置 all-scope 行补 is_bootstrap = TRUE
--
-- 背景
--   V2 结构迁移新增 platform_admin_scopes.is_bootstrap（默认 false）。存量库的
--   种子 V1 已记入迁移账本，不会重放；且 ON CONFLICT DO NOTHING 也不会更新既有行。
--   不修正时，种子管理员的作用域会按 fail-closed 语义失效（组织列表为空）。
--
-- 边界（必须遵守）
--   只命中 built-in 组织 + username='admin' + scope_type='all' + approved_by IS NULL
--   的那一行。其它 approved_by IS NULL 的存量行一律保持 false（继续无效），
--   禁止把本脚本改成无条件 UPDATE。
--
-- 幂等：重复执行命中 0 行。
-- 执行权限：RLS 策略 is_super_admin()，需以 superuser / 迁移角色执行，或在会话中
--           先执行 SELECT set_config('app.current_role','super_admin', false)。
--
-- 推荐流程（真实环境）
--   1) 先跑下面的 SELECT 复核命中行（应恰好 1 行）；
--   2) 备份该行；
--   3) 事务内执行 UPDATE；
--   4) 再跑一次 SELECT 确认 is_bootstrap 已为 true。
--
-- SELECT 复核：
--   SELECT s.id, s.scope_type, s.approved_by, s.expires_at, s.is_bootstrap
--   FROM guliangiam.platform_admin_scopes s
--   JOIN guliangiam.platform_admins pa ON pa.id = s.platform_admin_id
--   JOIN guliangiam.users u ON u.id = pa.user_id
--   JOIN guliangiam.organizations o ON o.id = u.organization_id
--   WHERE u.username = 'admin' AND o.slug = 'built-in'
--     AND u.deleted_at IS NULL AND o.deleted_at IS NULL
--     AND s.scope_type = 'all' AND s.approved_by IS NULL;

BEGIN;

UPDATE guliangiam.platform_admin_scopes s
SET is_bootstrap = TRUE
FROM guliangiam.platform_admins pa
JOIN guliangiam.users u ON u.id = pa.user_id
JOIN guliangiam.organizations o ON o.id = u.organization_id
WHERE s.platform_admin_id = pa.id
  AND u.username = 'admin'
  AND o.slug = 'built-in'
  AND u.deleted_at IS NULL
  AND o.deleted_at IS NULL
  AND s.scope_type = 'all'
  AND s.approved_by IS NULL
  AND s.is_bootstrap = FALSE;

COMMIT;
