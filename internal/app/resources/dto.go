package resources

import "time"

type ResourceListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	ApplicationID  string    `json:"applicationId,omitempty"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Type           string    `json:"type"`
	Tag            string    `json:"tag,omitempty"`
	FilePath       string    `json:"filePath"`
	FileSize       int64     `json:"fileSize"`
	MimeType       string    `json:"mimeType,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type ResourceDetail struct {
	ResourceListItem
	Revision int64 `json:"revision"`
}
