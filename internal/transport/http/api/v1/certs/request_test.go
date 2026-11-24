package certs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateCertRequest_ToCommand(t *testing.T) {
	t.Run("full_mapping", func(t *testing.T) {
		req := CreateCertRequest{
			OrganizationID:  "org-001",
			Name:            "my-cert",
			DisplayName:     "My Certificate",
			Scope:           "global",
			Type:            "x509",
			CryptoAlgorithm: "RSA",
			BitSize:         2048,
			ExpireInYears:   1,
			Certificate:     "-----BEGIN CERT-----",
			PrivateKey:      "-----BEGIN KEY-----",
			Domain:          []string{"example.com", "api.example.com"},
		}

		cmd := req.toCommand()

		assert.Equal(t, "org-001", cmd.OrganizationID)
		assert.Equal(t, "my-cert", cmd.Name)
		assert.Equal(t, "RSA", cmd.CryptoAlgorithm)
		assert.Equal(t, 2048, cmd.BitSize)
		assert.Equal(t, []string{"example.com", "api.example.com"}, cmd.Domain)
	})
}

func TestPatchCertRequest_ToCommand(t *testing.T) {
	t.Run("with_domain", func(t *testing.T) {
		displayName := "Updated Cert"
		req := PatchCertRequest{
			DisplayName: &displayName,
			Domain:      []string{"new.example.com"},
			Revision:    2,
		}

		cmd := req.toCommand("cert-001")

		assert.Equal(t, "cert-001", cmd.ID)
		assert.Equal(t, []string{"new.example.com"}, cmd.Domain)
	})

	t.Run("nil_domain", func(t *testing.T) {
		req := PatchCertRequest{
			Domain: nil,
		}

		cmd := req.toCommand("cert-002")

		assert.Nil(t, cmd.Domain)
	})
}
