package products

import (
	"context"
	"encoding/json"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/products"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	return r.getByIDQuery(ctx, QueryGetByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Product, error) {
	return r.getByIDQuery(ctx, QueryGetByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Product, error) {
	p, err := scanProduct(r.db.QueryRow(ctx, query, args...))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("product not found")
		}
		return nil, err
	}
	return p, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Product, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList, q.OrganizationID, q.Keyword, q.State, q.IsRecharge, shared.ClampPageSize(q.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]*domain.Product, 0)
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, p)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryCount, q.OrganizationID, q.Keyword, q.State, q.IsRecharge).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, p *domain.Product) error {
	rechargeOpts, _ := json.Marshal(p.RechargeOptions)
	providers, _ := json.Marshal(p.Providers)
	_, err := r.db.Exec(ctx, QueryInsert, p.ID, p.OrganizationID, p.Name, p.DisplayName, p.Image, p.Detail, p.Description, p.Tag, p.Currency, p.Price, p.Quantity, p.Sold, p.IsRecharge, rechargeOpts, p.DisableCustomRecharge, providers, p.SuccessUrl, p.State, p.CreatedAt, p.UpdatedAt, p.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, p *domain.Product) error {
	rechargeOpts, _ := json.Marshal(p.RechargeOptions)
	providers, _ := json.Marshal(p.Providers)
	tag, err := r.db.Exec(ctx, QueryUpdate, p.DisplayName, p.Image, p.Detail, p.Description, p.Tag, p.Currency, p.Price, p.Quantity, p.Sold, p.IsRecharge, rechargeOpts, p.DisableCustomRecharge, providers, p.SuccessUrl, p.State, p.UpdatedAt, p.Revision, p.ID, p.OrganizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// 实体已在 app 层按租户读取：0 行只可能是同一租户内的行已消失，
		// 跨租户 id 在读取阶段就已 NotFound。
		return apperrors.NewNotFound("product not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("product not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

// IncrementSold 原子消费库存（BE-C-11）。守卫在 SQL 内：sold+q<=quantity
// 且 status='active'，0 行命中时回读区分「不存在/跨租户」（NotFound）、
// 「不可售」与「库存不足」（422），绝不退化为读改写。
func (r Repository) IncrementSold(ctx context.Context, id, organizationID string, quantity int, now time.Time) (*domain.Product, error) {
	p, err := scanProduct(r.db.QueryRow(ctx, QueryIncrementSold, id, organizationID, quantity, now))
	if err == nil {
		return p, nil
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}
	current, readErr := r.GetByIDForOrganization(ctx, id, organizationID)
	if readErr != nil {
		return nil, readErr
	}
	if current.State != domain.StateActive {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "product is not available for purchase", map[string]any{
			"field": "state", "value": string(current.State), "expected": string(domain.StateActive),
		})
	}
	return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "insufficient product quantity", map[string]any{
		"field": "quantity", "value": quantity, "max": current.Quantity - current.Sold,
	})
}

// ReleaseSold 归还未支付订单预留的库存（BE-C-11）：sold 下限为 0，
// 跨租户/不存在的 id 与 NotFound 不可区分。
func (r Repository) ReleaseSold(ctx context.Context, id, organizationID string, quantity int, now time.Time) (*domain.Product, error) {
	p, err := scanProduct(r.db.QueryRow(ctx, QueryReleaseSold, id, organizationID, quantity, now))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("product not found")
		}
		return nil, err
	}
	return p, nil
}
