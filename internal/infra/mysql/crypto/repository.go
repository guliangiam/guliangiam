// Package crypto 是 ADR-031 Decision 3 的 MySQL 8.4+ EncryptionKey 查询 DB 实现。
// 实现 domain/crypto.EncryptionKeyRepository 接口。
//
// 优先级（scope）: organization > region > global
// 状态筛选：active / retiring / retired / revoked 都可用于解密；只有 active/retiring 可用于新加密
package crypto

import (
	"context"
	"database/sql"

	domaincrypto "github.com/guliangiam/guliangiam/internal/domain/crypto"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 EncryptionKeyRepository。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

var _ domaincrypto.EncryptionKeyRepository = Repository{}

// scanEncryptionKey 把 SQL Row/Scan 结果映射到 domaincrypto.EncryptionKey。
func scanEncryptionKey(scanner interface{ Scan(...any) error }) (*domaincrypto.EncryptionKey, error) {
	var k domaincrypto.EncryptionKey
	var rev, ret, act, rot sql.NullTime
	if err := scanner.Scan(
		&k.ID, &k.KeyID, &k.KeyVersion, &k.Algorithm, &k.Purpose, &k.Status,
		&k.KeyMaterialRef, &k.KeyFingerprint,
		&k.TenantScope, &k.ScopeID, &k.RegionCode, &k.SovereigntyMode,
		&rev, &ret, &act, &rot,
	); err != nil {
		return nil, err
	}
	if rev.Valid {
		t := rev.Time
		k.RevokedAt = &t
	}
	if ret.Valid {
		t := ret.Time
		k.RetiredAt = &t
	}
	if act.Valid {
		t := act.Time
		k.ActivatedAt = &t
	}
	if rot.Valid {
		t := rot.Time
		k.RotateAfter = &t
	}
	return &k, nil
}

// ListActiveForOrg 列出 org 可用的所有 KEK（active + retiring，用于解密 fallback）。
// 按 scope 优先级排序：organization > region > global，再按 version ASC。
func (r Repository) ListActiveForOrg(ctx context.Context, orgID string) ([]*domaincrypto.EncryptionKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ek.id, ek.key_id, ek.key_version, ek.algorithm, ek.purpose, ek.status,
		        ek.key_material_ref, ek.key_fingerprint,
		        ek.tenant_scope, ek.scope_id, ek.region_code, ek.sovereignty_mode,
		        ek.revoked_at, ek.retired_at, ek.activated_at, ek.rotate_after
		 FROM guliangiam.encryption_keys ek
		 WHERE ek.status IN ('active', 'retiring')
		   AND (
		     (ek.tenant_scope = 'organization' AND ek.scope_id = ?)
		     OR (ek.tenant_scope = 'region' AND ek.scope_id = (
		         SELECT o.region_code FROM guliangiam.organizations o WHERE o.id = ?
		         UNION
		         SELECT dr.region_code FROM guliangiam.tenant_instances ti
		         JOIN guliangiam.deployment_regions dr ON ti.region_id = dr.id
		         WHERE ti.organization_id = ? AND ti.is_primary = TRUE
		         LIMIT 1
		     ))
		     OR ek.tenant_scope = 'global'
		   )
		 ORDER BY
		   CASE ek.tenant_scope
		     WHEN 'organization' THEN 1
		     WHEN 'region' THEN 2
		     WHEN 'global' THEN 3
		     ELSE 99
		   END,
		   ek.key_version ASC`,
		orgID, orgID, orgID,
	)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "query active KEKs for org %s", orgID)
	}
	defer rows.Close()

	keys := []*domaincrypto.EncryptionKey{}
	for rows.Next() {
		k, err := scanEncryptionKey(rows)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "scan encryption key")
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// ListForDecrypt 列出可用于解密的 KEK（active / retiring / retired / revoked）。
// 吊销的 KEK 也能解旧 token — Experience 100007470 安全分层。
func (r Repository) ListForDecrypt(ctx context.Context, orgID string) ([]*domaincrypto.EncryptionKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT ek.id, ek.key_id, ek.key_version, ek.algorithm, ek.purpose, ek.status,
		        ek.key_material_ref, ek.key_fingerprint,
		        ek.tenant_scope, ek.scope_id, ek.region_code, ek.sovereignty_mode,
		        ek.revoked_at, ek.retired_at, ek.activated_at, ek.rotate_after
		 FROM guliangiam.encryption_keys ek
		 WHERE ek.status IN ('active', 'retiring', 'retired', 'revoked')
		   AND (
		     (ek.tenant_scope = 'organization' AND ek.scope_id = ?)
		     OR (ek.tenant_scope = 'region' AND ek.scope_id = (
		         SELECT o.region_code FROM guliangiam.organizations o WHERE o.id = ?
		         UNION
		         SELECT dr.region_code FROM guliangiam.tenant_instances ti
		         JOIN guliangiam.deployment_regions dr ON ti.region_id = dr.id
		         WHERE ti.organization_id = ? AND ti.is_primary = TRUE
		         LIMIT 1
		     ))
		     OR ek.tenant_scope = 'global'
		   )
		 ORDER BY
		   CASE ek.tenant_scope
		     WHEN 'organization' THEN 1
		     WHEN 'region' THEN 2
		     WHEN 'global' THEN 3
		     ELSE 99
		   END,
		   ek.key_version ASC`,
		orgID, orgID, orgID,
	)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "query decrypt KEKs for org %s", orgID)
	}
	defer rows.Close()

	keys := []*domaincrypto.EncryptionKey{}
	for rows.Next() {
		k, err := scanEncryptionKey(rows)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "scan encryption key")
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// GetByVersion 按 purpose + key_version 精确查（token 里带 version 时用）。
func (r Repository) GetByVersion(ctx context.Context, purpose string, keyVersion int) (*domaincrypto.EncryptionKey, error) {
	var k domaincrypto.EncryptionKey
	var rev, ret, act, rot sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, key_id, key_version, algorithm, purpose, status,
		        key_material_ref, key_fingerprint,
		        tenant_scope, scope_id, region_code, sovereignty_mode,
		        revoked_at, retired_at, activated_at, rotate_after
		 FROM guliangiam.encryption_keys
		 WHERE purpose = ? AND key_version = ?
		 LIMIT 1`,
		purpose, keyVersion,
	).Scan(
		&k.ID, &k.KeyID, &k.KeyVersion, &k.Algorithm, &k.Purpose, &k.Status,
		&k.KeyMaterialRef, &k.KeyFingerprint,
		&k.TenantScope, &k.ScopeID, &k.RegionCode, &k.SovereigntyMode,
		&rev, &ret, &act, &rot,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFoundf("encryption key purpose=%s version=%d", purpose, keyVersion)
		}
		return nil, apperrors.WrapInternalf(err, "get encryption key purpose=%s version=%d", purpose, keyVersion)
	}
	if rev.Valid {
		t := rev.Time
		k.RevokedAt = &t
	}
	if ret.Valid {
		t := ret.Time
		k.RetiredAt = &t
	}
	if act.Valid {
		t := act.Time
		k.ActivatedAt = &t
	}
	if rot.Valid {
		t := rot.Time
		k.RotateAfter = &t
	}
	return &k, nil
}
