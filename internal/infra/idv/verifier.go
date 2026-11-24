package idv

import (
	domainidv "github.com/guliangiam/guliangiam/internal/domain/idv"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type VerificationStatus = domainidv.VerificationStatus
type VerificationResult = domainidv.VerificationResult
type InitiateResult = domainidv.InitiateResult
type InitiateOptions = domainidv.InitiateOptions
type IdentityVerifier = domainidv.IdentityVerifier
type ProviderCredentials = domainidv.ProviderCredentials
type VerifierFactory = domainidv.VerifierFactory
type OrderResultQuerier = domainidv.OrderResultQuerier

type FaceAction = domainidv.FaceAction

const (
	ActionRealNameVerifyCN       = domainidv.ActionRealNameVerifyCN
	ActionLoginVerify            = domainidv.ActionLoginVerify
	ActionRealNameVerifyOverseas = domainidv.ActionRealNameVerifyOverseas
)

type DocumentType = domainidv.DocumentType
type DocumentImage = domainidv.DocumentImage
type DocumentFields = domainidv.DocumentFields
type DocumentRecognizer = domainidv.DocumentRecognizer
type OCRConfig = domainidv.OCRConfig
type OCRFactory = domainidv.OCRFactory

const (
	DocumentIDCard        = domainidv.DocumentIDCard
	DocumentPassport      = domainidv.DocumentPassport
	DocumentDriverLicense = domainidv.DocumentDriverLicense
)

// tencentOCRProviderName is the provider registration key for Tencent Cloud
// 文字识别 OCR; it is declared in tencent_ocr.go.

var _ OCRFactory = Factory{}

const (
	StatusPending    VerificationStatus = domainidv.StatusPending
	StatusInProgress VerificationStatus = domainidv.StatusInProgress
	StatusApproved   VerificationStatus = domainidv.StatusApproved
	StatusRejected   VerificationStatus = domainidv.StatusRejected
	StatusExpired    VerificationStatus = domainidv.StatusExpired
	StatusError      VerificationStatus = domainidv.StatusError
)

// Factory builds a real provider verifier from tenant-scoped credentials.
// It replaces the former FakeVerifier: unknown or incomplete configurations are
// rejected instead of silently returning a fabricated approval.
type Factory struct{}

var _ VerifierFactory = Factory{}

func NewFactory() Factory { return Factory{} }

func (Factory) CallbackReference(provider string, payload []byte) (string, error) {
	switch provider {
	case jumioProviderName:
		return jumioCallbackReference(payload)
	case aliyunProviderName:
		return aliyunCallbackReference(payload)
	case tencentIDVProviderName:
		return tencentCallbackReference(payload)
	case onfidoProviderName:
		return onfidoCallbackReference(payload)
	case veriffProviderName:
		return veriffCallbackReference(payload)
	case personaProviderName:
		return personaCallbackReference(payload)
	default:
		return "", apperrors.NewNotFound("idv: unknown identity provider")
	}
}

func (Factory) Build(credentials ProviderCredentials) (IdentityVerifier, error) {
	switch credentials.Provider {
	case jumioProviderName:
		if credentials.APIKey == "" || credentials.APISecret == "" {
			return nil, apperrors.NewInvalidArgument("jumio: api token and api secret are required")
		}
		return NewJumioVerifierWithEndpoint(
			credentials.APIKey,
			credentials.APISecret,
			credentials.WebhookSecret,
			credentials.Sandbox,
			credentials.Endpoint,
		), nil
	case aliyunProviderName:
		if credentials.APIKey == "" || credentials.APISecret == "" {
			return nil, apperrors.NewInvalidArgument("aliyun_idv: accessKeyId and accessKeySecret are required")
		}
		return NewAliyunIDVVerifierWithOptions(
			credentials.APIKey,
			credentials.APISecret,
			credentials.Region,
			credentials.WebhookSecret,
			credentials.Endpoint,
			credentials.SceneID,
			credentials.ProductCode,
			"",
		), nil
	case tencentIDVProviderName:
		if credentials.APIKey == "" || credentials.APISecret == "" {
			return nil, apperrors.NewInvalidArgument("tencent_idv: SecretId and SecretKey are required")
		}
		return NewTencentIDVVerifierWithOptions(
			credentials.APIKey,
			credentials.APISecret,
			credentials.Region,
			credentials.Endpoint,
			credentials.SceneID,
			credentials.WebhookSecret,
		), nil
	case onfidoProviderName:
		if credentials.APIKey == "" {
			return nil, apperrors.NewInvalidArgument("onfido: api token is required")
		}
		return NewOnfidoVerifier(credentials.APIKey, credentials.WebhookSecret, credentials.SceneID, credentials.Endpoint), nil
	case veriffProviderName:
		if credentials.APIKey == "" || credentials.APISecret == "" {
			return nil, apperrors.NewInvalidArgument("veriff: api key and shared secret are required")
		}
		return NewVeriffVerifier(credentials.APIKey, credentials.APISecret, credentials.WebhookSecret, credentials.Endpoint), nil
	case personaProviderName:
		if credentials.APIKey == "" {
			return nil, apperrors.NewInvalidArgument("persona: api key is required")
		}
		return NewPersonaVerifier(credentials.APIKey, credentials.WebhookSecret, credentials.SceneID, credentials.Endpoint), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("idv: unsupported provider %q", credentials.Provider)
	}
}

// BuildOCR builds a document recognizer. Aliyun and Tencent Cloud are the
// implemented providers; the Tencent OCR adapter reuses the SecretId/SecretKey
// of the Tencent 实人认证 provider row.
func (Factory) BuildOCR(config OCRConfig) (DocumentRecognizer, error) {
	switch config.Provider {
	case aliyunOCRProviderName, aliyunProviderName:
		if config.AccessKey == "" || config.SecretKey == "" {
			return nil, apperrors.NewInvalidArgument("aliyun_ocr: accessKeyId and accessKeySecret are required")
		}
		return NewAliyunOCRRecognizer(config), nil
	case tencentOCRProviderName, tencentIDVProviderName:
		if config.AccessKey == "" || config.SecretKey == "" {
			return nil, apperrors.NewInvalidArgument("tencent_ocr: SecretId and SecretKey are required")
		}
		return NewTencentOCRRecognizer(config), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("ocr: unsupported provider %q", config.Provider)
	}
}
