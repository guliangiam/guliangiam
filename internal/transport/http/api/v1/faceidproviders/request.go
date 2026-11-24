package faceidproviders

import appfaceidproviders "github.com/guliangiam/guliangiam/internal/app/faceidproviders"

type ListFaceIdProvidersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	IsEnabled      *bool
}

type CreateFaceIdProviderRequest struct {
	OrganizationID  string         `json:"organizationId"`
	Name            string         `json:"name"`
	ProviderType    string         `json:"providerType"`
	AccessKeyID     string         `json:"accessKeyId,omitempty"`
	AccessKeySecret string         `json:"accessKeySecret,omitempty"`
	Endpoint        string         `json:"endpoint,omitempty"`
	Config          map[string]any `json:"config,omitempty"`
	IsEnabled       *bool          `json:"isEnabled,omitempty"`
}

type PatchFaceIdProviderRequest struct {
	Name            *string        `json:"name,omitempty"`
	ProviderType    *string        `json:"providerType,omitempty"`
	AccessKeyID     *string        `json:"accessKeyId,omitempty"`
	AccessKeySecret *string        `json:"accessKeySecret,omitempty"`
	Endpoint        *string        `json:"endpoint,omitempty"`
	Config          map[string]any `json:"config,omitempty"`
	IsEnabled       *bool          `json:"isEnabled,omitempty"`
	Revision        int64          `json:"revision,omitempty"`
}

func (r CreateFaceIdProviderRequest) toCommand() appfaceidproviders.CreateFaceIdProviderCommand {
	return appfaceidproviders.CreateFaceIdProviderCommand{
		OrganizationID:  r.OrganizationID,
		Name:            r.Name,
		ProviderType:    r.ProviderType,
		AccessKeyID:     r.AccessKeyID,
		AccessKeySecret: r.AccessKeySecret,
		Endpoint:        r.Endpoint,
		Config:          r.Config,
		IsEnabled:       r.IsEnabled,
	}
}

func (r PatchFaceIdProviderRequest) toCommand(id string) appfaceidproviders.PatchFaceIdProviderCommand {
	command := appfaceidproviders.PatchFaceIdProviderCommand{
		ID:              id,
		Name:            r.Name,
		ProviderType:    r.ProviderType,
		AccessKeyID:     r.AccessKeyID,
		AccessKeySecret: r.AccessKeySecret,
		Endpoint:        r.Endpoint,
		IsEnabled:       r.IsEnabled,
		Revision:        r.Revision,
	}
	if r.Config != nil {
		command.Config = r.Config
	}
	return command
}
