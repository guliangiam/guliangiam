package users

import (
	"errors"
	"log/slog"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	domain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// userEncryptedMapper seals user_mfa_config.totp_secret (DB-058 confidential column).
var userEncryptedMapper = shared.NewEncryptedFieldMapper()

// user_mfa_config is keyed by user_id, not by an "id" column.
const (
	userMfaConfigTable = "guliangiam.user_mfa_config"
	userMfaColumnTotp  = "totp_secret"
)

// user_profiles document-number columns (V19 `_sealed` twins; 规范 9.5 要求
// 证件号 AES-256-GCM 加密存储，明文列在清除迁移获批前保持双写）。
const (
	userProfilesTable                = "guliangiam.user_profiles"
	userProfileColumnIDCard          = "id_card"
	userProfileColumnPassportNo      = "passport_no"
	userProfileColumnDriverLicenseNo = "driver_license_no"
)

func userMfaAAD(userID string) crypto.AADContext {
	return crypto.AADContext{Table: userMfaConfigTable, Column: userMfaColumnTotp, PKValue: userID}
}

// sealTotpSecret encrypts the TOTP seed before the user_mfa_config upsert; an
// empty seed stays empty because "MFA not configured" is the empty string.
func sealTotpSecret(userID, plaintext string) (string, error) {
	if plaintext == "" || !userEncryptedMapper.IsEnabled() {
		return plaintext, nil
	}
	sealed, err := userEncryptedMapper.EncryptColumn(userMfaAAD(userID), plaintext)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "encrypt user_mfa_config.totp_secret for user %s", userID)
	}
	return sealed, nil
}

// openTotpSecret decrypts the stored TOTP seed. A pre-GM-G2 plaintext value
// fails closed (non-envelope) and points at the backfill.
func openTotpSecret(userID, stored string) (string, error) {
	if stored == "" || !userEncryptedMapper.IsEnabled() {
		return stored, nil
	}
	plaintext, err := userEncryptedMapper.DecryptEnvelopeColumn(userMfaAAD(userID), stored)
	if err == nil {
		return plaintext, nil
	}
	if errors.Is(err, crypto.ErrNotEnvelope) {
		return "", apperrors.NewInternalf(
			"user_mfa_config.totp_secret for user %s is not envelope-encrypted; run the MFA secret backfill before reading users", userID)
	}
	return "", apperrors.WrapInternalf(err, "decrypt user_mfa_config.totp_secret for user %s", userID)
}

// userProfileSealedColumn resolves the V19 mapping for one legacy document-number
// column. Target and PK come from the registry so the write/read AAD can never
// drift from `migrate backfill-pii-sealed-columns`.
func userProfileSealedColumn(sourceColumn string) (crypto.LegacySealedColumn, error) {
	for _, col := range crypto.LegacySealedColumns {
		if col.Table == userProfilesTable && col.SourceColumn == sourceColumn {
			return col, nil
		}
	}
	return crypto.LegacySealedColumn{}, apperrors.NewInternalf(
		"no sealed target registered for %s.%s", userProfilesTable, sourceColumn)
}

// sealProfileDocument writes the v2 twin of one legacy document-number column.
// An empty value stays unsealed (NULL twin = "no document number").
func sealProfileDocument(sourceColumn, userID, plaintext string) (*string, error) {
	if plaintext == "" {
		return nil, nil
	}
	col, err := userProfileSealedColumn(sourceColumn)
	if err != nil {
		return nil, err
	}
	sealed, err := userEncryptedMapper.EncryptColumn(col.AAD(userID), plaintext)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "encrypt user_profiles %s", col.TargetColumn)
	}
	return &sealed, nil
}

// sealProfileDocuments returns the three sealed twins for one profile write.
func sealProfileDocuments(userID string, profile *domain.UserProfile) (idCard, passportNo, driverLicenseNo *string, err error) {
	if idCard, err = sealProfileDocument(userProfileColumnIDCard, userID, profile.IDCard); err != nil {
		return nil, nil, nil, err
	}
	if passportNo, err = sealProfileDocument(userProfileColumnPassportNo, userID, profile.PassportNo); err != nil {
		return nil, nil, nil, err
	}
	if driverLicenseNo, err = sealProfileDocument(userProfileColumnDriverLicenseNo, userID, profile.DriverLicenseNo); err != nil {
		return nil, nil, nil, err
	}
	return idCard, passportNo, driverLicenseNo, nil
}

// openProfileDocument prefers the sealed twin. A NULL/empty twin is the
// backfill window: the legacy plaintext is returned and a WARN marks the row
// for `migrate backfill-pii-sealed-columns`. A sealed value that cannot be
// authenticated is an error and never silently downgrades to plaintext.
func openProfileDocument(sourceColumn, userID, plaintext string, sealed *string) (string, error) {
	if sealed == nil || *sealed == "" {
		if plaintext != "" {
			slog.Warn("user_profiles document number is not sealed; run `migrate backfill-pii-sealed-columns`",
				"user_id", userID, "column", sourceColumn)
		}
		return plaintext, nil
	}
	col, err := userProfileSealedColumn(sourceColumn)
	if err != nil {
		return "", err
	}
	opened, err := userEncryptedMapper.DecryptEnvelopeColumn(col.AAD(userID), *sealed)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "decrypt user_profiles %s", col.TargetColumn)
	}
	return opened, nil
}

func toUser(row userRow) (*domain.User, error) {
	user := &domain.User{
		ID:                  row.ID,
		OrganizationID:      row.OrganizationID,
		Username:            row.Username,
		ExternalID:          row.ExternalID,
		UserType:            row.UserType,
		Status:              row.Status,
		IsAdmin:             row.IsAdmin,
		IsForbidden:         row.IsForbidden,
		IsVerified:          row.IsVerified,
		IsOnline:            row.IsOnline,
		CreatedIp:           row.CreatedIp,
		SignupApplicationID: row.SignupApplicationID,
		RegisterType:        row.RegisterType,
		RegisterSource:      row.RegisterSource,
		Tag:                 row.Tag,
		Score:               row.Score,
		SigninWrongTimes:    row.SigninWrongTimes,
		LastSigninWrongTime: row.LastSigninWrongTime,
		PreferredMfaType:    row.PreferredMfaType,
		TotpSecret:          row.TotpSecret,
		MfaRadiusUsername:   row.MfaRadiusUsername,
		MfaRadiusProvider:   row.MfaRadiusProvider,
		MfaPushReceiver:     row.MfaPushReceiver,
		MfaPushProvider:     row.MfaPushProvider,
		MfaPushEnabled:      row.MfaPushEnabled,
		MfaRememberDeadline: row.MfaRememberDeadline,
		LastSigninAt:        row.LastSigninAt,
		LastSigninIP:        row.LastSigninIP,
		Balance:             row.Balance,
		BalanceCredit:       row.BalanceCredit,
		BalanceCurrency:     row.BalanceCurrency,
		IsDeleted:           row.IsDeleted,
		IpWhitelist:         row.IpWhitelist,
		Custom1:             row.Custom1,
		Custom2:             row.Custom2,
		Custom3:             row.Custom3,
		Custom4:             row.Custom4,
		Custom5:             row.Custom5,
		Custom6:             row.Custom6,
		Custom7:             row.Custom7,
		Custom8:             row.Custom8,
		Custom9:             row.Custom9,
		Custom10:            row.Custom10,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		DeletedAt:           row.DeletedAt,
		Revision:            row.Revision,
	}

	user.RecoveryCodeHashes = row.RecoveryCodeHashes
	if err := shared.UnmarshalJSON(row.Addresses, &user.Addresses); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal addresses")
	}
	if err := shared.UnmarshalJSON(row.ManagedAccounts, &user.ManagedAccounts); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal managed_accounts")
	}
	if err := shared.UnmarshalJSON(row.FaceIds, &user.FaceIds); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal face_ids")
	}
	if err := shared.UnmarshalJSON(row.Cart, &user.Cart); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal cart")
	}
	if err := shared.UnmarshalJSON(row.MfaItems, &user.MfaItems); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal mfa_items")
	}
	if err := shared.UnmarshalJSON(row.MfaAccounts, &user.MfaAccounts); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal mfa_accounts")
	}
	if err := shared.UnmarshalJSON(row.ApplicationScopes, &user.ApplicationScopes); err != nil {
		return nil, apperrors.WrapInternalf(err, "unmarshal application_scopes")
	}
	totpSecret, err := openTotpSecret(user.ID, user.TotpSecret)
	if err != nil {
		return nil, err
	}
	user.TotpSecret = totpSecret
	return user, nil
}

func applyProfile(target *domain.User, row userProfileRow) error {
	profile := domain.UserProfile{
		UserID:                row.UserID,
		DisplayName:           row.DisplayName,
		FirstName:             row.FirstName,
		LastName:              row.LastName,
		AvatarURL:             row.AvatarURL,
		Email:                 row.Email,
		EmailVerified:         row.EmailVerified,
		Phone:                 row.Phone,
		PhoneVerified:         row.PhoneVerified,
		CountryCode:           row.CountryCode,
		Region:                row.Region,
		Location:              row.Location,
		Birthday:              row.Birthday,
		Gender:                row.Gender,
		Bio:                   row.Bio,
		RealName:              row.RealName,
		IDCardType:            row.IDCardType,
		IDCard:                row.IDCard,
		PassportNo:            row.PassportNo,
		DriverLicenseNo:       row.DriverLicenseNo,
		IDCardExpiry:          row.IDCardExpiry,
		IssueCountry:          row.IssueCountry,
		IssuingAuthority:      row.IssuingAuthority,
		PassportPhotoURL:      row.PassportPhotoURL,
		DriverLicenseFrontURL: row.DriverLicenseFrontURL,
		DriverLicenseBackURL:  row.DriverLicenseBackURL,
	}

	if err := shared.UnmarshalJSON(row.Properties, &profile.Properties); err != nil {
		return err
	}

	if v, ok := profile.Properties["affiliation"]; ok {
		if s, ok := v.(string); ok {
			profile.Affiliation = s
		}
	}

	var err error
	if profile.IDCard, err = openProfileDocument(userProfileColumnIDCard, row.UserID, row.IDCard, row.IDCardSealed); err != nil {
		return err
	}
	if profile.PassportNo, err = openProfileDocument(userProfileColumnPassportNo, row.UserID, row.PassportNo, row.PassportNoSealed); err != nil {
		return err
	}
	if profile.DriverLicenseNo, err = openProfileDocument(userProfileColumnDriverLicenseNo, row.UserID, row.DriverLicenseNo, row.DriverLicenseNoSealed); err != nil {
		return err
	}

	target.Profile = profile
	return nil
}

func applyCredentials(target *domain.User, row userCredentialsRow) {
	target.Credentials = domain.UserCredentials{
		UserID:                    row.UserID,
		HasPassword:               row.PasswordHash != "",
		PasswordHash:              row.PasswordHash,
		PasswordAlgo:              row.PasswordAlgo,
		PasswordUpdatedAt:         row.PasswordUpdatedAt,
		AccessKey:                 row.AccessKey,
		AccessSecretHash:          row.AccessSecretHash,
		AccessSecretRotatedAt:     row.AccessSecretRotatedAt,
		NeedUpdatePassword:        row.NeedUpdatePassword,
		AccessSecretHashPrev:      row.AccessSecretHashPrev,
		AccessSecretPrevExpiresAt: row.AccessSecretPrevExpiresAt,
	}
}

func toIdentity(row userIdentityRow) (domain.UserIdentity, error) {
	identity := domain.UserIdentity{
		ID:              row.ID,
		UserID:          row.UserID,
		ProviderID:      row.ProviderID,
		IdentityType:    row.IdentityType,
		Subject:         row.Subject,
		ProviderAccount: row.ProviderAccount,
		LinkedAt:        row.LinkedAt,
	}

	if err := shared.UnmarshalJSON(row.ProfileSnapshot, &identity.ProfileSnapshot); err != nil {
		return domain.UserIdentity{}, err
	}

	return identity, nil
}

func toMfaMethod(row userMfaMethodRow) domain.UserMfaMethod {
	return domain.UserMfaMethod{
		ID:          row.ID,
		UserID:      row.UserID,
		MethodType:  row.MethodType,
		Enabled:     row.Enabled,
		Verified:    row.Verified,
		DisplayName: row.DisplayName,
		LastUsedAt:  row.LastUsedAt,
	}
}
