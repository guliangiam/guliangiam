package users

import (
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

// Passkey policy keys live in organization.Security.MfaPolicy. They mirror the
// auth service's WebAuthn policy so both ceremony paths agree on tenant policy.
// They are optional: an absent key keeps the historical behaviour.

// passkeyEnabled reports whether the organization allows WebAuthn ceremonies.
// Unknown/malformed values default to true so a typo cannot silently lock an
// entire tenant out of its enrolled passkeys.
func passkeyEnabled(organization *orgdomain.Organization) bool {
	if organization == nil {
		return true
	}
	if enabled, ok := platformwebauthn.PolicyBool(organization.Security.MfaPolicy,
		platformwebauthn.PolicyKeyPasskeyEnabled, platformwebauthn.PolicyKeyWebAuthnEnabled); ok {
		return enabled
	}
	return true
}

// webAuthnUVRequired merges the deployment-level switch with the organization
// policy. The deployment switch can only tighten the requirement; an
// organization can tighten it further but can never relax it.
func webAuthnUVRequired(organization *orgdomain.Organization) bool {
	if organization == nil {
		return platformwebauthn.DeployRequiresUV()
	}
	return platformwebauthn.RequiresUserVerification(platformwebauthn.DeployRequiresUV(), organization.Security.MfaPolicy)
}
