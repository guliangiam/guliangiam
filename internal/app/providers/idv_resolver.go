package providers

import (
	"context"
	"fmt"
	"slices"
	"strings"

	domainidv "github.com/guliangiam/guliangiam/internal/domain/idv"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	idvKindJumio      = "jumio"
	idvKindAliyunIDV  = "aliyun_idv"
	idvKindTencentIDV = "tencent_idv"
	idvKindOnfido     = "onfido"
	idvKindVeriff     = "veriff"
	idvKindPersona    = "persona"
)

// idvKinds lists the routable identity-verification kinds.
var idvKinds = []string{idvKindJumio, idvKindAliyunIDV, idvKindTencentIDV, idvKindOnfido, idvKindVeriff, idvKindPersona}

// idvKindsRequiringAPISecret lists the kinds whose provider needs both halves
// of the credential pair; Onfido and Persona authenticate with a single token.
var idvKindsRequiringAPISecret = map[string]bool{
	idvKindJumio:      true,
	idvKindAliyunIDV:  true,
	idvKindTencentIDV: true,
	idvKindVeriff:     true,
}

// ResolveIDVProvider resolves the enabled IDVerification provider for the
// tenant and maps its encrypted credential fields onto the IDV domain port.
// Non-secret per-provider options (sceneId, endpoint, sandbox) are read from
// provider_configs; credentials never leave the app/infra boundary.
func (s Service) ResolveIDVProvider(ctx context.Context, organizationID, providerName string) (*domainidv.ProviderCredentials, error) {
	if err := requireOrganizationID(organizationID); err != nil {
		return nil, err
	}

	items, _, err := s.UOW.Providers().List(ctx, domain.ListQuery{
		OrganizationID: organizationID,
		Category:       string(domain.CategoryIDVerification),
		Page:           1,
		PageSize:       100,
	})
	if err != nil {
		return nil, err
	}

	var selected *domain.Provider
	requested := strings.TrimSpace(providerName)
	for _, item := range items {
		if item == nil || !item.Enabled {
			continue
		}
		kind := strings.ToLower(strings.TrimSpace(string(item.Kind)))
		if !slices.Contains(idvKinds, kind) {
			continue
		}
		if requested != "" &&
			!strings.EqualFold(item.Name, requested) &&
			!strings.EqualFold(kind, requested) &&
			!strings.EqualFold(string(item.ProviderType), requested) {
			continue
		}
		selected = item
		if requested != "" {
			break
		}
	}
	if selected == nil {
		return nil, apperrors.NewInvalidArgument("no enabled identity-verification provider is configured for this organization")
	}

	config := map[string]any{}
	if stored, err := s.UOW.Providers().GetConfig(ctx, selected.ID); err == nil && stored != nil {
		config = stored.Config
	}

	return credentialsFromProvider(selected, config)
}

// CredentialsFromProvider maps a decrypted provider row onto the IDV port.
func CredentialsFromProvider(provider *domain.Provider) (*domainidv.ProviderCredentials, error) {
	return credentialsFromProvider(provider, nil)
}

// CredentialsFromProviderConfig includes non-secret options used by the
// read-only connection probe, keeping it consistent with the runtime resolver.
func CredentialsFromProviderConfig(provider *domain.Provider, config map[string]any) (*domainidv.ProviderCredentials, error) {
	return credentialsFromProvider(provider, config)
}

func credentialsFromProvider(provider *domain.Provider, config map[string]any) (*domainidv.ProviderCredentials, error) {
	if provider == nil {
		return nil, apperrors.NewInvalidArgument("identity-verification provider is missing")
	}
	kind := strings.ToLower(strings.TrimSpace(string(provider.Kind)))
	if kind == "" {
		kind = strings.ToLower(strings.TrimSpace(string(provider.ProviderType)))
	}
	endpoint := strings.TrimSpace(provider.ProviderURL)
	if override := configString(config, "idvEndpoint"); override != "" {
		endpoint = override
	}

	credentials := &domainidv.ProviderCredentials{
		Provider:  kind,
		Endpoint:  endpoint,
		Region:    strings.TrimSpace(provider.Region),
		ReturnURL: configString(config, "idvReturnUrl"),
	}
	switch kind {
	case idvKindJumio:
		credentials.APIKey = provider.ClientID
		credentials.APISecret = provider.ClientSecret
		credentials.WebhookSecret = provider.ClientSecret2
		credentials.Sandbox = configBool(config, "idvSandbox") ||
			strings.EqualFold(provider.Region, "sandbox") ||
			strings.Contains(strings.ToLower(provider.ProviderURL), "sandbox")
	case idvKindAliyunIDV:
		credentials.APIKey = provider.Admin
		credentials.APISecret = provider.Password
		credentials.WebhookSecret = provider.ClientSecret2
		credentials.SceneID = configString(config, "idvSceneId")
	case idvKindTencentIDV:
		credentials.APIKey = provider.Admin
		credentials.APISecret = provider.Password
		credentials.WebhookSecret = provider.ClientSecret2
		credentials.SceneID = configString(config, "idvRuleId")
		if credentials.SceneID == "" {
			credentials.SceneID = configString(config, "idvSceneId")
		}
	case idvKindOnfido:
		credentials.APIKey = provider.ClientID
		credentials.WebhookSecret = provider.ClientSecret2
		credentials.SceneID = configString(config, "idvWorkflowId")
		if credentials.SceneID == "" {
			credentials.SceneID = configString(config, "idvSceneId")
		}
	case idvKindVeriff:
		credentials.APIKey = provider.ClientID
		credentials.APISecret = provider.ClientSecret
		credentials.WebhookSecret = provider.ClientSecret2
	case idvKindPersona:
		credentials.APIKey = provider.ClientID
		credentials.WebhookSecret = provider.ClientSecret2
		credentials.SceneID = configString(config, "idvTemplateId")
		if credentials.SceneID == "" {
			credentials.SceneID = configString(config, "idvSceneId")
		}
	default:
		return nil, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"idv: unsupported provider kind "+kind,
			map[string]any{
				"field":   "providerType",
				"value":   kind,
				"allowed": idvKinds,
			},
		)
	}
	if credentials.APIKey == "" || (idvKindsRequiringAPISecret[kind] && credentials.APISecret == "") {
		// A configured provider with missing credentials is an actionable operator
		// error (422), not a malformed request (400) nor "service unavailable"
		// (503). meta.field names the form control the UI should highlight.
		missing := idvMissingCredentialField(kind, credentials)
		return nil, apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("%s: provider credentials are incomplete (missing %s)", kind, missing),
			map[string]any{"field": missing, "kind": kind},
		)
	}
	return credentials, nil
}

// idvMissingCredentialField maps the absent credential half onto the management
// form field name so the UI can highlight the exact control to fill. Aliyun and
// Tencent store the AccessKey pair in admin/password; the token-based kinds use
// clientId/clientSecret.
func idvMissingCredentialField(kind string, credentials *domainidv.ProviderCredentials) string {
	switch kind {
	case idvKindAliyunIDV, idvKindTencentIDV:
		if credentials.APIKey == "" {
			return "admin"
		}
		return "password"
	default:
		if credentials.APIKey == "" {
			return "clientId"
		}
		return "clientSecret"
	}
}

func configString(config map[string]any, key string) string {
	if config == nil {
		return ""
	}
	value, ok := config[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func configBool(config map[string]any, key string) bool {
	if config == nil {
		return false
	}
	switch value := config[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}
