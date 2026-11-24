package enforcers

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
)

// rowToDomain 把 MySQL 表扫描结果转成 domain.Enforcer。
func rowToDomain(row enforcerRow) *domain.Enforcer {
	return &domain.Enforcer{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name,
		DisplayName: row.DisplayName, Description: row.Description,
		ModelID: nullOrString(row.ModelID), AdapterID: nullOrString(row.AdapterID), IsEnabled: row.IsEnabled,
		Owner: row.Owner, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
	}
}
