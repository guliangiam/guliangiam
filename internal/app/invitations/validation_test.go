package invitations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateCreateInvitationCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateInvitationCommand{
			OrganizationID: "org-001",
			Email:          "user@example.com",
		}
		assert.NoError(t, validateCreateInvitationCommand(cmd))
	})

	t.Run("missing_organization_id", func(t *testing.T) {
		cmd := CreateInvitationCommand{Email: "user@example.com"}
		assert.Error(t, validateCreateInvitationCommand(cmd))
	})

	t.Run("missing_email", func(t *testing.T) {
		cmd := CreateInvitationCommand{OrganizationID: "org-001"}
		assert.Error(t, validateCreateInvitationCommand(cmd))
	})

	t.Run("whitespace_only_email", func(t *testing.T) {
		cmd := CreateInvitationCommand{
			OrganizationID: "org-001",
			Email:          "   ",
		}
		assert.Error(t, validateCreateInvitationCommand(cmd))
	})
}
