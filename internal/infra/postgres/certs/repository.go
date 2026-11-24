package certs

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Cert, error) {
	var row certRow
	err := r.db.QueryRow(ctx, queryGetCertByID, id, organizationID).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
		&row.Scope, &row.Type, &row.CryptoAlgorithm, &row.BitSize, &row.ExpireInYears,
		&row.ExpireTime, &row.Certificate, &row.PrivateKey,
		&row.AuthorityPublicKey, &row.AuthorityRootPublicKey,
		&row.AcmeIssuer, &row.AcmeEndpoint,
		&row.Provider, &row.Account, &row.AccessKey, &row.AccessSecret, &row.Domain,
		&row.DomainExpireTime,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("cert not found")
		}
		return nil, err
	}

	cert, err := toCert(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode cert")
	}
	return cert, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Cert, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListCerts,
		query.OrganizationID, query.Keyword, query.Scope, query.Type,
		shared.ClampPageSize(query.PageSize), offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Cert, 0)
	for rows.Next() {
		var row certRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.Scope, &row.Type, &row.CryptoAlgorithm, &row.BitSize, &row.ExpireInYears,
			&row.ExpireTime, &row.Certificate, &row.PrivateKey,
			&row.AuthorityPublicKey, &row.AuthorityRootPublicKey,
			&row.AcmeIssuer, &row.AcmeEndpoint,
			&row.Provider, &row.Account, &row.AccessKey, &row.AccessSecret, &row.Domain,
			&row.DomainExpireTime,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		cert, err := toCert(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode cert")
		}
		items = append(items, cert)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountCerts,
		query.OrganizationID, query.Keyword, query.Scope, query.Type,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, cert *domain.Cert) error {
	domainJSON, err := shared.MarshalJSON(cert.Domain)
	if err != nil {
		return err
	}
	privateKey, accessSecret, err := certSecretsForWrite(cert)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryInsertCert,
		cert.ID, cert.OrganizationID, cert.Name, cert.DisplayName,
		cert.Scope, cert.Type, cert.CryptoAlgorithm, cert.BitSize, cert.ExpireInYears,
		cert.ExpireTime, cert.Certificate, privateKey,
		cert.AuthorityPublicKey, cert.AuthorityRootPublicKey,
		cert.AcmeIssuer, cert.AcmeEndpoint,
		cert.Provider, cert.Account, cert.AccessKey, accessSecret, domainJSON,
		cert.DomainExpireTime,
		cert.CreatedAt, cert.UpdatedAt, cert.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, cert *domain.Cert) error {
	domainJSON, err := shared.MarshalJSON(cert.Domain)
	if err != nil {
		return err
	}
	privateKey, accessSecret, err := certSecretsForWrite(cert)
	if err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateCert,
		cert.DisplayName, cert.Scope, cert.Type, cert.CryptoAlgorithm,
		cert.BitSize, cert.ExpireInYears, cert.ExpireTime, cert.Certificate,
		privateKey,
		cert.AuthorityPublicKey, cert.AuthorityRootPublicKey,
		cert.AcmeIssuer, cert.AcmeEndpoint,
		cert.Provider, cert.Account, cert.AccessKey,
		accessSecret, domainJSON, cert.DomainExpireTime,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		cert.UpdatedAt, cert.Revision-1, cert.ID, cert.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("cert revision conflict")
		}
		return err
	}
	cert.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteCert, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("cert not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

// ListExpiring 扫描即将过期的证书。仅供维护作业在系统作用域调用：不做组织
// 过滤，跨租户返回候选行由调用方按 organization_id 分组投递。
func (r Repository) ListExpiring(ctx context.Context, now, deadline time.Time, limit int) ([]*domain.Cert, error) {
	rows, err := r.db.Query(ctx, queryListExpiringCerts, now, deadline, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.Cert, 0)
	for rows.Next() {
		var row certRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName,
			&row.Scope, &row.Type, &row.CryptoAlgorithm, &row.BitSize, &row.ExpireInYears,
			&row.ExpireTime, &row.Certificate, &row.PrivateKey,
			&row.AuthorityPublicKey, &row.AuthorityRootPublicKey,
			&row.AcmeIssuer, &row.AcmeEndpoint,
			&row.Provider, &row.Account, &row.AccessKey, &row.AccessSecret, &row.Domain,
			&row.DomainExpireTime,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, err
		}
		cert, err := toCert(row)
		if err != nil {
			return nil, apperrors.NewInternal("decode cert")
		}
		items = append(items, cert)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

var _ domain.ExpiringScanner = Repository{}
