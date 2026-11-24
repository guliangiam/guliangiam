package auth

import (
	"context"
	"log/slog"

	captchaproviders "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
)

// CaptchaStatusResult indicates whether captcha is required for a given organization.
type CaptchaStatusResult struct {
	Required     bool   `json:"required"`
	ProviderType string `json:"providerType,omitempty"`
	SiteKey      string `json:"siteKey,omitempty"`
}

// GetCaptchaStatus checks if captcha verification is required for login in the given organization.
func (s *Service) GetCaptchaStatus(ctx context.Context, organizationID string) (*CaptchaStatusResult, error) {
	result := &CaptchaStatusResult{Required: false}

	if organizationID == "" {
		return result, nil
	}

	enabled := true
	providers, _, err := s.UOW.CaptchaProviders().List(ctx, captchaproviders.ListQuery{
		OrganizationID: organizationID,
		IsEnabled:      &enabled,
		Page:           1,
		PageSize:       1,
	})
	if err != nil {
		// Fail-open: a lookup failure must not block login, but it is still
		// logged so the misconfiguration is observable.
		slog.WarnContext(ctx, "captcha provider lookup failed; skipping captcha requirement", "error", err)
		return result, nil
	}

	if len(providers) > 0 {
		result.Required = true
		result.ProviderType = providers[0].ProviderType
		result.SiteKey = providers[0].SiteKey
	}

	return result, nil
}
