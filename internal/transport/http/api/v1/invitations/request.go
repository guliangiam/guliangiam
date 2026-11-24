package invitations

import appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"

type ListInvitationsRequest struct {
	Page           int    `json:"page"`
	PageSize       int    `json:"pageSize"`
	OrganizationID string `json:"organizationId"`
	Keyword        string `json:"keyword"`
	Status         string `json:"status"`
}

type CreateInvitationRequest struct {
	OrganizationID string   `json:"organizationId"`
	Email          string   `json:"email"`
	RoleIDs        []string `json:"roleIds"`
	GroupIDs       []string `json:"groupIds"`
	InvitedBy      string   `json:"invitedBy"`
	ValidDays      int      `json:"validDays"`
}

func (r CreateInvitationRequest) toCommand() appinvitations.CreateInvitationCommand {
	return appinvitations.CreateInvitationCommand{
		OrganizationID: r.OrganizationID,
		Email:          r.Email,
		RoleIDs:        r.RoleIDs,
		GroupIDs:       r.GroupIDs,
		InvitedBy:      r.InvitedBy,
		ValidDays:      r.ValidDays,
	}
}

type RevokeInvitationRequest struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
}

type ResendInvitationRequest struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
}
