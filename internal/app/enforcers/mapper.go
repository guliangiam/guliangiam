package enforcers

import domain "github.com/guliangiam/guliangiam/internal/domain/enforcers"

func toListItem(e *domain.Enforcer) EnforcerListItem {
	return EnforcerListItem{
		ID:             e.ID,
		OrganizationID: e.OrganizationID,
		Name:           e.Name,
		DisplayName:    e.DisplayName,
		ModelID:        e.ModelID,
		AdapterID:      e.AdapterID,
		IsEnabled:      e.IsEnabled,
		Owner:          e.Owner,
		CreatedAt:      e.CreatedAt,
		UpdatedAt:      e.UpdatedAt,
	}
}

func toDetail(e *domain.Enforcer) EnforcerDetail {
	return EnforcerDetail{
		EnforcerListItem: toListItem(e),
		Description:      e.Description,
		Revision:         e.Revision,
	}
}
