package permissions

import domain "github.com/guliangiam/guliangiam/internal/domain/permissions"

func toListItem(permission *domain.Permission) PermissionListItem {
	return PermissionListItem{
		ID:             permission.ID,
		OrganizationID: permission.OrganizationID,
		Name:           permission.Name,
		DisplayName:    permission.DisplayName,
		Description:    permission.Description,
		ResourceType:   permission.ResourceType,
		Effect:         permission.Effect,
		IsEnabled:      permission.IsEnabled,
		Submitter:      permission.Submitter,
		Approver:       permission.Approver,
		State:          permission.State,
		CreatedAt:      permission.CreatedAt,
		UpdatedAt:      permission.UpdatedAt,
	}
}

func toDetail(permission *domain.Permission) PermissionDetail {
	return PermissionDetail{
		PermissionListItem: toListItem(permission),
		Users:              append([]string(nil), permission.Users...),
		Groups:             append([]string(nil), permission.Groups...),
		Roles:              append([]string(nil), permission.Roles...),
		Domains:            append([]string(nil), permission.Domains...),
		Resources:          append([]string(nil), permission.Resources...),
		Actions:            append([]string(nil), permission.Actions...),
		Model:              permission.Model,
		Adapter:            permission.Adapter,
		Sorter:             permission.Sorter,
		Application:        permission.Application,
		ApproveTime:        permission.ApproveTime,
		Revision:           permission.Revision,
	}
}
