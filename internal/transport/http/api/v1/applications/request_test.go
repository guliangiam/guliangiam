package applications

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateApplicationRequest_ToCommand(t *testing.T) {
	t.Run("maps_basic_fields", func(t *testing.T) {
		isShared := true
		enableSignup := true

		req := CreateApplicationRequest{
			OrganizationID: "org-001",
			Slug:           "test-app",
			DisplayName:    "Test Application",
			Category:       "web",
			AppType:        "native",
			Status:         "active",
			ClientID:       "client-123",
			RedirectURIs:   []string{"https://app.example.com/callback"},
			GrantTypes:     []string{"authorization_code", "refresh_token"},
			IsShared:       &isShared,
			EnableSignup:   &enableSignup,
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "test-app", cmd.Slug)
		assert.Equal(t, "Test Application", cmd.DisplayName)
		assert.Equal(t, "active", cmd.Status)
		assert.Equal(t, []string{"authorization_code", "refresh_token"}, cmd.GrantTypes)
	})

	t.Run("maps_token_signing_method", func(t *testing.T) {
		req := CreateApplicationRequest{TokenSigningMethod: "SM2-SM3"}
		cmd := req.toCommand()
		assert.Equal(t, "SM2-SM3", cmd.TokenSigningMethod)
	})

	t.Run("empty_fields", func(t *testing.T) {
		req := CreateApplicationRequest{}
		cmd := req.toCommand()
		assert.Equal(t, "", cmd.OrganizationID)
		assert.Equal(t, "", cmd.Slug)
	})
}

func TestPatchApplicationRequest_ToCommand(t *testing.T) {
	t.Run("maps_patch_fields", func(t *testing.T) {
		displayName := "Updated App"
		status := "inactive"

		req := PatchApplicationRequest{
			DisplayName: &displayName,
			Status:      &status,
			Revision:    2,
		}

		cmd := req.toCommand("app-001")

		assert.Equal(t, "app-001", cmd.ID)
		assert.Equal(t, &displayName, cmd.DisplayName)
		assert.Equal(t, int64(2), cmd.Revision)
	})

	t.Run("nil_optional_fields", func(t *testing.T) {
		req := PatchApplicationRequest{}
		cmd := req.toCommand("app-002")
		assert.Equal(t, "app-002", cmd.ID)
		assert.Nil(t, cmd.DisplayName)
	})

	t.Run("forwards_nested_config_slices", func(t *testing.T) {
		req := PatchApplicationRequest{
			SignupItems:     json.RawMessage(`[{"name":"Email","type":"Input","visible":true,"required":true}]`),
			ProviderItems:   json.RawMessage(`[{"provider":"prov-001","canSignIn":true}]`),
			SigninMethods:   json.RawMessage(`["Password"]`),
			SigninItems:     json.RawMessage(`[{"name":"Username","visible":true}]`),
			SamlAttributes:  json.RawMessage(`[{"name":"email","value":"user.email"}]`),
			TokenAttributes: json.RawMessage(`[{"name":"role","sourceField":"user.role"}]`),
		}

		cmd := req.toCommand("app-003")

		assert.JSONEq(t, string(req.SignupItems), string(cmd.SignupItems))
		assert.JSONEq(t, string(req.ProviderItems), string(cmd.ProviderItems))
		assert.JSONEq(t, string(req.SigninMethods), string(cmd.SigninMethods))
		assert.JSONEq(t, string(req.SigninItems), string(cmd.SigninItems))
		assert.JSONEq(t, string(req.SamlAttributes), string(cmd.SamlAttributes))
		assert.JSONEq(t, string(req.TokenAttributes), string(cmd.TokenAttributes))
	})

	t.Run("absent_slices_stay_nil", func(t *testing.T) {
		cmd := PatchApplicationRequest{}.toCommand("app-004")
		assert.Nil(t, cmd.SignupItems)
		assert.Nil(t, cmd.ProviderItems)
		assert.Nil(t, cmd.SigninMethods)
		assert.Nil(t, cmd.SigninItems)
	})
}

func TestReplaceApplicationProviderBindingsRequest_ToCommands(t *testing.T) {
	t.Run("maps_items", func(t *testing.T) {
		req := ReplaceApplicationProviderBindingsRequest{
			Items: []ReplaceApplicationProviderBindingItemRequest{
				{ProviderID: "prov-001"},
				{ProviderID: "prov-002"},
			},
		}

		cmds := req.toCommands()
		assert.Len(t, cmds, 2)
		assert.Equal(t, "prov-001", cmds[0].ProviderID)
		assert.Equal(t, "prov-002", cmds[1].ProviderID)
	})
}

func TestCreateApplicationRequest_ToCommand_MapsClientCertificates(t *testing.T) {
	req := CreateApplicationRequest{
		Slug: "mtls-app",
		ClientCertificates: []ApplicationClientCertificateItemRequest{
			{FingerprintSHA256: "fp-1", Certificate: "pem-1", SelfSigned: true},
			{FingerprintSHA256: "fp-2", Certificate: "pem-2"},
		},
	}

	cmd := req.toCommand()

	if assert.Len(t, cmd.ClientCertificates, 2) {
		assert.Equal(t, "fp-1", cmd.ClientCertificates[0].FingerprintSHA256)
		assert.Equal(t, "pem-1", cmd.ClientCertificates[0].Certificate)
		assert.True(t, cmd.ClientCertificates[0].SelfSigned)
		assert.False(t, cmd.ClientCertificates[1].SelfSigned)
	}
}

func TestPatchApplicationRequest_ToCommand_MapsClientCertificates(t *testing.T) {
	t.Run("provided_list_is_preserved", func(t *testing.T) {
		items := []ApplicationClientCertificateItemRequest{
			{FingerprintSHA256: "fp-1", Certificate: "pem-1"},
		}
		req := PatchApplicationRequest{ClientCertificates: &items}

		cmd := req.toCommand("app-001")

		if assert.NotNil(t, cmd.ClientCertificates) {
			assert.Len(t, *cmd.ClientCertificates, 1)
			assert.Equal(t, "fp-1", (*cmd.ClientCertificates)[0].FingerprintSHA256)
			assert.Equal(t, "pem-1", (*cmd.ClientCertificates)[0].Certificate)
		}
	})

	t.Run("explicit_empty_list_means_replace_with_empty", func(t *testing.T) {
		items := []ApplicationClientCertificateItemRequest{}
		req := PatchApplicationRequest{ClientCertificates: &items}

		cmd := req.toCommand("app-001")

		if assert.NotNil(t, cmd.ClientCertificates, "空集合必须保留指针，否则会被当成不修改") {
			assert.Empty(t, *cmd.ClientCertificates)
		}
	})

	t.Run("absent_field_means_no_change", func(t *testing.T) {
		cmd := PatchApplicationRequest{}.toCommand("app-001")
		assert.Nil(t, cmd.ClientCertificates)
	})
}
