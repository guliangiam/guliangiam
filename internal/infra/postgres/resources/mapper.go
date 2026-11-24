package resources

import domain "github.com/guliangiam/guliangiam/internal/domain/resources"

func toResource(row resourceRow) *domain.Resource {
	return &domain.Resource{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		ApplicationID:  row.ApplicationID,
		Name:           row.Name,
		DisplayName:    row.DisplayName,
		Type:           row.Type,
		Tag:            row.Tag,
		FilePath:       row.FilePath,
		FileSize:       row.FileSize,
		MimeType:       row.MimeType,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
}
