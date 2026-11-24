package captchaproviders

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
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

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.CaptchaProvider, error) {
	var row captchaProviderRow
	err := r.db.QueryRow(ctx, queryGetCaptchaProviderByID, id, organizationID).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.SiteKey, &row.SecretKey, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("captcha provider not found")
		}
		return nil, err
	}

	provider, err := toCaptchaProvider(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode captcha provider")
	}
	return provider, nil
}

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.CaptchaProvider, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListCaptchaProviders, query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.CaptchaProvider, 0)
	for rows.Next() {
		var row captchaProviderRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.ProviderType, &row.SiteKey, &row.SecretKey, &row.Config, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision); err != nil {
			return nil, 0, err
		}
		provider, err := toCaptchaProvider(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode captcha provider")
		}
		items = append(items, provider)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountCaptchaProviders, query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, provider *domain.CaptchaProvider) error {
	config, err := shared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}

	secret, err := captchaSecretForWrite(provider)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryInsertCaptchaProvider, provider.ID, provider.OrganizationID, provider.Name, provider.ProviderType, provider.SiteKey, secret, config, provider.IsEnabled, provider.CreatedAt, provider.UpdatedAt, provider.Revision)
	return err
}

// Update 以实体 OrganizationID 作为额外谓词；app 层已 Revision++，因此 WHERE
// 使用 Revision-1 做乐观锁守卫。返回 ErrNoRows 表示 revision 不匹配（跨租户
// 在 app 层的 scoped GetByID 已先行返回 NotFound，正常不会走到这里）。
func (r Repository) Update(ctx context.Context, provider *domain.CaptchaProvider) error {
	config, err := shared.MarshalJSON(provider.Config)
	if err != nil {
		return err
	}

	secret, err := captchaSecretForWrite(provider)
	if err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateCaptchaProvider, provider.Name, provider.ProviderType, provider.SiteKey, secret, config, provider.IsEnabled, provider.UpdatedAt, provider.Revision-1, provider.ID, provider.OrganizationID).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("captcha provider revision conflict")
		}
		return err
	}
	provider.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteCaptchaProvider, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("captcha provider not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
