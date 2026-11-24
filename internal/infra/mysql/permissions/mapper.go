package permissions

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// toPermission 把 MySQL 表扫描结果转成 domain.Permission。
// JSON 列 (users/groups/roles/domains/resources/actions) 从 []byte 反序列化为 []string。
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
	if err := mysqlshared.UnmarshalJSON(row.Users, &permission.Users); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.Groups, &permission.Groups); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.Roles, &permission.Roles); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.Domains, &permission.Domains); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.Resources, &permission.Resources); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.Actions, &permission.Actions); err != nil {
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
