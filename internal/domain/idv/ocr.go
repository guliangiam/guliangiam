package idv

import "context"

// DocumentType is the closed set of identity documents the platform can
// recognize. The values are the API contract used by the transport layer.
type DocumentType string

const (
	DocumentIDCard        DocumentType = "id_card"
	DocumentPassport      DocumentType = "passport"
	DocumentDriverLicense DocumentType = "driver_license"
)

// ParseDocumentType normalizes user input and reports whether it is supported.
func ParseDocumentType(value string) (DocumentType, bool) {
	switch DocumentType(value) {
	case DocumentIDCard, DocumentPassport, DocumentDriverLicense:
		return DocumentType(value), true
	default:
		return "", false
	}
}

// DocumentImage is one side of an identity document. Data is the raw image
// bytes; the transport layer enforces the size and media-type limits before it
// reaches an adapter.
type DocumentImage struct {
	Side      string
	MediaType string
	Data      []byte
}

// DocumentFields is the provider-neutral recognition result. Only fields the
// provider actually returned are populated; the caller decides whether the
// operator must complete the rest manually.
type DocumentFields struct {
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

// DocumentRecognizer performs OCR on one document image. Implementations must
// translate provider failures into internal/platform/errors.
type DocumentRecognizer interface {
	RecognizeDocument(ctx context.Context, documentType DocumentType, image DocumentImage) (*DocumentFields, error)
}

// OCRConfig carries the tenant-scoped credential needed to build an OCR
// recognizer. It is deliberately separate from ProviderCredentials because the
// OCR product uses a different endpoint and API version than CloudAuth.
type OCRConfig struct {
	Provider  string
	AccessKey string
	SecretKey string
	Region    string
	Endpoint  string
}

// OCRFactory builds a recognizer from tenant-scoped credentials. Implemented by
// infra and injected at the composition root so app never imports infra.
type OCRFactory interface {
	BuildOCR(config OCRConfig) (DocumentRecognizer, error)
}
