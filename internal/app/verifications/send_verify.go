package verifications

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	organizationdomain "github.com/guliangiam/guliangiam/internal/domain/organizations"
	verificationsdomain "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/platform/config"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

const (
	verificationCodeLength  = 6
	verificationCodeTimeout = 10 * time.Minute

	VerificationTypeEmail = "email"
	VerificationTypePhone = "phone"
)

// verificationTypes is the closed set accepted by send/verify; the switch and
// the 422 `allowed` metadata read the same list.
var verificationTypes = []string{VerificationTypeEmail, VerificationTypePhone}

// SendVerificationCommand contains parameters for sending a verification code.
type SendVerificationCommand struct {
	OrganizationID string
	// ApplicationID optionally scopes delivery to the provider bound to one
	// application (application binding > organization provider > deployment
	// fallback). Empty keeps the organization-level resolution.
	ApplicationID string
	Type          string // "email" or "phone"
	Recipient     string // email address or phone number
	CaptchaToken  string
	CaptchaID     string
	ClientIP      string
}

// SendVerificationResult contains the result of a verification code send operation.
type SendVerificationResult struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Recipient string    `json:"recipient"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// VerifyCodeCommand contains parameters for verifying a code.
type VerifyCodeCommand struct {
	Type      string // "email" or "phone"
	Recipient string // email address or phone number
	Code      string
	ClientIP  string // client IP for rate limiting
	// OrganizationID optionally binds verification to the tenant that issued
	// the code (BE-A-20). Empty keeps the legacy org-agnostic behaviour for
	// callers without tenant context (the public FE verify modal sends none).
	OrganizationID string
	// ApplicationID is the application context of the flow. Verification uses
	// the provider binding recorded at send time; when that is unavailable this
	// value scopes provider-managed checks.
	ApplicationID string
}

// SendVerification generates a verification code, delivers it, and only then
// stores the pending row. A failed delivery must not leave a redeemable code
// behind, so the insert is deliberately after the sender call.
func (s Service) SendVerification(ctx context.Context, command SendVerificationCommand) (*SendVerificationResult, error) {
	if err := validateSendVerificationCommand(command); err != nil {
		return nil, err
	}

	organizationID, err := s.resolveOrganizationID(ctx, command.OrganizationID)
	if err != nil {
		return nil, err
	}
	command.OrganizationID = organizationID
	recipient := strings.TrimSpace(command.Recipient)

	// Human-check gate before delivery: limits code sends to verified humans
	// when the organization enables a captcha provider (anti SMS/mail bombing).
	if s.CaptchaVerifier != nil {
		if err := s.CaptchaVerifier(ctx, command.OrganizationID, command.CaptchaToken, command.CaptchaID, command.ClientIP); err != nil {
			return nil, err
		}
	}

	// Resolve the channel sender before doing anything else: an organization
	// without a usable provider must get 503 instead of an undeliverable code.
	emailSender, smsSender, err := s.resolveSender(ctx, command.OrganizationID, command.ApplicationID, command.Type)
	if err != nil {
		return nil, err
	}

	code, err := generateNumericCode(verificationCodeLength)
	if err != nil {
		return nil, apperrors.NewInternal("generate verification code")
	}

	now := s.Clock.Now().UTC()
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternal("generate verification id")
	}

	// Provider-managed senders (Twilio Verify) generate and check the code
	// themselves, so the stored code stays empty and verification delegates.
	storedCode := ""
	if _, ok := smsCodeVerifier(smsSender); !ok || command.Type != VerificationTypePhone {
		storedCode, err = s.digestVerificationCode(ctx, command.OrganizationID, command.Type, recipient, code)
		if err != nil {
			return nil, err
		}
	}

	if err := s.deliverVerification(ctx, command.Type, recipient, code, emailSender, smsSender); err != nil {
		return nil, err
	}

	// A freshly delivered code supersedes any still-pending codes for the same
	// recipient+type (BE-A-04): multiple live codes widen the brute-force
	// window. Runs after delivery so a failed send leaves the old code usable.
	if err := s.supersedePendingVerifications(ctx, command.Type, recipient); err != nil {
		return nil, err
	}

	verification := &verificationsdomain.Verification{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Type:           command.Type,
		Recipient:      recipient,
		Code:           storedCode,
		Status:         "pending",
		ExpiresAt:      now.Add(verificationCodeTimeout),
		CreatedAt:      now,
	}

	if err := s.UOW.Verifications().Create(ctx, verification); err != nil {
		return nil, err
	}

	// Record which application/provider actually delivered the code. This is
	// traceability only: the code has already been delivered, so a failure here
	// is logged instead of turned into a client error (a retry would send a
	// second SMS).
	s.recordApplicationBinding(ctx, verification.ID, command.ApplicationID, emailSender, smsSender)

	return &SendVerificationResult{
		ID:        verification.ID,
		Type:      verification.Type,
		Recipient: verification.Recipient,
		ExpiresAt: verification.ExpiresAt,
	}, nil
}

// VerifyCode validates a verification code against the stored pending verification.
// It enforces rate limiting: 3 failed attempts invalidate the code, and IP-level
// limits cap attempts at 10 per hour.
func (s Service) VerifyCode(ctx context.Context, command VerifyCodeCommand) error {
	if err := validateVerifyCodeCommand(command); err != nil {
		return err
	}

	recipient := strings.TrimSpace(command.Recipient)

	// Check recipient-level rate limit (max 3 failed attempts)
	if s.RateLimiter != nil && !s.RateLimiter.CheckAttempt(command.Type, recipient) {
		return apperrors.NewTooManyRequests("too many failed attempts, please request a new code")
	}

	// Check IP-level rate limit (max 10 attempts per hour)
	if s.RateLimiter != nil && !s.RateLimiter.CheckIP(command.ClientIP) {
		return apperrors.NewTooManyRequests("too many verification attempts from this IP")
	}

	verification, err := s.findActiveVerification(ctx, command.Type, recipient, strings.TrimSpace(command.Code))
	if err != nil {
		// Record the failed attempt
		if s.RateLimiter != nil {
			s.RateLimiter.RecordFailedAttempt(command.Type, recipient)
			s.RateLimiter.RecordIPAttempt(command.ClientIP)
		}
		return apperrors.NewInvalidArgument("verification code is invalid or expired")
	}

	// Tenant binding (BE-A-20), aligned with consumeVerificationCode: a code
	// issued for another organization is indistinguishable from an invalid
	// code. Only enforced when the caller supplies tenant context.
	if strings.TrimSpace(command.OrganizationID) != "" {
		organizationID, resolveErr := s.resolveOrganizationID(ctx, command.OrganizationID)
		if resolveErr != nil {
			return resolveErr
		}
		if organizationID != "" && !strings.EqualFold(verification.OrganizationID, organizationID) {
			if s.RateLimiter != nil {
				s.RateLimiter.RecordFailedAttempt(command.Type, recipient)
				s.RateLimiter.RecordIPAttempt(command.ClientIP)
			}
			return apperrors.NewInvalidArgument("verification code is invalid or expired")
		}
	}

	if err := s.UOW.Verifications().UpdateStatus(ctx, verification.ID, "verified"); err != nil {
		return err
	}

	// Clear failed attempts on success
	if s.RateLimiter != nil {
		s.RateLimiter.ResetAttempts(command.Type, recipient)
	}

	return nil
}

// resolveSender returns the sender for the requested channel. A nil resolver
// means the service was constructed without messaging; the channel send is
// then skipped, matching the previous behaviour for embedders that do not
// configure delivery.
//
// When applicationID is set and the resolver supports application-scoped
// bindings, the application's provider wins over the organization default.
func (s Service) resolveSender(ctx context.Context, organizationID, applicationID, verificationType string) (verificationsdomain.EmailSender, verificationsdomain.SmsSender, error) {
	if s.SenderResolver == nil {
		return nil, nil, nil
	}
	if applicationID != "" {
		if resolver, ok := s.SenderResolver.(verificationsdomain.ApplicationSenderResolver); ok {
			switch verificationType {
			case VerificationTypeEmail:
				sender, err := resolver.ResolveEmailSenderForApplication(ctx, organizationID, applicationID)
				return sender, nil, err
			case VerificationTypePhone:
				sender, err := resolver.ResolveSmsSenderForApplication(ctx, organizationID, applicationID)
				return nil, sender, err
			}
		}
	}
	switch verificationType {
	case VerificationTypeEmail:
		sender, err := s.SenderResolver.ResolveEmailSender(ctx, organizationID)
		return sender, nil, err
	case VerificationTypePhone:
		sender, err := s.SenderResolver.ResolveSmsSender(ctx, organizationID)
		return nil, sender, err
	}
	return nil, nil, nil
}

// smsCodeVerifier unwraps provider-id decorations before checking for the
// provider-managed verification capability.
func smsCodeVerifier(sender verificationsdomain.SmsSender) (verificationsdomain.SmsCodeVerifier, bool) {
	for i := 0; i < 3 && sender != nil; i++ {
		if verifier, ok := sender.(verificationsdomain.SmsCodeVerifier); ok {
			return verifier, true
		}
		decorated, ok := sender.(verificationsdomain.ProviderIDSmsSender)
		if !ok {
			return nil, false
		}
		sender = decorated.InnerSmsSender()
	}
	return nil, false
}

// verificationBindingRecorder persists the application/provider behind a code.
// Optional: repository implementations that predate application-scoped
// resolution simply omit the methods and the fields stay empty.
type verificationBindingRecorder interface {
	RecordApplicationBinding(ctx context.Context, id, applicationID, providerID string) error
}

// verificationBindingReader returns the binding recorded for a verification.
type verificationBindingReader interface {
	ApplicationBinding(ctx context.Context, id string) (applicationID, providerID string, err error)
}

func (s Service) recordApplicationBinding(ctx context.Context, id, applicationID string,
	emailSender verificationsdomain.EmailSender, smsSender verificationsdomain.SmsSender) {
	if id == "" {
		return
	}
	providerID := ""
	if decorated, ok := emailSender.(verificationsdomain.ProviderIDEmailSender); ok {
		providerID = decorated.ProviderID()
	}
	if decorated, ok := smsSender.(verificationsdomain.ProviderIDSmsSender); ok {
		providerID = decorated.ProviderID()
	}
	if applicationID == "" && providerID == "" {
		return
	}
	recorder, ok := s.UOW.Verifications().(verificationBindingRecorder)
	if !ok {
		return
	}
	if err := recorder.RecordApplicationBinding(ctx, id, applicationID, providerID); err != nil {
		slog.WarnContext(ctx, "failed to record verification application/provider binding",
			"verification_id", id, "application_id", applicationID, "provider_id", providerID, "error", err)
	}
}

// deliverVerification sends the plaintext code through the resolved channel.
func (s Service) deliverVerification(ctx context.Context, verificationType, recipient, code string,
	emailSender verificationsdomain.EmailSender, smsSender verificationsdomain.SmsSender) error {
	switch verificationType {
	case VerificationTypeEmail:
		if emailSender == nil {
			return nil
		}
		if err := emailSender.Send(ctx, recipient, "Verification Code",
			fmt.Sprintf("Your verification code is: %s", code)); err != nil {
			return verificationDeliveryError(err)
		}
	case VerificationTypePhone:
		if smsSender == nil {
			return nil
		}
		if err := smsSender.Send(ctx, recipient, code); err != nil {
			return verificationDeliveryError(err)
		}
	}
	return nil
}

// verificationDeliveryError preserves an already-classified application error
// and treats an unclassified sender failure as a dependency outage (503).
func verificationDeliveryError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperrors.Wrap(apperrors.CodeUnavailable, err, "verification delivery is unavailable")
}

// digestVerificationCode returns "<algorithm>$<hex>" for the organization's
// configured derive algorithm. The plaintext code never reaches the database.
func (s Service) digestVerificationCode(ctx context.Context, organizationID, verificationType, recipient, code string) (string, error) {
	algorithm, err := s.verificationDeriveAlgorithm(ctx, organizationID)
	if err != nil {
		return "", err
	}
	pepper, err := s.verificationPepper()
	if err != nil {
		return "", err
	}
	return verificationCodeDigest(algorithm, pepper, organizationID, verificationType, recipient, code)
}

func verificationCodeDigest(algorithm string, pepper []byte, organizationID, verificationType, recipient, code string) (string, error) {
	input, err := json.Marshal(struct {
		Domain         string `json:"domain"`
		OrganizationID string `json:"organizationId"`
		Recipient      string `json:"recipient"`
		Type           string `json:"type"`
		Code           string `json:"code"`
	}{
		Domain:         "guliangiam:verification-code:v1",
		OrganizationID: organizationID,
		Recipient:      recipient,
		Type:           verificationType,
		Code:           code,
	})
	if err != nil {
		return "", apperrors.WrapInternal(err, "encode verification code digest input")
	}
	digest, err := platformcrypto.DeriveKey(algorithm, pepper, input)
	if err != nil {
		return "", err
	}
	return algorithm + "$" + hex.EncodeToString(digest), nil
}

func (s Service) verificationDeriveAlgorithm(ctx context.Context, organizationID string) (string, error) {
	if strings.TrimSpace(organizationID) == "" || s.UOW.Organizations() == nil {
		return organizationdomain.PasswordDeriveAlgorithmHMACSHA256, nil
	}
	organization, err := s.UOW.Organizations().GetByID(ctx, organizationID)
	if err != nil {
		return "", err
	}
	return organization.Security.EffectivePasswordDeriveAlgorithm(), nil
}

// verificationPepperGeneration identifies which root-key generation a
// verification digest was derived from. The value never leaves the process:
// stored digests carry only the algorithm prefix, so a rotation-window match
// trials the generations below instead of persisting a pepper id.
type verificationPepperGeneration int

const (
	verificationPepperCurrent verificationPepperGeneration = iota
	verificationPepperPrevious
	verificationPepperLegacy
)

func (generation verificationPepperGeneration) String() string {
	switch generation {
	case verificationPepperPrevious:
		return "previous"
	case verificationPepperLegacy:
		return "legacy"
	default:
		return "current"
	}
}

// verificationPepperCandidate pairs a derived pepper with the root-key
// generation it came from, so a hit outside the active generation can be
// attributed without persisting a key id.
type verificationPepperCandidate struct {
	generation verificationPepperGeneration
	pepper     []byte
}

// verificationPepper derives the pepper used to issue new digests. It reads
// CURRENT and falls back to LEGACY only; PREVIOUS is a verification fallback
// during rotation and must never bind a freshly issued code to a key that is
// about to be retired.
func (s Service) verificationPepper() ([]byte, error) {
	if s.pepper != nil {
		return s.pepper()
	}
	return verificationCodePepper()
}

// verificationPepperCandidates returns the peppers a stored digest may have
// been derived from, in CURRENT → PREVIOUS → LEGACY order.
func (s Service) verificationPepperCandidates() ([]verificationPepperCandidate, error) {
	if s.pepper != nil {
		pepper, err := s.pepper()
		if err != nil {
			return nil, err
		}
		return []verificationPepperCandidate{{generation: verificationPepperCurrent, pepper: pepper}}, nil
	}
	return verificationCodePepperCandidates()
}

// verificationCodePepperCandidates resolves the rotation-window sequence from
// the field-encryption root keys. Empty values are skipped and duplicate keys
// are collapsed (the earlier generation wins), so PREVIOUS equal to CURRENT
// cannot double-count or shift the reported generation.
//
// Exit criterion: verification codes expire after verificationCodeTimeout
// (10 minutes). Once a full TTL has elapsed with no
// guliangiam_verification_pepper_rotation_hit_total{generation="previous"|"legacy"}
// increments and no "verification code matched retired pepper generation"
// records, the operator can remove ENCRYPTION_KEY_PREVIOUS (or the legacy
// ENCRYPTION_KEY) and delete this fallback. No pepper_id column or new table is
// needed: the stored digest records the derive algorithm only, and the trial
// is bounded by the three environment generations.
func verificationCodePepperCandidates() ([]verificationPepperCandidate, error) {
	roots := []struct {
		generation verificationPepperGeneration
		root       string
	}{
		{verificationPepperCurrent, config.EncryptionKeyCurrent()},
		{verificationPepperPrevious, config.EncryptionKeyPrevious()},
		{verificationPepperLegacy, config.EncryptionKeyLegacy()},
	}
	seen := make(map[string]struct{}, len(roots))
	candidates := make([]verificationPepperCandidate, 0, len(roots))
	for _, candidate := range roots {
		if candidate.root == "" {
			continue
		}
		if _, duplicate := seen[candidate.root]; duplicate {
			continue
		}
		seen[candidate.root] = struct{}{}
		pepper, err := verificationPepperFromRoot(candidate.root)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, verificationPepperCandidate{
			generation: candidate.generation,
			pepper:     pepper,
		})
	}
	if len(candidates) == 0 {
		return nil, apperrors.NewUnavailable("verification code pepper is not configured")
	}
	return candidates, nil
}

func verificationCodePepper() ([]byte, error) {
	root := config.EncryptionKeyCurrent()
	if root == "" {
		root = config.EncryptionKeyLegacy()
	}
	if root == "" {
		return nil, apperrors.NewUnavailable("verification code pepper is not configured")
	}
	return verificationPepperFromRoot(root)
}

// verificationPepperFromRoot keeps one derivation context string for every
// generation, so candidates differ only by the root key.
func verificationPepperFromRoot(root string) ([]byte, error) {
	pepper, err := platformcrypto.DeriveKey(
		platformcrypto.DeriveAlgorithmHMACSHA256,
		[]byte(root),
		[]byte("guliangiam:verification-code:pepper:v1"),
	)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "derive verification code pepper")
	}
	return pepper, nil
}

// auditVerificationPepperRotation records a digest accepted by a retired
// generation. It deliberately omits the plaintext code, the recipient and all
// key material; the verification id and organization id are enough to correlate
// the event with the stored row.
func auditVerificationPepperRotation(ctx context.Context, verification *verificationsdomain.Verification,
	verificationType string, generation verificationPepperGeneration) {
	if generation == verificationPepperCurrent {
		return
	}
	slog.WarnContext(ctx, "verification code matched retired pepper generation",
		"verification_id", verification.ID,
		"verification_type", verificationType,
		"key_generation", generation.String(),
		"organization_id", verification.OrganizationID,
	)
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func verificationDigestAlgorithm(stored string) (string, bool) {
	for _, algorithm := range []string{
		platformcrypto.DeriveAlgorithmHMACSHA256,
		platformcrypto.DeriveAlgorithmHMACSM3,
	} {
		if strings.HasPrefix(stored, algorithm+"$") {
			return algorithm, true
		}
	}
	return "", false
}

// MatchActiveVerificationCode verifies a plaintext code against the newest
// pending row without consuming it. Rows written before digest storage still
// compare as plaintext; new rows carry their issue-time algorithm prefix, so
// transactional auth flows can verify them without an extra organization read.
func MatchActiveVerificationCode(ctx context.Context, repository verificationsdomain.Repository,
	recipient, verificationType, code string) (*verificationsdomain.Verification, error) {
	pending, err := repository.GetActiveByRecipient(ctx, recipient, verificationType)
	if err != nil {
		return nil, err
	}
	algorithm, isDigest := verificationDigestAlgorithm(pending.Code)
	if !isDigest {
		if strings.Contains(pending.Code, "$") {
			return nil, apperrors.NewNotFound("verification not found or expired")
		}
		if constantTimeEqual(pending.Code, code) {
			return pending, nil
		}
		return nil, apperrors.NewNotFound("verification not found or expired")
	}
	pepperCandidates, err := verificationCodePepperCandidates()
	if err != nil {
		return nil, err
	}
	for _, candidate := range pepperCandidates {
		expected, digestErr := verificationCodeDigest(algorithm, candidate.pepper,
			pending.OrganizationID, verificationType, recipient, code)
		if digestErr != nil {
			return nil, digestErr
		}
		if !constantTimeEqual(pending.Code, expected) {
			continue
		}
		recordVerificationPepperRotationHit(candidate.generation, verificationType)
		auditVerificationPepperRotation(ctx, pending, verificationType, candidate.generation)
		return pending, nil
	}
	return nil, apperrors.NewNotFound("verification not found or expired")
}

func isVerificationNotFound(err error) bool {
	return apperrors.FromError(err).Code == apperrors.CodeNotFound
}

// findActiveVerification returns the pending verification matched by code.
// New rows store a salted digest; legacy rows still carry the six-digit code
// and remain readable until their TTL expires. Provider-managed SMS senders
// store an empty code and check upstream; an approved code is then persisted
// as a digest so later checks take the normal path.
func (s Service) findActiveVerification(ctx context.Context, verificationType, recipient, code string) (*verificationsdomain.Verification, error) {
	repository := s.UOW.Verifications()
	pending, err := repository.GetActiveByRecipient(ctx, recipient, verificationType)
	if err != nil {
		return nil, err
	}

	algorithm, err := s.verificationDeriveAlgorithm(ctx, pending.OrganizationID)
	if err != nil {
		return nil, err
	}
	pepper, err := s.verificationPepper()
	if err != nil {
		return nil, err
	}
	expectedDigest, err := verificationCodeDigest(algorithm, pepper,
		pending.OrganizationID, verificationType, recipient, code)
	if err != nil {
		return nil, err
	}
	verification, err := repository.GetActiveByRecipientAndCode(ctx, recipient, expectedDigest, verificationType)
	if err == nil {
		if constantTimeEqual(verification.Code, expectedDigest) {
			return verification, nil
		}
		return nil, apperrors.NewNotFound("verification not found or expired")
	}
	if !isVerificationNotFound(err) {
		return nil, err
	}
	digestErr := err

	// Root-key rotation window: a digest issued moments before the rotation is
	// still inside its ten-minute TTL. Trial the retired generations in
	// CURRENT → PREVIOUS → LEGACY order. The issuance digest was already
	// queried above, so skip the candidate that reproduces it.
	candidates, candidatesErr := s.verificationPepperCandidates()
	if candidatesErr != nil {
		return nil, candidatesErr
	}
	for _, candidate := range candidates {
		rotatedDigest, digestBuildErr := verificationCodeDigest(algorithm, candidate.pepper,
			pending.OrganizationID, verificationType, recipient, code)
		if digestBuildErr != nil {
			return nil, digestBuildErr
		}
		if rotatedDigest == expectedDigest {
			continue
		}
		rotated, lookupErr := repository.GetActiveByRecipientAndCode(ctx, recipient, rotatedDigest, verificationType)
		if lookupErr != nil {
			if !isVerificationNotFound(lookupErr) {
				return nil, lookupErr
			}
			continue
		}
		if !constantTimeEqual(rotated.Code, rotatedDigest) {
			return nil, apperrors.NewNotFound("verification not found or expired")
		}
		recordVerificationPepperRotationHit(candidate.generation, verificationType)
		auditVerificationPepperRotation(ctx, rotated, verificationType, candidate.generation)
		return rotated, nil
	}

	// Compatibility for rows written before digest storage. Only values without
	// a known digest prefix can match here; expired rows fall out naturally.
	legacy, legacyErr := repository.GetActiveByRecipientAndCode(ctx, recipient, code, verificationType)
	if legacyErr == nil {
		if constantTimeEqual(legacy.Code, code) {
			return legacy, nil
		}
		return nil, apperrors.NewNotFound("verification not found or expired")
	}
	if !isVerificationNotFound(legacyErr) {
		return nil, legacyErr
	}

	if verificationType != VerificationTypePhone || s.SenderResolver == nil || pending.Code != "" {
		return nil, digestErr
	}
	// Provider-managed verification must query the same provider account that
	// delivered the code, so replay the application binding recorded at send
	// time instead of always using the organization default.
	applicationID := ""
	if reader, ok := repository.(verificationBindingReader); ok {
		if recorded, _, readErr := reader.ApplicationBinding(ctx, pending.ID); readErr == nil {
			applicationID = recorded
		}
	}
	_, sender, resolveErr := s.resolveSender(ctx, pending.OrganizationID, applicationID, VerificationTypePhone)
	if resolveErr != nil {
		return nil, digestErr
	}
	verifier, ok := smsCodeVerifier(sender)
	if !ok {
		return nil, digestErr
	}
	approved, checkErr := verifier.CheckCode(ctx, recipient, code)
	if checkErr != nil {
		return nil, checkErr
	}
	if !approved {
		return nil, digestErr
	}
	if updateErr := repository.UpdateCode(ctx, pending.ID, expectedDigest); updateErr != nil {
		return nil, apperrors.WrapInternal(updateErr, "persist provider-approved verification code")
	}
	pending.Code = expectedDigest
	return pending, nil
}

// resolveOrganizationID accepts either an organization UUID or a slug.
//
// The unauthenticated auth surface is addressed by slug — the login form holds
// `organization.slug` — while `verifications.organization_id` is a UUID column
// whose insert casts with `$N::uuid` (unlike the list queries, which go through
// `guliangiam.resolve_org_id`). Forwarding a slug therefore failed with
// `invalid input syntax for type uuid`. The sibling auth endpoints
// (`/api/v1/auth/captcha`, `/api/v1/auth/captcha-status`) accept either form, so
// this one has to match them.
func (s Service) resolveOrganizationID(ctx context.Context, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}

	if platformuuid.IsValid(trimmed) {
		return trimmed, nil
	}

	organization, err := s.UOW.Organizations().GetBySlug(ctx, trimmed)
	if err != nil {
		return "", apperrors.NewInvalidArgumentf("unknown organization: %s", trimmed)
	}

	return organization.ID, nil
}

func validateSendVerificationCommand(command SendVerificationCommand) error {
	recipient := strings.TrimSpace(command.Recipient)
	if recipient == "" {
		return apperrors.NewInvalidArgument("recipient is required")
	}
	switch command.Type {
	case VerificationTypeEmail, VerificationTypePhone:
		// valid
	default:
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "type must be 'email' or 'phone'", map[string]any{"field": "type", "value": command.Type, "allowed": append([]string{}, verificationTypes...)})
	}
	return nil
}

func validateVerifyCodeCommand(command VerifyCodeCommand) error {
	recipient := strings.TrimSpace(command.Recipient)
	if recipient == "" {
		return apperrors.NewInvalidArgument("recipient is required")
	}
	if strings.TrimSpace(command.Code) == "" {
		return apperrors.NewInvalidArgument("code is required")
	}
	switch command.Type {
	case VerificationTypeEmail, VerificationTypePhone:
		// valid
	default:
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "type must be 'email' or 'phone'", map[string]any{"field": "type", "value": command.Type, "allowed": append([]string{}, verificationTypes...)})
	}
	return nil
}

// generateNumericCode generates a random numeric code of the specified length.
// Each digit is drawn with rejection sampling (crypto/rand.Int) so every digit
// 0-9 has exactly probability 1/10 — byte%10 biased 0-5 by 26/256 vs 25/256
// (BE-A-16).
func generateNumericCode(length int) (string, error) {
	digits := make([]byte, length)
	ten := big.NewInt(10)
	for i := range digits {
		n, err := rand.Int(rand.Reader, ten)
		if err != nil {
			return "", err
		}
		digits[i] = '0' + byte(n.Int64())
	}
	return string(digits), nil
}

// supersedePendingVerifications marks every still-pending code for the
// recipient+type as superseded before a new code is stored. The loop is
// bounded: each iteration retires one row, and a concurrent consumption
// (NotFound on the pending-guarded update) simply moves on.
func (s Service) supersedePendingVerifications(ctx context.Context, verificationType, recipient string) error {
	const maxSupersedeIterations = 20
	for i := 0; i < maxSupersedeIterations; i++ {
		pending, err := s.UOW.Verifications().GetActiveByRecipient(ctx, recipient, verificationType)
		if err != nil {
			if isVerificationNotFound(err) {
				return nil
			}
			return err
		}
		if pending == nil {
			return nil
		}
		if err := s.UOW.Verifications().UpdateStatus(ctx, pending.ID, "superseded"); err != nil {
			if isVerificationNotFound(err) {
				continue
			}
			return err
		}
	}
	return nil
}

// IsVerificationCodeValid checks if a verification code is valid without consuming it.
// This is used by the auth service for MFA and login flows.
func (s Service) IsVerificationCodeValid(ctx context.Context, verificationType, recipient, code string) bool {
	verification, err := s.findActiveVerification(ctx, verificationType, strings.TrimSpace(recipient), strings.TrimSpace(code))
	if err != nil {
		return false
	}
	return verification != nil
}

// ConsumeVerificationCode validates and marks a verification code as used.
// It enforces rate limiting: 3 failed attempts invalidate the code.
func (s Service) ConsumeVerificationCode(ctx context.Context, verificationType, recipient, code string) error {
	return s.consumeVerificationCode(ctx, "", verificationType, recipient, code, "")
}

// ConsumeVerificationCodeWithClientIP is the login entry point: it adds the
// per-IP budget and refuses a code that belongs to a different organization
// than the tenant resolved from the login request.
func (s Service) ConsumeVerificationCodeWithClientIP(ctx context.Context, organizationID, verificationType, recipient, code, clientIP string) error {
	return s.consumeVerificationCode(ctx, strings.TrimSpace(organizationID),
		verificationType, recipient, code, strings.TrimSpace(clientIP))
}

func (s Service) consumeVerificationCode(ctx context.Context, organizationID, verificationType, recipient, code, clientIP string) error {
	recipient = strings.TrimSpace(recipient)

	// Check per-IP budget first so a single source cannot grind many recipients.
	if clientIP != "" && s.RateLimiter != nil && !s.RateLimiter.CheckIP(clientIP) {
		return apperrors.NewTooManyRequests("too many attempts from this address, please try again later")
	}

	// Check recipient-level rate limit (max 3 failed attempts)
	if s.RateLimiter != nil && !s.RateLimiter.CheckAttempt(verificationType, recipient) {
		return apperrors.NewTooManyRequests("too many failed attempts, please request a new code")
	}

	verification, err := s.findActiveVerification(ctx, verificationType, recipient, strings.TrimSpace(code))
	if err != nil || verification == nil ||
		(organizationID != "" && !strings.EqualFold(verification.OrganizationID, organizationID)) {
		// Record the failed attempt; a cross-tenant match is indistinguishable
		// from an invalid code so the caller learns nothing about other tenants.
		if s.RateLimiter != nil {
			s.RateLimiter.RecordFailedAttempt(verificationType, recipient)
			if clientIP != "" {
				s.RateLimiter.RecordIPAttempt(clientIP)
			}
		}
		return apperrors.NewInvalidArgument("verification code is invalid or expired")
	}

	if err := s.UOW.Verifications().UpdateStatus(ctx, verification.ID, "consumed"); err != nil {
		return err
	}

	// Clear failed attempts on success
	if s.RateLimiter != nil {
		s.RateLimiter.ResetAttempts(verificationType, recipient)
	}

	return nil
}

// FormatVerificationCode is a helper to format code for display/debugging.
func FormatVerificationCode(code string) string {
	if len(code) == 6 {
		return fmt.Sprintf("%s-%s", code[:3], code[3:])
	}
	return code
}
