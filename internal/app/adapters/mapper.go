package adapters

import domain "github.com/guliangiam/guliangiam/internal/domain/adapters"

func toListItem(a *domain.Adapter) AdapterListItem {
	return AdapterListItem{
		ID:             a.ID,
		OrganizationID: a.OrganizationID,
		Name:           a.Name,
		DisplayName:    a.DisplayName,
		Type:           a.Type,
		Server:         a.Server,
		Database:       a.Database,
		Owner:          a.Owner,
		UseSameDb:      a.UseSameDb,
		IsEnabled:      a.IsEnabled,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
}

func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "*****"
}

func toDetail(a *domain.Adapter) AdapterDetail {
	return AdapterDetail{
		AdapterListItem:  toListItem(a),
		Description:      a.Description,
		TableName:        a.TableName,
		DatabaseType:     a.DatabaseType,
		Host:             a.Host,
		Port:             a.Port,
		DbUser:           a.DbUser,
		DbPasswordSet:    a.DbPassword != "",
		DbPasswordMasked: maskSecret(a.DbPassword),
		DatabaseName:     a.DatabaseName,
		Policy:           a.Policy,
		Revision:         a.Revision,
	}
}
