package providers

import (
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func toListItem(provider *domain.Provider) ProviderListItem {
	status := domain.StatusDisabled
	if provider.Enabled {
		status = domain.StatusActive
	}
	return ProviderListItem{
		ID:             provider.ID,
		OrganizationID: provider.OrganizationID,
		Name:           provider.Name,
		DisplayName:    provider.DisplayName,
		Family:         string(domain.FamilyIdentity),
		Kind:           strings.ToLower(string(provider.ProviderType)),
		Status:         string(status),
		Enabled:        provider.Enabled,
		Category:       string(provider.Category),
		ProviderType:   string(provider.ProviderType),
		SubType:        provider.SubType,
		ClientID:       provider.ClientID,
		ProviderURL:    provider.ProviderURL,
		CreatedAt:      provider.CreatedAt,
		UpdatedAt:      provider.UpdatedAt,
		Revision:       provider.Revision,
	}
}

func toUnifiedListItem(provider *Provider) ProviderListItem {
	item := ProviderListItem{
		ID: provider.Registration.ID, OrganizationID: provider.Registration.OrganizationID,
		Name: provider.Registration.Name, DisplayName: provider.Registration.DisplayName,
		Family: string(provider.Registration.Family), Kind: string(provider.Registration.Kind),
		Variant: string(provider.Registration.Variant), Status: string(provider.Registration.Status),
		Enabled: provider.Registration.IsRoutable(), Category: string(provider.Registration.Family),
		ProviderType: string(provider.Registration.Kind), SubType: string(provider.Registration.Variant),
		CreatedAt: provider.Registration.CreatedAt, UpdatedAt: provider.Registration.UpdatedAt,
		Revision: provider.Registration.Revision,
	}
	if provider.Registration.Family == domain.FamilyIdentity && provider.row != nil {
		item = toListItem(provider.row)
		item.Family = string(provider.Registration.Family)
		item.Kind = string(provider.Registration.Kind)
		item.Variant = string(provider.Registration.Variant)
		item.Status = string(provider.Registration.Status)
		item.Enabled = provider.Registration.IsRoutable()
		item.Revision = provider.Registration.Revision
	}
	return item
}

func toUnifiedDetail(service *UnifiedService, provider *Provider) (*ProviderDetail, error) {
	item := toUnifiedListItem(provider)
	detail := &ProviderDetail{ProviderListItem: item, Revision: provider.Registration.Revision}
	if provider.Registration.Family == domain.FamilyIdentity {
		if provider.row == nil {
			return nil, apperrors.NewInternal("identity provider detail is missing")
		}
		detail = toDetail(provider.row)
		detail.ProviderListItem = item
		detail.Revision = provider.Registration.Revision
		return detail, nil
	}
	strategy, err := service.strategy(provider.Registration.Family)
	if err != nil {
		return nil, err
	}
	detail.Detail = strategy.Sanitize(provider)
	return detail, nil
}

func toDetail(provider *domain.Provider) *ProviderDetail {
	return &ProviderDetail{
		ProviderListItem:            toListItem(provider),
		ClientSecretSet:             provider.ClientSecret != "",
		ClientSecretMasked:          maskSecret(provider.ClientSecret),
		ClientID2:                   provider.ClientID2,
		ClientSecret2Set:            provider.ClientSecret2 != "",
		ClientSecret2Masked:         maskSecret(provider.ClientSecret2),
		Method:                      provider.Method,
		AuthUrl:                     provider.AuthUrl,
		TokenUrl:                    provider.TokenUrl,
		UserInfoUrl:                 provider.UserInfoUrl,
		CallbackUrl:                 provider.CallbackUrl,
		SigningAlg:                  provider.SigningAlg,
		Scopes:                      append([]string(nil), provider.Scopes...),
		IdPEntityId:                 provider.IdPEntityId,
		SsoUrl:                      provider.SsoUrl,
		SloUrl:                      provider.SloUrl,
		Metadata:                    provider.Metadata,
		IssuerUrl:                   provider.IssuerUrl,
		Region:                      provider.Region,
		Host:                        provider.Host,
		Port:                        provider.Port,
		DatabaseType:                provider.DatabaseType,
		UserMapping:                 provider.UserMapping,
		Template:                    provider.Template,
		CustomApi:                   provider.CustomApi,
		CustomHeader:                provider.CustomHeader,
		CustomBody:                  provider.CustomBody,
		Admin:                       provider.Admin,
		PasswordSet:                 provider.Password != "",
		PasswordMasked:              maskSecret(provider.Password),
		AppId:                       provider.AppId,
		AgentID:                     provider.AgentID,
		AppSecretSet:                provider.AppSecret != "",
		AppSecretMasked:             maskSecret(provider.AppSecret),
		ContentType:                 provider.ContentType,
		Sender:                      provider.Sender,
		Receiver:                    provider.Receiver,
		EmailRegex:                  provider.EmailRegex,
		EnablePkce:                  provider.EnablePkce,
		EnableProxy:                 provider.EnableProxy,
		HttpHeaders:                 provider.HttpHeaders,
		RequireMessageAuthenticator: provider.RequireMessageAuthenticator,
		EnableSignAuthnRequest:      provider.EnableSignAuthnRequest,
		CustomLogoutUrl:             provider.CustomLogoutUrl,
		CustomLogo:                  provider.CustomLogo,
		Cert:                        provider.Cert,
		State:                       provider.State,
		Revision:                    provider.Revision,
	}
}

// maskSecret returns first 2 chars + *** + last 2 chars, or empty string.
// Short secrets (< 6 chars) return fully masked "******".
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) < 6 {
		return "******"
	}
	return s[:2] + "***" + s[len(s)-2:]
}

func toConfigDetail(config *domain.ProviderConfig) *ProviderConfigDetail {
	return &ProviderConfigDetail{
		ProviderID: config.ProviderID,
		ConfigType: config.ConfigType,
		Config:     config.Config,
		UpdatedAt:  config.UpdatedAt,
	}
}

func toSecretStatusItems(items []domain.ProviderSecretStatus) []ProviderSecretStatusItem {
	result := make([]ProviderSecretStatusItem, 0, len(items))
	for i := range items {
		item := &items[i]
		result = append(result, ProviderSecretStatusItem{
			ID:            item.ID,
			ProviderID:    item.ProviderID,
			SecretType:    item.SecretType,
			Status:        item.Status,
			LastRotatedAt: item.LastRotatedAt,
			ExpiresAt:     item.ExpiresAt,
			UpdatedAt:     item.UpdatedAt,
			Version:       item.Version,
		})
	}
	return result
}
