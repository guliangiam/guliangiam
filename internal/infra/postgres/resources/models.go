package resources

import "time"

type resourceRow struct {
	ID             string
	OrganizationID string
	ApplicationID  string
	Name           string
	DisplayName    string
	Type           string
	Tag            string
	FilePath       string
	FileSize       int64
	MimeType       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
