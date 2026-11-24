package roles

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
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

func (r Repository) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Role, error) {
	var row roleRow
	err := r.db.QueryRow(ctx, queryGetRoleByIDForOrganization, id, organizationID).Scan(&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description, &row.Users, &row.GroupIDs, &row.RoleIDs, &row.Domains, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision)
	if err != nil {
		if err == pgx.ErrNoRows {
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

func (r Repository) List(ctx context.Context, query domain.ListQuery) ([]*domain.Role, int64, error) {
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListRoles, query.OrganizationID, query.Keyword, query.IsEnabled, shared.ClampPageSize(query.PageSize), offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*domain.Role, 0)
	for rows.Next() {
		var row roleRow
		if err := rows.Scan(&row.ID, &row.OrganizationID, &row.Name, &row.DisplayName, &row.Description, &row.Users, &row.GroupIDs, &row.RoleIDs, &row.Domains, &row.IsEnabled, &row.CreatedAt, &row.UpdatedAt, &row.Revision); err != nil {
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

	var total int64
	if err := r.db.QueryRow(ctx, queryCountRoles, query.OrganizationID, query.Keyword, query.IsEnabled).Scan(&total); err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r Repository) Create(ctx context.Context, role *domain.Role) error {
	users, err := shared.MarshalJSON(role.Users)
	if err != nil {
		return err
	}
	groupIDs, err := shared.MarshalJSON(role.GroupIDs)
	if err != nil {
		return err
	}
	roleIDs, err := shared.MarshalJSON(role.RoleIDs)
	if err != nil {
		return err
	}
	domains, err := shared.MarshalJSON(role.Domains)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, queryInsertRole, role.ID, role.OrganizationID, role.Name, role.DisplayName, role.Description, users, groupIDs, roleIDs, domains, role.IsEnabled, role.CreatedAt, role.UpdatedAt, role.Revision)
	return pgerror.Map(err, "uq_roles_org_name", "fk_roles_organization_id_organizations")
}

func (r Repository) Update(ctx context.Context, role *domain.Role) error {
	users, err := shared.MarshalJSON(role.Users)
	if err != nil {
		return err
	}
	groupIDs, err := shared.MarshalJSON(role.GroupIDs)
	if err != nil {
		return err
	}
	roleIDs, err := shared.MarshalJSON(role.RoleIDs)
	if err != nil {
		return err
	}
	domains, err := shared.MarshalJSON(role.Domains)
	if err != nil {
		return err
	}

	var newRevision int64
	err = r.db.QueryRow(ctx, queryUpdateRole, role.DisplayName, role.Description, users, groupIDs, roleIDs, domains, role.IsEnabled, role.UpdatedAt, role.Revision-1, role.ID, role.OrganizationID).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("role revision conflict")
		}
		return err
	}
	role.Revision = newRevision
	return nil
}

func (r Repository) Delete(ctx context.Context, id, organizationID string) error {
	tag, err := r.db.Exec(ctx, queryDeleteRole, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("role not found")
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
