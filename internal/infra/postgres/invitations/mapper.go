package invitations

import (
	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
)

func toInvitation(row invitationRow) domain.Invitation {
	return domain.Invitation{
		ID:             row.ID,
		OrganizationID: row.OrganizationID,
		Email:          row.Email,
		Code:           row.Code,
		RoleIDs:        append([]string(nil), row.RoleIDs...),
		GroupIDs:       append([]string(nil), row.GroupIDs...),
		Status:         row.Status,
		ExpiresAt:      row.ExpiresAt,
		AcceptedAt:     row.AcceptedAt,
		InvitedBy:      row.InvitedBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		Revision:       row.Revision,
	}
}
