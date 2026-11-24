package keys

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/keys"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Key, error) {
	k, err := scanKey(r.db.QueryRow(ctx, QueryGetByID, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("key not found")
		}
		return nil, err
	}
	return k, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Key, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.KeyType, q.IsEnabled, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Key, 0)
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, k)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.KeyType, q.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, k *domain.Key) error {
	privateKey, err := keyPrivateKeyForWrite(k)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, QueryInsert, k.ID, k.OrganizationID, k.Name, k.DisplayName, k.KeyType, k.PublicKey, privateKey, k.Algorithm, k.ExpiresAt, k.IsEnabled, k.CreatedAt, k.UpdatedAt, k.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, k *domain.Key) error {
	privateKey, err := keyPrivateKeyForWrite(k)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, QueryUpdate, k.DisplayName, k.PublicKey, privateKey, k.Algorithm, k.ExpiresAt, k.IsEnabled, k.UpdatedAt, k.Revision, k.ID, k.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("key not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("key not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
