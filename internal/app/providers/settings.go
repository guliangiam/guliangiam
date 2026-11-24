package providers

import (
	"context"
	"encoding/json"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// settingsConfigType is the provider_configs row that carries the
// Casdoor-compatible provider options. Keeping them in JSON avoids a schema
// migration for every compatibility field while the typed DTO stays explicit.
const settingsConfigType = "_provider_settings_v1"

// providerSettings is the JSON shape persisted in provider_configs. Field names
// match the API so a downstream reader does not need a translation table.
type providerSettings struct {
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

func settingsFromProvider(provider *domain.Provider) providerSettings {
	if provider == nil {
		return providerSettings{}
	}
	settings := providerSettings{
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
	}
	if len(settings.HttpHeaders) == 0 {
		settings.HttpHeaders = nil
	}
	return settings
}

func (s providerSettings) applyTo(provider *domain.Provider) {
	if provider == nil {
		return
	}
	provider.EmailRegex = s.EmailRegex
	provider.EnablePkce = s.EnablePkce
	provider.EnableProxy = s.EnableProxy
	provider.HttpHeaders = s.HttpHeaders
	provider.RequireMessageAuthenticator = s.RequireMessageAuthenticator
	provider.EnableSignAuthnRequest = s.EnableSignAuthnRequest
	provider.CustomLogoutUrl = s.CustomLogoutUrl
	provider.CustomLogo = s.CustomLogo
	provider.Cert = s.Cert
	provider.State = s.State
}

func (s providerSettings) hasValues() bool {
	return s.EmailRegex != "" || s.EnablePkce || s.EnableProxy || len(s.HttpHeaders) > 0 ||
		s.RequireMessageAuthenticator || s.EnableSignAuthnRequest || s.CustomLogoutUrl != "" ||
		s.CustomLogo != "" || s.Cert != "" || s.State != ""
}

func (s providerSettings) config(providerID string) (*domain.ProviderConfig, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "marshal provider settings")
	}
	values := map[string]any{}
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, apperrors.WrapInternalf(err, "decode provider settings")
	}
	return &domain.ProviderConfig{
		ProviderID: providerID,
		ConfigType: settingsConfigType,
		Config:     values,
	}, nil
}

func decodeProviderSettings(config *domain.ProviderConfig) (providerSettings, error) {
	if config == nil {
		return providerSettings{}, nil
	}
	payload, err := json.Marshal(config.Config)
	if err != nil {
		return providerSettings{}, apperrors.WrapInternalf(err, "marshal provider settings config")
	}
	var settings providerSettings
	if err := json.Unmarshal(payload, &settings); err != nil {
		return providerSettings{}, apperrors.WrapInternalf(err, "decode provider settings")
	}
	return settings, nil
}

// loadProviderSettings reads the settings row. A missing row is the zero value,
// not an error: every provider created before C7 has no settings row.
func loadProviderSettings(ctx context.Context, repo domain.Repository, providerID string) (providerSettings, error) {
	settingsRepo, ok := repo.(registryConfigRepository)
	if !ok {
		return providerSettings{}, nil
	}
	config, err := settingsRepo.GetConfigByType(ctx, providerID, settingsConfigType)
	if err != nil {
		if isNotFound(err) {
			return providerSettings{}, nil
		}
		return providerSettings{}, err
	}
	return decodeProviderSettings(config)
}

// saveProviderSettings persists the options for an identity provider.
// `force` is true on the patch path: a PATCH may deliberately clear the last
// value, and skipping the write would leave the previous row in place. On
// create a zero-value record stays absent so unrelated providers do not gain
// an empty settings row.
func (s *UnifiedService) saveProviderSettings(ctx context.Context, tx appuow.UnitOfWork, provider *Provider, force bool) error {
	if provider == nil || provider.Registration.Family != domain.FamilyIdentity || provider.row == nil {
		return nil
	}
	settings := settingsFromProvider(provider.row)
	if !force && !settings.hasValues() {
		return nil
	}
	config, err := settings.config(provider.Registration.ID)
	if err != nil {
		return err
	}
	config.UpdatedAt = s.Clock.Now()
	return tx.Providers().UpdateConfig(ctx, config)
}
