package bootstrap

import (
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/guliangiam/guliangiam/internal/infra/captcha"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
)

// captchaLocalStore resolves the answer store shared by the local captcha
// providers ("default" base64Captcha + "go_captcha"). Fail-open like the
// session cache precedent: CAPTCHA_STORE_DRIVER=memory forces memory;
// otherwise CAPTCHA_STORE_DRIVER=redis or a configured REDIS_ADDR attempts a
// Redis store, and an unreachable Redis degrades to the in-process MemoryStore
// with a warning instead of blocking startup or logins.
func captchaLocalStore() captcha.Store {
	driver := strings.ToLower(strings.TrimSpace(envOrEmpty("CAPTCHA_STORE_DRIVER")))
	addr := platformconfig.RedisAddr()
	if driver == "memory" || addr == "" {
		return captcha.NewMemoryStore(10 * time.Minute)
	}
	if driver != "" && driver != "redis" {
		slog.Warn("captcha store: unknown CAPTCHA_STORE_DRIVER, using memory", "driver", driver)
		return captcha.NewMemoryStore(10 * time.Minute)
	}
	client := redis.NewClient(&redis.Options{
		Addr:        addr,
		Password:    platformconfig.RedisPassword(),
		DB:          0,
		DialTimeout: 2 * time.Second,
	})
	store, err := captcha.NewRedisStore(client, 5*time.Minute)
	if err != nil {
		slog.Warn("captcha store: redis unavailable, falling back to memory", "addr", addr, "error", err.Error())
		_ = client.Close()
		return captcha.NewMemoryStore(10 * time.Minute)
	}
	slog.Info("captcha store: redis", "addr", addr)
	return store
}

// registerCaptchaProviders wires the CAPTCHA families: reCAPTCHA v2/v3/
// Enterprise, hCaptcha, Cloudflare Turnstile, GeeTest, Aliyun, Tencent, NetEase
// Yidun, Baidu Security Verification and the always-available default image
// CAPTCHA. Keys match `domain/captchaproviders.ProviderTypes`; the
// pre-alignment "cloudflare"/"aliyun" keys stay registered as aliases for
// existing rows.
func registerCaptchaProviders() *captcha.Factory {
	captchaFactory := captcha.NewFactory()
	if v := envOrEmpty("RECAPTCHA_SECRET_KEY"); v != "" {
		captchaFactory.Register("recaptcha", captcha.NewReCAPTCHAV2Provider(v))
	}
	if v := envOrEmpty("HCAPTCHA_SECRET_KEY"); v != "" {
		captchaFactory.Register("hcaptcha", captcha.NewHCaptchaProvider(v))
	}
	if v := envOrEmpty("RECAPTCHA_V3_SECRET_KEY"); v != "" {
		captchaFactory.Register("recaptcha_v3", captcha.NewReCAPTCHAV3Provider(v,
			envFloat("RECAPTCHA_V3_SCORE_THRESHOLD"), envOrEmpty("RECAPTCHA_V3_ACTION")))
	}
	if v := envOrEmpty("TURNSTILE_SECRET_KEY"); v != "" {
		turnstile := captcha.NewCloudflareTurnstileProvider(v)
		captchaFactory.Register("turnstile", turnstile)
		// Rows written before the type vocabulary was aligned still say
		// "cloudflare"; keep them resolvable instead of failing their lookups.
		captchaFactory.Register("cloudflare", turnstile)
	}
	if v := envOrEmpty("GEETEST_CAPTCHA_ID"); v != "" {
		captchaFactory.Register("geetest", captcha.NewGeeTestProvider(v, envOrEmpty("GEETEST_CAPTCHA_KEY")))
	}
	if v := envOrEmpty("ALIYUN_CAPTCHA_ACCESS_KEY_ID"); v != "" {
		aliyun := captcha.NewAliyunCaptchaProvider(v, envOrEmpty("ALIYUN_CAPTCHA_ACCESS_KEY_SECRET"), envOrEmpty("ALIYUN_CAPTCHA_APP_KEY"))
		captchaFactory.Register("aliyun_captcha", aliyun)
		// Legacy rows and deployments using the old key keep working.
		captchaFactory.Register("aliyun", aliyun)
	}
	if v := envOrEmpty("TENCENT_CAPTCHA_APP_ID"); v != "" {
		captchaFactory.Register("tencent_captcha", captcha.NewTencentCaptchaProvider(
			envOrEmpty("TENCENT_CAPTCHA_SECRET_ID"),
			envOrEmpty("TENCENT_CAPTCHA_SECRET_KEY"),
			v,
			envOrEmpty("TENCENT_CAPTCHA_APP_SECRET_KEY"),
		))
	}
	if v := envOrEmpty("NETEASE_YIDUN_CAPTCHA_ID"); v != "" {
		captchaFactory.Register("netease_yidun", captcha.NewNetEaseYidunProvider(
			v,
			envOrEmpty("NETEASE_YIDUN_SECRET_ID"),
			envOrEmpty("NETEASE_YIDUN_SECRET_KEY"),
		))
	}
	if v := envOrEmpty("BAIDU_CAPTCHA_APP_ID"); v != "" {
		captchaFactory.Register("baidu_captcha", captcha.NewBaiduCaptchaProvider(
			v,
			envOrEmpty("BAIDU_CAPTCHA_PRIVATE_KEY"),
			envOrEmpty("BAIDU_CAPTCHA_APP_NAME"),
		))
	}
	if v := envOrEmpty("RECAPTCHA_ENTERPRISE_API_KEY"); v != "" {
		captchaFactory.Register("recaptcha_enterprise", captcha.NewReCAPTCHAEnterpriseProvider(
			envOrEmpty("RECAPTCHA_ENTERPRISE_SITE_KEY"),
			v,
			envOrEmpty("RECAPTCHA_ENTERPRISE_PROJECT_ID"),
			envFloat("RECAPTCHA_ENTERPRISE_SCORE_THRESHOLD"),
			envOrEmpty("RECAPTCHA_ENTERPRISE_ACTION"),
		))
	}
	// Local image CAPTCHAs share one answer store (Redis or memory fallback):
	// "default" is the base64Captcha digit/string/math driver configured via
	// CAPTCHA_DEFAULT_TYPE / CAPTCHA_DEFAULT_LENGTH, "go_captcha" covers the
	// click/slide shapes and is skipped (with a log) when its embedded
	// resources are unavailable.
	localStore := captchaLocalStore()
	captchaFactory.Register("default", captcha.NewDefaultProviderWithConfig(
		localStore, envOrEmpty("CAPTCHA_DEFAULT_TYPE"), envInt("CAPTCHA_DEFAULT_LENGTH", 6)))
	if goProv, err := captcha.NewGoCaptchaProvider(localStore); err != nil {
		slog.Warn("go_captcha provider unavailable; type will not be registered", "error", err.Error())
	} else {
		captchaFactory.Register("go_captcha", goProv)
	}

	return captchaFactory
}
