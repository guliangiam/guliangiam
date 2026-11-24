package captchaproviders

import appcaptchaproviders "github.com/guliangiam/guliangiam/internal/app/captchaproviders"

type ListCaptchaProvidersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type GetCaptchaProviderRequest struct {
	ID string
}

type CreateCaptchaProviderRequest struct {
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	ProviderType   string         `json:"providerType"`
	SiteKey        string         `json:"siteKey,omitempty"`
	SecretKey      string         `json:"secretKey,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	IsEnabled      *bool          `json:"isEnabled,omitempty"`
}

type PatchCaptchaProviderRequest struct {
	Name         *string        `json:"name,omitempty"`
	ProviderType *string        `json:"providerType,omitempty"`
	SiteKey      *string        `json:"siteKey,omitempty"`
	SecretKey    *string        `json:"secretKey,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	IsEnabled    *bool          `json:"isEnabled,omitempty"`
	Revision     int64          `json:"revision,omitempty"`
}

func (r CreateCaptchaProviderRequest) toCommand() appcaptchaproviders.CreateCaptchaProviderCommand {
	return appcaptchaproviders.CreateCaptchaProviderCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		ProviderType:   r.ProviderType,
		SiteKey:        r.SiteKey,
		SecretKey:      r.SecretKey,
		Config:         r.Config,
		IsEnabled:      r.IsEnabled,
	}
}

func (r PatchCaptchaProviderRequest) toCommand(id string) appcaptchaproviders.PatchCaptchaProviderCommand {
	return appcaptchaproviders.PatchCaptchaProviderCommand{
		ID:           id,
		Name:         r.Name,
		ProviderType: r.ProviderType,
		SiteKey:      r.SiteKey,
		SecretKey:    r.SecretKey,
		Config:       r.Config,
		IsEnabled:    r.IsEnabled,
		Revision:     r.Revision,
	}
}
