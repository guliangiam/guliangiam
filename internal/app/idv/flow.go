package idv

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

const (
	metaKeyField   = "field"
	metaKeyValue   = "value"
	metaKeyAllowed = "allowed"
)

// InitiateCommand starts a real identity-verification session with the tenant's
// configured third-party provider.
type InitiateCommand struct {
	OrganizationID string
	UserID         *string
	Provider       string
	ReturnURL      string
	Locale         string
	Country        string
	IDType         string
	FullName       string
	IDNumber       string
	// Action selects the face-verification semantics; empty defaults to
	// domestic real-name verification. Country=CHN forces the domestic action.
	Action        string
	MetaInfo      string
	TargetFaceURL string
	ActorID       string
	IPAddress     string
	UserAgent     string
	RequestID     string
	TraceID       string
}

// InitiateOutcome is returned to the caller after the provider accepted the
// session and the pending record was persisted.
type InitiateOutcome struct {
	VerificationID string             `json:"verificationId"`
	Provider       string             `json:"provider"`
	Status         VerificationStatus `json:"status"`
	RedirectURL    string             `json:"redirectUrl,omitempty"`
	ExpiresAt      string             `json:"expiresAt,omitempty"`
}

// Initiate creates a provider session, then persists the pending verification
// together with the audit record and outbox event in one transaction.
func (s Service) Initiate(ctx context.Context, cmd InitiateCommand) (*InitiateOutcome, error) {
	if strings.TrimSpace(cmd.OrganizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if s.Providers == nil || s.Factory == nil {
		return nil, apperrors.NewUnavailable("identity verification provider is not configured")
	}

	action := domain.FaceAction(strings.TrimSpace(cmd.Action))
	if !action.Valid() {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "unsupported face action", map[string]any{
			metaKeyField:   "action",
			metaKeyValue:   string(action),
			metaKeyAllowed: []string{string(domain.ActionRealNameVerifyCN), string(domain.ActionLoginVerify), string(domain.ActionRealNameVerifyOverseas)},
		})
	}
	if action == "" {
		action = domain.ActionRealNameVerifyCN
	}
	// CHN subjects always use the domestic authoritative-source product; the
	// overseas eKYC route must never receive a mainland ID claim.
	if strings.EqualFold(strings.TrimSpace(cmd.Country), "CHN") && action == domain.ActionRealNameVerifyOverseas {
		action = domain.ActionRealNameVerifyCN
	}

	// Real-name verification is a one-way gate: an already-verified user must
	// not start another real-name session (design §3.1). Login verification is
	// repeatable and never flips the verified flag.
	if action.IsRealName() && cmd.UserID != nil && *cmd.UserID != "" {
		user, err := s.UOW.Users().GetByIDForOrganization(ctx, *cmd.UserID, cmd.OrganizationID)
		if err != nil {
			return nil, err
		}
		if user.IsVerified {
			return nil, apperrors.NewUnprocessableEntity("user is already real-name verified")
		}
	}

	bizSeq, err := newBizSeq()
	if err != nil {
		return nil, err
	}
	var subjectRefValue string
	if action.IsRealName() {
		subjectRefValue, err = subjectRef(cmd.IDNumber)
		if err != nil {
			return nil, err
		}
	}

	credentials, err := s.Providers.ResolveIDVProvider(ctx, cmd.OrganizationID, cmd.Provider)
	if err != nil {
		return nil, err
	}
	verifier, err := s.Factory.Build(*credentials)
	if err != nil {
		return nil, err
	}

	returnURL := cmd.ReturnURL
	if returnURL == "" {
		returnURL = credentials.ReturnURL
	}

	session, err := verifier.InitiateVerification(ctx, domain.InitiateOptions{
		UserID:         ptrString(cmd.UserID),
		OrganizationID: cmd.OrganizationID,
		RedirectURL:    returnURL,
		Locale:         cmd.Locale,
		Country:        cmd.Country,
		IDType:         cmd.IDType,
		FullName:       cmd.FullName,
		IDNumber:       cmd.IDNumber,
		Action:         action,
		BizSeq:         bizSeq,
		MetaInfo:       cmd.MetaInfo,
		TargetFaceURL:  cmd.TargetFaceURL,
	})
	if err != nil {
		return nil, providerCredentialError(err)
	}

	now := s.now()
	verification := &domain.Verification{
		ID:             s.newID(),
		OrganizationID: cmd.OrganizationID,
		UserID:         cmd.UserID,
		Provider:       credentials.Provider,
		Status:         domain.StatusPending,
		RedirectURL:    stringPtr(session.RedirectURL),
		SessionToken:   stringPtr(session.SessionToken),
		ProviderRef:    stringPtr(session.VerificationID),
		BizSeq:         stringPtr(bizSeq),
		Action:         stringPtr(string(action)),
		ProductCode:    stringPtr(session.ProductCode),
		SubjectRef:     stringPtr(subjectRefValue),
		InitiatedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.IDV().Create(ctx, verification); err != nil {
			return err
		}
		return s.embedAudit(ctx, tx, cmd, verification, "idv.verification.initiated", "")
	}); err != nil {
		return nil, err
	}

	return &InitiateOutcome{
		VerificationID: verification.ID,
		Provider:       credentials.Provider,
		Status:         verification.Status,
		RedirectURL:    session.RedirectURL,
		ExpiresAt:      session.ExpiresAt,
	}, nil
}

// Refresh polls the provider for a non-terminal verification and applies the
// result. It is best-effort: provider outages leave the record untouched and
// surface through logs, so a read request still returns the stored state.
func (s Service) Refresh(ctx context.Context, id, organizationID, actorID string) (VerificationDTO, error) {
	verification, err := s.UOW.IDV().GetByID(ctx, id, organizationID)
	if err != nil {
		return VerificationDTO{}, err
	}
	if verification.Status.IsTerminal() {
		return toDTO(verification), nil
	}
	if verification.ProviderRef == nil || *verification.ProviderRef == "" {
		return toDTO(verification), nil
	}
	if s.Providers == nil || s.Factory == nil {
		return toDTO(verification), nil
	}

	credentials, err := s.Providers.ResolveIDVProvider(ctx, organizationID, verification.Provider)
	if err != nil {
		slog.WarnContext(ctx, "identity verification refresh: provider resolution failed",
			"verification_id", verification.ID, "provider", verification.Provider, "error", err)
		return toDTO(verification), nil
	}
	// The persisted product code routes result queries to the right API
	// (e.g. eKYC_PRO → international CheckResult instead of DescribeFaceVerify).
	if verification.ProductCode != nil {
		credentials.ProductCode = *verification.ProductCode
	}
	verifier, err := s.Factory.Build(*credentials)
	if err != nil {
		slog.WarnContext(ctx, "identity verification refresh: provider build failed",
			"verification_id", verification.ID, "provider", verification.Provider, "error", err)
		return toDTO(verification), nil
	}

	result, err := s.queryResult(ctx, verifier, verification)
	if err != nil {
		slog.WarnContext(ctx, "identity verification refresh: provider query failed",
			"verification_id", verification.ID, "provider", verification.Provider, "error", err)
		return toDTO(verification), nil
	}
	updated, err := s.applyResult(ctx, id, organizationID, actorID, result, "idv.verification.refreshed")
	if err != nil {
		return VerificationDTO{}, err
	}
	return toDTO(updated), nil
}

// queryResult prefers the order-aware query when the row carries a BizSeq:
// Cloudauth-intl CheckResult requires the merchant sequence alongside the
// provider reference. Providers without the extension fall back to the plain
// reference query.
func (s Service) queryResult(ctx context.Context, verifier domain.IdentityVerifier, verification *domain.Verification) (*VerificationResult, error) {
	if querier, ok := verifier.(domain.OrderResultQuerier); ok && verification.BizSeq != nil && *verification.BizSeq != "" {
		return querier.GetVerificationResultWithOrder(ctx, *verification.ProviderRef, *verification.BizSeq)
	}
	return verifier.GetVerificationResult(ctx, *verification.ProviderRef)
}

// HandleCallback verifies a provider webhook signature and applies the result
// idempotently. The HTTP route is intentionally not registered in this batch
// (see the reported open item), so this entry point is exercised by tests only.
func (s Service) HandleCallback(ctx context.Context, organizationID, provider string, payload []byte, headers map[string]string) (*VerificationResult, error) {
	if s.Providers == nil || s.Factory == nil {
		return nil, apperrors.NewUnavailable("identity verification provider is not configured")
	}
	credentials, err := s.Providers.ResolveIDVProvider(ctx, organizationID, provider)
	if err != nil {
		return nil, err
	}
	verifier, err := s.Factory.Build(*credentials)
	if err != nil {
		return nil, err
	}
	result, err := verifier.VerifyCallback(ctx, payload, headers)
	if err != nil {
		return nil, err
	}

	verification, err := s.UOW.IDV().GetByProviderRef(ctx, result.ProviderName, result.VerificationID)
	if err != nil {
		return nil, err
	}
	if _, err := s.applyResult(ctx, verification.ID, organizationID, "", result, "idv.verification.callback"); err != nil {
		return nil, err
	}
	return result, nil
}

// HandlePublicCallback processes a provider webhook that arrives without a
// tenant context. The opaque provider reference identifies the owning tenant;
// only after that lookup does the service verify the signature with the
// tenant's configured secret. Unknown providers, unknown references and
// providers that are not enabled for the owning tenant all surface as
// not-found so the endpoint does not leak provider or tenant existence.
func (s Service) HandlePublicCallback(ctx context.Context, provider string, payload []byte, headers map[string]string) (*VerificationResult, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if s.Providers == nil || s.Factory == nil {
		return nil, apperrors.NewUnavailable("identity verification provider is not configured")
	}

	reference, err := s.Factory.CallbackReference(provider, payload)
	if err != nil {
		return nil, err
	}
	verification, err := s.UOW.IDV().GetByProviderRef(ctx, provider, reference)
	if err != nil {
		return nil, err
	}
	credentials, err := s.Providers.ResolveIDVProvider(ctx, verification.OrganizationID, provider)
	if err != nil {
		return nil, apperrors.NewNotFound("identity verification provider is not enabled for this tenant")
	}
	verifier, err := s.Factory.Build(*credentials)
	if err != nil {
		return nil, apperrors.NewNotFound("identity verification provider is not configured for this tenant")
	}
	result, err := verifier.VerifyCallback(ctx, payload, headers)
	if err != nil {
		return nil, err
	}
	if result.ProviderName != provider || result.VerificationID != reference {
		return nil, apperrors.NewUnauthorized("identity verification callback reference mismatch")
	}

	if _, err := s.applyResult(ctx, verification.ID, verification.OrganizationID, "", result, "idv.verification.callback"); err != nil {
		return nil, err
	}
	return result, nil
}

// applyResult re-reads the row inside the transaction so concurrent callbacks
// and polling are idempotent, then persists the state transition, the user's
// verified flag, audit and outbox event atomically.
func (s Service) applyResult(ctx context.Context, id, organizationID, actorID string, result *VerificationResult, eventType string) (*domain.Verification, error) {
	if result == nil || result.Status == domain.StatusError {
		return s.UOW.IDV().GetByID(ctx, id, organizationID)
	}

	var updated *domain.Verification
	// previousStatus + transitioned let the post-commit notification fire once
	// per real state change, even though callbacks and polling can both observe
	// the same result.
	var previousStatus domain.VerificationStatus
	transitioned := false
	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		verification, err := tx.IDV().GetByID(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if verification.Status.IsTerminal() {
			updated = verification
			return nil
		}
		if verification.Status == result.Status {
			// Duplicate webhook delivery or a poll that observed no transition:
			// the state is unchanged, so emitting another audit/outbox row would
			// record an event that did not happen.
			updated = verification
			return nil
		}

		previousStatus = verification.Status
		now := s.now()
		verification.Status = result.Status
		transitioned = true
		if result.FirstName != "" {
			verification.FirstName = stringPtr(result.FirstName)
		}
		if result.LastName != "" {
			verification.LastName = stringPtr(result.LastName)
		}
		if result.DateOfBirth != "" {
			verification.DateOfBirth = stringPtr(result.DateOfBirth)
		}
		if result.Nationality != "" {
			verification.Nationality = stringPtr(result.Nationality)
		}
		if result.IDType != "" {
			verification.IDType = stringPtr(result.IDType)
		}
		if result.Reason != "" {
			verification.Reason = stringPtr(result.Reason)
		}
		if result.Confidence > 0 {
			verification.Confidence = &result.Confidence
		}
		if verification.Status.IsTerminal() {
			verification.CompletedAt = &now
		}
		verification.UpdatedAt = now

		if verification.Status == domain.StatusApproved && verification.SubjectRef == nil {
			// De-identified subject reference: providers that return the ID
			// number only at completion (hosted capture flows) get the salted
			// SM3 hash here; Aliyun rows carry it from initiation.
			idNumber := result.IDNumber
			if idNumber == "" {
				idNumber = ptrString(verification.IDNumber)
			}
			ref, refErr := subjectRef(idNumber)
			if refErr != nil {
				return refErr
			}
			verification.SubjectRef = stringPtr(ref)
		}
		if err := tx.IDV().Update(ctx, verification); err != nil {
			return err
		}
		if verification.Status == domain.StatusApproved {
			// Only real-name actions flip the verified flag; login verification
			// must never mark a user real-name verified.
			if domain.FaceAction(ptrString(verification.Action)).IsRealName() &&
				verification.UserID != nil && *verification.UserID != "" {
				if err := markUserVerified(ctx, tx, *verification.UserID, organizationID, now); err != nil {
					return err
				}
			}
		}
		if err := s.embedAudit(ctx, tx, InitiateCommand{
			OrganizationID: organizationID,
			ActorID:        actorID,
		}, verification, eventType, result.GatewayCode); err != nil {
			return err
		}
		updated = verification
		return nil
	}); err != nil {
		return nil, err
	}
	if transitioned {
		s.notifyVerificationResult(ctx, updated, previousStatus)
	}
	return updated, nil
}

func markUserVerified(ctx context.Context, tx appuow.UnitOfWork, userID, organizationID string, now time.Time) error {
	user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
	if err != nil {
		var appErr *apperrors.Error
		if errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound {
			return nil
		}
		return err
	}
	if user.IsVerified {
		return nil
	}
	user.IsVerified = true
	user.UpdatedAt = now
	return tx.Users().UpdateForOrganization(ctx, user, organizationID)
}

func (s Service) embedAudit(ctx context.Context, tx appuow.UnitOfWork, cmd InitiateCommand, verification *domain.Verification, eventType, gatewayCode string) error {
	// Audit payload is strictly de-identified: BizSeq / CertifyId / GatewayCode
	// / result only. RealName and IDNumber must never appear here or in logs.
	payload := map[string]any{
		"provider": verification.Provider,
		"status":   string(verification.Status),
	}
	if verification.BizSeq != nil {
		payload["biz_seq"] = *verification.BizSeq
	}
	if verification.Action != nil {
		payload["action"] = *verification.Action
	}
	if verification.ProductCode != nil {
		payload["product_code"] = *verification.ProductCode
	}
	if verification.ProviderRef != nil {
		payload["certify_id"] = *verification.ProviderRef
	}
	if gatewayCode != "" {
		payload["gateway_code"] = gatewayCode
	}
	return audit.EmbedAuditEvent(
		idvClock{s.now},
		tx, ctx,
		cmd.IPAddress, cmd.UserAgent, "management", cmd.RequestID, cmd.TraceID,
		cmd.OrganizationID, cmd.ActorID,
		eventType, "identity_verification", verification.ID,
		"identity_verification", eventType,
		payload,
	)
}

type idvClock struct{ now func() time.Time }

func (c idvClock) Now() time.Time { return c.now() }

func ptrString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
