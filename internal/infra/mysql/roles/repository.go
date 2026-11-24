package roles

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.4+ 实现的 roles 存储层。
// 与 internal/infra/postgres/roles.Repository 实现同域接口。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// rowsAffected 从 sql.Result 安全提取影响行数，替代 PG 的 commandTag.RowsAffected()。
func rowsAffected(r sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

// scannable 是 sql.Row 和 sql.Rows 的公共接口。
type scannable interface {
	Scan(dest ...any) error
}

// scanRoleRow 扫描 roles 单表查询。
// 列顺序必须与 sql.go 各 SELECT 严格一致（13 列）。
func scanRoleRow(s scannable) (roleRow, error) {
	var row roleRow
	err := s.Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description,
		&row.Users, &row.GroupIDs, &row.RoleIDs, &row.Domains,
		&row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	return row, err
}

// ── domain.Repository 实现 ────────────────────────────────────────

// GetByIDForOrganization 按主键 + 租户取 role；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Role, error) {
	row, err := scanRoleRow(r.db.QueryRowContext(ctx, queryGetRoleByIDForOrganization, id, organizationID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("role not found")
		}
		return nil, err
	}

	role, err := toRole(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode role")
	}
	return role, nil
}

// List 分页列出 roles。
// SQL 占位符展开：
//
//	organization_id 出现 2 次
//	keyword 出现 4 次
//	is_enabled 出现 2 次
//
// 共 8 个过滤参数 + limit + offset = 10 个。
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Role, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	// 把 *bool 解包为传给 SQL 的值：nil 时传 nil（对应 (? IS NULL)），非 nil 时传 *p。
	var isEnabled any
	if query.IsEnabled != nil {
		isEnabled = *query.IsEnabled
	}

	rows, err := r.db.QueryContext(ctx, queryListRoles,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Role, 0)
	for rows.Next() {
		row, err := scanRoleRow(rows)
		if err != nil {
			return nil, 0, err
		}
		role, err := toRole(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode role")
		}
		items = append(items, role)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count: organization_id(×2), keyword(×4), is_enabled(×2) = 8 个
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountRoles,
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword, query.Keyword,
		isEnabled, isEnabled,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Create 插入 role。
func (r Repository) Create(ctx context.Context, role *domain.Role) error {
	users, err := mysqlshared.MarshalJSON(role.Users)
	if err != nil {
		return err
	}
	groupIDs, err := mysqlshared.MarshalJSON(role.GroupIDs)
	if err != nil {
		return err
	}
	roleIDs, err := mysqlshared.MarshalJSON(role.RoleIDs)
	if err != nil {
		return err
	}
	domains, err := mysqlshared.MarshalJSON(role.Domains)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryInsertRole,
		role.ID, role.OrganizationID, role.Name, role.DisplayName, role.Description,
		users, groupIDs, roleIDs, domains,
		role.IsEnabled, role.CreatedAt, role.UpdatedAt, role.Revision,
	)
	return err
}

// Update 乐观锁 + 租户谓词更新。
// PG 版用 RETURNING revision 一步到位；MySQL 版拆成两步：
//  1. UPDATE ... WHERE id = ? AND revision = ? AND organization_id = ? → 判断影响行数
//  2. SELECT revision FROM ... WHERE id = ? AND organization_id = ? → 回读新 revision
func (r Repository) Update(ctx context.Context, role *domain.Role) error {
	users, err := mysqlshared.MarshalJSON(role.Users)
	if err != nil {
		return err
	}
	groupIDs, err := mysqlshared.MarshalJSON(role.GroupIDs)
	if err != nil {
		return err
	}
	roleIDs, err := mysqlshared.MarshalJSON(role.RoleIDs)
	if err != nil {
		return err
	}
	domains, err := mysqlshared.MarshalJSON(role.Domains)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdateRole,
		role.DisplayName, role.Description,
		users, groupIDs, roleIDs, domains,
		role.IsEnabled, role.UpdatedAt,
		role.ID, role.Revision-1, role.OrganizationID,
	)
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("role revision conflict")
	}

	// 回读新 revision
	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, role.ID, role.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("role not found")
		}
		return err
	}
	role.Revision = newRevision
	return nil
}

// Delete 按 id + organization_id 删除 role；跨租户按不存在处理。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeleteRole, id, organizationID)
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("role not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
