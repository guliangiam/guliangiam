package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	mfachallengedomain "github.com/guliangiam/guliangiam/internal/domain/mfachallenges"
	orgdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	userdomain "github.com/guliangiam/guliangiam/internal/domain/users"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/mfatotp"
	audit "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

func enabledVerifiedMfaMethods(user *userdomain.User) []userdomain.UserMfaMethod {
	methods := make([]userdomain.UserMfaMethod, 0, len(user.MfaMethods))
	for _, method := range user.MfaMethods {
		if !method.Enabled || !method.Verified {
			continue
		}
		methods = append(methods, method)
	}

	return methods
}

func requiredMfaSetupTypes(organization *orgdomain.Organization, user *userdomain.User) []string {
	if !organizationRequiresMfa(organization) {
		return nil
	}
	if len(enabledVerifiedMfaMethods(user)) > 0 {
		return nil
	}

	preferred := normalizeSetupMfaType(user.PreferredMfaType)
	if preferred != "" {
		return []string{preferred}
	}

	return []string{keyApp}
}

func organizationRequiresMfa(organization *orgdomain.Organization) bool {
	if organization == nil || len(organization.Security.MfaPolicy) == 0 {
		return false
	}

	required, ok := organization.Security.MfaPolicy["required"]
	if !ok {
		return false
	}

	switch typed := required.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	default:
		return false
	}
}

func normalizeSetupMfaType(methodType string) string {
	switch strings.TrimSpace(methodType) {
	case "totp":
		return keyApp
	case keyApp, contactEmail, contactSMS, "radius", contactPush:
		return strings.TrimSpace(methodType)
	default:
		return ""
	}
}

func requiresMfaVerification(now time.Time, user *userdomain.User) bool {
	if user.MfaRememberDeadline == nil {
		return true
	}

	return !user.MfaRememberDeadline.After(now)
}

func hasMfaVerificationAttempt(command LoginCommand) bool {
	return strings.TrimSpace(command.Passcode) != "" || strings.TrimSpace(command.RecoveryCode) != "" || strings.TrimSpace(command.MfaType) == contactPush
}

type loginMfaVerificationResult struct {
	Completed bool
	Response  LoginResponse
	// Failure carries the rejected attempt that has to be committed after the
	// login transaction rolls back.
	Failure *mfaChallengeFailure
}

const maxMfaAttempts = 5

func verifyLoginMfa(ctx context.Context, tx appuow.UnitOfWork, command LoginCommand, organization *orgdomain.Organization, user *userdomain.User, challenge *mfachallengedomain.Challenge, methods []userdomain.UserMfaMethod, now time.Time, pushNotifier PushNotifier) (loginMfaVerificationResult, error) {
	// Check if MFA attempts have been exhausted for this challenge.
	if challenge != nil && challenge.FailedAttempts >= maxMfaAttempts {
		// The delete is applied after the rollback; the marker only records the
		// intent so the row cannot survive a rejected login transaction.
		return loginMfaVerificationResult{Failure: &mfaChallengeFailure{
			ChallengeID: challenge.ID,
			Now:         now,
			Exhausted:   true,
		}}, apperrors.NewForbidden("too many failed MFA attempts, please start a new login")
	}

	if recoveryCode := strings.TrimSpace(command.RecoveryCode); recoveryCode != "" {
		if err := verifyRecoveryCode(user, recoveryCode); err != nil {
			failure := mfaFailureOf(challenge, mfaTypeRecovery, now)
			recordMfaFailure(challenge, now)
			return loginMfaVerificationResult{Failure: failure}, err
		}
		applyMfaRememberDeadline(command, organization, user, now)
		return loginMfaVerificationResult{Completed: true}, nil
	}

	mfaType := resolveRequestedMfaType(command.MfaType, user.PreferredMfaType, methods)
	if mfaType == "" {
		return loginMfaVerificationResult{}, apperrors.NewInvalidArgument("mfaType is required when passcode is provided")
	}
	if !containsMethodType(methods, mfaType) {
		return loginMfaVerificationResult{}, apperrors.NewUnauthorized("requested mfaType is not available for the user")
	}

	if !isSupportedMfaType(mfaType) {
		return loginMfaVerificationResult{}, apperrors.NewInvalidArgumentf("unsupported mfa type %q", mfaType)
	}

	passcode := strings.TrimSpace(command.Passcode)
	if passcode == "" && mfaType != mfaTypePush {
		return loginMfaVerificationResult{}, apperrors.NewInvalidArgument("missing passcode or recoveryCode")
	}

	switch mfaType {
	case mfaTypeTotp:
		matchedAt, err := verifyTotpPasscodeStep(user.TotpSecret, passcode, now)
		if err != nil {
			return rejectMfaAttempt(challenge, mfaTypeTotp, now, err)
		}
		if totpCodeReplayed(userMfaMethod(user, mfaTypeTotp), matchedAt) {
			return rejectMfaAttempt(challenge, mfaTypeTotp, now, apperrors.NewUnauthorized("invalid mfa passcode"))
		}
		if totpResyncRequired(challenge, matchedAt) {
			return rejectMfaAttempt(challenge, mfaTypeTotp, now, apperrors.NewUnauthorized("invalid mfa passcode"))
		}
	case mfaTypeEmail:
		if err := verifyEmailPasscode(ctx, user, passcode); err != nil {
			return rejectMfaAttempt(challenge, mfaTypeEmail, now, err)
		}
	case mfaTypeSms:
		if err := verifySmsPasscode(ctx, user, passcode); err != nil {
			return rejectMfaAttempt(challenge, mfaTypeSms, now, err)
		}
	case mfaTypeRadius:
		if err := verifyRadiusPasscode(ctx, tx, user, passcode); err != nil {
			return rejectMfaAttempt(challenge, mfaTypeRadius, now, err)
		}
	case mfaTypePush:
		result, err := verifyPushMfa(ctx, tx, challenge, user, methods, now, pushNotifier, command.IPAddress)
		if err != nil {
			return loginMfaVerificationResult{}, err
		}
		if result.Completed {
			applyMfaRememberDeadline(command, organization, user, now)
		}
		return result, nil
	default:
		return loginMfaVerificationResult{}, apperrors.NewInvalidArgumentf("mfa passcode verification for type %q is not implemented", mfaType)
	}

	markMfaMethodUsed(user, mfaType, now)
	applyMfaRememberDeadline(command, organization, user, now)
	return loginMfaVerificationResult{Completed: true}, nil
}

func verifyEmailPasscode(ctx context.Context, user *userdomain.User, passcode string) error {
	dest := strings.TrimSpace(user.Profile.Email)
	if dest == "" {
		return apperrors.NewUnauthorized("email mfa is not configured for the user")
	}
	return verifyEmailCode(ctx, dest, passcode)
}

func verifySmsPasscode(ctx context.Context, user *userdomain.User, passcode string) error {
	phone := strings.TrimSpace(user.Profile.Phone)
	if phone == "" {
		return apperrors.NewUnauthorized("sms mfa is not configured for the user")
	}
	return verifySmsCode(ctx, phone, user.Profile.CountryCode, passcode)
}

func verifyRadiusPasscode(ctx context.Context, tx appuow.UnitOfWork, user *userdomain.User, passcode string) error {
	username := strings.TrimSpace(user.MfaRadiusUsername)
	if username == "" {
		return apperrors.NewUnauthorized("radius mfa is not configured for the user")
	}
	providerID := strings.TrimSpace(user.MfaRadiusProvider)
	if providerID == "" {
		return apperrors.NewUnauthorized("radius mfa provider is not configured for the user")
	}

	config, err := resolveRadiusProvider(ctx, tx, providerID)
	if err != nil {
		return err
	}
	return verifyRadiusCredentials(ctx, config, username, passcode)
}

func verifyPushMfa(ctx context.Context, tx appuow.UnitOfWork, challenge *mfachallengedomain.Challenge, user *userdomain.User, methods []userdomain.UserMfaMethod, now time.Time, pushNotifier PushNotifier, ipAddress string) (loginMfaVerificationResult, error) {
	if challenge == nil {
		return loginMfaVerificationResult{}, apperrors.NewInvalidArgument("push mfa requires challengeId")
	}
	if challenge.State == mfachallengedomain.StateApproved {
		markMfaMethodUsed(user, contactPush, now)
		return loginMfaVerificationResult{Completed: true}, nil
	}
	if challenge.State == mfachallengedomain.StatePushPending {
		return loginMfaVerificationResult{Response: toMfaRequiredLoginResponse(user, user.PreferredMfaType, methods, challenge.ID, challenge.State, challenge.ExpiresAt)}, nil
	}
	if challenge.State == mfachallengedomain.StateRejected {
		return loginMfaVerificationResult{}, apperrors.NewUnauthorized("push approval rejected")
	}

	receiver := strings.TrimSpace(user.MfaPushReceiver)
	if receiver == "" {
		return loginMfaVerificationResult{}, apperrors.NewUnauthorized("push mfa receiver is not configured for the user")
	}
	providerID := strings.TrimSpace(user.MfaPushProvider)
	if providerID == "" {
		return loginMfaVerificationResult{}, apperrors.NewUnauthorized("push mfa provider is not configured for the user")
	}

	config, err := resolvePushProvider(ctx, tx, providerID)
	if err != nil {
		return loginMfaVerificationResult{}, err
	}

	// Build structured MFA push notification with contextual information.
	notification := BuildMfaPushNotification(MfaPushNotificationParams{
		LoginTime:  now,
		IPAddress:  ipAddress,
		ActionType: "sign-in",
	})

	if pushNotifier != nil {
		err = pushNotifier.Send(ctx, config.providerType, config, receiver, notification.Title, notification.Body)
	} else {
		err = sendPushAuthentication(ctx, config, receiver, notification.Title, notification.Body)
	}
	if err != nil {
		return loginMfaVerificationResult{}, err
	}

	challenge.State = mfachallengedomain.StatePushPending
	challenge.MethodType = contactPush
	challenge.UpdatedAt = now
	challenge.Revision++
	if err := tx.MfaChallenges().Update(ctx, challenge); err != nil {
		return loginMfaVerificationResult{}, err
	}

	return loginMfaVerificationResult{Response: toMfaRequiredLoginResponse(user, user.PreferredMfaType, methods, challenge.ID, challenge.State, challenge.ExpiresAt)}, nil
}

func resolveRequestedMfaType(requested, preferred string, methods []userdomain.UserMfaMethod) string {
	requested = strings.TrimSpace(requested)
	if requested != "" {
		return requested
	}
	if preferred != "" && containsMethodType(methods, preferred) {
		return preferred
	}
	if len(methods) == 0 {
		return ""
	}
	return methods[0].MethodType
}

func containsMethodType(methods []userdomain.UserMfaMethod, methodType string) bool {
	for _, method := range methods {
		if method.MethodType == methodType {
			return true
		}
	}
	return false
}

func isSupportedMfaType(methodType string) bool {
	switch methodType {
	case mfaTypeTotp, mfaTypeEmail, mfaTypeSms, mfaTypeRadius, mfaTypePush:
		return true
	default:
		return false
	}
}

const (
	totpPeriod              = mfatotp.Period
	totpResyncAfterFailures = 3
)

// verifyTotpPasscode keeps the historical signature for existing callers and
// tests; TOTP-specific replay/resync logic uses verifyTotpPasscodeStep.
func verifyTotpPasscode(secret, passcode string, now time.Time) error {
	_, err := verifyTotpPasscodeStep(secret, passcode, now)
	return err
}

// verifyTotpPasscodeStep validates the passcode against the current window and
// its two neighbours (skew 1) and returns the matched window start. Returning
// the window lets callers reject replayed codes without a new user-level column.
func verifyTotpPasscodeStep(secret, passcode string, now time.Time) (time.Time, error) {
	if strings.TrimSpace(secret) == "" {
		return time.Time{}, apperrors.NewUnauthorized("totp is not configured for the user")
	}

	step, ok, err := matchTotpStep(secret, passcode, now)
	if err != nil {
		return time.Time{}, err
	}
	if !ok {
		return time.Time{}, apperrors.NewUnauthorized("invalid mfa passcode")
	}
	return totpStepStart(step), nil
}

// matchTotpStep returns the time-step that produced the passcode. The shared
// mfatotp implementation is also used by enrollment, so both paths accept the
// same ±1 window.
func matchTotpStep(secret, passcode string, now time.Time) (int64, bool, error) {
	return mfatotp.MatchStep(secret, passcode, now)
}

func totpStepStart(step int64) time.Time {
	return mfatotp.StepStart(step)
}

func userMfaMethod(user *userdomain.User, methodType string) *userdomain.UserMfaMethod {
	for index := range user.MfaMethods {
		if user.MfaMethods[index].MethodType == methodType {
			return &user.MfaMethods[index]
		}
	}
	return nil
}

// totpCodeReplayed reports whether the matched window was already consumed.
// A code stays valid for its own window plus skew, so the check covers the
// matched window and the following one.
func totpCodeReplayed(method *userdomain.UserMfaMethod, matchedAt time.Time) bool {
	if method == nil {
		return false
	}
	return mfatotp.Replayed(method.LastUsedAt, matchedAt)
}

// totpResyncRequired implements the controlled re-sync gate: once the login
// challenge has failed totpResyncAfterFailures times, only a code from a later
// window than the last failure is accepted. This forces the client clock/window
// to advance instead of replaying the same drifting code.
func totpResyncRequired(challenge *mfachallengedomain.Challenge, matchedAt time.Time) bool {
	if challenge == nil || challenge.FailedAttempts < totpResyncAfterFailures {
		return false
	}
	lastFailureStep := challenge.UpdatedAt.UTC().Unix() / int64(totpPeriod/time.Second)
	return !matchedAt.After(totpStepStart(lastFailureStep))
}

// rejectMfaAttempt counts one rejected attempt in memory and returns the error
// together with the sidecar that persistMfaChallengeFailure commits after the
// login transaction rolls back. The resync audit is emitted by that transaction
// because an in-transaction audit row would be discarded as well.
func rejectMfaAttempt(challenge *mfachallengedomain.Challenge, methodType string, now time.Time, cause error) (loginMfaVerificationResult, error) {
	failure := mfaFailureOf(challenge, methodType, now)
	recordMfaFailure(challenge, now)
	return loginMfaVerificationResult{Failure: failure}, cause
}

// emitTotpResyncAudit records the transition into controlled re-sync. The
// payload deliberately carries no passcode or secret material.
func emitTotpResyncAudit(ctx context.Context, tx appuow.UnitOfWork, challenge *mfachallengedomain.Challenge) {
	if tx == nil {
		return
	}
	if err := audit.EmbedAuditEvent(clock.SystemClock{}, tx, ctx,
		"", "", "auth", "", "",
		challenge.OrganizationID, challenge.UserID,
		"mfa.totp.resync_required", "mfa_challenge", challenge.ID,
		"mfa", "mfa.totp.resync_required",
		map[string]any{"challengeId": challenge.ID, "methodType": mfaTypeTotp},
	); err != nil {
		slog.WarnContext(ctx, "failed to embed totp resync audit", "challenge_id", challenge.ID, "error", err)
	}
}

func verifyRecoveryCode(user *userdomain.User, recoveryCode string) error {
	for index, hash := range user.RecoveryCodeHashes {
		ok, err := crypto.VerifyClientSecret(recoveryCode, hash)
		if err != nil {
			return apperrors.WrapInternalf(err, "verify recovery code")
		}
		if !ok {
			continue
		}
		user.RecoveryCodeHashes = append(user.RecoveryCodeHashes[:index], user.RecoveryCodeHashes[index+1:]...)
		return nil
	}
	return apperrors.NewUnauthorized("invalid recovery code")
}

func markMfaMethodUsed(user *userdomain.User, methodType string, usedAt time.Time) {
	for index := range user.MfaMethods {
		if user.MfaMethods[index].MethodType == methodType && user.MfaMethods[index].Enabled && user.MfaMethods[index].Verified {
			user.MfaMethods[index].LastUsedAt = &usedAt
			return
		}
	}
}

func applyMfaRememberDeadline(command LoginCommand, organization *orgdomain.Organization, user *userdomain.User, now time.Time) {
	if !command.EnableMfaRemember {
		return
	}

	if hours, ok := mfaRememberHours(organization.Security.SessionPolicy); ok && hours > 0 {
		deadline := now.Add(time.Duration(hours) * time.Hour)
		user.MfaRememberDeadline = &deadline
	}
}

func mfaRememberHours(policy map[string]any) (int64, bool) {
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
			_, err := fmt.Sscan(trimmed, &parsed)
			if err == nil {
				return parsed, parsed > 0
			}
		}
	}

	return 0, false
}
