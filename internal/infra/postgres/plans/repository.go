package plans

import (
	"context"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/plans"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Plan, error) {
	p, err := scanPlan(r.db.QueryRow(ctx, QueryGetByID, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("plan not found")
		}
		return nil, err
	}
	return p, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Plan, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.State, q.Period, q.IsExclusive, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Plan, 0)
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.State, q.Period, q.IsExclusive).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, p *domain.Plan) error {
	providers, _ := json.Marshal(p.PaymentProviders)
	_, err := r.db.Exec(ctx, QueryInsert, p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Description, p.Price, p.Currency, p.Period, p.Product, providers, p.State, p.IsExclusive, p.Role, p.CreatedAt, p.UpdatedAt, p.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, p *domain.Plan) error {
	providers, _ := json.Marshal(p.PaymentProviders)
	tag, err := r.db.Exec(ctx, QueryUpdate, p.DisplayName, p.Description, p.Price, p.Currency, p.Period, p.Product, providers, p.State, p.IsExclusive, p.Role, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// 实体已在 app 层按租户读取：0 行只可能是同一租户内的行已消失，
		// 跨租户 id 在读取阶段就已 NotFound。
		return apperrors.NewNotFound("plan not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("plan not found")
	}
	return nil
}

var _ domain.Repository = Repository{}
