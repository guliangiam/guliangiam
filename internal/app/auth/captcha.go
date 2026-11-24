package auth

import "context"

// CaptchaFactory is the interface for generating and verifying CAPTCHAs.
// This interface is implemented by infra/captcha.Factory.
type CaptchaFactory interface {
	// GenerateCaptcha generates a new CAPTCHA challenge.
	// Returns the captcha ID, the base64-encoded master image, the optional
	// base64 sub image (go_captcha click thumb / slide tile) and the resolved
	// shape ("" for the default image captcha).
	GenerateCaptcha(ctx context.Context, providerType, shape string) (id string, imageBase64 string, subImageBase64 string, resolvedShape string, err error)
	// VerifyCaptcha verifies a CAPTCHA response.
	VerifyCaptcha(ctx context.Context, providerType string, token string, remoteIP string) (bool, error)
	// VerifyCaptchaWithRow verifies using the credentials stored on a
	// captcha_providers row (site key, secret key and the JSONB config carrying
	// the extra credentials some vendors' official contracts require, e.g.
	// Aliyun SceneId or Tencent CAM SecretId/SecretKey). Rows without any
	// credentials fall back to the environment-registered provider.
	VerifyCaptchaWithRow(ctx context.Context, providerType, siteKey, secretKey, token, remoteIP string, config map[string]any) (bool, error)
}

// CaptchaGenerateResult contains the result of generating a CAPTCHA.
// Shape / CaptchaImageSub are only set by the go_captcha provider (click hint
// thumb or slide tile); the default image captcha keeps the historical fields.
type CaptchaGenerateResult struct {
	CaptchaID       string `json:"captchaId"`
	CaptchaImage    string `json:"captchaImage"`
	Type            string `json:"type"`
	Shape           string `json:"shape,omitempty"`
	CaptchaImageSub string `json:"captchaImageSub,omitempty"`
}

// CaptchaVerifyResult contains the result of verifying a CAPTCHA.
type CaptchaVerifyResult struct {
	Valid bool `json:"valid"`
}
