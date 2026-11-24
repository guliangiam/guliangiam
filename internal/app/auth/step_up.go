package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	sessionsdomain "github.com/guliangiam/guliangiam/internal/domain/sessions"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	webauthndomain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformwebauthn "github.com/guliangiam/guliangiam/internal/platform/webauthn"
)

// F2 step-up (AAL) challenge parameters.
const (
	// StepUpChallengeTTL bounds how long a step-up challenge may be redeemed.
	StepUpChallengeTTL = 5 * time.Minute

	// StepUpMethodType is stored in login_challenges.method_type. It marks a
	// challenge as a session elevation instead of a login MFA challenge.
	StepUpMethodType = "step_up"

	// StepUpMetadataSessionID / StepUpMetadataOperationScope bind the challenge
	// to one session and one operation. A challenge redeemed from another
	// session or for another scope is rejected.
	StepUpMetadataSessionID      = "step_up_session_id"
	StepUpMetadataOperationScope = "step_up_operation_scope"

	stepUpMaxOperationScopeLength = 128
)

type StepUpBeginCommand struct {
	OrganizationID string
	UserID         string
	SessionID      string
	OperationScope string
	ApplicationID  string
}

type StepUpBeginResult struct {
	ChallengeID    string
	OperationScope string
	ExpiresAt      time.Time
}

type StepUpFinishCommand struct {
	OrganizationID string
	UserID         string
	SessionID      string
	ApplicationID  string
	ChallengeID    string
	OperationScope string
	Passcode       string

	// Passkey assertion material. It is mutually exclusive with Passcode and
	// reuses the challenge table written by
	// /api/v1/auth/webauthn/authentication/begin — this entry point adds no new
	// table and no new route.
	WebAuthnChallengeID string
	WebAuthnChallenge   string
	CredentialID        string
	Signature           string
	AuthenticatorData   string
	ClientDataJSON      string
}

type StepUpFinishResult struct {
	AssuranceLevel string
	SessionID      string
	OperationScope string
	ElevatedAt     time.Time
}

// BeginStepUp creates a pending step-up challenge bound to the caller's
// session and the operation being elevated. The challenge is not a login
// challenge: it is consumed only by FinishStepUp.
func (s *Service) BeginStepUp(ctx context.Context, command StepUpBeginCommand) (*StepUpBeginResult, error) {
	command.OrganizationID = strings.TrimSpace(command.OrganizationID)
	command.UserID = strings.TrimSpace(command.UserID)
	command.SessionID = strings.TrimSpace(command.SessionID)
	command.ApplicationID = strings.TrimSpace(command.ApplicationID)

	if command.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if command.UserID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	if command.SessionID == "" {
		return nil, apperrors.NewInvalidArgument("sessionId is required")
	}

	operationScope, err := normalizeStepUpOperationScope(command.OperationScope)
	if err != nil {
		return nil, err
	}

	challengeID, err := s.UUIDGenerator.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate step-up challenge id")
	}

	now := s.Clock.Now()
	expiresAt := now.Add(StepUpChallengeTTL)
	challenge := &mfachallengedomain.Challenge{
		ID:             challengeID,
		OrganizationID: command.OrganizationID,
		UserID:         command.UserID,
		ApplicationID:  command.ApplicationID,
		State:          mfachallengedomain.StatePending,
		MethodType:     StepUpMethodType,
		Metadata: map[string]any{
			StepUpMetadataSessionID:      command.SessionID,
			StepUpMetadataOperationScope: operationScope,
		},
		ExpiresAt: expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// The transport identifies the session by its opaque id only. When the
		// caller did not supply an application id, inherit it from the session
		// that is being elevated (sessions.application_id is NOT NULL).
		if challenge.ApplicationID == "" {
			session, err := tx.Sessions().GetByIDForOrganization(ctx, command.SessionID, command.OrganizationID)
			if err != nil || session == nil || !session.IsActive(now) {
				return apperrors.NewUnauthorized("step-up session invalid or expired")
			}
			challenge.ApplicationID = session.ApplicationID
		}
		return tx.MfaChallenges().Create(ctx, challenge)
	}); err != nil {
		return nil, err
	}

	return &StepUpBeginResult{ChallengeID: challengeID, OperationScope: operationScope, ExpiresAt: expiresAt}, nil
}

// FinishStepUp verifies a TOTP code against an outstanding step-up challenge
// and raises the assurance level of the current session to AAL2. Other
// sessions and the global login result are left untouched.
func (s *Service) FinishStepUp(ctx context.Context, command StepUpFinishCommand) (*StepUpFinishResult, error) {
	command.OrganizationID = strings.TrimSpace(command.OrganizationID)
	command.UserID = strings.TrimSpace(command.UserID)
	command.SessionID = strings.TrimSpace(command.SessionID)
	command.ApplicationID = strings.TrimSpace(command.ApplicationID)
	command.ChallengeID = strings.TrimSpace(command.ChallengeID)

	if command.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if command.UserID == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	if command.SessionID == "" {
		return nil, apperrors.NewInvalidArgument("sessionId is required")
	}
	if command.ChallengeID == "" {
		return nil, apperrors.NewInvalidArgument("challengeId is required")
	}

	operationScope, err := normalizeStepUpOperationScope(command.OperationScope)
	if err != nil {
		return nil, err
	}

	passcode := strings.TrimSpace(command.Passcode)
	command.WebAuthnChallengeID = strings.TrimSpace(command.WebAuthnChallengeID)
	command.WebAuthnChallenge = strings.TrimSpace(command.WebAuthnChallenge)
	command.CredentialID = strings.TrimSpace(command.CredentialID)
	command.Signature = strings.TrimSpace(command.Signature)
	command.AuthenticatorData = strings.TrimSpace(command.AuthenticatorData)
	command.ClientDataJSON = strings.TrimSpace(command.ClientDataJSON)

	webAuthnAssertion := command.WebAuthnChallengeID != "" ||
		command.WebAuthnChallenge != "" ||
		command.CredentialID != "" ||
		command.Signature != "" ||
		command.AuthenticatorData != "" ||
		command.ClientDataJSON != ""

	if passcode == "" && !webAuthnAssertion {
		return nil, apperrors.NewInvalidArgument("passcode or WebAuthn assertion is required")
	}
	if passcode != "" && webAuthnAssertion {
		return nil, apperrors.NewInvalidArgument("passcode and WebAuthn assertion are mutually exclusive")
	}
	if webAuthnAssertion &&
		(command.WebAuthnChallengeID == "" ||
			command.WebAuthnChallenge == "" ||
			command.CredentialID == "" ||
			command.Signature == "" ||
			command.AuthenticatorData == "" ||
			command.ClientDataJSON == "") {
		return nil, apperrors.NewInvalidArgument("webauthnChallengeId, challenge, credentialId, signature, authenticatorData and clientDataJSON are required")
	}

	// A rejected TOTP attempt rolls the challenge counter back with the main
	// transaction, so the increment is committed afterwards in its own
	// transaction (same sidecar pattern as Login).
	var failedChallenge *mfaChallengeFailure
	// A passkey assertion that looks cloned revokes the credential in its own
	// transaction after the main transaction rolls back.
	var cloneSuspicion *stepUpCloneSuspicion

	var result *StepUpFinishResult
	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		challenge, err := tx.MfaChallenges().GetByID(ctx, command.ChallengeID)
		if err != nil || challenge == nil {
			return stepUpInvalidChallengeError()
		}
		if challenge.OrganizationID != command.OrganizationID ||
			challenge.UserID != command.UserID ||
			challenge.State != mfachallengedomain.StatePending ||
			challenge.MethodType != StepUpMethodType ||
			challenge.ExpiresAt.Before(now) ||
			!stepUpChallengeMatches(challenge, command.SessionID, operationScope) {
			return stepUpInvalidChallengeError()
		}

		user, err := tx.Users().GetByID(ctx, command.UserID)
		if err != nil || user == nil {
			return stepUpInvalidChallengeError()
		}

		factor := mfaTypeTotp
		if passcode != "" {
			if strings.TrimSpace(user.TotpSecret) == "" {
				return apperrors.NewUnprocessableEntity("totp is not configured")
			}

			reject := func(cause error) error {
				failedChallenge = mfaFailureOf(challenge, mfaTypeTotp, now)
				recordMfaFailure(challenge, now)
				return cause
			}

			matchedAt, verifyErr := verifyTotpPasscodeStep(user.TotpSecret, passcode, now)
			if verifyErr != nil {
				return reject(verifyErr)
			}
			if totpCodeReplayed(userMfaMethod(user, mfaTypeTotp), matchedAt) {
				return reject(apperrors.NewUnauthorized("invalid mfa passcode"))
			}
			if totpResyncRequired(challenge, matchedAt) {
				return reject(apperrors.NewUnauthorized("invalid mfa passcode"))
			}

			markMfaMethodUsed(user, mfaTypeTotp, now)
			user.Revision++
			user.UpdatedAt = now
			if err := tx.Users().Update(ctx, user); err != nil {
				return err
			}
		} else {
			factor = loginMfaMethodPasskey
			suspicion, passkeyErr := s.verifyStepUpPasskeyAssertion(ctx, tx, command, user, now)
			if suspicion != nil {
				cloneSuspicion = suspicion
			}
			if passkeyErr != nil {
				return passkeyErr
			}
		}

		challenge.State = mfachallengedomain.StateApproved
		challenge.VerifiedAt = &now
		challenge.UpdatedAt = now
		challenge.Revision++
		if err := tx.MfaChallenges().Update(ctx, challenge); err != nil {
			return err
		}

		session, err := tx.Sessions().GetByIDForOrganization(ctx, command.SessionID, command.OrganizationID)
		if err != nil || session == nil || !session.IsActive(now) {
			return apperrors.NewUnauthorized("step-up session invalid or expired")
		}

		// Only the current session is elevated: no other session is revoked and
		// no global login state is rewritten.
		session.AssuranceLevel = sessionsdomain.AssuranceLevelAAL2
		session.Revision++
		session.UpdatedAt = now
		if err := tx.Sessions().Update(ctx, session); err != nil {
			return err
		}

		if err := s.embedAuditOutboxPlain(tx, ctx,
			"", "", "authentication",
			command.OrganizationID, command.UserID,
			"auth.step_up.completed", "Session", command.SessionID,
			"session", "auth.step_up.completed",
			map[string]any{
				"sessionId":      command.SessionID,
				"operationScope": operationScope,
				"assuranceLevel": sessionsdomain.AssuranceLevelAAL2,
				"method":         factor,
			},
		); err != nil {
			return err
		}

		result = &StepUpFinishResult{
			AssuranceLevel: sessionsdomain.AssuranceLevelAAL2,
			SessionID:      command.SessionID,
			OperationScope: operationScope,
			ElevatedAt:     now,
		}
		return nil
	})
	if err != nil {
		if cloneSuspicion != nil {
			// The revocation must survive the rollback, so it is committed in its
			// own transaction (same sidecar pattern as the MFA failure counter).
			if revokeErr := s.revokeCredentialAfterCloneSuspicion(tenantctx.System(ctx), *cloneSuspicion); revokeErr != nil {
				slog.WarnContext(ctx, "failed to revoke credential after step-up sign count regression",
					"credential_id", cloneSuspicion.credentialID, "error", revokeErr)
			}
		}
		if failedChallenge != nil {
			// The counter must survive the rollback, so it is persisted in its
			// own system transaction. A persistence error never changes the
			// response returned to the caller.
			if recordErr := s.persistMfaChallengeFailure(tenantctx.System(ctx), *failedChallenge); recordErr != nil {
				slog.WarnContext(ctx, "failed to persist step-up challenge failure",
					"challenge_id", failedChallenge.ChallengeID, "error", recordErr)
			}
		}
		return nil, err
	}

	return result, nil
}

type stepUpCloneSuspicion struct {
	organizationID    string
	userID            string
	credentialID      string
	storedSignCount   int64
	receivedSignCount int64
}

// verifyStepUpPasskeyAssertion completes the passkey branch of a step-up
// challenge. It consumes the WebAuthn authentication challenge issued by the
// existing /api/v1/auth/webauthn/authentication/begin route, verifies the
// assertion against the stored credential, and advances the signature
// counter. Session elevation stays in the caller so the step-up challenge
// state, the session row and the counter all commit (or roll back) together.
func (s *Service) verifyStepUpPasskeyAssertion(
	ctx context.Context,
	tx appuow.UnitOfWork,
	command StepUpFinishCommand,
	user *userdomain.User,
	now time.Time,
) (*stepUpCloneSuspicion, error) {
	if err := s.requireWebAuthnOriginConfig(); err != nil {
		return nil, err
	}

	organization, err := tx.Organizations().GetByID(ctx, command.OrganizationID)
	if err != nil || organization == nil {
		return nil, stepUpInvalidChallengeError()
	}
	if !passkeyAsSecondFactorEnabled(organization) {
		return nil, apperrors.NewForbidden("passkey is not enabled as a second factor for this organization")
	}

	sessionData, err := tx.WebAuthnCredentials().ConsumeChallenge(ctx, command.WebAuthnChallengeID, now)
	if err != nil {
		// 缺失 / 已消费 / 已过期在仓储层同为 NotFound；统一 410 且消息一致，
		// 与登录 finish 的防枚举口径保持一致。
		return nil, apperrors.NewGone("authentication challenge is missing, expired or already used")
	}
	if sessionData.CeremonyType != webauthndomain.CeremonyTypeAuthentication {
		return nil, apperrors.NewForbidden("challenge was issued for another ceremony")
	}
	// The WebAuthn challenge is user-scoped; the step-up challenge is
	// session-scoped. Both must name the same user and organization.
	if sessionData.OrganizationID != command.OrganizationID || sessionData.UserID != user.ID {
		return nil, apperrors.NewForbidden("webauthn challenge does not belong to this session user")
	}
	if strings.TrimSpace(command.WebAuthnChallenge) == "" || !sessionData.MatchesChallenge(command.WebAuthnChallenge) {
		return nil, apperrors.NewUnprocessableEntity("challenge does not match the one issued by begin")
	}

	credentialIDBytes, decodeErr := base64.RawURLEncoding.DecodeString(command.CredentialID)
	if decodeErr != nil {
		return nil, apperrors.NewInvalidArgument("invalid credentialId encoding")
	}
	credential, err := tx.WebAuthnCredentials().GetByCredentialID(ctx, credentialIDBytes)
	if err != nil || credential == nil {
		return nil, apperrors.NewUnprocessableEntity("credential not found")
	}
	if credential.Status != webauthndomain.CredentialStatusActive {
		return nil, apperrors.NewUnprocessableEntityf("credential is %s; step-up denied", credential.Status)
	}
	if credential.UserID != user.ID {
		return nil, apperrors.NewForbidden("credential does not belong to this user")
	}

	newSignCount, verifyErr := platformwebauthn.VerifyAssertion(
		command.WebAuthnChallenge,
		sessionData.RelyingPartyID,
		command.ClientDataJSON,
		command.AuthenticatorData,
		command.Signature,
		credential.PublicKey,
		credential.SignCount,
		s.webAuthnAllowedOrigins(),
		s.webAuthnUVRequired(organization),
	)
	if verifyErr != nil {
		if errors.Is(verifyErr, platformwebauthn.ErrSignCountRegression) {
			// Same AC-AUTH-008 handling as the login ceremony: revoke the
			// credential and record a critical security event. Both happen in a
			// side transaction because the main transaction is about to roll back.
			return &stepUpCloneSuspicion{
				organizationID:    command.OrganizationID,
				userID:            command.UserID,
				credentialID:      credential.ID,
				storedSignCount:   credential.SignCount,
				receivedSignCount: newSignCount,
			}, apperrors.NewUnprocessableEntity("WebAuthn assertion sign count did not increase; credential revoked and security event recorded")
		}
		return nil, apperrors.NewUnprocessableEntityf("WebAuthn assertion verification failed: %s", verifyErr.Error())
	}

	// 计数器恒为 0 的认证器（未上报）不写回，避免 0/0 被仓储单调性谓词误判为回退。
	if newSignCount > 0 {
		if err := tx.WebAuthnCredentials().UpdateSignCount(ctx, credential.ID, newSignCount); err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (s *Service) revokeCredentialAfterCloneSuspicion(ctx context.Context, suspicion stepUpCloneSuspicion) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.WebAuthnCredentials().UpdateStatus(ctx, suspicion.credentialID, webauthndomain.CredentialStatusRevoked); err != nil {
			return err
		}
		return s.embedAuditOutboxPlain(tx, ctx,
			"", "", "authentication",
			suspicion.organizationID, suspicion.userID,
			"webauthn.credential.clone_suspected", "webauthn_credential", suspicion.credentialID,
			"webauthn_credential", "webauthn.credential.clone_suspected",
			map[string]any{
				"reason":            "sign_count_not_increased",
				"storedSignCount":   suspicion.storedSignCount,
				"receivedSignCount": suspicion.receivedSignCount,
				"surface":           "step_up",
			},
		)
	})
}

func normalizeStepUpOperationScope(raw string) (string, error) {
	scope := strings.TrimSpace(raw)
	if scope == "" {
		return "", apperrors.NewInvalidArgument("operationScope is required")
	}
	if utf8.RuneCountInString(scope) > stepUpMaxOperationScopeLength {
		return "", apperrors.NewInvalidArgument("operationScope must be at most 128 characters")
	}
	return scope, nil
}

// stepUpChallengeMatches reports whether the challenge was minted for exactly
// the presented session and operation scope. Missing or non-string metadata is
// treated as a mismatch.
func stepUpChallengeMatches(challenge *mfachallengedomain.Challenge, sessionID, operationScope string) bool {
	if challenge == nil {
		return false
	}
	return stepUpMetadataString(challenge.Metadata, StepUpMetadataSessionID) == sessionID &&
		stepUpMetadataString(challenge.Metadata, StepUpMetadataOperationScope) == operationScope
}

func stepUpMetadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key].(string)
	if !ok {
		return ""
	}
	return value
}

func stepUpInvalidChallengeError() error {
	return apperrors.NewUnauthorized("step-up challenge invalid or expired")
}
