package groups

import appgroups "github.com/guliangiam/guliangiam/internal/app/groups"

type ListGroupsRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Status         string
	Type           string
	SortField      string
	SortOrder      string
}

type GetGroupRequest struct {
	ID string
}

type CreateGroupRequest struct {
	OrganizationID string   `json:"organizationId"`
	Name           string   `json:"name"`
	DisplayName    string   `json:"displayName"`
	Manager        string   `json:"manager,omitempty"`
	ContactEmail   string   `json:"contactEmail,omitempty"`
	Type           string   `json:"type,omitempty"`
	ParentID       string   `json:"parentId,omitempty"`
	IsTopGroup     bool     `json:"isTopGroup"`
	IsEnabled      bool     `json:"isEnabled"`
	Homepage       string   `json:"homepage,omitempty"`
	Users          []string `json:"users,omitempty"`
}

type PatchGroupRequest struct {
	Name         *string  `json:"name,omitempty"`
	DisplayName  *string  `json:"displayName,omitempty"`
	Manager      *string  `json:"manager,omitempty"`
	ContactEmail *string  `json:"contactEmail,omitempty"`
	Type         *string  `json:"type,omitempty"`
	ParentID     *string  `json:"parentId,omitempty"`
	IsTopGroup   *bool    `json:"isTopGroup,omitempty"`
	IsEnabled    *bool    `json:"isEnabled,omitempty"`
	Homepage     *string  `json:"homepage,omitempty"`
	Users        []string `json:"users,omitempty"`
	Revision     int64    `json:"revision,omitempty"`
}

func (r CreateGroupRequest) toCommand() appgroups.CreateGroupCommand {
	return appgroups.CreateGroupCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		DisplayName:    r.DisplayName,
		Manager:        r.Manager,
		ContactEmail:   r.ContactEmail,
		Type:           r.Type,
		ParentID:       r.ParentID,
		IsTopGroup:     r.IsTopGroup,
		IsEnabled:      r.IsEnabled,
		Homepage:       r.Homepage,
		Users:          r.Users,
	}
}

func (r PatchGroupRequest) toCommand(id string) appgroups.PatchGroupCommand {
	command := appgroups.PatchGroupCommand{
		ID:           id,
		Name:         r.Name,
		DisplayName:  r.DisplayName,
		Manager:      r.Manager,
		ContactEmail: r.ContactEmail,
		Type:         r.Type,
		ParentID:     r.ParentID,
		IsTopGroup:   r.IsTopGroup,
		IsEnabled:    r.IsEnabled,
		Homepage:     r.Homepage,
		Revision:     r.Revision,
	}
	if r.Users != nil {
		command.Users = r.Users
	}
	return command
}
