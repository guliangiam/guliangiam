package roles

import (
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"

	domain "github.com/guliangiam/guliangiam/internal/domain/roles"
)

// toRole 把 MySQL 表扫描结果转成 domain.Role。
// 四个 JSON 列（users / group_ids / role_ids / domains）从 []byte 反序列化为 []string。
// 与 PG 版 mapper.go 行为一致。
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
	if err := mysqlshared.UnmarshalJSON(row.Users, &role.Users); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.GroupIDs, &role.GroupIDs); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.RoleIDs, &role.RoleIDs); err != nil {
		return nil, err
	}
	if err := mysqlshared.UnmarshalJSON(row.Domains, &role.Domains); err != nil {
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
