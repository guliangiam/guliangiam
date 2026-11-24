package notificationproviders

import appnotificationproviders "github.com/guliangiam/guliangiam/internal/app/notificationproviders"

type ListNotificationProvidersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	IsEnabled      *bool
}

type GetNotificationProviderRequest struct {
	ID string
}

type CreateNotificationProviderRequest struct {
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	ProviderType   string         `json:"providerType"`
	WebhookURL     string         `json:"webhookUrl,omitempty"`
	AccessToken    string         `json:"accessToken,omitempty"`
	Config         map[string]any `json:"config,omitempty"`
	IsEnabled      *bool          `json:"isEnabled,omitempty"`
}

type PatchNotificationProviderRequest struct {
	Name         *string        `json:"name,omitempty"`
	ProviderType *string        `json:"providerType,omitempty"`
	WebhookURL   *string        `json:"webhookUrl,omitempty"`
	AccessToken  *string        `json:"accessToken,omitempty"`
	Config       map[string]any `json:"config,omitempty"`
	IsEnabled    *bool          `json:"isEnabled,omitempty"`
	Revision     int64          `json:"revision,omitempty"`
}

func (r CreateNotificationProviderRequest) toCommand() appnotificationproviders.CreateNotificationProviderCommand {
	return appnotificationproviders.CreateNotificationProviderCommand{
		OrganizationID: r.OrganizationID,
		Name:           r.Name,
		ProviderType:   r.ProviderType,
		WebhookURL:     r.WebhookURL,
		AccessToken:    r.AccessToken,
		Config:         r.Config,
		IsEnabled:      r.IsEnabled,
	}
}

func (r PatchNotificationProviderRequest) toCommand(id string) appnotificationproviders.PatchNotificationProviderCommand {
	return appnotificationproviders.PatchNotificationProviderCommand{
		ID:           id,
		Name:         r.Name,
		ProviderType: r.ProviderType,
		WebhookURL:   r.WebhookURL,
		AccessToken:  r.AccessToken,
		Config:       r.Config,
		IsEnabled:    r.IsEnabled,
		Revision:     r.Revision,
	}
}
