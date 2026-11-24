package auth

import (
	"context"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// EnforceCaptcha verifies a human-check response before a sensitive auth action
// (signup, verification-code delivery). The provider is resolved server-side
// from the organization's enabled captcha_providers row, never from a
// client-supplied type, so a caller cannot downgrade to a weaker provider.
//
// Fail-open mirrors GetCaptchaStatus: an organization with no enabled provider
// has not opted into captcha, so the action proceeds. Once a provider is
// enabled the gate is fail-close — a missing or invalid response is rejected.
func (s *Service) EnforceCaptcha(ctx context.Context, organizationID, captchaToken, captchaID, remoteIP string) error {
	// No captcha factory means the subsystem is not wired (embedders/tests):
	// fail-open before touching the provider repository, mirroring the
	// lookup-failure fail-open in GetCaptchaStatus.
	if s.CaptchaFactory == nil {
		return nil
	}

	providerType, row := s.resolveCaptchaProvider(ctx, organizationID)
	if row == nil {
		return nil
	}

	token := strings.TrimSpace(captchaToken)
	if token == "" {
		return apperrors.NewInvalidArgument("captcha is required")
	}

	// Server-generated challenges bind the response to the issued challenge id
	// ("id:payload"); third-party widgets forward their opaque response token.
	if providerType == "default" || providerType == "go_captcha" {
		id := strings.TrimSpace(captchaID)
		if id == "" {
			return apperrors.NewInvalidArgument("captcha is required")
		}
		token = id + ":" + token
	}

	var (
		valid bool
		err   error
	)
	if providerType != "default" {
		valid, err = s.CaptchaFactory.VerifyCaptchaWithRow(ctx, providerType, row.SiteKey, row.SecretKey, token, remoteIP, row.Config)
	} else {
		valid, err = s.CaptchaFactory.VerifyCaptcha(ctx, providerType, token, remoteIP)
	}
	if err != nil {
		return apperrors.WrapInternal(err, "verify captcha")
	}
	if !valid {
		return apperrors.NewInvalidArgument("captcha verification failed")
	}
	return nil
}
