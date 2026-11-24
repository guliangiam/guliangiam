// Package regions 是 ADR-031 Decision 1 + Decision 3 的 MySQL 8.4+ 区域解析 DB 实现。
// 同时实现 domain/regions.RegionResolver 和 domain/regions.RegionKeyResolver 接口。
//
// Decision 1 查询路径：organizations.region_code (V4 migration 冗余列) 快速命中
// Decision 3 KEK 路由：委托给 infra/mysql/crypto EncryptionKeyRepository
package regions

import (
	"context"
	"database/sql"

	domaincrypto "github.com/guliangiam/guliangiam/internal/domain/crypto"
	domain "github.com/guliangiam/guliangiam/internal/domain/regions"
	cryptorepo "github.com/guliangiam/guliangiam/internal/infra/mysql/crypto"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 RegionResolver + RegionKeyResolver。
type Repository struct {
	db mysqlshared.DBTX
}

func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

var _ domain.RegionResolver = Repository{}

// ── Decision 1: RegionResolver ──

func (r Repository) Resolve(ctx context.Context, orgID string) (string, error) {
	var code sql.NullString
	err := r.db.QueryRowContext(ctx,
		"SELECT region_code FROM guliangiam.organizations WHERE id = ? AND region_code != ''", orgID,
	).Scan(&code)
	if err == nil && code.Valid {
		return code.String, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return "", apperrors.WrapInternalf(err, "query organizations.region_code")
	}

	var regionCode string
	err = r.db.QueryRowContext(ctx,
		`SELECT dr.region_code
		 FROM guliangiam.tenant_instances ti
		 JOIN guliangiam.deployment_regions dr ON ti.region_id = dr.id
		 WHERE ti.organization_id = ? AND ti.is_primary = TRUE LIMIT 1`,
		orgID,
	).Scan(&regionCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", apperrors.NewNotFoundf("region binding for org %s", orgID)
		}
		return "", apperrors.WrapInternalf(err, "query tenant_instances")
	}
	return regionCode, nil
}

func (r Repository) ResolveBatch(ctx context.Context, orgIDs []string) (map[string]string, error) {
	result := make(map[string]string)
	if len(orgIDs) == 0 {
		return result, nil
	}

	const batchSize = 500
	for i := 0; i < len(orgIDs); i += batchSize {
		end := i + batchSize
		if end > len(orgIDs) {
			end = len(orgIDs)
		}
		batch := orgIDs[i:end]

		ph, args := "", make([]any, 0, len(batch))
		for idx, id := range batch {
			if idx > 0 {
				ph += ","
			}
			ph += "?"
			args = append(args, id)
		}

		rows, err := r.db.QueryContext(ctx,
			`SELECT o.id, o.region_code FROM guliangiam.organizations o WHERE o.id IN (`+ph+`) AND o.region_code != ''
			 UNION ALL
			 SELECT ti.organization_id, dr.region_code
			 FROM guliangiam.tenant_instances ti JOIN guliangiam.deployment_regions dr ON ti.region_id = dr.id
			 WHERE ti.organization_id IN (`+ph+`) AND ti.is_primary = TRUE
			   AND ti.organization_id NOT IN (
			     SELECT o2.id FROM guliangiam.organizations o2 WHERE o2.id IN (`+ph+`) AND o2.region_code != ''
			   )`,
			append(append([]any{}, args...), append([]any{}, args...)...)...,
		)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "batch resolve")
		}
		for rows.Next() {
			var orgID, code string
			if err := rows.Scan(&orgID, &code); err != nil {
				rows.Close()
				return nil, apperrors.WrapInternalf(err, "scan batch")
			}
			result[orgID] = code
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, apperrors.WrapInternalf(err, "iterate batch")
		}
	}
	return result, nil
}

// ── Decision 3: RegionKeyResolver ──

func (r Repository) ResolveKEKs(ctx context.Context, orgID, purpose string) (keysOut map[int]*domaincrypto.EncryptionKey, versionOut int, err error) {
	cr := cryptorepo.NewRepository(r.db)
	keys, err := cr.ListForDecrypt(ctx, orgID)
	if err != nil {
		return nil, 0, err
	}

	m := make(map[int]*domaincrypto.EncryptionKey)
	var currentVersion int
	foundActive := false
	for _, k := range keys {
		if k.Purpose != purpose {
			continue
		}
		m[k.KeyVersion] = k
		if k.IsUsableForEncrypt() && k.KeyVersion > currentVersion {
			currentVersion = k.KeyVersion
			foundActive = true
		}
	}
	if !foundActive && len(m) > 0 {
		for v := range m {
			if v > currentVersion {
				currentVersion = v
			}
		}
	}
	return m, currentVersion, nil
}

func (r Repository) CurrentKEKVersion(ctx context.Context, orgID, purpose string) (int, error) {
	m, cur, err := r.ResolveKEKs(ctx, orgID, purpose)
	if err != nil {
		return 0, err
	}
	if len(m) == 0 {
		return 0, apperrors.NewNotFoundf("no KEK for purpose %s org %s", purpose, orgID)
	}
	return cur, nil
}
