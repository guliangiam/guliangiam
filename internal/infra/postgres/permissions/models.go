package permissions

import "time"

type permissionRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Users          []byte
	Groups         []byte
	Roles          []byte
	Domains        []byte
	ResourceType   string
	Resources      []byte
	Actions        []byte
	Effect         string
	IsEnabled      bool
	Submitter      string
	Approver       string
	State          string
	Model          string
	Adapter        string
	Sorter         string
	Application    string
	ApproveTime    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
