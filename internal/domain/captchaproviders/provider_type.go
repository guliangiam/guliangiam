package captchaproviders

import enumset "github.com/guliangiam/guliangiam/internal/platform/enumset"

// ProviderTypes is the closed set of CAPTCHA provider types a provider record
// may name.
//
// The values are the keys the captcha factory is registered under
// (`internal/bootstrap/runtime.go`), and each one has a constructor in
// `internal/infra/captcha`. They are lowercase because that is what the factory
// looks up: the previous console sent `Default`/`reCAPTCHA`/`Turnstile`, none of
// which matched a registered key, so every provider created in the console was
// unusable and the login captcha endpoint answered 500 through a wrapped
// internal error.
//
// Membership means "this deployment can resolve the type once its secret is
// configured", not "the type is ready right now": every type except "default"
// registers only when its secret is present, and pre-configuring a provider
// before the secret is rolled out is a legitimate workflow. A type that no code
// path ever registers must not be listed — accepting it would repeat the
// original defect in the opposite direction.
//
// recaptcha_enterprise, netease_yidun and baidu_captcha exist only in the
// dedicated captcha subsystem: they are never unified provider kinds, so they
// are deliberately absent from contracts/registries/provider-kinds.yaml and the
// providers kind closure.
//
// The values use the same vocabulary as `domain/providers` kinds
// (`turnstile`, `aliyun_captcha`, `recaptcha_v3`) so a provider row and a
// legacy captcha row name one type with one spelling. The bootstrap factory
// additionally registers the old "cloudflare"/"aliyun" keys so rows written
// before this alignment keep resolving; those spellings are not accepted for
// new writes.
var ProviderTypes = enumset.NewSet(
	"captchaProviderType",
	"default",
	"recaptcha",
	"recaptcha_v3",
	"recaptcha_enterprise",
	"hcaptcha",
	"turnstile",
	"geetest",
	"aliyun_captcha",
	"tencent_captcha",
	"netease_yidun",
	"baidu_captcha",
	"go_captcha",
)

// ProviderTypeDefault is the fallback used when an organization has no enabled
// provider record.
const ProviderTypeDefault = "default"
