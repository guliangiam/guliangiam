package radiusaccountings

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 radiusaccountings 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// GetByID 按 id 查 RadiusAccounting（organization_id 谓词防止跨租户读取）。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.RadiusAccounting, error) {
	var ra domain.RadiusAccounting
	err := r.db.QueryRowContext(ctx, queryGetRAByID, id, organizationID).Scan(
		&ra.ID, &ra.OrganizationID, &ra.Name, &ra.Username, &ra.ServiceType, &ra.NasID, &ra.NasIPAddr,
		&ra.NasPortID, &ra.NasPortType, &ra.NasPort, &ra.FramedIPAddr, &ra.FramedIPNetmask,
		&ra.AcctSessionID, &ra.AcctSessionTime, &ra.AcctInputTotal, &ra.AcctOutputTotal,
		&ra.AcctInputPackets, &ra.AcctOutputPackets, &ra.AcctTerminateCause, &ra.AcctStartTime,
		&ra.AcctStopTime, &ra.LastUpdate, &ra.CreatedAt, &ra.UpdatedAt, &ra.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("radius accounting not found")
		}
		return nil, err
	}
	return &ra, nil
}

// List 分页列出 RadiusAccounting（简版列表，只返回摘要列）。
// countArgs: org_id×2 + keyword×3 + username×2 = 7
// listArgs: 同上 + limit + offset = 9
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.RadiusAccounting, int64, error) {
	countArgs := []interface{}{
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword,
		q.Username, q.Username,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountRA, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)

	rows, err := r.db.QueryContext(ctx, queryListRA, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.RadiusAccounting, 0)
	for rows.Next() {
		var ra domain.RadiusAccounting
		if err := rows.Scan(&ra.ID, &ra.OrganizationID, &ra.Name, &ra.Username, &ra.AcctSessionID, &ra.AcctStartTime, &ra.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, &ra)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return items, total, nil
}

// Create 插入 RadiusAccounting。
func (r Repository) Create(ctx context.Context, ra *domain.RadiusAccounting) error {
	_, err := r.db.ExecContext(ctx, queryInsertRA,
		ra.ID, ra.OrganizationID, ra.Name, ra.Username, ra.ServiceType, ra.NasID, ra.NasIPAddr,
		ra.NasPortID, ra.NasPortType, ra.NasPort, ra.FramedIPAddr, ra.FramedIPNetmask,
		ra.AcctSessionID, ra.AcctSessionTime, ra.AcctInputTotal, ra.AcctOutputTotal,
		ra.AcctInputPackets, ra.AcctOutputPackets, ra.AcctTerminateCause, ra.AcctStartTime,
		ra.AcctStopTime, ra.LastUpdate, ra.CreatedAt, ra.UpdatedAt, ra.Revision,
	)
	return err
}

// Update 更新 RadiusAccounting 的计费字段（organization_id 谓词防止跨租户写入）。
func (r Repository) Update(ctx context.Context, ra *domain.RadiusAccounting) error {
	res, err := r.db.ExecContext(ctx, queryUpdateRA,
		ra.AcctSessionTime, ra.AcctInputTotal, ra.AcctOutputTotal, ra.AcctInputPackets,
		ra.AcctOutputPackets, ra.AcctTerminateCause, ra.AcctStopTime, ra.LastUpdate,
		ra.UpdatedAt, ra.Revision, ra.ID, ra.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("radius accounting not found")
	}
	return nil
}

// Delete 删除 RadiusAccounting（organization_id 谓词防止跨租户删除）。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteRA, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("radius accounting not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
