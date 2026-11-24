package auth

import (
	"context"

	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// AppLoginResult contains all information needed to render the login page for an application.
type AppLoginResult struct {
	Organization  AppLoginOrganization `json:"organization"`
	Application   *AppLoginApplication `json:"application,omitempty"`
	CaptchaStatus *CaptchaStatusResult `json:"captchaStatus,omitempty"`
	Providers     []AppLoginProvider   `json:"providers"`
}

// AppLoginOrganization contains organization details for the login page.
type AppLoginOrganization struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	LogoURL     string `json:"logoUrl"`
	FaviconURL  string `json:"faviconUrl"`
	WebsiteURL  string `json:"websiteUrl"`
}

// AppLoginApplication contains application details for the login page.
type AppLoginApplication struct {
	ID               string   `json:"id"`
	Slug             string   `json:"slug"`
	DisplayName      string   `json:"displayName"`
	Logo             string   `json:"logo"`
	SigninMethods    []string `json:"signinMethods"`
	EnableSignup     bool     `json:"enableSignup"`
	EnablePassword   bool     `json:"enablePassword"`
	EnableCodeSignin bool     `json:"enableCodeSignin"`
}

// AppLoginProvider contains provider details for the login page.
type AppLoginProvider struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Category    string `json:"category"`
	DisplayName string `json:"displayName"`
	Name        string `json:"name"`
}

// GetAppLogin returns the login page configuration for an application.
// It combines organization info, application details, captcha status, and available providers
// into a single response, reducing the number of API round-trips from the frontend.
func (s *Service) GetAppLogin(ctx context.Context, orgSlug, appSlug string) (*AppLoginResult, error) {
	org, err := s.UOW.Organizations().GetBySlug(ctx, orgSlug)
	if err != nil {
		return nil, apperrors.WrapInternal(err, "get organization")
	}

	result := &AppLoginResult{
		Organization: AppLoginOrganization{
			ID:          org.ID,
			Slug:        org.Slug,
			DisplayName: org.DisplayName,
			LogoURL:     org.LogoURL,
			FaviconURL:  org.FaviconURL,
			WebsiteURL:  org.WebsiteURL,
		},
	}

	// Check captcha status for the organization
	captchaStatus, err := s.GetCaptchaStatus(ctx, org.ID)
	if err == nil && captchaStatus.Required {
		result.CaptchaStatus = captchaStatus
	}

	// If appSlug is provided, get application details
	if appSlug != "" {
		app, err := s.UOW.Applications().GetByOrganizationAndSlug(ctx, org.ID, appSlug)
		if err == nil {
			result.Application = &AppLoginApplication{
				ID:               app.ID,
				Slug:             app.Slug,
				DisplayName:      app.DisplayName,
				Logo:             app.Logo,
				SigninMethods:    app.SigninMethods,
				EnableSignup:     app.EnableSignup,
				EnablePassword:   app.PasswordEnabled(),
				EnableCodeSignin: app.CodeSigninEnabled(),
			}
		}
	}

	// List enabled social/OAuth providers for this organization
	providers, _, err := s.UOW.Providers().List(ctx, providerdomain.ListQuery{
		OrganizationID: org.ID,
		PageSize:       100,
	})
	if err == nil {
		for _, p := range providers {
			result.Providers = append(result.Providers, AppLoginProvider{
				ID:          p.ID,
				Type:        string(p.ProviderType),
				Category:    string(p.Category),
				DisplayName: p.DisplayName,
				Name:        p.Name,
			})
		}
	}

	return result, nil
}
