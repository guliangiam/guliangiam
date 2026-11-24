package providers

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func providerFromRow(row *domain.Provider, metadata registryMetadata) (*Provider, error) {
	provider := &Provider{row: row, metadata: metadata, legacy: metadata.Family == ""}
	if metadata.Family == "" {
		status := domain.StatusDisabled
		if row.Enabled {
			status = domain.StatusActive
		}
		provider.Registration = domain.Registration{
			ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name, DisplayName: row.DisplayName,
			Family: domain.FamilyIdentity, Kind: canonicalIdentityKind(string(row.ProviderType)),
			Status: status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
		}
		provider.Detail = row
		provider.metadata = metadataFromRegistration(provider.Registration, row.ID)
		return provider, nil
	}
	registration := domain.Registration{
		ID: row.ID, OrganizationID: row.OrganizationID, Name: row.Name, DisplayName: row.DisplayName,
		Family: domain.Family(metadata.Family), Kind: domain.Kind(metadata.Kind), Variant: domain.Variant(metadata.Variant),
		Status: domain.Status(metadata.Status), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Revision: row.Revision,
	}
	if err := registration.Validate(); err != nil {
		return nil, err
	}
	provider.Registration = registration
	if registration.Family == domain.FamilyIdentity {
		provider.Detail = row
	}
	return provider, nil
}

func (s *UnifiedService) loadProvider(ctx context.Context, repo domain.Repository, nativeSource any, id, family, organizationID string, withDetail bool) (*Provider, error) {
	var (
		row *domain.Provider
		err error
	)
	if strings.TrimSpace(organizationID) == "" {
		// System scope for unrouted lifecycle helpers (ChangeStatus/RotateSecret).
		// Every routed by-ID method requires a trusted tenant before reaching
		// this loader.
		row, err = repo.GetByID(ctx, id)
	} else {
		row, err = repo.GetByIDForOrganization(ctx, id, organizationID)
	}
	if err != nil {
		return nil, err
	}
	metadata, err := getRegistryMetadata(ctx, repo, id)
	if err != nil && !isNotFound(err) {
		return nil, err
	}
	provider, err := providerFromRow(row, metadata)
	if err != nil {
		return nil, err
	}
	if provider.Registration.Family == domain.FamilyIdentity {
		settings, err := loadProviderSettings(ctx, repo, id)
		if err != nil {
			return nil, err
		}
		settings.applyTo(provider.row)
	}
	if family != "" && provider.Registration.Family != normalizeFamily(family) {
		return nil, apperrors.NewNotFound("provider not found")
	}
	if withDetail && provider.Registration.Family != domain.FamilyIdentity {
		if provider.metadata.DetailID == "" {
			return nil, apperrors.NewInternal("provider detail anchor is missing")
		}
		if err := s.loadNativeDetail(ctx, nativeSource, provider); err != nil {
			return nil, err
		}
	}
	return provider, nil
}

func (s *UnifiedService) loadProviderTx(ctx context.Context, tx appuow.UnitOfWork, id, family, organizationID string) (*Provider, error) {
	return s.loadProvider(ctx, tx.Providers(), tx, id, family, organizationID, true)
}

func newRegistrationRow(provider *Provider) *domain.Provider {
	registration := provider.Registration
	return &domain.Provider{
		ID: registration.ID, OrganizationID: registration.OrganizationID,
		Name: registration.Name, DisplayName: registration.DisplayName,
		Category:     domain.ProviderCategory(strings.ToUpper(string(registration.Family))),
		ProviderType: domain.ProviderType(registration.Kind), SubType: string(registration.Variant),
		Family: registration.Family, Kind: registration.Kind, Variant: registration.Variant,
		Scopes: []string{}, Enabled: registration.Status == domain.StatusActive,
		CreatedAt: registration.CreatedAt, UpdatedAt: registration.UpdatedAt, Revision: registration.Revision,
	}
}

func syncRegistration(provider *Provider) {
	registration := &provider.Registration
	provider.metadata.Family = string(registration.Family)
	provider.metadata.Kind = string(registration.Kind)
	provider.metadata.Variant = string(registration.Variant)
	provider.metadata.Status = string(registration.Status)
	if provider.metadata.DetailID == "" {
		provider.metadata.DetailID = registration.ID
	}
	row := provider.row
	row.ID = registration.ID
	row.OrganizationID = registration.OrganizationID
	row.Name = registration.Name
	row.DisplayName = registration.DisplayName
	row.Family = registration.Family
	row.Kind = registration.Kind
	row.Variant = registration.Variant
	row.Enabled = registration.IsRoutable()
	row.CreatedAt = registration.CreatedAt
	row.UpdatedAt = registration.UpdatedAt
	row.Revision = registration.Revision
	if registration.Family != domain.FamilyIdentity {
		row.Category = domain.ProviderCategory(strings.ToUpper(string(registration.Family)))
		row.ProviderType = domain.ProviderType(registration.Kind)
		row.SubType = string(registration.Variant)
		if row.Scopes == nil {
			row.Scopes = []string{}
		}
		return
	}
	// identity：provider_type 保留厂商写法（AzureAD / AlibabaSMS / WeCom …），
	// kind 只承载注册表键。两者曾被强制写成同一个值，导致创建后 types 选择器
	// 回显 kind（azure_ad、custom）而不是厂商值；这里只在该列为空时回落。
	if strings.TrimSpace(string(row.ProviderType)) == "" {
		row.ProviderType = domain.ProviderType(registration.Kind)
	}
}

func (s *UnifiedService) saveMetadata(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error {
	syncRegistration(provider)
	config, err := provider.metadata.config(provider.Registration.ID, s.Clock.Now())
	if err != nil {
		return err
	}
	return tx.Providers().UpdateConfig(ctx, config)
}

func (s *UnifiedService) createNative(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error {
	if provider.Registration.Family == domain.FamilyIdentity {
		provider.row = provider.Detail.(*domain.Provider)
		syncRegistration(provider)
		if err := tx.Providers().Create(ctx, provider.row); err != nil {
			return err
		}
	} else {
		provider.row = newRegistrationRow(provider)
		if err := tx.Providers().Create(ctx, provider.row); err != nil {
			return err
		}
		if err := s.createNativeDetail(ctx, tx, provider); err != nil {
			return err
		}
	}
	return nil
}

func (s *UnifiedService) updateRegistration(ctx context.Context, tx appuow.UnitOfWork, provider *Provider) error {
	legacyIdentity := provider.legacy && provider.Registration.Family == domain.FamilyIdentity
	syncRegistration(provider)
	if err := tx.Providers().Update(ctx, provider.row); err != nil {
		return err
	}
	if legacyIdentity {
		return nil
	}
	return s.saveMetadata(ctx, tx, provider)
}
