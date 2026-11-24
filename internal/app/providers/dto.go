package providers

import "time"

type ProviderListItem struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organizationId"`
	Name           string    `json:"name"`
	DisplayName    string    `json:"displayName"`
	Family         string    `json:"family"`
	Kind           string    `json:"kind"`
	Variant        string    `json:"variant,omitempty"`
	Status         string    `json:"status"`
	Enabled        bool      `json:"enabled"`
	Category       string    `json:"category"`
	ProviderType   string    `json:"providerType"`
	SubType        string    `json:"subType"`
	ClientID       string    `json:"clientId"`
	ProviderURL    string    `json:"providerUrl"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	Revision       int64     `json:"revision"`
}

type ProviderDetail struct {
	ProviderListItem
	ClientSecretSet             bool              `json:"clientSecretSet"`
	ClientSecretMasked          string            `json:"clientSecretMasked,omitempty"`
	ClientID2                   string            `json:"clientId2"`
	ClientSecret2Set            bool              `json:"clientSecret2Set"`
	ClientSecret2Masked         string            `json:"clientSecret2Masked,omitempty"`
	Method                      string            `json:"method"`
	AuthUrl                     string            `json:"authUrl"`
	TokenUrl                    string            `json:"tokenUrl"`
	UserInfoUrl                 string            `json:"userInfoUrl"`
	CallbackUrl                 string            `json:"callbackUrl"`
	SigningAlg                  string            `json:"signingAlg"`
	Scopes                      []string          `json:"scopes"`
	IdPEntityId                 string            `json:"idpEntityId"`
	SsoUrl                      string            `json:"ssoUrl"`
	SloUrl                      string            `json:"sloUrl"`
	Metadata                    string            `json:"metadata"`
	IssuerUrl                   string            `json:"issuerUrl"`
	Region                      string            `json:"region"`
	Host                        string            `json:"host"`
	Port                        int               `json:"port"`
	DatabaseType                string            `json:"databaseType"`
	UserMapping                 map[string]any    `json:"userMapping"`
	Template                    string            `json:"template"`
	CustomApi                   string            `json:"customApi"`
	CustomHeader                string            `json:"customHeader"`
	CustomBody                  string            `json:"customBody"`
	Admin                       string            `json:"admin"`
	PasswordSet                 bool              `json:"passwordSet"`
	PasswordMasked              string            `json:"passwordMasked,omitempty"`
	AppId                       string            `json:"appId"`
	AgentID                     string            `json:"agentId,omitempty"`
	AppSecretSet                bool              `json:"appSecretSet"`
	AppSecretMasked             string            `json:"appSecretMasked,omitempty"`
	ContentType                 string            `json:"contentType"`
	Sender                      string            `json:"sender"`
	Receiver                    string            `json:"receiver"`
	EmailRegex                  string            `json:"emailRegex,omitempty"`
	EnablePkce                  bool              `json:"enablePkce"`
	EnableProxy                 bool              `json:"enableProxy"`
	HttpHeaders                 map[string]string `json:"httpHeaders,omitempty"`
	RequireMessageAuthenticator bool              `json:"requireMessageAuthenticator"`
	EnableSignAuthnRequest      bool              `json:"enableSignAuthnRequest"`
	CustomLogoutUrl             string            `json:"customLogoutUrl,omitempty"`
	CustomLogo                  string            `json:"customLogo,omitempty"`
	Cert                        string            `json:"cert,omitempty"`
	State                       string            `json:"state,omitempty"`
	Revision                    int64             `json:"revision"`
	Detail                      map[string]any    `json:"detail,omitempty"`
}

type ProviderConfigDetail struct {
	ProviderID string         `json:"providerId"`
	ConfigType string         `json:"configType"`
	Config     map[string]any `json:"config"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

type ProviderSecretStatusItem struct {
	ID            string    `json:"id"`
	ProviderID    string    `json:"providerId"`
	SecretType    string    `json:"secretType"`
	Status        string    `json:"status"`
	LastRotatedAt time.Time `json:"lastRotatedAt"`
	ExpiresAt     time.Time `json:"expiresAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	Version       int64     `json:"version"`
}
