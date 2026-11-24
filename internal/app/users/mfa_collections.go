package users

import (
	"context"
	"fmt"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
)

// deriveUserMfaCollections recomputes the server-owned collections mirror of the
// real MFA factor state (the MfaItems / MfaAccounts JSONB entries on the user
// aggregate). Every binding / unbinding ceremony calls it inside its own
// transaction, right before the user row is written, so the mirror can never
// drift from the mfa-methods table and clients no longer hand-edit these
// pseudo-configurations.
//
// The organization is consulted only when an active TOTP factor needs a
// MfaAccount entry; issuer / accountName follow the exact same rules as
// InitiateUserMfaSetup so the mirror matches what authenticator apps scanned.
func deriveUserMfaCollections(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User) error {
	items := make([]userdomain.MfaItem, 0, len(user.MfaMethods))
	seen := make(map[string]struct{}, len(user.MfaMethods))
	totpActive := false
	for _, method := range user.MfaMethods {
		if !method.Enabled || !method.Verified {
			continue
		}
		itemType := normalizeExistingUserMfaMethodType(method.MethodType)
		if _, ok := seen[itemType]; ok {
			continue
		}
		seen[itemType] = struct{}{}
		items = append(items, userdomain.MfaItem{MfaType: itemType, Secret: mfaCollectionSecret(user, itemType)})
		if itemType == mfaMethodTOTP && user.TotpSecret != "" {
			totpActive = true
		}
	}
	user.MfaItems = items

	user.MfaAccounts = nil
	if totpActive {
		organization, err := tx.Organizations().GetByID(ctx, user.OrganizationID)
		if err != nil {
			return err
		}
		issuer, accountName := mfaTOTPAccountIdentity(user, organization)
		user.MfaAccounts = []userdomain.MfaAccount{{
			AccountName: accountName,
			Issuer:      issuer,
			SecretKey:   user.TotpSecret,
			Origin:      mfaMethodTOTP,
		}}
	}
	return nil
}

// mfaCollectionSecret maps an active factor type to the secret material stored on
// the user aggregate. Email / SMS factors carry no secret — their destination is
// profile data — so their entry stays type-only.
func mfaCollectionSecret(user *userdomain.User, itemType string) string {
	switch itemType {
	case mfaMethodTOTP:
		return user.TotpSecret
	case mfaMethodRadius:
		return user.MfaRadiusProvider
	case mfaMethodPush:
		return user.MfaPushProvider
	default:
		return ""
	}
}

// mfaTOTPAccountIdentity mirrors the issuer / accountName derivation of
// InitiateUserMfaSetup (the values baked into the provisioning URI).
func mfaTOTPAccountIdentity(user *userdomain.User, organization *orgdomain.Organization) (issuer, accountName string) {
	issuer = ""
	accountName = user.Username
	if organization != nil {
		if strings.TrimSpace(organization.DisplayName) != "" {
			issuer = organization.DisplayName
		} else {
			issuer = organization.Slug
		}
		accountName = fmt.Sprintf("%s/%s", organization.Slug, user.Username)
	}
	return issuer, accountName
}
