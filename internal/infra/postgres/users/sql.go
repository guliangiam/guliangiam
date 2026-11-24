package users

const (
	queryGetUserByID = `
SELECT u.id, u.organization_id, u.username, COALESCE(u.external_id, ''), u.user_type, u.status, u.is_admin, u.is_forbidden, COALESCE(u.is_verified, false), COALESCE(u.is_online, false), COALESCE(u.created_ip::text, ''), COALESCE(u.signup_application_id::text, ''), COALESCE(u.register_type, ''), COALESCE(u.register_source, ''), COALESCE(u.tag, ''), COALESCE(u.score, 0), COALESCE(st.signin_wrong_times, 0), st.last_signin_wrong_time, COALESCE(mc.preferred_mfa_type, ''), COALESCE(mc.recovery_code_hashes, '{}'::text[]), COALESCE(mc.totp_secret, ''), COALESCE(mc.mfa_radius_username, ''), COALESCE(mc.mfa_radius_provider, ''), COALESCE(mc.mfa_push_receiver, ''), COALESCE(mc.mfa_push_provider, ''), COALESCE(mc.mfa_push_enabled, false), mc.mfa_remember_deadline, st.last_signin_at, COALESCE(st.last_signin_ip::text, ''), COALESCE(fin.balance, 0), COALESCE(fin.balance_credit, 0), COALESCE(fin.balance_currency, 'USD'), COALESCE(u.is_deleted, false), COALESCE(u.ip_whitelist, ''), COALESCE(col.addresses, '[]'::jsonb), COALESCE(col.managed_accounts, '[]'::jsonb), COALESCE(col.face_ids, '[]'::jsonb), COALESCE(col.cart, '[]'::jsonb), COALESCE(col.mfa_items, '[]'::jsonb), COALESCE(col.mfa_accounts, '[]'::jsonb), COALESCE(col.application_scopes, '[]'::jsonb), COALESCE(ca.custom1, ''), COALESCE(ca.custom2, ''), COALESCE(ca.custom3, ''), COALESCE(ca.custom4, ''), COALESCE(ca.custom5, ''), COALESCE(ca.custom6, ''), COALESCE(ca.custom7, ''), COALESCE(ca.custom8, ''), COALESCE(ca.custom9, ''), COALESCE(ca.custom10, ''), u.created_at, u.updated_at, u.deleted_at, u.revision
FROM guliangiam.users u
LEFT JOIN guliangiam.user_signin_tracking st ON st.user_id = u.id
LEFT JOIN guliangiam.user_mfa_config mc ON mc.user_id = u.id
LEFT JOIN guliangiam.user_financials fin ON fin.user_id = u.id
LEFT JOIN guliangiam.user_collections col ON col.user_id = u.id
LEFT JOIN guliangiam.user_custom_attributes ca ON ca.user_id = u.id
WHERE u.id = $1::uuid AND u.deleted_at IS NULL`

	// queryGetUserByIDForOrganization 在 by-ID 读上追加租户谓词；跨租户行与不存在行
	// 一样落到 ErrNoRows → NotFound，调用方无法探测其他组织是否存在该 id。
	queryGetUserByIDForOrganization = `
SELECT u.id, u.organization_id, u.username, COALESCE(u.external_id, ''), u.user_type, u.status, u.is_admin, u.is_forbidden, COALESCE(u.is_verified, false), COALESCE(u.is_online, false), COALESCE(u.created_ip::text, ''), COALESCE(u.signup_application_id::text, ''), COALESCE(u.register_type, ''), COALESCE(u.register_source, ''), COALESCE(u.tag, ''), COALESCE(u.score, 0), COALESCE(st.signin_wrong_times, 0), st.last_signin_wrong_time, COALESCE(mc.preferred_mfa_type, ''), COALESCE(mc.recovery_code_hashes, '{}'::text[]), COALESCE(mc.totp_secret, ''), COALESCE(mc.mfa_radius_username, ''), COALESCE(mc.mfa_radius_provider, ''), COALESCE(mc.mfa_push_receiver, ''), COALESCE(mc.mfa_push_provider, ''), COALESCE(mc.mfa_push_enabled, false), mc.mfa_remember_deadline, st.last_signin_at, COALESCE(st.last_signin_ip::text, ''), COALESCE(fin.balance, 0), COALESCE(fin.balance_credit, 0), COALESCE(fin.balance_currency, 'USD'), COALESCE(u.is_deleted, false), COALESCE(u.ip_whitelist, ''), COALESCE(col.addresses, '[]'::jsonb), COALESCE(col.managed_accounts, '[]'::jsonb), COALESCE(col.face_ids, '[]'::jsonb), COALESCE(col.cart, '[]'::jsonb), COALESCE(col.mfa_items, '[]'::jsonb), COALESCE(col.mfa_accounts, '[]'::jsonb), COALESCE(col.application_scopes, '[]'::jsonb), COALESCE(ca.custom1, ''), COALESCE(ca.custom2, ''), COALESCE(ca.custom3, ''), COALESCE(ca.custom4, ''), COALESCE(ca.custom5, ''), COALESCE(ca.custom6, ''), COALESCE(ca.custom7, ''), COALESCE(ca.custom8, ''), COALESCE(ca.custom9, ''), COALESCE(ca.custom10, ''), u.created_at, u.updated_at, u.deleted_at, u.revision
FROM guliangiam.users u
LEFT JOIN guliangiam.user_signin_tracking st ON st.user_id = u.id
LEFT JOIN guliangiam.user_mfa_config mc ON mc.user_id = u.id
LEFT JOIN guliangiam.user_financials fin ON fin.user_id = u.id
LEFT JOIN guliangiam.user_collections col ON col.user_id = u.id
LEFT JOIN guliangiam.user_custom_attributes ca ON ca.user_id = u.id
WHERE u.id = $1::uuid AND u.organization_id = guliangiam.resolve_org_id($2) AND u.deleted_at IS NULL`

	// queryLockUserByID 取用户行的行级锁（FOR UPDATE），直到事务提交/回滚。
	// 「会话计数 + 淘汰」必须与它同事务，否则并发登录会各自读到未超限（《开发设计》22.5）。
	// 锁锚点用用户行而不是会话行：首次登录时会话行还不存在，锁不存在的行不互斥。
	queryLockUserByID = `
SELECT id::text FROM guliangiam.users
WHERE id = $1::uuid AND deleted_at IS NULL
FOR UPDATE`

	queryGetUserByUsername = `
SELECT u.id, u.organization_id, u.username, COALESCE(u.external_id, ''), u.user_type, u.status, u.is_admin, u.is_forbidden, COALESCE(u.is_verified, false), COALESCE(u.is_online, false), COALESCE(u.created_ip::text, ''), COALESCE(u.signup_application_id::text, ''), COALESCE(u.register_type, ''), COALESCE(u.register_source, ''), COALESCE(u.tag, ''), COALESCE(u.score, 0), COALESCE(st.signin_wrong_times, 0), st.last_signin_wrong_time, COALESCE(mc.preferred_mfa_type, ''), COALESCE(mc.recovery_code_hashes, '{}'::text[]), COALESCE(mc.totp_secret, ''), COALESCE(mc.mfa_radius_username, ''), COALESCE(mc.mfa_radius_provider, ''), COALESCE(mc.mfa_push_receiver, ''), COALESCE(mc.mfa_push_provider, ''), COALESCE(mc.mfa_push_enabled, false), mc.mfa_remember_deadline, st.last_signin_at, COALESCE(st.last_signin_ip::text, ''), COALESCE(fin.balance, 0), COALESCE(fin.balance_credit, 0), COALESCE(fin.balance_currency, 'USD'), COALESCE(u.is_deleted, false), COALESCE(u.ip_whitelist, ''), COALESCE(col.addresses, '[]'::jsonb), COALESCE(col.managed_accounts, '[]'::jsonb), COALESCE(col.face_ids, '[]'::jsonb), COALESCE(col.cart, '[]'::jsonb), COALESCE(col.mfa_items, '[]'::jsonb), COALESCE(col.mfa_accounts, '[]'::jsonb), COALESCE(col.application_scopes, '[]'::jsonb), COALESCE(ca.custom1, ''), COALESCE(ca.custom2, ''), COALESCE(ca.custom3, ''), COALESCE(ca.custom4, ''), COALESCE(ca.custom5, ''), COALESCE(ca.custom6, ''), COALESCE(ca.custom7, ''), COALESCE(ca.custom8, ''), COALESCE(ca.custom9, ''), COALESCE(ca.custom10, ''), u.created_at, u.updated_at, u.deleted_at, u.revision
FROM guliangiam.users u
LEFT JOIN guliangiam.user_signin_tracking st ON st.user_id = u.id
LEFT JOIN guliangiam.user_mfa_config mc ON mc.user_id = u.id
LEFT JOIN guliangiam.user_financials fin ON fin.user_id = u.id
LEFT JOIN guliangiam.user_collections col ON col.user_id = u.id
LEFT JOIN guliangiam.user_custom_attributes ca ON ca.user_id = u.id
WHERE u.organization_id = guliangiam.resolve_org_id($1) AND u.username = $2 AND u.deleted_at IS NULL`

	// queryGetUserByLoginIdentifier 按「账号标识」解析登录主体：用户名精确匹配 >
	// 邮箱忽略大小写 > 手机号精确匹配。租户谓词始终生效，因此全局唯一的邮箱也
	// 不会被跨租户解析。多行结果由仓储侧裁决（见 GetByLoginIdentifier）。
	queryGetUserByLoginIdentifier = `
SELECT u.id, u.organization_id, u.username, COALESCE(u.external_id, ''), u.user_type, u.status, u.is_admin, u.is_forbidden, COALESCE(u.is_verified, false), COALESCE(u.is_online, false), COALESCE(u.created_ip::text, ''), COALESCE(u.signup_application_id::text, ''), COALESCE(u.register_type, ''), COALESCE(u.register_source, ''), COALESCE(u.tag, ''), COALESCE(u.score, 0), COALESCE(st.signin_wrong_times, 0), st.last_signin_wrong_time, COALESCE(mc.preferred_mfa_type, ''), COALESCE(mc.recovery_code_hashes, '{}'::text[]), COALESCE(mc.totp_secret, ''), COALESCE(mc.mfa_radius_username, ''), COALESCE(mc.mfa_radius_provider, ''), COALESCE(mc.mfa_push_receiver, ''), COALESCE(mc.mfa_push_provider, ''), COALESCE(mc.mfa_push_enabled, false), mc.mfa_remember_deadline, st.last_signin_at, COALESCE(st.last_signin_ip::text, ''), COALESCE(fin.balance, 0), COALESCE(fin.balance_credit, 0), COALESCE(fin.balance_currency, 'USD'), COALESCE(u.is_deleted, false), COALESCE(u.ip_whitelist, ''), COALESCE(col.addresses, '[]'::jsonb), COALESCE(col.managed_accounts, '[]'::jsonb), COALESCE(col.face_ids, '[]'::jsonb), COALESCE(col.cart, '[]'::jsonb), COALESCE(col.mfa_items, '[]'::jsonb), COALESCE(col.mfa_accounts, '[]'::jsonb), COALESCE(col.application_scopes, '[]'::jsonb), COALESCE(ca.custom1, ''), COALESCE(ca.custom2, ''), COALESCE(ca.custom3, ''), COALESCE(ca.custom4, ''), COALESCE(ca.custom5, ''), COALESCE(ca.custom6, ''), COALESCE(ca.custom7, ''), COALESCE(ca.custom8, ''), COALESCE(ca.custom9, ''), COALESCE(ca.custom10, ''), u.created_at, u.updated_at, u.deleted_at, u.revision
FROM guliangiam.users u
LEFT JOIN guliangiam.user_signin_tracking st ON st.user_id = u.id
LEFT JOIN guliangiam.user_mfa_config mc ON mc.user_id = u.id
LEFT JOIN guliangiam.user_financials fin ON fin.user_id = u.id
LEFT JOIN guliangiam.user_collections col ON col.user_id = u.id
LEFT JOIN guliangiam.user_custom_attributes ca ON ca.user_id = u.id
LEFT JOIN guliangiam.user_profiles p ON p.user_id = u.id AND p.deleted_at IS NULL
WHERE u.organization_id = guliangiam.resolve_org_id($1) AND u.deleted_at IS NULL
  AND (u.username = $2 OR lower(p.email) = lower($2) OR p.phone = $2)
LIMIT 5`

	queryListUsers = `
SELECT u.id, u.organization_id, u.username, COALESCE(u.external_id, ''), u.user_type, u.status, u.is_admin, u.is_forbidden, COALESCE(u.is_verified, false), COALESCE(u.is_online, false), COALESCE(u.created_ip::text, ''), COALESCE(u.signup_application_id::text, ''), COALESCE(u.register_type, ''), COALESCE(u.register_source, ''), COALESCE(u.tag, ''), COALESCE(u.score, 0), COALESCE(st.signin_wrong_times, 0), st.last_signin_wrong_time, COALESCE(mc.preferred_mfa_type, ''), COALESCE(mc.recovery_code_hashes, '{}'::text[]), COALESCE(mc.totp_secret, ''), COALESCE(mc.mfa_radius_username, ''), COALESCE(mc.mfa_radius_provider, ''), COALESCE(mc.mfa_push_receiver, ''), COALESCE(mc.mfa_push_provider, ''), COALESCE(mc.mfa_push_enabled, false), mc.mfa_remember_deadline, st.last_signin_at, COALESCE(st.last_signin_ip::text, ''), COALESCE(fin.balance, 0), COALESCE(fin.balance_credit, 0), COALESCE(fin.balance_currency, 'USD'), COALESCE(u.is_deleted, false), COALESCE(u.ip_whitelist, ''), COALESCE(col.addresses, '[]'::jsonb), COALESCE(col.managed_accounts, '[]'::jsonb), COALESCE(col.face_ids, '[]'::jsonb), COALESCE(col.cart, '[]'::jsonb), COALESCE(col.mfa_items, '[]'::jsonb), COALESCE(col.mfa_accounts, '[]'::jsonb), COALESCE(col.application_scopes, '[]'::jsonb), COALESCE(ca.custom1, ''), COALESCE(ca.custom2, ''), COALESCE(ca.custom3, ''), COALESCE(ca.custom4, ''), COALESCE(ca.custom5, ''), COALESCE(ca.custom6, ''), COALESCE(ca.custom7, ''), COALESCE(ca.custom8, ''), COALESCE(ca.custom9, ''), COALESCE(ca.custom10, ''), u.created_at, u.updated_at, u.deleted_at, u.revision
FROM guliangiam.users u
LEFT JOIN guliangiam.user_signin_tracking st ON st.user_id = u.id
LEFT JOIN guliangiam.user_mfa_config mc ON mc.user_id = u.id
LEFT JOIN guliangiam.user_financials fin ON fin.user_id = u.id
LEFT JOIN guliangiam.user_collections col ON col.user_id = u.id
LEFT JOIN guliangiam.user_custom_attributes ca ON ca.user_id = u.id
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE u.organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR u.username ILIKE '%' || $2 || '%' OR u.external_id ILIKE '%' || $2 || '%')
  AND u.deleted_at IS NULL
  AND ($3 = '' OR u.status = $3)
  AND ($4 = '' OR u.user_type = $4)
  AND ($5::boolean IS NULL OR u.is_admin = $5::boolean)`

	queryCountUsers = `
SELECT COUNT(*)
FROM guliangiam.users u
LEFT JOIN guliangiam.user_signin_tracking st ON st.user_id = u.id
LEFT JOIN guliangiam.user_mfa_config mc ON mc.user_id = u.id
LEFT JOIN guliangiam.user_financials fin ON fin.user_id = u.id
LEFT JOIN guliangiam.user_collections col ON col.user_id = u.id
LEFT JOIN guliangiam.user_custom_attributes ca ON ca.user_id = u.id
WHERE (CASE WHEN $1 = '' THEN TRUE ELSE u.organization_id = guliangiam.resolve_org_id($1) END)
  AND ($2 = '' OR u.username ILIKE '%' || $2 || '%' OR u.external_id ILIKE '%' || $2 || '%')
  AND u.deleted_at IS NULL
  AND ($3 = '' OR u.status = $3)
  AND ($4 = '' OR u.user_type = $4)
  AND ($5::boolean IS NULL OR u.is_admin = $5::boolean)`

	queryGetUserProfileByUserID = `
SELECT user_id, COALESCE(display_name, ''), COALESCE(first_name, ''), COALESCE(last_name, ''), COALESCE(avatar_url, ''), COALESCE(email, ''), email_verified, COALESCE(phone, ''), phone_verified, COALESCE(country_code, ''), COALESCE(region, ''), COALESCE(location, ''), COALESCE(birthday::text, ''), COALESCE(gender, ''), COALESCE(bio, ''), COALESCE(real_name, ''), COALESCE(id_card_type, ''), COALESCE(id_card, ''), COALESCE(passport_no, ''), COALESCE(driver_license_no, ''), id_card_sealed, passport_no_sealed, driver_license_no_sealed, COALESCE(id_card_expiry::text, ''), COALESCE(issue_country, ''), COALESCE(issuing_authority, ''), COALESCE(passport_photo_url, ''), COALESCE(driver_license_front_url, ''), COALESCE(driver_license_back_url, ''), properties
FROM guliangiam.user_profiles
WHERE user_id = $1::uuid`

	queryGetUserCredentialsByUserID = `
SELECT user_id, password_hash, password_algo, password_updated_at, access_key, access_secret_hash, access_secret_rotated_at, need_update_password, COALESCE(access_secret_hash_prev, ''), access_secret_prev_expires_at
FROM guliangiam.user_credentials
WHERE user_id = $1::uuid`

	queryListUserIdentitiesByUserID = `
SELECT id, user_id, provider_id, identity_type, subject, provider_account, profile_snapshot, linked_at
FROM guliangiam.user_identities
WHERE user_id = $1::uuid
ORDER BY linked_at DESC`

	queryListUserMfaMethodsByUserID = `
SELECT id, user_id, method_type, enabled, verified, display_name, last_used_at
FROM guliangiam.user_mfa_methods
WHERE user_id = $1::uuid
ORDER BY method_type ASC`

	queryInsertUserCore = `INSERT INTO guliangiam.users (id, organization_id, username, external_id, user_type, status, is_admin, is_forbidden, is_verified, is_online, created_ip, signup_application_id, register_type, register_source, tag, score, is_deleted, ip_whitelist, created_at, updated_at, deleted_at, revision) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::uuid, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`
	queryUpdateUserCore = `UPDATE guliangiam.users
SET external_id = $1,
    user_type = $2,
    status = $3,
    is_admin = $4,
    is_forbidden = $5,
    is_verified = $6,
    is_online = $7,
    created_ip = $8,
    signup_application_id = $9::uuid,
    register_type = $10,
    register_source = $11,
    tag = $12,
    score = $13,
    is_deleted = $14,
    ip_whitelist = $15,
    updated_at = $16,
    deleted_at = $17,
    revision = revision + 1
  WHERE id = $19::uuid AND revision = $18 RETURNING revision`

	// queryUpdateUserCoreForOrganization 与 queryUpdateUserCore 同构，追加
	// organization_id 谓词（$20）：跨租户 UPDATE 影响 0 行，由仓储映射为 NotFound。
	queryUpdateUserCoreForOrganization = `UPDATE guliangiam.users
SET external_id = $1,
    user_type = $2,
    status = $3,
    is_admin = $4,
    is_forbidden = $5,
    is_verified = $6,
    is_online = $7,
    created_ip = $8,
    signup_application_id = $9::uuid,
    register_type = $10,
    register_source = $11,
    tag = $12,
    score = $13,
    is_deleted = $14,
    ip_whitelist = $15,
    updated_at = $16,
    deleted_at = $17,
    revision = revision + 1
  WHERE id = $19::uuid AND revision = $18 AND organization_id = guliangiam.resolve_org_id($20) AND deleted_at IS NULL`

	// queryUserExistsForOrganization 只在 scoped UPDATE 影响 0 行后被调用，用来区分
	// 「同租户但 revision 过期」（Conflict）与「跨租户/不存在」（NotFound）。谓词与
	// UPDATE 完全同构，因此不会泄露其他组织的行是否存在。
	queryUserExistsForOrganization = `
SELECT 1 FROM guliangiam.users
WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`
	queryUpsertUserSigninTracking = `INSERT INTO guliangiam.user_signin_tracking (user_id, signin_wrong_times, last_signin_wrong_time, last_signin_at, last_signin_ip) VALUES ($1::uuid, $2, $3, $4, $5)
ON CONFLICT (user_id)
DO UPDATE SET
    signin_wrong_times = EXCLUDED.signin_wrong_times,
    last_signin_wrong_time = EXCLUDED.last_signin_wrong_time,
    last_signin_at = EXCLUDED.last_signin_at,
    last_signin_ip = EXCLUDED.last_signin_ip`
	queryUpsertUserMfaConfig = `INSERT INTO guliangiam.user_mfa_config (user_id, preferred_mfa_type, recovery_code_hashes, totp_secret, mfa_radius_username, mfa_radius_provider, mfa_push_receiver, mfa_push_provider, mfa_push_enabled, mfa_remember_deadline) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id)
DO UPDATE SET
    preferred_mfa_type = EXCLUDED.preferred_mfa_type,
    recovery_code_hashes = EXCLUDED.recovery_code_hashes,
    totp_secret = EXCLUDED.totp_secret,
    mfa_radius_username = EXCLUDED.mfa_radius_username,
    mfa_radius_provider = EXCLUDED.mfa_radius_provider,
    mfa_push_receiver = EXCLUDED.mfa_push_receiver,
    mfa_push_provider = EXCLUDED.mfa_push_provider,
    mfa_push_enabled = EXCLUDED.mfa_push_enabled,
    mfa_remember_deadline = EXCLUDED.mfa_remember_deadline`
	queryUpsertUserFinancials = `INSERT INTO guliangiam.user_financials (user_id, balance, balance_credit, balance_currency) VALUES ($1::uuid, $2, $3, $4)
ON CONFLICT (user_id)
DO UPDATE SET
    balance = EXCLUDED.balance,
    balance_credit = EXCLUDED.balance_credit,
    balance_currency = EXCLUDED.balance_currency`
	queryUpsertUserCollections = `INSERT INTO guliangiam.user_collections (user_id, addresses, managed_accounts, face_ids, cart, mfa_items, mfa_accounts, application_scopes) VALUES ($1::uuid, $2::jsonb, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb)
ON CONFLICT (user_id)
DO UPDATE SET
    addresses = EXCLUDED.addresses,
    managed_accounts = EXCLUDED.managed_accounts,
    face_ids = EXCLUDED.face_ids,
    cart = EXCLUDED.cart,
    mfa_items = EXCLUDED.mfa_items,
    mfa_accounts = EXCLUDED.mfa_accounts,
    application_scopes = EXCLUDED.application_scopes`
	queryUpdateUserContactVerified = `UPDATE guliangiam.user_profiles
SET email_verified = $3, phone_verified = $4, updated_at = now()
WHERE user_id = $1::uuid
  AND EXISTS (
      SELECT 1 FROM guliangiam.users
      WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL
  )`
	// queryUpsertUserCollectionsForOrganization 在 upsert 前追加租户存在性谓词：
	// 跨租户或已删除的用户会得到 0 行受影响，由仓储转成 NotFound。
	queryUpsertUserCollectionsForOrganization = `INSERT INTO guliangiam.user_collections (user_id, addresses, managed_accounts, face_ids, cart, mfa_items, mfa_accounts, application_scopes)
SELECT $1::uuid, $2::jsonb, $3::jsonb, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, $8::jsonb
WHERE EXISTS (
    SELECT 1 FROM guliangiam.users
    WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($9) AND deleted_at IS NULL
)
ON CONFLICT (user_id)
DO UPDATE SET
    addresses = EXCLUDED.addresses,
    managed_accounts = EXCLUDED.managed_accounts,
    face_ids = EXCLUDED.face_ids,
    cart = EXCLUDED.cart,
    mfa_items = EXCLUDED.mfa_items,
    mfa_accounts = EXCLUDED.mfa_accounts,
    application_scopes = EXCLUDED.application_scopes`
	queryUpsertUserCustomAttributes = `INSERT INTO guliangiam.user_custom_attributes (user_id, custom1, custom2, custom3, custom4, custom5, custom6, custom7, custom8, custom9, custom10) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (user_id)
DO UPDATE SET
    custom1 = EXCLUDED.custom1,
    custom2 = EXCLUDED.custom2,
    custom3 = EXCLUDED.custom3,
    custom4 = EXCLUDED.custom4,
    custom5 = EXCLUDED.custom5,
    custom6 = EXCLUDED.custom6,
    custom7 = EXCLUDED.custom7,
    custom8 = EXCLUDED.custom8,
    custom9 = EXCLUDED.custom9,
    custom10 = EXCLUDED.custom10`

	queryUpsertUserProfile = `
INSERT INTO guliangiam.user_profiles (user_id, display_name, first_name, last_name, avatar_url, email, email_verified, phone, phone_verified, country_code, region, location, birthday, gender, bio, real_name, id_card_type, id_card, passport_no, driver_license_no, id_card_expiry, issue_country, issuing_authority, passport_photo_url, driver_license_front_url, driver_license_back_url, properties, id_card_sealed, passport_no_sealed, driver_license_no_sealed) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27::jsonb, $28, $29, $30)
ON CONFLICT (user_id)
DO UPDATE SET
    display_name = EXCLUDED.display_name,
    first_name = EXCLUDED.first_name,
    last_name = EXCLUDED.last_name,
    avatar_url = EXCLUDED.avatar_url,
    email = EXCLUDED.email,
    email_verified = EXCLUDED.email_verified,
    phone = EXCLUDED.phone,
    phone_verified = EXCLUDED.phone_verified,
    country_code = EXCLUDED.country_code,
    region = EXCLUDED.region,
    location = EXCLUDED.location,
    birthday = EXCLUDED.birthday,
    gender = EXCLUDED.gender,
    bio = EXCLUDED.bio,
    real_name = EXCLUDED.real_name,
    id_card_type = EXCLUDED.id_card_type,
    id_card = EXCLUDED.id_card,
    passport_no = EXCLUDED.passport_no,
    driver_license_no = EXCLUDED.driver_license_no,
    id_card_expiry = EXCLUDED.id_card_expiry,
    issue_country = EXCLUDED.issue_country,
    issuing_authority = EXCLUDED.issuing_authority,
    passport_photo_url = EXCLUDED.passport_photo_url,
    driver_license_front_url = EXCLUDED.driver_license_front_url,
    driver_license_back_url = EXCLUDED.driver_license_back_url,
    properties = EXCLUDED.properties,
    id_card_sealed = EXCLUDED.id_card_sealed,
    passport_no_sealed = EXCLUDED.passport_no_sealed,
    driver_license_no_sealed = EXCLUDED.driver_license_no_sealed`

	queryUpsertUserCredentials = `
INSERT INTO guliangiam.user_credentials (user_id, password_hash, password_algo, password_updated_at, access_key, access_secret_hash, access_secret_rotated_at, need_update_password, access_secret_hash_prev, access_secret_prev_expires_at) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (user_id)
DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    password_algo = EXCLUDED.password_algo,
    password_updated_at = EXCLUDED.password_updated_at,
    access_key = EXCLUDED.access_key,
    access_secret_hash = EXCLUDED.access_secret_hash,
    access_secret_rotated_at = EXCLUDED.access_secret_rotated_at,
    need_update_password = EXCLUDED.need_update_password,
    access_secret_hash_prev = EXCLUDED.access_secret_hash_prev,
    access_secret_prev_expires_at = EXCLUDED.access_secret_prev_expires_at`

	// 定向更新口令列：绝不触碰 access_key / access_secret_hash /
	// access_secret_rotated_at，避免自助改密把既有访问密钥写成空值。
	queryUpdatePasswordCredential = `
INSERT INTO guliangiam.user_credentials
    (user_id, password_hash, password_algo, password_updated_at, need_update_password)
VALUES ($1::uuid, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    password_algo = EXCLUDED.password_algo,
    password_updated_at = EXCLUDED.password_updated_at,
    need_update_password = EXCLUDED.need_update_password`

	queryDeleteUserIdentities = `DELETE FROM guliangiam.user_identities WHERE user_id = $1::uuid`
	queryInsertUserIdentity   = `INSERT INTO guliangiam.user_identities (id, user_id, provider_id, identity_type, subject, provider_account, profile_snapshot, linked_at) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::jsonb, $8)`
	queryDeleteSingleIdentity = `DELETE FROM guliangiam.user_identities WHERE user_id = $1::uuid AND id = $2`

	queryDeleteUserMfaMethods = `DELETE FROM guliangiam.user_mfa_methods WHERE user_id = $1::uuid`
	queryInsertUserMfaMethod  = `INSERT INTO guliangiam.user_mfa_methods (id, user_id, method_type, enabled, verified, display_name, last_used_at) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)`

	queryDeleteUserRoleBindings = `DELETE FROM guliangiam.user_role_bindings WHERE user_id = $1::uuid`
	queryInsertUserRoleBinding  = `INSERT INTO guliangiam.user_role_bindings (user_id, role_id) VALUES ($1::uuid, $2::uuid)`

	queryInsertUserRoleBindingIgnore = `INSERT INTO guliangiam.user_role_bindings (user_id, role_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`
	queryDeleteSingleUserRoleBinding = `DELETE FROM guliangiam.user_role_bindings WHERE user_id = $1::uuid AND role_id = $2::uuid`
	queryListRoleBindingsByUser      = `SELECT user_id, role_id FROM guliangiam.user_role_bindings WHERE user_id = $1::uuid`

	queryDeleteUserGroupMemberships = `DELETE FROM guliangiam.user_group_memberships WHERE user_id = $1::uuid`
	queryInsertUserGroupMembership  = `INSERT INTO guliangiam.user_group_memberships (user_id, group_id) VALUES ($1::uuid, $2::uuid)`

	queryDeleteUser = `UPDATE guliangiam.users SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1::uuid AND organization_id = guliangiam.resolve_org_id($2) AND deleted_at IS NULL`

	queryListRoleBindingsByUserIDs = `SELECT user_id, role_id FROM guliangiam.user_role_bindings WHERE user_id = ANY($1::uuid[])`

	queryListRoleNamesForUser = `
SELECT r.name
FROM guliangiam.user_role_bindings ur
JOIN guliangiam.roles r ON ur.role_id = r.id
WHERE ur.user_id = $1::uuid AND r.is_enabled = true
ORDER BY r.name`

	queryListGroupMembershipsByUserIDs = `SELECT user_id, group_id FROM guliangiam.user_group_memberships WHERE user_id = ANY($1::uuid[])`

	queryGetPasswordHistory = `
SELECT id, user_id, password_hash, password_algo, created_at
FROM guliangiam.user_password_history
WHERE user_id = $1::uuid
ORDER BY created_at DESC
LIMIT $2`

	queryInsertPasswordHistory = `
INSERT INTO guliangiam.user_password_history (id, user_id, password_hash, password_algo, created_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5)`

	queryDeletePasswordHistoryBeyond = `
DELETE FROM guliangiam.user_password_history
WHERE user_id = $1::uuid
  AND id NOT IN (
    SELECT id FROM guliangiam.user_password_history
    WHERE user_id = $1::uuid
    ORDER BY created_at DESC
    LIMIT $2
  )`

	queryGetUserByProviderIdentity = `
	SELECT u.id, u.organization_id, u.username, COALESCE(u.external_id, ''), u.user_type, u.status, u.is_admin, u.is_forbidden, COALESCE(u.is_verified, false), COALESCE(u.is_online, false), COALESCE(u.created_ip::text, ''), COALESCE(u.signup_application_id::text, ''), COALESCE(u.register_type, ''), COALESCE(u.register_source, ''), COALESCE(u.tag, ''), COALESCE(u.score, 0), COALESCE(st.signin_wrong_times, 0), st.last_signin_wrong_time, COALESCE(mc.preferred_mfa_type, ''), COALESCE(mc.recovery_code_hashes, '{}'::text[]), COALESCE(mc.totp_secret, ''), COALESCE(mc.mfa_radius_username, ''), COALESCE(mc.mfa_radius_provider, ''), COALESCE(mc.mfa_push_receiver, ''), COALESCE(mc.mfa_push_provider, ''), COALESCE(mc.mfa_push_enabled, false), mc.mfa_remember_deadline, st.last_signin_at, COALESCE(st.last_signin_ip::text, ''), COALESCE(fin.balance, 0), COALESCE(fin.balance_credit, 0), COALESCE(fin.balance_currency, 'USD'), COALESCE(u.is_deleted, false), COALESCE(u.ip_whitelist, ''), COALESCE(col.addresses, '[]'::jsonb), COALESCE(col.managed_accounts, '[]'::jsonb), COALESCE(col.face_ids, '[]'::jsonb), COALESCE(col.cart, '[]'::jsonb), COALESCE(col.mfa_items, '[]'::jsonb), COALESCE(col.mfa_accounts, '[]'::jsonb), COALESCE(col.application_scopes, '[]'::jsonb), COALESCE(ca.custom1, ''), COALESCE(ca.custom2, ''), COALESCE(ca.custom3, ''), COALESCE(ca.custom4, ''), COALESCE(ca.custom5, ''), COALESCE(ca.custom6, ''), COALESCE(ca.custom7, ''), COALESCE(ca.custom8, ''), COALESCE(ca.custom9, ''), COALESCE(ca.custom10, ''), u.created_at, u.updated_at, u.deleted_at, u.revision
	FROM guliangiam.users u
LEFT JOIN guliangiam.user_signin_tracking st ON st.user_id = u.id
LEFT JOIN guliangiam.user_mfa_config mc ON mc.user_id = u.id
LEFT JOIN guliangiam.user_financials fin ON fin.user_id = u.id
LEFT JOIN guliangiam.user_collections col ON col.user_id = u.id
LEFT JOIN guliangiam.user_custom_attributes ca ON ca.user_id = u.id
	INNER JOIN guliangiam.user_identities ui ON ui.user_id = u.id
    WHERE ui.provider_id = $1 AND ui.subject = $2 AND u.deleted_at IS NULL`

	queryBatchLoadCredentials = `
SELECT user_id, password_hash, password_algo, password_updated_at, access_key, access_secret_hash, access_secret_rotated_at, need_update_password, COALESCE(access_secret_hash_prev, ''), access_secret_prev_expires_at
FROM guliangiam.user_credentials
WHERE user_id = ANY($1::uuid[])`

	queryBatchLoadIdentities = `
SELECT id, user_id, provider_id, identity_type, subject, provider_account, profile_snapshot, linked_at
FROM guliangiam.user_identities
WHERE user_id = ANY($1::uuid[])
ORDER BY linked_at DESC`

	queryBatchLoadMfaMethods = `
SELECT id, user_id, method_type, enabled, verified, display_name, last_used_at
FROM guliangiam.user_mfa_methods
WHERE user_id = ANY($1::uuid[])
ORDER BY method_type ASC`
)
