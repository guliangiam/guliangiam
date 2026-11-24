package invitations

import (
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/invitations"
	"github.com/stretchr/testify/assert"
)

func TestToInvitation(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		now := time.Now()
		acceptedAt := now.Add(time.Hour)
		row := invitationRow{
			ID:             "inv-001",
			OrganizationID: "org-001",
			Email:          "user@example.com",
			Code:           "ABC123",
			RoleIDs:        []string{"role-001", "role-002"},
			GroupIDs:       []string{"group-001"},
			Status:         "pending",
			ExpiresAt:      now.Add(24 * time.Hour),
			AcceptedAt:     &acceptedAt,
			InvitedBy:      "admin-001",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}

		result := toInvitation(row)

		assert.Equal(t, domain.Invitation{
			ID:             "inv-001",
			OrganizationID: "org-001",
			Email:          "user@example.com",
			Code:           "ABC123",
			RoleIDs:        []string{"role-001", "role-002"},
			GroupIDs:       []string{"group-001"},
			Status:         "pending",
			ExpiresAt:      now.Add(24 * time.Hour),
			AcceptedAt:     &acceptedAt,
			InvitedBy:      "admin-001",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       1,
		}, result)
	})

	t.Run("empty_slices", func(t *testing.T) {
		now := time.Now()
		row := invitationRow{
			ID:             "inv-002",
			OrganizationID: "org-001",
			Email:          "test@example.com",
			Code:           "XYZ789",
			RoleIDs:        nil,
			GroupIDs:       nil,
			Status:         "expired",
			ExpiresAt:      now,
			AcceptedAt:     nil,
			InvitedBy:      "admin-002",
			CreatedAt:      now,
			UpdatedAt:      now,
			Revision:       2,
		}

		result := toInvitation(row)

		assert.Equal(t, "inv-002", result.ID)
		assert.Nil(t, result.RoleIDs)
		assert.Nil(t, result.GroupIDs)
		assert.Nil(t, result.AcceptedAt)
	})
}
