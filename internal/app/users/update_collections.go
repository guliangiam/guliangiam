package users

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Collection input shapes exposed to the transport layer. Addresses, cart,
// face ids and consent records are plain aliases of the domain structs; the
// secret-bearing entries need a wire shape that also accepts the masked
// projection a client received from the read API (passwordSet/passwordMasked
// and friends). Those masked fields are accepted for round-tripping and then
// dropped — only a non-empty plaintext secret is ever persisted.
//
// mfaItems / mfaAccounts are deliberately absent: they are server-owned
// derivations of the real factor state (see deriveUserMfaCollections) and any
// such key arriving on the write path is ignored.
type (
	AddressInput       = userdomain.Address
	FaceIDInput        = userdomain.FaceId
	ProductInfoInput   = userdomain.ProductInfo
	ConsentRecordInput = userdomain.ConsentRecord
)

// ManagedAccountInput is the write shape of one managed account entry.
type ManagedAccountInput struct {
	Application    string `json:"application"`
	Username       string `json:"username"`
	Password       string `json:"password,omitempty"`
	PasswordSet    bool   `json:"passwordSet,omitempty"`
	PasswordMasked string `json:"passwordMasked,omitempty"`
	SigninUrl      string `json:"signinUrl"`
}

// UserCollectionsInput is the full-replacement payload of the collections
// endpoint. MFA entries are not writable here; the server derives them from the
// mfa-methods table during every binding / unbinding ceremony.
type UserCollectionsInput struct {
	Addresses         []AddressInput        `json:"addresses"`
	ManagedAccounts   []ManagedAccountInput `json:"managedAccounts"`
	FaceIds           []FaceIDInput         `json:"faceIds"`
	Cart              []ProductInfoInput    `json:"cart"`
	ApplicationScopes []ConsentRecordInput  `json:"applicationScopes"`
}

// toDomain drops the masked projections and keeps only writable plaintext.
func (in UserCollectionsInput) toDomain() userdomain.UserCollections {
	collections := userdomain.UserCollections{
		Addresses:         in.Addresses,
		FaceIds:           in.FaceIds,
		Cart:              in.Cart,
		ApplicationScopes: in.ApplicationScopes,
	}
	for _, account := range in.ManagedAccounts {
		collections.ManagedAccounts = append(collections.ManagedAccounts, userdomain.ManagedAccount{
			Application: account.Application,
			Username:    account.Username,
			Password:    account.Password,
			SigninUrl:   account.SigninUrl,
		})
	}
	return collections
}

// UpdateUserCollectionsCommand replaces the writable JSONB collections of one
// user.
//
// Collections are deliberately a full replacement: the detail page keeps a local
// draft and submits the whole array. Secrets (managed account passwords) are
// write-only — an empty value keeps the stored one so a client that only
// received the masked projection cannot wipe credentials. The server-owned
// mfaItems / mfaAccounts entries are always carried over from the stored user,
// never taken from the payload.
type UpdateUserCollectionsCommand struct {
	OrganizationID string
	UserID         string
	Collections    UserCollectionsInput
}

const (
	maxUserAddresses        = 20
	maxUserManagedAccounts  = 50
	maxUserCartItems        = 100
	maxUserApplicationScope = 100
	maxAddressFieldLength   = 128
	maxStreetLength         = 256
	maxPostalCodeLength     = 32
	maxAccountFieldLength   = 128
	maxScopeCount           = 32
)

func (s *Service) UpdateUserCollections(ctx context.Context, command UpdateUserCollectionsCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if strings.TrimSpace(command.UserID) == "" {
		return apperrors.NewInvalidArgument("userId is required")
	}
	incoming := command.Collections.toDomain()
	if err := validateUserCollections(incoming); err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// Resolve ownership first: a foreign user must stay untouched.
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		collections := mergeCollectionSecrets(userdomain.UserCollections{
			Addresses:         user.Addresses,
			ManagedAccounts:   user.ManagedAccounts,
			FaceIds:           user.FaceIds,
			Cart:              user.Cart,
			MfaItems:          user.MfaItems,
			MfaAccounts:       user.MfaAccounts,
			ApplicationScopes: user.ApplicationScopes,
		}, incoming)

		// mfaItems / mfaAccounts are server-owned derivations of the real factor
		// state; the payload cannot add, edit or clear them.
		collections.MfaItems = user.MfaItems
		collections.MfaAccounts = user.MfaAccounts

		if err := tx.Users().UpdateCollectionsForOrganization(ctx, command.UserID, command.OrganizationID, collections); err != nil {
			return err
		}

		return s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.updated",
			"User",
			user.ID,
			"user",
			"user.collections.updated",
			map[string]any{},
		)
	})
}

// mergeCollectionSecrets keeps stored secrets when the incoming entry omits them.
// Entries are matched by their natural business key because the JSONB shape has
// no stable id for managed accounts.
func mergeCollectionSecrets(existing, incoming userdomain.UserCollections) userdomain.UserCollections {
	managedPasswords := make(map[string]string, len(existing.ManagedAccounts))
	for _, account := range existing.ManagedAccounts {
		managedPasswords[account.Application+"\x00"+account.Username] = account.Password
	}
	for i := range incoming.ManagedAccounts {
		if incoming.ManagedAccounts[i].Password != "" {
			continue
		}
		key := incoming.ManagedAccounts[i].Application + "\x00" + incoming.ManagedAccounts[i].Username
		if password, ok := managedPasswords[key]; ok {
			incoming.ManagedAccounts[i].Password = password
		}
	}

	return incoming
}

func validateUserCollections(collections userdomain.UserCollections) error {
	if len(collections.Addresses) > maxUserAddresses {
		return collectionLimitError("addresses", maxUserAddresses)
	}
	defaultAddresses := 0
	for index, address := range collections.Addresses {
		if strings.TrimSpace(address.Street) == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "addresses.street is required", map[string]any{metaKeyIndex: index, metaKeyField: "addresses.street"})
		}
		if len(address.Street) > maxStreetLength {
			return collectionFieldTooLong(index, "addresses.street", maxStreetLength)
		}
		if address.CountryCode != "" && !isISO3166Alpha2(address.CountryCode) {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "addresses.countryCode must be a two-letter ISO 3166-1 code", map[string]any{metaKeyIndex: index, metaKeyField: "addresses.countryCode"})
		}
		for field, value := range map[string]string{
			"addresses.province": address.Province,
			"addresses.city":     address.City,
			"addresses.district": address.District,
			"addresses.details":  address.Details,
			"addresses.region":   address.Region,
		} {
			if len(value) > maxAddressFieldLength {
				return collectionFieldTooLong(index, field, maxAddressFieldLength)
			}
		}
		if len(address.PostalCode) > maxPostalCodeLength {
			return collectionFieldTooLong(index, "addresses.postalCode", maxPostalCodeLength)
		}
		if address.IsDefault {
			defaultAddresses++
		}
	}
	if defaultAddresses > 1 {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "at most one address can be the default", map[string]any{metaKeyField: "addresses.isDefault"})
	}

	if len(collections.ManagedAccounts) > maxUserManagedAccounts {
		return collectionLimitError("managedAccounts", maxUserManagedAccounts)
	}
	for index, account := range collections.ManagedAccounts {
		if strings.TrimSpace(account.Application) == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "managedAccounts.application is required", map[string]any{metaKeyIndex: index, metaKeyField: "managedAccounts.application"})
		}
		if len(account.Application) > maxAccountFieldLength || len(account.Username) > maxAccountFieldLength {
			return collectionFieldTooLong(index, "managedAccounts", maxAccountFieldLength)
		}
	}

	if len(collections.Cart) > maxUserCartItems {
		return collectionLimitError("cart", maxUserCartItems)
	}
	for index, item := range collections.Cart {
		if strings.TrimSpace(item.ProductID) == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "cart.productId is required", map[string]any{metaKeyIndex: index, metaKeyField: "cart.productId"})
		}
		if item.Quantity <= 0 {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "cart.quantity must be greater than zero", map[string]any{metaKeyIndex: index, metaKeyField: "cart.quantity"})
		}
		if item.Price < 0 {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "cart.price must not be negative", map[string]any{metaKeyIndex: index, metaKeyField: "cart.price"})
		}
	}

	if len(collections.ApplicationScopes) > maxUserApplicationScope {
		return collectionLimitError("applicationScopes", maxUserApplicationScope)
	}
	for index, scope := range collections.ApplicationScopes {
		if strings.TrimSpace(scope.Application) == "" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "applicationScopes.application is required", map[string]any{metaKeyIndex: index, metaKeyField: "applicationScopes.application"})
		}
		if len(scope.Scopes) > maxScopeCount {
			return collectionLimitError("applicationScopes.scopes", maxScopeCount)
		}
	}

	return nil
}

func collectionLimitError(field string, max int) error {
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, field+" accepts more entries than allowed", map[string]any{metaKeyField: field, "max": max})
}

func collectionFieldTooLong(index int, field string, max int) error {
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, field+" is too long", map[string]any{metaKeyIndex: index, metaKeyField: field, "max": max})
}

func isISO3166Alpha2(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 2 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}
