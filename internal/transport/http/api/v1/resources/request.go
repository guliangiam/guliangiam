package resources

import (
	"io"

	appresources "github.com/guliangiam/guliangiam/internal/app/resources"
)

type ListResourcesRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	ApplicationID  string
	Keyword        string
	Type           string
	Tag            string
}

type CreateResourceRequest struct {
	OrganizationID string `json:"organizationId"`
	ApplicationID  string `json:"applicationId,omitempty"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	Type           string `json:"type,omitempty"`
	Tag            string `json:"tag,omitempty"`
	FilePath       string `json:"filePath,omitempty"`
	FileSize       int64  `json:"fileSize,omitempty"`
	MimeType       string `json:"mimeType,omitempty"`
}

type PatchResourceRequest struct {
	DisplayName *string `json:"displayName,omitempty"`
	Type        *string `json:"type,omitempty"`
	Tag         *string `json:"tag,omitempty"`
	FilePath    *string `json:"filePath,omitempty"`
	FileSize    *int64  `json:"fileSize,omitempty"`
	MimeType    *string `json:"mimeType,omitempty"`
	Revision    int64   `json:"revision,omitempty"`
}

// UploadResourceCommand is the transport-layer command for file uploads.
type UploadResourceCommand struct {
	Owner    string
	User     string
	Tag      string
	FileName string
	FileSize int64
	MimeType string
	Reader   io.Reader
}

func (r CreateResourceRequest) toCommand() appresources.CreateResourceCommand {
	return appresources.CreateResourceCommand{
		OrganizationID: r.OrganizationID,
		ApplicationID:  r.ApplicationID,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		Type:           r.Type,
		Tag:            r.Tag,
		FilePath:       r.FilePath,
		FileSize:       r.FileSize,
		MimeType:       r.MimeType,
	}
}

func (r PatchResourceRequest) toCommand(id string) appresources.PatchResourceCommand {
	return appresources.PatchResourceCommand{
		ID:          id,
		DisplayName: r.DisplayName,
		Type:        r.Type,
		Tag:         r.Tag,
		FilePath:    r.FilePath,
		FileSize:    r.FileSize,
		MimeType:    r.MimeType,
		Revision:    r.Revision,
	}
}

func (r UploadResourceCommand) toServiceCommand() appresources.UploadResourceCommand {
	return appresources.UploadResourceCommand{
		Owner:    r.Owner,
		User:     r.User,
		Tag:      r.Tag,
		FileName: r.FileName,
		FileSize: r.FileSize,
		MimeType: r.MimeType,
		Reader:   r.Reader,
	}
}
