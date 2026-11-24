package certs

import (
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

func TestValidateCreateCertCommand(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		cmd := CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert"}
		assert.NoError(t, validateCreateCertCommand(cmd))
	})
	t.Run("valid_with_scope_and_type", func(t *testing.T) {
		cmd := CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert", Scope: "JWT", Type: "x509", CryptoAlgorithm: "RS256"}
		assert.NoError(t, validateCreateCertCommand(cmd))
	})
	t.Run("missing_org", func(t *testing.T) {
		cmd := CreateCertCommand{Name: "cert", DisplayName: "My Cert"}
		assert.Error(t, validateCreateCertCommand(cmd))
	})
	t.Run("invalid_scope", func(t *testing.T) {
		cmd := CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert", Scope: "invalid"}
		assert.Error(t, validateCreateCertCommand(cmd))
	})
	t.Run("invalid_type", func(t *testing.T) {
		cmd := CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert", Type: "invalid"}
		assert.Error(t, validateCreateCertCommand(cmd))
	})
	t.Run("invalid_crypto_algorithm", func(t *testing.T) {
		cmd := CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert", CryptoAlgorithm: "invalid"}
		assert.Error(t, validateCreateCertCommand(cmd))
	})
}

func TestValidatePatchCertCommand(t *testing.T) {
	t.Run("valid_nil", func(t *testing.T) {
		assert.NoError(t, validatePatchCertCommand(PatchCertCommand{}))
	})
	t.Run("invalid_scope", func(t *testing.T) {
		invalid := "invalid"
		assert.Error(t, validatePatchCertCommand(PatchCertCommand{Scope: &invalid}))
	})
	t.Run("invalid_crypto", func(t *testing.T) {
		invalid := "invalid"
		assert.Error(t, validatePatchCertCommand(PatchCertCommand{CryptoAlgorithm: &invalid}))
	})
	t.Run("valid_scope", func(t *testing.T) {
		scope := "JWT"
		assert.NoError(t, validatePatchCertCommand(PatchCertCommand{Scope: &scope}))
	})
}

func TestCertValidation_Answers422(t *testing.T) {
	badScope, badType, badAlgo := "bogus", "bogus", "bogus"
	tests := []struct {
		name string
		err  error
	}{
		{"create_scope", validateCreateCertCommand(CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert", Scope: badScope})},
		{"create_type", validateCreateCertCommand(CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert", Type: badType})},
		{"create_crypto", validateCreateCertCommand(CreateCertCommand{OrganizationID: "org-001", Name: "cert", DisplayName: "My Cert", CryptoAlgorithm: badAlgo})},
		{"patch_scope", validatePatchCertCommand(PatchCertCommand{Scope: &badScope})},
		{"patch_type", validatePatchCertCommand(PatchCertCommand{Type: &badType})},
		{"patch_crypto", validatePatchCertCommand(PatchCertCommand{CryptoAlgorithm: &badAlgo})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var appErr *apperrors.Error
			if !errors.As(tt.err, &appErr) {
				t.Fatalf("expected *apperrors.Error, got %T (%v)", tt.err, tt.err)
			}
			assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
			assert.NotNil(t, appErr.Meta["field"])
			assert.NotNil(t, appErr.Meta["allowed"])
		})
	}
}
