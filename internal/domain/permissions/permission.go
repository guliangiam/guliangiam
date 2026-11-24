package permissions

import "time"

type Permission struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Description    string
	Users          []string
	Groups         []string
	Roles          []string
	Domains        []string
	ResourceType   string
	Resources      []string
	Actions        []string
	Effect         string
	Model          string
	Adapter        string
	Sorter         string
	Application    string
	IsEnabled      bool
	Submitter      string
	Approver       string
	State          string
	ApproveTime    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}
