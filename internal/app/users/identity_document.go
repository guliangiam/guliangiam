package users

import (
	"context"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// IdentityDocumentRecognitionRequest is the provider-neutral OCR input.
// The users application service owns the request shape so it does not import
// the IDV application or infra packages.
type IdentityDocumentRecognitionRequest struct {
	OrganizationID string
	UserID         string
	DocumentType   string
	Side           string
	MediaType      string
	Data           []byte
}

// IdentityDocumentFields mirrors the recognized identity fields. It is a plain
// DTO so the transport layer can render it without importing the IDV domain.
type IdentityDocumentFields struct {
	FullName         string  `json:"fullName,omitempty"`
	IDNumber         string  `json:"idNumber,omitempty"`
	IDType           string  `json:"idType,omitempty"`
	Gender           string  `json:"gender,omitempty"`
	Birthday         string  `json:"birthday,omitempty"`
	Nationality      string  `json:"nationality,omitempty"`
	Address          string  `json:"address,omitempty"`
	IssuingAuthority string  `json:"issuingAuthority,omitempty"`
	ValidFrom        string  `json:"validFrom,omitempty"`
	ValidTo          string  `json:"validTo,omitempty"`
	Confidence       float64 `json:"confidence,omitempty"`
}

// IdentityDocumentRecognizer performs document OCR. Implemented by a bootstrap
// adapter over the IDV application service.
type IdentityDocumentRecognizer interface {
	RecognizeIdentityDocument(ctx context.Context, request IdentityDocumentRecognitionRequest) (IdentityDocumentFields, error)
}

// RecognizeUserIdentityDocument runs OCR for a user that belongs to the trusted
// organization. Ownership is checked before the provider is called so a
// cross-tenant user id cannot be used to spend another tenant's OCR quota.
func (s *Service) RecognizeUserIdentityDocument(ctx context.Context, request IdentityDocumentRecognitionRequest) (IdentityDocumentFields, error) {
	if err := requireOrganizationID(request.OrganizationID); err != nil {
		return IdentityDocumentFields{}, err
	}
	if strings.TrimSpace(request.UserID) == "" {
		return IdentityDocumentFields{}, apperrors.NewInvalidArgument("userId is required")
	}
	if _, err := s.UOW.Users().GetByIDForOrganization(ctx, request.UserID, request.OrganizationID); err != nil {
		return IdentityDocumentFields{}, err
	}
	if s.DocumentRecognizer == nil {
		return IdentityDocumentFields{}, apperrors.NewUnavailable("identity document OCR provider is not configured")
	}
	return s.DocumentRecognizer.RecognizeIdentityDocument(ctx, request)
}
