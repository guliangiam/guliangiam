package auth

import (
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

// policyKeyPasskeyAsSecondFactor opts a tenant into offering an enrolled
// passkey as an MFA second factor at login. Absent or false keeps the
// historical TOTP-only behaviour, so the switch is opt-in per organization.
const policyKeyPasskeyAsSecondFactor = "passkeyAsSecondFactor"

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
func (s *Service) webAuthnUVRequired(organization *orgdomain.Organization) bool {
	if organization == nil {
		return s.WebAuthnRequireUV
	}
	return platformwebauthn.RequiresUserVerification(s.WebAuthnRequireUV, organization.Security.MfaPolicy)
}

// passkeyAsSecondFactorEnabled reports whether the organization allows an
// enrolled passkey to complete the MFA step at login. It defaults to false and
// is additionally gated by passkeyEnabled: a tenant that disabled WebAuthn
// outright cannot re-enable it through the second-factor switch.
func passkeyAsSecondFactorEnabled(organization *orgdomain.Organization) bool {
	if organization == nil || !passkeyEnabled(organization) {
		return false
	}
	enabled, ok := platformwebauthn.PolicyBool(organization.Security.MfaPolicy, policyKeyPasskeyAsSecondFactor)
	return ok && enabled
}
