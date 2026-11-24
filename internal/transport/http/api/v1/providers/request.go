package providers

import appproviders "github.com/guliangiam/guliangiam/internal/app/providers"

type ListProvidersRequest struct {
	Page           int
	PageSize       int
	OrganizationID string
	Keyword        string
	Category       string
	ProviderType   string
}

// CreateProviderRequest mirrors the identity-provider columns exposed by the
// management form. DecodeJSON 使用 DisallowUnknownFields，因此表单里的每个字段
// 都必须在这里有对应项，否则创建/编辑请求会直接 400。
type CreateProviderRequest struct {
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName"`
	Category       string         `json:"category"`
	ProviderType   string         `json:"providerType"`
	SubType        string         `json:"subType,omitempty"`
	Method         string         `json:"method,omitempty"`
	ClientID       string         `json:"clientId,omitempty"`
	ClientSecret   string         `json:"clientSecret,omitempty"`
	ClientID2      string         `json:"clientId2,omitempty"`
	ClientSecret2  string         `json:"clientSecret2,omitempty"`
	ProviderURL    string         `json:"providerUrl,omitempty"`
	AuthUrl        string         `json:"authUrl,omitempty"`
	TokenUrl       string         `json:"tokenUrl,omitempty"`
	UserInfoUrl    string         `json:"userInfoUrl,omitempty"`
	CallbackUrl    string         `json:"callbackUrl,omitempty"`
	SigningAlg     string         `json:"signingAlg,omitempty"`
	Scopes         []string       `json:"scopes,omitempty"`
	IdPEntityId    string         `json:"idpEntityId,omitempty"`
	SsoUrl         string         `json:"ssoUrl,omitempty"`
	SloUrl         string         `json:"sloUrl,omitempty"`
	Metadata       string         `json:"metadata,omitempty"`
	IssuerUrl      string         `json:"issuerUrl,omitempty"`
	Region         string         `json:"region,omitempty"`
	Host           string         `json:"host,omitempty"`
	Port           int            `json:"port,omitempty"`
	DatabaseType   string         `json:"databaseType,omitempty"`
	UserMapping    map[string]any `json:"userMapping,omitempty"`
	Template       string         `json:"template,omitempty"`
	CustomApi      string         `json:"customApi,omitempty"`
	CustomHeader   string         `json:"customHeader,omitempty"`
	CustomBody     string         `json:"customBody,omitempty"`
	Admin          string         `json:"admin,omitempty"`
	Password       string         `json:"password,omitempty"`
	AppId          string         `json:"appId,omitempty"`
	AgentID        string         `json:"agentId,omitempty"`
	ContentType    string         `json:"contentType,omitempty"`
	Sender         string         `json:"sender,omitempty"`
	Receiver       string         `json:"receiver,omitempty"`

	EmailRegex                  string            `json:"emailRegex,omitempty"`
	EnablePkce                  bool              `json:"enablePkce,omitempty"`
	EnableProxy                 bool              `json:"enableProxy,omitempty"`
	HttpHeaders                 map[string]string `json:"httpHeaders,omitempty"`
	RequireMessageAuthenticator bool              `json:"requireMessageAuthenticator,omitempty"`
	EnableSignAuthnRequest      bool              `json:"enableSignAuthnRequest,omitempty"`
	CustomLogoutUrl             string            `json:"customLogoutUrl,omitempty"`
	CustomLogo                  string            `json:"customLogo,omitempty"`
	Cert                        string            `json:"cert,omitempty"`
	State                       string            `json:"state,omitempty"`
}

// PatchProviderRequest uses pointers so "not provided" stays distinct from
// "provided as empty", matching PATCH semantics.
type PatchProviderRequest struct {
	DisplayName   *string        `json:"displayName,omitempty"`
	Category      *string        `json:"category,omitempty"`
	ProviderType  *string        `json:"providerType,omitempty"`
	SubType       *string        `json:"subType,omitempty"`
	Method        *string        `json:"method,omitempty"`
	ClientID      *string        `json:"clientId,omitempty"`
	ClientSecret  *string        `json:"clientSecret,omitempty"`
	ClientID2     *string        `json:"clientId2,omitempty"`
	ClientSecret2 *string        `json:"clientSecret2,omitempty"`
	ProviderURL   *string        `json:"providerUrl,omitempty"`
	AuthUrl       *string        `json:"authUrl,omitempty"`
	TokenUrl      *string        `json:"tokenUrl,omitempty"`
	UserInfoUrl   *string        `json:"userInfoUrl,omitempty"`
	CallbackUrl   *string        `json:"callbackUrl,omitempty"`
	SigningAlg    *string        `json:"signingAlg,omitempty"`
	Scopes        []string       `json:"scopes,omitempty"`
	IdPEntityId   *string        `json:"idpEntityId,omitempty"`
	SsoUrl        *string        `json:"ssoUrl,omitempty"`
	SloUrl        *string        `json:"sloUrl,omitempty"`
	Metadata      *string        `json:"metadata,omitempty"`
	IssuerUrl     *string        `json:"issuerUrl,omitempty"`
	Region        *string        `json:"region,omitempty"`
	Host          *string        `json:"host,omitempty"`
	Port          *int           `json:"port,omitempty"`
	DatabaseType  *string        `json:"databaseType,omitempty"`
	UserMapping   map[string]any `json:"userMapping,omitempty"`
	Template      *string        `json:"template,omitempty"`
	CustomApi     *string        `json:"customApi,omitempty"`
	CustomHeader  *string        `json:"customHeader,omitempty"`
	CustomBody    *string        `json:"customBody,omitempty"`
	Admin         *string        `json:"admin,omitempty"`
	Password      *string        `json:"password,omitempty"`
	AppId         *string        `json:"appId,omitempty"`
	AgentID       *string        `json:"agentId,omitempty"`
	ContentType   *string        `json:"contentType,omitempty"`
	Sender        *string        `json:"sender,omitempty"`
	Receiver      *string        `json:"receiver,omitempty"`

	EmailRegex                  *string           `json:"emailRegex,omitempty"`
	EnablePkce                  *bool             `json:"enablePkce,omitempty"`
	EnableProxy                 *bool             `json:"enableProxy,omitempty"`
	HttpHeaders                 map[string]string `json:"httpHeaders,omitempty"`
	RequireMessageAuthenticator *bool             `json:"requireMessageAuthenticator,omitempty"`
	EnableSignAuthnRequest      *bool             `json:"enableSignAuthnRequest,omitempty"`
	CustomLogoutUrl             *string           `json:"customLogoutUrl,omitempty"`
	CustomLogo                  *string           `json:"customLogo,omitempty"`
	Cert                        *string           `json:"cert,omitempty"`
	State                       *string           `json:"state,omitempty"`

	Revision int64 `json:"revision,omitempty"`
}

type UpdateProviderConfigRequest struct {
	ConfigType string         `json:"configType"`
	Config     map[string]any `json:"config,omitempty"`
}

func (r CreateProviderRequest) toCommand() appproviders.CreateProviderCommand {
	return appproviders.CreateProviderCommand{
		OrganizationID:              r.OrganizationID,
		Name:                        r.Name,
		DisplayName:                 r.DisplayName,
		Category:                    r.Category,
		ProviderType:                r.ProviderType,
		SubType:                     r.SubType,
		Method:                      r.Method,
		ClientID:                    r.ClientID,
		ClientSecret:                r.ClientSecret,
		ClientID2:                   r.ClientID2,
		ClientSecret2:               r.ClientSecret2,
		ProviderURL:                 r.ProviderURL,
		AuthUrl:                     r.AuthUrl,
		TokenUrl:                    r.TokenUrl,
		UserInfoUrl:                 r.UserInfoUrl,
		CallbackUrl:                 r.CallbackUrl,
		SigningAlg:                  r.SigningAlg,
		Scopes:                      r.Scopes,
		IdPEntityId:                 r.IdPEntityId,
		SsoUrl:                      r.SsoUrl,
		SloUrl:                      r.SloUrl,
		Metadata:                    r.Metadata,
		IssuerUrl:                   r.IssuerUrl,
		Region:                      r.Region,
		Host:                        r.Host,
		Port:                        r.Port,
		DatabaseType:                r.DatabaseType,
		UserMapping:                 r.UserMapping,
		Template:                    r.Template,
		CustomApi:                   r.CustomApi,
		CustomHeader:                r.CustomHeader,
		CustomBody:                  r.CustomBody,
		Admin:                       r.Admin,
		Password:                    r.Password,
		AppId:                       r.AppId,
		AgentID:                     r.AgentID,
		ContentType:                 r.ContentType,
		Sender:                      r.Sender,
		Receiver:                    r.Receiver,
		EmailRegex:                  r.EmailRegex,
		EnablePkce:                  r.EnablePkce,
		EnableProxy:                 r.EnableProxy,
		HttpHeaders:                 r.HttpHeaders,
		RequireMessageAuthenticator: r.RequireMessageAuthenticator,
		EnableSignAuthnRequest:      r.EnableSignAuthnRequest,
		CustomLogoutUrl:             r.CustomLogoutUrl,
		CustomLogo:                  r.CustomLogo,
		Cert:                        r.Cert,
		State:                       r.State,
	}
}

func (r PatchProviderRequest) toCommand(id string) appproviders.PatchProviderCommand {
	return appproviders.PatchProviderCommand{
		ID:                          id,
		DisplayName:                 r.DisplayName,
		Category:                    r.Category,
		ProviderType:                r.ProviderType,
		SubType:                     r.SubType,
		Method:                      r.Method,
		ClientID:                    r.ClientID,
		ClientSecret:                r.ClientSecret,
		ClientID2:                   r.ClientID2,
		ClientSecret2:               r.ClientSecret2,
		ProviderURL:                 r.ProviderURL,
		AuthUrl:                     r.AuthUrl,
		TokenUrl:                    r.TokenUrl,
		UserInfoUrl:                 r.UserInfoUrl,
		CallbackUrl:                 r.CallbackUrl,
		SigningAlg:                  r.SigningAlg,
		Scopes:                      r.Scopes,
		IdPEntityId:                 r.IdPEntityId,
		SsoUrl:                      r.SsoUrl,
		SloUrl:                      r.SloUrl,
		Metadata:                    r.Metadata,
		IssuerUrl:                   r.IssuerUrl,
		Region:                      r.Region,
		Host:                        r.Host,
		Port:                        r.Port,
		DatabaseType:                r.DatabaseType,
		UserMapping:                 r.UserMapping,
		Template:                    r.Template,
		CustomApi:                   r.CustomApi,
		CustomHeader:                r.CustomHeader,
		CustomBody:                  r.CustomBody,
		Admin:                       r.Admin,
		Password:                    r.Password,
		AppId:                       r.AppId,
		AgentID:                     r.AgentID,
		ContentType:                 r.ContentType,
		Sender:                      r.Sender,
		Receiver:                    r.Receiver,
		EmailRegex:                  r.EmailRegex,
		EnablePkce:                  r.EnablePkce,
		EnableProxy:                 r.EnableProxy,
		HttpHeaders:                 r.HttpHeaders,
		RequireMessageAuthenticator: r.RequireMessageAuthenticator,
		EnableSignAuthnRequest:      r.EnableSignAuthnRequest,
		CustomLogoutUrl:             r.CustomLogoutUrl,
		CustomLogo:                  r.CustomLogo,
		Cert:                        r.Cert,
		State:                       r.State,
		Revision:                    r.Revision,
	}
}

func (r UpdateProviderConfigRequest) toCommand(providerID string) appproviders.UpdateProviderConfigCommand {
	return appproviders.UpdateProviderConfigCommand{
		ProviderID: providerID,
		ConfigType: r.ConfigType,
		Config:     r.Config,
	}
}
