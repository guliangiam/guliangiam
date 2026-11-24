package captcha

import (
	"encoding/json"
	"strconv"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// NewProviderFromRow builds a provider from a captcha_providers row: the two
// credential columns plus the JSONB config for the vendors whose official
// contract needs more than two credentials. This is the row-backed counterpart
// of Factory.NewProvider (which only sees the two slots) and is what the auth
// flow uses so credentials configured in the console are actually consumed.
//
// Recognised config keys (camelCase, per the console field mapping):
//   - aliyun_captcha: sceneId
//   - tencent_captcha: camSecretId, camSecretKey (TC3 signing credentials)
//   - recaptcha_enterprise: projectId, scoreThreshold, action
//   - recaptcha_v3: scoreThreshold, action
//   - netease_yidun: secretId
//   - baidu_captcha: appName
//
// "default" and unknown types return an error: the caller falls back to the
// environment-registered factory, which owns the shared image-captcha store.
func NewProviderFromRow(providerType, siteKey, secretKey string, config map[string]any) (Provider, error) {
	switch providerType {
	case "recaptcha":
		return NewReCAPTCHAV2Provider(secretKey), nil
	case "recaptcha_v3":
		return NewReCAPTCHAV3Provider(secretKey, configFloat(config, "scoreThreshold"), configString(config, "action")), nil
	case "recaptcha_enterprise":
		return NewReCAPTCHAEnterpriseProvider(siteKey, secretKey, configString(config, "projectId"),
			configFloat(config, "scoreThreshold"), configString(config, "action")), nil
	case "hcaptcha":
		return NewHCaptchaProvider(secretKey), nil
	case "turnstile":
		return NewCloudflareTurnstileProvider(secretKey), nil
	case "geetest":
		return NewGeeTestProvider(siteKey, secretKey), nil
	case "aliyun_captcha":
		return NewAliyunCaptchaProvider(siteKey, secretKey, configString(config, "sceneId")), nil
	case "tencent_captcha":
		return NewTencentCaptchaProvider(configString(config, "camSecretId"), configString(config, "camSecretKey"),
			siteKey, secretKey), nil
	case "netease_yidun":
		return NewNetEaseYidunProvider(siteKey, configString(config, "secretId"), secretKey), nil
	case "baidu_captcha":
		return NewBaiduCaptchaProvider(siteKey, secretKey, configString(config, "appName")), nil
	default:
		return nil, apperrors.NewInvalidArgumentf("captcha provider type %q cannot be built from a row", providerType)
	}
}

// configString reads a string config value, tolerating JSON-decoded numbers.
func configString(config map[string]any, key string) string {
	v, ok := config[key]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	default:
		return ""
	}
}

// configFloat reads a numeric config value; anything unparsable yields 0, which
// the constructors map to their documented defaults.
func configFloat(config map[string]any, key string) float64 {
	switch t := config[key].(type) {
	case float64:
		return t
	case json.Number:
		f, err := t.Float64()
		if err != nil {
			return 0
		}
		return f
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
