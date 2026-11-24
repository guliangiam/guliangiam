package captcha

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Factory creates and manages CAPTCHA providers.
type Factory struct {
	providers map[string]Provider
}

// NewFactory creates a new CAPTCHA factory.
func NewFactory() *Factory {
	return &Factory{
		providers: make(map[string]Provider),
	}
}

// Register registers a CAPTCHA provider for a given type.
func (f *Factory) Register(providerType string, provider Provider) {
	f.providers[providerType] = provider
}

// GetProvider returns the CAPTCHA provider for the given type.
func (f *Factory) GetProvider(providerType string) (Provider, error) {
	p, ok := f.providers[providerType]
	if !ok {
		return nil, apperrors.NewNotFoundf("captcha provider not found: %s", providerType)
	}
	return p, nil
}

// GenerateCaptcha generates a new CAPTCHA challenge using the specified provider
// type. Server-side generated providers return the captcha ID plus base64 images:
//   - "default": (id, PNG, "", "") — shape is ignored
//   - "go_captcha": (id, master JPEG, sub image, resolved shape) — sub is the
//     click hint thumb or the slide tile; an empty shape falls back to "click"
//
// Third-party providers render client-side with their site key and are rejected here.
func (f *Factory) GenerateCaptcha(ctx context.Context, providerType, shape string) (captchaID, imageBase64, subImageBase64, resolvedShape string, err error) {
	switch providerType {
	case "default":
		dp, err := f.GetProvider("default")
		if err != nil {
			return "", "", "", "", err
		}
		defaultProv, ok := dp.(*DefaultProvider)
		if !ok {
			return "", "", "", "", apperrors.NewInvalidArgument("default captcha provider is not a DefaultProvider")
		}
		id, imgBytes, err := defaultProv.Generate(ctx)
		if err != nil {
			return "", "", "", "", apperrors.WrapInternalf(err, "generate default captcha")
		}
		return id, base64.StdEncoding.EncodeToString(imgBytes), "", "", nil
	case "go_captcha":
		gp, err := f.GetProvider("go_captcha")
		if err != nil {
			return "", "", "", "", err
		}
		goProv, ok := gp.(*GoCaptchaProvider)
		if !ok {
			return "", "", "", "", apperrors.NewInvalidArgument("go_captcha provider is not a GoCaptchaProvider")
		}
		res, err := goProv.Generate(ctx, shape)
		if err != nil {
			return "", "", "", "", apperrors.WrapInternalf(err, "generate go_captcha challenge")
		}
		return res.ID, base64.StdEncoding.EncodeToString(res.MasterImage), base64.StdEncoding.EncodeToString(res.SubImage), res.Shape, nil
	default:
		// Third-party providers are not generated server-side; the client uses the site key.
		return "", "", "", "", apperrors.NewInvalidArgumentf("captcha generation for provider type %q is handled client-side", providerType)
	}
}

// VerifyCaptcha verifies a CAPTCHA token using the specified provider type.
func (f *Factory) VerifyCaptcha(ctx context.Context, providerType, token, remoteIP string) (bool, error) {
	p, err := f.GetProvider(providerType)
	if err != nil {
		return false, err
	}
	return p.Verify(ctx, token, remoteIP)
}

// VerifyCaptchaWithRow verifies using the credentials stored on a
// captcha_providers row, so providers configured in the console are actually
// consumed (the row-backed counterpart of VerifyCaptcha). A row that carries no
// credentials at all falls back to the environment-registered provider; "default"
// and unknown types always resolve through the factory.
func (f *Factory) VerifyCaptchaWithRow(ctx context.Context, providerType, siteKey, secretKey, token, remoteIP string, config map[string]any) (bool, error) {
	if strings.TrimSpace(siteKey) == "" && strings.TrimSpace(secretKey) == "" {
		return f.VerifyCaptcha(ctx, providerType, token, remoteIP)
	}
	p, err := NewProviderFromRow(providerType, siteKey, secretKey, config)
	if err != nil {
		return f.VerifyCaptcha(ctx, providerType, token, remoteIP)
	}
	return p.Verify(ctx, token, remoteIP)
}

// NewProvider creates a CAPTCHA provider based on the provider type and configuration.
// This is a convenience function for creating providers without a factory. It only
// carries two credential slots: providers whose official contract needs more
// (Aliyun CAPTCHA 2.0 SceneId, Tencent CAPTCHA CAM SecretId/SecretKey,
// reCAPTCHA Enterprise projectId, NetEase Yidun secretId) map the slots to the
// two primary credentials and the rest must come from New<Provider>
// constructors, NewProviderFromRow or the env runtime path.
func NewProvider(providerType, secretKey, siteKey string) (Provider, error) {
	switch providerType {
	case "default":
		return NewDefaultProvider(NewMemoryStore(5 * time.Minute)), nil
	case "geetest":
		return NewGeeTestProvider(siteKey, secretKey), nil
	case "hcaptcha":
		return NewHCaptchaProvider(secretKey), nil
	case "recaptcha":
		return NewReCAPTCHAV2Provider(secretKey), nil
	case "recaptcha_v3":
		return NewReCAPTCHAV3Provider(secretKey, 0, ""), nil
	case "turnstile", "cloudflare":
		// "cloudflare" is the key rows written before the vocabulary was aligned
		// with the provider kind still carry; it resolves to the same adapter.
		return NewCloudflareTurnstileProvider(secretKey), nil
	case "aliyun_captcha", "aliyun":
		// OPEN-API-75 gap ①: Aliyun CAPTCHA 2.0 signs requests with
		// AccessKeyId/AccessKeySecret (ACS3-HMAC-SHA256). The two-slot mapping is
		// siteKey→AccessKeyID, secretKey→AccessKeySecret; SceneId (AppKey) is not
		// expressible here — use NewAliyunCaptchaProvider or the env runtime path.
		return NewAliyunCaptchaProvider(siteKey, secretKey, ""), nil
	case "tencent_captcha":
		// siteKey→CaptchaAppId, secretKey→AppSecretKey; the CAM SecretId/SecretKey
		// required for TC3-HMAC-SHA256 signing are not expressible in this
		// two-slot mapping (verification then fails closed as Unavailable) — use
		// NewTencentCaptchaProvider or the env runtime path.
		return NewTencentCaptchaProvider("", "", siteKey, secretKey), nil
	case "recaptcha_enterprise":
		// siteKey→site key, secretKey→API key; the project id required by the
		// assessments endpoint is not expressible here — use
		// NewReCAPTCHAEnterpriseProvider, NewProviderFromRow or the env path.
		return NewReCAPTCHAEnterpriseProvider(siteKey, secretKey, "", 0, ""), nil
	case "netease_yidun":
		// siteKey→captchaId, secretKey→business secret; the secretId required by
		// the verify endpoint is not expressible here — use NewNetEaseYidunProvider,
		// NewProviderFromRow or the env path.
		return NewNetEaseYidunProvider(siteKey, "", secretKey), nil
	case "baidu_captcha":
		// siteKey→app_id, secretKey→private_key; the optional app_name only
		// travels through NewProviderFromRow / the env path.
		return NewBaiduCaptchaProvider(siteKey, secretKey, ""), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("unsupported captcha provider: %s", providerType)
	}
}
