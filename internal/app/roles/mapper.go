package roles

import domain "github.com/guliangiam/guliangiam/internal/domain/roles"

func toListItem(role *domain.Role) RoleListItem {
	return RoleListItem{
		ID:             role.ID,
		OrganizationID: role.OrganizationID,
		Name:           role.Name,
		DisplayName:    role.DisplayName,
		Description:    role.Description,
		GroupIDs:       append([]string(nil), role.GroupIDs...),
		IsEnabled:      role.IsEnabled,
		CreatedAt:      role.CreatedAt,
		UpdatedAt:      role.UpdatedAt,
	}
}

func toDetail(role *domain.Role) RoleDetail {
	return RoleDetail{
		RoleListItem: toListItem(role),
		Users:        append([]string(nil), role.Users...),
		RoleIDs:      append([]string(nil), role.RoleIDs...),
		Domains:      append([]string(nil), role.Domains...),
		Revision:     role.Revision,
	}
}
