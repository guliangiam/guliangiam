package permissions

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

func toPermission(row permissionRow) (*domain.Permission, error) {
	permission := &domain.Permission{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Description:    row.Description,
		ResourceType:   row.ResourceType,
		Effect:         row.Effect,
		IsEnabled:      row.IsEnabled,
		Submitter:      row.Submitter,
		Approver:       row.Approver,
		State:          row.State,
		Model:          row.Model,
		Adapter:        row.Adapter,
		Sorter:         row.Sorter,
		Application:    row.Application,
		ApproveTime:    row.ApproveTime,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
	if err := shared.UnmarshalJSON(row.Users, &permission.Users); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.Groups, &permission.Groups); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.Roles, &permission.Roles); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.Domains, &permission.Domains); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.Resources, &permission.Resources); err != nil {
		return nil, err
	}
	if err := shared.UnmarshalJSON(row.Actions, &permission.Actions); err != nil {
		return nil, err
	}
	if permission.Users == nil {
		permission.Users = []string{}
	}
	if permission.Groups == nil {
		permission.Groups = []string{}
	}
	if permission.Roles == nil {
		permission.Roles = []string{}
	}
	if permission.Domains == nil {
		permission.Domains = []string{}
	}
	if permission.Resources == nil {
		permission.Resources = []string{}
	}
	if permission.Actions == nil {
		permission.Actions = []string{}
	}
	return permission, nil
}
