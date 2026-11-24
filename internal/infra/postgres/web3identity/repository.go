// Package web3identity is the PostgreSQL implementation of domain/web3identity.Repository.
package web3identity

import (
	"context"
	"errors"
	"time"

	domainweb3identity "github.com/guliangiam/guliangiam/internal/domain/web3identity"
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

var _ domainweb3identity.Repository = Repository{}

func scanMapping(scanner interface{ Scan(...any) error }) (*domainweb3identity.Mapping, error) {
	var m domainweb3identity.Mapping
	err := scanner.Scan(
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
	_, err := r.db.Exec(ctx, queryCreate,
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
	row := r.db.QueryRow(ctx, queryByCredentialID, credentialID)
	m, err := scanMapping(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainweb3identity.ErrNotFound
		}
		return nil, apperrors.WrapInternalf(err, "get web3 identity mapping by credential_id %s", credentialID)
	}
	return m, nil
}

func (r Repository) GetByWalletAddress(ctx context.Context, walletAddress string) (*domainweb3identity.Mapping, error) {
	row := r.db.QueryRow(ctx, queryByWalletAddress, walletAddress)
	m, err := scanMapping(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainweb3identity.ErrNotFound
		}
		return nil, apperrors.WrapInternalf(err, "get web3 identity mapping by wallet_address %s", walletAddress)
	}
	return m, nil
}

func (r Repository) TouchLastUsedAt(ctx context.Context, id string, now time.Time) error {
	ct, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	tag, err := r.db.Exec(ct, queryTouchLastUsed, id, now)
	if err != nil {
		return apperrors.WrapInternalf(err, "touch last_used_at for mapping %s", id)
	}
	if tag.RowsAffected() == 0 {
		return domainweb3identity.ErrNotFound
	}
	return nil
}
