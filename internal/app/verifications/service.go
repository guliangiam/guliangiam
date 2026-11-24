package verifications

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	domainverifications "github.com/guliangiam/guliangiam/internal/domain/verifications"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// CaptchaVerifier gates code delivery on a human check. It is injected from the
// composition root (bound to the auth service's EnforceCaptcha) so this package
// does not import the auth app layer. A nil verifier skips the gate for
// embedders/tests that do not configure captcha; the injected implementation is
// fail-open for organizations without an enabled provider and fail-close once
// one is configured.
type CaptchaVerifier func(ctx context.Context, organizationID, captchaToken, captchaID, remoteIP string) error

type Service struct {
	Clock           clock.Clock
	UUID            platformuuid.Generator
	UOW             uow.AuthUoW
	SenderResolver  domainverifications.SenderResolver
	RateLimiter     *RateLimiter
	CaptchaVerifier CaptchaVerifier
	// pepper is injected by tests; production derives it from the existing
	// field-encryption root key with a verification-specific label.
	pepper func() ([]byte, error)
}
