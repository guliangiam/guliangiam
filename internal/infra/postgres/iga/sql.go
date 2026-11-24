package iga

// Campaigns
const qListCampaignsBase = `SELECT id, organization_id, name, status, start_date, end_date, created_at, updated_at, revision FROM guliangiam.access_campaigns WHERE organization_id = $1`

// qCountCampaignsBase 是 qListCampaignsBase 的计数投影：count 查询不能拼接在
// `SELECT id, ...` 前面（会产生 `SELECT count(*) SELECT id, ...` 的语法错误）。
const qCountCampaignsBase = `SELECT count(*) FROM guliangiam.access_campaigns WHERE organization_id = $1`
const qGetCampaign = `SELECT id, organization_id, name, status, start_date, end_date, created_at, updated_at, revision FROM guliangiam.access_campaigns WHERE id = $1 AND organization_id = $2`
const qInsertCampaign = `INSERT INTO guliangiam.access_campaigns (id, organization_id, name, status, start_date, end_date) VALUES ($1, $2, $3, 'draft', $4, $5)`
const qUpdateCampaignStatus = `UPDATE guliangiam.access_campaigns SET status = $1, updated_at = now() WHERE id = $2 AND organization_id = $3`
const qDeleteCampaign = `DELETE FROM guliangiam.access_campaigns WHERE id = $1 AND organization_id = $2`

// SOD Rules
const qListSODRules = `SELECT id, organization_id, name, role_a_id, role_b_id, created_at, updated_at, revision FROM guliangiam.sod_rules WHERE organization_id = $1 ORDER BY name DESC`

// qListSODRulesByRoleIDs 只取与给定角色集合相关的规则（任一端命中），
// 供 SoD 授权前检查用单查询替代全租户分页扫描（BE-D-17）。
const qListSODRulesByRoleIDs = `SELECT id, organization_id, name, role_a_id, role_b_id, created_at, updated_at, revision FROM guliangiam.sod_rules WHERE organization_id = $1 AND (role_a_id = ANY($2) OR role_b_id = ANY($2)) ORDER BY name DESC`
const qGetSODRule = `SELECT id, organization_id, name, role_a_id, role_b_id, created_at, updated_at, revision FROM guliangiam.sod_rules WHERE id = $1 AND organization_id = $2`
const qInsertSODRule = `INSERT INTO guliangiam.sod_rules (id, organization_id, name, role_a_id, role_b_id) VALUES ($1, $2, $3, $4, $5)`
const qUpdateSODRule = `UPDATE guliangiam.sod_rules SET name = $1, role_a_id = $2, role_b_id = $3, updated_at = now() WHERE id = $4 AND organization_id = $5`
const qDeleteSODRule = `DELETE FROM guliangiam.sod_rules WHERE id = $1 AND organization_id = $2`

// Lifecycle
const qListLifecycleBase = `SELECT id, organization_id, user_id, role_id, granted_at, expires_at, status, created_at, revision FROM guliangiam.access_lifecycle WHERE organization_id = $1`

// qCountLifecycleBase 同上：计数查询必须自带 FROM/WHERE，不能复用列表投影。
const qCountLifecycleBase = `SELECT count(*) FROM guliangiam.access_lifecycle WHERE organization_id = $1`
const qLifecycleColumns = `id, organization_id, user_id, role_id, COALESCE(ticket_id::text, ''), COALESCE(enforcer_id::text, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(policy_id::text, ''), granted_at, starts_at, expires_at, revoked_at, status, retry_count, COALESCE(last_error, ''), created_at, revision`
const qGetLifecycle = `SELECT id, organization_id, user_id, role_id, COALESCE(ticket_id::text, ''), COALESCE(enforcer_id::text, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(policy_id::text, ''), granted_at, starts_at, expires_at, revoked_at, status, retry_count, COALESCE(last_error, ''), created_at, revision FROM guliangiam.access_lifecycle WHERE id = $1 AND organization_id = $2`
const qGetLifecycleForUpdate = `SELECT id, organization_id, user_id, role_id, COALESCE(ticket_id::text, ''), COALESCE(enforcer_id::text, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(policy_id::text, ''), granted_at, starts_at, expires_at, revoked_at, status, retry_count, COALESCE(last_error, ''), created_at, revision FROM guliangiam.access_lifecycle WHERE id = $1 AND organization_id = $2 FOR UPDATE`
const qInsertLifecycle = `INSERT INTO guliangiam.access_lifecycle (id, organization_id, user_id, role_id, ticket_id, enforcer_id, resource, action, policy_id, granted_at, starts_at, expires_at, status) VALUES ($1, $2, $3, $4, $5::uuid, $6::uuid, $7, $8, $9::uuid, $10, $11, $12, $13)`
const qUpdateLifecycleStatus = `UPDATE guliangiam.access_lifecycle SET status = $1 WHERE id = $2 AND organization_id = $3`
const qRevokeLifecycle = `UPDATE guliangiam.access_lifecycle SET status = 'revoked' WHERE id = $1 AND organization_id = $2`
const qExpireLifecycle = `UPDATE guliangiam.access_lifecycle SET status = 'expired' WHERE organization_id = $1 AND status = 'active' AND expires_at < $2`

// Access requests and JIT artifacts.
const qGetAccessRequest = `SELECT id, organization_id, COALESCE(requester_id::text, ''), COALESCE(role_id::text, ''), COALESCE(application_id::text, ''), COALESCE(enforcer_id::text, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(description, ''), COALESCE(workflow_state, status), starts_at, expires_at, COALESCE(manager_approved_by::text, ''), COALESCE(sap_approved_by::text, ''), COALESCE(lifecycle_id::text, ''), created_at, updated_at, revision FROM guliangiam.tickets WHERE organization_id = $1 AND id = $2 AND type = 'iga_access_request' AND deleted_at IS NULL`
const qGetAccessRequestForUpdate = `SELECT id, organization_id, COALESCE(requester_id::text, ''), COALESCE(role_id::text, ''), COALESCE(application_id::text, ''), COALESCE(enforcer_id::text, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(description, ''), COALESCE(workflow_state, status), starts_at, expires_at, COALESCE(manager_approved_by::text, ''), COALESCE(sap_approved_by::text, ''), COALESCE(lifecycle_id::text, ''), created_at, updated_at, revision FROM guliangiam.tickets WHERE organization_id = $1 AND id = $2 AND type = 'iga_access_request' AND deleted_at IS NULL FOR UPDATE`
const qListAccessRequestsBase = `SELECT id, organization_id, COALESCE(requester_id::text, ''), COALESCE(role_id::text, ''), COALESCE(application_id::text, ''), COALESCE(enforcer_id::text, ''), COALESCE(resource, ''), COALESCE(action, ''), COALESCE(description, ''), COALESCE(workflow_state, status), starts_at, expires_at, COALESCE(manager_approved_by::text, ''), COALESCE(sap_approved_by::text, ''), COALESCE(lifecycle_id::text, ''), created_at, updated_at, revision FROM guliangiam.tickets WHERE organization_id = $1 AND type = 'iga_access_request' AND deleted_at IS NULL`
const qInsertAccessRequest = `INSERT INTO guliangiam.tickets (id, organization_id, name, display_name, type, status, creator, description, requester_id, role_id, application_id, enforcer_id, resource, action, starts_at, expires_at, workflow_state, created_at, updated_at, revision) VALUES ($1, $2, $3, $4, 'iga_access_request', 'open', $5, $6, $7::uuid, $8::uuid, $9::uuid, $10::uuid, $11, $12, $13, $14, $15, $16, $17, $18)`
const qUpdateAccessRequest = `UPDATE guliangiam.tickets SET workflow_state = $1::text, status = CASE WHEN $1::text IN ('rejected', 'withdrawn') THEN 'closed' WHEN $1::text = 'provisioned' THEN 'resolved' ELSE 'open' END, manager_approved_by = $2::uuid, sap_approved_by = $3::uuid, lifecycle_id = $4::uuid, updated_at = $5, revision = revision + 1 WHERE organization_id = $6 AND id = $7 AND type = 'iga_access_request' AND workflow_state = $8::text AND revision = $9`
const qInsertTicketMessage = `INSERT INTO guliangiam.ticket_messages (id, ticket_id, author_id, action, content, created_at) VALUES ($1, $2::uuid, $3::uuid, $4, $5, $6)`
const qListTicketMessages = `SELECT m.id, m.ticket_id, COALESCE(m.author_id::text, ''), m.action, m.content, m.created_at FROM guliangiam.ticket_messages m JOIN guliangiam.tickets t ON t.id = m.ticket_id WHERE t.organization_id = $1 AND m.ticket_id = $2 ORDER BY m.created_at, m.id`
const qValidateAccessTargets = `SELECT EXISTS (SELECT 1 FROM guliangiam.users WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL), EXISTS (SELECT 1 FROM guliangiam.roles WHERE organization_id = $1 AND id = $3 AND is_enabled = true), ($4::text = '' OR EXISTS (SELECT 1 FROM guliangiam.applications WHERE organization_id = $1 AND id = $4::uuid)), EXISTS (SELECT 1 FROM guliangiam.enforcers WHERE organization_id = $1 AND id = $5 AND is_enabled = true), COALESCE((SELECT username FROM guliangiam.users WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL), ''), COALESCE((SELECT slug FROM guliangiam.organizations WHERE id = $1 AND deleted_at IS NULL), '')`
const qListEffectiveRoleIDs = `SELECT DISTINCT b.role_id FROM guliangiam.user_role_bindings b JOIN guliangiam.roles r ON r.id = b.role_id WHERE r.organization_id = $1 AND b.user_id = $2 AND (b.starts_at IS NULL OR b.starts_at <= $3) AND (b.expires_at IS NULL OR b.expires_at > $3) UNION SELECT l.role_id FROM guliangiam.access_lifecycle l WHERE l.organization_id = $1 AND l.user_id = $2 AND l.status = 'active' AND l.starts_at <= $3 AND l.expires_at > $3`
const qResolveDefaultEnforcer = `SELECT id FROM guliangiam.enforcers WHERE organization_id = $1 AND is_enabled = true ORDER BY created_at LIMIT 1`

// qInsertJITBinding grants a time-boxed role to a user.
//
// user_role_bindings is keyed by (user_id, role_id), so a repeated provisioning
// of the same role for the same user would violate the primary key. Grants are
// therefore upserts: an existing JIT row is refreshed to the new window, while a
// permanent (manual) binding is left untouched — the role is already held, and
// the JIT expiry must not revoke permanent access.
const qInsertJITBinding = `
INSERT INTO guliangiam.user_role_bindings (user_id, role_id, source, starts_at, expires_at, lifecycle_id)
VALUES ($1, $2, 'iga_jit', $3, $4, $5)
ON CONFLICT (user_id, role_id) DO UPDATE
SET starts_at = EXCLUDED.starts_at,
    expires_at = EXCLUDED.expires_at,
    lifecycle_id = EXCLUDED.lifecycle_id,
    updated_at = now(),
    revision = user_role_bindings.revision + 1
WHERE user_role_bindings.source = 'iga_jit'`
const qDeleteJITBinding = `DELETE FROM guliangiam.user_role_bindings WHERE lifecycle_id = $1 AND source = 'iga_jit'`
const qInsertJITPolicy = `INSERT INTO guliangiam.casbin_rules (id, enforcer_id, ptype, v0, v1, v2, v3, v4, v5) VALUES ($1, $2, 'p', $3, $4, $5, $6, $7, $8)`
const qDeleteJITPolicy = `DELETE FROM guliangiam.casbin_rules WHERE id = $1 AND ptype = 'p'`
const qRevokePrivilegedSessions = `UPDATE guliangiam.sessions SET session_state = 'revoked', revoked_at = $2, is_privileged = false, updated_at = $2, revision = revision + 1 WHERE lifecycle_id = $1 AND revoked_at IS NULL`
const qListExpiringLifecycle = `SELECT ` + qLifecycleColumns + ` FROM guliangiam.access_lifecycle l WHERE l.status = 'active' AND l.expires_at <= $1 ORDER BY l.expires_at LIMIT $2`
const qListExpiringLifecycleByOrg = `SELECT ` + qLifecycleColumns + ` FROM guliangiam.access_lifecycle l WHERE l.organization_id = $1 AND l.status = 'active' AND l.expires_at <= $2 ORDER BY l.expires_at LIMIT $3`
const qCompleteLifecycle = `UPDATE guliangiam.access_lifecycle SET status = $1, revoked_at = $2, retry_count = 0, last_error = '', revision = revision + 1 WHERE organization_id = $3 AND id = $4 AND status = 'active'`
const qRecordLifecycleFailure = `UPDATE guliangiam.access_lifecycle SET retry_count = retry_count + 1, last_error = $1, revision = revision + 1 WHERE organization_id = $2 AND id = $3 AND status = 'active'`
