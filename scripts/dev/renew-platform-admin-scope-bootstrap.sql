-- 运维：续期 built-in 管理员 all-tenant bootstrap scope（OPEN-API-54）。
-- 仅在 scope 即将到期（<= 7 天）时执行；业务键定位，幂等，返回更新行。
-- 安全边界：只处理 built-in 组织的 admin、approved_by IS NULL 的 all-scope 行；
-- 其它经审批或非 bootstrap 的 scope 不受影响。执行前务必确认变更单。
UPDATE guliangiam.platform_admin_scopes AS s
SET expires_at = created_at + interval '1 year'
WHERE s.id = (
    SELECT candidate.id
    FROM guliangiam.platform_admin_scopes candidate
    JOIN guliangiam.platform_admins pa ON pa.id = candidate.platform_admin_id
    JOIN guliangiam.users u ON u.id = pa.user_id
    JOIN guliangiam.organizations o ON o.id = u.organization_id
    WHERE o.slug = 'built-in'
      AND u.username = 'admin'
      AND u.deleted_at IS NULL
      AND o.deleted_at IS NULL
      AND candidate.scope_type = 'all'
      AND candidate.approved_by IS NULL
      AND candidate.expires_at < now() + interval '7 days'
      AND candidate.created_at + interval '1 year' > now() + interval '7 days'
    ORDER BY candidate.expires_at
    LIMIT 1
)
RETURNING s.id, s.expires_at;
