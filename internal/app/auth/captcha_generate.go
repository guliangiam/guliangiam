package auth

import (
	"context"
	"strings"

	captchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// GenerateCaptcha generates a new CAPTCHA challenge for the given organization.
// Server-side generated types ("default", "go_captcha") consume the enabled row's
// JSONB config (go_captcha reads config.shape); third-party types are rendered
// client-side and rejected by the factory.
func (s *Service) GenerateCaptcha(ctx context.Context, organizationID string) (*CaptchaGenerateResult, error) {
	providerType, row := s.resolveCaptchaProvider(ctx, organizationID)

	if s.CaptchaFactory == nil {
		return nil, apperrors.NewInternal("captcha factory not configured")
	}

	shape := ""
	if row != nil {
		if v, ok := row.Config["shape"].(string); ok {
			shape = strings.TrimSpace(v)
		}
	}

	id, imageBase64, subImageBase64, resolvedShape, err := s.CaptchaFactory.GenerateCaptcha(ctx, providerType, shape)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "generate captcha")
	}

	result := &CaptchaGenerateResult{
		CaptchaID:    id,
		CaptchaImage: imageBase64,
		Type:         providerType,
	}
	if resolvedShape != "" {
		result.Shape = resolvedShape
		result.CaptchaImageSub = subImageBase64
	}
	return result, nil
}

// VerifyCaptcha verifies a CAPTCHA response.
func (s *Service) VerifyCaptcha(ctx context.Context, organizationID, token, remoteIP string) (*CaptchaVerifyResult, error) {
	providerType, row := s.resolveCaptchaProvider(ctx, organizationID)

	if s.CaptchaFactory == nil {
		return nil, apperrors.NewInternal("captcha factory not configured")
	}

	var (
		valid bool
		err   error
	)
	if row != nil && providerType != "default" {
		// The console row owns the credentials (including the JSONB config for
		// vendors whose official contract needs more than two fields).
		valid, err = s.CaptchaFactory.VerifyCaptchaWithRow(ctx, providerType, row.SiteKey, row.SecretKey, token, remoteIP, row.Config)
	} else {
		valid, err = s.CaptchaFactory.VerifyCaptcha(ctx, providerType, token, remoteIP)
	}
	if err != nil {
		return nil, apperrors.WrapInternal(err, "verify captcha")
	}

	return &CaptchaVerifyResult{Valid: valid}, nil
}

// resolveCaptchaProvider determines the captcha provider type and the enabled
// provider row for the given organization. Returns "default"/nil when no
// provider is configured or the organization ID is empty.
func (s *Service) resolveCaptchaProvider(ctx context.Context, organizationID string) (string, *captchaproviders.CaptchaProvider) {
	if organizationID == "" {
		return "default", nil
	}

	repository := s.UOW.CaptchaProviders()
	if repository == nil {
		return "default", nil
	}

	enabled := true
	providers, _, err := repository.List(ctx, captchaproviders.ListQuery{
		OrganizationID: organizationID,
		IsEnabled:      &enabled,
		Page:           1,
		PageSize:       1,
	})
	if err != nil || len(providers) == 0 {
		return "default", nil
	}

	return providers[0].ProviderType, providers[0]
}
