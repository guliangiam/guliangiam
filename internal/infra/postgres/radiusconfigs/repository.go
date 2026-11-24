package radiusconfigs

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
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

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.RadiusConfig, error) {
	var row radiusConfigRow
	err := r.db.QueryRow(ctx, queryGetRadiusConfigByID, id, organizationID).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Port, &row.SharedSecret, &row.AuthProtocol, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("radius config not found")
		}
		return nil, err
	}

	return toRadiusConfig(row)
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.RadiusConfig, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListRadiusConfigs, query.OrganizationID, query.IsEnabled, query.AuthProtocol, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.RadiusConfig, 0)
	for rows.Next() {
		var row radiusConfigRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.Host, &row.Port, &row.SharedSecret, &row.AuthProtocol, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision); err != nil {
			return nil, 0, err
		}
		config, err := toRadiusConfig(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, config)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountRadiusConfigs, query.OrganizationID, query.IsEnabled, query.AuthProtocol).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, config *domain.RadiusConfig) error {
	secret, err := radiusSecretForWrite(config)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertRadiusConfig, config.ID, config.OrganizationID, config.Name, config.Host, config.Port, secret, config.AuthProtocol, config.IsEnabled, config.CreatedAt, config.UpdatedAt, config.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, config *domain.RadiusConfig) error {
	secret, err := radiusSecretForWrite(config)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateRadiusConfig, config.Name, config.Host, config.Port, secret, config.AuthProtocol, config.IsEnabled, config.UpdatedAt, config.Revision-1, config.ID, config.OrganizationID).Scan(&newRevision)
	if err != nil {
		// 实体已在 app 层按租户读取：ErrNoRows 只代表 revision 冲突。
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("radius config revision conflict")
		}
		return err
	}
	config.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteRadiusConfig, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("radius config not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
