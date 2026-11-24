package invitations

import (
	"time"

	appinvitations "github.com/guliangiam/guliangiam/internal/app/invitations"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

type InvitationResponse struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organizationId"`
	Email          string     `json:"email"`
	Code           string     `json:"code"`
	RoleIDs        []string   `json:"roleIds"`
	GroupIDs       []string   `json:"groupIds"`
	Status         string     `json:"status"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	AcceptedAt     *time.Time `json:"acceptedAt,omitempty"`
	InvitedBy      string     `json:"invitedBy"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
	Revision       int64      `json:"revision"`
}

func toResponse(invitation *appinvitations.InvitationDTO) *InvitationResponse {
	// 空集合必须序列化为 []：nil slice 会输出 null，前端 zod 契约要求数组。
	roleIDs := append([]string{}, invitation.RoleIDs...)
	groupIDs := append([]string{}, invitation.GroupIDs...)
	return &InvitationResponse{
		ID:             invitation.ID,
		OrganizationID: invitation.OrganizationID,
		Email:          invitation.Email,
		Code:           invitation.Code,
		RoleIDs:        roleIDs,
		GroupIDs:       groupIDs,
		Status:         invitation.Status,
		ExpiresAt:      invitation.ExpiresAt,
		AcceptedAt:     invitation.AcceptedAt,
		InvitedBy:      invitation.InvitedBy,
		CreatedAt:      invitation.CreatedAt,
		UpdatedAt:      invitation.UpdatedAt,
		Revision:       invitation.Revision,
	}
}

type ListInvitationsResponse = presenter.ListResponse[*appinvitations.InvitationDTO]

type CreateInvitationResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
