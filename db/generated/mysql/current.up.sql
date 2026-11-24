-- ============================================================
-- GuliangIAM MySQL 8.4+ Schema (V1)
-- 生成物 — 请勿手工修改；唯一事实数据源为 contracts/schema/*.yaml，
-- 由 scripts/gen_schema.py --dialect mysql 生成。
-- PostgreSQL 与 MySQL 产物语义等价（ADR-023 第二基线）。
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;


-- 分组：00_platform.yaml

CREATE TABLE IF NOT EXISTS `encryption_keys` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `key_id` TEXT NOT NULL,
    `key_version` INT NOT NULL DEFAULT 1,
    `algorithm` VARCHAR(64) NOT NULL DEFAULT 'AES-256-GCM',
    `purpose` VARCHAR(64) NOT NULL DEFAULT 'field-encryption',
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `key_material_ref` VARCHAR(255) NOT NULL DEFAULT '',
    `key_fingerprint` VARCHAR(255) NOT NULL DEFAULT '',
    `tenant_scope` VARCHAR(64) NOT NULL DEFAULT 'global',
    `scope_id` VARCHAR(255) NOT NULL DEFAULT '',
    `region_code` VARCHAR(255) NOT NULL DEFAULT '',
    `sovereignty_mode` VARCHAR(64) NOT NULL DEFAULT 'platform-default',
    `activated_at` TIMESTAMP,
    `retired_at` TIMESTAMP,
    `revoked_at` TIMESTAMP,
    `rotate_after` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_encryption_keys_purpose_key_version UNIQUE (`purpose`, `key_id`(255), `key_version`),
    CONSTRAINT ck_encryption_keys_algorithm CHECK (algorithm IN ('AES-256-GCM', 'SM4-GCM')),
    CONSTRAINT ck_encryption_keys_status CHECK (status IN ('pending', 'active', 'retiring', 'retired', 'revoked')),
    CONSTRAINT ck_encryption_keys_key_version CHECK (key_version >= 1),
    CONSTRAINT ck_encryption_keys_tenant_scope CHECK (tenant_scope IN ('global', 'region', 'organization')),
    CONSTRAINT ck_encryption_keys_sovereignty_mode CHECK (sovereignty_mode IN ('platform-default', 'byok', 'hyok'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'encryption_keys' AND INDEX_NAME = 'idx_encryption_keys_purpose_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_encryption_keys_purpose_status` ON `encryption_keys` (`purpose`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_encryption_keys_rotate_after: predicate status = 'active' (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'encryption_keys' AND INDEX_NAME = 'idx_encryption_keys_rotate_after' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_encryption_keys_rotate_after` ON `encryption_keys` (`rotate_after`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'encryption_keys' AND INDEX_NAME = 'idx_encryption_keys_tenant_scope' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_encryption_keys_tenant_scope` ON `encryption_keys` (`tenant_scope`, `scope_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `encryption_reencrypt_progress` (
    `col_key` TEXT NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `last_revision` BIGINT,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    PRIMARY KEY (`col_key`(255)),
    CONSTRAINT ck_encryption_reencrypt_status CHECK (status IN ('pending', 'in_progress', 'completed', 'failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Re-encryption job checkpoint table (DB-based alternative to file checkpoint)';

CREATE TABLE IF NOT EXISTS `platform_admins` (
    `id` CHAR(36) NOT NULL COMMENT 'Platform admin primary key' PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL COMMENT 'FK -> users.id (admin user)',
    `created_by` CHAR(36) COMMENT 'FK -> users.id (creating admin; nullable for bootstrap)',
    `is_active` TINYINT(1) NOT NULL DEFAULT true COMMENT 'Master on/off switch for this platform admin',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT uq_platform_admins_user_id UNIQUE (`user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Platform admin registry — cross-tenant operator credentials and scope';

CREATE TABLE IF NOT EXISTS `platform_admin_scopes` (
    `id` CHAR(36) NOT NULL COMMENT 'Scope primary key' PRIMARY KEY,
    `platform_admin_id` CHAR(36) NOT NULL COMMENT 'FK -> platform_admins.id',
    `scope_type` VARCHAR(32) NOT NULL COMMENT 'Scope kind: all | targeted',
    `target_tenants` JSON NOT NULL COMMENT 'Tenant UUIDs — empty for scope_type=''all''',
    `approved_by` CHAR(36) COMMENT 'FK -> users.id (approver; NULL for bootstrap)',
    `approved_at` TIMESTAMP COMMENT 'Approval timestamp (must be set when approved_by is set)',
    `is_bootstrap` TINYINT(1) NOT NULL DEFAULT false COMMENT 'Bootstrap exemption — seed-created built-in admin ONLY; never backfill other approved_by IS NULL rows',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Scope expiry (max 1 year from creation)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT ck_pas_scope_type CHECK (scope_type IN ('all', 'targeted')),
    CONSTRAINT ck_pas_all_empty_tenants CHECK (scope_type <> 'all' OR JSON_LENGTH(target_tenants) = 0),
    CONSTRAINT ck_pas_targeted_has_tenants CHECK (scope_type <> 'targeted' OR JSON_LENGTH(target_tenants) > 0),
    CONSTRAINT ck_pas_expires_within_year CHECK (expires_at <= DATE_ADD(created_at, INTERVAL 1 YEAR)),
    CONSTRAINT ck_pas_expires_after_created CHECK (expires_at >= created_at),
    CONSTRAINT fk_platform_admin_scopes_platform_admin_id_platform_admins FOREIGN KEY (`platform_admin_id`) REFERENCES `platform_admins`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Cross-tenant scopes granted to platform admins (with expiry and approval)';
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'platform_admin_scopes' AND INDEX_NAME = 'idx_pas_platform_admin_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pas_platform_admin_id` ON `platform_admin_scopes` (`platform_admin_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


-- 分组：10_tenant.yaml

CREATE TABLE IF NOT EXISTS `organizations` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `slug` TEXT NOT NULL,
    `display_name` TEXT NOT NULL,
    `website_url` TEXT,
    `logo_url` TEXT,
    `logo_dark_url` TEXT,
    `favicon_url` TEXT,
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `region_code` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `created_by` CHAR(36),
    `updated_by` CHAR(36),
    `revision` BIGINT NOT NULL DEFAULT 1,
    `init_score` INT NOT NULL DEFAULT 0,
    `default_avatar` TEXT,
    `disable_signin` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_soft_deletion` TINYINT(1) NOT NULL DEFAULT FALSE,
    `ip_whitelist` VARCHAR(255) NOT NULL DEFAULT '',
    `initiate_login_url` VARCHAR(255) NOT NULL DEFAULT '',
    `disable_signup` TINYINT(1) NOT NULL DEFAULT FALSE,
    `parent_id` CHAR(36) COMMENT 'Parent organization in the tenant hierarchy; NULL = root. Tree does not change data-plane isolation (ADR-028).',
    gen_organizations_slug_active TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_organizations_slug_active UNIQUE (`slug`(255), gen_organizations_slug_active),
    CONSTRAINT fk_organizations_parent_id_organizations FOREIGN KEY (`parent_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organizations' AND INDEX_NAME = 'idx_organizations_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_organizations_created_at_desc` ON `organizations` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organizations' AND COLUMN_NAME = 'gen_orgs_slug_deleted');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `organizations` ADD COLUMN gen_orgs_slug_deleted TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organizations' AND INDEX_NAME = 'idx_orgs_slug_deleted' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_orgs_slug_deleted` ON `organizations` (`slug`(255), `gen_orgs_slug_deleted`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organizations' AND COLUMN_NAME = 'gen_organizations_parent_id');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `organizations` ADD COLUMN gen_organizations_parent_id TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organizations' AND INDEX_NAME = 'idx_organizations_parent_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_organizations_parent_id` ON `organizations` (`parent_id`, `gen_organizations_parent_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `organization_auth_config` (
    `organization_id` CHAR(36) NOT NULL PRIMARY KEY,
    `password_type` VARCHAR(64) NOT NULL DEFAULT 'argon2id',
    `password_salt` VARCHAR(255) NOT NULL DEFAULT '',
    `password_obfuscator_type` TEXT,
    `password_obfuscator_key` VARCHAR(255) NOT NULL DEFAULT '',
    `password_options` JSON NOT NULL,
    `master_password` VARCHAR(255) NOT NULL DEFAULT '',
    `master_verification_code` VARCHAR(255) NOT NULL DEFAULT '',
    `default_password` VARCHAR(255) NOT NULL DEFAULT '',
    `use_email_as_username` TINYINT(1) NOT NULL DEFAULT FALSE,
    `ldap_attributes` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_auth_config_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `organization_kerberos_config` (
    `organization_id` CHAR(36) NOT NULL PRIMARY KEY,
    `kerberos_realm` VARCHAR(255) NOT NULL DEFAULT '',
    `kerberos_kdc_host` VARCHAR(255) NOT NULL DEFAULT '',
    `kerberos_keytab` VARCHAR(255) NOT NULL DEFAULT '',
    `kerberos_service_name` VARCHAR(64) NOT NULL DEFAULT 'HTTP',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_kerberos_config_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `organization_billing` (
    `organization_id` CHAR(36) NOT NULL PRIMARY KEY,
    `balance` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `balance_credit` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `currency` VARCHAR(3) NOT NULL DEFAULT 'USD',
    `default_currency` VARCHAR(3) NOT NULL DEFAULT 'CNY',
    `enable_deposit` TINYINT(1) NOT NULL DEFAULT FALSE,
    `mu` INT NOT NULL DEFAULT 0,
    `default_quota` INT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_billing_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `organization_branding` (
    `organization_id` CHAR(36) NOT NULL PRIMARY KEY,
    `enable_dark_logo` TINYINT(1) NOT NULL DEFAULT FALSE,
    `has_privilege_consent` TINYINT(1) NOT NULL DEFAULT FALSE,
    `is_profile_public` TINYINT(1) NOT NULL DEFAULT FALSE,
    `use_permanent_avatar` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_tour` TINYINT(1) NOT NULL DEFAULT FALSE,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_organization_branding_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `organization_settings` (
    `organization_id` CHAR(36) NOT NULL PRIMARY KEY,
    `languages` JSON NOT NULL,
    `default_application_id` TEXT,
    `account_items` JSON NOT NULL,
    `nav_preferences` JSON NOT NULL,
    `feature_flags` JSON NOT NULL,
    `country_codes` JSON NOT NULL,
    `user_types` JSON NOT NULL,
    `tags` JSON NOT NULL,
    `mfa_items` JSON NOT NULL,
    `account_menu` VARCHAR(64) NOT NULL DEFAULT 'horizontal',
    `mfa_remember_in_hours` INT NOT NULL DEFAULT 0,
    `dcr_policy` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT fk_organization_settings_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_orgs_account_items_gin: PostgreSQL GIN 索引，MySQL 无等价实现，已跳过

CREATE TABLE IF NOT EXISTS `organization_theme_settings` (
    `organization_id` CHAR(36) NOT NULL PRIMARY KEY,
    `theme_name` VARCHAR(255) NOT NULL DEFAULT '',
    `tokens` JSON NOT NULL,
    `custom_css` VARCHAR(255) NOT NULL DEFAULT '',
    `assets` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT fk_organization_theme_settings_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `organization_security_policies` (
    `organization_id` CHAR(36) NOT NULL PRIMARY KEY,
    `password_policy` JSON NOT NULL,
    `password_derive_algorithm` VARCHAR(16) NOT NULL DEFAULT 'hmac_sha256' COMMENT '密码派生摘要算法；存储哈希固定 argon2id，不提供国密降级选项',
    `mfa_policy` JSON NOT NULL,
    `ip_policy` JSON NOT NULL,
    `session_policy` JSON NOT NULL,
    `risk_policy` JSON NOT NULL,
    `lockout_policy` JSON NOT NULL,
    `concurrent_session_policy` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT ck_organization_security_policies_password_derive_algorithm CHECK (password_derive_algorithm IN ('hmac_sha256', 'hmac_sm3')),
    CONSTRAINT fk_organization_security_policies_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_org_security_password_policy: PostgreSQL GIN 索引，MySQL 无等价实现，已跳过
-- INDEX idx_orgs_password_policy_gin: PostgreSQL GIN 索引，MySQL 无等价实现，已跳过

CREATE TABLE IF NOT EXISTS `outbox` (
    `id` CHAR(36) NOT NULL COMMENT 'Outbox event primary key (UUID)' PRIMARY KEY,
    `organization_id` CHAR(36),
    `aggregate_type` TEXT NOT NULL COMMENT 'Source aggregate type (users / orders / sessions / ...)',
    `aggregate_id` CHAR(36) NOT NULL COMMENT 'Source aggregate UUID',
    `event_type` TEXT NOT NULL COMMENT 'Domain event type name (e.g. user.created, order.paid)',
    `event_version` INT NOT NULL DEFAULT 1,
    `payload` JSON NOT NULL COMMENT 'Full event body (structured document); consumed by subscribers without needing to re-read the aggregate',
    `headers` JSON NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending' COMMENT 'Dispatch lifecycle: pending / dispatching / delivered / failed / dead_lettered',
    `attempts` INT NOT NULL DEFAULT 0,
    `max_attempts` INT NOT NULL DEFAULT 12,
    `available_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `locked_by` TEXT,
    `locked_at` TIMESTAMP,
    `processed_at` TIMESTAMP,
    `last_error` TEXT COMMENT 'Last dispatch failure message, truncated',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Event emission timestamp (transaction-committed moment)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last row modification timestamp',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    CONSTRAINT ck_outbox_status CHECK (status IN ('pending', 'processing', 'succeeded', 'failed', 'dead')),
    CONSTRAINT ck_outbox_attempts CHECK (attempts >= 0 AND attempts <= max_attempts),
    CONSTRAINT fk_outbox_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_outbox_pending_available_at: predicate status = 'pending' (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox' AND INDEX_NAME = 'idx_outbox_pending_available_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_outbox_pending_available_at` ON `outbox` (`available_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox' AND INDEX_NAME = 'idx_outbox_status_created_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_outbox_status_created_at` ON `outbox` (`status`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox' AND INDEX_NAME = 'idx_outbox_aggregate' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_outbox_aggregate` ON `outbox` (`aggregate_type`(255), `aggregate_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox' AND INDEX_NAME = 'idx_outbox_organization_id_created_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_outbox_organization_id_created_at` ON `outbox` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `outbox_dead_letters` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) COMMENT 'Tenant owner copied from outbox; NULL rows are super-admin only',
    `aggregate_type` TEXT NOT NULL,
    `aggregate_id` TEXT NOT NULL,
    `event_type` TEXT NOT NULL,
    `event_id` TEXT NOT NULL,
    `payload` JSON NOT NULL,
    `headers` JSON,
    `status` VARCHAR(64) NOT NULL DEFAULT 'dead',
    `attempts` INT NOT NULL DEFAULT 12,
    `last_error` TEXT,
    `dead_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `replayed_at` TIMESTAMP,
    `replayed_by` CHAR(36),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_outbox_dead_letters_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_dead_letters' AND INDEX_NAME = 'idx_outbox_dead_letters_status_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_outbox_dead_letters_status_created` ON `outbox_dead_letters` (`status`, `dead_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'outbox_dead_letters' AND INDEX_NAME = 'idx_outbox_dead_letters_event_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_outbox_dead_letters_event_id` ON `outbox_dead_letters` (`event_id`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `idempotency_keys` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36),
    `scope` TEXT NOT NULL,
    `idempotency_key` TEXT NOT NULL,
    `request_method` TEXT NOT NULL,
    `request_path` TEXT NOT NULL,
    `request_hash` TEXT NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'in_progress',
    `response_status` INT,
    `response_body` JSON,
    `content_type` TEXT,
    `completed_at` TIMESTAMP,
    `expires_at` TIMESTAMP NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_idempotency_keys_scope_key UNIQUE (`scope`(255), `idempotency_key`(255)),
    CONSTRAINT ck_idempotency_keys_status CHECK (status IN ('in_progress', 'succeeded', 'failed')),
    CONSTRAINT fk_idempotency_keys_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'idempotency_keys' AND INDEX_NAME = 'idx_idempotency_keys_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_idempotency_keys_expires_at` ON `idempotency_keys` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'idempotency_keys' AND INDEX_NAME = 'idx_idempotency_keys_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_idempotency_keys_organization_id` ON `idempotency_keys` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `deployment_regions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `region_code` TEXT NOT NULL,
    `name` VARCHAR(255) NOT NULL DEFAULT '',
    `country_code` VARCHAR(255) NOT NULL DEFAULT '',
    `timezone` VARCHAR(255) NOT NULL DEFAULT '',
    `is_primary` TINYINT(1) NOT NULL DEFAULT FALSE,
    `rto_minutes` INT NOT NULL DEFAULT 0,
    `rpo_minutes` INT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_deployment_regions_region_code UNIQUE (`region_code`(255)),
    CONSTRAINT ck_deployment_regions_country_code CHECK (char_length(country_code) = 2),
    CONSTRAINT ck_deployment_regions_rto_minutes CHECK (rto_minutes >= 0),
    CONSTRAINT ck_deployment_regions_rpo_minutes CHECK (rpo_minutes >= 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `tenant_instances` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `region_id` CHAR(36) NOT NULL,
    `region_code` VARCHAR(255) NOT NULL DEFAULT '',
    `is_primary` TINYINT(1) NOT NULL DEFAULT FALSE,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending' COMMENT 'Topology lifecycle: pending / active / failed / decommissioned',
    `placement_kind` VARCHAR(64) NOT NULL DEFAULT 'shared_cluster' COMMENT 'Placement kind: shared_cluster today; dedicated_cluster requires routing work',
    `storage_boundary` VARCHAR(64) NOT NULL DEFAULT 'shared' COMMENT 'Storage boundary: shared / tenant (metadata only, never an RLS input)',
    `backup_boundary` VARCHAR(64) NOT NULL DEFAULT 'shared' COMMENT 'Backup boundary: shared / tenant',
    `data_residency_country` VARCHAR(2) NOT NULL DEFAULT '' COMMENT 'ISO 3166-1 alpha-2; defaults from deployment_regions.country_code',
    `capacity_quota_units` BIGINT NOT NULL DEFAULT 0 COMMENT 'Capacity quota units; 0 means unlimited',
    `provisioned_at` TIMESTAMP,
    `effective_from` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_tenant_instances_org_region UNIQUE (`organization_id`, `region_id`),
    gen_tenant_instances_primary TINYINT(1) AS (CASE WHEN (is_primary AND status <> 'decommissioned') THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_tenant_instances_primary UNIQUE (`organization_id`, gen_tenant_instances_primary),
    CONSTRAINT ck_tenant_instances_status CHECK (status IN ('pending', 'active', 'failed', 'decommissioned')),
    CONSTRAINT ck_tenant_instances_country CHECK (data_residency_country = '' OR char_length(data_residency_country) = 2),
    CONSTRAINT fk_tenant_instances_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_tenant_instances_region_id_deployment_regions FOREIGN KEY (`region_id`) REFERENCES `deployment_regions`(`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_instances' AND INDEX_NAME = 'idx_tenant_instances_region_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_tenant_instances_region_id` ON `tenant_instances` (`region_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `tenant_identities` (
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant identity primary key; equals organizations.id (single tenant_id source)' PRIMARY KEY,
    `status` VARCHAR(64) NOT NULL DEFAULT 'provisioning' COMMENT 'Identity lifecycle: provisioning / active / failed / decommissioned',
    `isolation_tier` VARCHAR(64) NOT NULL DEFAULT 'row' COMMENT 'Isolation tier: row only today; schema/database require a new ADR',
    `is_built_in` TINYINT(1) NOT NULL DEFAULT FALSE COMMENT 'Built-in organization identity cannot be decommissioned',
    `provisioned_at` TIMESTAMP,
    `decommissioned_at` TIMESTAMP,
    `last_error` TEXT COMMENT 'Sanitized provisioning failure reason; never contains credentials',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_tenant_identities_status CHECK (status IN ('provisioning', 'active', 'failed', 'decommissioned')),
    CONSTRAINT ck_tenant_identities_isolation_tier CHECK (isolation_tier IN ('row')),
    CONSTRAINT fk_tenant_identities_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'tenant_identities' AND INDEX_NAME = 'idx_tenant_identities_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_tenant_identities_status` ON `tenant_identities` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `organization_domains` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `domain` VARCHAR(253) NOT NULL,
    `verification_token` VARCHAR(255) NOT NULL,
    `status` VARCHAR(16) NOT NULL DEFAULT 'pending',
    `verified_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `attempts` INT NOT NULL DEFAULT 0 COMMENT 'Number of failed DNS TXT verification attempts while pending',
    `last_error` VARCHAR(255) COMMENT 'Sanitized failure classification code + short reason, truncated to 255 chars',
    `next_retry_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Earliest time this pending row may be claimed again; frozen once terminal',
    `last_checked_at` TIMESTAMP COMMENT 'Last DNS TXT query time',
    CONSTRAINT uq_org_domain UNIQUE (`organization_id`, `domain`),
    CONSTRAINT ck_organization_domains_status CHECK (status IN ('pending', 'verified', 'failed')),
    CONSTRAINT fk_organization_domains_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organization_domains' AND INDEX_NAME = 'idx_organization_domains_domain' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_organization_domains_domain` ON `organization_domains` (`domain`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organization_domains' AND INDEX_NAME = 'idx_organization_domains_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_organization_domains_status` ON `organization_domains` (`status`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'organization_domains' AND INDEX_NAME = 'idx_organization_domains_retry' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_organization_domains_retry` ON `organization_domains` (`status`, `next_retry_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `organization_policy_inheritance` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `policy_key` VARCHAR(128) NOT NULL,
    `mode` VARCHAR(16) NOT NULL,
    `source_organization_id` CHAR(36),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_organization_policy_inheritance_key UNIQUE (`organization_id`, `policy_key`),
    CONSTRAINT ck_organization_policy_inheritance_mode CHECK (mode IN ('inherit', 'override', 'lock')),
    CONSTRAINT fk_organization_policy_inheritance_organization_id_org_5d6d705e FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_organization_policy_inheritance_source_organization_698f8308 FOREIGN KEY (`source_organization_id`) REFERENCES `organizations`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- 分组：20_identity.yaml

CREATE TABLE IF NOT EXISTS `users` (
    `id` CHAR(36) NOT NULL COMMENT 'User primary key (UUID)' PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant org this user belongs to (RLS predicate key)',
    `username` TEXT NOT NULL COMMENT 'User login name, unique within organization',
    `external_id` TEXT COMMENT 'IdP subject identifier when account was created via SSO; NULL for local users',
    `user_type` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'User classification: human / service_account / bot',
    `status` VARCHAR(64) NOT NULL DEFAULT 'active' COMMENT 'Account lifecycle state: active / disabled / locked / pending_verification',
    `is_admin` TINYINT(1) NOT NULL DEFAULT false COMMENT 'Org-level admin flag; when true the user bypasses role-based permission checks',
    `is_forbidden` TINYINT(1) NOT NULL DEFAULT false COMMENT 'Emergency block flag; when true all auth flows reject this user',
    `is_verified` TINYINT(1) NOT NULL DEFAULT false COMMENT 'Whether email/phone verification has completed',
    `is_online` TINYINT(1) NOT NULL DEFAULT false COMMENT 'Cached session presence flag updated by periodic heartbeat',
    `created_ip` VARCHAR(50) NOT NULL DEFAULT '' COMMENT 'First-seen creation IP address (varchar to tolerate IPv6 variants)',
    `signup_application_id` CHAR(36) COMMENT 'Application through which the user registered; NULL for admin-created accounts',
    `register_source` TEXT COMMENT 'Creation channel: self_signup | admin_create | invitation | SCIM | OpenID | SAML',
    `register_type` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Auth method used at signup: password | oidc | saml | webauthn | magic_link',
    `tag` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Free-form admin label (no schema constraint; use for internal cohorting)',
    `score` INT NOT NULL DEFAULT 0 COMMENT 'Composite risk / engagement score used by admin UX sorting',
    `is_deleted` TINYINT(1) NOT NULL DEFAULT false COMMENT 'Soft-delete marker; rows with true are hidden from normal queries',
    `ip_whitelist` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Semicolon-separated CIDR list; when non-empty, login restricted to these ranges',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Account creation timestamp',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last row modification timestamp (always set on any update)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp; NULL when is_deleted=false',
    `created_by` CHAR(36) COMMENT 'User UUID that created this account (admin self-serve flow)',
    `updated_by` CHAR(36) COMMENT 'User UUID that last modified this account',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter; incremented on every write',
    CONSTRAINT ck_users_status CHECK (status IN ('active', 'disabled', 'locked', 'pending_verification')),
    gen_users_org_username_active TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_users_org_username_active UNIQUE (`organization_id`, `username`(255), gen_users_org_username_active),
    CONSTRAINT fk_users_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'idx_users_org_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_org_created_at_desc` ON `users` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'idx_users_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_organization_id` ON `users` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'idx_users_org_deleted' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_org_deleted` ON `users` (`organization_id`, `deleted_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'gen_users_org_username_deleted');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `users` ADD COLUMN gen_users_org_username_deleted TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'idx_users_org_username_deleted' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_org_username_deleted` ON `users` (`organization_id`, `username`(255), `gen_users_org_username_deleted`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'gen_users_status');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `users` ADD COLUMN gen_users_status TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'idx_users_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_status` ON `users` (`status`, `gen_users_status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND COLUMN_NAME = 'gen_users_org_status');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `users` ADD COLUMN gen_users_org_status TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND INDEX_NAME = 'idx_users_org_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_org_status` ON `users` (`organization_id`, `status`, `gen_users_org_status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_signin_tracking` (
    `user_id` CHAR(36) NOT NULL PRIMARY KEY,
    `signin_wrong_times` INT NOT NULL DEFAULT 0,
    `last_signin_wrong_time` TIMESTAMP,
    `last_signin_at` TIMESTAMP,
    `last_signin_ip` VARCHAR(45),
    `signup_ip` VARCHAR(45),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_user_signin_tracking_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_mfa_config` (
    `user_id` CHAR(36) NOT NULL PRIMARY KEY,
    `preferred_mfa_type` TEXT,
    `recovery_code_hashes` JSON NOT NULL COMMENT 'Argon2id hashes of one-time recovery codes; plaintext never stored',
    `recovery_generated_at` TIMESTAMP COMMENT 'When the current recovery-code set was generated',
    `totp_secret` TEXT,
    `mfa_radius_provider` VARCHAR(255) NOT NULL DEFAULT '',
    `mfa_radius_username` VARCHAR(255) NOT NULL DEFAULT '',
    `mfa_push_receiver` VARCHAR(255) NOT NULL DEFAULT '',
    `mfa_push_provider` VARCHAR(255) NOT NULL DEFAULT '',
    `mfa_push_enabled` TINYINT(1) NOT NULL DEFAULT false,
    `mfa_remember_deadline` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_user_mfa_config_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_financials` (
    `user_id` CHAR(36) NOT NULL PRIMARY KEY,
    `balance` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `balance_credit` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `balance_currency` VARCHAR(3) NOT NULL DEFAULT 'USD',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_user_financials_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_custom_attributes` (
    `user_id` CHAR(36) NOT NULL PRIMARY KEY,
    `custom1` VARCHAR(255) NOT NULL DEFAULT '',
    `custom2` VARCHAR(255) NOT NULL DEFAULT '',
    `custom3` VARCHAR(255) NOT NULL DEFAULT '',
    `custom4` VARCHAR(255) NOT NULL DEFAULT '',
    `custom5` VARCHAR(255) NOT NULL DEFAULT '',
    `custom6` VARCHAR(255) NOT NULL DEFAULT '',
    `custom7` VARCHAR(255) NOT NULL DEFAULT '',
    `custom8` VARCHAR(255) NOT NULL DEFAULT '',
    `custom9` VARCHAR(255) NOT NULL DEFAULT '',
    `custom10` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_user_custom_attributes_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_collections` (
    `user_id` CHAR(36) NOT NULL PRIMARY KEY,
    `addresses` JSON NOT NULL,
    `managed_accounts` JSON NOT NULL,
    `face_ids` JSON NOT NULL,
    `cart` JSON NOT NULL,
    `mfa_items` JSON NOT NULL,
    `mfa_accounts` JSON NOT NULL,
    `managed_accounts_sealed` MEDIUMTEXT COMMENT 'v2 envelope of the managed_accounts plaintext payload (AAD table/column/user_id); NULL = not yet sealed, legacy column remains authoritative',
    `mfa_items_sealed` MEDIUMTEXT COMMENT 'v2 envelope of the mfa_items plaintext payload (AAD table/column/user_id); NULL = not yet sealed',
    `mfa_accounts_sealed` MEDIUMTEXT COMMENT 'v2 envelope of the mfa_accounts plaintext payload (AAD table/column/user_id); NULL = not yet sealed',
    `application_scopes` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT fk_user_collections_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_user_collections_addresses_gin: PostgreSQL GIN 索引，MySQL 无等价实现，已跳过
-- INDEX idx_user_collections_face_ids_gin: PostgreSQL GIN 索引，MySQL 无等价实现，已跳过

CREATE TABLE IF NOT EXISTS `user_profiles` (
    `user_id` CHAR(36) NOT NULL PRIMARY KEY,
    `display_name` TEXT,
    `first_name` TEXT,
    `last_name` TEXT,
    `avatar_url` TEXT,
    `email` TEXT,
    `email_verified` TINYINT(1) NOT NULL DEFAULT false,
    `phone` TEXT,
    `phone_verified` TINYINT(1) NOT NULL DEFAULT false,
    `country_code` TEXT,
    `region` TEXT,
    `location` TEXT,
    `birthday` DATE,
    `gender` TEXT,
    `bio` TEXT,
    `real_name` TEXT,
    `id_card_type` TEXT,
    `id_card` TEXT,
    `id_card_sealed` TEXT COMMENT 'v2 envelope of id_card (PII); NULL = legacy plaintext row remains',
    `passport_no` VARCHAR(64),
    `passport_no_sealed` TEXT COMMENT 'v2 envelope of passport_no (PII); NULL = legacy plaintext row remains',
    `driver_license_no` VARCHAR(64),
    `driver_license_no_sealed` TEXT COMMENT 'v2 envelope of driver_license_no (PII); NULL = legacy plaintext row remains',
    `id_card_expiry` DATE,
    `issue_country` VARCHAR(8),
    `issuing_authority` VARCHAR(255),
    `passport_photo_url` TEXT,
    `driver_license_front_url` TEXT,
    `driver_license_back_url` TEXT,
    `properties` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    gen_user_profiles_email_active TINYINT(1) AS (CASE WHEN (deleted_at IS NULL AND email IS NOT NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_user_profiles_email_active UNIQUE (`email`(255), gen_user_profiles_email_active),
    CONSTRAINT fk_user_profiles_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_profiles' AND COLUMN_NAME = 'gen_users_email_deleted');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `user_profiles` ADD COLUMN gen_users_email_deleted TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_profiles' AND INDEX_NAME = 'idx_users_email_deleted' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_email_deleted` ON `user_profiles` (`email`(255), `gen_users_email_deleted`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_profiles' AND COLUMN_NAME = 'gen_users_phone_deleted');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `user_profiles` ADD COLUMN gen_users_phone_deleted TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_profiles' AND INDEX_NAME = 'idx_users_phone_deleted' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_users_phone_deleted` ON `user_profiles` (`phone`(255), `gen_users_phone_deleted`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_credentials` (
    `user_id` CHAR(36) NOT NULL PRIMARY KEY,
    `password_hash` VARCHAR(255) NOT NULL DEFAULT '',
    `password_algo` VARCHAR(255) NOT NULL DEFAULT '',
    `password_updated_at` TIMESTAMP,
    `access_key` VARCHAR(255) NOT NULL DEFAULT '',
    `access_secret_hash` VARCHAR(255) NOT NULL DEFAULT '',
    `access_secret_rotated_at` TIMESTAMP,
    `access_secret_hash_prev` VARCHAR(255),
    `access_secret_prev_expires_at` TIMESTAMP,
    `need_update_password` TINYINT(1) NOT NULL DEFAULT false,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    CONSTRAINT fk_user_credentials_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `user_password_history` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL,
    `password_hash` TEXT NOT NULL,
    `password_algo` VARCHAR(64) NOT NULL DEFAULT 'argon2id',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_user_password_history_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_password_history' AND INDEX_NAME = 'idx_password_history_user_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_password_history_user_created` ON `user_password_history` (`user_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_identities` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL,
    `provider_id` VARCHAR(255) NOT NULL DEFAULT '',
    `identity_type` VARCHAR(255) NOT NULL DEFAULT '',
    `subject` VARCHAR(255) NOT NULL DEFAULT '',
    `provider_account` VARCHAR(255) NOT NULL DEFAULT '',
    `profile_snapshot` JSON NOT NULL,
    `linked_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `link_state` VARCHAR(64) NOT NULL DEFAULT 'auto_linked' COMMENT 'Identity merge lifecycle state declared by the state machine registry (user_identities_merge) and by the generated domain constants.',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT uq_identity_provider_subject UNIQUE (`identity_type`, `provider_id`, `subject`),
    CONSTRAINT ck_user_identities_link_state CHECK (link_state IN ('auto_linked', 'merge_pending', 'linked', 'rejected')),
    CONSTRAINT fk_user_identities_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_identities' AND INDEX_NAME = 'idx_user_identities_subject' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_identities_subject` ON `user_identities` (`subject`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_identities' AND INDEX_NAME = 'idx_user_identities_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_identities_user_id` ON `user_identities` (`user_id`, `linked_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_mfa_methods` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL,
    `method_type` VARCHAR(255) NOT NULL DEFAULT '',
    `enabled` TINYINT(1) NOT NULL DEFAULT false,
    `verified` TINYINT(1) NOT NULL DEFAULT false,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `last_used_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_user_mfa_methods_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_mfa_methods' AND INDEX_NAME = 'idx_user_mfa_methods_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_mfa_methods_user_id` ON `user_mfa_methods` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `applications` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `slug` TEXT NOT NULL,
    `display_name` TEXT NOT NULL,
    `category` VARCHAR(255) NOT NULL DEFAULT '',
    `app_type` VARCHAR(255) NOT NULL DEFAULT '',
    `description` TEXT,
    `homepage_url` TEXT,
    `domain` TEXT,
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `client_id` TEXT,
    `tags` JSON NOT NULL,
    `logo` VARCHAR(255) NOT NULL DEFAULT '',
    `is_portal` TINYINT(1) NOT NULL DEFAULT FALSE,
    `is_visible` TINYINT(1) NOT NULL DEFAULT TRUE,
    `is_shared` TINYINT(1) NOT NULL DEFAULT FALSE,
    `title` VARCHAR(255) NOT NULL DEFAULT '',
    `favicon` VARCHAR(255) NOT NULL DEFAULT '',
    `menu_order` INT NOT NULL DEFAULT 0,
    `menu_mode` VARCHAR(64) NOT NULL DEFAULT 'side',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `created_by` CHAR(36),
    `updated_by` CHAR(36),
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_applications_org_slug UNIQUE (`organization_id`, `slug`(255)),
    gen_applications_client_id_non_empty TINYINT(1) AS (CASE WHEN (client_id <> '') THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_applications_client_id_non_empty UNIQUE (`client_id`(255), gen_applications_client_id_non_empty),
    CONSTRAINT fk_applications_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'applications' AND INDEX_NAME = 'idx_apps_org_deleted' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_apps_org_deleted` ON `applications` (`organization_id`, `deleted_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'applications' AND COLUMN_NAME = 'gen_applications_organization_id_active');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `applications` ADD COLUMN gen_applications_organization_id_active TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'applications' AND INDEX_NAME = 'idx_applications_organization_id_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_applications_organization_id_active` ON `applications` (`organization_id`, `gen_applications_organization_id_active`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'applications' AND COLUMN_NAME = 'gen_applications_client_id_active');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `applications` ADD COLUMN gen_applications_client_id_active TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'applications' AND INDEX_NAME = 'idx_applications_client_id_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_applications_client_id_active` ON `applications` (`client_id`(255), `gen_applications_client_id_active`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `app_network_config` (
    `application_id` CHAR(36) NOT NULL PRIMARY KEY,
    `provider_items` JSON NOT NULL,
    `forced_redirect_origin` VARCHAR(255) NOT NULL DEFAULT '',
    `other_domains` JSON NOT NULL,
    `upstream_host` VARCHAR(255) NOT NULL DEFAULT '',
    `ssl_mode` VARCHAR(255) NOT NULL DEFAULT '',
    `ssl_cert` VARCHAR(255) NOT NULL DEFAULT '',
    `client_cert` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_app_network_config_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `app_oauth_config` (
    `application_id` CHAR(36) NOT NULL PRIMARY KEY,
    `redirect_uris` JSON NOT NULL,
    `grant_types` JSON NOT NULL,
    `token_endpoint_auth_method` VARCHAR(255) NOT NULL DEFAULT '',
    `client_secret_hash` TEXT,
    `registration_access_token_hash` TEXT,
    `custom_scopes` JSON NOT NULL,
    `token_format` VARCHAR(64) NOT NULL DEFAULT 'JWT',
    `expire_in_hours` INT NOT NULL DEFAULT 24,
    `refresh_token_expire_in_hours` INT NOT NULL DEFAULT 168,
    `token_signing_method` VARCHAR(64) NOT NULL DEFAULT 'RS256',
    `token_fields` JSON NOT NULL,
    `token_attributes` JSON NOT NULL,
    `dpop_required` TINYINT(1) NOT NULL DEFAULT FALSE,
    `pkce_required` TINYINT(1) NOT NULL DEFAULT FALSE,
    `cookie_expire_in_hours` INT NOT NULL DEFAULT 168,
    `enable_beta_oidc` TINYINT(1) NOT NULL DEFAULT FALSE,
    `consent_mode` VARCHAR(255) NOT NULL DEFAULT '',
    `backchannel_logout_uri` VARCHAR(255) NOT NULL DEFAULT '',
    `frontchannel_logout_uri` VARCHAR(255) NOT NULL DEFAULT '',
    `frontchannel_logout_session_required` TINYINT(1) NOT NULL DEFAULT FALSE,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_app_oauth_config_token_signing_method CHECK (token_signing_method IN ('RS256', 'SM2-SM3')),
    CONSTRAINT ck_app_oauth_config_token_endpoint_auth_method CHECK (token_endpoint_auth_method IN ('', 'none', 'client_secret_basic', 'client_secret_post', 'private_key_jwt', 'tls_client_auth', 'self_signed_tls_client_auth')),
    CONSTRAINT fk_app_oauth_config_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_app_oauth_config_grant_types: PostgreSQL GIN 索引，MySQL 无等价实现，已跳过

CREATE TABLE IF NOT EXISTS `app_client_secrets` (
    `id` CHAR(36) NOT NULL COMMENT 'Secret row identifier (UUIDv7, generated by the application layer)' PRIMARY KEY,
    `application_id` CHAR(36) NOT NULL COMMENT 'Owning OAuth client application',
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant owner (denormalised from applications.organization_id) so every repository query can filter explicitly without relying on RLS',
    `secret_prefix` VARCHAR(16) NOT NULL COMMENT 'Non-secret display prefix (gl_cs_ + first six random characters)',
    `secret_hash` VARCHAR(255) NOT NULL COMMENT 'Argon2id hash used by client_secret_basic and client_secret_post',
    `status` VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'Lifecycle state: active / grace / retired',
    `rotated_at` TIMESTAMP COMMENT 'Time this secret was superseded by a rotation',
    `grace_until` TIMESTAMP COMMENT 'End of the acceptance window for a rotated secret',
    `retired_at` TIMESTAMP COMMENT 'Time the secret was permanently retired',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_app_client_secrets_status CHECK (status IN ('active', 'grace', 'retired')),
    gen_app_client_secrets_active TINYINT(1) AS (CASE WHEN (status = 'active') THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_app_client_secrets_active UNIQUE (`application_id`, gen_app_client_secrets_active),
    CONSTRAINT fk_app_client_secrets_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_secrets_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Client secret lifecycle rows; secret_hash is the only stored credential material (client_secret_basic / client_secret_post)';
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_client_secrets' AND INDEX_NAME = 'idx_app_client_secrets_application_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_app_client_secrets_application_status` ON `app_client_secrets` (`application_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_client_secrets' AND INDEX_NAME = 'idx_app_client_secrets_org_application_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_app_client_secrets_org_application_status` ON `app_client_secrets` (`organization_id`, `application_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `app_client_jwks` (
    `id` CHAR(36) NOT NULL COMMENT 'JWK row identifier (UUIDv7, generated by the application layer)' PRIMARY KEY,
    `application_id` CHAR(36) NOT NULL COMMENT 'Owning OAuth client application',
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant owner (denormalised from applications.organization_id) so every repository query can filter explicitly without relying on RLS',
    `kid` VARCHAR(128) NOT NULL COMMENT 'Key identifier the client places in the assertion JWT header',
    `alg` VARCHAR(16) NOT NULL DEFAULT 'RS256' COMMENT 'Signature algorithm allowed for this key: RS256 / PS256 / ES256',
    `jwk` JSON NOT NULL COMMENT 'Public JWK document only (no d/p/q/dp/dq/qi/k members)',
    `status` VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'Lifecycle state: active / retired',
    `retired_at` TIMESTAMP COMMENT 'Time the key was retired',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_app_client_jwks_alg CHECK (alg IN ('RS256', 'PS256', 'ES256')),
    CONSTRAINT ck_app_client_jwks_status CHECK (status IN ('active', 'retired')),
    CONSTRAINT fk_app_client_jwks_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_jwks_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Public keys registered by a client for private_key_jwt assertions; private key parameters are rejected before persistence';
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_client_jwks' AND INDEX_NAME = 'uq_app_client_jwks_application_kid' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE UNIQUE INDEX `uq_app_client_jwks_application_kid` ON `app_client_jwks` (`application_id`, `kid`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_client_jwks' AND INDEX_NAME = 'idx_app_client_jwks_org_application_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_app_client_jwks_org_application_status` ON `app_client_jwks` (`organization_id`, `application_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `app_client_certificates` (
    `id` CHAR(36) NOT NULL COMMENT 'Certificate row identifier (UUIDv7, generated by the application layer)' PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant org of the owning application (RLS predicate and fingerprint scope)',
    `application_id` CHAR(36) NOT NULL COMMENT 'Owning OAuth client application',
    `auth_method` VARCHAR(32) NOT NULL COMMENT 'tls_client_auth (CA-issued) or self_signed_tls_client_auth (fingerprint only)',
    `certificate_pem` TEXT NOT NULL COMMENT 'Public certificate PEM (private key material is rejected on upload)',
    `fingerprint_sha256` VARCHAR(43) NOT NULL COMMENT 'base64url(SHA-256(DER)) RFC 8705 x5t#S256 value, 43 characters',
    `subject_dn` VARCHAR(512) NOT NULL COMMENT 'Subject distinguished name used for tls_client_auth binding',
    `issuer_dn` VARCHAR(512) NOT NULL COMMENT 'Issuer distinguished name; equals subject_dn for self-signed certificates',
    `not_before` TIMESTAMP COMMENT 'Certificate validity start',
    `not_after` TIMESTAMP COMMENT 'Certificate validity end',
    `status` VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'Lifecycle state: active / retired',
    `retired_at` TIMESTAMP COMMENT 'Time the certificate binding was retired',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_app_client_certificates_auth_method CHECK (auth_method IN ('tls_client_auth', 'self_signed_tls_client_auth')),
    CONSTRAINT ck_app_client_certificates_status CHECK (status IN ('active', 'retired')),
    CONSTRAINT fk_app_client_certificates_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_certificates_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Client certificates registered for tls_client_auth and self_signed_tls_client_auth; only public certificate PEM is stored';
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_client_certificates' AND INDEX_NAME = 'uq_app_client_certificates_org_fingerprint' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE UNIQUE INDEX `uq_app_client_certificates_org_fingerprint` ON `app_client_certificates` (`organization_id`, `fingerprint_sha256`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'app_client_certificates' AND INDEX_NAME = 'idx_app_client_certificates_application_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_app_client_certificates_application_status` ON `app_client_certificates` (`application_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `app_signin_config` (
    `application_id` CHAR(36) NOT NULL PRIMARY KEY,
    `disable_signin` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_exclusive_signin` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_signin_session` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_signup` TINYINT(1) NOT NULL DEFAULT TRUE,
    `enable_guest_signin` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_auto_signin` TINYINT(1) NOT NULL DEFAULT TRUE,
    `enable_link_with_email` TINYINT(1) NOT NULL DEFAULT FALSE,
    `signin_url` VARCHAR(255) NOT NULL DEFAULT '',
    `logout_url` VARCHAR(255) NOT NULL DEFAULT '',
    `logout_redirect_url` VARCHAR(255) NOT NULL DEFAULT '',
    `prompt_url` VARCHAR(255) NOT NULL DEFAULT '',
    `signup_url` VARCHAR(255) NOT NULL DEFAULT '',
    `forget_url` VARCHAR(255) NOT NULL DEFAULT '',
    `affiliation_url` VARCHAR(255) NOT NULL DEFAULT '',
    `signin_methods` JSON NOT NULL,
    `signin_items` JSON NOT NULL,
    `failed_signin_limit` INT NOT NULL DEFAULT 0,
    `failed_signin_frozen_time` INT NOT NULL DEFAULT 0,
    `code_resend_timeout` INT NOT NULL DEFAULT 0,
    `code_expire` INT NOT NULL DEFAULT 0,
    `default_group` VARCHAR(255) NOT NULL DEFAULT '',
    `org_choice_mode` VARCHAR(255) NOT NULL DEFAULT '',
    `ip_whitelist` VARCHAR(255) NOT NULL DEFAULT '',
    `ip_blacklist` VARCHAR(255) NOT NULL DEFAULT '',
    `cors_allowed_origins` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_app_signin_config_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `app_saml_config` (
    `application_id` CHAR(36) NOT NULL PRIMARY KEY,
    `saml_reply_url` VARCHAR(255) NOT NULL DEFAULT '',
    `certificate` VARCHAR(255) NOT NULL DEFAULT '',
    `saml_metadata` TEXT NOT NULL COMMENT 'IdP SAML metadata XML; multi-KB document, so unbounded text rather than a 255-char column',
    `saml_metadata_url` VARCHAR(2048) NOT NULL DEFAULT '' COMMENT 'IdP metadata URL; 2048 is the practical browser/HTTP URL ceiling',
    `enable_saml_compress` TINYINT(1) NOT NULL DEFAULT FALSE,
    `saml_name_id_format` VARCHAR(255) NOT NULL DEFAULT '',
    `saml_name_id_content` VARCHAR(255) NOT NULL DEFAULT '',
    `saml_sign_request` TINYINT(1) NOT NULL DEFAULT FALSE,
    `saml_attributes` JSON NOT NULL,
    `enable_saml_c14n10` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_saml_post_binding` TINYINT(1) NOT NULL DEFAULT FALSE,
    `disable_saml_attributes` TINYINT(1) NOT NULL DEFAULT FALSE,
    `enable_saml_assertion_signature` TINYINT(1) NOT NULL DEFAULT FALSE,
    `is_email_saml_nameid` TINYINT(1) NOT NULL DEFAULT FALSE,
    `saml_hash_algorithm` VARCHAR(255) NOT NULL DEFAULT '',
    `saml_c14n_prefix` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_app_saml_config_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `app_form_config` (
    `application_id` CHAR(36) NOT NULL PRIMARY KEY,
    `signup_items` JSON NOT NULL,
    `custom_css` TEXT,
    `custom_header` TEXT,
    `form_css` TEXT,
    `form_css_mobile` TEXT,
    `form_side_html` TEXT,
    `form_background_url` VARCHAR(255) NOT NULL DEFAULT '',
    `form_background_url_mobile` VARCHAR(255) NOT NULL DEFAULT '',
    `form_offset` VARCHAR(255) NOT NULL DEFAULT '',
    `form_position` VARCHAR(255) NOT NULL DEFAULT '',
    `header_html` TEXT,
    `footer_html` TEXT,
    `signup_html` TEXT,
    `signin_html` TEXT,
    `page_html` TEXT,
    `custom_js` VARCHAR(255) NOT NULL DEFAULT '',
    `terms_of_use` VARCHAR(255) NOT NULL DEFAULT '',
    `privacy_policy` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT fk_app_form_config_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `application_provider_bindings` (
    `application_id` CHAR(36) NOT NULL,
    `provider_id` CHAR(36) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    PRIMARY KEY (`application_id`, `provider_id`),
    CONSTRAINT fk_application_provider_bindings_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'application_provider_bindings' AND INDEX_NAME = 'idx_app_provider_bindings_provider_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_app_provider_bindings_provider_id` ON `application_provider_bindings` (`provider_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `providers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL DEFAULT '',
    `display_name` TEXT NOT NULL,
    `category` TEXT NOT NULL,
    `provider_type` TEXT NOT NULL,
    `sub_type` VARCHAR(255) NOT NULL DEFAULT '',
    `family` VARCHAR(32),
    `kind` VARCHAR(64),
    `variant` VARCHAR(32),
    `method` VARCHAR(255) NOT NULL DEFAULT '',
    `client_id` VARCHAR(255) NOT NULL DEFAULT '',
    `client_secret` TEXT NOT NULL,
    `client_id2` VARCHAR(255) NOT NULL DEFAULT '',
    `client_secret2` TEXT NOT NULL,
    `provider_url` VARCHAR(255) NOT NULL DEFAULT '',
    `auth_url` VARCHAR(255) NOT NULL DEFAULT '',
    `token_url` VARCHAR(255) NOT NULL DEFAULT '',
    `user_info_url` VARCHAR(255) NOT NULL DEFAULT '',
    `callback_url` VARCHAR(255) NOT NULL DEFAULT '',
    `signing_alg` VARCHAR(255) NOT NULL DEFAULT '',
    `scopes` JSON NOT NULL,
    `idp_entity_id` VARCHAR(255) NOT NULL DEFAULT '',
    `sso_url` VARCHAR(255) NOT NULL DEFAULT '',
    `slo_url` VARCHAR(255) NOT NULL DEFAULT '',
    `metadata` VARCHAR(255) NOT NULL DEFAULT '',
    `issuer_url` VARCHAR(255) NOT NULL DEFAULT '',
    `region` VARCHAR(255) NOT NULL DEFAULT '',
    `host` VARCHAR(255) NOT NULL DEFAULT '',
    `port` INT NOT NULL DEFAULT 0,
    `database_type` VARCHAR(255) NOT NULL DEFAULT '',
    `user_mapping` JSON NOT NULL,
    `template` VARCHAR(255) NOT NULL DEFAULT '',
    `custom_api` VARCHAR(255) NOT NULL DEFAULT '',
    `custom_header` VARCHAR(255) NOT NULL DEFAULT '',
    `custom_body` VARCHAR(255) NOT NULL DEFAULT '',
    `admin` VARCHAR(255) NOT NULL DEFAULT '',
    `password` TEXT NOT NULL,
    `app_id` VARCHAR(255) NOT NULL DEFAULT '',
    `app_secret` TEXT NOT NULL,
    `agent_id` VARCHAR(255) NOT NULL DEFAULT '',
    `content_type` VARCHAR(255) NOT NULL DEFAULT '',
    `sender` VARCHAR(255) NOT NULL DEFAULT '',
    `receiver` VARCHAR(255) NOT NULL DEFAULT '',
    `enabled` TINYINT(1) NOT NULL DEFAULT TRUE,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `created_by` CHAR(36),
    `updated_by` CHAR(36),
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_providers_family CHECK (family IS NULL OR family IN ('identity', 'ai', 'payment', 'notification')),
    CONSTRAINT ck_providers_kind CHECK (kind IS NULL OR (family = 'identity' AND kind IN ('github', 'google', 'azure_ad', 'azure_ad_b2c', 'facebook', 'linkedin', 'twitter', 'gitlab', 'gitee', 'okta', 'wechat', 'wechat_mobile', 'wechat_miniprogram', 'wecom_internal', 'wecom_third', 'dingtalk', 'lark', 'feishu', 'qq', 'weibo', 'baidu', 'douyin', 'kwai', 'bilibili', 'infoflow_internal', 'infoflow_third', 'telegram', 'ldap', 'adfs', 'saml', 'oidc', 'custom', 'goth', 'metamask', 'web3onboard', 'alipay', 'wechat_pay', 'smtp', 'default', 'local', 'faceid', 'apple', 'keycloak', 'slack', 'discord', 'steam', 'amazon', 'sendgrid', 'azure_acs', 'resend', 'submail_email', 'mailtrap_email', 'directmail', 'tencent_ses', 'sendcloud', 'mailgun', 'amazon_ses', 'postmark', 'mailersend', 'brevo', 'zoho_mail', 'custom_http', 'sms', 'twilio_sms', 'alibaba_sms', 'alibaba_pnvs', 'tencent_sms', 'volcengine_sms', 'huawei_sms', 'amazon_sns', 'azure_acs_sms', 'infobip', 'ucloud_sms', 'emay_sms', 'baidu_sms', 'submail', 'mailtrap_sms', 'oson_sms', 'mock_sms', 'twilio_verify', 'smsbao', 'msg91', 'custom_http_sms', 'unisms_sms', 'yunpian_sms', 'jdcloud_sms', 'vonage_sms', 'plivo_sms', 'telnyx_sms', 'sinch_sms', 'bird_sms', 'clickatell_sms', 'africastalking_sms', 'aws_s3', 'azure_blob', 'gcs', 'minio', 'ali_oss', 'tencent_cos', 'qiniu', 'cu_cloud', 'huawei_obs', 'cloudflare_r2', 'backblaze_b2', 'digitalocean_spaces', 'synology', 'recaptcha', 'recaptcha_v3', 'hcaptcha', 'turnstile', 'geetest', 'aliyun_captcha', 'tencent_captcha', 'totp', 'radius', 'push', 'aliyun_face_body', 'wechat_scan', 'dingtalk_scan', 'jumio', 'aliyun_idv', 'tencent_idv', 'onfido', 'veriff', 'persona', 'elasticsearch', 'loki')) OR (family = 'ai' AND kind IN ('openai', 'anthropic', 'gemini', 'deepseek', 'glm', 'kimi', 'qwen', 'seed', 'ollama', 'vllm', 'custom')) OR (family = 'payment' AND kind IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy', 'square', 'checkout')) OR (family = 'notification' AND kind IN ('email', 'sms', 'dingtalk', 'lark', 'wecom', 'slack', 'discord', 'rocketchat', 'teams', 'bark', 'telegram', 'pushover', 'line', 'matrix', 'pushbullet', 'viber', 'webpush', 'twitter', 'reddit', 'customhttp', 'googlechat', 'cucloud', 'ntfy', 'serverchan', 'pagerduty', 'opsgenie', 'gotify'))),
    CONSTRAINT ck_providers_variant CHECK (variant IS NULL OR (family = 'payment' AND variant IN ('sandbox', 'production')) OR (family IN ('identity', 'ai', 'notification') AND variant = '')),
    gen_providers_registration TINYINT(1) AS (CASE WHEN (family IS NOT NULL AND kind IS NOT NULL AND variant IS NOT NULL AND deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_providers_registration UNIQUE (`organization_id`, `family`, `kind`, `name`, `variant`, gen_providers_registration),
    CONSTRAINT fk_providers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'providers' AND INDEX_NAME = 'idx_providers_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_providers_organization_id` ON `providers` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'providers' AND COLUMN_NAME = 'gen_providers_category');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `providers` ADD COLUMN gen_providers_category TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'providers' AND INDEX_NAME = 'idx_providers_category' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_providers_category` ON `providers` (`category`(255), `gen_providers_category`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'providers' AND COLUMN_NAME = 'gen_providers_org_category');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `providers` ADD COLUMN gen_providers_org_category TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'providers' AND INDEX_NAME = 'idx_providers_org_category' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_providers_org_category` ON `providers` (`organization_id`, `category`(255), `gen_providers_org_category`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `provider_configs` (
    `provider_id` CHAR(36) NOT NULL,
    `config_type` VARCHAR(255) NOT NULL DEFAULT '',
    `config` JSON NOT NULL,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    PRIMARY KEY (`provider_id`, `config_type`),
    CONSTRAINT fk_provider_configs_provider_id_providers FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `provider_secrets` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `provider_id` CHAR(36) NOT NULL,
    `secret_type` VARCHAR(255) NOT NULL DEFAULT '',
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `last_rotated_at` TIMESTAMP,
    `expires_at` TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `version` BIGINT NOT NULL DEFAULT 1,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT fk_provider_secrets_provider_id_providers FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'provider_secrets' AND INDEX_NAME = 'idx_provider_secrets_provider_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_provider_secrets_provider_id` ON `provider_secrets` (`provider_id`, `updated_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `webhooks` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `url` VARCHAR(255) NOT NULL DEFAULT '',
    `method` VARCHAR(64) NOT NULL DEFAULT 'POST',
    `content_type` VARCHAR(64) NOT NULL DEFAULT 'application/json',
    `headers` VARCHAR(255) NOT NULL DEFAULT '',
    `events` JSON NOT NULL,
    `custom_body` VARCHAR(255) NOT NULL DEFAULT '',
    `custom_headers` JSON NOT NULL,
    `cert_id` VARCHAR(255) NOT NULL DEFAULT '',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `secret` VARCHAR(255) NOT NULL DEFAULT '',
    `is_error` TINYINT(1) NOT NULL DEFAULT false,
    `error_text` VARCHAR(255) NOT NULL DEFAULT '',
    `organization` VARCHAR(255) NOT NULL DEFAULT '',
    `token_fields` JSON NOT NULL,
    `object_fields` JSON NOT NULL,
    `is_user_extended` TINYINT(1) NOT NULL DEFAULT false,
    `single_org_only` TINYINT(1) NOT NULL DEFAULT false,
    `retry_count` INT NOT NULL DEFAULT 3,
    `max_retries` INT NOT NULL DEFAULT 3,
    `retry_interval` INT NOT NULL DEFAULT 60,
    `use_exponential_backoff` TINYINT(1) NOT NULL DEFAULT false,
    `timeout_ms` INT NOT NULL DEFAULT 5000,
    `retry_policy` JSON,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_webhooks_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_webhooks_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `agents` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(64) NOT NULL DEFAULT 'gateway-proxy',
    `host` VARCHAR(255) NOT NULL DEFAULT '',
    `port` INT NOT NULL DEFAULT 0,
    `url` VARCHAR(255) NOT NULL DEFAULT '',
    `token` VARCHAR(255) NOT NULL DEFAULT '',
    `application_id` CHAR(36),
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `last_ping_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_agents_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_agents_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_agents_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `verifications` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `type` TEXT NOT NULL,
    `recipient` TEXT NOT NULL,
    `code` TEXT NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `application_id` CHAR(36) COMMENT 'Application context used when the code was sent; drives provider resend resolution',
    `provider_id` CHAR(36) COMMENT 'Unified provider selected for this send (application binding or organization fallback)',
    `expires_at` TIMESTAMP NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_verifications_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'verifications' AND INDEX_NAME = 'idx_verifications_org_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_verifications_org_created_at_desc` ON `verifications` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'verifications' AND INDEX_NAME = 'idx_verifications_recipient' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_verifications_recipient` ON `verifications` (`recipient`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'verifications' AND INDEX_NAME = 'idx_verifications_type_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_verifications_type_status` ON `verifications` (`type`(255), `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_verifications_recipient_code_type_pending: predicate status = 'pending' (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'verifications' AND INDEX_NAME = 'idx_verifications_recipient_code_type_pending' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_verifications_recipient_code_type_pending` ON `verifications` (`recipient`(255), `code`(255), `type`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `captcha_providers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `provider_type` VARCHAR(255) NOT NULL DEFAULT '',
    `site_key` VARCHAR(255) NOT NULL DEFAULT '',
    `secret_key` TEXT NOT NULL,
    `config` JSON NOT NULL,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_captcha_providers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'captcha_providers' AND INDEX_NAME = 'idx_captcha_providers_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_captcha_providers_organization_id` ON `captcha_providers` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `face_id_providers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `provider_type` VARCHAR(255) NOT NULL DEFAULT '',
    `access_key_id` VARCHAR(255) NOT NULL DEFAULT '',
    `access_key_secret` VARCHAR(255) NOT NULL DEFAULT '',
    `endpoint` VARCHAR(255) NOT NULL DEFAULT '',
    `config` JSON NOT NULL,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_face_id_providers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'face_id_providers' AND INDEX_NAME = 'idx_face_id_providers_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_face_id_providers_organization_id` ON `face_id_providers` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `dpop_replay` (
    `jti` TEXT NOT NULL,
    `seen_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    PRIMARY KEY (`jti`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `webhook_events` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `webhook_id` CHAR(36) NOT NULL,
    `event_type` TEXT NOT NULL,
    `payload` JSON NOT NULL,
    `response_code` INT,
    `response_body` TEXT,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `retry_count` INT NOT NULL DEFAULT 0,
    `next_retry_at` TIMESTAMP,
    `idempotency_key` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_webhook_events_webhook_id_webhooks FOREIGN KEY (`webhook_id`) REFERENCES `webhooks`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `web3_identity_mappings` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL COMMENT 'Owning user — FK to guliangiam.users',
    `credential_id` CHAR(36) NOT NULL COMMENT 'FK to guliangiam.webauthn_credentials.id — the passkey whose pubkey was used to derive this address',
    `wallet_address` CHAR(42) NOT NULL COMMENT 'Derived EVM address (0x + 40 hex). Deterministic from passkey pubkey via keccak256(pubkey || "guliangiam-web3-v1")[12:]',
    `derivation_method` VARCHAR(64) NOT NULL DEFAULT 'keccak256-passkey' COMMENT 'Method used to derive the address. Currently only ''keccak256-passkey''. Reserved for future extension.',
    `derivation_seed` CHAR(64) NOT NULL COMMENT 'Hex-encoded keccak256(compressed_pubkey || "guliangiam-web3-v1") — full 32-byte hash captured for auditability',
    `chain_ids` JSON NOT NULL COMMENT 'Supported EVM chain IDs for this identity mapping',
    `ens_name` TEXT COMMENT 'Optional ENS name resolved at registration time',
    `is_primary` TINYINT(1) NOT NULL DEFAULT TRUE,
    `verified_at` TIMESTAMP NOT NULL COMMENT 'When the mapping was created/verified',
    `last_used_at` TIMESTAMP COMMENT 'Updated on each successful login via this passkey → address',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_web3_identity_mappings_wallet_address UNIQUE (`wallet_address`),
    CONSTRAINT uq_web3_identity_mappings_credential UNIQUE (`credential_id`),
    CONSTRAINT fk_web3_identity_mappings_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'web3_identity_mappings' AND INDEX_NAME = 'idx_web3_identity_mappings_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_web3_identity_mappings_user` ON `web3_identity_mappings` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `identity_verifications` (
    `id` CHAR(36) NOT NULL COMMENT 'Verification primary key' PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL COMMENT 'FK -> organizations.id (tenant boundary)',
    `user_id` CHAR(36) COMMENT 'FK -> users.id (nullable — pre-registration verification)',
    `provider` VARCHAR(32) NOT NULL COMMENT 'Verification provider (jumio/onfido/manual/etc)',
    `status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/in_progress/approved/rejected/expired',
    `redirect_url` TEXT COMMENT 'Post-verification redirect URL for provider callback',
    `session_token` VARCHAR(255) COMMENT 'Provider session correlation token',
    `provider_ref` VARCHAR(255) COMMENT 'Provider-side reference (session id / verification id)',
    `biz_seq` VARCHAR(32) COMMENT 'Merchant business sequence (32-char hex; fresh per request); unique constraint enforces idempotency / anti-replay',
    `action` VARCHAR(32) COMMENT 'Face action semantics (real_name_verify_cn/login_verify/real_name_verify_overseas); NULL = legacy row',
    `product_code` VARCHAR(32) COMMENT 'Provider product code (ID_PRO/ID_PLUS/LR_FR/eKYC_PRO/FACE_IDU/etc)',
    `subject_ref` VARCHAR(64) COMMENT 'De-identified subject reference (salted SM3 hex of the claimed ID number); never plaintext PII',
    `first_name` VARCHAR(64) COMMENT 'Given name (from provider)',
    `last_name` VARCHAR(64) COMMENT 'Family name (from provider)',
    `date_of_birth` DATE COMMENT 'Date of birth (from provider)',
    `nationality` VARCHAR(3) COMMENT 'ISO 3166-1 alpha-2 country code',
    `id_type` VARCHAR(32) COMMENT 'Government-issued ID type (passport/driver_license/etc)',
    `id_number` VARCHAR(128) COMMENT 'Government-issued ID number (PII — encrypted at rest)',
    `confidence` DECIMAL(19, 4) COMMENT 'Provider confidence score (0.0 – 1.0)',
    `reason` TEXT COMMENT 'Reject reason or completion note',
    `raw_result` JSON COMMENT 'Complete provider payload (PII — encrypted at rest)',
    `id_number_sealed` TEXT COMMENT 'v2 envelope of id_number (PII); NULL = legacy plaintext row remains',
    `raw_result_sealed` MEDIUMTEXT COMMENT 'v2 envelope of the raw_result plaintext payload (PII/audit); NULL = legacy plaintext row remains',
    `initiated_at` TIMESTAMP NOT NULL COMMENT 'When user started verification',
    `completed_at` TIMESTAMP COMMENT 'When provider returned final result',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_idv_status CHECK (status IN ('pending', 'in_progress', 'approved', 'rejected', 'expired')),
    CONSTRAINT ck_idv_action CHECK (action IS NULL OR action IN ('real_name_verify_cn', 'login_verify', 'real_name_verify_overseas')),
    CONSTRAINT uq_idv_biz_seq UNIQUE (`biz_seq`),
    CONSTRAINT fk_identity_verifications_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_identity_verifications_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Identity verification durable records — survives provider session lifetime';
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'identity_verifications' AND INDEX_NAME = 'idx_idv_org_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_idv_org_user` ON `identity_verifications` (`organization_id`, `user_id`, `initiated_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'identity_verifications' AND INDEX_NAME = 'idx_idv_provider_ref' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_idv_provider_ref` ON `identity_verifications` (`provider`, `provider_ref`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_face_templates` (
    `id` CHAR(36) NOT NULL COMMENT 'Template primary key' PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL COMMENT 'FK -> organizations.id (tenant boundary)',
    `user_id` CHAR(36) NOT NULL COMMENT 'FK -> users.id (one template per user)',
    `template_ref` TEXT NOT NULL COMMENT 'Envelope-encrypted reference to face image/feature (PII — never plaintext)',
    `provider` VARCHAR(32) COMMENT 'Face provider that enrolled the template (aliyun/lr_fr/localuniface/etc)',
    `provider_ref` VARCHAR(255) COMMENT 'Provider-side subject reference (e.g. MerchantUserId / face group code)',
    `status` VARCHAR(16) NOT NULL DEFAULT 'active' COMMENT 'active/revoked',
    `revoked_at` TIMESTAMP COMMENT 'When the template was revoked',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_user_face_templates_status CHECK (status IN ('active', 'revoked')),
    CONSTRAINT uq_user_face_templates_user UNIQUE (`user_id`),
    CONSTRAINT fk_user_face_templates_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_user_face_templates_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='FaceID login templates — encrypted references only, never plaintext images';
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_face_templates' AND INDEX_NAME = 'idx_user_face_templates_org_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_face_templates_org_status` ON `user_face_templates` (`organization_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `custom_attribute_definitions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `key` TEXT NOT NULL COMMENT 'Attribute key used inside user_profiles.properties (for example department)',
    `data_type` VARCHAR(16) NOT NULL DEFAULT 'string' COMMENT 'string | number | boolean | date | enum',
    `required` TINYINT(1) NOT NULL DEFAULT false,
    `options` JSON NOT NULL COMMENT 'Allowed values for data_type=enum (JSON array of strings)',
    `writable` TINYINT(1) NOT NULL DEFAULT true,
    `visibility` VARCHAR(16) NOT NULL DEFAULT 'private' COMMENT 'private | public | admin',
    `description` TEXT NOT NULL,
    `enabled` TINYINT(1) NOT NULL DEFAULT true,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_custom_attribute_definitions_data_type CHECK (data_type IN ('string', 'number', 'boolean', 'date', 'enum')),
    CONSTRAINT ck_custom_attribute_definitions_visibility CHECK (visibility IN ('private', 'public', 'admin')),
    CONSTRAINT fk_custom_attribute_definitions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'custom_attribute_definitions' AND INDEX_NAME = 'uq_custom_attribute_definitions_org_key' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE UNIQUE INDEX `uq_custom_attribute_definitions_org_key` ON `custom_attribute_definitions` (`organization_id`, `key`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'custom_attribute_definitions' AND INDEX_NAME = 'idx_custom_attribute_definitions_org_enabled' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_custom_attribute_definitions_org_enabled` ON `custom_attribute_definitions` (`organization_id`, `enabled`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `app_client_jwks_uris` (
    `application_id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `jwks_uri` TEXT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_app_client_jwks_uris_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_app_client_jwks_uris_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- 分组：30_authz.yaml

CREATE TABLE IF NOT EXISTS `roles` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` TEXT NOT NULL,
    `description` VARCHAR(255) NOT NULL DEFAULT '',
    `group_ids` JSON NOT NULL,
    `role_ids` JSON NOT NULL,
    `domains` JSON NOT NULL,
    `users` JSON NOT NULL,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_roles_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_roles_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_roles_org_enabled: predicate is_enabled = true (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'roles' AND INDEX_NAME = 'idx_roles_org_enabled' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_roles_org_enabled` ON `roles` (`organization_id`, `is_enabled`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_role_bindings` (
    `user_id` CHAR(36) NOT NULL,
    `role_id` CHAR(36) NOT NULL,
    `source` VARCHAR(64) NOT NULL DEFAULT 'manual',
    `starts_at` TIMESTAMP,
    `expires_at` TIMESTAMP,
    `lifecycle_id` CHAR(36),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    PRIMARY KEY (`user_id`, `role_id`),
    CONSTRAINT fk_user_role_bindings_role_id_roles FOREIGN KEY (`role_id`) REFERENCES `roles`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_user_role_bindings_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_role_bindings' AND INDEX_NAME = 'idx_user_role_bindings_role_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_role_bindings_role_id` ON `user_role_bindings` (`role_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_role_bindings' AND INDEX_NAME = 'idx_user_role_bindings_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_role_bindings_user_id` ON `user_role_bindings` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `permissions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` TEXT NOT NULL,
    `description` VARCHAR(255) NOT NULL DEFAULT '',
    `users` JSON NOT NULL,
    `groups` JSON NOT NULL,
    `roles` JSON NOT NULL,
    `domains` JSON NOT NULL,
    `resource_type` VARCHAR(255) NOT NULL DEFAULT '',
    `resources` JSON NOT NULL,
    `actions` JSON NOT NULL,
    `effect` VARCHAR(64) NOT NULL DEFAULT 'Allow',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `submitter` VARCHAR(255) NOT NULL DEFAULT '',
    `approver` VARCHAR(255) NOT NULL DEFAULT '',
    `status` VARCHAR(255) NOT NULL DEFAULT '',
    `model` VARCHAR(255) NOT NULL DEFAULT '',
    `adapter` VARCHAR(255) NOT NULL DEFAULT '',
    `sorter` VARCHAR(255) NOT NULL DEFAULT '',
    `application` VARCHAR(255) NOT NULL DEFAULT '',
    `approve_time` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_permissions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'permissions' AND INDEX_NAME = 'idx_permissions_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_permissions_organization_id` ON `permissions` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_permissions_org_enabled: predicate is_enabled = true (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'permissions' AND INDEX_NAME = 'idx_permissions_org_enabled' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_permissions_org_enabled` ON `permissions` (`organization_id`, `is_enabled`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `groups` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` TEXT NOT NULL,
    `manager` VARCHAR(255) NOT NULL DEFAULT '',
    `contact_email` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(255) NOT NULL DEFAULT '',
    `parent_id` CHAR(36),
    `is_top_group` TINYINT(1) NOT NULL DEFAULT false,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `homepage` VARCHAR(255) NOT NULL DEFAULT '',
    `users` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_groups_organization_id_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_groups_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_groups_parent_id_groups FOREIGN KEY (`parent_id`) REFERENCES `groups`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'groups' AND INDEX_NAME = 'idx_groups_parent_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_groups_parent_id` ON `groups` (`organization_id`, `parent_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_groups_org_enabled: predicate is_enabled = true (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'groups' AND INDEX_NAME = 'idx_groups_org_enabled' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_groups_org_enabled` ON `groups` (`organization_id`, `is_enabled`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_group_memberships` (
    `user_id` CHAR(36) NOT NULL,
    `group_id` CHAR(36) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    PRIMARY KEY (`user_id`, `group_id`),
    CONSTRAINT fk_user_group_memberships_group_id_groups FOREIGN KEY (`group_id`) REFERENCES `groups`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_user_group_memberships_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_group_memberships' AND INDEX_NAME = 'idx_user_group_memberships_group_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_group_memberships_group_id` ON `user_group_memberships` (`group_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_group_memberships' AND INDEX_NAME = 'idx_user_group_memberships_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_group_memberships_user_id` ON `user_group_memberships` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `casbin_rules` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `enforcer_id` CHAR(36) NOT NULL,
    `ptype` VARCHAR(255) NOT NULL DEFAULT '',
    `v0` VARCHAR(255) NOT NULL DEFAULT '',
    `v1` VARCHAR(255) NOT NULL DEFAULT '',
    `v2` VARCHAR(255) NOT NULL DEFAULT '',
    `v3` VARCHAR(255) NOT NULL DEFAULT '',
    `v4` VARCHAR(255) NOT NULL DEFAULT '',
    `v5` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_casbin_rules_rule UNIQUE (`enforcer_id`(36), `ptype`(96), `v0`(96), `v1`(96), `v2`(96), `v3`(96), `v4`(96), `v5`(96))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'casbin_rules' AND INDEX_NAME = 'idx_casbin_rules_ptype' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_casbin_rules_ptype` ON `casbin_rules` (`ptype`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `resources` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `application_id` CHAR(36),
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(64) NOT NULL DEFAULT 'file',
    `tag` VARCHAR(255) NOT NULL DEFAULT '',
    `file_path` VARCHAR(255) NOT NULL DEFAULT '',
    `file_size` BIGINT NOT NULL DEFAULT 0,
    `mime_type` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_resources_org_app_name UNIQUE (`organization_id`, `application_id`, `name`(255)),
    CONSTRAINT fk_resources_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_resources_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'resources' AND INDEX_NAME = 'idx_resources_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_resources_type` ON `resources` (`type`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `models` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` VARCHAR(255) NOT NULL DEFAULT '',
    `model_text` TEXT NOT NULL,
    `owner` VARCHAR(255) NOT NULL DEFAULT '',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT TRUE,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_models_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_models_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `adapters` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(64) NOT NULL DEFAULT 'database',
    `server` VARCHAR(255) NOT NULL DEFAULT '',
    `database` VARCHAR(255) NOT NULL DEFAULT '',
    `table_name` VARCHAR(255) NOT NULL DEFAULT '',
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `password` VARCHAR(255) NOT NULL DEFAULT '',
    `owner` VARCHAR(255) NOT NULL DEFAULT '',
    `use_same_db` TINYINT(1) NOT NULL DEFAULT TRUE,
    `database_type` VARCHAR(64) NOT NULL DEFAULT 'postgresql',
    `host` VARCHAR(255) NOT NULL DEFAULT '',
    `port` INT,
    `db_user` VARCHAR(255) NOT NULL DEFAULT '',
    `db_password` VARCHAR(255) NOT NULL DEFAULT '',
    `database_name` VARCHAR(255) NOT NULL DEFAULT '',
    `policy` VARCHAR(255) NOT NULL DEFAULT '',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT TRUE,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_adapters_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_adapters_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `enforcers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` VARCHAR(255) NOT NULL DEFAULT '',
    `model_id` CHAR(36),
    `adapter_id` CHAR(36),
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `owner` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_enforcers_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_enforcers_adapter_id_adapters FOREIGN KEY (`adapter_id`) REFERENCES `adapters`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_enforcers_model_id_models FOREIGN KEY (`model_id`) REFERENCES `models`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_enforcers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'enforcers' AND INDEX_NAME = 'idx_enforcers_is_enabled' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_enforcers_is_enabled` ON `enforcers` (`is_enabled`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `syncers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(64) NOT NULL DEFAULT 'database',
    `database_type` VARCHAR(255) NOT NULL DEFAULT '',
    `host` VARCHAR(255) NOT NULL DEFAULT '',
    `port` INT NOT NULL DEFAULT 0,
    `database` VARCHAR(255) NOT NULL DEFAULT '',
    `table_name` VARCHAR(255) NOT NULL DEFAULT '',
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `password` VARCHAR(255) NOT NULL DEFAULT '',
    `ssl_mode` VARCHAR(64) NOT NULL DEFAULT 'disable',
    `ssh_type` VARCHAR(255) NOT NULL DEFAULT '',
    `ssh_host` VARCHAR(255) NOT NULL DEFAULT '',
    `ssh_port` INT NOT NULL DEFAULT 0,
    `ssh_username` VARCHAR(255) NOT NULL DEFAULT '',
    `ssh_password` VARCHAR(255) NOT NULL DEFAULT '',
    `ssh_cert_id` VARCHAR(255) NOT NULL DEFAULT '',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `sync_interval` INT NOT NULL DEFAULT 0,
    `field_mapping` VARCHAR(255) NOT NULL DEFAULT '',
    `column_mapping` JSON NOT NULL,
    `is_read_only` TINYINT(1) NOT NULL DEFAULT false,
    `affinity` VARCHAR(255) NOT NULL DEFAULT '',
    `affiliation_table` VARCHAR(255) NOT NULL DEFAULT '',
    `avatar_base_url` VARCHAR(255) NOT NULL DEFAULT '',
    `error_text` VARCHAR(255) NOT NULL DEFAULT '',
    `last_sync_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_syncers_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_syncers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_syncers_is_enabled: predicate is_enabled = true (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'syncers' AND INDEX_NAME = 'idx_syncers_is_enabled' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_syncers_is_enabled` ON `syncers` (`is_enabled`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `relation_namespaces` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `relation_defs` JSON NOT NULL,
    `version` INT NOT NULL DEFAULT 1,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `revision_counter` BIGINT NOT NULL DEFAULT 0,
    CONSTRAINT uq_relation_namespaces_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_relation_namespaces_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `relation_tuples` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `namespace_id` CHAR(36) NOT NULL,
    `subject` TEXT NOT NULL,
    `subject_relation` TEXT,
    `relation` TEXT NOT NULL,
    `object` TEXT NOT NULL,
    `zookie` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT uq_relation_tuples_org_ns_sub_rel_obj UNIQUE (`organization_id`(36), `namespace_id`(36), `subject`(153), `relation`(153), `object`(153)),
    CONSTRAINT fk_relation_tuples_namespace_id_relation_namespaces FOREIGN KEY (`namespace_id`) REFERENCES `relation_namespaces`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_relation_tuples_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'relation_tuples' AND INDEX_NAME = 'idx_relation_tuples_object' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_relation_tuples_object` ON `relation_tuples` (`object`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'relation_tuples' AND INDEX_NAME = 'idx_relation_tuples_subject' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_relation_tuples_subject` ON `relation_tuples` (`subject`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


-- 分组：40_session.yaml

CREATE TABLE IF NOT EXISTS `sessions` (
    `id` CHAR(36) NOT NULL COMMENT 'Session primary key (UUID)' PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL COMMENT 'Owner user',
    `application_id` CHAR(36) NOT NULL COMMENT 'Client application the session was issued for',
    `session_state` VARCHAR(64) NOT NULL DEFAULT 'active' COMMENT 'Lifecycle: active / revoked / expired / idle_timeout',
    `sid` TEXT COMMENT 'Opaque session identifier used for session management lookups',
    `device_info` JSON NOT NULL COMMENT 'Client device fingerprint + OS/browser detail snapshot captured at login',
    `ip` VARCHAR(45) COMMENT 'IP address observed at session creation / last-seen refresh',
    `user_agent` TEXT COMMENT 'Raw User-Agent header captured at session creation',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Hard expiration timestamp; session becomes unusable after this point',
    `revoked_at` TIMESTAMP COMMENT 'Explicit revocation timestamp (admin revoke or logout); NULL when active',
    `last_seen_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last observed activity timestamp for session restore, idle timeout, and device management flows.',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Session creation timestamp',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last row modification timestamp',
    `authentication_level` VARCHAR(20) NOT NULL DEFAULT 'password' COMMENT 'How the user authenticated when this session was created: password / mfa / fido2. Used by concurrent-session policy (admin session exemption requires level >= mfa).',
    `assurance_level` VARCHAR(10) NOT NULL DEFAULT 'aal1' COMMENT 'NIST SP 800-63B authenticator assurance level: aal1 (password only) / aal2 (MFA or FIDO2 satisfied). step-up raises only the current session.',
    `is_privileged` TINYINT(1) NOT NULL DEFAULT false COMMENT 'Session elevated through an approved IGA access request',
    `lifecycle_id` CHAR(36) COMMENT 'IGA lifecycle that authorizes the privileged session',
    `ticket_id` CHAR(36) COMMENT 'Access request that authorized the privileged session',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    CONSTRAINT uq_sessions_sid UNIQUE (`sid`(255)),
    CONSTRAINT ck_sessions_state CHECK (session_state IN ('active', 'idle', 'expired', 'revoked')),
    CONSTRAINT ck_sessions_assurance_level CHECK (assurance_level IN ('aal1', 'aal2')),
    CONSTRAINT fk_sessions_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_sessions_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_expires_at` ON `sessions` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_sessions_user_active: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_user_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_user_active` ON `sessions` (`user_id`, `expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_application_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_application_id` ON `sessions` (`application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_sessions_user_app_active: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_user_app_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_user_app_active` ON `sessions` (`user_id`, `application_id`, `expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_user_expires_revoked' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_user_expires_revoked` ON `sessions` (`user_id`, `expires_at`, `revoked_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_expires_revoked' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_expires_revoked` ON `sessions` (`expires_at`, `revoked_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_org_app_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_org_app_created` ON `sessions` (`user_id`, `application_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sessions' AND INDEX_NAME = 'idx_sessions_user_id_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sessions_user_id_created` ON `sessions` (`user_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `access_tokens` (
    `id` CHAR(36) NOT NULL COMMENT 'Access token primary key (UUID)' PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL COMMENT 'Token owner user',
    `application_id` CHAR(36) NOT NULL COMMENT 'Client application the token was issued for',
    `token_hash` TEXT NOT NULL COMMENT 'SHA-256 hash of the token value used for lookup and revocation.',
    `token_prefix` TEXT NOT NULL COMMENT 'Non-sensitive prefix shown in management UIs instead of exposing the full token value.',
    `device_secret_hash` TEXT COMMENT 'SHA-256 of the bound device secret; set only for Proof-of-Possession tokens',
    `device_secret_expires_in` INT NOT NULL DEFAULT 0 COMMENT 'Seconds until device_secret_hash expires; 0 when not bound',
    `scopes` JSON NOT NULL COMMENT 'Space-separated OAuth scopes granted to this token',
    `audience` JSON NOT NULL COMMENT 'Intended API audiences (JWT aud claim)',
    `metadata` JSON NOT NULL COMMENT 'Extension attribute bag; may contain authorization_code_id, dpop_jkt, org_idp context etc.',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Token lifetime end; unix timestamp in seconds',
    `revoked_at` TIMESTAMP COMMENT 'Revocation timestamp; NULL when token is active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Token issuance timestamp',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last row modification timestamp',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    CONSTRAINT uq_access_tokens_hash UNIQUE (`token_hash`(255)),
    CONSTRAINT fk_access_tokens_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_access_tokens_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_access_tokens_authorization_code_id: 表达式索引 ["(metadata ->> 'authorizationCodeId')"] MySQL 无等价实现，已跳过
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_expires_at` ON `access_tokens` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_access_tokens_user_active: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_user_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_user_active` ON `access_tokens` (`user_id`, `expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_application_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_application_id` ON `access_tokens` (`application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_access_tokens_expires_at_not_revoked: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_expires_at_not_revoked' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_expires_at_not_revoked` ON `access_tokens` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_access_tokens_device_secret_hash: predicate device_secret_hash IS NOT NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_device_secret_hash' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_device_secret_hash` ON `access_tokens` (`device_secret_hash`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_tokens_app_expires' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_tokens_app_expires` ON `access_tokens` (`application_id`, `expires_at`, `revoked_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_user_app' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_user_app` ON `access_tokens` (`user_id`, `application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_user_id_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_user_id_created` ON `access_tokens` (`user_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_tokens' AND INDEX_NAME = 'idx_access_tokens_application_id_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_tokens_application_id_created` ON `access_tokens` (`application_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `refresh_tokens` (
    `id` CHAR(36) NOT NULL COMMENT 'Refresh token primary key (UUID)' PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL COMMENT 'Token owner user',
    `application_id` CHAR(36) NOT NULL COMMENT 'Client application the token was issued for',
    `token_hash` TEXT NOT NULL COMMENT 'SHA-256 of the refresh token; stored instead of plaintext',
    `token_prefix` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'First N chars of plaintext token shown in admin UIs for identification',
    `device_secret_hash` TEXT COMMENT 'Bound-device proof hash; refresh token rotation verifies same device',
    `device_secret_expires_in` INT NOT NULL DEFAULT 0 COMMENT 'Seconds until device_secret_hash expires',
    `scopes` JSON NOT NULL COMMENT 'Scopes this refresh token is restricted to (narrowest of parent access token)',
    `audience` JSON NOT NULL COMMENT 'Audience carried forward from the access token family',
    `access_token_id` CHAR(36) COMMENT 'The access token this refresh token was first minted with (link for revocation chains)',
    `family_id` CHAR(36) COMMENT 'Rotating-token family UUID; shared across token rotation generations for reuse detection',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Hard expiration; refresh rejected after this point even if unused',
    `revoked_at` TIMESTAMP COMMENT 'Revocation timestamp; part of reuse-detection flow (rotation marks previous gen revoked)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'First-generation issuance timestamp',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last row modification timestamp',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    CONSTRAINT uq_refresh_tokens_hash UNIQUE (`token_hash`(255)),
    CONSTRAINT fk_refresh_tokens_access_token_id_access_tokens FOREIGN KEY (`access_token_id`) REFERENCES `access_tokens`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_refresh_tokens_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_refresh_tokens_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_refresh_tokens_family_id_refresh_tokens FOREIGN KEY (`family_id`) REFERENCES `refresh_tokens`(`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_refresh_tokens_family_id: predicate family_id IS NOT NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_family_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_family_id` ON `refresh_tokens` (`family_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_access_token_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_access_token_id` ON `refresh_tokens` (`access_token_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_user_id` ON `refresh_tokens` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_refresh_tokens_expires_at_not_revoked: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_expires_at_not_revoked' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_expires_at_not_revoked` ON `refresh_tokens` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_application_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_application_id` ON `refresh_tokens` (`application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_refresh_tokens_hash: predicate token_hash IS NOT NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_hash' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_hash` ON `refresh_tokens` (`token_hash`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_refresh_tokens_family_revoked: predicate family_id IS NOT NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_family_revoked' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_family_revoked` ON `refresh_tokens` (`family_id`, `revoked_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_user_app' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_user_app` ON `refresh_tokens` (`user_id`, `application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_user_id_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_user_id_created` ON `refresh_tokens` (`user_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'refresh_tokens' AND INDEX_NAME = 'idx_refresh_tokens_application_id_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_refresh_tokens_application_id_created` ON `refresh_tokens` (`application_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `login_challenges` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) COMMENT 'Tenant org of the user attempting login',
    `user_id` CHAR(36) COMMENT 'User being authenticated (populated after account identification step)',
    `application_id` CHAR(36) COMMENT 'Client app that started the login flow',
    `challenge_type` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'MFA factor requested: totp / webauthn / sms / email / push / qr',
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending' COMMENT 'Current login challenge state such as pending, push_pending, approved, or rejected.',
    `method_type` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'MFA method currently associated with the challenge when state transitions need method-specific handling.',
    `verified_at` TIMESTAMP COMMENT 'Timestamp when an external approval path marked the challenge approved.',
    `identity_provider` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'IdP identifier for federated MFA; empty for local factors',
    `identity_sub` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'IdP subject claim for federated MFA correlation',
    `redirect_url` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Post-login callback URL; stored at challenge creation, sent to the approval channel',
    `device_info` JSON NOT NULL COMMENT 'Device fingerprint at challenge initiation',
    `ip` VARCHAR(45) COMMENT 'Request IP at challenge creation',
    `user_agent` TEXT COMMENT 'User-Agent header at challenge creation',
    `metadata` JSON NOT NULL COMMENT 'Attribute bag for factor-specific state (e.g. TOTP window, WebAuthn cred ID)',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Expiration timestamp for the pending MFA login challenge.',
    `resolved_at` TIMESTAMP COMMENT 'Timestamp when challenge transitioned to approved/rejected state',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Challenge initiation timestamp',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last state transition timestamp',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    CONSTRAINT ck_login_challenges_status CHECK (status IN ('pending', 'push_pending', 'approved', 'rejected')),
    CONSTRAINT fk_login_challenges_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_login_challenges_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_login_challenges_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Pending MFA login challenges created after password verification and consumed by second-step MFA verification.';
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'login_challenges' AND INDEX_NAME = 'idx_auth_login_challenges_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_auth_login_challenges_user_id` ON `login_challenges` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'login_challenges' AND INDEX_NAME = 'idx_auth_login_challenges_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_auth_login_challenges_expires_at` ON `login_challenges` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'login_challenges' AND INDEX_NAME = 'idx_auth_login_challenges_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_auth_login_challenges_status` ON `login_challenges` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'login_challenges' AND INDEX_NAME = 'idx_login_challenges_application_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_login_challenges_application_id` ON `login_challenges` (`application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `webauthn_credentials` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL,
    `credential_id` BLOB NOT NULL,
    `public_key` BLOB NOT NULL,
    `attestation_type` VARCHAR(255) NOT NULL DEFAULT '',
    `transport` JSON NOT NULL,
    `sign_count` BIGINT NOT NULL DEFAULT 0,
    `name` VARCHAR(255) NOT NULL DEFAULT '',
    `aaguid` CHAR(36),
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_webauthn_credentials_status CHECK (status IN ('active', 'revoked', 'deactivated')),
    CONSTRAINT fk_webauthn_credentials_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'webauthn_credentials' AND INDEX_NAME = 'idx_webauthn_credentials_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_webauthn_credentials_user_id` ON `webauthn_credentials` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_webauthn_credentials_aaguid: predicate aaguid IS NOT NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'webauthn_credentials' AND INDEX_NAME = 'idx_webauthn_credentials_aaguid' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_webauthn_credentials_aaguid` ON `webauthn_credentials` (`aaguid`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'webauthn_credentials' AND INDEX_NAME = 'uq_webauthn_credentials_credential_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE UNIQUE INDEX `uq_webauthn_credentials_credential_id` ON `webauthn_credentials` (`credential_id`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `webauthn_challenges` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `challenge_hash` TEXT NOT NULL COMMENT 'SHA-256 (hex) of the challenge issued by begin. The plaintext challenge is never persisted, so a database leak cannot be replayed as a valid ceremony.',
    `ceremony_type` TEXT NOT NULL COMMENT 'registration or authentication — the two ceremony kinds share one table but must not be interchangeable at finish. No TEXT default (check-schema-defaults.py freezes the legacy count of TEXT defaults; the service always sets this column).',
    `user_id` CHAR(36) NOT NULL,
    `organization_id` CHAR(36),
    `relying_party_id` TEXT NOT NULL COMMENT 'RP ID captured at begin; finish verifies the assertion against this value rather than against whatever the client sends. No TEXT default for the same reason as ceremony_type.',
    `expires_at` TIMESTAMP NOT NULL,
    `consumed_at` TIMESTAMP COMMENT 'Set exactly once by the atomic consume UPDATE. NULL means pending — the one-time guarantee is the RowsAffected=0 check on that conditional update.',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_webauthn_challenges_ceremony_type CHECK (ceremony_type IN ('registration', 'authentication', 'web3_authentication')),
    CONSTRAINT fk_webauthn_challenges_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_webauthn_challenges_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'webauthn_challenges' AND INDEX_NAME = 'idx_webauthn_challenges_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_webauthn_challenges_user_id` ON `webauthn_challenges` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'webauthn_challenges' AND INDEX_NAME = 'idx_webauthn_challenges_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_webauthn_challenges_expires_at` ON `webauthn_challenges` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `trusted_devices` (
    `id` CHAR(36) NOT NULL COMMENT 'Trusted device primary key (UUIDv7 generated by the application)' PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant that owns the trust record; also the RLS anchor',
    `user_id` CHAR(36) NOT NULL COMMENT 'User this device is trusted for',
    `device_credential_id` TEXT NOT NULL COMMENT 'SHA-256 digest of the client device credential, stored as 64 lowercase hex characters; the plaintext credential is never persisted',
    `device_public_key` BLOB NOT NULL COMMENT 'Device public key (PKIX DER) used to verify the trust ceremony signature',
    `ua_hash` TEXT NOT NULL COMMENT 'SHA-256 hex of the User-Agent observed at registration; device trust is bound to the client profile, never to an IP address',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Registration timestamp',
    `last_seen_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last time the device presented a valid trust proof',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Hard expiration (created_at + 30 days); expired rows are no longer trusted',
    `revoked_at` TIMESTAMP COMMENT 'Revocation timestamp; NULL while the device is trusted',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    CONSTRAINT fk_trusted_devices_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_trusted_devices_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_trusted_devices_user_active: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'trusted_devices' AND INDEX_NAME = 'idx_trusted_devices_user_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_trusted_devices_user_active` ON `trusted_devices` (`user_id`, `expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'trusted_devices' AND INDEX_NAME = 'idx_trusted_devices_org_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_trusted_devices_org_user` ON `trusted_devices` (`organization_id`, `user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'trusted_devices' AND INDEX_NAME = 'idx_trusted_devices_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_trusted_devices_expires_at` ON `trusted_devices` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `remember_me_families` (
    `id` CHAR(36) NOT NULL COMMENT 'Remember-me family primary key (UUID)' PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant that owns the family',
    `user_id` CHAR(36) NOT NULL COMMENT 'User the family restores',
    `application_id` CHAR(36) COMMENT 'Application the family was issued for',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Hard family expiration (issued_at + 30 days)',
    `revoked_at` TIMESTAMP COMMENT 'Revocation timestamp; reuse detection revokes the whole family',
    `revoke_reason` TEXT COMMENT 'logout / reuse_detected / admin / user',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Issuance timestamp',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last row modification timestamp',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    CONSTRAINT fk_remember_me_families_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_remember_me_families_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_remember_me_families_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_remember_me_families_user_active: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'remember_me_families' AND INDEX_NAME = 'idx_remember_me_families_user_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_remember_me_families_user_active` ON `remember_me_families` (`user_id`, `expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'remember_me_families' AND INDEX_NAME = 'idx_remember_me_families_org_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_remember_me_families_org_user` ON `remember_me_families` (`organization_id`, `user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'remember_me_families' AND INDEX_NAME = 'idx_remember_me_families_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_remember_me_families_expires_at` ON `remember_me_families` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `remember_me_tokens` (
    `id` CHAR(36) NOT NULL COMMENT 'Remember-me token primary key (UUID)' PRIMARY KEY,
    `family_id` CHAR(36) NOT NULL COMMENT 'Rotating family this token belongs to',
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant that owns the token (RLS anchor)',
    `token_hash` TEXT NOT NULL COMMENT 'SHA-256 hex of the plaintext token; plaintext is returned only once',
    `token_prefix` TEXT NOT NULL COMMENT 'Non-sensitive prefix for management UIs',
    `consumed_at` TIMESTAMP COMMENT 'Single-use consumption timestamp; non-NULL means already rotated',
    `expires_at` TIMESTAMP NOT NULL COMMENT 'Token expiration (mirrors the family expiration)',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Issuance timestamp',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last row modification timestamp',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic concurrency counter',
    CONSTRAINT uq_remember_me_tokens_hash UNIQUE (`token_hash`(255)),
    CONSTRAINT fk_remember_me_tokens_family_id_remember_me_families FOREIGN KEY (`family_id`) REFERENCES `remember_me_families`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_remember_me_tokens_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'remember_me_tokens' AND INDEX_NAME = 'idx_remember_me_tokens_family' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_remember_me_tokens_family` ON `remember_me_tokens` (`family_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_remember_me_tokens_expires_at: predicate consumed_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'remember_me_tokens' AND INDEX_NAME = 'idx_remember_me_tokens_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_remember_me_tokens_expires_at` ON `remember_me_tokens` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


-- 分组：50_protocol.yaml

CREATE TABLE IF NOT EXISTS `authorization_codes` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL,
    `application_id` CHAR(36) NOT NULL,
    `code_hash` TEXT NOT NULL,
    `code_prefix` VARCHAR(255) NOT NULL DEFAULT '',
    `redirect_uri` VARCHAR(255) NOT NULL DEFAULT '',
    `scopes` JSON NOT NULL,
    `status` VARCHAR(255) NOT NULL DEFAULT '',
    `sid` VARCHAR(255) NOT NULL DEFAULT '',
    `nonce` VARCHAR(255) NOT NULL DEFAULT '',
    `resource` VARCHAR(255) NOT NULL DEFAULT '',
    `code_challenge` VARCHAR(255) NOT NULL DEFAULT '',
    `code_challenge_method` VARCHAR(255) NOT NULL DEFAULT '',
    `auth_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `expires_at` TIMESTAMP NOT NULL,
    `used_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_authorization_codes_hash UNIQUE (`code_hash`(255)),
    CONSTRAINT fk_authorization_codes_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_authorization_codes_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authorization_codes' AND INDEX_NAME = 'idx_authorization_codes_expires_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_authorization_codes_expires_at` ON `authorization_codes` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `consents` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL,
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `organization_id` CHAR(36),
    `organization_slug` VARCHAR(255) NOT NULL DEFAULT '',
    `application_id` CHAR(36) NOT NULL,
    `application_slug` VARCHAR(255) NOT NULL DEFAULT '',
    `scopes` JSON NOT NULL,
    `granted_scopes` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revoked_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_consents_user_application UNIQUE (`user_id`, `application_id`),
    CONSTRAINT fk_consents_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_consents_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_consents_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
-- INDEX idx_consents_application_active: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'consents' AND INDEX_NAME = 'idx_consents_application_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_consents_application_active` ON `consents` (`application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_consents_user_active: predicate revoked_at IS NULL (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'consents' AND INDEX_NAME = 'idx_consents_user_active' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_consents_user_active` ON `consents` (`user_id`, `application_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `device_auth_codes` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `device_code` TEXT NOT NULL,
    `user_code` TEXT NOT NULL,
    `client_id` VARCHAR(255) NOT NULL DEFAULT '',
    `scope` VARCHAR(255) NOT NULL DEFAULT '',
    `expires_at` TIMESTAMP NOT NULL,
    `poll_interval_sec` INT NOT NULL DEFAULT 5,
    `verified` TINYINT(1) NOT NULL DEFAULT false,
    `verified_at` TIMESTAMP,
    `user_id` CHAR(36),
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `organization_id` CHAR(36),
    `last_polled_at` TIMESTAMP,
    `slow_down_count` INT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_device_auth_codes_device_code UNIQUE (`device_code`(255)),
    CONSTRAINT uq_device_auth_codes_user_code UNIQUE (`user_code`(255)),
    CONSTRAINT fk_device_auth_codes_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_device_auth_codes_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `qr_code_sessions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `code_id` TEXT NOT NULL,
    `code` VARCHAR(255) NOT NULL DEFAULT '',
    `organization_id` CHAR(36) NOT NULL,
    `application_id` CHAR(36) NOT NULL,
    `expires_at` TIMESTAMP NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `user_id` CHAR(36),
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `ip_address` VARCHAR(255) NOT NULL DEFAULT '',
    `user_agent` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_qr_code_sessions_code_id UNIQUE (`code_id`(255)),
    CONSTRAINT uq_qr_code_sessions_code UNIQUE (`code`),
    CONSTRAINT fk_qr_code_sessions_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_qr_code_sessions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_qr_code_sessions_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `web3_nonces` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `nonce` TEXT NOT NULL,
    `wallet_address` VARCHAR(255) NOT NULL DEFAULT '',
    `organization_id` CHAR(36) NOT NULL,
    `application_id` CHAR(36),
    `chain_id` VARCHAR(255) NOT NULL DEFAULT '',
    `expires_at` TIMESTAMP NOT NULL,
    `consumed` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_web3_nonces_nonce UNIQUE (`nonce`(255)),
    CONSTRAINT fk_web3_nonces_application_id_applications FOREIGN KEY (`application_id`) REFERENCES `applications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_web3_nonces_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `face_id_sessions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `session_id` TEXT NOT NULL,
    `challenge` VARCHAR(255) NOT NULL DEFAULT '',
    `user_id` CHAR(36) NOT NULL,
    `expires_at` TIMESTAMP NOT NULL,
    `verified` TINYINT(1) NOT NULL DEFAULT false,
    `liveness_ok` TINYINT(1) NOT NULL DEFAULT false,
    `attempt_count` INT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_face_id_sessions_session_id UNIQUE (`session_id`(255)),
    CONSTRAINT fk_face_id_sessions_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `cas_tickets` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `ticket_id` TEXT NOT NULL,
    `ticket_type` VARCHAR(64) NOT NULL DEFAULT 'service',
    `user_id` CHAR(36),
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `service` VARCHAR(255) NOT NULL DEFAULT '',
    `expires_at` TIMESTAMP NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `pgt_iou` TEXT,
    `pgt_iou_expires_at` TIMESTAMP,
    `pgt_iou_used` TINYINT(1) NOT NULL DEFAULT FALSE,
    CONSTRAINT uq_cas_tickets_ticket_id UNIQUE (`ticket_id`(255)),
    CONSTRAINT uq_cas_tickets_pgt_iou UNIQUE (`pgt_iou`(255)),
    CONSTRAINT fk_cas_tickets_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `kerberos_tickets` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `ticket_id` TEXT NOT NULL,
    `user_id` CHAR(36),
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `service` VARCHAR(255) NOT NULL DEFAULT '',
    `realm` VARCHAR(255) NOT NULL DEFAULT '',
    `expires_at` TIMESTAMP NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_kerberos_tickets_ticket_id UNIQUE (`ticket_id`(255)),
    CONSTRAINT fk_kerberos_tickets_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- 分组：60_audit.yaml

CREATE TABLE IF NOT EXISTS `records` (
    `id` CHAR(36) NOT NULL COMMENT 'Audit record primary key (UUID)' PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL COMMENT 'Tenant org (RLS predicate key)',
    `actor_id` CHAR(36) COMMENT 'User or service account UUID that performed the action; NULL for system events',
    `action` TEXT NOT NULL COMMENT 'Verb-style action name (create / update / delete / login / token_revoke / ...)',
    `resource_type` TEXT NOT NULL COMMENT 'Domain entity type: users / roles / orders / ...',
    `resource_id` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Entity UUID (or string ID) the action targeted',
    `ip` VARCHAR(45) COMMENT 'Client IP observed at the time of the action',
    `user_agent` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'User-Agent header captured with the request',
    `surface` VARCHAR(64) NOT NULL DEFAULT 'management' COMMENT 'Interface where the action originated: management / user_portal / api / cli / system',
    `request_id` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Incoming request correlation ID (X-Request-ID header); empty when unavailable',
    `trace_id` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'OpenTelemetry trace ID for distributed tracing correlation',
    `metadata` JSON NOT NULL COMMENT 'Action-specific payload (diff fields, risk scores, geo info, etc.)',
    `chain_id` VARCHAR(255) NOT NULL DEFAULT '' COMMENT 'Audit chain identifier; links records into tamper-evident append-only chains per (org, surface, period)',
    `chain_seq` BIGINT NOT NULL DEFAULT 0 COMMENT 'Monotonic sequence within the chain; used to verify append order',
    `prev_hash` BLOB COMMENT 'Previous entry_hash in the chain (NULL for the first entry); digest algorithm per chain_algo (sha256 or sm3)',
    `entry_hash` BLOB COMMENT 'Tamper-evident digest of the canonical record body plus prev_hash; algorithm per chain_algo (sha256 or sm3)',
    `chain_status` VARCHAR(16) NOT NULL DEFAULT 'pending' COMMENT 'Chain build state: worker advances pending -> chained -> anchored asynchronously; broken marks a verification failure',
    `chain_algo` VARCHAR(8) NOT NULL DEFAULT 'sha256' COMMENT 'Hash algorithm backing prev_hash / entry_hash: sha256 or sm3',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Timestamp of last modification (rarely used; records are mostly append-only)',
    CONSTRAINT ck_records_chain_seq_nonneg CHECK (chain_seq >= 0),
    CONSTRAINT ck_records_chain_hash_len CHECK ((entry_hash IS NULL AND prev_hash IS NULL) OR (octet_length(entry_hash) = 32 AND (prev_hash IS NULL OR octet_length(prev_hash) = 32))),
    CONSTRAINT ck_records_chain_status CHECK (chain_status IN ('pending', 'chained', 'anchored', 'broken')),
    CONSTRAINT ck_records_chain_algo CHECK (chain_algo IN ('sha256', 'sm3')),
    CONSTRAINT uq_records_chain_seq UNIQUE (`organization_id`, `chain_id`, `chain_seq`),
    CONSTRAINT fk_records_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_records_actor_id_users FOREIGN KEY (`actor_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_audit_records_org_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_audit_records_org_created_at_desc` ON `records` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_audit_records_action' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_audit_records_action` ON `records` (`action`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_audit_records_resource_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_audit_records_resource_type` ON `records` (`resource_type`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_audit_records_actor_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_audit_records_actor_id` ON `records` (`actor_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_org_action_created_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_org_action_created_at` ON `records` (`organization_id`, `action`(255), `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_action_created_at_date' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_action_created_at_date` ON `records` (`action`(255), `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_surface_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_surface_created` ON `records` (`surface`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_records_request_id: predicate request_id <> '' (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_request_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_request_id` ON `records` (`request_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_records_trace_id: predicate trace_id <> '' (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_trace_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_trace_id` ON `records` (`trace_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_created_at_desc` ON `records` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_user_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_user_created` ON `records` (`actor_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_actor_id_created_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_actor_id_created_at` ON `records` (`actor_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'records' AND INDEX_NAME = 'idx_records_action_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_records_action_created_at_desc` ON `records` (`action`(255), `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `secret_rotation_audit` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `provider_id` CHAR(36) NOT NULL,
    `secret_type` TEXT NOT NULL,
    `old_version` INT NOT NULL,
    `new_version` INT NOT NULL,
    `rotated_by` CHAR(36),
    `rotated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `reason` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT fk_secret_rotation_audit_provider_id_providers FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'secret_rotation_audit' AND INDEX_NAME = 'idx_secret_rotation_audit_provider' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_secret_rotation_audit_provider` ON `secret_rotation_audit` (`provider_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'secret_rotation_audit' AND INDEX_NAME = 'idx_secret_rotation_audit_rotated_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_secret_rotation_audit_rotated_at` ON `secret_rotation_audit` (`rotated_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `audit_chain_anchors` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `chain_id` TEXT NOT NULL,
    `anchor_seq` BIGINT NOT NULL,
    `entry_hash` BLOB NOT NULL,
    `proof` VARCHAR(255) NOT NULL DEFAULT '',
    `anchored_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `anchor_source` VARCHAR(64) NOT NULL DEFAULT 'system',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT ck_audit_chain_anchors_hash_len CHECK (octet_length(entry_hash) = 32),
    CONSTRAINT uq_audit_chain_anchors_org_chain_seq UNIQUE (`organization_id`, `chain_id`(255), `anchor_seq`),
    CONSTRAINT fk_audit_chain_anchors_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'audit_chain_anchors' AND INDEX_NAME = 'idx_audit_chain_anchors_anchored_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_audit_chain_anchors_anchored_at` ON `audit_chain_anchors` (`anchored_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `authorization_chain_state` (
    `id` INT NOT NULL PRIMARY KEY,
    `last_hash` VARCHAR(64),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_authorization_chain_state_singleton CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `cross_tenant_operations` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `scope_id` CHAR(36),
    `actor_id` CHAR(36) NOT NULL,
    `source_tenant` CHAR(36),
    `target_tenant` CHAR(36) NOT NULL,
    `resource_type` TEXT NOT NULL,
    `action` TEXT NOT NULL,
    `risk_level` VARCHAR(16) NOT NULL,
    `status` VARCHAR(16) NOT NULL DEFAULT 'pending',
    `approved_by` CHAR(36),
    `approved_at` TIMESTAMP,
    `expires_at` TIMESTAMP NOT NULL,
    `used_at` TIMESTAMP,
    `trace_id` VARCHAR(255),
    `reason` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_cross_tenant_operations_risk CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT ck_cross_tenant_operations_status CHECK (status IN ('pending', 'approved', 'rejected', 'used', 'expired')),
    CONSTRAINT fk_cross_tenant_operations_scope_id_platform_admin_scopes FOREIGN KEY (`scope_id`) REFERENCES `platform_admin_scopes`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_cross_tenant_operations_actor_id_users FOREIGN KEY (`actor_id`) REFERENCES `users`(`id`) ON DELETE RESTRICT,
    CONSTRAINT fk_cross_tenant_operations_source_tenant_organizations FOREIGN KEY (`source_tenant`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT,
    CONSTRAINT fk_cross_tenant_operations_target_tenant_organizations FOREIGN KEY (`target_tenant`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT,
    CONSTRAINT fk_cross_tenant_operations_approved_by_platform_admins FOREIGN KEY (`approved_by`) REFERENCES `platform_admins`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'cross_tenant_operations' AND INDEX_NAME = 'idx_cross_tenant_operations_actor' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_cross_tenant_operations_actor` ON `cross_tenant_operations` (`actor_id`, `status`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'cross_tenant_operations' AND INDEX_NAME = 'idx_cross_tenant_operations_scope' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_cross_tenant_operations_scope` ON `cross_tenant_operations` (`scope_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'cross_tenant_operations' AND INDEX_NAME = 'idx_cross_tenant_operations_target' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_cross_tenant_operations_target` ON `cross_tenant_operations` (`target_tenant`, `expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `authorization_audit_records` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `actor_id` CHAR(36) NOT NULL,
    `actor_type` VARCHAR(32) NOT NULL,
    `source_tenant` CHAR(36),
    `target_tenant` CHAR(36),
    `resource_type` TEXT NOT NULL,
    `action` TEXT NOT NULL,
    `risk_level` VARCHAR(16) NOT NULL,
    `scope_id` CHAR(36),
    `ticket_id` CHAR(36),
    `decision` VARCHAR(16) NOT NULL,
    `reason` TEXT,
    `prev_hash` VARCHAR(64),
    `hash` VARCHAR(64) NOT NULL,
    `trace_id` VARCHAR(255),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_authorization_audit_actor_type CHECK (actor_type IN ('user', 'platform_admin', 'service_account', 'system')),
    CONSTRAINT ck_authorization_audit_risk CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
    CONSTRAINT ck_authorization_audit_decision CHECK (decision IN ('allow', 'deny')),
    CONSTRAINT ck_authorization_audit_hash CHECK (char_length(hash) = 64 AND (prev_hash IS NULL OR char_length(prev_hash) = 64)),
    CONSTRAINT fk_authorization_audit_records_source_tenant_organizations FOREIGN KEY (`source_tenant`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT,
    CONSTRAINT fk_authorization_audit_records_target_tenant_organizations FOREIGN KEY (`target_tenant`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT,
    CONSTRAINT fk_authorization_audit_records_scope_id_platform_admin_scopes FOREIGN KEY (`scope_id`) REFERENCES `platform_admin_scopes`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_authorization_audit_records_ticket_id_cross_tenant__dc79ba9b FOREIGN KEY (`ticket_id`) REFERENCES `cross_tenant_operations`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authorization_audit_records' AND INDEX_NAME = 'idx_authorization_audit_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_authorization_audit_created` ON `authorization_audit_records` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authorization_audit_records' AND INDEX_NAME = 'idx_authorization_audit_actor' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_authorization_audit_actor` ON `authorization_audit_records` (`actor_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authorization_audit_records' AND INDEX_NAME = 'idx_authorization_audit_tenants' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_authorization_audit_tenants` ON `authorization_audit_records` (`source_tenant`, `target_tenant`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'authorization_audit_records' AND INDEX_NAME = 'idx_authorization_audit_ticket' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_authorization_audit_ticket` ON `authorization_audit_records` (`ticket_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


-- 分组：70_commerce.yaml

CREATE TABLE IF NOT EXISTS `products` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `image` VARCHAR(500) DEFAULT '',
    `detail` VARCHAR(2000) DEFAULT '',
    `description` VARCHAR(500) DEFAULT '',
    `tag` VARCHAR(255) DEFAULT '',
    `currency` VARCHAR(3) NOT NULL DEFAULT 'USD',
    `price` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `quantity` INT NOT NULL DEFAULT 0,
    `sold` INT NOT NULL DEFAULT 0,
    `is_recharge` TINYINT(1) NOT NULL DEFAULT false,
    `recharge_options` JSON,
    `disable_custom_recharge` TINYINT(1) NOT NULL DEFAULT false,
    `providers` JSON,
    `success_url` VARCHAR(1000) DEFAULT '',
    `status` VARCHAR(64) NOT NULL DEFAULT 'draft',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_products_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT ck_products_status CHECK (status IN ('draft', 'active', 'discontinued')),
    CONSTRAINT fk_products_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `orders` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `user_id` CHAR(36),
    `products` JSON,
    `product_infos` JSON,
    `payment` VARCHAR(255) DEFAULT '',
    `price` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `currency` VARCHAR(3) DEFAULT '',
    `status` VARCHAR(64) NOT NULL DEFAULT 'created',
    `message` VARCHAR(2000) DEFAULT '',
    `payment_url` VARCHAR(500) DEFAULT '',
    `trade_no` VARCHAR(255) DEFAULT '',
    `paid_at` TIMESTAMP,
    `coupon_name` VARCHAR(255) DEFAULT '',
    `coupon_discount` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `idempotency_key` TEXT,
    CONSTRAINT uq_orders_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT uq_orders_idempotency_key UNIQUE (`idempotency_key`(255)),
    CONSTRAINT ck_orders_status CHECK (status IN ('created', 'payment_pending', 'paid', 'completed', 'failed', 'cancelled', 'refunded', 'closed')),
    CONSTRAINT ck_orders_paid_at CHECK ((status IN ('paid', 'completed', 'refunded', 'closed')) = (paid_at IS NOT NULL)),
    CONSTRAINT fk_orders_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_orders_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_org_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_orders_org_user` ON `orders` (`organization_id`, `user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_orders_status` ON `orders` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_orders_created_at_desc` ON `orders` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_trade_no' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_orders_trade_no` ON `orders` (`trade_no`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_org_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_orders_org_status` ON `orders` (`organization_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'orders' AND INDEX_NAME = 'idx_orders_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_orders_user_id` ON `orders` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `payments` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `provider` VARCHAR(255) DEFAULT '',
    `type` VARCHAR(100) DEFAULT '',
    `products` JSON,
    `product_name` VARCHAR(255) DEFAULT '',
    `product_display_name` VARCHAR(255) DEFAULT '',
    `detail` VARCHAR(500) DEFAULT '',
    `currency` VARCHAR(3) DEFAULT '',
    `price` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `user_id` CHAR(36),
    `person_name` VARCHAR(255) DEFAULT '',
    `person_id_card` VARCHAR(255) DEFAULT '',
    `person_email` VARCHAR(255) DEFAULT '',
    `person_phone` VARCHAR(255) DEFAULT '',
    `invoice_type` VARCHAR(100) DEFAULT '',
    `invoice_title` VARCHAR(255) DEFAULT '',
    `invoice_tax_id` VARCHAR(255) DEFAULT '',
    `invoice_remark` VARCHAR(500) DEFAULT '',
    `invoice_url` VARCHAR(500) DEFAULT '',
    `order_id` CHAR(36),
    `out_order_id` VARCHAR(255) DEFAULT '',
    `pay_url` VARCHAR(2000) DEFAULT '',
    `success_url` VARCHAR(2000) DEFAULT '',
    `status` VARCHAR(64) NOT NULL DEFAULT 'created',
    `message` VARCHAR(2000) DEFAULT '',
    `channel_order_no` VARCHAR(255) DEFAULT '',
    `attempt_count` INT NOT NULL DEFAULT 0,
    `refunded_amount` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `refunding_amount` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `last_error_code` VARCHAR(255) DEFAULT '',
    `last_error_message` VARCHAR(500) DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `idempotency_key` TEXT,
    CONSTRAINT uq_payments_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT uq_payments_idempotency_key UNIQUE (`idempotency_key`(255)),
    CONSTRAINT ck_payments_status CHECK (status IN ('created', 'processing', 'succeeded', 'failed', 'cancelled', 'refunded')),
    CONSTRAINT fk_payments_order_id_orders FOREIGN KEY (`order_id`) REFERENCES `orders`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_payments_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_payments_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payments' AND INDEX_NAME = 'idx_payments_org_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_payments_org_status` ON `payments` (`organization_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payments' AND INDEX_NAME = 'idx_payments_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_payments_user_id` ON `payments` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `transactions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36),
    `order_id` CHAR(36),
    `payment_id` CHAR(36),
    `transaction_type` TEXT NOT NULL,
    `amount` DECIMAL(19, 4) NOT NULL,
    `currency` VARCHAR(3) NOT NULL DEFAULT 'USD',
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `provider_type` VARCHAR(255) NOT NULL DEFAULT '',
    `provider_id` CHAR(36),
    `metadata` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_transactions_status CHECK (status IN ('pending', 'completed', 'failed', 'cancelled')),
    CONSTRAINT ck_transactions_type CHECK (transaction_type IN ('payment', 'refund', 'cancellation')),
    CONSTRAINT fk_transactions_order_id_orders FOREIGN KEY (`order_id`) REFERENCES `orders`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_transactions_payment_id_payments FOREIGN KEY (`payment_id`) REFERENCES `payments`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_provider_id_providers FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transactions' AND INDEX_NAME = 'idx_transactions_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_transactions_org` ON `transactions` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transactions' AND INDEX_NAME = 'idx_transactions_order' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_transactions_order` ON `transactions` (`order_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transactions' AND INDEX_NAME = 'idx_transactions_payment' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_transactions_payment` ON `transactions` (`payment_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transactions' AND INDEX_NAME = 'idx_transactions_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_transactions_user` ON `transactions` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'transactions' AND INDEX_NAME = 'idx_transactions_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_transactions_status` ON `transactions` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `plans` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` VARCHAR(500) DEFAULT '',
    `price` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `currency` VARCHAR(3) DEFAULT '',
    `period` VARCHAR(50) DEFAULT '',
    `product` VARCHAR(255) DEFAULT '',
    `payment_providers` JSON,
    `status` VARCHAR(64) NOT NULL DEFAULT 'draft',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `is_exclusive` TINYINT(1) NOT NULL DEFAULT false,
    `role` VARCHAR(255) DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_plans_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT ck_plans_status CHECK (status IN ('draft', 'active', 'archived')),
    CONSTRAINT ck_plans_period CHECK (period IS NULL OR period IN ('daily', 'weekly', 'monthly', 'quarterly', 'yearly', 'lifetime')),
    CONSTRAINT fk_plans_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `pricings` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` VARCHAR(500) DEFAULT '',
    `plans` JSON,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `trial_duration` INT NOT NULL DEFAULT 0,
    `application` VARCHAR(255) DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_pricings_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT fk_pricings_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `subscriptions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `user_id` CHAR(36),
    `plan_id` CHAR(36),
    `pricing_id` CHAR(36),
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `start_at` TIMESTAMP,
    `end_at` TIMESTAMP,
    `expire_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_subscriptions_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT ck_subscriptions_status CHECK (status IN ('pending', 'trialing', 'active', 'past_due', 'expired', 'cancelled')),
    CONSTRAINT fk_subscriptions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_subscriptions_plan_id_plans FOREIGN KEY (`plan_id`) REFERENCES `plans`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_subscriptions_pricing_id_pricings FOREIGN KEY (`pricing_id`) REFERENCES `pricings`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_subscriptions_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'subscriptions' AND INDEX_NAME = 'idx_subscriptions_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_subscriptions_user_id` ON `subscriptions` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `coupons` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` VARCHAR(255) NOT NULL DEFAULT '',
    `code` VARCHAR(255) NOT NULL DEFAULT '',
    `discount_type` VARCHAR(64) NOT NULL DEFAULT 'percentage',
    `discount` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `max_discount` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `scope` VARCHAR(255) NOT NULL DEFAULT '',
    `products` JSON NOT NULL,
    `users` JSON NOT NULL,
    `quantity` INT NOT NULL DEFAULT 0,
    `used_count` INT NOT NULL DEFAULT 0,
    `max_usage_per_user` INT NOT NULL DEFAULT 0,
    `start_time` VARCHAR(255) NOT NULL DEFAULT '',
    `expire_time` VARCHAR(255) NOT NULL DEFAULT '',
    `min_order_amount` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `currency` VARCHAR(3) NOT NULL DEFAULT '',
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_coupons_code UNIQUE (`code`),
    CONSTRAINT ck_coupons_status CHECK (status IN ('active', 'disabled', 'expired')),
    CONSTRAINT fk_coupons_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'coupons' AND INDEX_NAME = 'idx_coupons_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_coupons_org` ON `coupons` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `entries` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `provider` VARCHAR(255) NOT NULL DEFAULT '',
    `application` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(255) NOT NULL DEFAULT '',
    `client_ip` VARCHAR(255) NOT NULL DEFAULT '',
    `user_agent` VARCHAR(255) NOT NULL DEFAULT '',
    `message` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_entries_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_entries_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'entries' AND INDEX_NAME = 'idx_entries_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_entries_type` ON `entries` (`type`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'entries' AND INDEX_NAME = 'idx_entries_application' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_entries_application` ON `entries` (`application`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `coupon_usage` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `coupon_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36) NOT NULL,
    `order_id` CHAR(36),
    `used_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_coupon_usage_coupon_id_coupons FOREIGN KEY (`coupon_id`) REFERENCES `coupons`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_coupon_usage_order_id_orders FOREIGN KEY (`order_id`) REFERENCES `orders`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_coupon_usage_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'coupon_usage' AND INDEX_NAME = 'idx_cu_coupon_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_cu_coupon_id` ON `coupon_usage` (`coupon_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'coupon_usage' AND INDEX_NAME = 'idx_cu_user_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_cu_user_id` ON `coupon_usage` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'coupon_usage' AND INDEX_NAME = 'idx_cu_order_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_cu_order_id` ON `coupon_usage` (`order_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `pay_channel_configs` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `provider_id` CHAR(36),
    `channel` VARCHAR(50) NOT NULL,
    `environment` VARCHAR(20) NOT NULL DEFAULT 'production',
    `config_sealed` TEXT NOT NULL,
    `config_fingerprint` VARCHAR(16) NOT NULL,
    `config_set` JSON,
    `version_no` INT NOT NULL DEFAULT 1,
    `status` VARCHAR(32) NOT NULL DEFAULT 'draft',
    `test_passed_at` TIMESTAMP,
    `activated_at` TIMESTAMP,
    `disabled_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_pay_channel_configs_status CHECK (status IN ('draft', 'testing', 'active', 'disabled')),
    CONSTRAINT ck_pay_channel_configs_env CHECK (environment IN ('sandbox', 'production')),
    CONSTRAINT ck_pay_channel_configs_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy', 'square', 'checkout')),
    gen_pay_channel_configs_org_channel_env_version TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_pay_channel_configs_org_channel_env_version UNIQUE (`organization_id`, `channel`, `environment`, `version_no`, gen_pay_channel_configs_org_channel_env_version),
    CONSTRAINT uq_pay_channel_configs_provider_version UNIQUE (`provider_id`, `version_no`),
    gen_pay_channel_configs_one_active TINYINT(1) AS (CASE WHEN (provider_id IS NOT NULL AND status = 'active' AND deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_pay_channel_configs_one_active UNIQUE (`provider_id`, gen_pay_channel_configs_one_active),
    CONSTRAINT fk_pay_channel_configs_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT fk_pay_channel_configs_provider_id_providers FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_channel_configs' AND INDEX_NAME = 'idx_pay_channel_configs_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_channel_configs_org` ON `pay_channel_configs` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_channel_configs' AND INDEX_NAME = 'idx_pay_channel_configs_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_channel_configs_status` ON `pay_channel_configs` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_channel_configs' AND INDEX_NAME = 'idx_pay_channel_configs_channel_env' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_channel_configs_channel_env` ON `pay_channel_configs` (`channel`, `environment`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `pay_callback_receipts` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `channel` VARCHAR(50) NOT NULL,
    `payload_sha256` VARCHAR(64) NOT NULL,
    `payload` BLOB NOT NULL,
    `headers` JSON,
    `received_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `client_ip` VARCHAR(64),
    `process_state` VARCHAR(32) NOT NULL DEFAULT 'received',
    `event_key` VARCHAR(255),
    `out_trade_no` VARCHAR(255),
    `raw_event` JSON,
    `error_message` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_receipts_process_state CHECK (process_state IN ('received', 'verified', 'rejected', 'duplicated', 'normalized', 'accepted')),
    CONSTRAINT ck_receipts_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy', 'square', 'checkout')),
    CONSTRAINT uq_receipts_channel_sha256 UNIQUE (`channel`, `payload_sha256`),
    CONSTRAINT uq_receipts_channel_event_key UNIQUE (`channel`, `event_key`),
    CONSTRAINT fk_pay_callback_receipts_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_callback_receipts' AND INDEX_NAME = 'idx_receipts_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_receipts_org` ON `pay_callback_receipts` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_callback_receipts' AND INDEX_NAME = 'idx_receipts_out_trade_no' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_receipts_out_trade_no` ON `pay_callback_receipts` (`out_trade_no`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_callback_receipts' AND INDEX_NAME = 'idx_receipts_process_state' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_receipts_process_state` ON `pay_callback_receipts` (`process_state`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_callback_receipts' AND INDEX_NAME = 'idx_receipts_received_at' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_receipts_received_at` ON `pay_callback_receipts` (`received_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `pay_refunds` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `payment_id` CHAR(36) NOT NULL,
    `out_refund_no` VARCHAR(255) NOT NULL,
    `channel` VARCHAR(50) NOT NULL,
    `amount` DECIMAL(19, 4) NOT NULL,
    `currency` VARCHAR(3) NOT NULL,
    `state` VARCHAR(32) NOT NULL DEFAULT 'requested',
    `reason` VARCHAR(500) DEFAULT '',
    `channel_refund_no` VARCHAR(255),
    `error_code` VARCHAR(100),
    `error_message` VARCHAR(2000),
    `requested_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `completed_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_pay_refunds_state CHECK (state IN ('requested', 'processing', 'succeeded', 'failed', 'cancelled')),
    CONSTRAINT ck_pay_refunds_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy', 'square', 'checkout')),
    gen_pay_refunds_channel_out_no TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_pay_refunds_channel_out_no UNIQUE (`channel`, `out_refund_no`, gen_pay_refunds_channel_out_no),
    CONSTRAINT fk_pay_refunds_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_pay_refunds_payment_id_payments FOREIGN KEY (`payment_id`) REFERENCES `payments`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_refunds' AND INDEX_NAME = 'idx_pay_refunds_payment' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_refunds_payment` ON `pay_refunds` (`payment_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_refunds' AND INDEX_NAME = 'idx_pay_refunds_state' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_refunds_state` ON `pay_refunds` (`state`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `pay_attempts` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `payment_id` CHAR(36) NOT NULL,
    `refund_id` CHAR(36),
    `channel` VARCHAR(50) NOT NULL,
    `attempt_type` VARCHAR(20) NOT NULL,
    `attempt_no` INT NOT NULL DEFAULT 1,
    `channel_order_no` VARCHAR(255),
    `channel_request_id` VARCHAR(255),
    `state` VARCHAR(32) NOT NULL DEFAULT 'pending',
    `amount` DECIMAL(19, 4),
    `currency` VARCHAR(3),
    `error_code` VARCHAR(100),
    `error_message` VARCHAR(2000),
    `latency_ms` INT,
    `occurred_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `raw_response` JSON,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT ck_pay_attempts_state CHECK (state IN ('pending', 'succeeded', 'failed', 'unknown', 'rejected')),
    CONSTRAINT ck_pay_attempts_type CHECK (attempt_type IN ('charge', 'query', 'refund', 'query_refund', 'close', 'callback')),
    CONSTRAINT ck_pay_attempts_channel CHECK (channel IN ('alipay', 'wechat_pay', 'wxpay', 'stripe', 'paypal', 'adyen', 'airwallex', 'paddle', 'fastspring', 'lemonsqueezy', 'polar', 'balance', 'gc', 'dummy', 'square', 'checkout')),
    CONSTRAINT fk_pay_attempts_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_pay_attempts_payment_id_payments FOREIGN KEY (`payment_id`) REFERENCES `payments`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_pay_attempts_refund_id_pay_refunds FOREIGN KEY (`refund_id`) REFERENCES `pay_refunds`(`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_attempts' AND INDEX_NAME = 'idx_pay_attempts_payment' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_attempts_payment` ON `pay_attempts` (`payment_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_attempts' AND INDEX_NAME = 'idx_pay_attempts_refund' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_attempts_refund` ON `pay_attempts` (`refund_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_attempts' AND INDEX_NAME = 'idx_pay_attempts_occurred' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_attempts_occurred` ON `pay_attempts` (`occurred_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `pay_recon_batches` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `channel` VARCHAR(50) NOT NULL,
    `batch_date` DATE NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'pending',
    `diff_count` INT NOT NULL DEFAULT 0,
    `total_local` INT NOT NULL DEFAULT 0,
    `total_channel` INT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_pay_recon_batches_status CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    gen_pay_recon_batches_org_channel_date TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_pay_recon_batches_org_channel_date UNIQUE (`organization_id`, `channel`, `batch_date`, gen_pay_recon_batches_org_channel_date),
    CONSTRAINT fk_pay_recon_batches_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_recon_batches' AND INDEX_NAME = 'idx_pay_recon_batches_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_recon_batches_org` ON `pay_recon_batches` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_recon_batches' AND INDEX_NAME = 'idx_pay_recon_batches_date' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_recon_batches_date` ON `pay_recon_batches` (`batch_date`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `pay_recon_items` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `batch_id` CHAR(36) NOT NULL,
    `diff_type` VARCHAR(30) NOT NULL,
    `channel_order_no` VARCHAR(255),
    `local_payment_id` CHAR(36),
    `channel_amount` DECIMAL(19, 4),
    `local_amount` DECIMAL(19, 4),
    `channel_state` VARCHAR(50),
    `local_state` VARCHAR(50),
    `resolved` TINYINT(1) NOT NULL DEFAULT false,
    `resolution_state` VARCHAR(20) NOT NULL DEFAULT 'pending',
    `resolved_by` VARCHAR(255) DEFAULT '',
    `resolved_at` TIMESTAMP,
    `note` VARCHAR(1000) DEFAULT '',
    `approved_by` CHAR(36),
    `approved_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_pay_recon_items_diff_type CHECK (diff_type IN ('missing_local', 'missing_channel', 'amount_mismatch', 'state_mismatch', 'refund_mismatch')),
    CONSTRAINT ck_pay_recon_items_operator_distinct_approver CHECK (approved_by IS NULL OR approved_by <> resolved_by),
    CONSTRAINT ck_pay_recon_items_resolution_state CHECK (resolution_state IN ('pending', 'resolved', 'approved', 'rejected')),
    CONSTRAINT fk_pay_recon_items_batch_id_pay_recon_batches FOREIGN KEY (`batch_id`) REFERENCES `pay_recon_batches`(`id`) ON DELETE CASCADE ON UPDATE CASCADE,
    CONSTRAINT fk_pay_recon_items_approved_by_users FOREIGN KEY (`approved_by`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_recon_items' AND INDEX_NAME = 'idx_pay_recon_items_batch' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_recon_items_batch` ON `pay_recon_items` (`batch_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'pay_recon_items' AND INDEX_NAME = 'idx_pay_recon_items_resolved' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_pay_recon_items_resolved` ON `pay_recon_items` (`resolved`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


-- 分组：75_ai_gateway.yaml

CREATE TABLE IF NOT EXISTS `llm_providers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `provider_id` CHAR(36),
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `provider_type` VARCHAR(50) NOT NULL,
    `base_url` VARCHAR(500) NOT NULL,
    `api_key_sealed` TEXT NOT NULL,
    `api_key_fingerprint` VARCHAR(64) NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `request_timeout_ms` INT NOT NULL DEFAULT 30000,
    `max_retries` INT NOT NULL DEFAULT 2,
    `priority` INT NOT NULL DEFAULT 100,
    `weight` INT NOT NULL DEFAULT 1,
    `rpm_limit` INT,
    `tpm_limit` INT,
    `supported_protocols` JSON NOT NULL,
    `preferred_protocol` VARCHAR(30) NOT NULL DEFAULT 'openai',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp (implied by soft_delete: true)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_llm_providers_status CHECK (status IN ('active', 'disabled', 'testing')),
    CONSTRAINT ck_llm_providers_provider_type CHECK (provider_type IN ('openai', 'anthropic', 'gemini', 'deepseek', 'glm', 'kimi', 'qwen', 'seed', 'ollama', 'vllm', 'custom')),
    CONSTRAINT ck_llm_providers_preferred_protocol CHECK (preferred_protocol IN ('openai', 'anthropic', 'gemini')),
    gen_llm_providers_org_name TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_llm_providers_org_name UNIQUE (`organization_id`, `name`, gen_llm_providers_org_name),
    CONSTRAINT uq_llm_providers_provider UNIQUE (`provider_id`),
    CONSTRAINT fk_llm_providers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT fk_llm_providers_provider_id_providers FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_providers' AND INDEX_NAME = 'idx_llm_providers_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_providers_org` ON `llm_providers` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_providers' AND INDEX_NAME = 'idx_llm_providers_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_providers_status` ON `llm_providers` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_models` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `provider_id` CHAR(36) NOT NULL,
    `provider_model_name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `capabilities` JSON NOT NULL,
    `max_context_tokens` INT,
    `max_output_tokens` INT,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp (implied by soft_delete: true)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_llm_models_status CHECK (status IN ('active', 'deprecated')),
    gen_llm_models_provider_model TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_llm_models_provider_model UNIQUE (`provider_id`, `provider_model_name`, gen_llm_models_provider_model),
    CONSTRAINT fk_llm_models_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_models_provider_id_llm_providers FOREIGN KEY (`provider_id`) REFERENCES `llm_providers`(`id`) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_models' AND INDEX_NAME = 'idx_llm_models_provider' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_models_provider` ON `llm_models` (`provider_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_models' AND INDEX_NAME = 'idx_llm_models_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_models_org` ON `llm_models` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_model_aliases` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `alias` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp (implied by soft_delete: true)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_llm_aliases_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT uq_llm_aliases_org_alias UNIQUE (`organization_id`, `alias`),
    CONSTRAINT fk_llm_model_aliases_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `llm_routes` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `alias_id` CHAR(36) NOT NULL,
    `model_id` CHAR(36) NOT NULL,
    `weight` INT NOT NULL DEFAULT 1,
    `priority` INT NOT NULL DEFAULT 100,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp (implied by soft_delete: true)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_llm_routes_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT fk_llm_routes_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_routes_alias_id_llm_model_aliases FOREIGN KEY (`alias_id`) REFERENCES `llm_model_aliases`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_routes_model_id_llm_models FOREIGN KEY (`model_id`) REFERENCES `llm_models`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_routes' AND INDEX_NAME = 'idx_llm_routes_alias' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_routes_alias` ON `llm_routes` (`alias_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_routes' AND INDEX_NAME = 'idx_llm_routes_model' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_routes_model` ON `llm_routes` (`model_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_api_keys` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `key_prefix` VARCHAR(20) NOT NULL,
    `key_hash` VARCHAR(128) NOT NULL,
    `allowed_aliases` JSON NOT NULL,
    `rpm_limit` INT,
    `tpm_limit` INT,
    `monthly_budget` DECIMAL(19, 4),
    `currency` VARCHAR(3) NOT NULL DEFAULT 'USD',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `expires_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp (implied by soft_delete: true)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_llm_api_keys_status CHECK (status IN ('active', 'revoked', 'expired')),
    gen_llm_api_keys_org_name TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_llm_api_keys_org_name UNIQUE (`organization_id`, `name`, gen_llm_api_keys_org_name),
    CONSTRAINT fk_llm_api_keys_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_api_keys' AND INDEX_NAME = 'idx_llm_api_keys_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_api_keys_org` ON `llm_api_keys` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_quota_policies` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `scope` VARCHAR(20) NOT NULL,
    `scope_ref` CHAR(36),
    `limit_type` VARCHAR(20) NOT NULL,
    `limit_value` DECIMAL(19, 4) NOT NULL,
    `reset_cycle` VARCHAR(20) NOT NULL DEFAULT 'monthly',
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp (implied by soft_delete: true)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_llm_quota_scope CHECK (scope IN ('organization', 'api_key', 'alias')),
    CONSTRAINT ck_llm_quota_reset_cycle CHECK (reset_cycle IN ('daily', 'monthly')),
    CONSTRAINT ck_llm_quota_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT fk_llm_quota_policies_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_quota_policies' AND INDEX_NAME = 'idx_llm_quota_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_quota_org` ON `llm_quota_policies` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_quota_policies' AND INDEX_NAME = 'idx_llm_quota_scope_ref' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_quota_scope_ref` ON `llm_quota_policies` (`scope`, `scope_ref`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_price_versions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `model_id` CHAR(36) NOT NULL,
    `version_no` INT NOT NULL DEFAULT 1,
    `input_price_per_1m` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `output_price_per_1m` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `currency` VARCHAR(3) NOT NULL DEFAULT 'USD',
    `effective_from` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `deleted_at` TIMESTAMP COMMENT 'Soft-delete timestamp (implied by soft_delete: true)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT uq_llm_price_model_version UNIQUE (`model_id`, `version_no`),
    CONSTRAINT fk_llm_price_versions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_price_versions_model_id_llm_models FOREIGN KEY (`model_id`) REFERENCES `llm_models`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_price_versions' AND INDEX_NAME = 'idx_llm_price_effective' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_price_effective` ON `llm_price_versions` (`effective_from`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_provider_health` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `provider_id` CHAR(36) NOT NULL,
    `state` VARCHAR(32) NOT NULL DEFAULT 'healthy',
    `consecutive_failures` INT NOT NULL DEFAULT 0,
    `open_until` TIMESTAMP,
    `last_checked_at` TIMESTAMP,
    `last_latency_ms` INT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    `revision` BIGINT NOT NULL DEFAULT 1 COMMENT 'Optimistic lock version (implied by revision: true)',
    CONSTRAINT ck_llm_health_state CHECK (state IN ('healthy', 'degraded', 'open', 'half_open')),
    CONSTRAINT fk_llm_provider_health_provider_id_llm_providers FOREIGN KEY (`provider_id`) REFERENCES `llm_providers`(`id`) ON DELETE CASCADE ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_provider_health' AND INDEX_NAME = 'uq_llm_health_provider' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE UNIQUE INDEX `uq_llm_health_provider` ON `llm_provider_health` (`provider_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_usage_logs` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `api_key_id` CHAR(36),
    `alias_id` CHAR(36) NOT NULL,
    `provider_id` CHAR(36) NOT NULL,
    `model_id` CHAR(36) NOT NULL,
    `price_version_id` CHAR(36),
    `protocol_face` VARCHAR(30) NOT NULL,
    `attempt_no` INT NOT NULL DEFAULT 1,
    `input_tokens` INT NOT NULL DEFAULT 0,
    `output_tokens` INT NOT NULL DEFAULT 0,
    `total_tokens` INT NOT NULL DEFAULT 0,
    `cost_amount` DECIMAL(19, 4) NOT NULL DEFAULT 0,
    `currency` VARCHAR(3) NOT NULL DEFAULT 'USD',
    `status` TEXT NOT NULL,
    `error_code` VARCHAR(100),
    `ttft_ms` INT,
    `total_latency_ms` INT,
    `upstream_request_id` VARCHAR(255),
    `failover_from` VARCHAR(255),
    `request_id` VARCHAR(255) NOT NULL,
    `created_at` TIMESTAMP NOT NULL,
    CONSTRAINT ck_llm_usage_status CHECK (status IN ('succeeded', 'failed', 'timeout')),
    CONSTRAINT ck_llm_usage_protocol_face CHECK (protocol_face IN ('openai', 'anthropic', 'gemini')),
    CONSTRAINT fk_llm_usage_logs_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_api_key_id_llm_api_keys FOREIGN KEY (`api_key_id`) REFERENCES `llm_api_keys`(`id`) ON DELETE SET NULL ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_alias_id_llm_model_aliases FOREIGN KEY (`alias_id`) REFERENCES `llm_model_aliases`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_provider_id_llm_providers FOREIGN KEY (`provider_id`) REFERENCES `llm_providers`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_model_id_llm_models FOREIGN KEY (`model_id`) REFERENCES `llm_models`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT fk_llm_usage_logs_price_version_id_llm_price_versions FOREIGN KEY (`price_version_id`) REFERENCES `llm_price_versions`(`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_usage_logs' AND INDEX_NAME = 'idx_llm_usage_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_usage_org` ON `llm_usage_logs` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_usage_logs' AND INDEX_NAME = 'idx_llm_usage_alias' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_usage_alias` ON `llm_usage_logs` (`alias_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_usage_logs' AND INDEX_NAME = 'idx_llm_usage_provider' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_usage_provider` ON `llm_usage_logs` (`provider_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_usage_logs' AND INDEX_NAME = 'idx_llm_usage_api_key' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_usage_api_key` ON `llm_usage_logs` (`api_key_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_usage_logs' AND INDEX_NAME = 'idx_llm_usage_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_usage_created` ON `llm_usage_logs` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `llm_metering_events` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `outbox_id` CHAR(36) NOT NULL,
    `organization_id` CHAR(36),
    `aggregate_type` VARCHAR(50) NOT NULL,
    `aggregate_id` CHAR(36),
    `event_type` VARCHAR(100) NOT NULL,
    `provider_id` CHAR(36),
    `model_id` CHAR(36),
    `payload_json` JSON NOT NULL,
    `recorded_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Creation timestamp (schema convention: every table has created_at)',
    CONSTRAINT fk_llm_metering_events_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_llm_metering_events_provider_id_llm_providers FOREIGN KEY (`provider_id`) REFERENCES `llm_providers`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_llm_metering_events_model_id_llm_models FOREIGN KEY (`model_id`) REFERENCES `llm_models`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_metering_events' AND INDEX_NAME = 'idx_llm_metering_events_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_metering_events_org` ON `llm_metering_events` (`organization_id`, `recorded_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_metering_events' AND INDEX_NAME = 'idx_llm_metering_events_provider' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_metering_events_provider` ON `llm_metering_events` (`provider_id`, `recorded_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'llm_metering_events' AND INDEX_NAME = 'idx_llm_metering_events_event_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_llm_metering_events_event_type` ON `llm_metering_events` (`event_type`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


-- 分组：80_governance.yaml

CREATE TABLE IF NOT EXISTS `invitations` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `email` TEXT NOT NULL,
    `code` TEXT NOT NULL,
    `role_ids` JSON NOT NULL,
    `group_ids` JSON NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `expires_at` TIMESTAMP NOT NULL,
    `accepted_at` TIMESTAMP,
    `invited_by` CHAR(36),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_invitations_code UNIQUE (`code`(255)),
    CONSTRAINT ck_invitations_status CHECK (status IN ('pending', 'accepted', 'expired', 'revoked')),
    CONSTRAINT fk_invitations_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'invitations' AND INDEX_NAME = 'idx_invitations_org_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_invitations_org_created_at_desc` ON `invitations` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'invitations' AND INDEX_NAME = 'idx_invitations_email' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_invitations_email` ON `invitations` (`email`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'invitations' AND INDEX_NAME = 'idx_invitations_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_invitations_status` ON `invitations` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `certs` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `scope` VARCHAR(64) NOT NULL DEFAULT 'JWT',
    `type` VARCHAR(64) NOT NULL DEFAULT 'x509',
    `crypto_algorithm` VARCHAR(64) NOT NULL DEFAULT 'RS256',
    `bit_size` INT NOT NULL DEFAULT 2048,
    `expire_in_years` INT NOT NULL DEFAULT 1,
    `expire_time` TIMESTAMP,
    `certificate` VARCHAR(255) NOT NULL DEFAULT '',
    `private_key` TEXT NOT NULL,
    `provider` VARCHAR(255) NOT NULL DEFAULT '',
    `account` VARCHAR(255) NOT NULL DEFAULT '',
    `access_key` VARCHAR(255) NOT NULL DEFAULT '',
    `access_secret` TEXT NOT NULL,
    `domain` JSON NOT NULL,
    `authority_public_key` VARCHAR(255) NOT NULL DEFAULT '',
    `authority_root_public_key` VARCHAR(255) NOT NULL DEFAULT '',
    `acme_issuer` VARCHAR(255) NOT NULL DEFAULT '',
    `acme_endpoint` VARCHAR(255) NOT NULL DEFAULT '',
    `domain_expire_time` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_certs_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_certs_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'certs' AND INDEX_NAME = 'idx_certs_scope' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_certs_scope` ON `certs` (`scope`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'certs' AND INDEX_NAME = 'idx_certs_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_certs_type` ON `certs` (`type`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `lockout_attempts` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `user_id` CHAR(36) NOT NULL,
    `attempts` INT NOT NULL DEFAULT 0,
    `locked_until` TIMESTAMP,
    `window_started_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `lockout_level` INT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_lockout_attempts_user_id UNIQUE (`user_id`),
    CONSTRAINT fk_lockout_attempts_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ip_rules` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `rule_type` VARCHAR(64) NOT NULL DEFAULT 'block',
    `cidr` VARCHAR(45) NOT NULL,
    `description` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_ip_rules_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ip_rules' AND INDEX_NAME = 'idx_ip_rules_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ip_rules_org` ON `ip_rules` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `rules` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `scope` VARCHAR(255) NOT NULL DEFAULT '',
    `rule_type` VARCHAR(255) NOT NULL DEFAULT '',
    `conditions` JSON NOT NULL,
    `action` VARCHAR(255) NOT NULL DEFAULT '',
    `priority` INT NOT NULL DEFAULT 0,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_rules_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_rules_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `mcp_servers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `url` VARCHAR(255) NOT NULL DEFAULT '',
    `token` VARCHAR(255) NOT NULL DEFAULT '',
    `application` VARCHAR(255) NOT NULL DEFAULT '',
    `tools` JSON NOT NULL,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_mcp_servers_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_mcp_servers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `radius_accountings` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `username` VARCHAR(255) NOT NULL DEFAULT '',
    `service_type` BIGINT NOT NULL DEFAULT 0,
    `nas_id` VARCHAR(255) NOT NULL DEFAULT '',
    `nas_ip_addr` VARCHAR(255) NOT NULL DEFAULT '',
    `nas_port_id` VARCHAR(255) NOT NULL DEFAULT '',
    `nas_port_type` BIGINT NOT NULL DEFAULT 0,
    `nas_port` BIGINT NOT NULL DEFAULT 0,
    `framed_ip_addr` VARCHAR(255) NOT NULL DEFAULT '',
    `framed_ip_netmask` VARCHAR(255) NOT NULL DEFAULT '',
    `acct_session_id` VARCHAR(255) NOT NULL DEFAULT '',
    `acct_session_time` BIGINT NOT NULL DEFAULT 0,
    `acct_input_total` BIGINT NOT NULL DEFAULT 0,
    `acct_output_total` BIGINT NOT NULL DEFAULT 0,
    `acct_input_packets` BIGINT NOT NULL DEFAULT 0,
    `acct_output_packets` BIGINT NOT NULL DEFAULT 0,
    `acct_terminate_cause` BIGINT NOT NULL DEFAULT 0,
    `acct_start_time` TIMESTAMP NOT NULL,
    `acct_stop_time` TIMESTAMP,
    `last_update` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_radius_accountings_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'radius_accountings' AND INDEX_NAME = 'idx_radius_accountings_org' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_radius_accountings_org` ON `radius_accountings` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'radius_accountings' AND INDEX_NAME = 'idx_radius_accountings_username' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_radius_accountings_username` ON `radius_accountings` (`username`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `servers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `endpoint` TEXT NOT NULL,
    `protocol` VARCHAR(64) NOT NULL DEFAULT 'https',
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `metadata` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_servers_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_servers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'servers' AND INDEX_NAME = 'idx_servers_org_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_servers_org_created_at_desc` ON `servers` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'servers' AND COLUMN_NAME = 'gen_servers_status');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `servers` ADD COLUMN gen_servers_status TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'servers' AND INDEX_NAME = 'idx_servers_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_servers_status` ON `servers` (`status`, `gen_servers_status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `sites` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `host` TEXT NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `ssl_mode` VARCHAR(20) NOT NULL DEFAULT 'off',
    `cert_id` CHAR(36),
    `force_https` TINYINT(1) NOT NULL DEFAULT false,
    `enable_health_check` TINYINT(1) NOT NULL DEFAULT false,
    `health_check_interval` INT NOT NULL DEFAULT 60,
    `last_health_status` VARCHAR(20) NOT NULL DEFAULT 'unknown',
    `last_health_at` TIMESTAMP,
    `enable_alert` TINYINT(1) NOT NULL DEFAULT false,
    `alert_interval` INT NOT NULL DEFAULT 300,
    `alert_try_times` INT NOT NULL DEFAULT 3,
    `alert_provider_ids` JSON NOT NULL,
    `metadata` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_sites_org_name UNIQUE (`organization_id`, `name`(255)),
    gen_sites_host_active TINYINT(1) AS (CASE WHEN (deleted_at IS NULL) THEN 1 ELSE NULL END) STORED, CONSTRAINT uq_sites_host_active UNIQUE (`host`(255), gen_sites_host_active),
    CONSTRAINT fk_sites_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_sites_cert_id_certs FOREIGN KEY (`cert_id`) REFERENCES `certs`(`id`) ON DELETE SET NULL ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sites' AND INDEX_NAME = 'idx_sites_org_created_at_desc' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sites_org_created_at_desc` ON `sites` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sites' AND COLUMN_NAME = 'gen_sites_status');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `sites` ADD COLUMN gen_sites_status TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'sites' AND INDEX_NAME = 'idx_sites_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_sites_status` ON `sites` (`status`, `gen_sites_status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `keys` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `key_type` TEXT NOT NULL,
    `public_key` TEXT NOT NULL,
    `private_key` TEXT,
    `algorithm` VARCHAR(64) NOT NULL DEFAULT 'RS256',
    `expires_at` TIMESTAMP,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_keys_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_keys_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'keys' AND INDEX_NAME = 'idx_keys_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_keys_type` ON `keys` (`key_type`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `notification_providers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `provider_id` CHAR(36),
    `name` TEXT NOT NULL,
    `provider_type` VARCHAR(255) NOT NULL DEFAULT '',
    `webhook_url` VARCHAR(255) NOT NULL DEFAULT '',
    `access_token` TEXT NOT NULL,
    `config` JSON NOT NULL,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_notification_providers_provider UNIQUE (`provider_id`),
    CONSTRAINT fk_notification_providers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_notification_providers_provider_id_providers FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notification_providers' AND INDEX_NAME = 'idx_notification_providers_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_notification_providers_organization_id` ON `notification_providers` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `radius_configs` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `host` VARCHAR(255) NOT NULL DEFAULT '',
    `port` INT NOT NULL DEFAULT 1812,
    `shared_secret` TEXT NOT NULL,
    `auth_protocol` VARCHAR(64) NOT NULL DEFAULT 'PAP',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_radius_configs_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'radius_configs' AND INDEX_NAME = 'idx_radius_configs_organization_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_radius_configs_organization_id` ON `radius_configs` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `ldaps` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `server_name` VARCHAR(255) NOT NULL DEFAULT '',
    `host` VARCHAR(255) NOT NULL DEFAULT '',
    `port` INT NOT NULL DEFAULT 389,
    `admin` VARCHAR(255) NOT NULL DEFAULT '',
    `bind_dn` VARCHAR(255) NOT NULL DEFAULT '',
    `bind_password` VARCHAR(255) NOT NULL DEFAULT '',
    `base_dn` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(255) NOT NULL DEFAULT '',
    `attribute_name` VARCHAR(255) NOT NULL DEFAULT '',
    `password_type` VARCHAR(64) NOT NULL DEFAULT 'Plain',
    `filter` VARCHAR(255) NOT NULL DEFAULT '',
    `filter_fields` JSON NOT NULL,
    `enable_ssl` TINYINT(1) NOT NULL DEFAULT false,
    `allow_self_signed_cert` TINYINT(1) NOT NULL DEFAULT false,
    `enable_groups` TINYINT(1) NOT NULL DEFAULT false,
    `default_groups` JSON NOT NULL,
    `custom_attributes` JSON NOT NULL,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `sync_interval` INT NOT NULL DEFAULT 0,
    `last_sync_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_ldaps_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_ldaps_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ldap_sync_configs` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `ldap_id` CHAR(36) NOT NULL,
    `sync_interval_minutes` INT NOT NULL DEFAULT 60,
    `base_dn` TEXT NOT NULL,
    `user_filter` VARCHAR(64) DEFAULT '(objectClass=person)',
    `group_filter` VARCHAR(64) DEFAULT '(objectClass=group)',
    `enabled` TINYINT(1) NOT NULL DEFAULT true,
    `last_sync_at` TIMESTAMP,
    `next_sync_at` TIMESTAMP,
    `sync_status` VARCHAR(64) DEFAULT 'idle',
    `sync_error` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_ldap_sync_configs_ldap_id_ldaps FOREIGN KEY (`ldap_id`) REFERENCES `ldaps`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_ldap_sync_configs_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ldap_sync_configs' AND INDEX_NAME = 'idx_ldap_sync_configs_org_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ldap_sync_configs_org_id` ON `ldap_sync_configs` (`organization_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ldap_sync_configs' AND INDEX_NAME = 'idx_ldap_sync_configs_ldap_id' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ldap_sync_configs_ldap_id` ON `ldap_sync_configs` (`ldap_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `ldap_sync_tasks` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `ldap_id` CHAR(36) NOT NULL,
    `config_id` CHAR(36) NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'pending',
    `trigger_type` VARCHAR(32) NOT NULL,
    `users_synced` INT NOT NULL DEFAULT 0,
    `users_created` INT NOT NULL DEFAULT 0,
    `users_updated` INT NOT NULL DEFAULT 0,
    `users_failed` INT NOT NULL DEFAULT 0,
    `groups_synced` INT NOT NULL DEFAULT 0,
    `groups_created` INT NOT NULL DEFAULT 0,
    `error_message` VARCHAR(2000) NOT NULL DEFAULT '',
    `started_at` TIMESTAMP,
    `completed_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_ldap_sync_tasks_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed')),
    CONSTRAINT ck_ldap_sync_tasks_trigger_type CHECK (trigger_type IN ('manual', 'scheduled')),
    CONSTRAINT fk_ldap_sync_tasks_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_ldap_sync_tasks_ldap_id_ldaps FOREIGN KEY (`ldap_id`) REFERENCES `ldaps`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_ldap_sync_tasks_config_id_ldap_sync_configs FOREIGN KEY (`config_id`) REFERENCES `ldap_sync_configs`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ldap_sync_tasks' AND INDEX_NAME = 'idx_ldap_sync_tasks_ldap_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ldap_sync_tasks_ldap_created` ON `ldap_sync_tasks` (`ldap_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ldap_sync_tasks' AND INDEX_NAME = 'idx_ldap_sync_tasks_config' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ldap_sync_tasks_config` ON `ldap_sync_tasks` (`config_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ldap_sync_tasks' AND INDEX_NAME = 'idx_ldap_sync_tasks_org_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ldap_sync_tasks_org_status` ON `ldap_sync_tasks` (`organization_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `scim_schemas` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `schema_id` TEXT NOT NULL,
    `name` TEXT NOT NULL,
    `description` TEXT,
    `attributes` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_scim_schemas_org_schema_id UNIQUE (`organization_id`, `schema_id`(255)),
    CONSTRAINT fk_scim_schemas_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `notifications` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `provider_id` CHAR(36) NOT NULL,
    `user_notification_id` CHAR(36),
    `channel` VARCHAR(32),
    `provider_kind` VARCHAR(64),
    `recipient` TEXT NOT NULL,
    `subject` TEXT,
    `content` TEXT NOT NULL,
    `template_id` TEXT,
    `variables` JSON,
    `status` VARCHAR(64) DEFAULT 'pending',
    `error` TEXT,
    `sent_at` TIMESTAMP,
    `retry_count` INT DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_notifications_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_notifications_provider_id_notification_providers FOREIGN KEY (`provider_id`) REFERENCES `notification_providers`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notifications' AND INDEX_NAME = 'idx_notifications_org_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_notifications_org_status` ON `notifications` (`organization_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notifications' AND INDEX_NAME = 'idx_notifications_user_notification' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_notifications_user_notification` ON `notifications` (`user_notification_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notifications' AND INDEX_NAME = 'idx_notifications_status_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_notifications_status_created` ON `notifications` (`status`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_notifications` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36) NOT NULL,
    `category` VARCHAR(64) NOT NULL DEFAULT 'system',
    `event_type` VARCHAR(128) NOT NULL DEFAULT '',
    `title` TEXT NOT NULL,
    `content` TEXT NOT NULL,
    `link` VARCHAR(512) NOT NULL DEFAULT '',
    `source_type` VARCHAR(64) NOT NULL DEFAULT '',
    `source_id` CHAR(36),
    `allow_reply` TINYINT(1) NOT NULL DEFAULT false,
    `is_read` TINYINT(1) NOT NULL DEFAULT false,
    `read_at` TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_notifications_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_user_notifications_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_notifications' AND INDEX_NAME = 'idx_user_notifications_user_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_notifications_user_created` ON `user_notifications` (`organization_id`, `user_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_col_exists := (SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_notifications' AND COLUMN_NAME = 'gen_user_notifications_user_unread');
SET @_sql := IF(@_col_exists IS NULL,   'ALTER TABLE `user_notifications` ADD COLUMN gen_user_notifications_user_unread TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) STORED',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_notifications' AND INDEX_NAME = 'idx_user_notifications_user_unread' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_notifications_user_unread` ON `user_notifications` (`organization_id`, `user_id`, `is_read`, `gen_user_notifications_user_unread`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_notifications' AND INDEX_NAME = 'idx_user_notifications_event' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_notifications_event` ON `user_notifications` (`event_type`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `user_notification_replies` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `notification_id` CHAR(36) NOT NULL,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36) NOT NULL,
    `content` TEXT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT fk_user_notification_replies_notification_id_user_notifications FOREIGN KEY (`notification_id`) REFERENCES `user_notifications`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_user_notification_replies_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_user_notification_replies_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'user_notification_replies' AND INDEX_NAME = 'idx_user_notification_replies_notification' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_user_notification_replies_notification` ON `user_notification_replies` (`notification_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `notification_preferences` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36) NOT NULL,
    `category` VARCHAR(64) NOT NULL,
    `channel` VARCHAR(32) NOT NULL,
    `enabled` TINYINT(1) NOT NULL DEFAULT true,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_notification_preferences_user_category_channel UNIQUE (`user_id`, `category`, `channel`),
    CONSTRAINT fk_notification_preferences_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_notification_preferences_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notification_preferences' AND INDEX_NAME = 'idx_notification_preferences_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_notification_preferences_user` ON `notification_preferences` (`organization_id`, `user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `notification_channel_bindings` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36) NOT NULL,
    `channel` VARCHAR(32) NOT NULL,
    `external_id` VARCHAR(255) NOT NULL,
    `verified` TINYINT(1) NOT NULL DEFAULT false,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_notification_channel_bindings_user_channel_external UNIQUE (`user_id`, `channel`, `external_id`),
    CONSTRAINT fk_notification_channel_bindings_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_notification_channel_bindings_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notification_channel_bindings' AND INDEX_NAME = 'idx_notification_channel_bindings_user_channel' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_notification_channel_bindings_user_channel` ON `notification_channel_bindings` (`organization_id`, `user_id`, `channel`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `access_campaigns` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'draft',
    `start_date` TIMESTAMP,
    `end_date` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_access_campaigns_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT ck_access_campaigns_status CHECK (status IN ('draft', 'active', 'completed', 'cancelled')),
    CONSTRAINT fk_access_campaigns_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `sod_rules` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `role_a_id` CHAR(36) NOT NULL,
    `role_b_id` CHAR(36) NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_sod_rules_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT uq_sod_rules_org_roles UNIQUE (`organization_id`, `role_a_id`, `role_b_id`),
    CONSTRAINT fk_sod_rules_role_a_id_roles FOREIGN KEY (`role_a_id`) REFERENCES `roles`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_sod_rules_role_b_id_roles FOREIGN KEY (`role_b_id`) REFERENCES `roles`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_sod_rules_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `access_lifecycle` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36) NOT NULL,
    `role_id` CHAR(36) NOT NULL,
    `ticket_id` CHAR(36),
    `enforcer_id` CHAR(36),
    `resource` VARCHAR(255) NOT NULL DEFAULT '',
    `action` VARCHAR(64) NOT NULL DEFAULT '',
    `policy_id` CHAR(36),
    `granted_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `starts_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `expires_at` TIMESTAMP NOT NULL,
    `revoked_at` TIMESTAMP,
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `retry_count` INT NOT NULL DEFAULT 0,
    `last_error` VARCHAR(2000) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Last update timestamp (implied by revision / soft_delete)',
    CONSTRAINT ck_access_lifecycle_status CHECK (status IN ('active', 'expired', 'revoked')),
    CONSTRAINT fk_access_lifecycle_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_access_lifecycle_role_id_roles FOREIGN KEY (`role_id`) REFERENCES `roles`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_access_lifecycle_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_lifecycle' AND INDEX_NAME = 'idx_access_lifecycle_org_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_lifecycle_org_user` ON `access_lifecycle` (`organization_id`, `user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
-- INDEX idx_access_lifecycle_expires: predicate status = 'active' (MySQL 不支持 WHERE 子句，降级为普通索引)
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'access_lifecycle' AND INDEX_NAME = 'idx_access_lifecycle_expires' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_access_lifecycle_expires` ON `access_lifecycle` (`expires_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `risk_signals` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36),
    `signal_type` TEXT NOT NULL,
    `signal_value` JSON NOT NULL,
    `ip` VARCHAR(45),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_risk_signals_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_risk_signals_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'risk_signals' AND INDEX_NAME = 'idx_risk_signals_org_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_risk_signals_org_type` ON `risk_signals` (`organization_id`, `signal_type`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'risk_signals' AND INDEX_NAME = 'idx_risk_signals_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_risk_signals_user` ON `risk_signals` (`user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'risk_signals' AND INDEX_NAME = 'idx_risk_signals_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_risk_signals_created` ON `risk_signals` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `risk_decisions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `user_id` CHAR(36),
    `score` INT NOT NULL DEFAULT 0,
    `decision` VARCHAR(64) NOT NULL DEFAULT 'allow',
    `factors` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_risk_decisions_decision CHECK (decision IN ('allow', 'challenge', 'deny')),
    CONSTRAINT ck_risk_decisions_score CHECK (score >= 0 AND score <= 100),
    CONSTRAINT fk_risk_decisions_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_risk_decisions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'risk_decisions' AND INDEX_NAME = 'idx_risk_decisions_org_user' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_risk_decisions_org_user` ON `risk_decisions` (`organization_id`, `user_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'risk_decisions' AND INDEX_NAME = 'idx_risk_decisions_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_risk_decisions_created` ON `risk_decisions` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `session_governance` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `session_id` CHAR(36),
    `risk_score` INT NOT NULL DEFAULT 0,
    `action` VARCHAR(64) NOT NULL DEFAULT 'monitor',
    `reason` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_session_governance_action CHECK (action IN ('monitor', 'step_up', 'revoke')),
    CONSTRAINT fk_session_governance_session_id_sessions FOREIGN KEY (`session_id`) REFERENCES `sessions`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_session_governance_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'session_governance' AND INDEX_NAME = 'idx_session_governance_org_session' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_session_governance_org_session` ON `session_governance` (`organization_id`, `session_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'session_governance' AND INDEX_NAME = 'idx_session_governance_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_session_governance_created` ON `session_governance` (`created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `identity_actions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `trigger_type` VARCHAR(255) NOT NULL DEFAULT '',
    `status` VARCHAR(64) NOT NULL DEFAULT 'draft',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_identity_actions_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT ck_identity_actions_status CHECK (status IN ('draft', 'active', 'disabled', 'deleted')),
    CONSTRAINT fk_identity_actions_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `action_versions` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `action_id` CHAR(36) NOT NULL,
    `version` INT NOT NULL DEFAULT 1,
    `code` VARCHAR(255) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_action_versions_action_version UNIQUE (`action_id`, `version`),
    CONSTRAINT fk_action_versions_action_id_identity_actions FOREIGN KEY (`action_id`) REFERENCES `identity_actions`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `execution_logs` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `action_id` CHAR(36) NOT NULL,
    `version` INT NOT NULL DEFAULT 1,
    `status` VARCHAR(64) NOT NULL DEFAULT 'pending',
    `input` JSON NOT NULL,
    `output` JSON,
    `error` TEXT,
    `executed_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_execution_logs_status CHECK (status IN ('pending', 'running', 'success', 'failed', 'timeout')),
    CONSTRAINT fk_execution_logs_action_id_identity_actions FOREIGN KEY (`action_id`) REFERENCES `identity_actions`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'execution_logs' AND INDEX_NAME = 'idx_execution_logs_action' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_execution_logs_action` ON `execution_logs` (`action_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'execution_logs' AND INDEX_NAME = 'idx_execution_logs_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_execution_logs_status` ON `execution_logs` (`status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `scim_reconciliation_reports` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `connector_id` TEXT NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'completed',
    `started_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `finished_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `user_report` JSON NOT NULL,
    `group_report` JSON NOT NULL,
    `error_message` VARCHAR(2048) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_scim_reconciliation_reports_status CHECK (status IN ('completed', 'failed')),
    CONSTRAINT fk_scim_reconciliation_reports_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'scim_reconciliation_reports' AND INDEX_NAME = 'idx_scim_reconciliation_reports_org_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_scim_reconciliation_reports_org_created` ON `scim_reconciliation_reports` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `connectors` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `scim_base_url` TEXT NOT NULL,
    `bearer_token_sealed` TEXT NOT NULL,
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `timeout_ms` INT NOT NULL DEFAULT 5000,
    `max_rate_per_min` INT NOT NULL DEFAULT 300,
    `revision` BIGINT NOT NULL DEFAULT 1,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_connectors_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT fk_connectors_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'connectors' AND INDEX_NAME = 'idx_connectors_org_enabled' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_connectors_org_enabled` ON `connectors` (`organization_id`, `is_enabled`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `ssf_receivers` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `receiver_url` TEXT NOT NULL,
    `auth_token_hash` VARCHAR(255) NOT NULL DEFAULT '',
    `event_types` JSON NOT NULL,
    `status` VARCHAR(32) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` CHAR(36),
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_ssf_receivers_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT ck_ssf_receivers_status CHECK (status IN ('active', 'disabled')),
    CONSTRAINT fk_ssf_receivers_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ssf_receivers' AND INDEX_NAME = 'idx_ssf_receivers_org_status' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ssf_receivers_org_status` ON `ssf_receivers` (`organization_id`, `status`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `dsar_requests` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `subject_type` VARCHAR(16) NOT NULL,
    `subject_id` CHAR(36) NOT NULL,
    `request_type` VARCHAR(24) NOT NULL,
    `state` VARCHAR(16) NOT NULL DEFAULT 'received',
    `sla_due_at` TIMESTAMP NOT NULL,
    `closed_at` TIMESTAMP,
    `cooling_until` TIMESTAMP,
    `rejection_reason` VARCHAR(255),
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_by` CHAR(36),
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT ck_dsar_requests_type CHECK (request_type IN ('access', 'rectify', 'freeze', 'erase', 'portability', 'withdraw_consent')),
    CONSTRAINT ck_dsar_requests_state CHECK (state IN ('received', 'verifying', 'in_progress', 'fulfilled', 'rejected')),
    CONSTRAINT ck_dsar_requests_subject_type CHECK (subject_type IN ('user', 'contact')),
    CONSTRAINT ck_dsar_requests_closed CHECK (state NOT IN ('fulfilled', 'rejected') OR closed_at IS NOT NULL),
    CONSTRAINT fk_dsar_requests_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'dsar_requests' AND INDEX_NAME = 'idx_dsar_requests_org_state' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_dsar_requests_org_state` ON `dsar_requests` (`organization_id`, `state`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'dsar_requests' AND INDEX_NAME = 'idx_dsar_requests_sla' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_dsar_requests_sla` ON `dsar_requests` (`state`, `sla_due_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `dsar_events` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `dsar_id` CHAR(36) NOT NULL,
    `action` VARCHAR(32) NOT NULL,
    `actor_id` CHAR(36),
    `payload` JSON,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT ck_dsar_events_action CHECK (action IN ('created', 'verifying', 'in_progress', 'fulfilled', 'rejected')),
    CONSTRAINT fk_dsar_events_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_dsar_events_dsar_id_dsar_requests FOREIGN KEY (`dsar_id`) REFERENCES `dsar_requests`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_dsar_events_actor_id_users FOREIGN KEY (`actor_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'dsar_events' AND INDEX_NAME = 'idx_dsar_events_dsar_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_dsar_events_dsar_created` ON `dsar_events` (`dsar_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'dsar_events' AND INDEX_NAME = 'idx_dsar_events_org_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_dsar_events_org_created` ON `dsar_events` (`organization_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `consent_records` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `subject_type` VARCHAR(16) NOT NULL,
    `subject_id` CHAR(36) NOT NULL,
    `purpose` VARCHAR(64) NOT NULL,
    `version` INT NOT NULL,
    `granted_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `withdrawn_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_consent_records_version UNIQUE (`organization_id`, `subject_type`, `subject_id`, `purpose`, `version`),
    CONSTRAINT ck_consent_records_subject_type CHECK (subject_type IN ('user', 'contact')),
    CONSTRAINT ck_consent_records_withdrawn CHECK (withdrawn_at IS NULL OR withdrawn_at >= granted_at),
    CONSTRAINT fk_consent_records_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- 分组：82_support.yaml

CREATE TABLE IF NOT EXISTS `tickets` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `type` VARCHAR(100) DEFAULT '',
    `status` VARCHAR(50) NOT NULL DEFAULT 'open',
    `priority` VARCHAR(50) DEFAULT '',
    `assignee` VARCHAR(255) DEFAULT '',
    `creator` VARCHAR(255) DEFAULT '',
    `description` VARCHAR(255) DEFAULT '',
    `requester_id` CHAR(36),
    `role_id` CHAR(36),
    `application_id` CHAR(36),
    `enforcer_id` CHAR(36),
    `resource` VARCHAR(255) NOT NULL DEFAULT '',
    `action` VARCHAR(64) NOT NULL DEFAULT '',
    `starts_at` TIMESTAMP,
    `expires_at` TIMESTAMP,
    `workflow_state` VARCHAR(64),
    `manager_approved_by` CHAR(36),
    `sap_approved_by` CHAR(36),
    `lifecycle_id` CHAR(36),
    `attachment` VARCHAR(500) DEFAULT '',
    `resolved_at` TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_tickets_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT ck_tickets_status CHECK (status IN ('open', 'pending', 'resolved', 'closed')),
    CONSTRAINT fk_tickets_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `ticket_messages` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `ticket_id` CHAR(36) NOT NULL,
    `author_id` CHAR(36),
    `action` VARCHAR(64) NOT NULL,
    `content` TEXT NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_ticket_messages_ticket_id_tickets FOREIGN KEY (`ticket_id`) REFERENCES `tickets`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_ticket_messages_author_id_users FOREIGN KEY (`author_id`) REFERENCES `users`(`id`) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'ticket_messages' AND INDEX_NAME = 'idx_ticket_messages_ticket_created' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_ticket_messages_ticket_created` ON `ticket_messages` (`ticket_id`, `created_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `forms` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` VARCHAR(255) NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` VARCHAR(255) DEFAULT '',
    `type` VARCHAR(100) DEFAULT '',
    `properties` VARCHAR(255) DEFAULT '',
    `is_enabled` TINYINT(1) NOT NULL DEFAULT true,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `deleted_at` TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_forms_org_name UNIQUE (`organization_id`, `name`),
    CONSTRAINT fk_forms_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- 分组：85_workload.yaml

CREATE TABLE IF NOT EXISTS `identity_pools` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `display_name` VARCHAR(255) NOT NULL DEFAULT '',
    `description` TEXT,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_identity_pools_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT fk_identity_pools_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `federation_trusts` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `pool_id` CHAR(36) NOT NULL,
    `name` TEXT NOT NULL,
    `issuer` TEXT NOT NULL,
    `jwks_uri` VARCHAR(255) NOT NULL DEFAULT '',
    `audience` JSON NOT NULL,
    `subjects` JSON NOT NULL,
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_federation_trusts_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT ck_federation_trusts_status CHECK (status IN ('active', 'disabled', 'deleted')),
    CONSTRAINT fk_federation_trusts_pool_id_identity_pools FOREIGN KEY (`pool_id`) REFERENCES `identity_pools`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_federation_trusts_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'federation_trusts' AND INDEX_NAME = 'idx_federation_trusts_pool' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_federation_trusts_pool` ON `federation_trusts` (`pool_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `service_accounts` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `pool_id` CHAR(36),
    `name` TEXT NOT NULL,
    `description` TEXT,
    `client_id` TEXT NOT NULL,
    `client_secret_hash` VARCHAR(255) NOT NULL DEFAULT '',
    `scopes` JSON NOT NULL,
    `access_token_ttl_seconds` INT NOT NULL DEFAULT 900,
    `last_rotated_at` TIMESTAMP,
    `status` VARCHAR(64) NOT NULL DEFAULT 'active',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_service_accounts_org_name UNIQUE (`organization_id`, `name`(255)),
    CONSTRAINT uq_service_accounts_client_id UNIQUE (`client_id`(255)),
    CONSTRAINT ck_service_accounts_status CHECK (status IN ('active', 'disabled', 'deleted')),
    CONSTRAINT ck_service_accounts_access_token_ttl CHECK (access_token_ttl_seconds BETWEEN 60 AND 3600),
    CONSTRAINT fk_service_accounts_pool_id_identity_pools FOREIGN KEY (`pool_id`) REFERENCES `identity_pools`(`id`) ON DELETE SET NULL,
    CONSTRAINT fk_service_accounts_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'service_accounts' AND INDEX_NAME = 'idx_service_accounts_pool' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_service_accounts_pool` ON `service_accounts` (`pool_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


-- 分组：90_metering.yaml

CREATE TABLE IF NOT EXISTS `metering_events` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `event_type` TEXT NOT NULL,
    `subject` VARCHAR(255) NOT NULL DEFAULT '',
    `quantity` BIGINT NOT NULL DEFAULT 0,
    `unit` VARCHAR(255) NOT NULL DEFAULT '',
    `idempotency_key` TEXT NOT NULL,
    `occurred_at` TIMESTAMP NOT NULL,
    `metadata` JSON NOT NULL,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_metering_events_org_idem UNIQUE (`organization_id`, `idempotency_key`(255)),
    CONSTRAINT fk_metering_events_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'metering_events' AND INDEX_NAME = 'idx_metering_events_org_type' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_metering_events_org_type` ON `metering_events` (`organization_id`, `event_type`(255))', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'metering_events' AND INDEX_NAME = 'idx_metering_events_occurred' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_metering_events_occurred` ON `metering_events` (`organization_id`, `occurred_at`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `metering_aggregates` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `period` TEXT NOT NULL,
    `subject` VARCHAR(255) NOT NULL DEFAULT '',
    `metric` TEXT NOT NULL,
    `quantity` BIGINT NOT NULL DEFAULT 0,
    `unit` VARCHAR(255) NOT NULL DEFAULT '',
    `calculated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_metering_aggregates_org_period_subject_metric UNIQUE (`organization_id`(36), `period`(192), `subject`(192), `metric`(192)),
    CONSTRAINT fk_metering_aggregates_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS `plan_quotas` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `plan_id` CHAR(36) NOT NULL,
    `quota_key` VARCHAR(64) NOT NULL,
    `soft_limit` BIGINT,
    `hard_limit` BIGINT NOT NULL,
    `unit` VARCHAR(32) NOT NULL DEFAULT '',
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_plan_quotas_key UNIQUE (`organization_id`, `plan_id`, `quota_key`),
    CONSTRAINT ck_plan_quotas_hard_limit CHECK (hard_limit >= 0),
    CONSTRAINT ck_plan_quotas_soft_limit CHECK (soft_limit IS NULL OR (soft_limit >= 0 AND soft_limit <= hard_limit)),
    CONSTRAINT fk_plan_quotas_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE,
    CONSTRAINT fk_plan_quotas_plan_id_plans FOREIGN KEY (`plan_id`) REFERENCES `plans`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
SET @_idx_exists := (SELECT INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'plan_quotas' AND INDEX_NAME = 'idx_plan_quotas_plan' LIMIT 1);
SET @_sql := IF(@_idx_exists IS NULL, 'CREATE INDEX `idx_plan_quotas_plan` ON `plan_quotas` (`plan_id`)', 'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

CREATE TABLE IF NOT EXISTS `tenant_quotas` (
    `id` CHAR(36) NOT NULL PRIMARY KEY,
    `organization_id` CHAR(36) NOT NULL,
    `quota_key` VARCHAR(64) NOT NULL,
    `limit_value` BIGINT NOT NULL,
    `used_value` BIGINT NOT NULL DEFAULT 0,
    `created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `revision` BIGINT NOT NULL DEFAULT 1,
    CONSTRAINT uq_tenant_quotas_key UNIQUE (`organization_id`, `quota_key`),
    CONSTRAINT ck_tenant_quotas_limit CHECK (limit_value >= 0),
    CONSTRAINT ck_tenant_quotas_used CHECK (used_value >= 0),
    CONSTRAINT fk_tenant_quotas_organization_id_organizations FOREIGN KEY (`organization_id`) REFERENCES `organizations`(`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 延迟外键（条件化执行，避免前向引用）

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'platform_admins' AND CONSTRAINT_NAME = 'fk_platform_admins_user_id_users');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `platform_admins` ADD CONSTRAINT fk_platform_admins_user_id_users FOREIGN KEY (`user_id`) REFERENCES `users`(`id`) ON DELETE CASCADE;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'platform_admins' AND CONSTRAINT_NAME = 'fk_platform_admins_created_by_users');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `platform_admins` ADD CONSTRAINT fk_platform_admins_created_by_users FOREIGN KEY (`created_by`) REFERENCES `users`(`id`) ON DELETE SET NULL;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'platform_admin_scopes' AND CONSTRAINT_NAME = 'fk_platform_admin_scopes_approved_by_users');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `platform_admin_scopes` ADD CONSTRAINT fk_platform_admin_scopes_approved_by_users FOREIGN KEY (`approved_by`) REFERENCES `users`(`id`) ON DELETE SET NULL;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'users' AND CONSTRAINT_NAME = 'fk_users_signup_application_id');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `users` ADD CONSTRAINT fk_users_signup_application_id FOREIGN KEY (`signup_application_id`) REFERENCES `applications`(`id`) ON DELETE SET NULL;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'application_provider_bindings' AND CONSTRAINT_NAME = 'fk_app_provider_bindings_provider_id');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `application_provider_bindings` ADD CONSTRAINT fk_app_provider_bindings_provider_id FOREIGN KEY (`provider_id`) REFERENCES `providers`(`id`) ON DELETE CASCADE;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'web3_identity_mappings' AND CONSTRAINT_NAME = 'fk_web3_identity_mappings_credential_id_webauthn_credentials');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `web3_identity_mappings` ADD CONSTRAINT fk_web3_identity_mappings_credential_id_webauthn_credentials FOREIGN KEY (`credential_id`) REFERENCES `webauthn_credentials`(`id`) ON DELETE CASCADE;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'casbin_rules' AND CONSTRAINT_NAME = 'fk_casbin_rules_enforcer_id_enforcers');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `casbin_rules` ADD CONSTRAINT fk_casbin_rules_enforcer_id_enforcers FOREIGN KEY (`enforcer_id`) REFERENCES `enforcers`(`id`) ON DELETE CASCADE;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @_cname := (SELECT CONSTRAINT_NAME FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'notifications' AND CONSTRAINT_NAME = 'fk_notifications_user_notification_id_user_notifications');
SET @_sql := IF(@_cname IS NULL,   'ALTER TABLE `notifications` ADD CONSTRAINT fk_notifications_user_notification_id_user_notifications FOREIGN KEY (`user_notification_id`) REFERENCES `user_notifications`(`id`) ON DELETE CASCADE;',   'SELECT 1');
PREPARE stmt FROM @_sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;


SET FOREIGN_KEY_CHECKS = 1;
