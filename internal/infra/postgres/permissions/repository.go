package permissions

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.Permission, error) {
	return r.getByIDQuery(ctx, queryGetPermissionByID, id)
}

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Permission, error) {
	return r.getByIDQuery(ctx, queryGetPermissionByIDForOrganization, id, organizationID)
}

func (r Repository) getByIDQuery(ctx context.Context, query string, args ...any) (*domain.Permission, error) {
	var row permissionRow
	err := r.db.QueryRow(ctx, query, args...).Scan(
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
	if err != nil {
		if err == pgx.ErrNoRows {
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

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Permission, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListPermissions, query.OrganizationID, query.Keyword, query.IsEnabled, query.State, query.ResourceType, query.Effect, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Permission, 0)
	for rows.Next() {
		var row permissionRow
		if err := rows.Scan(
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
		); err != nil {
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

	var total int64
	if err := r.db.QueryRow(ctx, queryCountPermissions, query.OrganizationID, query.Keyword, query.IsEnabled, query.State, query.ResourceType, query.Effect).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, permission *domain.Permission) error {
	users, err := shared.MarshalJSON(permission.Users)
	if err != nil {
		return err
	}
	groups, err := shared.MarshalJSON(permission.Groups)
	if err != nil {
		return err
	}
	roles, err := shared.MarshalJSON(permission.Roles)
	if err != nil {
		return err
	}
	domains, err := shared.MarshalJSON(permission.Domains)
	if err != nil {
		return err
	}
	resources, err := shared.MarshalJSON(permission.Resources)
	if err != nil {
		return err
	}
	actions, err := shared.MarshalJSON(permission.Actions)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryInsertPermission, permission.ID, permission.OrganizationID, permission.Name, permission.DisplayName, permission.Description, users, groups, roles, domains, permission.ResourceType, resources, actions, permission.Effect, permission.IsEnabled, permission.Submitter, permission.Approver, permission.State, permission.Model, permission.Adapter, permission.Sorter, permission.Application, permission.ApproveTime, permission.CreatedAt, permission.UpdatedAt, permission.Revision)
	return err
}

func (r Repository) Update(ctx context.Context, permission *domain.Permission) error {
	users, err := shared.MarshalJSON(permission.Users)
	if err != nil {
		return err
	}
	groups, err := shared.MarshalJSON(permission.Groups)
	if err != nil {
		return err
	}
	roles, err := shared.MarshalJSON(permission.Roles)
	if err != nil {
		return err
	}
	domains, err := shared.MarshalJSON(permission.Domains)
	if err != nil {
		return err
	}
	resources, err := shared.MarshalJSON(permission.Resources)
	if err != nil {
		return err
	}
	actions, err := shared.MarshalJSON(permission.Actions)
	if err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdatePermission, permission.DisplayName, permission.Description, users, groups, roles, domains, permission.ResourceType, resources, actions, permission.Effect, permission.IsEnabled, permission.Submitter, permission.Approver, permission.State, permission.Model, permission.Adapter, permission.Sorter, permission.Application, permission.ApproveTime, permission.UpdatedAt, permission.Revision-1, permission.ID, permission.OrganizationID).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("permission revision conflict")
		}
		return err
	}
	permission.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeletePermission, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("permission not found")
	}
	return nil
}

var _ domain.Repository = Repository{}

func maxInt(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}
