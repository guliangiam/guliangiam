// Package web3identity is the MySQL implementation of domain/web3identity.Repository.
package web3identity

import (
	"context"
	"database/sql"
	"errors"
	"time"

	domainweb3identity "github.com/guliangiam/guliangiam/internal/domain/web3identity"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 实现的 web3_identity_mappings 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

var _ domainweb3identity.Repository = Repository{}

func scanMapping(row interface {
	Scan(dest ...any) error
}) (*domainweb3identity.Mapping, error) {
	var m domainweb3identity.Mapping
	err := row.Scan(
		&m.ID, &m.UserID, &m.CredentialID, &m.WalletAddress,
		&m.DerivationMethod, &m.DerivationSeed, &m.ChainIDs, &m.EnsName, &m.IsPrimary,
		&m.VerifiedAt, &m.LastUsedAt, &m.CreatedAt, &m.UpdatedAt, &m.Revision,
	)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r Repository) Create(ctx context.Context, m *domainweb3identity.Mapping) error {
	_, err := r.db.ExecContext(ctx, queryCreate,
		m.ID, m.UserID, m.CredentialID, m.WalletAddress,
		m.DerivationMethod, m.DerivationSeed, m.ChainIDs, m.EnsName, m.IsPrimary,
		m.VerifiedAt, m.LastUsedAt,
	)
	if err != nil {
		return apperrors.WrapInternalf(err, "insert web3 identity mapping for credential %s", m.CredentialID)
	}
	return nil
}

func (r Repository) GetByCredentialID(ctx context.Context, credentialID string) (*domainweb3identity.Mapping, error) {
	row := r.db.QueryRowContext(ctx, queryByCredentialID, credentialID)
	m, err := scanMapping(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainweb3identity.ErrNotFound
		}
		return nil, apperrors.WrapInternalf(err, "get web3 identity mapping by credential_id %s", credentialID)
	}
	return m, nil
}

func (r Repository) GetByWalletAddress(ctx context.Context, walletAddress string) (*domainweb3identity.Mapping, error) {
	row := r.db.QueryRowContext(ctx, queryByWalletAddress, walletAddress)
	m, err := scanMapping(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainweb3identity.ErrNotFound
		}
		return nil, apperrors.WrapInternalf(err, "get web3 identity mapping by wallet_address %s", walletAddress)
	}
	return m, nil
}

func (r Repository) TouchLastUsedAt(ctx context.Context, id string, now time.Time) error {
	ct, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	result, err := r.db.ExecContext(ct, queryTouchLastUsed, now, id)
	if err != nil {
		return apperrors.WrapInternalf(err, "touch last_used_at for mapping %s", id)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return domainweb3identity.ErrNotFound
	}
	return nil
}
