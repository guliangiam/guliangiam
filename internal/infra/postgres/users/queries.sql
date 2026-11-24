-- name: GetUserByID :one
SELECT
    id,
    organization_id,
    username,
    external_id,
    user_type,
    status,
    is_admin,
    is_forbidden,
    signup_application_id,
    register_source,
    COALESCE(preferred_mfa_type, ''),
    COALESCE(recovery_code_hashes, '{}'::text[]),
    COALESCE(totp_secret, ''),
    mfa_remember_deadline,
    last_signin_at,
    last_signin_ip,
    created_at,
    updated_at,
    deleted_at,
    revision
FROM guliangiam.users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT
    id,
    organization_id,
    username,
    external_id,
    user_type,
    status,
    is_admin,
    is_forbidden,
    signup_application_id,
    register_source,
    COALESCE(preferred_mfa_type, ''),
    COALESCE(recovery_code_hashes, '{}'::text[]),
    COALESCE(totp_secret, ''),
    mfa_remember_deadline,
    last_signin_at,
    last_signin_ip,
    created_at,
    updated_at,
    deleted_at,
    revision
FROM guliangiam.users
WHERE organization_id = $1
  AND username = $2;

-- name: GetUserByEmail :one
SELECT
    u.id,
    u.organization_id,
    u.username,
    u.external_id,
    u.user_type,
    u.status,
    u.is_admin,
    u.is_forbidden,
    u.signup_application_id,
    u.register_source,
    COALESCE(u.preferred_mfa_type, ''),
    COALESCE(u.recovery_code_hashes, '{}'::text[]),
    COALESCE(u.totp_secret, ''),
    u.mfa_remember_deadline,
    u.last_signin_at,
    u.last_signin_ip,
    u.created_at,
    u.updated_at,
    u.deleted_at,
    u.revision
FROM guliangiam.users u
JOIN guliangiam.user_profiles p ON p.user_id = u.id
WHERE u.organization_id = $1
  AND p.email = $2;

-- name: ListUsers :many
SELECT
    id,
    organization_id,
    username,
    external_id,
    user_type,
    status,
    is_admin,
    is_forbidden,
    signup_application_id,
    register_source,
    COALESCE(preferred_mfa_type, ''),
    COALESCE(recovery_code_hashes, '{}'::text[]),
    COALESCE(totp_secret, ''),
    mfa_remember_deadline,
    last_signin_at,
    last_signin_ip,
    created_at,
    updated_at,
    deleted_at,
    revision
FROM guliangiam.users
WHERE ($1 = '' OR organization_id = $1)
  AND ($2 = '' OR username ILIKE '%' || $2 || '%' OR external_id ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;

-- name: CountUsers :one
SELECT COUNT(*)
FROM guliangiam.users
WHERE ($1 = '' OR organization_id = $1)
  AND ($2 = '' OR username ILIKE '%' || $2 || '%' OR external_id ILIKE '%' || $2 || '%')
  AND ($3 = '' OR status = $3);

-- name: GetUserProfileByUserID :one
SELECT
    user_id,
    display_name,
    first_name,
    last_name,
    avatar_url,
    email,
    email_verified,
    phone,
    phone_verified,
    country_code,
    region,
    location,
    birthday,
    gender,
    bio,
    real_name,
    id_card_type,
    id_card,
    passport_no,
    driver_license_no,
    id_card_expiry,
    issue_country,
    issuing_authority,
    passport_photo_url,
    driver_license_front_url,
    driver_license_back_url,
    properties
FROM guliangiam.user_profiles
WHERE user_id = $1;

-- name: GetUserCredentialsByUserID :one
SELECT
    user_id,
    password_hash,
    password_algo,
    password_updated_at,
    access_key,
    access_secret_hash,
    access_secret_rotated_at,
    need_update_password
FROM guliangiam.user_credentials
WHERE user_id = $1;

-- name: ListUserIdentitiesByUserID :many
SELECT
    id,
    user_id,
    provider_id,
    identity_type,
    subject,
    provider_account,
    profile_snapshot,
    linked_at
FROM guliangiam.user_identities
WHERE user_id = $1
ORDER BY linked_at DESC;

-- name: ListUserMfaMethodsByUserID :many
SELECT
    id,
    user_id,
    method_type,
    enabled,
    verified,
    display_name,
    last_used_at
FROM guliangiam.user_mfa_methods
WHERE user_id = $1
ORDER BY method_type ASC;

-- name: InsertUser :exec
INSERT INTO guliangiam.users (
    id,
    organization_id,
    username,
    external_id,
    user_type,
    status,
    is_admin,
    is_forbidden,
    signup_application_id,
    register_source,
    preferred_mfa_type,
    recovery_code_hashes,
    totp_secret,
    mfa_remember_deadline,
    last_signin_at,
    last_signin_ip,
    created_at,
    updated_at,
    deleted_at,
    revision
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::jsonb, $13, $14, $15, $16, $17, $18, $19, $20);

-- name: UpdateUser :exec
UPDATE guliangiam.users
SET external_id = $1,
    user_type = $2,
    status = $3,
    is_admin = $4,
    is_forbidden = $5,
    signup_application_id = $6,
    register_source = $7,
    preferred_mfa_type = $8,
    recovery_code_hashes = $9::jsonb,
    totp_secret = $10,
    mfa_remember_deadline = $11,
    last_signin_at = $12,
    last_signin_ip = $13,
    updated_at = $14,
    deleted_at = $15,
    revision = $16
WHERE id = $17;

-- name: UpsertUserProfile :exec
INSERT INTO guliangiam.user_profiles (
    user_id,
    display_name,
    first_name,
    last_name,
    avatar_url,
    email,
    email_verified,
    phone,
    phone_verified,
    country_code,
    region,
    location,
    birthday,
    gender,
    bio,
    real_name,
    id_card_type,
    id_card,
    passport_no,
    driver_license_no,
    id_card_expiry,
    issue_country,
    issuing_authority,
    passport_photo_url,
    driver_license_front_url,
    driver_license_back_url,
    properties
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27::jsonb)
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
    properties = EXCLUDED.properties;

-- name: UpsertUserCredentials :exec
INSERT INTO guliangiam.user_credentials (
    user_id,
    password_hash,
    password_algo,
    password_updated_at,
    access_key,
    access_secret_hash,
    access_secret_rotated_at,
    need_update_password
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (user_id)
DO UPDATE SET
    password_hash = EXCLUDED.password_hash,
    password_algo = EXCLUDED.password_algo,
    password_updated_at = EXCLUDED.password_updated_at,
    access_key = EXCLUDED.access_key,
    access_secret_hash = EXCLUDED.access_secret_hash,
    access_secret_rotated_at = EXCLUDED.access_secret_rotated_at,
    need_update_password = EXCLUDED.need_update_password;

-- name: DeleteUserMfaMethods :exec
DELETE FROM guliangiam.user_mfa_methods WHERE user_id = $1;

-- name: InsertUserMfaMethod :exec
INSERT INTO guliangiam.user_mfa_methods (id, user_id, method_type, enabled, verified, display_name, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: DeleteUserRoleBindings :exec
DELETE FROM guliangiam.user_role_bindings WHERE user_id = $1;

-- name: InsertUserRoleBinding :exec
INSERT INTO guliangiam.user_role_bindings (user_id, role_id) VALUES ($1, $2);

-- name: DeleteUserGroupMemberships :exec
DELETE FROM guliangiam.user_group_memberships WHERE user_id = $1;

-- name: InsertUserGroupMembership :exec
INSERT INTO guliangiam.user_group_memberships (user_id, group_id) VALUES ($1, $2);