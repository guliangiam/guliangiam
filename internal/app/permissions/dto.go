package permissions

import "time"

type PermissionListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Description    string    `json:"description"`
	ResourceType   string    `json:"resourceType"`
	Effect         string    `json:"effect"`
	IsEnabled      bool      `json:"isEnabled"`
	Submitter      string    `json:"submitter"`
	Approver       string    `json:"approver"`
	State          string    `json:"state"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type PermissionDetail struct {
	PermissionListItem
	Users       []string   `json:"users"`
	Groups      []string   `json:"groups"`
	Roles       []string   `json:"roles"`
	Domains     []string   `json:"domains"`
	Resources   []string   `json:"resources"`
	Actions     []string   `json:"actions"`
	Model       string     `json:"model,omitempty"`
	Adapter     string     `json:"adapter,omitempty"`
	Sorter      string     `json:"sorter,omitempty"`
	Application string     `json:"application,omitempty"`
	ApproveTime *time.Time `json:"approveTime,omitempty"`
	Revision    int64      `json:"revision"`
}

type EnforceResult struct {
	Allowed bool `json:"allowed"`
}

type BatchEnforceResult struct {
	Results []bool `json:"results"`
}

type ImportPermission struct {
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	ResourceType   string   `json:"resourceType"`
	Effect         string   `json:"effect"`
	Resources      []string `json:"resources,omitempty"`
	Actions        []string `json:"actions,omitempty"`
	Users          []string `json:"users,omitempty"`
	Groups         []string `json:"groups,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	OrganizationID string   `json:"organizationId"`
}

type BulkImportResult struct {
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
}
