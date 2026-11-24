package invitations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateInvitationRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := CreateInvitationRequest{
			OrganizationID: "org-001",
			Email:          "newuser@example.com",
			RoleIDs:        []string{"role-001", "role-002"},
			GroupIDs:       []string{"group-001"},
			InvitedBy:      "admin-001",
			ValidDays:      3,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "newuser@example.com", cmd.Email)
		assert.Equal(t, []string{"role-001", "role-002"}, cmd.RoleIDs)
		assert.Equal(t, []string{"group-001"}, cmd.GroupIDs)
		assert.Equal(t, "admin-001", cmd.InvitedBy)
		assert.Equal(t, 3, cmd.ValidDays)
	})

	t.Run("empty_role_ids", func(t *testing.T) {
		req := CreateInvitationRequest{
			OrganizationID: "org-002",
			Email:          "test@example.com",
			RoleIDs:        nil,
			InvitedBy:      "admin-002",
		}

		cmd := req.toCommand()

		assert.Nil(t, cmd.RoleIDs)
	})
}
