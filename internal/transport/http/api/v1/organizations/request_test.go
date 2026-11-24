package organizations

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateOrganizationRequest_ToCommand(t *testing.T) {
	t.Run("maps_basic_fields", func(t *testing.T) {
		req := CreateOrganizationRequest{
			Slug:                 "test-org",
			DisplayName:          "Test Organization",
			WebsiteURL:           "https://example.com",
			Status:               "active",
			Languages:            []string{"en", "zh-CN"},
			InitScore:            100,
			DefaultApplicationID: "app-1",
		}

		cmd := req.toCommand()

		assert.Equal(t, "test-org", cmd.Slug)
		assert.Equal(t, "Test Organization", cmd.DisplayName)
		assert.Equal(t, "https://example.com", cmd.WebsiteURL)
		assert.Equal(t, "active", cmd.Status)
		assert.Equal(t, []string{"en", "zh-CN"}, cmd.Languages)
		assert.Equal(t, 100, cmd.InitScore)
		assert.Equal(t, "app-1", cmd.DefaultApplicationID)
	})

	t.Run("empty_fields", func(t *testing.T) {
		req := CreateOrganizationRequest{}
		cmd := req.toCommand()
		assert.Equal(t, "", cmd.Slug)
		assert.Equal(t, "", cmd.DisplayName)
	})
}

func TestPatchOrganizationRequest_ToCommand(t *testing.T) {
	t.Run("maps_patch_fields_with_id", func(t *testing.T) {
		displayName := "Updated Org"
		websiteURL := "https://updated.example.com"
		status := "inactive"
		disableSignin := true
		initScore := 100

		req := PatchOrganizationRequest{
			DisplayName:   &displayName,
			WebsiteURL:    &websiteURL,
			Status:        &status,
			DisableSignin: &disableSignin,
			InitScore:     &initScore,
			Revision:      2,
		}

		cmd := req.toCommand("org-001")

		assert.Equal(t, "org-001", cmd.ID)
		assert.Equal(t, &displayName, cmd.DisplayName)
		assert.Equal(t, &disableSignin, cmd.DisableSignin)
		assert.Equal(t, int64(2), cmd.Revision)
	})

	t.Run("nil_optional_fields", func(t *testing.T) {
		req := PatchOrganizationRequest{}
		cmd := req.toCommand("org-002")
		assert.Equal(t, "org-002", cmd.ID)
		assert.Nil(t, cmd.DisplayName)
		assert.Nil(t, cmd.Status)
	})
}
