package auth

import (
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// tokenSigningAlgorithmForApplication is the single decision point for the
// JOSE algorithm an application's self-issued tokens carry. The application's
// token_signing_method is authoritative; an empty value keeps the historical
// RS256 default, exactly as the ID token path resolves it.
func tokenSigningAlgorithmForApplication(method string) string {
	if method == "" {
		return applicationdomain.TokenSigningMethodRS256
	}
	return method
}

// logoutTokenSigningAlgorithm resolves the algorithm for a back-channel logout
// token. The logout path has no downstream issuer to reject values that are
// outside the application enum, so it rejects them here instead of letting the
// signer fall back to RS256.
func logoutTokenSigningAlgorithm(method string) (string, error) {
	algorithm := tokenSigningAlgorithmForApplication(method)
	switch algorithm {
	case applicationdomain.TokenSigningMethodRS256, applicationdomain.TokenSigningMethodSM2SM3:
		return algorithm, nil
	}
	return "", apperrors.NewInvalidArgumentf(
		"application token_signing_method %q cannot sign a logout token; allowed: [%s %s]",
		method,
		applicationdomain.TokenSigningMethodRS256,
		applicationdomain.TokenSigningMethodSM2SM3,
	)
}
