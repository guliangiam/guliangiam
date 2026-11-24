package permissions

import (
	"time"

	apppermissions "github.com/guliangiam/guliangiam/internal/app/permissions"
)

type ListPermissionsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
	State          string
	ResourceType   string
	Effect         string
}

type GetPermissionRequest struct {
	ID string
}

type CreatePermissionRequest struct {
	OrganizationID string   `json:"organizationId"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Description    string   `json:"description,omitempty"`
	Users          []string `json:"users,omitempty"`
	Groups         []string `json:"groups,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	ResourceType   string   `json:"resourceType"`
	Resources      []string `json:"resources,omitempty"`
	Actions        []string `json:"actions,omitempty"`
	Effect         string   `json:"effect,omitempty"`
	Model          string   `json:"model,omitempty"`
	Adapter        string   `json:"adapter,omitempty"`
	Sorter         string   `json:"sorter,omitempty"`
	Application    string   `json:"application,omitempty"`
	IsEnabled      *bool    `json:"isEnabled,omitempty"`
	Submitter      string   `json:"submitter,omitempty"`
	Approver       string   `json:"approver,omitempty"`
	State          string   `json:"state,omitempty"`
	ApproveTime    string   `json:"approveTime,omitempty"`
}

type PatchPermissionRequest struct {
	DisplayName  *string  `json:"displayName,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Users        []string `json:"users,omitempty"`
	Groups       []string `json:"groups,omitempty"`
	Roles        []string `json:"roles,omitempty"`
	Domains      []string `json:"domains,omitempty"`
	ResourceType *string  `json:"resourceType,omitempty"`
	Resources    []string `json:"resources,omitempty"`
	Actions      []string `json:"actions,omitempty"`
	Effect       *string  `json:"effect,omitempty"`
	Model        *string  `json:"model,omitempty"`
	Adapter      *string  `json:"adapter,omitempty"`
	Sorter       *string  `json:"sorter,omitempty"`
	Application  *string  `json:"application,omitempty"`
	IsEnabled    *bool    `json:"isEnabled,omitempty"`
	Submitter    *string  `json:"submitter,omitempty"`
	Approver     *string  `json:"approver,omitempty"`
	State        *string  `json:"state,omitempty"`
	ApproveTime  string   `json:"approveTime,omitempty"`
	Revision     int64    `json:"revision,omitempty"`
}

type EnforceRequest struct {
	EnforcerID string `json:"enforcerId"`
	Sub        string `json:"sub"`
	Dom        string `json:"dom,omitempty"`
	Obj        string `json:"obj"`
	Act        string `json:"act"`
}

type BatchEnforceRequestItem struct {
	Sub string `json:"sub"`
	Dom string `json:"dom,omitempty"`
	Obj string `json:"obj"`
	Act string `json:"act"`
}

type BatchEnforceRequest struct {
	EnforcerID string                    `json:"enforcerId"`
	Requests   []BatchEnforceRequestItem `json:"requests"`
}

type PolicyItemRequest struct {
	PType string   `json:"pType"`
	Rules []string `json:"rules"`
}

type UpdatePolicyRequest struct {
	Policies []PolicyItemRequest `json:"policies"`
}

func (r CreatePermissionRequest) toCommand() apppermissions.CreatePermissionCommand {
	var approveTime *time.Time
	if r.ApproveTime != "" {
		t, err := time.Parse(time.RFC3339, r.ApproveTime)
		if err == nil {
			approveTime = &t
		}
	}

	return apppermissions.CreatePermissionCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		Description:    r.Description,
		Users:          r.Users,
		Groups:         r.Groups,
		Roles:          r.Roles,
		Domains:        r.Domains,
		ResourceType:   r.ResourceType,
		Resources:      r.Resources,
		Actions:        r.Actions,
		Effect:         r.Effect,
		Model:          r.Model,
		Adapter:        r.Adapter,
		Sorter:         r.Sorter,
		Application:    r.Application,
		IsEnabled:      r.IsEnabled,
		Submitter:      r.Submitter,
		Approver:       r.Approver,
		State:          r.State,
		ApproveTime:    approveTime,
	}
}

func (r PatchPermissionRequest) toCommand(id string) apppermissions.PatchPermissionCommand {
	command := apppermissions.PatchPermissionCommand{
		ID:           id,
		DisplayName:  r.DisplayName,
		Description:  r.Description,
		ResourceType: r.ResourceType,
		Effect:       r.Effect,
		Model:        r.Model,
		Adapter:      r.Adapter,
		Sorter:       r.Sorter,
		Application:  r.Application,
		IsEnabled:    r.IsEnabled,
		Submitter:    r.Submitter,
		Approver:     r.Approver,
		State:        r.State,
		Revision:     r.Revision,
	}
	if r.Users != nil {
		command.Users = r.Users
	}
	if r.Groups != nil {
		command.Groups = r.Groups
	}
	if r.Roles != nil {
		command.Roles = r.Roles
	}
	if r.Domains != nil {
		command.Domains = r.Domains
	}
	if r.Resources != nil {
		command.Resources = r.Resources
	}
	if r.Actions != nil {
		command.Actions = r.Actions
	}
	if r.ApproveTime != "" {
		t, err := time.Parse(time.RFC3339, r.ApproveTime)
		if err == nil {
			command.ApproveTime = &t
		}
	}
	return command
}
