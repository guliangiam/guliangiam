package models

import domain "github.com/guliangiam/guliangiam/internal/domain/models"

func toListItem(m *domain.Model) ModelListItem {
	return ModelListItem{
		ID:             m.ID,
		OrganizationID: m.OrganizationID,
		Name:           m.Name,
		DisplayName:    m.DisplayName,
		Description:    m.Description,
		Owner:          m.Owner,
		IsEnabled:      m.IsEnabled,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func toDetail(m *domain.Model) ModelDetail {
	return ModelDetail{
		ModelListItem: toListItem(m),
		ModelText:     m.ModelText,
		Revision:      m.Revision,
	}
}
