// Package regions 是 ADR-031 Decision 1 + Decision 3 的 PostgreSQL DB 实现。
// 同时实现 domain/regions.RegionResolver 和 domain/regions.RegionKeyResolver。
// MySQL 版: internal/infra/mysql/regions
package regions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	domaincrypto "github.com/guliangiam/guliangiam/internal/domain/crypto"
	domain "github.com/guliangiam/guliangiam/internal/domain/regions"
	cryptorepo "github.com/guliangiam/guliangiam/internal/infra/postgres/crypto"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/sqlguard"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

var _ domain.RegionResolver = Repository{}

// ── Decision 1: RegionResolver ──

func (r Repository) Resolve(ctx context.Context, orgID string) (string, error) {
	var code string
	err := r.db.QueryRow(ctx, "SELECT region_code FROM guliangiam.organizations WHERE id = $1 AND region_code != ''", orgID).Scan(&code)
	if err == nil {
		return code, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", apperrors.WrapInternalf(err, "query organizations.region_code")
	}

	var regionCode string
	err = r.db.QueryRow(ctx,
		`SELECT dr.region_code FROM guliangiam.tenant_instances ti
		 JOIN guliangiam.deployment_regions dr ON ti.region_id = dr.id
		 WHERE ti.organization_id = $1 AND ti.is_primary = TRUE LIMIT 1`,
		orgID,
	).Scan(&regionCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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

		var ph string
		args := make([]any, 0, len(batch))
		for idx, id := range batch {
			if idx > 0 {
				ph += ","
			}
			ph += fmt.Sprintf("$%d", idx+1)
			args = append(args, id)
		}
		offset := len(args)
		var ph2 string
		for idx := range batch {
			if idx > 0 {
				ph2 += ","
			}
			ph2 += fmt.Sprintf("$%d", offset+idx+1)
		}
		if err := sqlguard.ValidINPlaceholders(ph); err != nil {
			return nil, apperrors.NewInternal("sql guard violation: " + err.Error())
		}
		if err := sqlguard.ValidINPlaceholders(ph2); err != nil {
			return nil, apperrors.NewInternal("sql guard violation: " + err.Error())
		}

		fullSQL := fmt.Sprintf(`
			SELECT o.id, o.region_code FROM guliangiam.organizations o
			WHERE o.id IN (%s) AND o.region_code != ''
			UNION ALL
			SELECT ti.organization_id, dr.region_code
			FROM guliangiam.tenant_instances ti JOIN guliangiam.deployment_regions dr ON ti.region_id = dr.id
			WHERE ti.organization_id IN (%s) AND ti.is_primary = TRUE
			  AND ti.organization_id NOT IN (
			    SELECT o2.id FROM guliangiam.organizations o2
			    WHERE o2.id IN (%s) AND o2.region_code != ''
			  )`, ph, ph2, ph2)

		allArgs := append(append([]any{}, args...), args...)
		rows, err := r.db.Query(ctx, fullSQL, allArgs...)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "batch resolve")
		}
		for rows.Next() {
			var oid, code string
			if err := rows.Scan(&oid, &code); err != nil {
				rows.Close()
				return nil, apperrors.WrapInternalf(err, "scan batch")
			}
			result[oid] = code
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
