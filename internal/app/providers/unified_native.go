package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainai "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainnotification "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

type aiProviderRepositorySource interface {
	AIProviders() domainai.ProvidersRepo
}

type paymentChannelConfigRepositorySource interface {
	PaymentChannelConfigs() domainpayment.ChannelConfigsRepo
}

type notificationProviderRepositorySource interface {
	NotificationProviders() domainnotification.Repository
}

type paymentDetailDeleter interface {
	Delete(ctx context.Context, id string) error
}

func (s *UnifiedService) cipher() FieldCipher {
	if s.Encryptor != nil {
		return s.Encryptor
	}
	return crypto.GlobalFieldEncryptor()
}

func fingerprintSecret(plaintext string) string {
	if plaintext == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:8])
}

func (s *UnifiedService) newProvider(ctx context.Context, tx appuow.UnitOfWork, command CreateProviderCommand) (*Provider, error) {
	family := normalizeFamily(command.Family)
	if family == "" {
		family = domain.FamilyIdentity
	}
	id := command.ID
	var err error
	if id == "" {
		id, err = s.UUID.NewString()
		if err != nil {
			return nil, err
		}
	}
	now := s.Clock.Now()
	displayName := command.DisplayName
	if family != domain.FamilyIdentity && strings.TrimSpace(displayName) == "" {
		displayName = command.Name
	}
	provider := &Provider{
		Registration: domain.Registration{
			ID: id, OrganizationID: command.OrganizationID, Name: command.Name, DisplayName: displayName,
			Family: family, Kind: domain.Kind(command.Kind), Variant: domain.Variant(command.Variant), Status: domain.StatusDraft,
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		},
		keyVersion: 0,
	}
	provider.Registration.Normalize()

	switch family {
	case domain.FamilyIdentity:
		if provider.Registration.Kind == "" {
			provider.Registration.Kind = canonicalIdentityKind(command.ProviderType)
		}
		if err := validateCreateProviderCommand(command); err != nil {
			return nil, err
		}
		scopes, err := normalizeProviderScopes(command.Scopes)
		if err != nil {
			return nil, err
		}
		provider.Detail = &domain.Provider{
			ID: id, OrganizationID: command.OrganizationID, Name: command.Name, DisplayName: displayName,
			Category: domain.ProviderCategory(command.Category), ProviderType: domain.ProviderType(command.ProviderType),
			SubType: command.SubType, Method: command.Method,
			ClientID: command.ClientID, ClientSecret: command.ClientSecret,
			ClientID2: command.ClientID2, ClientSecret2: command.ClientSecret2,
			ProviderURL: command.ProviderURL, AuthUrl: command.AuthUrl, TokenUrl: command.TokenUrl,
			UserInfoUrl: command.UserInfoUrl, CallbackUrl: command.CallbackUrl, SigningAlg: command.SigningAlg,
			Scopes:      scopes,
			IdPEntityId: command.IdPEntityId, SsoUrl: command.SsoUrl, SloUrl: command.SloUrl,
			Metadata: command.Metadata, IssuerUrl: command.IssuerUrl, Region: command.Region,
			Host: command.Host, Port: command.Port, DatabaseType: command.DatabaseType,
			UserMapping: command.UserMapping, Template: command.Template,
			CustomApi: command.CustomApi, CustomHeader: command.CustomHeader, CustomBody: command.CustomBody,
			Admin: command.Admin, Password: command.Password, AppId: command.AppId,
			AgentID:     command.AgentID,
			ContentType: command.ContentType, Sender: command.Sender, Receiver: command.Receiver,
			EmailRegex: command.EmailRegex, EnablePkce: command.EnablePkce, EnableProxy: command.EnableProxy,
			HttpHeaders: command.HttpHeaders, RequireMessageAuthenticator: command.RequireMessageAuthenticator,
			EnableSignAuthnRequest: command.EnableSignAuthnRequest, CustomLogoutUrl: command.CustomLogoutUrl,
			CustomLogo: command.CustomLogo, Cert: command.Cert, State: command.State,
			Enabled: false, CreatedAt: now, UpdatedAt: now, Revision: 1,
		}
		if command.ClientSecret != "" {
			provider.secretType = secretTypeClientSecret
			provider.plaintext = command.ClientSecret
			provider.fingerprint = fingerprintSecret(command.ClientSecret)
			provider.keyVersion = s.cipher().CurrentKeyVersion()
		}
	case domain.FamilyAI:
		if command.AI == nil {
			return nil, apperrors.NewInvalidArgument("ai provider config is required")
		}
		config := command.AI
		if provider.Registration.Kind == "" {
			provider.Registration.Kind = domain.Kind(strings.ToLower(config.ProviderType))
		}
		sealed, err := s.cipher().EncryptField(config.APIKey)
		if err != nil {
			return nil, err
		}
		provider.Detail = &domainai.LlmProvider{
			ID: id, OrganizationID: command.OrganizationID, Name: command.Name, DisplayName: displayName,
			ProviderType: config.ProviderType, BaseURL: config.BaseURL, APIKeySealed: sealed,
			APIKeyFingerprint: fingerprintSecret(config.APIKey), Status: domainai.ProviderStatusTesting,
			SupportedProtocols: append([]string(nil), config.SupportedProtocols...), PreferredProtocol: config.PreferredProtocol,
			RequestTimeoutMs: config.RequestTimeoutMs, MaxRetries: config.MaxRetries, Priority: config.Priority,
			Weight: config.Weight, RPMLimit: config.RPMLimit, TPMLimit: config.TPMLimit,
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		}
		provider.secretType = secretTypeAPIKey
		provider.plaintext = config.APIKey
		provider.fingerprint = fingerprintSecret(config.APIKey)
		provider.keyVersion = s.cipher().CurrentKeyVersion()
	case domain.FamilyPayment:
		if command.Payment == nil {
			return nil, apperrors.NewInvalidArgument("payment provider config is required")
		}
		config := command.Payment
		if provider.Registration.Kind == "" {
			provider.Registration.Kind = domain.Kind(strings.ToLower(config.Channel))
		}
		if provider.Registration.Variant == "" {
			provider.Registration.Variant = domain.Variant(config.Environment)
			if provider.Registration.Variant == "" {
				provider.Registration.Variant = domain.VariantProduction
			}
		}
		plaintext := config.Config
		sealed := config.ConfigSealed
		if plaintext != "" {
			sealed, err = s.cipher().EncryptField(plaintext)
			if err != nil {
				return nil, err
			}
		} else {
			if sealed == "" {
				return nil, apperrors.NewInvalidArgument("payment config is required")
			}
			plaintext, err = s.cipher().DecryptField(sealed)
			if err != nil {
				return nil, validateConfigSealed()
			}
		}
		channelRepo, err := paymentChannelConfigs(tx)
		if err != nil {
			return nil, err
		}
		existing, err := channelRepo.List(ctx, command.OrganizationID)
		if err != nil {
			return nil, err
		}
		version := 1
		for _, item := range existing {
			if item.Channel == config.Channel && item.Environment == string(provider.Registration.Variant) && item.VersionNo >= version {
				version = item.VersionNo + 1
			}
		}
		provider.Detail = &domainpayment.PayChannelConfig{
			ID: id, OrganizationID: command.OrganizationID, ProviderID: id, Channel: config.Channel,
			Environment: string(provider.Registration.Variant), ConfigSealed: sealed,
			ConfigFingerprint: fingerprintSecret(plaintext), ConfigSet: append([]string(nil), config.ConfigSet...),
			VersionNo: version, Status: domainpayment.ChannelConfigStatusDraft,
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		}
		provider.secretType = secretTypeConfig
		provider.plaintext = plaintext
		provider.fingerprint = fingerprintSecret(plaintext)
		provider.keyVersion = s.cipher().CurrentKeyVersion()
	case domain.FamilyNotification:
		if command.Notification == nil {
			return nil, apperrors.NewInvalidArgument("notification provider config is required")
		}
		config := command.Notification
		if provider.Registration.Kind == "" {
			provider.Registration.Kind = domain.Kind(strings.ToLower(config.ProviderType))
		}
		values := config.Config
		if values == nil {
			values = map[string]any{}
		}
		provider.Detail = &domainnotification.NotificationProvider{
			ID: id, OrganizationID: command.OrganizationID, Name: command.Name, ProviderType: config.ProviderType,
			WebhookURL: config.WebhookURL, AccessToken: config.AccessToken, Config: values, IsEnabled: false,
			CreatedAt: now, UpdatedAt: now, Revision: 1,
		}
		if config.AccessToken != "" {
			provider.secretType = secretTypeAccessToken
			provider.plaintext = config.AccessToken
			provider.fingerprint = fingerprintSecret(config.AccessToken)
			provider.keyVersion = s.cipher().CurrentKeyVersion()
		}
	default:
		return nil, domain.ValidateFamily(family)
	}
	provider.Registration.Normalize()
	if err := provider.Registration.Validate(); err != nil {
		return nil, err
	}
	provider.metadata = metadataFromRegistration(provider.Registration, id)
	return provider, nil
}

func aiProviders(source any) (domainai.ProvidersRepo, error) {
	repos, ok := source.(aiProviderRepositorySource)
	if !ok || repos.AIProviders() == nil {
		return nil, apperrors.NewUnimplemented("ai provider repository is unavailable in provider unit of work")
	}
	return repos.AIProviders(), nil
}

func paymentChannelConfigs(source any) (domainpayment.ChannelConfigsRepo, error) {
	repos, ok := source.(paymentChannelConfigRepositorySource)
	if !ok || repos.PaymentChannelConfigs() == nil {
		return nil, apperrors.NewUnimplemented("payment channel config repository is unavailable in provider unit of work")
	}
	return repos.PaymentChannelConfigs(), nil
}

func notificationProviders(source any) (domainnotification.Repository, error) {
	repos, ok := source.(notificationProviderRepositorySource)
	if !ok || repos.NotificationProviders() == nil {
		return nil, apperrors.NewUnimplemented("notification provider repository is unavailable in provider unit of work")
	}
	return repos.NotificationProviders(), nil
}

func (s *UnifiedService) loadNativeDetail(ctx context.Context, source any, provider *Provider) error {
	detailID := provider.metadata.DetailID
	switch provider.Registration.Family {
	case domain.FamilyAI:
		repo, err := aiProviders(source)
		if err != nil {
			return err
		}
		detail, err := repo.GetByID(ctx, detailID)
		if err != nil {
			return err
		}
		provider.Detail = detail
	case domain.FamilyPayment:
		repo, err := paymentChannelConfigs(source)
		if err != nil {
			return err
		}
		detail, err := repo.GetByID(ctx, detailID)
		if err != nil {
			return err
		}
		provider.Detail = detail
	case domain.FamilyNotification:
		repo, err := notificationProviders(source)
		if err != nil {
			return err
		}
		detail, err := repo.GetByID(ctx, detailID)
		if err != nil {
			return err
		}
		provider.Detail = detail
	case domain.FamilyIdentity:
		return nil // identity 注册无 native detail；调用方（unified_registry.go）已跳过
	default:
		return domain.ValidateFamily(provider.Registration.Family)
	}
	return nil
}

func syncNativeStatus(provider *Provider) {
	status := provider.Registration.Status
	switch detail := provider.Detail.(type) {
	case *domain.Provider:
		detail.Enabled = status == domain.StatusActive
	case *domainai.LlmProvider:
		if status == domain.StatusDraft {
			detail.Status = domainai.ProviderStatusTesting
		} else {
			detail.Status = string(status)
		}
	case *domainpayment.PayChannelConfig:
		detail.Status = string(status)
	case *domainnotification.NotificationProvider:
		detail.IsEnabled = status == domain.StatusActive
	}
}

func (s *UnifiedService) createNativeDetail(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error {
	syncNativeStatus(provider)
	switch detail := provider.Detail.(type) {
	case *domainai.LlmProvider:
		repo, err := aiProviders(tx)
		if err != nil {
			return err
		}
		return repo.Create(ctx, detail)
	case *domainpayment.PayChannelConfig:
		repo, err := paymentChannelConfigs(tx)
		if err != nil {
			return err
		}
		return repo.Create(ctx, detail)
	case *domainnotification.NotificationProvider:
		repo, err := notificationProviders(tx)
		if err != nil {
			return err
		}
		return repo.Create(ctx, detail)
	default:
		return apperrors.NewInternal("invalid native provider detail")
	}
}

func (s *UnifiedService) updateNativeDetail(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error {
	syncNativeStatus(provider)
	now := s.Clock.Now()
	switch detail := provider.Detail.(type) {
	case *domainai.LlmProvider:
		detail.UpdatedAt = now
		detail.Revision++
		repo, err := aiProviders(tx)
		if err != nil {
			return err
		}
		return repo.Update(ctx, detail)
	case *domainpayment.PayChannelConfig:
		detail.UpdatedAt = now
		detail.Revision++
		repo, err := paymentChannelConfigs(tx)
		if err != nil {
			return err
		}
		return repo.Update(ctx, detail)
	case *domainnotification.NotificationProvider:
		detail.UpdatedAt = now
		detail.Revision++
		repo, err := notificationProviders(tx)
		if err != nil {
			return err
		}
		return repo.Update(ctx, detail)
	default:
		return apperrors.NewInternal("invalid native provider detail")
	}
}

func applyProviderPatch(provider *Provider, command PatchProviderCommand) error {
	registration := &provider.Registration
	if command.Name != nil {
		registration.Name = strings.TrimSpace(*command.Name)
	}
	if command.DisplayName != nil {
		registration.DisplayName = strings.TrimSpace(*command.DisplayName)
	}
	if command.Kind != nil {
		registration.Kind = domain.Kind(strings.ToLower(strings.TrimSpace(*command.Kind)))
	}
	if command.Variant != nil {
		registration.Variant = domain.Variant(strings.ToLower(strings.TrimSpace(*command.Variant)))
	}
	switch detail := provider.Detail.(type) {
	case *domain.Provider:
		if command.Category != nil {
			detail.Category = domain.ProviderCategory(*command.Category)
		}
		if command.ProviderType != nil {
			detail.ProviderType = domain.ProviderType(*command.ProviderType)
			registration.Kind = canonicalIdentityKind(*command.ProviderType)
		}
		if command.ClientID != nil {
			detail.ClientID = *command.ClientID
		}
		if command.ProviderURL != nil {
			detail.ProviderURL = *command.ProviderURL
		}
		applyOptionalString := func(target *string, value *string) {
			if value != nil {
				*target = *value
			}
		}
		applyOptionalString(&detail.SubType, command.SubType)
		applyOptionalString(&detail.Method, command.Method)
		applyOptionalString(&detail.ClientID2, command.ClientID2)
		applyOptionalString(&detail.AuthUrl, command.AuthUrl)
		applyOptionalString(&detail.TokenUrl, command.TokenUrl)
		applyOptionalString(&detail.UserInfoUrl, command.UserInfoUrl)
		applyOptionalString(&detail.CallbackUrl, command.CallbackUrl)
		applyOptionalString(&detail.SigningAlg, command.SigningAlg)
		applyOptionalString(&detail.IdPEntityId, command.IdPEntityId)
		applyOptionalString(&detail.SsoUrl, command.SsoUrl)
		applyOptionalString(&detail.SloUrl, command.SloUrl)
		applyOptionalString(&detail.Metadata, command.Metadata)
		applyOptionalString(&detail.IssuerUrl, command.IssuerUrl)
		applyOptionalString(&detail.Region, command.Region)
		applyOptionalString(&detail.Host, command.Host)
		applyOptionalString(&detail.DatabaseType, command.DatabaseType)
		applyOptionalString(&detail.Template, command.Template)
		applyOptionalString(&detail.CustomApi, command.CustomApi)
		applyOptionalString(&detail.CustomHeader, command.CustomHeader)
		applyOptionalString(&detail.CustomBody, command.CustomBody)
		applyOptionalString(&detail.Admin, command.Admin)
		applyOptionalString(&detail.AppId, command.AppId)
		applyOptionalString(&detail.AgentID, command.AgentID)
		applyOptionalString(&detail.ContentType, command.ContentType)
		applyOptionalString(&detail.Sender, command.Sender)
		applyOptionalString(&detail.Receiver, command.Receiver)
		if command.Scopes != nil && providerScopesChanged(detail.Scopes, command.Scopes) {
			scopes, err := normalizeProviderScopes(command.Scopes)
			if err != nil {
				return err
			}
			detail.Scopes = scopes
		}
		if command.Port != nil {
			detail.Port = *command.Port
		}
		if command.UserMapping != nil {
			detail.UserMapping = command.UserMapping
		}
		applyOptionalString(&detail.EmailRegex, command.EmailRegex)
		applyOptionalString(&detail.CustomLogoutUrl, command.CustomLogoutUrl)
		applyOptionalString(&detail.CustomLogo, command.CustomLogo)
		applyOptionalString(&detail.Cert, command.Cert)
		applyOptionalString(&detail.State, command.State)
		if command.EnablePkce != nil {
			detail.EnablePkce = *command.EnablePkce
		}
		if command.EnableProxy != nil {
			detail.EnableProxy = *command.EnableProxy
		}
		if command.RequireMessageAuthenticator != nil {
			detail.RequireMessageAuthenticator = *command.RequireMessageAuthenticator
		}
		if command.EnableSignAuthnRequest != nil {
			detail.EnableSignAuthnRequest = *command.EnableSignAuthnRequest
		}
		if command.HttpHeaders != nil {
			detail.HttpHeaders = command.HttpHeaders
		}
		// clientSecret 走 rotateNativeSecret 的密钥轮换路径；第二密钥与密码
		// 没有独立轮换记录，直接写回原生行（与创建时的落库方式一致）。
		if command.ClientSecret2 != nil {
			detail.ClientSecret2 = *command.ClientSecret2
		}
		if command.Password != nil {
			detail.Password = *command.Password
		}
		detail.Name = registration.Name
		detail.DisplayName = registration.DisplayName
	case *domainai.LlmProvider:
		if command.AI != nil {
			patch := command.AI
			if patch.ProviderType != nil {
				detail.ProviderType = *patch.ProviderType
				registration.Kind = domain.Kind(strings.ToLower(*patch.ProviderType))
			}
			if patch.BaseURL != nil {
				detail.BaseURL = *patch.BaseURL
			}
			if patch.SupportedProtocols != nil {
				detail.SupportedProtocols = append([]string(nil), patch.SupportedProtocols...)
			}
			if patch.PreferredProtocol != nil {
				detail.PreferredProtocol = *patch.PreferredProtocol
			}
			if patch.RequestTimeoutMs != nil {
				detail.RequestTimeoutMs = *patch.RequestTimeoutMs
			}
			if patch.MaxRetries != nil {
				detail.MaxRetries = *patch.MaxRetries
			}
			if patch.Priority != nil {
				detail.Priority = *patch.Priority
			}
			if patch.Weight != nil {
				detail.Weight = *patch.Weight
			}
			if patch.RPMLimit != nil {
				detail.RPMLimit = patch.RPMLimit
			}
			if patch.TPMLimit != nil {
				detail.TPMLimit = patch.TPMLimit
			}
		}
		detail.Name = registration.Name
		detail.DisplayName = registration.DisplayName
		if registration.Kind != "" {
			detail.ProviderType = string(registration.Kind)
		}
	case *domainpayment.PayChannelConfig:
		if command.Payment != nil {
			if command.Payment.Channel != "" && !strings.EqualFold(command.Payment.Channel, detail.Channel) {
				return apperrors.NewInvalidArgument("payment channel is immutable; create a new provider")
			}
			if command.Payment.Environment != "" && !strings.EqualFold(command.Payment.Environment, detail.Environment) {
				return apperrors.NewInvalidArgument("payment environment is immutable; create a new provider")
			}
		}
		registration.Kind = domain.Kind(strings.ToLower(detail.Channel))
		registration.Variant = domain.Variant(detail.Environment)
	case *domainnotification.NotificationProvider:
		if command.Notification != nil {
			patch := command.Notification
			if patch.ProviderType != nil {
				detail.ProviderType = *patch.ProviderType
				registration.Kind = domain.Kind(strings.ToLower(*patch.ProviderType))
			}
			if patch.WebhookURL != nil {
				detail.WebhookURL = *patch.WebhookURL
			}
			if patch.Config != nil {
				detail.Config = patch.Config
			}
		}
		detail.Name = registration.Name
		if registration.Kind != "" {
			detail.ProviderType = string(registration.Kind)
		}
	default:
		return apperrors.NewInternal("invalid native provider detail")
	}
	registration.Normalize()
	if provider.legacy && registration.Family == domain.FamilyIdentity {
		return nil
	}
	return registration.Validate()
}

func (s *UnifiedService) rotateNativeSecret(ctx context.Context, tx appuow.UnitOfWork, provider *Provider, command RotateProviderSecretCommand) error {
	plaintext := command.Plaintext
	if plaintext == "" {
		return apperrors.NewInvalidArgument("secret is required")
	}
	secretType := command.SecretType
	if secretType == "" {
		switch provider.Registration.Family {
		case domain.FamilyIdentity:
			secretType = secretTypeClientSecret
		case domain.FamilyAI:
			secretType = secretTypeAPIKey
		case domain.FamilyPayment:
			secretType = secretTypeConfig
		case domain.FamilyNotification:
			secretType = secretTypeAccessToken
		}
	}
	provider.secretType = secretType
	provider.plaintext = plaintext
	provider.fingerprint = fingerprintSecret(plaintext)
	provider.keyVersion = s.cipher().CurrentKeyVersion()

	if err := validateSecretType(provider.Registration.Family, secretType); err != nil {
		return err
	}

	switch detail := provider.Detail.(type) {
	case *domain.Provider:
		switch secretType {
		case secretTypeClientSecret:
			detail.ClientSecret = plaintext
		case secretTypeClientSecret2:
			detail.ClientSecret2 = plaintext
		case secretTypePassword:
			detail.Password = plaintext
		case secretTypeAppSecret:
			detail.AppSecret = plaintext
		}
		detail.UpdatedAt = s.Clock.Now()
		detail.Revision = provider.Registration.Revision
	case *domainai.LlmProvider:
		sealed, err := s.cipher().EncryptField(plaintext)
		if err != nil {
			return err
		}
		detail.APIKeySealed = sealed
		detail.APIKeyFingerprint = provider.fingerprint
		return s.updateNativeDetail(ctx, tx, provider)
	case *domainnotification.NotificationProvider:
		detail.AccessToken = plaintext
		return s.updateNativeDetail(ctx, tx, provider)
	case *domainpayment.PayChannelConfig:
		sealed, err := s.cipher().EncryptField(plaintext)
		if err != nil {
			return err
		}
		repo, err := paymentChannelConfigs(tx)
		if err != nil {
			return err
		}
		now := s.Clock.Now()
		newID, err := s.UUID.NewString()
		if err != nil {
			return err
		}
		next := *detail
		if next.ProviderID != "" && next.ProviderID != provider.Registration.ID {
			return apperrors.NewInternal("payment provider anchor mismatch")
		}
		if next.ProviderID == "" {
			next.ProviderID = provider.Registration.ID
		}
		next.ID = newID
		next.ConfigSealed = sealed
		next.ConfigFingerprint = provider.fingerprint
		next.VersionNo = detail.VersionNo + 1
		next.CreatedAt = now
		next.UpdatedAt = now
		next.Revision = 1
		next.DeletedAt = nil
		detail.Status = domainpayment.ChannelConfigStatusDisabled
		detail.DisabledAt = &now
		detail.UpdatedAt = now
		detail.Revision++
		if err := repo.Update(ctx, detail); err != nil {
			return err
		}
		syncNativeStatus(provider)
		provider.Detail = &next
		provider.metadata.DetailID = newID
		return repo.Create(ctx, &next)
	default:
		return apperrors.NewInternal("invalid native provider detail")
	}
	return nil
}

func (s *UnifiedService) deleteNativeDetail(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error {
	now := s.Clock.Now()
	switch detail := provider.Detail.(type) {
	case *domainai.LlmProvider:
		detail.Status = domainai.ProviderStatusDisabled
		detail.DeletedAt = &now
		detail.UpdatedAt = now
		detail.Revision++
		repo, err := aiProviders(tx)
		if err != nil {
			return err
		}
		return repo.Update(ctx, detail)
	case *domainpayment.PayChannelConfig:
		repo, err := paymentChannelConfigs(tx)
		if err != nil {
			return err
		}
		deleter, ok := repo.(paymentDetailDeleter)
		if !ok {
			return apperrors.NewUnimplemented("payment channel soft delete is unavailable")
		}
		return deleter.Delete(ctx, detail.ID)
	case *domainnotification.NotificationProvider:
		detail.IsEnabled = false
		detail.UpdatedAt = now
		detail.Revision++
		repo, err := notificationProviders(tx)
		if err != nil {
			return err
		}
		return repo.Update(ctx, detail)
	default:
		return apperrors.NewInternal("invalid native provider detail")
	}
}

// paymentConfigFromSealed opens the stored channel config with the row AAD, so
// a v2 token is authenticated against (pay_channel_configs, config_sealed,
// detail.ID) while existing v1/v0 rows stay readable. Decryption failures keep
// the cause chain so ErrAADMismatch remains inspectable by callers.
func paymentConfigFromSealed(cipher FieldCipher, provider *Provider) (string, error) {
	detail := provider.Detail.(*domainpayment.PayChannelConfig)
	plaintext, err := cipher.DecryptColumn(crypto.PayChannelConfigSealedColumn.AAD(detail.ID), detail.ConfigSealed)
	if err != nil {
		return "", apperrors.WrapInternal(err, "decrypt payment provider config")
	}
	provider.plaintext = plaintext
	return plaintext, nil
}
