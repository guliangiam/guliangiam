package b2b

import (
	"context"
	"database/sql"
	"strings"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/b2b"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 实现 domain/b2b.Repository 端口的 MySQL 8.0+ 存储层。
//
// 与 PostgreSQL 版本语义一致：
//   - Create 依赖 uq_org_domain 唯一约束拒绝重复认领
//   - Update 只作用于 pending 行 —— 终态（verified / failed）不可逆由 SQL 兜底
//   - FindVerifiedByDomain 是跨租户查询，不携带 organization_id 过滤
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx，使仓储可在 UoW 事务内复用。
func NewRepository(db mysqlshared.DBTX) Repository { return Repository{db: db} }

// scanRow 把一行 organization_domains 映射为聚合根。
func scanRow(scan func(dest ...any) error) (*domain.OrganizationDomain, error) {
	var d domain.OrganizationDomain
	var verifiedAt sql.NullTime
	var lastError sql.NullString
	var lastCheckedAt sql.NullTime
	if err := scan(
		&d.ID, &d.OrganizationID, &d.Domain, &d.VerificationToken,
		&d.Status, &verifiedAt, &d.CreatedAt, &d.UpdatedAt,
		&d.Attempts, &lastError, &d.NextRetryAt, &lastCheckedAt,
	); err != nil {
		return nil, err
	}
	if verifiedAt.Valid {
		d.VerifiedAt = verifiedAt.Time
	}
	if lastError.Valid {
		d.LastError = lastError.String
	}
	if lastCheckedAt.Valid {
		d.LastCheckedAt = lastCheckedAt.Time
	}
	return &d, nil
}

// Create 插入域名认领；重复认领由 uq_org_domain 拒绝。
func (r Repository) Create(ctx context.Context, d *domain.OrganizationDomain) error {
	var verifiedAt any
	if !d.VerifiedAt.IsZero() {
		verifiedAt = d.VerifiedAt
	}
	_, err := r.db.ExecContext(ctx, queryInsertOrganizationDomain,
		d.ID, d.OrganizationID, d.Domain, d.VerificationToken,
		d.Status, verifiedAt, d.CreatedAt, d.UpdatedAt,
	)
	return err
}

// GetByID 按主键查；未命中返回 NotFound。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.OrganizationDomain, error) {
	d, err := scanRow(r.db.QueryRowContext(ctx, queryGetOrganizationDomainByID, id).Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("organization_domain not found")
		}
		return nil, err
	}
	return d, nil
}

// GetByIDForOrganization 按 id + organization_id 查；跨租户按不存在处理。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.OrganizationDomain, error) {
	d, err := scanRow(r.db.QueryRowContext(ctx, queryGetOrganizationDomainByIDForOrganization, id, organizationID).Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("organization_domain not found")
		}
		return nil, err
	}
	return d, nil
}

// GetByDomain 按 (organization_id, domain) 查 —— 创建前查重用；未命中返回 nil。
func (r Repository) GetByDomain(ctx context.Context, organizationID, domainName string) (*domain.OrganizationDomain, error) {
	d, err := scanRow(r.db.QueryRowContext(ctx, queryGetOrganizationDomainByName, organizationID, domainName).Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return d, nil
}

// FindVerifiedByDomain 邮箱域 SSO 发现入口；跨租户匹配多条视为歧义。
func (r Repository) FindVerifiedByDomain(ctx context.Context, domainName string) (*domain.OrganizationDomain, error) {
	rows, err := r.db.QueryContext(ctx, queryFindVerifiedOrganizationDomain, domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found *domain.OrganizationDomain
	for rows.Next() {
		if found != nil {
			return nil, apperrors.NewConflict("multiple verified organization domains match this domain")
		}
		if found, err = scanRow(rows.Scan); err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return found, nil
}

// List 按组织分页列出，status 为空表示不过滤。
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.OrganizationDomain, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountOrganizationDomains,
		q.OrganizationID, q.Status, q.Status,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := mysqlshared.ClampPageSize(q.PageSize)
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.QueryContext(ctx, queryListOrganizationDomains,
		q.OrganizationID, q.Status, q.Status, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.OrganizationDomain, 0)
	for rows.Next() {
		d, err := scanRow(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ClaimPending 原子认领到期行并加租约。
// MySQL 无 RETURNING：先 SELECT ids FOR UPDATE SKIP LOCKED，再 UPDATE 租约，
// 最后 SELECT 全量行返回（与 mysql/outbox 的 LockBatch 同构）。
// 调用方必须在事务中执行，否则 FOR UPDATE 锁会被立即释放。
func (r Repository) ClaimPending(ctx context.Context, limit int, lease time.Duration) ([]*domain.OrganizationDomain, error) {
	pickRows, err := r.db.QueryContext(ctx, queryClaimPendingIDs, limit)
	if err != nil {
		return nil, err
	}
	var pickedIDs []string
	for pickRows.Next() {
		var id string
		if err := pickRows.Scan(&id); err != nil {
			pickRows.Close()
			return nil, err
		}
		pickedIDs = append(pickedIDs, id)
	}
	pickRows.Close()
	if err := pickRows.Err(); err != nil {
		return nil, err
	}
	if len(pickedIDs) == 0 {
		return nil, nil
	}

	seconds := int(lease.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	inClause := buildIDINClause(pickedIDs)

	updateArgs := make([]any, 0, len(pickedIDs)+1)
	updateArgs = append(updateArgs, seconds)
	for _, id := range pickedIDs {
		updateArgs = append(updateArgs, id)
	}
	if _, err := r.db.ExecContext(ctx, strings.Replace(queryClaimPendingLease, "__PICKED_IDS__", inClause, 1), updateArgs...); err != nil {
		return nil, err
	}

	selectArgs := make([]any, 0, len(pickedIDs))
	for _, id := range pickedIDs {
		selectArgs = append(selectArgs, id)
	}
	rows, err := r.db.QueryContext(ctx, strings.Replace(queryClaimPendingSelect, "__PICKED_IDS__", inClause, 1), selectArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.OrganizationDomain, 0, len(pickedIDs))
	for rows.Next() {
		d, err := scanRow(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// Update 持久化聚合；仅 pending 行可更新，否则视为状态冲突。
func (r Repository) Update(ctx context.Context, d *domain.OrganizationDomain) error {
	var verifiedAt any
	if !d.VerifiedAt.IsZero() {
		verifiedAt = d.VerifiedAt
	}
	var nextRetryAt any
	if !d.NextRetryAt.IsZero() {
		nextRetryAt = d.NextRetryAt
	}
	var lastCheckedAt any
	if !d.LastCheckedAt.IsZero() {
		lastCheckedAt = d.LastCheckedAt
	}
	res, err := r.db.ExecContext(ctx, queryUpdatePendingOrganizationDomain,
		d.Domain, d.VerificationToken, d.Status, verifiedAt, d.UpdatedAt,
		d.Attempts, d.LastError, nextRetryAt, lastCheckedAt,
		d.ID,
	)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return apperrors.NewConflict("organization domain is no longer pending")
	}
	return nil
}

// Delete 按 id + organization_id 删除域名认领；跨租户按不存在处理。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteOrganizationDomain, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("organization_domain not found")
	}
	return nil
}

// 编译期接口检查 —— Repository 必须实现 domain.Repository。
var _ domain.Repository = Repository{}
