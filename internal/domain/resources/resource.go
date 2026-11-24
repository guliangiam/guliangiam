package resources

import "time"

type Resource struct {
	ID             string
	OrganizationID string
	ApplicationID  string
	Name           string
	DisplayName    string
	Type           string // image, css, js, file, video, audio
	Tag            string
	FilePath       string
	FileSize       int64
	MimeType       string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
