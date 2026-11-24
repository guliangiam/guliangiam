package faceidproviders

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
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

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.FaceIdProvider, error) {
	var row faceIdProviderRow
	err := r.db.QueryRow(ctx, queryGetFaceIdProviderByID, id, organizationID).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.AccessKeyID, &row.AccessKeySecret, &row.Endpoint, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("face id provider not found")
		}
		return nil, err
	}

	provider, err := toFaceIdProvider(row)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "decode face id provider")
	}
	return provider, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.FaceIdProvider, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListFaceIdProviders, query.OrganizationID, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.FaceIdProvider, 0)
	for rows.Next() {
		var row faceIdProviderRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.AccessKeyID, &row.AccessKeySecret, &row.Endpoint, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision); err != nil {
			return nil, 0, err
		}
		provider, err := toFaceIdProvider(row)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "decode face id provider")
		}
		items = append(items, provider)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountFaceIdProviders, query.OrganizationID, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, provider *domain.FaceIdProvider) error {
	config, err := shared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}
	accessKeySecret, err := faceIdAccessKeySecretForWrite(provider)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryInsertFaceIdProvider, provider.ID, provider.OrganizationID, provider.Name, provider.ProviderType, provider.AccessKeyID, accessKeySecret, provider.Endpoint, config, provider.IsEnabled, provider.CreatedAt, provider.UpdatedAt, provider.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, provider *domain.FaceIdProvider) error {
	config, err := shared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}
	accessKeySecret, err := faceIdAccessKeySecretForWrite(provider)
	if err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateFaceIdProvider, provider.Name, provider.ProviderType, provider.AccessKeyID, accessKeySecret, provider.Endpoint, config, provider.IsEnabled, provider.UpdatedAt, provider.Revision-1, provider.ID, provider.OrganizationID).Scan(&newRevision)
	if err != nil {
		// 实体已在 app 层按租户读取：ErrNoRows 只代表 revision 冲突。
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("face id provider revision conflict")
		}
		return err
	}
	provider.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteFaceIdProvider, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("face id provider not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
