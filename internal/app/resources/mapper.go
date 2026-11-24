package resources

import domain "github.com/guliangiam/guliangiam/internal/domain/resources"

func toListItem(r *domain.Resource) ResourceListItem {
	return ResourceListItem{
		ID:             r.ID,
		OrganizationID: r.OrganizationID,
		ApplicationID:  r.ApplicationID,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		Type:           r.Type,
		Tag:            r.Tag,
		FilePath:       r.FilePath,
		FileSize:       r.FileSize,
		MimeType:       r.MimeType,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func toDetail(r *domain.Resource) ResourceDetail {
	return ResourceDetail{
		ResourceListItem: toListItem(r),
		Revision:         r.Revision,
	}
}
