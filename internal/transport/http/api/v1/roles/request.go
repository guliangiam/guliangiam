package roles

import approles "github.com/guliangiam/guliangiam/internal/app/roles"

type ListRolesRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type GetRoleRequest struct {
	ID string
}

type CreateRoleRequest struct {
	OrganizationID string   `json:"organizationId"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Description    string   `json:"description,omitempty"`
	GroupIDs       []string `json:"groupIds,omitempty"`
	Users          []string `json:"users,omitempty"`
	RoleIDs        []string `json:"roleIds,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	IsEnabled      *bool    `json:"isEnabled,omitempty"`
}

type PatchRoleRequest struct {
	DisplayName *string  `json:"displayName,omitempty"`
	Description *string  `json:"description,omitempty"`
	GroupIDs    []string `json:"groupIds,omitempty"`
	Users       []string `json:"users,omitempty"`
	RoleIDs     []string `json:"roleIds,omitempty"`
	Domains     []string `json:"domains,omitempty"`
	IsEnabled   *bool    `json:"isEnabled,omitempty"`
	Revision    int64    `json:"revision,omitempty"`
}

func (r CreateRoleRequest) toCommand() approles.CreateRoleCommand {
	return approles.CreateRoleCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		Description:    r.Description,
		GroupIDs:       r.GroupIDs,
		Users:          r.Users,
		RoleIDs:        r.RoleIDs,
		Domains:        r.Domains,
		IsEnabled:      r.IsEnabled,
	}
}

func (r PatchRoleRequest) toCommand(id string) approles.PatchRoleCommand {
	command := approles.PatchRoleCommand{
		ID:          id,
		DisplayName: r.DisplayName,
		Description: r.Description,
		IsEnabled:   r.IsEnabled,
		Revision:    r.Revision,
	}
	if r.GroupIDs != nil {
		command.GroupIDs = r.GroupIDs
	}
	if r.Users != nil {
		command.Users = r.Users
	}
	if r.RoleIDs != nil {
		command.RoleIDs = r.RoleIDs
	}
	if r.Domains != nil {
		command.Domains = r.Domains
	}
	return command
}
