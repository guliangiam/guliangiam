package scim

import (
	"context"
	"errors" // stdlib errors retained for errors.Is/errors.As only; apperrors used for error creation
	"fmt"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/guliangiam/guliangiam/internal/domain/scim"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (r *Repository) GetGroupByID(ctx context.Context, groupID string) (*scim.ScimGroup, error) {
	var model ScimGroupModel
	err := r.pool.QueryRow(ctx, QueryGetScimGroupByID, groupID).Scan(
		&model.ID,
		&model.OrganizationID,
		&model.Name,
		&model.DisplayName,
		&model.CreatedAt,
		&model.UpdatedAt,
		&model.Revision,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrGroupNotFound
		}
		return nil, apperrors.WrapInternalf(err, "failed to get scim group")
	}

	// Get group members
	members, err := r.getGroupMembers(ctx, groupID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "failed to get group members")
	}

	return r.modelToGroup(&model, members)
}

func (r *Repository) ListGroups(ctx context.Context, organizationID string, query *scim.ListQuery) ([]*scim.ScimGroup, int64, error) {
	baseQuery := `FROM guliangiam.groups WHERE organization_id = $1`
	countQuery := `SELECT COUNT(*) ` + baseQuery
	selectQuery := `SELECT id, organization_id, name, display_name, created_at, updated_at, revision ` + baseQuery

	args := []interface{}{organizationID}
	argPos := 2

	// Apply filter if provided
	if query != nil && query.Filter != "" {
		whereClause := r.buildFilterWhere(query.Filter, argPos)
		selectQuery += " AND " + whereClause.clause
		countQuery += " AND " + whereClause.clause
		args = append(args, whereClause.args...)
		argPos = whereClause.nextPos
	}

	// Get total count
	var total int64
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "failed to count scim groups")
	}

	// Add ordering and pagination
	selectQuery += " ORDER BY display_name"
	if query != nil && query.SortOrder == "descending" {
		selectQuery += " DESC"
	} else {
		selectQuery += " ASC"
	}

	if query != nil && query.Count > 0 {
		selectQuery += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, query.Count)
		argPos++
	}

	if query != nil && query.StartIndex > 0 {
		selectQuery += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, query.StartIndex-1) // SCIM uses 1-based index
	}

	// Execute query
	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, apperrors.WrapInternalf(err, "failed to list scim groups")
	}
	defer rows.Close()

	var groups []*scim.ScimGroup
	for rows.Next() {
		var model ScimGroupModel
		err := rows.Scan(
			&model.ID,
			&model.OrganizationID,
			&model.Name,
			&model.DisplayName,
			&model.CreatedAt,
			&model.UpdatedAt,
			&model.Revision,
		)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "failed to scan scim group")
		}

		// Get group members
		members, _ := r.getGroupMembers(ctx, model.ID.String())

		group, err := r.modelToGroup(&model, members)
		if err != nil {
			return nil, 0, apperrors.WrapInternalf(err, "failed to convert scim group")
		}
		groups = append(groups, group)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}

	return groups, total, nil
}

func (r *Repository) CreateGroup(ctx context.Context, group *scim.ScimGroup) error {
	now := time.Now()
	// 建组与成员写入必须原子：成员失败时不能留下“有组无成员”的半成品。
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.WrapInternalf(err, "begin scim group transaction")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := createGroupRows(ctx, tx, group, now); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperrors.WrapInternalf(err, "commit scim group transaction")
	}
	return nil
}

// execer 是 *pgxpool.Pool 与 pgx.Tx 的最小写接口，便于单测注入失败路径。
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// createGroupRows 写入组与全部成员；任一成员失败即返回，由调用方回滚事务。
func createGroupRows(ctx context.Context, exec execer, group *scim.ScimGroup, now time.Time) error {
	if _, err := exec.Exec(ctx, QueryCreateScimGroup,
		group.ID,
		group.OrganizationID,
		group.DisplayName, // Use displayName as name
		group.DisplayName,
		now,
		now,
		group.Revision,
	); err != nil {
		return apperrors.WrapInternalf(err, "failed to create scim group")
	}
	for _, member := range group.Members {
		if _, err := exec.Exec(ctx, QueryAddGroupMember, member.Value, group.ID); err != nil {
			return apperrors.WrapInternalf(err, "add scim group member %s", member.Value)
		}
	}
	return nil
}

func (r *Repository) UpdateGroup(ctx context.Context, group *scim.ScimGroup) error {
	// SCIM PUT: full replacement of group attributes
	var newRevision int64
	err := r.pool.QueryRow(ctx, QueryUpdateScimGroup,
		group.ID,
		group.DisplayName,
		group.DisplayName, // name mirrors displayName
		time.Now(),
	).Scan(&newRevision)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrGroupNotFound
		}
		return apperrors.WrapInternalf(err, "failed to update scim group")
	}

	group.Revision = newRevision
	group.UpdatedAt = time.Now()

	// Sync members: delete all then re-insert
	_, _ = r.pool.Exec(ctx, QueryRemoveGroupMembersByGroup, group.ID)
	for _, member := range group.Members {
		_, _ = r.pool.Exec(ctx, QueryAddGroupMember,
			member.Value, group.ID,
		)
	}

	return nil
}

func (r *Repository) DeleteGroup(ctx context.Context, groupID string) error {
	result, err := r.pool.Exec(ctx, QueryDeleteScimGroup, groupID)
	if err != nil {
		return apperrors.WrapInternalf(err, "failed to delete scim group")
	}

	if result.RowsAffected() == 0 {
		return ErrGroupNotFound
	}

	return nil
}

func (r *Repository) PatchGroup(ctx context.Context, groupID string, patches []scim.PatchOperation) (*scim.ScimGroup, error) {
	// Get current group
	group, err := r.GetGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}

	// Apply patches
	for _, patch := range patches {
		switch patch.Op {
		case "replace":
			r.applyGroupReplacePatch(group, patch)
		case "add":
			r.applyGroupAddPatch(group, patch)
		case "remove":
			r.applyGroupRemovePatch(group, patch)
		}
	}

	// Persist changes
	err = r.UpdateGroup(ctx, group)
	if err != nil {
		return nil, err
	}

	return group, nil
}

// ==================== Helper Methods ====================

func (r *Repository) getGroupMembers(ctx context.Context, groupID string) ([]scim.Member, error) {
	rows, err := r.pool.Query(ctx, QueryGetGroupMembers, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []scim.Member
	for rows.Next() {
		var id, username, displayName string
		err := rows.Scan(&id, &username, &displayName)
		if err != nil {
			return nil, err
		}
		members = append(members, scim.Member{
			Value:   id,
			Display: displayName,
			Ref:     "/scim/v2/Users/" + id,
			Type:    resourceTypeUser,
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}

	return members, nil
}

func (r *Repository) addGroupMember(ctx context.Context, groupID, userID string) error {
	_, err := r.pool.Exec(ctx, QueryAddGroupMember, userID, groupID)
	return err
}

type whereClause struct {
	clause  string
	args    []interface{}
	nextPos int
}
