package iga

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// Repository 是 MySQL 8.4+ 实现的 iga 存储层。
// 与 internal/infra/postgres/iga.Repository 实现同域接口。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// ─── Campaigns ────────────────────────────────────────────────────

// ListCampaigns 分页列出 campaigns（动态拼接 status filter）。
func (r Repository) ListCampaigns(ctx context.Context, q domain.ListCampaignsQuery) ([]*domain.AccessCampaign, int64, error) {
	selectSQL, countSQL, args := buildListCampaignsSQL(q, false)

	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	pagedArgs := append(append([]any{}, args...), limit, offset)
	selectSQL += paginationSuffix

	rows, err := r.db.QueryContext(ctx, selectSQL, pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []*domain.AccessCampaign{}
	for rows.Next() {
		var c domain.AccessCampaign
		var statusStr string
		if err := rows.Scan(&c.ID, &c.OrganizationID, &c.Name, &statusStr,
			&c.StartDate, &c.EndDate, &c.CreatedAt, &c.UpdatedAt, &c.Revision); err != nil {
			return nil, 0, err
		}
		c.Status = domain.CampaignStatus(statusStr)
		items = append(items, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetCampaign 按 id + orgID 取 campaign。
func (r Repository) GetCampaign(ctx context.Context, id, orgID string) (*domain.AccessCampaign, error) {
	var c domain.AccessCampaign
	var statusStr string
	if err := r.db.QueryRowContext(ctx, qGetCampaign, id, orgID).Scan(
		&c.ID, &c.OrganizationID, &c.Name, &statusStr,
		&c.StartDate, &c.EndDate, &c.CreatedAt, &c.UpdatedAt, &c.Revision,
	); err != nil {
		return nil, err
	}
	c.Status = domain.CampaignStatus(statusStr)
	return &c, nil
}

// CreateCampaign 插入 campaign（status 固定为 'draft'）。
func (r Repository) CreateCampaign(ctx context.Context, c *domain.AccessCampaign) error {
	_, err := r.db.ExecContext(ctx, qInsertCampaign,
		c.ID, c.OrganizationID, c.Name, c.StartDate, c.EndDate,
	)
	return err
}

// UpdateCampaignStatus 更新 campaign 的 status。
func (r Repository) UpdateCampaignStatus(ctx context.Context, orgID, id string, status domain.CampaignStatus) error {
	_, err := r.db.ExecContext(ctx, qUpdateCampaignStatus, string(status), id, orgID)
	return err
}

// DeleteCampaign 删除指定 id + org 的 campaign。
func (r Repository) DeleteCampaign(ctx context.Context, id, orgID string) error {
	_, err := r.db.ExecContext(ctx, qDeleteCampaign, id, orgID)
	return err
}

// ─── SOD Rules ────────────────────────────────────────────────────

// ListSODRules 分页列出 SOD rules。
func (r Repository) ListSODRules(ctx context.Context, q domain.ListSODRulesQuery) ([]*domain.SODRule, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, qCountSODRules, q.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.QueryContext(ctx, qListSODRules, q.OrganizationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []*domain.SODRule{}
	for rows.Next() {
		var s domain.SODRule
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.RoleAID, &s.RoleBID,
			&s.CreatedAt, &s.UpdatedAt, &s.Revision); err != nil {
			return nil, 0, err
		}
		items = append(items, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// GetSODRule 按 id + orgID 取 SOD rule。
func (r Repository) GetSODRule(ctx context.Context, id, orgID string) (*domain.SODRule, error) {
	var s domain.SODRule
	if err := r.db.QueryRowContext(ctx, qGetSODRule, id, orgID).Scan(
		&s.ID, &s.OrganizationID, &s.Name, &s.RoleAID, &s.RoleBID,
		&s.CreatedAt, &s.UpdatedAt, &s.Revision,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateSODRule 插入 SOD rule。
func (r Repository) CreateSODRule(ctx context.Context, s *domain.SODRule) error {
	_, err := r.db.ExecContext(ctx, qInsertSODRule,
		s.ID, s.OrganizationID, s.Name, s.RoleAID, s.RoleBID,
	)
	return err
}

// UpdateSODRule 更新指定 SOD rule 的 name / roleA / roleB。
func (r Repository) UpdateSODRule(ctx context.Context, orgID, id, name, roleA, roleB string) error {
	_, err := r.db.ExecContext(ctx, qUpdateSODRule, name, roleA, roleB, id, orgID)
	return err
}

// DeleteSODRule 删除指定 id + org 的 SOD rule。
func (r Repository) DeleteSODRule(ctx context.Context, id, orgID string) error {
	_, err := r.db.ExecContext(ctx, qDeleteSODRule, id, orgID)
	return err
}

// ─── Lifecycle ────────────────────────────────────────────────────

// ListLifecycle 分页列出 lifecycle grants（动态拼接 user_id/role_id/status filter）。
func (r Repository) ListLifecycle(ctx context.Context, q domain.ListLifecycleQuery) ([]*domain.AccessLifecycle, int64, error) {
	selectSQL, countSQL, args := buildListLifecycleSQL(q, false)

	var total int64
	if err := r.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	pagedArgs := append(append([]any{}, args...), limit, offset)
	selectSQL += paginationSuffix

	rows, err := r.db.QueryContext(ctx, selectSQL, pagedArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []*domain.AccessLifecycle{}
	for rows.Next() {
		var l domain.AccessLifecycle
		var statusStr string
		if err := rows.Scan(&l.ID, &l.OrganizationID, &l.UserID, &l.RoleID,
			&l.GrantedAt, &l.ExpiresAt, &statusStr, &l.CreatedAt, &l.Revision); err != nil {
			return nil, 0, err
		}
		l.Status = domain.LifecycleStatus(statusStr)
		items = append(items, &l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CreateLifecycle 插入包含时间窗与声明范围的 lifecycle grant。
func (r Repository) CreateLifecycle(ctx context.Context, l *domain.AccessLifecycle) error {
	_, err := r.db.ExecContext(ctx, qInsertLifecycle,
		l.ID, l.OrganizationID, l.UserID, l.RoleID, nullableUUID(l.TicketID),
		nullableUUID(l.EnforcerID), l.Resource, l.Action, nullableUUID(l.PolicyID),
		l.GrantedAt, l.StartsAt, l.ExpiresAt, l.Status,
	)
	return err
}

// GetLifecycle 按 id + orgID 取 lifecycle grant。
func (r Repository) GetLifecycle(ctx context.Context, id, orgID string) (*domain.AccessLifecycle, error) {
	return scanLifecycle(r.db.QueryRowContext(ctx, qGetLifecycle, id, orgID))
}

// UpdateLifecycleStatus 更新 lifecycle grant 的 status（active→revoked/expired）。
func (r Repository) UpdateLifecycleStatus(ctx context.Context, orgID, id string, status domain.LifecycleStatus) error {
	_, err := r.db.ExecContext(ctx, qUpdateLifecycleStatus, string(status), id, orgID)
	return err
}

// RevokeLifecycle 撤销指定 lifecycle grant。
func (r Repository) RevokeLifecycle(ctx context.Context, id, orgID string) error {
	_, err := r.db.ExecContext(ctx, qRevokeLifecycle, id, orgID)
	return err
}

// ExpireLifecycle 批量过期所有 expires_at < before 的 active grants。
func (r Repository) ExpireLifecycle(ctx context.Context, orgID string, before time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, qExpireLifecycle, orgID, before)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// 编译期接口断言
var _ domain.Repository = Repository{}
