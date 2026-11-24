package pricings

import (
	"context"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/pricings"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Pricing, error) {
	p, err := scanPricing(r.db.QueryRow(ctx, QueryGetByID, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("pricing not found")
		}
		return nil, err
	}
	return p, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Pricing, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.IsEnabled, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Pricing, 0)
	for rows.Next() {
		p, err := scanPricing(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, p *domain.Pricing) error {
	plans, _ := json.Marshal(p.Plans)
	_, err := r.db.Exec(ctx, QueryInsert, p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Description, plans, p.IsEnabled, p.TrialDuration, p.Application, p.CreatedAt, p.UpdatedAt, p.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, p *domain.Pricing) error {
	plans, _ := json.Marshal(p.Plans)
	tag, err := r.db.Exec(ctx, QueryUpdate, p.DisplayName, p.Description, plans, p.IsEnabled, p.TrialDuration, p.Application, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// 实体已在 app 层按租户读取：0 行只可能是同一租户内的行已消失，
		// 跨租户 id 在读取阶段就已 NotFound。
		return apperrors.NewNotFound("pricing not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("pricing not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
