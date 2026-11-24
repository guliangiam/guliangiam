package radiusconfigs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateRadiusConfigRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		isEnabled := true
		req := CreateRadiusConfigRequest{
			OrganizationID: "org-001",
			Name:           "main-radius",
			Host:           "radius.example.com",
			Port:           1812,
			SharedSecret:   "secret-abc",
			AuthProtocol:   "PAP",
			IsEnabled:      &isEnabled,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "main-radius", cmd.Name)
		assert.Equal(t, 1812, cmd.Port)
		assert.Equal(t, "PAP", cmd.AuthProtocol)
	})
}

func TestPatchRadiusConfigRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		name := "updated-radius"
		port := 1813
		req := PatchRadiusConfigRequest{
			Name:     &name,
			Port:     &port,
			Revision: 2,
		}

		cmd := req.toCommand("rc-001")

		assert.Equal(t, "rc-001", cmd.ID)
		assert.Equal(t, &name, cmd.Name)
		assert.Equal(t, &port, cmd.Port)
	})
}
