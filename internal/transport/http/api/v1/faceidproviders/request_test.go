package faceidproviders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFaceIdProviderRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		isEnabled := true
		req := CreateFaceIdProviderRequest{
			OrganizationID:  "org-001",
			Name:            "aliyun-face",
			ProviderType:    "aliyun",
			AccessKeyID:     "ak-123",
			AccessKeySecret: "sk-456",
			Endpoint:        "https://faceid.aliyuncs.com",
			Config:          map[string]any{"confidence": 0.95},
			IsEnabled:       &isEnabled,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "aliyun-face", cmd.Name)
		assert.Equal(t, "ak-123", cmd.AccessKeyID)
	})

	t.Run("nil_config", func(t *testing.T) {
		req := CreateFaceIdProviderRequest{
			OrganizationID: "org-001",
			Name:           "test",
			Config:         nil,
		}

		cmd := req.toCommand()

		assert.Nil(t, cmd.Config)
	})
}

func TestPatchFaceIdProviderRequest_ToCommand(t *testing.T) {
	t.Run("with_config", func(t *testing.T) {
		name := "updated-face"
		req := PatchFaceIdProviderRequest{
			Name:   &name,
			Config: map[string]any{"liveness": true},
		}

		cmd := req.toCommand("fid-001")

		assert.Equal(t, "fid-001", cmd.ID)
		assert.Equal(t, true, cmd.Config["liveness"])
	})

	t.Run("nil_config", func(t *testing.T) {
		req := PatchFaceIdProviderRequest{
			Config: nil,
		}

		cmd := req.toCommand("fid-002")

		assert.Nil(t, cmd.Config)
	})
}
