package captcha

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/errors"
	base64Captcha "github.com/mojocn/base64Captcha"
)

const (
	defaultCodeLength = 6
	defaultWidth      = 120
	defaultHeight     = 40

	// 本地图形验证码答案的存储 TTL（与历史行为一致）。
	localCaptchaTTL = 5 * time.Minute
)

// Store is the interface for storing and retrieving CAPTCHA codes.
// Local captcha providers (default / go_captcha) share this port;
// MemoryStore and RedisStore implement it.
type Store interface {
	Set(ctx context.Context, id, code string) error
	Get(ctx context.Context, id string) (string, error)
	Delete(ctx context.Context, id string) error
}

// DefaultProvider 是「本地图形验证码」provider（provider_type="default"）。
// 内部实现首选 github.com/mojocn/base64Captcha（digit/string/math 驱动，字体随库
// 内嵌）；对外契约保持不变：Generate 返回 (id, PNG bytes)，Verify 消费
// "captchaId:code" token，答案经共享 Store 存取（Redis 或内存降级）。
type DefaultProvider struct {
	captcha *base64Captcha.Captcha
	store   Store
}

// NewDefaultProvider creates a digit-mode default provider (same shape as the
// historical hand-rolled implementation: 6 characters, 120x40 PNG).
func NewDefaultProvider(store Store) *DefaultProvider {
	return NewDefaultProviderWithConfig(store, "digit", defaultCodeLength)
}

// NewDefaultProviderWithConfig builds the base64Captcha driver per configuration:
// captchaType is one of "digit" (default), "string" (alphanumeric) or "math";
// length controls the answer length for digit/string (math ignores it).
func NewDefaultProviderWithConfig(store Store, captchaType string, length int) *DefaultProvider {
	if length <= 0 {
		length = defaultCodeLength
	}
	var driver base64Captcha.Driver
	switch strings.ToLower(strings.TrimSpace(captchaType)) {
	case "string":
		driver = base64Captcha.NewDriverString(
			defaultHeight, defaultWidth, 100, 1, length,
			"2345678abcdefghjkmnpqrstuvwxyz", nil, nil, nil)
	case "math":
		driver = base64Captcha.NewDriverMath(defaultHeight, defaultWidth, 100, 1, nil, nil, nil)
	default: // "digit"
		driver = base64Captcha.NewDriverDigit(defaultHeight, defaultWidth, length, 0.2, 20)
	}
	return &DefaultProvider{
		captcha: base64Captcha.NewCaptcha(driver, b64Store{store: store}),
		store:   store,
	}
}

// Verify checks a "captchaId:code" token against the shared store. A
// missing/expired captcha id is a failed verification, not an infrastructure
// error: consume nothing and answer false.
func (p *DefaultProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return false, errors.NewInvalidArgument("invalid captcha token format")
	}

	id := parts[0]
	code := parts[1]

	stored, err := p.store.Get(ctx, id)
	if err == nil {
		_ = p.store.Delete(ctx, id)
		return strings.EqualFold(strings.TrimSpace(stored), strings.TrimSpace(code)), nil
	}

	return false, nil
}

// Generate returns the captcha id and the PNG image bytes. The answer is stored
// through the shared Store by base64Captcha itself.
func (p *DefaultProvider) Generate(ctx context.Context) (codeOut string, imageData []byte, err error) {
	id, b64s, _, err := p.captcha.Generate()
	if err != nil {
		return "", nil, errors.WrapInternalf(err, "generate default captcha")
	}

	raw := b64s
	if idx := strings.Index(raw, "base64,"); idx >= 0 {
		raw = raw[idx+len("base64,"):]
	}
	imgBytes, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", nil, errors.WrapInternalf(err, "decode captcha image")
	}

	return id, imgBytes, nil
}
