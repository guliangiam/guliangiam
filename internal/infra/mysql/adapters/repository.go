package adapters

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/adapters"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 adapters 存储层。
// 与 internal/infra/postgres/adapters.Repository 实现同域接口。
//
// 关键 PG→MySQL 差异处理：
//  1. guliangiam.resolve_org_id() → 直接传 organization_id
//  2. ILIKE → LIKE + CONCAT('%', ?, '%')
//  3. RETURNING revision → UPDATE 后 SELECT revision（两步）
//  4. pgx.ErrNoRows → sql.ErrNoRows
//  5. QueryRow / Query / Exec → QueryRowContext / QueryContext / ExecContext
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

// scanAdapterRow 扫描 adapters 单表查询，24 列严格匹配 models.go adapterRow 顺序。
func scanAdapterRow(s scannable) (adapterRow, error) {
	var row adapterRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.Type, &row.Server, &row.Database, &row.TableName, &row.Username, &row.Password,
		&row.Owner, &row.UseSameDb, &row.DatabaseType, &row.Host, &row.Port,
		&row.DbUser, &row.DbPassword, &row.DatabaseName, &row.Policy, &row.IsEnabled,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

// ── domain.Repository 实现 ────────────────────────────────────────

// GetByID 按 id + organization_id 取 adapter；跨租户按不存在处理。
func (r Repository) GetByID(ctx context.Context, id, organizationID string) (*domain.Adapter, error) {
	row, err := scanAdapterRow(r.db.QueryRowContext(ctx, queryGetAdapterByID, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("adapter not found")
		}
		return nil, err
	}
	return rowToDomain(row)
}

// List 分页列出 adapters。
// SQL 占位符展开：
//
//	organization_id × 2, keyword × 3, is_enabled × 2, database_type × 2, limit, offset
//	共 11 个；Count 同构无 limit/offset = 9 个。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Adapter, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListAdapters,
		query.OrganizationID, query.OrganizationID, // orgID × 2
		query.Keyword, query.Keyword, query.Keyword, // keyword × 3
		isEnabled, isEnabled, // is_enabled × 2
		query.DatabaseType, query.DatabaseType, // database_type × 2
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Adapter, 0)
	for rows.Next() {
		row, err := scanAdapterRow(rows)
		if err != nil {
			return nil, 0, err
		}
		a, err := rowToDomain(row)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, a)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count: orgID(×2), keyword(×3), is_enabled(×2), database_type(×2) = 9 个
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountAdapters,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		query.DatabaseType, query.DatabaseType,
	).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// Create 插入 adapter。
func (r Repository) Create(ctx context.Context, a *domain.Adapter) error {
	password, dbPassword, err := adapterSecretsForWrite(a)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertAdapter,
		a.ID, a.OrganizationID, a.Name, a.DisplayName, a.Description,
		a.Type, a.Server, a.Database, a.TableName, a.Username, password,
		a.Owner, a.UseSameDb, a.DatabaseType, a.Host, a.Port,
		a.DbUser, dbPassword, a.DatabaseName, a.Policy, a.IsEnabled,
		a.CreatedAt, a.UpdatedAt, a.Revision)
	return err
}

// Update 乐观锁更新。
// PG 版用 RETURNING revision 一步到位；MySQL 版拆成两步：
//  1. UPDATE ... WHERE id = ? AND revision = ? AND organization_id = ? → 判断影响行数
//  2. SELECT revision FROM ... WHERE id = ? AND organization_id = ? → 回读新 revision
//
// 0 行按 revision 冲突返回：跨租户请求在 app 层 scoped GetByID 已被拒绝，
// 到不了这里，因此不存在“跨租户 404 vs 同租户 409”的探测差异。
func (r Repository) Update(ctx context.Context, a *domain.Adapter) error {
	password, dbPassword, err := adapterSecretsForWrite(a)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx, queryUpdateAdapter,
		a.DisplayName, a.Description, a.Type, a.Server, a.Database, a.TableName,
		a.Username, password,
		a.Owner, a.UseSameDb, a.DatabaseType, a.Host, a.Port,
		a.DbUser, dbPassword, a.DatabaseName, a.Policy, a.IsEnabled,
		a.UpdatedAt, a.ID, a.Revision, a.OrganizationID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("adapter revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, a.ID, a.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("adapter not found")
		}
		return err
	}
	a.Revision = newRevision
	return nil
}

// Delete 按 id + organization_id 删除 adapter；跨租户按不存在处理。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteAdapter, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("adapter not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
