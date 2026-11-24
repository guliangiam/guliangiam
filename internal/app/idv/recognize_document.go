package idv

import (
	"context"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	maxDocumentImageBytes = 5 << 20
	// aliyunIDVProviderName is the provider registration key whose credential
	// is reused for OCR. OCR is a separate Aliyun product but shares the RAM
	// AccessKey, so no new provider table is needed.
	aliyunIDVProviderName = "aliyun_idv"
	// tencentIDVProviderName is the Tencent Cloud 实人认证 provider key; its
	// SecretId/SecretKey are reused for the Tencent OCR product the same way.
	tencentIDVProviderName = "tencent_idv"
)

// ocrCapableIDVProviders lists the IDV provider kinds that have an OCR product
// behind the same credentials, in resolution priority order.
var ocrCapableIDVProviders = []string{aliyunIDVProviderName, tencentIDVProviderName}

// RecognizeDocumentCommand carries one uploaded document image.
type RecognizeDocumentCommand struct {
	OrganizationID string
	UserID         string
	DocumentType   string
	Side           string
	MediaType      string
	Data           []byte
}

// RecognizeDocument runs OCR on one document image and returns the recognized
// fields. It never persists anything: the operator reviews the values and the
// normal profile update writes them.
func (s Service) RecognizeDocument(ctx context.Context, cmd RecognizeDocumentCommand) (*domain.DocumentFields, error) {
	if strings.TrimSpace(cmd.OrganizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(cmd.UserID) == "" {
		return nil, apperrors.NewInvalidArgument("userId is required")
	}
	documentType, ok := domain.ParseDocumentType(strings.TrimSpace(cmd.DocumentType))
	if !ok {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "documentType is not supported", map[string]any{
			"field": "documentType",
			"allowed": []string{
				string(domain.DocumentIDCard),
				string(domain.DocumentPassport),
				string(domain.DocumentDriverLicense),
			},
		})
	}
	mediaType := strings.ToLower(strings.TrimSpace(cmd.MediaType))
	switch mediaType {
	case "image/jpeg", "image/jpg", "image/png", "image/webp":
	default:
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "image must be jpeg, png or webp", map[string]any{
			"field": "file",
		})
	}
	if len(cmd.Data) == 0 {
		return nil, apperrors.NewInvalidArgument("image body is empty")
	}
	if len(cmd.Data) > maxDocumentImageBytes {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "image exceeds the 5MiB limit", map[string]any{
			"field": "file", "value": len(cmd.Data), "max": maxDocumentImageBytes,
		})
	}
	if s.Providers == nil || s.OCRFactory == nil {
		return nil, apperrors.NewUnavailable("identity document OCR provider is not configured")
	}

	credentials, err := s.resolveOCRProviderCredentials(ctx, cmd.OrganizationID)
	if err != nil {
		return nil, err
	}

	recognizer, err := s.OCRFactory.BuildOCR(domain.OCRConfig{
		Provider:  credentials.Provider,
		AccessKey: credentials.APIKey,
		SecretKey: credentials.APISecret,
		Region:    credentials.Region,
		// The OCR product has its own endpoint; the CloudAuth endpoint stored on
		// the provider row must not be reused here.
	})
	if err != nil {
		return nil, err
	}

	fields, err := recognizer.RecognizeDocument(ctx, documentType, domain.DocumentImage{
		Side:      strings.TrimSpace(cmd.Side),
		MediaType: mediaType,
		Data:      cmd.Data,
	})
	if err != nil {
		return nil, providerCredentialError(err)
	}
	if fields != nil {
		fields.IDType = string(documentType)
	}
	return fields, nil
}

// resolveOCRProviderCredentials resolves the tenant's OCR-capable IDV provider
// (Aliyun first, then Tencent Cloud). A tenant with no enabled provider reads as
// "not configured" (503); a tenant whose provider exists but has incomplete
// credentials reads as an actionable operator error (422) instead of being
// masked as unavailable. Fallback to the next kind is preserved: a complete
// Tencent provider still wins over a broken Aliyun one.
func (s Service) resolveOCRProviderCredentials(ctx context.Context, organizationID string) (*domain.ProviderCredentials, error) {
	var incompleteErr error
	for _, providerName := range ocrCapableIDVProviders {
		credentials, err := s.Providers.ResolveIDVProvider(ctx, organizationID, providerName)
		if err == nil {
			return credentials, nil
		}
		if appErr, ok := err.(*apperrors.Error); ok {
			switch appErr.Code {
			case apperrors.CodeUnprocessableEntity:
				// Configured but incomplete: remember the actionable reason and
				// keep trying the next OCR-capable kind.
				if incompleteErr == nil {
					incompleteErr = err
				}
				continue
			case apperrors.CodeInvalidArgument:
				// No enabled provider of this kind: try the next one.
				continue
			}
		}
		return nil, err
	}
	if incompleteErr != nil {
		return nil, incompleteErr
	}
	return nil, apperrors.NewUnavailable("identity document OCR provider is not configured")
}
