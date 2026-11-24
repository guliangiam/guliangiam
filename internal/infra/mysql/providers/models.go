package providers

import "time"

type providerRow struct {
	ID             string
	OrganizationID string
	Name           string
	DisplayName    string
	Category       string
	ProviderType   string
	SubType        string
	Method         string
	ClientID       string
	ClientSecret   string
	ClientID2      string
	ClientSecret2  string
	ProviderURL    string
	AuthUrl        string
	TokenUrl       string
	UserInfoUrl    string
	CallbackUrl    string
	SigningAlg     string
	Scopes         []byte
	IdPEntityId    string
	SsoUrl         string
	SloUrl         string
	Metadata       string
	IssuerUrl      string
	Region         string
	Host           string
	Port           int
	DatabaseType   string
	UserMapping    []byte
	Template       string
	CustomApi      string
	CustomHeader   string
	CustomBody     string
	Admin          string
	Password       string
	AppId          string
	AppSecret      string
	AgentID        string
	ContentType    string
	Sender         string
	Receiver       string
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

type providerConfigRow struct {
	ProviderID string
	ConfigType string
	Config     []byte
	UpdatedAt  time.Time
}

type providerSecretStatusRow struct {
	ID            string
	ProviderID    string
	SecretType    string
	Status        string
	LastRotatedAt time.Time
	ExpiresAt     time.Time
	UpdatedAt     time.Time
	Version       int64
}
