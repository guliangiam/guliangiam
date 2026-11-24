package users

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

const (
	adminWebAuthnChallengeTimeout = 5 * time.Minute
	adminWebAuthnDefaultName      = "Passkey"
	adminWebAuthnReservedID       = "registration"
	adminWebAuthnNameMaxLen       = 255
)

// invalidWebAuthnNameLength answers 422: the body bound, but the name exceeds
// the column ceiling, so the caller succeeds by shortening one field.
func invalidWebAuthnNameLength(name string) error {
	return apperrors.NewWithMeta(
		apperrors.CodeUnprocessableEntity,
		"name must be at most 255 characters",
		map[string]any{
			"field": "name",
			"value": utf8.RuneCountInString(name),
			"max":   adminWebAuthnNameMaxLen,
		},
	)
}

// RenameUserWebAuthnCredentialCommand renames a passkey owned by the given user.
type RenameUserWebAuthnCredentialCommand struct {
	OrganizationID string
	UserID         string
	CredentialID   string // base64url authenticator credentialId (URL path value)
	Name           string
}

// AdminWebAuthnRegistrationBeginCommand starts a server-driven registration
// ceremony for an existing user in the trusted tenant.
type AdminWebAuthnRegistrationBeginCommand struct {
	OrganizationID string
	UserID         string
	Name           string
}

// AdminWebAuthnRegistrationBeginResult contains the PublicKeyCredentialCreationOptions
// material the browser needs. userId is a stable, opaque user handle (never the raw UUID).
type AdminWebAuthnRegistrationBeginResult struct {
	Challenge          string   `json:"challenge"`
	ChallengeID        string   `json:"challengeId"`
	RelyingPartyID     string   `json:"rpId"`
	RelyingPartyName   string   `json:"rpName"`
	UserID             string   `json:"userId"`
	UserDisplayName    string   `json:"userDisplayName"`
	Timeout            int64    `json:"timeout"`
	ExcludeCredentials []string `json:"excludeCredentials,omitempty"`
	// UserVerification 与 finish 的 UV 校验同源：required（强制 UV）或 preferred。
	UserVerification string `json:"userVerification"`
}

// AdminWebAuthnRegistrationFinishCommand completes a server-driven registration.
// The client never supplies publicKey/aaguid/signCount: authenticatorData and the
// COSE public key are derived from the attestationObject.
type AdminWebAuthnRegistrationFinishCommand struct {
	OrganizationID    string
	UserID            string
	ChallengeID       string
	Challenge         string
	CredentialID      string // optional; when present it must match the derived credentialId
	ClientDataJSON    string
	AttestationObject string
	Transports        []string
	Name              string
}

// GetUserWebAuthnCredential returns one credential of a user in the trusted tenant.
func (s *Service) GetUserWebAuthnCredential(ctx context.Context, organizationID, userID, credentialID string) (*WebAuthnCredentialItem, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}
	if userID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	decoded, err := decodeWebAuthnCredentialID(credentialID)
	if err != nil {
		return nil, err
	}

	// webauthn_credentials has no organization column; the parent user is the
	// tenant anchor. A foreign user must look exactly like a missing one.
	if _, err := s.UOW.Users().GetByIDForOrganization(ctx, userID, organizationID); err != nil {
		return nil, err
	}

	cred, err := s.UOW.WebAuthnCredentials().GetByCredentialID(ctx, decoded)
	if err != nil {
		return nil, apperrors.NewNotFound("credential not found")
	}
	if cred.UserID != userID {
		return nil, apperrors.NewNotFound("credential not found") // don't leak ownership
	}

	item := toWebAuthnCredentialItem(cred)
	return &item, nil
}

// RenameUserWebAuthnCredential changes a credential's display name.
func (s *Service) RenameUserWebAuthnCredential(ctx context.Context, command RenameUserWebAuthnCredentialCommand) (*WebAuthnCredentialItem, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return nil, err
	}
	if command.UserID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	name := strings.TrimSpace(command.Name)
	if name == "" {
		return nil, apperrors.NewInvalidArgument("name must not be blank")
	}
	if utf8.RuneCountInString(name) > adminWebAuthnNameMaxLen {
		return nil, invalidWebAuthnNameLength(name)
	}
	decoded, err := decodeWebAuthnCredentialID(command.CredentialID)
	if err != nil {
		return nil, err
	}

	var item WebAuthnCredentialItem
	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}
		cred, err := tx.WebAuthnCredentials().GetByCredentialID(ctx, decoded)
		if err != nil {
			return apperrors.NewNotFound("credential not found")
		}
		if cred.UserID != command.UserID {
			return apperrors.NewNotFound("credential not found") // don't leak ownership
		}

		now := s.Clock.Now().UTC()
		cred.Name = name
		cred.UpdatedAt = now
		if err := tx.WebAuthnCredentials().Update(ctx, cred); err != nil {
			return apperrors.WrapInternal(err, "update webauthn credential")
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			user.OrganizationID,
			"",
			"users.webauthn.credential.renamed",
			"WebAuthnCredential",
			cred.ID,
			"users",
			"users.webauthn.credential.renamed",
			map[string]any{"name": cred.Name},
		); err != nil {
			return err
		}

		item = toWebAuthnCredentialItem(cred)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// BeginAdminWebAuthnRegistration starts a registration ceremony on behalf of a user.
func (s *Service) BeginAdminWebAuthnRegistration(ctx context.Context, command AdminWebAuthnRegistrationBeginCommand) (*AdminWebAuthnRegistrationBeginResult, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return nil, err
	}
	if command.UserID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	if name := strings.TrimSpace(command.Name); utf8.RuneCountInString(name) > adminWebAuthnNameMaxLen {
		return nil, invalidWebAuthnNameLength(name)
	}

	organization, err := s.UOW.Organizations().GetByID(ctx, command.OrganizationID)
	if err != nil {
		return nil, err
	}
	if !passkeyEnabled(organization) {
		return nil, apperrors.NewForbidden("passkey is disabled for this organization")
	}

	user, err := s.UOW.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
	if err != nil {
		return nil, err
	}

	rpID, err := platformwebauthn.ConfiguredRelyingPartyID()
	if err != nil {
		return nil, apperrors.NewInternalf(
			"WebAuthn RP ID unavailable (%s): configure CORS_ALLOWED_ORIGINS / ISSUER_URL so a shared domain can be derived", err.Error())
	}

	challenge, err := generateAdminWebAuthnChallenge()
	if err != nil {
		return nil, apperrors.NewInternal("generate webauthn challenge")
	}

	challengeID, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate webauthn challenge id")
	}

	now := s.Clock.Now().UTC()
	if err := s.UOW.WebAuthnCredentials().CreateChallenge(ctx, &webauthndomain.Challenge{
		ID:             challengeID,
		ChallengeHash:  webauthndomain.HashChallenge(challenge),
		CeremonyType:   webauthndomain.CeremonyTypeRegistration,
		UserID:         user.ID,
		OrganizationID: organization.ID,
		RelyingPartyID: rpID,
		ExpiresAt:      now.Add(adminWebAuthnChallengeTimeout),
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}); err != nil {
		return nil, apperrors.NewInternal("store webauthn challenge")
	}

	displayName := strings.TrimSpace(user.Profile.DisplayName)
	if displayName == "" {
		displayName = user.Username
	}

	existing, err := s.UOW.WebAuthnCredentials().ListByUserID(ctx, user.ID)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "list webauthn credentials")
	}

	return &AdminWebAuthnRegistrationBeginResult{
		Challenge:          challenge,
		ChallengeID:        challengeID,
		RelyingPartyID:     rpID,
		RelyingPartyName:   "GuliangIAM",
		UserID:             adminWebAuthnUserHandle(user.ID),
		UserDisplayName:    displayName,
		Timeout:            int64(adminWebAuthnChallengeTimeout / time.Millisecond),
		ExcludeCredentials: platformwebauthn.EncodeCredentialIDs(webauthndomain.ActiveCredentialIDs(existing)),
		UserVerification:   platformwebauthn.UserVerificationValue(webAuthnUVRequired(organization)),
	}, nil
}

// FinishAdminWebAuthnRegistration verifies and persists a server-driven registration.
func (s *Service) FinishAdminWebAuthnRegistration(ctx context.Context, command AdminWebAuthnRegistrationFinishCommand) (*WebAuthnCredentialItem, error) {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return nil, err
	}
	if command.UserID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	if strings.TrimSpace(command.ChallengeID) == "" {
		return nil, apperrors.NewNotFound("registration challenge not found")
	}
	if strings.TrimSpace(command.Challenge) == "" {
		return nil, apperrors.NewInvalidArgument("challenge is required")
	}
	if strings.TrimSpace(command.ClientDataJSON) == "" {
		return nil, apperrors.NewInvalidArgument("clientDataJSON is required")
	}
	if strings.TrimSpace(command.AttestationObject) == "" {
		return nil, apperrors.NewInvalidArgument("attestationObject is required")
	}

	allowedOrigins := platformwebauthn.ConfiguredAllowedOrigins()
	if len(allowedOrigins) == 0 {
		return nil, apperrors.NewInternal("WebAuthn origin allowlist is empty: configure CORS_ALLOWED_ORIGINS or ISSUER_URL")
	}
	rpID, err := platformwebauthn.ConfiguredRelyingPartyID()
	if err != nil {
		return nil, apperrors.NewInternalf(
			"WebAuthn RP ID unavailable (%s): configure CORS_ALLOWED_ORIGINS / ISSUER_URL so a shared domain can be derived", err.Error())
	}

	name := strings.TrimSpace(command.Name)
	if name == "" {
		name = adminWebAuthnDefaultName
	}
	if utf8.RuneCountInString(name) > adminWebAuthnNameMaxLen {
		return nil, invalidWebAuthnNameLength(name)
	}

	var item WebAuthnCredentialItem
	err = runInTx(ctx, s.UOW, func(ctx context.Context, tx appuow.UnitOfWork) error {
		organization, err := tx.Organizations().GetByID(ctx, command.OrganizationID)
		if err != nil {
			return err
		}
		if !passkeyEnabled(organization) {
			return apperrors.NewForbidden("passkey is disabled for this organization")
		}
		user, err := tx.Users().GetByIDForOrganization(ctx, command.UserID, command.OrganizationID)
		if err != nil {
			return err
		}

		// Atomic one-time consumption: missing / consumed / expired all surface
		// as NotFound from the repository and are indistinguishable by design.
		challenge, err := tx.WebAuthnCredentials().ConsumeChallenge(ctx, command.ChallengeID, s.Clock.Now())
		if err != nil {
			return apperrors.NewGone("registration challenge is missing, expired or already used")
		}
		if challenge.CeremonyType != webauthndomain.CeremonyTypeRegistration {
			return apperrors.NewForbidden("challenge was issued for another ceremony")
		}
		if challenge.UserID != user.ID || challenge.OrganizationID != organization.ID {
			return apperrors.NewForbidden("challenge does not belong to this user and organization")
		}
		if challenge.RelyingPartyID != rpID {
			return apperrors.NewUnprocessableEntity("challenge relying party does not match the current configuration")
		}
		if !challenge.MatchesChallenge(command.Challenge) {
			return apperrors.NewUnprocessableEntity("challenge does not match the one issued by begin")
		}

		material, err := platformwebauthn.DeriveRegistrationMaterial(command.AttestationObject)
		if err != nil {
			return apperrors.NewUnprocessableEntity("attestationObject is malformed")
		}
		if material.CredentialIDB64 == adminWebAuthnReservedID {
			return apperrors.NewInvalidArgument(`credentialId "registration" is reserved`)
		}
		if strings.TrimSpace(command.CredentialID) != "" {
			if command.CredentialID == adminWebAuthnReservedID {
				return apperrors.NewInvalidArgument(`credentialId "registration" is reserved`)
			}
			requestCredentialID, err := base64.RawURLEncoding.DecodeString(command.CredentialID)
			if err != nil {
				return apperrors.NewUnprocessableEntity("invalid credentialId encoding (expect base64url)")
			}
			if base64.RawURLEncoding.EncodeToString(requestCredentialID) != material.CredentialIDB64 {
				return apperrors.NewUnprocessableEntity("credentialId does not match the authenticatorData")
			}
		}

		aaguid, registrationSignCount, err := platformwebauthn.VerifyRegistration(
			command.Challenge,
			rpID,
			material.CredentialIDB64,
			material.PublicKey,
			command.ClientDataJSON,
			material.AuthenticatorDataB64,
			command.AttestationObject,
			allowedOrigins,
			webAuthnUVRequired(organization),
		)
		if err != nil {
			return apperrors.NewUnprocessableEntityf("WebAuthn registration verification failed: %s", err.Error())
		}

		existing, err := tx.WebAuthnCredentials().GetByCredentialID(ctx, material.CredentialID)
		if err != nil {
			var appErr *apperrors.Error
			if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
				return apperrors.WrapInternal(err, "check duplicate webauthn credential")
			}
		} else if existing != nil {
			return apperrors.NewAlreadyExists("credential is already registered")
		}

		credentialID, err := s.UUID.NewString()
		if err != nil {
			return apperrors.NewInternal("generate credential id")
		}

		now := s.Clock.Now().UTC()
		credential := &webauthndomain.Credential{
			ID:              credentialID,
			UserID:          user.ID,
			CredentialID:    material.CredentialID,
			PublicKey:       material.PublicKey,
			AttestationType: material.AttestationType,
			Transport:       command.Transports,
			SignCount:       registrationSignCount,
			Name:            name,
			Status:          webauthndomain.CredentialStatusActive,
			AAGUID:          aaguid,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := tx.WebAuthnCredentials().Create(ctx, credential); err != nil {
			var appErr *apperrors.Error
			if errors.As(err, &appErr) {
				return err
			}
			return apperrors.WrapInternal(err, "create webauthn credential")
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			organization.ID,
			"",
			"users.webauthn.credential.registered",
			"WebAuthnCredential",
			credential.ID,
			"users",
			"users.webauthn.credential.registered",
			map[string]any{
				"attestation_type": material.AttestationType,
				"user_id":          user.ID,
			},
		); err != nil {
			return err
		}

		item = toWebAuthnCredentialItem(credential)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func decodeWebAuthnCredentialID(value string) ([]byte, error) {
	if strings.TrimSpace(value) == "" {
		return nil, apperrors.NewInvalidArgument("credentialId is required")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "invalid credentialId encoding (expect base64url)", map[string]any{metaKeyField: "credentialId", metaKeyValue: value, metaKeyFormat: "base64url"})
	}
	return decoded, nil
}

// generateAdminWebAuthnChallenge mirrors auth.generateWebAuthnChallenge: 32 random
// bytes, base64url-encoded; only the SHA-256 hash is persisted.
func generateAdminWebAuthnChallenge() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// adminWebAuthnUserHandle is the WebAuthn user handle: sha256(user.ID) truncated
// to 32 bytes and base64url-encoded. It is stable for a user and never exposes
// the raw UUID.
func adminWebAuthnUserHandle(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return base64.RawURLEncoding.EncodeToString(sum[:32])
}
