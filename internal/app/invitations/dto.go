package invitations

import (
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
)

// InvitationDTO is the data transfer object returned by the service layer,
// decoupling consumers from the domain entity.
type InvitationDTO struct {
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

func toInvitationDTO(inv *domain.Invitation) *InvitationDTO {
	if inv == nil {
		return nil
	}
	return &InvitationDTO{
		ID:             inv.ID,
		OrganizationID: inv.OrganizationID,
		Email:          inv.Email,
		Code:           inv.Code,
		// 空集合序列化为 []：nil slice 输出 null 会破坏前端数组契约。
		RoleIDs:    append([]string{}, inv.RoleIDs...),
		GroupIDs:   append([]string{}, inv.GroupIDs...),
		Status:     inv.Status,
		ExpiresAt:  inv.ExpiresAt,
		AcceptedAt: inv.AcceptedAt,
		InvitedBy:  inv.InvitedBy,
		CreatedAt:  inv.CreatedAt,
		UpdatedAt:  inv.UpdatedAt,
		Revision:   inv.Revision,
	}
}
