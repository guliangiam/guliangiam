package permissions

import (
	"context"
	"database/sql"

	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Repository 是 MySQL 8.0+ 实现的 permissions 存储层。
// 与 internal/infra/postgres/permissions.Repository 实现同域接口。
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

// scanPermissionRow 扫描单条 permissions 行（GetByID / List 共用）。
// 列顺序必须与 sql.go queryGetPermissionByID / queryListPermissions 的 SELECT 严格一致（25 列）。
func scanPermissionRow(s interface{ Scan(dest ...any) error }) (permissionRow, error) {
	var row permissionRow
	err := s.Scan(
		&row.ID,
		&row.OrganizationID,
		&row.Name,
		&row.DisplayName,
		&row.Description,
		&row.Users,
		&row.Groups,
		&row.Roles,
		&row.Domains,
		&row.ResourceType,
		&row.Resources,
		&row.Actions,
		&row.Effect,
		&row.IsEnabled,
		&row.Submitter,
		&row.Approver,
		&row.State,
		&row.Model,
		&row.Adapter,
		&row.Sorter,
		&row.Application,
		&row.ApproveTime,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.Revision,
	)
	return row, err
}

// ── domain.Repository 实现 ────────────────────────────────────────

// GetByID 按主键取 permission（系统/后台任务路径，不带请求租户）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Permission, error) {
	return r.getByIDQuery(ctx, queryGetPermissionByID, id)
}

// GetByIDForOrganization 只在 permission 属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Permission, error) {
	return r.getByIDQuery(ctx, queryGetPermissionByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Permission, error) {
	row, err := scanPermissionRow(r.db.QueryRowContext(ctx, query, args...))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("permission not found")
		}
		return nil, err
	}

	permission, err := toPermission(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode permission")
	}
	return permission, nil
}

// List 分页列出 permissions。
// SQL 参数绑定（占位符重复）：
//
//	org_id × 2, keyword × 4, is_enabled × 2, state × 2, resource_type × 2, effect × 2, limit, offset = 16
func (r Repository) List(ctx context.Context, q domain.ListQuery) ([]*domain.Permission, int64, error) {
	offset := mysqlshared.PageOffset(q.Page, q.PageSize)
	limit := mysqlshared.ClampPageSize(q.PageSize)

	rows, err := r.db.QueryContext(ctx, queryListPermissions,
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword, q.Keyword,
		q.IsEnabled, q.IsEnabled,
		q.State, q.State,
		q.ResourceType, q.ResourceType,
		q.Effect, q.Effect,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Permission, 0)
	for rows.Next() {
		row, err := scanPermissionRow(rows)
		if err != nil {
			return nil, 0, err
		}
		permission, err := toPermission(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode permission")
		}
		items = append(items, permission)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	// Count 参数：与 List 同构，去掉 limit/offset = 14
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountPermissions,
		q.OrganizationID, q.OrganizationID,
		q.Keyword, q.Keyword, q.Keyword, q.Keyword,
		q.IsEnabled, q.IsEnabled,
		q.State, q.State,
		q.ResourceType, q.ResourceType,
		q.Effect, q.Effect,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

// Create 插入 permission。
func (r Repository) Create(ctx context.Context, permission *domain.Permission) error {
	users, err := mysqlshared.MarshalJSON(permission.Users)
	if err != nil {
		return err
	}
	groups, err := mysqlshared.MarshalJSON(permission.Groups)
	if err != nil {
		return err
	}
	roles, err := mysqlshared.MarshalJSON(permission.Roles)
	if err != nil {
		return err
	}
	domains, err := mysqlshared.MarshalJSON(permission.Domains)
	if err != nil {
		return err
	}
	resources, err := mysqlshared.MarshalJSON(permission.Resources)
	if err != nil {
		return err
	}
	actions, err := mysqlshared.MarshalJSON(permission.Actions)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx, queryInsertPermission,
		permission.ID, permission.OrganizationID, permission.Name, permission.DisplayName, permission.Description,
		users, groups, roles, domains,
		permission.ResourceType, resources, actions, permission.Effect, permission.IsEnabled,
		permission.Submitter, permission.Approver, permission.State, permission.Model, permission.Adapter,
		permission.Sorter, permission.Application, permission.ApproveTime,
		permission.CreatedAt, permission.UpdatedAt, permission.Revision,
	)
	return err
}

// Update 乐观锁更新。
// PG 版用 RETURNING revision 一步到位；MySQL 版拆成两步：
//  1. UPDATE ... WHERE id = ? AND revision = ? AND organization_id = ? → 判断影响行数
//  2. SELECT revision FROM ... WHERE id = ? AND organization_id = ? → 回读新 revision
func (r Repository) Update(ctx context.Context, permission *domain.Permission) error {
	users, err := mysqlshared.MarshalJSON(permission.Users)
	if err != nil {
		return err
	}
	groups, err := mysqlshared.MarshalJSON(permission.Groups)
	if err != nil {
		return err
	}
	roles, err := mysqlshared.MarshalJSON(permission.Roles)
	if err != nil {
		return err
	}
	domains, err := mysqlshared.MarshalJSON(permission.Domains)
	if err != nil {
		return err
	}
	resources, err := mysqlshared.MarshalJSON(permission.Resources)
	if err != nil {
		return err
	}
	actions, err := mysqlshared.MarshalJSON(permission.Actions)
	if err != nil {
		return err
	}

	res, err := r.db.ExecContext(ctx, queryUpdatePermission,
		permission.DisplayName, permission.Description,
		users, groups, roles, domains,
		permission.ResourceType, resources, actions, permission.Effect, permission.IsEnabled,
		permission.Submitter, permission.Approver, permission.State, permission.Model, permission.Adapter,
		permission.Sorter, permission.Application, permission.ApproveTime, permission.UpdatedAt,
		permission.ID, permission.Revision-1, permission.OrganizationID,
	)
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewConflict("permission revision conflict")
	}

	// 回读新 revision（带组织谓词）
	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectRevision, permission.ID, permission.OrganizationID).Scan(&newRevision); err != nil {
		if err == sql.ErrNoRows {
			return apperrors.NewNotFound("permission not found")
		}
		return err
	}
	permission.Revision = newRevision
	return nil
}

// Delete 只在 permission 属于该组织时删除；0 行受影响统一 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryDeletePermission, id, organizationID)
	if err != nil {
		return err
	}
	n, err := rowsAffected(res, err)
	if err != nil {
		return err
	}
	if n == 0 {
		return apperrors.NewNotFound("permission not found")
	}
	return nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
