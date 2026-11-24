package resources

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateResourceRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := CreateResourceRequest{
			OrganizationID: "org-001",
			ApplicationID:  "app-001",
			Name:           "my-api",
			DisplayName:    "My API",
			Type:           "api",
			Tag:            "v1",
			FilePath:       "/resources/api.json",
			FileSize:       2048,
			MimeType:       "application/json",
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "app-001", cmd.ApplicationID)
		assert.Equal(t, "my-api", cmd.Name)
		assert.Equal(t, int64(2048), cmd.FileSize)
	})
}

func TestPatchResourceRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		displayName := "Updated Resource"
		fileSize := int64(4096)
		req := PatchResourceRequest{
			DisplayName: &displayName,
			FileSize:    &fileSize,
			Revision:    2,
		}

		cmd := req.toCommand("res-001")

		assert.Equal(t, "res-001", cmd.ID)
		assert.Equal(t, &displayName, cmd.DisplayName)
		assert.Equal(t, &fileSize, cmd.FileSize)
		assert.Equal(t, int64(2), cmd.Revision)
	})
}

func TestUploadResourceCommand_ToServiceCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		reader := strings.NewReader("file content")
		req := UploadResourceCommand{
			Owner:    "org-001",
			User:     "user-001",
			Tag:      "upload",
			FileName: "test.txt",
			FileSize: 12,
			MimeType: "text/plain",
			Reader:   reader,
		}

		cmd := req.toServiceCommand()

		assert.Equal(t, "org-001", cmd.Owner)
		assert.Equal(t, "user-001", cmd.User)
		assert.Equal(t, "test.txt", cmd.FileName)
		assert.Equal(t, int64(12), cmd.FileSize)
		assert.NotNil(t, cmd.Reader)
	})
}
