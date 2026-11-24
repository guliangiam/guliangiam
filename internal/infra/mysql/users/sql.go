package users

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

const (
	// columnUsername 是用户名列（也用于排序映射的 JSON 键）。
	columnUsername = "username"
	// columnUpdatedAt 是更新时间列（Upsert 更新列表复用）。
	columnUpdatedAt = "updated_at"
)

// PG→MySQL 转换规则：
//   $N::uuid/jsonb/text/inet/boolean → ?            (MySQL 自动类型推断)
//   guliangiam.table → `table`                      (MySQL 无 schema 前缀)
//   ILIKE → LIKE                                    (MySQL 默认 CI collation)
//   'a' || b || 'c' → CONCAT('a', b, 'c')           (MySQL 字符串拼接函数)
//   now() / NOW() → NOW()                           (MySQL 统一大写)
//   ON CONFLICT (col) DO UPDATE SET col = EXCLUDED.col → ON DUPLICATE KEY UPDATE col = VALUES(col)
//   RETURNING col → 删掉 → Create 后 SELECT 回读
//   ANY($1::uuid[]) → IN (?, ?, ?, ...)              (repository 动态拼)
//   guliangiam.resolve_org_id($1) → 直接用 ?         (PG 自定义函数，MySQL 不需要)
//   ::text cast 冗余 → 删掉
//   '[]'::jsonb → '[]'
//   signin_wrong_times / preferred_mfa_type / totp_secret / balance_* 等列不在
//     `users` 主表：按 ADR-022 的 God Table 拆分，读路径 LEFT JOIN 5 张 user_* 子表，
//     写路径先写主表再以 user_id 为键 upsert 子表（与 PostgreSQL 同构）。
//   CASE WHEN $1 = '' THEN TRUE ELSE u.org = guliangiam.resolve_org_id($1) END → (? = '' OR u.org = ?)

const (
	// userAggregateColumns 是用户聚合读的列顺序：基础列来自 `users`，其余来自
	// ADR-022 拆分的 5 张 user_* 子表。扫描目标 userRow 的字段顺序必须与此一致。
	userAggregateColumns = `u.id, u.organization_id, u.username, COALESCE(u.external_id, ''), u.user_type, u.status, u.is_admin, u.is_forbidden, COALESCE(u.is_verified, false), COALESCE(u.is_online, false), COALESCE(u.created_ip, ''), COALESCE(u.signup_application_id, ''), COALESCE(u.register_type, ''), COALESCE(u.register_source, ''), COALESCE(u.tag, ''), COALESCE(u.score, 0), COALESCE(st.signin_wrong_times, 0), st.last_signin_wrong_time, COALESCE(mc.preferred_mfa_type, ''), COALESCE(mc.recovery_code_hashes, '[]'), COALESCE(mc.totp_secret, ''), COALESCE(mc.mfa_radius_username, ''), COALESCE(mc.mfa_radius_provider, ''), COALESCE(mc.mfa_push_receiver, ''), COALESCE(mc.mfa_push_provider, ''), COALESCE(mc.mfa_push_enabled, false), mc.mfa_remember_deadline, st.last_signin_at, COALESCE(st.last_signin_ip, ''), COALESCE(fin.balance, 0), COALESCE(fin.balance_credit, 0), COALESCE(fin.balance_currency, 'USD'), COALESCE(u.is_deleted, false), COALESCE(u.ip_whitelist, ''), COALESCE(col.addresses, '[]'), COALESCE(col.managed_accounts, '[]'), COALESCE(col.face_ids, '[]'), COALESCE(col.cart, '[]'), COALESCE(col.mfa_items, '[]'), COALESCE(col.mfa_accounts, '[]'), COALESCE(col.application_scopes, '[]'), COALESCE(ca.custom1, ''), COALESCE(ca.custom2, ''), COALESCE(ca.custom3, ''), COALESCE(ca.custom4, ''), COALESCE(ca.custom5, ''), COALESCE(ca.custom6, ''), COALESCE(ca.custom7, ''), COALESCE(ca.custom8, ''), COALESCE(ca.custom9, ''), COALESCE(ca.custom10, ''), u.created_at, u.updated_at, u.deleted_at, u.revision`

	userAggregateFrom = `
FROM ` + "`users`" + ` u
LEFT JOIN ` + "`user_signin_tracking`" + ` st ON st.user_id = u.id
LEFT JOIN ` + "`user_mfa_config`" + ` mc ON mc.user_id = u.id
LEFT JOIN ` + "`user_financials`" + ` fin ON fin.user_id = u.id
LEFT JOIN ` + "`user_collections`" + ` col ON col.user_id = u.id
LEFT JOIN ` + "`user_custom_attributes`" + ` ca ON ca.user_id = u.id`

	queryGetUserByID = `SELECT ` + userAggregateColumns + userAggregateFrom + `
WHERE u.id = ? AND u.deleted_at IS NULL`

	// queryGetUserByIDForOrganization 与 PG 同构：by-ID 读追加 organization_id，
	// 跨租户行与不存在行一样返回 sql.ErrNoRows → NotFound。
	queryGetUserByIDForOrganization = `SELECT ` + userAggregateColumns + userAggregateFrom + `
WHERE u.id = ? AND u.organization_id = ? AND u.deleted_at IS NULL`

	// queryLockUserByID 取用户行的行级锁（FOR UPDATE），直到事务提交/回滚。
	// 与 postgres 同名常量语义一致：锚点必须是用户行（首次登录时会话行尚不存在）。
	queryLockUserByID = `
SELECT id FROM ` + "`users`" + `
WHERE id = ? AND deleted_at IS NULL
FOR UPDATE`

	queryGetUserByUsername = `SELECT ` + userAggregateColumns + userAggregateFrom + `
WHERE (? = '' OR u.organization_id = ?) AND u.username = ? AND u.deleted_at IS NULL`

	// queryGetUserByLoginIdentifier 与 PG 同构：用户名精确 > 邮箱忽略大小写 > 手机号精确。
	// MySQL 没有 schema 前缀与 resolve_org_id 函数，租户谓词直接用两个 ?。
	queryGetUserByLoginIdentifier = `SELECT ` + userAggregateColumns + userAggregateFrom + `
LEFT JOIN ` + "`user_profiles`" + ` p ON p.user_id = u.id AND p.deleted_at IS NULL
WHERE (? = '' OR u.organization_id = ?) AND u.deleted_at IS NULL
  AND (u.username = ? OR LOWER(p.email) = LOWER(?) OR p.phone = ?)
LIMIT 5`

	queryListUsers = `SELECT ` + userAggregateColumns + userAggregateFrom + `
WHERE (? = '' OR u.organization_id = ?)
  AND (? = '' OR u.username LIKE CONCAT('%', ?, '%') OR u.external_id LIKE CONCAT('%', ?, '%'))
  AND u.deleted_at IS NULL
  AND (? = '' OR u.status = ?)
  AND (? = '' OR u.user_type = ?)
  AND (? IS NULL OR u.is_admin = ?)`

	queryCountUsers = `
SELECT COUNT(*)
FROM ` + "`users`" + ` u
WHERE (? = '' OR u.organization_id = ?)
  AND (? = '' OR u.username LIKE CONCAT('%', ?, '%') OR u.external_id LIKE CONCAT('%', ?, '%'))
  AND u.deleted_at IS NULL
  AND (? = '' OR u.status = ?)
  AND (? = '' OR u.user_type = ?)
  AND (? IS NULL OR u.is_admin = ?)`

	queryGetUserProfileByUserID = `
SELECT user_id, COALESCE(display_name, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(avatar_url, ''), COALESCE(email, ''), email_verified, COALESCE(phone, ''), phone_verified, COALESCE(country_code, ''), COALESCE(region, ''), COALESCE(location, ''), COALESCE(CAST(birthday AS CHAR), ''), COALESCE(gender, ''), COALESCE(bio, ''), COALESCE(real_name, ''), COALESCE(id_card_type, ''), COALESCE(id_card, ''), COALESCE(passport_no, ''), COALESCE(driver_license_no, ''), COALESCE(CAST(id_card_expiry AS CHAR), ''), COALESCE(issue_country, ''), COALESCE(issuing_authority, ''), COALESCE(passport_photo_url, ''), COALESCE(driver_license_front_url, ''), COALESCE(driver_license_back_url, ''), properties
FROM ` + "`user_profiles`" + `
WHERE user_id = ?`

	queryGetUserCredentialsByUserID = `
SELECT user_id, password_hash, password_algo, password_updated_at, access_key, access_secret_hash, access_secret_rotated_at, need_update_password, COALESCE(access_secret_hash_prev, ''), access_secret_prev_expires_at
FROM ` + "`user_credentials`" + `
WHERE user_id = ?`

	queryListUserIdentitiesByUserID = `
SELECT id, user_id, provider_id, identity_type, subject, provider_account, profile_snapshot, linked_at
FROM ` + "`user_identities`" + `
WHERE user_id = ?
ORDER BY linked_at DESC`

	queryListUserMfaMethodsByUserID = `
SELECT id, user_id, method_type, enabled, verified, display_name, last_used_at
FROM ` + "`user_mfa_methods`" + `
WHERE user_id = ?
ORDER BY method_type ASC`

	// queryInsertUserCore 只写 `users` 主表的 22 列；子表由 upsertUserSubtables 写入。
	queryInsertUserCore = `
INSERT INTO ` + "`users`" + ` (id, organization_id, username, external_id, user_type, status, is_admin, is_forbidden, is_verified, is_online, created_ip, signup_application_id, register_type, register_source, tag, score, is_deleted, ip_whitelist, created_at, updated_at, deleted_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// queryUpdateUserCore 只更新 `users` 主表；WHERE 带 revision 做乐观锁，
	// revision = revision + 1 与 PostgreSQL 的 RETURNING revision 语义等价。
	queryUpdateUserCore = `
UPDATE ` + "`users`" + `
SET external_id = ?,
    user_type = ?,
    status = ?,
    is_admin = ?,
    is_forbidden = ?,
    is_verified = ?,
    is_online = ?,
    created_ip = ?,
    signup_application_id = ?,
    register_type = ?,
    register_source = ?,
    tag = ?,
    score = ?,
    is_deleted = ?,
    ip_whitelist = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
  WHERE id = ? AND revision = ?`

	// queryUpdateUserCoreForOrganization 与 queryUpdateUserCore 同构，WHERE 追加
	// organization_id：跨租户 UPDATE 影响 0 行，由仓储映射为 NotFound。
	queryUpdateUserCoreForOrganization = `
UPDATE ` + "`users`" + `
SET external_id = ?,
    user_type = ?,
    status = ?,
    is_admin = ?,
    is_forbidden = ?,
    is_verified = ?,
    is_online = ?,
    created_ip = ?,
    signup_application_id = ?,
    register_type = ?,
    register_source = ?,
    tag = ?,
    score = ?,
    is_deleted = ?,
    ip_whitelist = ?,
    updated_at = ?,
    deleted_at = ?,
    revision = revision + 1
  WHERE id = ? AND revision = ? AND organization_id = ? AND deleted_at IS NULL`

	// queryUserExistsForOrganization 只在 scoped UPDATE 影响 0 行后被调用，用来区分
	// 「同租户但 revision 过期」（Conflict）与「跨租户/不存在」（NotFound）。MySQL
	// 无 RLS，谓词必须显式带 organization_id，且与 UPDATE 完全同构。
	queryUserExistsForOrganization = `
SELECT 1 FROM ` + "`users`" + `
WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryUpsertUserSigninTracking = `
INSERT INTO ` + "`user_signin_tracking`" + ` (user_id, signin_wrong_times, last_signin_wrong_time, last_signin_at, last_signin_ip) VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    signin_wrong_times = VALUES(signin_wrong_times),
    last_signin_wrong_time = VALUES(last_signin_wrong_time),
    last_signin_at = VALUES(last_signin_at),
    last_signin_ip = VALUES(last_signin_ip)`

	queryUpsertUserMfaConfig = `
INSERT INTO ` + "`user_mfa_config`" + ` (user_id, preferred_mfa_type, recovery_code_hashes, totp_secret, mfa_radius_username, mfa_radius_provider, mfa_push_receiver, mfa_push_provider, mfa_push_enabled, mfa_remember_deadline) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    preferred_mfa_type = VALUES(preferred_mfa_type),
    recovery_code_hashes = VALUES(recovery_code_hashes),
    totp_secret = VALUES(totp_secret),
    mfa_radius_username = VALUES(mfa_radius_username),
    mfa_radius_provider = VALUES(mfa_radius_provider),
    mfa_push_receiver = VALUES(mfa_push_receiver),
    mfa_push_provider = VALUES(mfa_push_provider),
    mfa_push_enabled = VALUES(mfa_push_enabled),
    mfa_remember_deadline = VALUES(mfa_remember_deadline)`

	queryUpsertUserFinancials = `
INSERT INTO ` + "`user_financials`" + ` (user_id, balance, balance_credit, balance_currency) VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    balance = VALUES(balance),
    balance_credit = VALUES(balance_credit),
    balance_currency = VALUES(balance_currency)`

	queryUpdateUserContactVerified = `
UPDATE ` + "`user_profiles`" + ` p
JOIN ` + "`users`" + ` u ON u.id = p.user_id
SET p.email_verified = ?, p.phone_verified = ?
WHERE p.user_id = ? AND u.organization_id = ? AND u.deleted_at IS NULL`

	queryUpsertUserCollections = `
INSERT INTO ` + "`user_collections`" + ` (user_id, addresses, managed_accounts, face_ids, cart, mfa_items, mfa_accounts, application_scopes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    addresses = VALUES(addresses),
    managed_accounts = VALUES(managed_accounts),
    face_ids = VALUES(face_ids),
    cart = VALUES(cart),
    mfa_items = VALUES(mfa_items),
    mfa_accounts = VALUES(mfa_accounts),
    application_scopes = VALUES(application_scopes)`

	queryUpsertUserCustomAttributes = `
INSERT INTO ` + "`user_custom_attributes`" + ` (user_id, custom1, custom2, custom3, custom4, custom5, custom6, custom7, custom8, custom9, custom10) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
    custom1 = VALUES(custom1),
    custom2 = VALUES(custom2),
    custom3 = VALUES(custom3),
    custom4 = VALUES(custom4),
    custom5 = VALUES(custom5),
    custom6 = VALUES(custom6),
    custom7 = VALUES(custom7),
    custom8 = VALUES(custom8),
    custom9 = VALUES(custom9),
    custom10 = VALUES(custom10)`

	queryDeleteUserIdentities = `DELETE FROM ` + "`user_identities`" + ` WHERE user_id = ?`
	queryInsertUserIdentity   = `INSERT INTO ` + "`user_identities`" + ` (id, user_id, provider_id, identity_type, subject, provider_account, profile_snapshot, linked_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	queryDeleteSingleIdentity = `DELETE FROM ` + "`user_identities`" + ` WHERE user_id = ? AND id = ?`

	queryDeleteUserMfaMethods = `DELETE FROM ` + "`user_mfa_methods`" + ` WHERE user_id = ?`
	queryInsertUserMfaMethod  = `INSERT INTO ` + "`user_mfa_methods`" + ` (id, user_id, method_type, enabled, verified, display_name, last_used_at) VALUES (?, ?, ?, ?, ?, ?, ?)`

	queryDeleteUserRoleBindings = `DELETE FROM ` + "`user_role_bindings`" + ` WHERE user_id = ?`
	queryInsertUserRoleBinding  = `INSERT INTO ` + "`user_role_bindings`" + ` (user_id, role_id) VALUES (?, ?)`

	queryDeleteUserGroupMemberships = `DELETE FROM ` + "`user_group_memberships`" + ` WHERE user_id = ?`
	queryInsertUserGroupMembership  = `INSERT INTO ` + "`user_group_memberships`" + ` (user_id, group_id) VALUES (?, ?)`

	// queryDeleteUser 软删。PostgreSQL 由 trg_users_sync_is_deleted 触发器从
	// deleted_at 派生 is_deleted；MySQL 无触发器，SET 显式同步该列。
	queryDeleteUser = `UPDATE ` + "`users`" + ` SET deleted_at = NOW(), is_deleted = true, updated_at = NOW() WHERE id = ? AND organization_id = ? AND deleted_at IS NULL`

	queryListRoleBindingsByUserIDs = `SELECT user_id, role_id FROM ` + "`user_role_bindings`" + ` WHERE user_id IN (PLACEHOLDER_IDS)`
	queryListRoleNamesForUser      = `
SELECT r.name
FROM ` + "`user_role_bindings`" + ` ur
JOIN ` + "`roles`" + ` r ON ur.role_id = r.id
WHERE ur.user_id = ? AND r.is_enabled = 1
ORDER BY r.name`
	queryListGroupMembershipsByUserIDs = `SELECT user_id, group_id FROM ` + "`user_group_memberships`" + ` WHERE user_id IN (PLACEHOLDER_IDS)`

	queryGetPasswordHistory = `
SELECT id, user_id, password_hash, password_algo, created_at
FROM ` + "`user_password_history`" + `
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ?`

	queryInsertPasswordHistory = `
INSERT INTO ` + "`user_password_history`" + ` (id, user_id, password_hash, password_algo, created_at)
VALUES (?, ?, ?, ?, ?)`

	queryDeletePasswordHistoryBeyond = `
DELETE FROM ` + "`user_password_history`" + `
WHERE user_id = ?
  AND id NOT IN (
    SELECT id FROM (
      SELECT id FROM ` + "`user_password_history`" + `
      WHERE user_id = ?
      ORDER BY created_at DESC
      LIMIT ?
    ) AS t
  )`

	queryGetUserByProviderIdentity = `SELECT ` + userAggregateColumns + userAggregateFrom + `
INNER JOIN ` + "`user_identities`" + ` ui ON ui.user_id = u.id
WHERE ui.provider_id = ? AND ui.subject = ? AND u.deleted_at IS NULL`

	queryBatchLoadProfiles = `
SELECT user_id, COALESCE(display_name, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(avatar_url, ''), COALESCE(email, ''), email_verified, COALESCE(phone, ''), phone_verified, COALESCE(country_code, ''), COALESCE(region, ''), COALESCE(location, ''), COALESCE(CAST(birthday AS CHAR), ''), COALESCE(gender, ''), COALESCE(bio, ''), COALESCE(real_name, ''), COALESCE(id_card_type, ''), COALESCE(id_card, ''), COALESCE(passport_no, ''), COALESCE(driver_license_no, ''), COALESCE(CAST(id_card_expiry AS CHAR), ''), COALESCE(issue_country, ''), COALESCE(issuing_authority, ''), COALESCE(passport_photo_url, ''), COALESCE(driver_license_front_url, ''), COALESCE(driver_license_back_url, ''), properties
FROM ` + "`user_profiles`" + `
WHERE user_id IN (PLACEHOLDER_IDS)`

	queryBatchLoadCredentials = `
SELECT user_id, password_hash, password_algo, password_updated_at, access_key, access_secret_hash, access_secret_rotated_at, need_update_password, COALESCE(access_secret_hash_prev, ''), access_secret_prev_expires_at
FROM ` + "`user_credentials`" + `
WHERE user_id IN (PLACEHOLDER_IDS)`

	queryBatchLoadIdentities = `
SELECT id, user_id, provider_id, identity_type, subject, provider_account, profile_snapshot, linked_at
FROM ` + "`user_identities`" + `
WHERE user_id IN (PLACEHOLDER_IDS)
ORDER BY linked_at DESC`

	queryBatchLoadMfaMethods = `
SELECT id, user_id, method_type, enabled, verified, display_name, last_used_at
FROM ` + "`user_mfa_methods`" + `
WHERE user_id IN (PLACEHOLDER_IDS)
ORDER BY method_type ASC`
)

// 方言敏感 UPSERT — 经 mysqlshared.Dialect.Upsert 生成
var queryUpsertUserProfile = `INSERT INTO ` + "`user_profiles`" + ` (user_id, display_name, first_name, last_name, avatar_url, email, email_verified, phone, phone_verified, country_code, region, location, birthday, gender, bio, real_name, id_card_type, id_card, passport_no, driver_license_no, id_card_expiry, issue_country, issuing_authority, passport_photo_url, driver_license_front_url, driver_license_back_url, properties) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{"display_name", "first_name", "last_name", "avatar_url", "email", "email_verified", "phone", "phone_verified", "country_code", "region", "location", "birthday", "gender", "bio", "real_name", "id_card_type", "id_card", "passport_no", "driver_license_no", "id_card_expiry", "issue_country", "issuing_authority", "passport_photo_url", "driver_license_front_url", "driver_license_back_url", "properties", columnUpdatedAt}}, nil) + `
`

var queryUpsertUserCredentials = `INSERT INTO ` + "`user_credentials`" + ` (user_id, password_hash, password_algo, password_updated_at, access_key, access_secret_hash, access_secret_rotated_at, need_update_password, access_secret_hash_prev, access_secret_prev_expires_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{UpdateColumns: []string{"password_hash", "password_algo", "password_updated_at", "access_key", "access_secret_hash", "access_secret_rotated_at", "need_update_password", "access_secret_hash_prev", "access_secret_prev_expires_at", columnUpdatedAt}}, nil) + `
`

// 定向更新口令列：绝不触碰 access_key / access_secret_hash / access_secret_rotated_at。
var queryUpdatePasswordCredential = `INSERT INTO ` + "`user_credentials`" + ` (password_hash, password_algo, password_updated_at, need_update_password, user_id) VALUES (?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), password_algo = VALUES(password_algo), password_updated_at = VALUES(password_updated_at), need_update_password = VALUES(need_update_password)`
