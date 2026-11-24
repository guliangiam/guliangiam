package groups

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/groups"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformsorting "github.com/guliangiam/guliangiam/internal/platform/sorting"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Group, error) {
	return r.getByIDQuery(ctx, queryGetGroupByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Group, error) {
	return r.getByIDQuery(ctx, queryGetGroupByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Group, error) {
	var row groupRow
	// parent_id 可为 NULL（顶层组），直接扫进 string 会报
	// “cannot scan NULL into *string”，因此先扫到指针再归一成空串。
	var parentID *string
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Manager,
		&row.ContactEmail, &row.Type, &parentID, &row.IsTopGroup, &row.IsEnabled,
		&row.Homepage, &row.Users,
		&row.CreatedAt, &row.UpdatedAt, &row.Revision,
	)
	row.ParentID = stringOrEmpty(parentID)
	if err != nil {
		if err == pgx.ErrNoRows {
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

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Group, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	if query.Offset > 0 {
		offset = query.Offset
	}
	orderBy, orderErr := buildGroupOrderBy(query.SortField, query.SortOrder)
	if orderErr != nil {
		return nil, 0, orderErr
	}
	listSQL := queryListGroups + " " + orderBy + " LIMIT $5 OFFSET $6"
	rows, err := r.db.Query(ctx, listSQL, query.OrganizationID, query.Keyword, query.Status, query.Type, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Group, 0)
	for rows.Next() {
		var row groupRow
		var parentID *string
		if err := rows.Scan(
			&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Manager,
			&row.ContactEmail, &row.Type, &parentID, &row.IsTopGroup, &row.IsEnabled,
			&row.Homepage, &row.Users,
			&row.CreatedAt, &row.UpdatedAt, &row.Revision,
		); err != nil {
			return nil, 0, err
		}
		row.ParentID = stringOrEmpty(parentID)
		group, err := toGroup(row)
		if err != nil {
			return nil, 0, apperrors.NewInternal("decode group")
		}
		items = append(items, group)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	var total int64
	if err := r.db.QueryRow(ctx, queryCountGroups, query.OrganizationID, query.Keyword, query.Status, query.Type).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, group *domain.Group) error {
	users, err := shared.MarshalJSON(group.Users)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, queryInsertGroup,
		group.ID, group.OrganizationID, group.Name, group.DisplayName,
		group.Manager, group.ContactEmail, group.Type, nullableUUID(group.ParentID),
		group.IsTopGroup, group.IsEnabled, group.Homepage, users,
		group.CreatedAt, group.UpdatedAt, group.Revision,
	)
	return err
}

func (r Repository) Update(ctx context.Context, group *domain.Group) error {
	users, err := shared.MarshalJSON(group.Users)
	if err != nil {
		return err
	}
	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateGroup,
		group.Name, group.DisplayName, group.Manager, group.ContactEmail,
		group.Type, nullableUUID(group.ParentID), group.IsTopGroup, group.IsEnabled,
		group.Homepage, users,
		// App 层已 Revision++，这里要传自增前的值做乐观锁守卫。
		group.UpdatedAt, group.Revision-1, group.ID, group.OrganizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("group revision conflict")
		}
		return err
	}
	group.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	result, err := r.db.Exec(ctx, queryDeleteGroup, id, organizationID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return apperrors.NewNotFound("group not found")
	}
	return nil
}

func (r Repository) CountChildrenByParentID(ctx context.Context, organizationID, parentID string) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, queryCountChildrenByParentID, organizationID, parentID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func stringOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// nullableUUID 把空字符串转成 NULL：parent_id 是 uuid 列，直接写 "" 会触发
// SQLSTATE 22P02（invalid input syntax for type uuid）。
func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

var _ domain.Repository = Repository{}

// maxInt returns value when positive, otherwise fallback. Kept package-local
// (and renamed from max) so it does not shadow the predeclared max builtin.
func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
