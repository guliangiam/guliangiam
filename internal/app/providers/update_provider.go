package providers

import "context"

type PatchProviderCommand struct {
	ID string
	// OrganizationID is the trusted tenant scope resolved by transport; it is
	// compared with the persisted row inside the transaction before any write.
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID       string
	Family        string
	Name          *string
	DisplayName   *string
	Kind          *string
	Variant       *string
	Category      *string
	ProviderType  *string
	SubType       *string
	Method        *string
	ClientID      *string
	ClientSecret  *string
	ClientID2     *string
	ClientSecret2 *string
	ProviderURL   *string
	AuthUrl       *string
	TokenUrl      *string
	UserInfoUrl   *string
	CallbackUrl   *string
	SigningAlg    *string
	Scopes        []string
	IdPEntityId   *string
	SsoUrl        *string
	SloUrl        *string
	Metadata      *string
	IssuerUrl     *string
	Region        *string
	Host          *string
	Port          *int
	DatabaseType  *string
	UserMapping   map[string]any
	Template      *string
	CustomApi     *string
	CustomHeader  *string
	CustomBody    *string
	Admin         *string
	Password      *string
	AppId         *string
	AgentID       *string
	ContentType   *string
	Sender        *string
	Receiver      *string

	// Casdoor-compatible options persisted in provider_configs.
	EmailRegex                  *string
	EnablePkce                  *bool
	EnableProxy                 *bool
	HttpHeaders                 map[string]string
	RequireMessageAuthenticator *bool
	EnableSignAuthnRequest      *bool
	CustomLogoutUrl             *string
	CustomLogo                  *string
	Cert                        *string
	State                       *string

	Revision     int64
	AI           *AIProviderPatch
	Payment      *PaymentProviderConfig
	Notification *NotificationProviderPatch
	Secret       *RotateProviderSecretCommand
}

func (s Service) PatchProvider(ctx context.Context, command PatchProviderCommand) error {
	if err := requireOrganizationID(command.OrganizationID); err != nil {
		return err
	}
	_, err := s.unified().Patch(ctx, command)
	return err
}
