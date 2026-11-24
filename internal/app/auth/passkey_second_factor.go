package auth

import (
	"context"
	"log/slog"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
)

// loginMfaMethodPasskey is the virtual MFA method type exposed in the login
// response when the organization opts into passkey-as-second-factor. It is not
// a stored user_mfa_methods row: completion happens through the existing
// /auth/webauthn/authentication/{begin,finish} ceremony.
const (
	loginMfaMethodPasskey    = "passkey"
	loginMfaMethodPasskeyKey = "passkey"
)

// mfaMethodsWithPasskeySecondFactor appends a virtual passkey option to the
// stored MFA methods when—and only when—the organization enabled
// passkeyAsSecondFactor and the user has at least one active credential.
//
// The stored TOTP/email/SMS/push methods are never removed, so TOTP remains
// the fallback. Errors and missing repositories degrade to the original list:
// a passkey lookup problem must not block the login MFA challenge.
func (s *Service) mfaMethodsWithPasskeySecondFactor(ctx context.Context, tx appuow.UnitOfWork, organization *orgdomain.Organization, user *userdomain.User, methods []userdomain.UserMfaMethod) []userdomain.UserMfaMethod {
	if len(methods) == 0 || !passkeyAsSecondFactorEnabled(organization) || tx == nil {
		return methods
	}

	repo := tx.WebAuthnCredentials()
	if repo == nil {
		return methods
	}

	credentials, err := repo.ListByUserID(ctx, user.ID)
	if err != nil {
		slog.DebugContext(ctx, "passkey second factor lookup failed; keeping stored MFA methods",
			"user_id", user.ID, "error", err)
		return methods
	}

	for _, credential := range credentials {
		if credential == nil || credential.Status != webauthndomain.CredentialStatusActive {
			continue
		}
		return append(methods, userdomain.UserMfaMethod{
			ID:          loginMfaMethodPasskeyKey,
			MethodType:  loginMfaMethodPasskey,
			DisplayName: "Passkey",
			Enabled:     true,
			Verified:    true,
		})
	}

	return methods
}
