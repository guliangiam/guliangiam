package users

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/mfatotp"
)

type InitiateUserMfaSetupCommand struct {
	OrganizationID string
	UserID         string
	MfaType        string
}

type InitiateUserMfaSetupResult struct {
	Secret             string
	URL                string
	RecoveryCodes      []string
	CountryCode        string
	MfaRememberInHours int64
}

type VerifyUserMfaSetupCommand struct {
	OrganizationID string
	UserID         string
	MfaType        string
	Passcode       string
	Secret         string
	Dest           string
	CountryCode    string
}

type EnableUserMfaSetupCommand struct {
	OrganizationID string
	UserID         string
	MfaType        string
	Secret         string
	Dest           string
	CountryCode    string
	RecoveryCodes  []string
	// Passcode is the proof of possession collected by the setup ceremony.
	// Enabling only flips Verified=true after this value is checked server-side.
	Passcode          string
	PreviousSessionID string // S9-sensitive: callers own opaque session, rotated on MFA enable
}

func (s *Service) InitiateUserMfaSetup(ctx context.Context, command InitiateUserMfaSetupCommand) (*InitiateUserMfaSetupResult, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return nil, err
	}
	if err := validateInitiateUserMfaSetupCommand(command); err != nil {
		return nil, err
	}

	setupType, _, err := normalizeUserMfaSetupType(command.MfaType)
	if err != nil {
		return nil, err
	}

	var result *InitiateUserMfaSetupResult
	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		organization, err := tx.Organizations().GetByID(ctx, user.OrganizationID)
		if err != nil {
			return err
		}

		mfaUtil := getMfaUtil(setupType, nil)
		if mfaUtil == nil {
			return unsupportedMfaTypeError(command.MfaType)
		}

		issuer := ""
		accountName := user.Username
		if organization != nil {
			if strings.TrimSpace(organization.DisplayName) != "" {
				issuer = organization.DisplayName
			} else {
				issuer = organization.Slug
			}
			accountName = fmt.Sprintf("%s/%s", organization.Slug, user.Username)
		}

		props, err := mfaUtil.Initiate(accountName, issuer)
		if err != nil {
			return err
		}

		recoveryCode, err := s.UUID.NewString()
		if err != nil {
			return err
		}

		rememberHours, _ := userMfaRememberHours(organization)
		result = &InitiateUserMfaSetupResult{
			Secret:             strings.TrimSpace(props.Secret),
			URL:                strings.TrimSpace(props.URL),
			RecoveryCodes:      []string{recoveryCode},
			CountryCode:        user.Profile.CountryCode,
			MfaRememberInHours: rememberHours,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Service) VerifyUserMfaSetup(ctx context.Context, command VerifyUserMfaSetupCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	if err := validateVerifyUserMfaSetupCommand(command); err != nil {
		return err
	}

	setupType, _, err := normalizeUserMfaSetupType(command.MfaType)
	if err != nil {
		return err
	}

	return runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if _, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID); err != nil {
			return err
		}

		config, err := buildUserMfaSetupConfig(setupType, command.Secret, command.Dest, command.CountryCode)
		if err != nil {
			return err
		}

		mfaUtil := getMfaUtil(setupType, config)
		if mfaUtil == nil {
			return unsupportedMfaTypeError(command.MfaType)
		}

		return mfaUtil.SetupVerify(strings.TrimSpace(command.Passcode))
	})
}

func (s *Service) EnableUserMfaSetup(ctx context.Context, command EnableUserMfaSetupCommand) (string, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return "", err
	}
	if err := validateEnableUserMfaSetupCommand(command); err != nil {
		return "", err
	}

	setupType, storedMethodType, err := normalizeUserMfaSetupType(command.MfaType)
	if err != nil {
		return "", err
	}

	newSessionID := ""
	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		// Proof of possession must succeed before any state changes: a failed
		// passcode leaves the factor's verified/enabled state untouched.
		proof, err := s.verifyUserMfaSetupProof(setupType, storedMethodType, user, command)
		if err != nil {
			return err
		}

		profileChanged, err := applyUserMfaSetupProfile(&user.Profile, setupType, command.Dest, command.CountryCode)
		if err != nil {
			return err
		}

		updatedMethods, err := s.upsertUserMfaSetupMethod(user.MfaMethods, user.ID, storedMethodType, proof.LastUsedAt)
		if err != nil {
			return err
		}
		user.MfaMethods = updatedMethods

		if err := applyUserMfaSetupState(user, setupType, storedMethodType, command.Secret, command.Dest, command.RecoveryCodes); err != nil {
			return err
		}

		// Server-owned mirror: keep the collections MfaItems / MfaAccounts in
		// step with the real factor state so clients never hand-edit them.
		if err := deriveUserMfaCollections(ctx, tx, user); err != nil {
			return err
		}

		user.UpdatedAt = s.Clock.Now()
		user.Revision++

		if err := tx.Users().UpdateForOrganization(ctx, user, command.OrganizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"user.mfa.updated",
			"User",
			user.ID,
			"user",
			"user.mfa.updated",
			map[string]any{},
		); err != nil {
			return err
		}
		if profileChanged {
			user.Profile.UserID = user.ID
			if err := tx.Users().UpdateProfile(ctx, &user.Profile); err != nil {
				return err
			}
		}

		if err := tx.Users().ReplaceMfaMethods(ctx, user.ID, user.MfaMethods); err != nil {
			return err
		}

		// S9-sensitive: enabling MFA elevates assurance — rotate caller session.
		if strings.TrimSpace(command.PreviousSessionID) != "" {
			now := s.Clock.Now().UTC()
			if sess, getErr := tx.Sessions().GetByID(ctx, command.PreviousSessionID); getErr == nil {
				_ = tx.Sessions().Revoke(ctx, command.PreviousSessionID, now)
				sessID, idErr := s.UUID.NewString()
				if idErr != nil {
					return apperrors.NewInternal("generate rotated session id")
				}
				if cErr := tx.Sessions().Create(ctx, &sessionsdomain.Session{
					ID:               sessID,
					Sid:              newSessionSidLocal(),
					UserID:           sess.UserID,
					Username:         sess.Username,
					OrganizationID:   sess.OrganizationID,
					OrganizationSlug: sess.OrganizationSlug,
					ApplicationID:    sess.ApplicationID,
					ApplicationSlug:  sess.ApplicationSlug,
					SessionState:     sessionsdomain.StateActive,
					DeviceInfo:       sess.DeviceInfo,
					IPAddress:        sess.IPAddress,
					UserAgent:        sess.UserAgent,
					ExpiresAt:        now.Add(30 * 24 * time.Hour),
					CreatedAt:        now,
					LastSeenAt:       now,
					UpdatedAt:        now,
					Revision:         1,
				}); cErr != nil {
					return cErr
				}
				newSessionID = sessID
			}
		}
		return nil
	})
	return newSessionID, err
}

// newSessionSidLocal generates an OIDC session identifier. Duplicated here
// because users.Service cannot import app/auth without a circular dependency.
func newSessionSidLocal() string {
	b := make([]byte, 24)
	// crypto/rand is imported via webauthn_credentials.go (same package)
	rand.Read(b)
	return "sid-" + base64.RawURLEncoding.EncodeToString(b)
}

func normalizeUserMfaSetupType(raw string) (setupType, method string, err error) {
	trimmed := strings.TrimSpace(raw)
	if !slices.Contains(allowedMfaSetupTypes, trimmed) {
		return "", "", unsupportedMfaTypeError(raw)
	}
	if trimmed == mfaMethodApp || trimmed == mfaMethodTOTP {
		return mfaMethodApp, mfaMethodTOTP, nil
	}
	return trimmed, trimmed, nil
}

// unsupportedMfaTypeError answers 422 for a value outside the MFA type closed
// set; `allowed` comes from the same slice the dispatcher checks.
func unsupportedMfaTypeError(raw string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"unsupported mfaType",
		map[string]any{
			metaKeyField: "mfaType",
			metaKeyValue: strings.TrimSpace(raw),
			"allowed":    append([]string{}, allowedMfaSetupTypes...),
		},
	)
}

func buildUserMfaSetupConfig(setupType, secret, dest, countryCode string) (*mfaSetupProps, error) {
	config := &mfaSetupProps{MfaType: setupType}
	trimmedSecret := strings.TrimSpace(secret)
	trimmedDest := strings.TrimSpace(dest)
	trimmedCountryCode := strings.TrimSpace(countryCode)

	switch setupType {
	case mfaMethodApp:
		if trimmedSecret == "" {
			return nil, apperrors.NewInvalidArgument("totp secret is missing")
		}
		config.Secret = trimmedSecret
	case mfaMethodEmail:
		if trimmedDest == "" {
			return nil, apperrors.NewInvalidArgument("destination is missing")
		}
		config.Secret = trimmedDest
	case mfaMethodSMS:
		if trimmedDest == "" {
			return nil, apperrors.NewInvalidArgument("destination is missing")
		}
		if trimmedCountryCode == "" {
			return nil, apperrors.NewInvalidArgument("country code is missing")
		}
		config.Secret = trimmedDest
		config.CountryCode = trimmedCountryCode
	case mfaMethodRadius:
		if trimmedDest == "" {
			return nil, apperrors.NewInvalidArgument("RADIUS username is missing")
		}
		if trimmedSecret == "" {
			return nil, apperrors.NewInvalidArgument("RADIUS provider is missing")
		}
		config.Secret = trimmedDest
		config.URL = trimmedSecret
	case mfaMethodPush:
		if trimmedDest == "" {
			return nil, apperrors.NewInvalidArgument("push notification receiver is missing")
		}
		if trimmedSecret == "" {
			return nil, apperrors.NewInvalidArgument("push notification provider is missing")
		}
		config.Secret = trimmedDest
		config.URL = trimmedSecret
	}

	return config, nil
}

// userMfaSetupProof is the server-side proof-of-possession required before a
// factor may be marked verified. LastUsedAt is set only for TOTP so the shared
// replay window used by login also covers enrollment.
type userMfaSetupProof struct {
	LastUsedAt *time.Time
}

// verifyUserMfaSetupProof checks the enrollment passcode with the same
// implementation used at login. It never mutates state, so a rejected passcode
// leaves the factor's verified/enabled flags untouched.
func (s *Service) verifyUserMfaSetupProof(setupType, storedMethodType string, user *userdomain.User, command EnableUserMfaSetupCommand) (*userMfaSetupProof, error) {
	config, err := buildUserMfaSetupConfig(setupType, command.Secret, command.Dest, command.CountryCode)
	if err != nil {
		return nil, err
	}

	passcode := strings.TrimSpace(command.Passcode)
	if setupType == mfaMethodApp {
		if passcode == "" {
			return nil, apperrors.NewInvalidArgument("passcode is required")
		}
		step, ok, err := mfatotp.MatchStep(config.Secret, passcode, s.Clock.Now().UTC())
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, apperrors.NewUnauthorized("invalid mfa passcode")
		}
		matchedAt := mfatotp.StepStart(step)
		if existing := findUserMfaMethodByType(user.MfaMethods, storedMethodType); existing != nil && mfatotp.Replayed(existing.LastUsedAt, matchedAt) {
			return nil, apperrors.NewUnauthorized("invalid mfa passcode")
		}
		return &userMfaSetupProof{LastUsedAt: &matchedAt}, nil
	}

	mfaUtil := getMfaUtil(setupType, config)
	if mfaUtil == nil {
		return nil, unsupportedMfaTypeError(command.MfaType)
	}
	if passcode == "" && setupType != mfaMethodPush {
		return nil, apperrors.NewInvalidArgument("passcode is required")
	}
	if err := mfaUtil.SetupVerify(passcode); err != nil {
		return nil, err
	}
	return &userMfaSetupProof{}, nil
}

func findUserMfaMethodByType(methods []userdomain.UserMfaMethod, storedMethodType string) *userdomain.UserMfaMethod {
	for index := range methods {
		if normalizeExistingUserMfaMethodType(methods[index].MethodType) == storedMethodType {
			return &methods[index]
		}
	}
	return nil
}

func applyUserMfaSetupProfile(profile *userdomain.UserProfile, setupType, dest, countryCode string) (bool, error) {
	changed := false
	trimmedDest := strings.TrimSpace(dest)
	trimmedCountryCode := strings.TrimSpace(countryCode)

	switch setupType {
	case mfaMethodEmail:
		if strings.TrimSpace(profile.Email) == "" {
			if trimmedDest == "" {
				return false, apperrors.NewInvalidArgument("destination is missing")
			}
			profile.Email = trimmedDest
			changed = true
		}
		if !profile.EmailVerified {
			profile.EmailVerified = true
			changed = true
		}
	case mfaMethodSMS:
		if strings.TrimSpace(profile.Phone) == "" {
			if trimmedDest == "" {
				return false, apperrors.NewInvalidArgument("destination is missing")
			}
			if trimmedCountryCode == "" {
				return false, apperrors.NewInvalidArgument("country code is missing")
			}
			profile.Phone = trimmedDest
			profile.CountryCode = trimmedCountryCode
			changed = true
		}
	}

	return changed, nil
}

func applyUserMfaSetupState(user *userdomain.User, setupType, storedMethodType, secret, dest string, recoveryCodes []string) error {
	for _, code := range recoveryCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" {
			continue
		}
		hash, err := crypto.HashClientSecret(trimmed)
		if err != nil {
			return apperrors.WrapInternalf(err, "hash recovery code")
		}
		user.RecoveryCodeHashes = append(user.RecoveryCodeHashes, hash)
	}

	if user.PreferredMfaType == "" {
		user.PreferredMfaType = storedMethodType
	}

	switch setupType {
	case mfaMethodApp:
		trimmedSecret := strings.TrimSpace(secret)
		if trimmedSecret == "" {
			return apperrors.NewInvalidArgument("totp secret is missing")
		}
		user.TotpSecret = trimmedSecret
	case mfaMethodRadius:
		user.MfaRadiusUsername = strings.TrimSpace(dest)
		user.MfaRadiusProvider = strings.TrimSpace(secret)
	case mfaMethodPush:
		user.MfaPushReceiver = strings.TrimSpace(dest)
		user.MfaPushProvider = strings.TrimSpace(secret)
	}

	return nil
}

func (s *Service) upsertUserMfaSetupMethod(methods []userdomain.UserMfaMethod, userID, storedMethodType string, lastUsedAt *time.Time) ([]userdomain.UserMfaMethod, error) {
	updated := make([]userdomain.UserMfaMethod, 0, len(methods)+1)
	replaced := false
	for _, method := range methods {
		if normalizeExistingUserMfaMethodType(method.MethodType) == storedMethodType {
			method.MethodType = storedMethodType
			method.Enabled = true
			method.Verified = true
			if lastUsedAt != nil {
				method.LastUsedAt = lastUsedAt
			}
			if strings.TrimSpace(method.DisplayName) == "" {
				method.DisplayName = userMfaMethodDisplayName(storedMethodType)
			}
			updated = append(updated, method)
			replaced = true
			continue
		}
		updated = append(updated, method)
	}

	if !replaced {
		methodID, err := s.UUID.NewString()
		if err != nil {
			return nil, err
		}
		updated = append(updated, userdomain.UserMfaMethod{
			ID:          methodID,
			UserID:      userID,
			MethodType:  storedMethodType,
			Enabled:     true,
			Verified:    true,
			DisplayName: userMfaMethodDisplayName(storedMethodType),
			LastUsedAt:  lastUsedAt,
		})
	}

	return updated, nil
}

func normalizeExistingUserMfaMethodType(methodType string) string {
	trimmed := strings.TrimSpace(methodType)
	if trimmed == mfaMethodApp {
		return mfaMethodTOTP
	}
	return trimmed
}

func userMfaMethodDisplayName(methodType string) string {
	switch methodType {
	case mfaMethodTOTP:
		return "Authenticator App"
	case mfaMethodEmail:
		return "Email"
	case mfaMethodSMS:
		return "SMS"
	case mfaMethodRadius:
		return "RADIUS"
	case mfaMethodPush:
		return "Push"
	default:
		return strings.ToUpper(methodType)
	}
}

func userMfaRememberHours(organization *orgdomain.Organization) (int64, bool) {
	if organization == nil {
		return 0, false
	}

	return parseUserMfaRememberHours(organization.Security.SessionPolicy)
}

func parseUserMfaRememberHours(policy map[string]any) (int64, bool) {
	if len(policy) == 0 {
		return 0, false
	}

	for _, key := range []string{"mfaRememberHours", "mfaRememberInHours"} {
		value, ok := policy[key]
		if !ok {
			continue
		}

		switch typed := value.(type) {
		case int:
			return int64(typed), typed > 0
		case int32:
			return int64(typed), typed > 0
		case int64:
			return typed, typed > 0
		case float64:
			return int64(typed), typed > 0
		case string:
			trimmed := strings.TrimSpace(typed)
			if trimmed == "" {
				return 0, false
			}
			var parsed int64
			if _, err := fmt.Sscan(trimmed, &parsed); err == nil {
				return parsed, parsed > 0
			}
		}
	}

	return 0, false
}
