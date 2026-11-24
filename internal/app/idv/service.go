package idv

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Service exposes identity-verification use cases.
type Service struct {
	UOW   appuow.UnitOfWork
	UUID  uuid.Generator
	Clock func() time.Time
	// Providers resolves the tenant's enabled IDVerification provider and its
	// decrypted credentials. Nil means no real provider is wired, and every
	// initiating use case fails instead of fabricating a result.
	Providers IDVProviderResolver
	// Factory builds the real HTTP adapter for the resolved provider. Injected
	// from the composition root so app never imports infra.
	Factory domain.VerifierFactory
	// OCRFactory builds the real document recognizer. Nil means OCR is not
	// wired, and recognition fails with unavailable instead of fabricating data.
	OCRFactory domain.OCRFactory
	// Notifier publishes terminal verification results to the subject user's
	// personal notification inbox. Nil disables the notification fan-out.
	Notifier Notifier
}

// providerCredentialError remaps a provider-side credential rejection to a
// dependency error. A rejected AccessKey is an operator configuration problem,
// not the end user's authentication failure; surfacing it as 401 would make the
// UI treat it as an expired session and hide the real cause.
func providerCredentialError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		return err
	}
	switch appErr.Code {
	case apperrors.CodeUnauthorized, apperrors.CodeForbidden:
		// Keep the infra Meta: providerCode must survive this remap, or the
		// credential-rejection surface collapses back into an undiagnosable 503.
		return apperrors.NewWithMeta(apperrors.CodeUnavailable, "identity provider rejected its credentials; update the provider configuration", appErr.Meta)
	}
	return err
}

// IDVProviderResolver is implemented by app/providers. It returns decrypted
// credentials for one tenant-scoped provider.
type IDVProviderResolver interface {
	ResolveIDVProvider(ctx context.Context, organizationID, providerName string) (*domain.ProviderCredentials, error)
}

// NewService constructs an IDV Service with default UUID generator and clock.
func NewService(uow appuow.UnitOfWork) Service {
	return Service{
		UOW:   uow,
		UUID:  uuid.V7Generator{},
		Clock: func() time.Time { return time.Now().UTC() },
	}
}

// ─── Domain type aliases ───────────────────────────────────────────────────

type VerificationStatus = domain.VerificationStatus

func (s Service) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now().UTC()
}
func (s Service) newID() string {
	if s.UUID != nil {
		if id, err := s.UUID.NewString(); err == nil {
			return id
		}
	}
	id, _ := uuid.V7Generator{}.NewString()
	return id
}

// ─── Aliases ──────────────────────────────────────────────────────────────

type (
	ListQuery          = domain.ListQuery
	CreateCommand      = domain.CreateCommand
	VerificationResult = domain.VerificationResult
)

const (
	StatusPending    = domain.StatusPending
	StatusInProgress = domain.StatusInProgress
	StatusApproved   = domain.StatusApproved
	StatusRejected   = domain.StatusRejected
	StatusExpired    = domain.StatusExpired
)

// CompleteCommand records the provider result against an existing verification.
type CompleteCommand struct {
	ID             string
	OrganizationID string
	Status         VerificationStatus
	FirstName      *string
	LastName       *string
	DateOfBirth    *string
	Nationality    *string
	IDType         *string
	IDNumber       *string
	Confidence     *float64
	Reason         *string
	RawResult      json.RawMessage
	ProviderRef    *string
}

// ─── DTO ──────────────────────────────────────────────────────────────────

type VerificationDTO struct {
	ID                 string     `json:"id"`
	OrganizationID     string     `json:"organizationId"`
	UserID             *string    `json:"userId,omitempty"`
	Provider           string     `json:"provider"`
	Status             string     `json:"status"`
	RedirectURL        *string    `json:"redirectUrl,omitempty"`
	SessionTokenSet    bool       `json:"sessionTokenSet"`
	SessionTokenMasked string     `json:"sessionTokenMasked,omitempty"`
	ProviderRef        *string    `json:"providerRef,omitempty"`
	BizSeq             *string    `json:"bizSeq,omitempty"`
	Action             *string    `json:"action,omitempty"`
	ProductCode        *string    `json:"productCode,omitempty"`
	FirstName          *string    `json:"firstName,omitempty"`
	LastName           *string    `json:"lastName,omitempty"`
	DateOfBirth        *string    `json:"dateOfBirth,omitempty"`
	Nationality        *string    `json:"nationality,omitempty"`
	IDType             *string    `json:"idType,omitempty"`
	IDNumberSet        bool       `json:"idNumberSet"`
	IDNumberMasked     string     `json:"idNumberMasked,omitempty"`
	Confidence         *float64   `json:"confidence,omitempty"`
	Reason             *string    `json:"reason,omitempty"`
	RawResultSet       bool       `json:"rawResultSet"`
	RawResultMasked    string     `json:"rawResultMasked,omitempty"`
	InitiatedAt        time.Time  `json:"initiatedAt"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

func toDTO(v *domain.Verification) VerificationDTO {
	return VerificationDTO{
		ID:                 v.ID,
		OrganizationID:     v.OrganizationID,
		UserID:             v.UserID,
		Provider:           v.Provider,
		Status:             string(v.Status),
		RedirectURL:        v.RedirectURL,
		SessionTokenSet:    sessionTokenSet(v.SessionToken),
		SessionTokenMasked: maskSessionToken(v.SessionToken),
		ProviderRef:        v.ProviderRef,
		BizSeq:             v.BizSeq,
		Action:             v.Action,
		ProductCode:        v.ProductCode,
		FirstName:          v.FirstName,
		LastName:           v.LastName,
		DateOfBirth:        v.DateOfBirth,
		Nationality:        v.Nationality,
		IDType:             v.IDType,
		IDNumberSet:        piiValueSet(v.IDNumber),
		IDNumberMasked:     maskPII(v.IDNumber),
		Confidence:         v.Confidence,
		Reason:             v.Reason,
		RawResultSet:       len(v.RawResult) > 0,
		RawResultMasked:    maskRawResult(v.RawResult),
		InitiatedAt:        v.InitiatedAt,
		CompletedAt:        v.CompletedAt,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
	}
}

func sessionTokenSet(token *string) bool {
	return token != nil && *token != ""
}

// maskSessionToken returns first 2 + *** + last 2 characters, or empty string.
// Short tokens (< 6 chars) are fully masked; the plaintext is never returned.
func maskSessionToken(token *string) string {
	if !sessionTokenSet(token) {
		return ""
	}
	if len(*token) < 6 {
		return "******"
	}
	return (*token)[:2] + "***" + (*token)[len(*token)-2:]
}

func piiValueSet(value *string) bool {
	return value != nil && *value != ""
}

// maskPII returns first 2 + *** + last 2 characters, or empty string.
// Short values (< 6 chars) are fully masked; the plaintext is never returned.
func maskPII(value *string) string {
	if !piiValueSet(value) {
		return ""
	}
	if len(*value) < 6 {
		return "******"
	}
	return (*value)[:2] + "***" + (*value)[len(*value)-2:]
}

// maskRawResult never returns the provider payload: raw_result carries PII and
// audit material, so the read DTO exposes only the set flag plus a fixed mask.
func maskRawResult(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	return "***"
}

// ─── Use cases ────────────────────────────────────────────────────────────

func (s Service) Create(ctx context.Context, cmd CreateCommand) (VerificationDTO, error) {
	now := s.now()
	v := &domain.Verification{
		ID:             s.newID(),
		OrganizationID: cmd.OrganizationID,
		UserID:         cmd.UserID,
		Provider:       cmd.Provider,
		Status:         cmd.Status,
		RedirectURL:    cmd.RedirectURL,
		SessionToken:   cmd.SessionToken,
		ProviderRef:    cmd.ProviderRef,
		BizSeq:         cmd.BizSeq,
		Action:         cmd.Action,
		ProductCode:    cmd.ProductCode,
		SubjectRef:     cmd.SubjectRef,
		InitiatedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if v.Status == "" {
		v.Status = domain.StatusPending
	}
	if err := s.UOW.IDV().Create(ctx, v); err != nil {
		return VerificationDTO{}, err
	}
	return toDTO(v), nil
}

func (s Service) GetByID(ctx context.Context, id, organizationID string) (*VerificationDTO, error) {
	v, err := s.UOW.IDV().GetByID(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	dto := toDTO(v)
	return &dto, nil
}

func (s Service) List(ctx context.Context, q ListQuery) ([]VerificationDTO, int64, error) {
	items, total, err := s.UOW.IDV().List(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]VerificationDTO, 0, len(items))
	for _, v := range items {
		dtos = append(dtos, toDTO(v))
	}
	return dtos, total, nil
}

// Complete records the outcome of an identity-verification attempt.
// Returns InvalidArgument if the record is not in pending state.
//
// It has no HTTP entry point on purpose: provider callbacks go through
// applyResult, which also flips user.IsVerified and emits audit/outbox rows.
// Exposing this method would persist a status without those effects, so the
// transport layer must not route to it.
func (s Service) Complete(ctx context.Context, cmd CompleteCommand) (VerificationDTO, error) {
	if cmd.Status == "" {
		return VerificationDTO{}, apperrors.NewInvalidArgument("verification status is required")
	}
	if !cmd.Status.Persistable() {
		return VerificationDTO{}, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"verification status "+string(cmd.Status)+" cannot be persisted",
			map[string]any{
				"field": "status",
				"value": string(cmd.Status),
				"allowed": []string{
					string(domain.StatusPending),
					string(domain.StatusInProgress),
					string(domain.StatusApproved),
					string(domain.StatusRejected),
					string(domain.StatusExpired),
				},
			},
		)
	}
	v, err := s.UOW.IDV().GetByID(ctx, cmd.ID, cmd.OrganizationID)
	if err != nil {
		return VerificationDTO{}, err
	}
	if v.Status != domain.StatusPending {
		return VerificationDTO{}, apperrors.NewInvalidArgumentf(
			"verification %s already in status %s", cmd.ID, v.Status,
		)
	}
	now := s.now()
	v.Status = cmd.Status
	if cmd.FirstName != nil {
		v.FirstName = cmd.FirstName
	}
	if cmd.LastName != nil {
		v.LastName = cmd.LastName
	}
	if cmd.DateOfBirth != nil {
		v.DateOfBirth = cmd.DateOfBirth
	}
	if cmd.Nationality != nil {
		v.Nationality = cmd.Nationality
	}
	if cmd.IDType != nil {
		v.IDType = cmd.IDType
	}
	if cmd.IDNumber != nil {
		v.IDNumber = cmd.IDNumber
	}
	if cmd.Confidence != nil {
		v.Confidence = cmd.Confidence
	}
	if cmd.Reason != nil {
		v.Reason = cmd.Reason
	}
	if cmd.RawResult != nil {
		v.RawResult = cmd.RawResult
	}
	if cmd.ProviderRef != nil {
		v.ProviderRef = cmd.ProviderRef
	}
	v.CompletedAt = &now
	v.UpdatedAt = now
	if err := s.UOW.IDV().Update(ctx, v); err != nil {
		return VerificationDTO{}, err
	}
	return toDTO(v), nil
}

func (s Service) Delete(ctx context.Context, id, organizationID string) error {
	repo := s.UOW.IDV()
	_, err := repo.GetByID(ctx, id, organizationID)
	if err != nil {
		return err
	}
	return repo.Delete(ctx, id, organizationID)
}
