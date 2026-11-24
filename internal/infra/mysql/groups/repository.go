package groups

import (
	"context"
	"database/sql"
	"encoding/json"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformsorting "github.com/guliangiam/guliangiam/internal/platform/sorting"
)

// Repository 是 MySQL 8.0+ 实现的 groups 存储层。
type Repository struct {
	db mysqlshared.DBTX
}

// NewRepository 接受 *sql.DB 或 *sql.Tx 构造 Repository。
func NewRepository(db mysqlshared.DBTX) Repository {
	return Repository{db: db}
}

// groupRow 是 groups 表扫描目标。
// Users 在 PG 是 jsonb → MySQL TEXT (JSON 字符串)。
type groupRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Manager        string
	ContactEmail   string
	Type           string
	ParentID       string
	IsTopGroup     bool
	IsEnabled      bool
	Homepage       string
	Users          string // JSON 字符串 → []string
	CreatedAt      sql.NullTime
	UpdatedAt      sql.NullTime
	Revision       int64
}

// GetByID 按 id 查 Group（系统/协议路径，无请求租户）。
func (r Repository) GetByID(ctx context.Context, id string) (*domain.Group, error) {
	return r.getByIDQuery(ctx, queryGetGroupByID, id)
}

// GetByIDForOrganization 只在 group 属于该组织时返回；跨租户与不存在统一 NotFound。
func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Group, error) {
	return r.getByIDQuery(ctx, queryGetGroupByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...interface{}) (*domain.Group, error) {
	var row groupRow
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Manager,
		&row.ContactEmail, &row.Type, &row.ParentID, &row.IsTopGroup, &row.IsEnabled,
		&row.Homepage, &row.Users,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("group not found")
		}
		return nil, err
	}
	group, err := toGroup(row)
	if err != nil {
		return nil, apperrors.NewInternal("decode group")
	}
	return group, nil
}

var groupSortSpec = platformsorting.NewSpec(map[string]string{
	"id":          "id",
	"name":        "name",
	"displayName": "display_name",
	"createdAt":   "created_at",
	"updatedAt":   "updated_at",
})

// buildGroupOrderBy 保留无排序参数时的历史默认 created_at DESC，
// 其余走手册 4.8.1 白名单（未列字段 422，尾随 id tiebreaker）。
func buildGroupOrderBy(sortField, sortOrder string) (string, error) {
	if sortField == "" && sortOrder == "" {
		sortOrder = "desc"
	}
	return groupSortSpec.Resolve(sortField, sortOrder, "created_at")
}

// List 分页列出 Group。
// countArgs: org×2 + keyword×3 + status×2 + type×2 = 9
// listArgs: countArgs + 2 = 11
func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Group, int64, error) {
	offset := mysqlshared.PageOffset(query.Page, query.PageSize)
	limit := mysqlshared.ClampPageSize(query.PageSize)

	orderBy, orderErr := buildGroupOrderBy(query.SortField, query.SortOrder)
	if orderErr != nil {
		return nil, 0, orderErr
	}

	countArgs := []interface{}{
		query.OrganizationID, query.OrganizationID,
		query.Keyword, query.Keyword, query.Keyword,
		query.Status, query.Status,
		query.Type, query.Type,
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, queryCountGroups, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append(append([]interface{}{}, countArgs...), limit, offset)
	listSQL := queryListGroups + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Group, 0)
	for rows.Next() {
		var row groupRow
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Manager,
			&row.ContactEmail, &row.Type, &row.ParentID, &row.IsTopGroup, &row.IsEnabled,
			&row.Homepage, &row.Users,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		group, err := toGroup(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode group")
		}
		items = append(items, group)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}
	return items, total, nil
}

// Create 插入 Group。Users 序列化为 JSON 字符串。
func (r Repository) Create(ctx context.Context, group *domain.Group) error {
	usersStr, err := marshalUsers(group.Users)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryInsertGroup,
		group.ID, group.OrganizationID, group.Name, group.DisplayName,
		group.Manager, group.ContactEmail, group.Type, group.ParentID,
		group.IsTopGroup, group.IsEnabled, group.Homepage, usersStr,
		group.CreatedAt, group.UpdatedAt, group.Revision,
	)
	return err
}

// Update 更新 Group。MySQL 不支持 RETURNING revision，拆成 UPDATE + SELECT。
// affected rows == 0 → revision 冲突。
func (r Repository) Update(ctx context.Context, group *domain.Group) error {
	usersStr, err := marshalUsers(group.Users)
	if err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, queryUpdateGroup,
		group.Name, group.DisplayName, group.Manager, group.ContactEmail,
		group.Type, group.ParentID, group.IsTopGroup, group.IsEnabled,
		group.Homepage, usersStr,
		group.UpdatedAt, group.ID, group.Revision, group.OrganizationID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewConflict("group revision conflict")
	}

	var newRevision int64
	if err := r.db.QueryRowContext(ctx, querySelectGroupRevision, group.ID, group.OrganizationID).Scan(&newRevision); err != nil {
		return err
	}
	group.Revision = newRevision
	return nil
}

// Delete 按 id + organization_id 删除 Group；跨租户与不存在均返回 NotFound。
func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	result, err := r.db.ExecContext(ctx, queryDeleteGroup, id, organizationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return apperrors.NewNotFound("group not found")
	}
	return nil
}

// CountChildrenByParentID 按 parent_id 统计子 group 数量。
func (r Repository) CountChildrenByParentID(ctx context.Context, organizationID, parentID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, queryCountChildrenByParentID, organizationID, parentID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// toGroup 把 MySQL groupRow 转成 domain.Group，处理 users JSON 反序列化。
func toGroup(row groupRow) (*domain.Group, error) {
	group := &domain.Group{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Manager:        row.Manager,
		ContactEmail:   row.ContactEmail,
		Type:           row.Type,
		ParentID:       row.ParentID,
		IsTopGroup:     row.IsTopGroup,
		IsEnabled:      row.IsEnabled,
		Homepage:       row.Homepage,
		Revision:       row.Revision,
	}
	if row.CreatedAt.Valid {
		group.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		group.UpdatedAt = row.UpdatedAt.Time
	}
	if row.Users != "" && row.Users != "null" {
		if err := json.Unmarshal([]byte(row.Users), &group.Users); err != nil {
			return nil, err
		}
	}
	if group.Users == nil {
		group.Users = []string{}
	}
	return group, nil
}

func marshalUsers(users []string) (string, error) {
	if users == nil {
		return "[]", nil
	}
	b, err := json.Marshal(users)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// 编译期接口断言
var _ domain.Repository = Repository{}
