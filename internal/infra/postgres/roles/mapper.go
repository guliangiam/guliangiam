package roles

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func toRole(row roleRow) (*domain.Role, error) {
	role := &domain.Role{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Description:    row.Description,
		IsEnabled:      row.IsEnabled,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if err := shared.UnmarshalJSON(row.Users, &role.Users); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.GroupIDs, &role.GroupIDs); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.RoleIDs, &role.RoleIDs); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.Domains, &role.Domains); err != nil {
		return nil, err
	}
	if role.Users == nil {
		role.Users = []string{}
	}
	if role.GroupIDs == nil {
		role.GroupIDs = []string{}
	}
	if role.RoleIDs == nil {
		role.RoleIDs = []string{}
	}
	if role.Domains == nil {
		role.Domains = []string{}
	}
	return role, nil
}
