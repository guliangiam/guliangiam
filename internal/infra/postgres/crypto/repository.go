// Package crypto 是 ADR-031 Decision 3 的 PostgreSQL EncryptionKeyRepository DB 实现。
// MySQL 版: internal/infra/mysql/crypto
package crypto

import (
	"context"
	"errors"

	domaincrypto "github.com/guliangiam/guliangiam/internal/domain/crypto"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// Repository 是 PostgreSQL 实现的 EncryptionKeyRepository。
type Repository struct {
	db shared.DBTX
}

// NewRepository 接受 pgx/v5 Conn / ConnPool / Tx。
func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

var _ domaincrypto.EncryptionKeyRepository = Repository{}

func scanEncryptionKey(scanner interface{ Scan(...any) error }) (*domaincrypto.EncryptionKey, error) {
	var k domaincrypto.EncryptionKey
	err := scanner.Scan(
		&k.ID, &k.KeyID, &k.KeyVersion, &k.Algorithm, &k.Purpose, &k.Status,
		&k.KeyMaterialRef, &k.KeyFingerprint,
		&k.TenantScope, &k.ScopeID, &k.RegionCode, &k.SovereigntyMode,
		&k.RevokedAt, &k.RetiredAt, &k.ActivatedAt, &k.RotateAfter,
	)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ListActiveForOrg 列出 org 可用的 KEK（active + retiring）。
func (r Repository) ListActiveForOrg(ctx context.Context, orgID string) ([]*domaincrypto.EncryptionKey, error) {
	return r.queryForOrg(ctx, orgID, "'active', 'retiring'")
}

// ListForDecrypt 列出可用于解密的 KEK（active / retiring / retired / revoked）。
func (r Repository) ListForDecrypt(ctx context.Context, orgID string) ([]*domaincrypto.EncryptionKey, error) {
	return r.queryForOrg(ctx, orgID, "'active', 'retiring', 'retired', 'revoked'")
}

// queryForOrg 是 ListActiveForOrg + ListForDecrypt 的共享查询。
// postgres 用 $N 占位符 + LIMIT 1 子查询获取 org region_code。
func (r Repository) queryForOrg(ctx context.Context, orgID, statusIn string) ([]*domaincrypto.EncryptionKey, error) {
	sql := `
		SELECT ek.id, ek.key_id, ek.key_version, ek.algorithm, ek.purpose, ek.status,
		       ek.key_material_ref, ek.key_fingerprint,
		       ek.tenant_scope, ek.scope_id, ek.region_code, ek.sovereignty_mode,
		       ek.revoked_at, ek.retired_at, ek.activated_at, ek.rotate_after
		FROM guliangiam.encryption_keys ek
		WHERE ek.status IN (` + statusIn + `)
		  AND (
		    (ek.tenant_scope = 'organization' AND ek.scope_id = $1)
		    OR (ek.tenant_scope = 'region' AND ek.scope_id = (
		        SELECT COALESCE(
		          (SELECT o.region_code FROM guliangiam.organizations o WHERE o.id = $1 AND o.region_code != ''),
		          (SELECT dr.region_code FROM guliangiam.tenant_instances ti
		           JOIN guliangiam.deployment_regions dr ON ti.region_id = dr.id
		           WHERE ti.organization_id = $1 AND ti.is_primary = TRUE
		           LIMIT 1)
		        )
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
		  ek.key_version ASC`

	rows, err := r.db.Query(ctx, sql, orgID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "query KEKs for org %s", orgID)
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

// GetByVersion 按 purpose + key_version 精确查。
func (r Repository) GetByVersion(ctx context.Context, purpose string, keyVersion int) (*domaincrypto.EncryptionKey, error) {
	var k domaincrypto.EncryptionKey
	err := r.db.QueryRow(ctx,
		`SELECT id, key_id, key_version, algorithm, purpose, status,
		        key_material_ref, key_fingerprint,
		        tenant_scope, scope_id, region_code, sovereignty_mode,
		        revoked_at, retired_at, activated_at, rotate_after
		 FROM guliangiam.encryption_keys
		 WHERE purpose = $1 AND key_version = $2
		 LIMIT 1`,
		purpose, keyVersion,
	).Scan(
		&k.ID, &k.KeyID, &k.KeyVersion, &k.Algorithm, &k.Purpose, &k.Status,
		&k.KeyMaterialRef, &k.KeyFingerprint,
		&k.TenantScope, &k.ScopeID, &k.RegionCode, &k.SovereigntyMode,
		&k.RevokedAt, &k.RetiredAt, &k.ActivatedAt, &k.RotateAfter,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFoundf("encryption key purpose=%s version=%d", purpose, keyVersion)
		}
		return nil, apperrors.WrapInternalf(err, "get encryption key purpose=%s version=%d", purpose, keyVersion)
	}
	return &k, nil
}
