package notificationproviders

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
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

func (r Repository) GetByID(ctx context.Context, id string) (*domain.NotificationProvider, error) {
	return r.getByID(ctx, queryGetNotificationProviderByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.NotificationProvider, error) {
	return r.getByID(ctx, queryGetNotificationProviderByIDForOrganization, id, organizationID)
}

func (r Repository) getByID(ctx context.Context, query string, args ...any) (*domain.NotificationProvider, error) {
	var row notificationProviderRow
	err := r.db.QueryRow(ctx, query, args...).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.WebhookURL, &row.AccessToken, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("notification provider not found")
		}
		return nil, err
	}

	provider, err := toNotificationProvider(row)
	if err != nil {
		return nil, err
	}
	return provider, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.NotificationProvider, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListNotificationProviders, query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.NotificationProvider, 0)
	for rows.Next() {
		var row notificationProviderRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.WebhookURL, &row.AccessToken, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision); err != nil {
			return nil, 0, err
		}
		provider, err := toNotificationProvider(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, provider)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountNotificationProviders, query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, provider *domain.NotificationProvider) error {
	config, err := shared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}

	accessToken, err := sealAccessToken(provider)
	if err != nil {
		return err
	}

	// provider_id 指向通用 providers(id)，控制台未提供该字段，写 NULL 即可；
	// 之前误传了 notification provider 自身的 ID，触发 fk_..._provider_id 外键错误。
	_, err = r.db.Exec(ctx, queryInsertNotificationProvider, provider.ID, provider.OrganizationID, nil, provider.Name, provider.ProviderType, provider.WebhookURL, accessToken, config, provider.IsEnabled, provider.CreatedAt, provider.UpdatedAt, provider.Revision)
	return err
}

// Update 以实体 OrganizationID 作为额外谓词；app 层已 Revision++，因此 WHERE
// 使用 Revision-1 做乐观锁守卫。返回 ErrNoRows 表示 revision 不匹配（跨租户
// 在 app 层的 scoped GetByID 已先行返回 NotFound，正常不会走到这里）。
func (r Repository) Update(ctx context.Context, provider *domain.NotificationProvider) error {
	config, err := shared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}

	accessToken, err := sealAccessToken(provider)
	if err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateNotificationProvider, provider.Name, provider.ProviderType, provider.WebhookURL, accessToken, config, provider.IsEnabled, provider.UpdatedAt, provider.Revision-1, provider.ID, provider.OrganizationID).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("notification provider revision conflict")
		}
		return err
	}
	provider.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteNotificationProvider, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("notification provider not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
