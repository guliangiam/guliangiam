package b2b

import (
	"context"
	stderrors "errors"
	"strconv"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/b2b"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repository 实现 domain/b2b.Repository 端口接口。
//
// 所有方法遵循：
//   - 携带 shared.DBTX 以支持事务内调用（UoW B2BUoW 接口的 ctx）
//   - Create 依赖 DB 层 uq_org_domain 唯一约束拒绝重复认领
//   - Update 不做业务状态校验 —— 调用方（app/b2b/service.go）必须先调用
//     RecordVerificationSuccess / RecordVerificationFailure（或 MarkVerified /
//     MarkFailed）再传过来
//   - ClaimPending 用单语句 CTE + FOR UPDATE SKIP LOCKED 原子认领并加租约
type Repository struct{ db shared.DBTX }

func NewRepository(db shared.DBTX) Repository { return Repository{db: db} }

// ── scanOne ─────────────────────────────────────────────────────────────────

func scanOne(s scannable) (*domain.OrganizationDomain, error) {
	var d domain.OrganizationDomain
	var verifiedAt *time.Time
	var lastError *string
	var lastCheckedAt *time.Time
	err := s.Scan(
		&d.ID,
		&d.OrganizationID,
		&d.Domain,
		&d.VerificationToken,
		&d.Status,
		&verifiedAt,
		&d.CreatedAt,
		&d.UpdatedAt,
		&d.Attempts,
		&lastError,
		&d.NextRetryAt,
		&lastCheckedAt,
	)
	if err != nil {
		return nil, err
	}
	if verifiedAt != nil {
		d.VerifiedAt = *verifiedAt
	}
	if lastError != nil {
		d.LastError = *lastError
	}
	if lastCheckedAt != nil {
		d.LastCheckedAt = *lastCheckedAt
	}
	return &d, nil
}

// ── Domain.Repository 接口 ──────────────────────────────────────────────────

func (r Repository) Create(ctx context.Context, d *domain.OrganizationDomain) error {
	var verifiedAt any
	if !d.VerifiedAt.IsZero() {
		verifiedAt = d.VerifiedAt
	}
	_, err := r.db.Exec(ctx, QueryInsert,
		d.ID, d.OrganizationID, d.Domain, d.VerificationToken,
		d.Status, verifiedAt, d.CreatedAt, d.UpdatedAt,
	)
	var pgErr *pgconn.PgError
	if stderrors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apperrors.NewAlreadyExistsf(
			"domain %q already claimed by organization %s", d.Domain, d.OrganizationID,
		)
	}
	return err
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.OrganizationDomain, error) {
	d, err := scanOne(r.db.QueryRow(ctx, QueryGetByID, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("organization_domain not found")
		}
		return nil, err
	}
	return d, nil
}

// GetByIDForOrganization 只在域名属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.OrganizationDomain, error) {
	d, err := scanOne(r.db.QueryRow(ctx, QueryGetByIDForOrganization, id, organizationID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("organization_domain not found")
		}
		return nil, err
	}
	return d, nil
}

// GetByDomain 按 (organization_id, domain) 查 —— 创建前查重用。
func (r Repository) GetByDomain(ctx context.Context, organizationID, domainName string) (*domain.OrganizationDomain, error) {
	d, err := scanOne(r.db.QueryRow(ctx, QueryGetByDomain, organizationID, domainName))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // 允许返回 nil 表示未认领
		}
		return nil, err
	}
	return d, nil
}

// FindVerifiedByDomain 邮箱域 SSO 发现入口。
// 跨租户查询 —— 不带 organization_id 过滤（任何组织只要认领了该域名就返回）。
func (r Repository) FindVerifiedByDomain(ctx context.Context, domainName string) (*domain.OrganizationDomain, error) {
	rows, err := r.db.Query(ctx, QueryFindVerifiedByDomain, domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found *domain.OrganizationDomain
	for rows.Next() {
		if found != nil {
			return nil, apperrors.NewConflict("multiple verified organization domains match this domain")
		}
		found, err = scanOne(rows)
		if err != nil {
			return nil, err
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return found, nil
}

func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.OrganizationDomain, int64, error) {
	offset := shared.PageOffset(q.Page, q.PageSize)
	rows, err := r.db.Query(ctx, QueryList,
		q.OrganizationID, q.Status,
		shared.ClampPageSize(q.PageSize), offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.OrganizationDomain, 0)
	for rows.Next() {
		d, err := scanOne(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.db.QueryRow(ctx, QueryListCount, q.OrganizationID, q.Status).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// ClaimPending 单语句原子认领到期行并加租约。
// FOR UPDATE SKIP LOCKED 保证多实例不会领取同一批 pending 行；
// 租约由 next_retry_at 承载，实例崩溃后到期自动恢复可认领。
func (r Repository) ClaimPending(ctx context.Context, limit int, lease time.Duration) ([]*domain.OrganizationDomain, error) {
	rows, err := r.db.Query(ctx, QueryClaimPending, limit, leaseInterval(lease))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*domain.OrganizationDomain, 0, limit)
	for rows.Next() {
		d, err := scanOne(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, d)
	}
	return items, rows.Err()
}

// leaseInterval 把租约转成 PostgreSQL interval 文本。
// 小于 1 秒按 1 秒处理，避免 next_retry_at 原地不动导致租约失效。
func leaseInterval(lease time.Duration) string {
	seconds := int(lease.Round(time.Second) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds) + " seconds"
}

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
	tag, err := r.db.Exec(ctx, QueryUpdate,
		d.Domain, d.VerificationToken, d.Status, verifiedAt, d.UpdatedAt,
		d.Attempts, d.LastError, nextRetryAt, lastCheckedAt,
		d.ID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return apperrors.NewConflict("organization domain is no longer pending")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, QueryDelete, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("organization_domain not found")
	}
	return nil
}

// scannable 是 pgx Rows / pgx.Row 的公共 Scan 接口。
type scannable interface {
	Scan(dest ...interface{}) error
}

// 编译期接口检查 —— Repository 必须实现 domain.Repository。
var _ domain.Repository = Repository{}
