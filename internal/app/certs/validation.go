package certs

import (
	"slices"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// The closed sets below are the single source for both the accept/reject rule
// and the 422 `allowed` metadata.
var validScopes = []string{"JWT", certTypeSSL}
var validTypes = []string{"x509", certTypeSSL}
var validCryptoAlgorithms = []string{
	"RS256", "RS384", "RS512",
	"ES256", "ES384", "ES512",
	"PS256", "PS384", "PS512",
}

func validateCreateCertCommand(command CreateCertCommand) error {
	if strings.TrimSpace(command.OrganizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(command.Name) == "" {
		return apperrors.NewInvalidArgument("name is required")
	}
	if strings.TrimSpace(command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName is required")
	}
	if command.Scope != "" && !slices.Contains(validScopes, command.Scope) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid scope, must be JWT or SSL", map[string]any{"field": "scope", "value": command.Scope, "allowed": append([]string{}, validScopes...)})
	}
	if command.Type != "" && !slices.Contains(validTypes, command.Type) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid type, must be x509 or SSL", map[string]any{"field": "type", "value": command.Type, "allowed": append([]string{}, validTypes...)})
	}
	if command.CryptoAlgorithm != "" && !slices.Contains(validCryptoAlgorithms, command.CryptoAlgorithm) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid cryptoAlgorithm", map[string]any{"field": "cryptoAlgorithm", "value": command.CryptoAlgorithm, "allowed": append([]string{}, validCryptoAlgorithms...)})
	}
	return nil
}

func validatePatchCertCommand(command PatchCertCommand) error {
	if command.DisplayName != nil && strings.TrimSpace(*command.DisplayName) == "" {
		return apperrors.NewInvalidArgument("displayName must not be blank")
	}
	if command.Scope != nil && !slices.Contains(validScopes, *command.Scope) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid scope, must be JWT or SSL", map[string]any{"field": "scope", "value": *command.Scope, "allowed": append([]string{}, validScopes...)})
	}
	if command.Type != nil && !slices.Contains(validTypes, *command.Type) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid type, must be x509 or SSL", map[string]any{"field": "type", "value": *command.Type, "allowed": append([]string{}, validTypes...)})
	}
	if command.CryptoAlgorithm != nil && !slices.Contains(validCryptoAlgorithms, *command.CryptoAlgorithm) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid cryptoAlgorithm", map[string]any{"field": "cryptoAlgorithm", "value": *command.CryptoAlgorithm, "allowed": append([]string{}, validCryptoAlgorithms...)})
	}
	return nil
}
