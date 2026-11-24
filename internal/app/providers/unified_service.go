package providers

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainnotification "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	domainpayment "github.com/guliangiam/guliangiam/internal/domain/payments"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type FieldCipher interface {
	EncryptField(plaintext string) (string, error)
	DecryptField(ciphertext string) (string, error)
	// DecryptColumn authenticates v2 tokens against the row AAD while keeping
	// v1/v0 values readable. Stored-row reads must use it instead of
	// DecryptField, which has no row binding.
	DecryptColumn(aad crypto.AADContext, ciphertext string) (string, error)
	CurrentKeyVersion() int
}

type PaymentConnectionTester func(ctx context.Context, provider *domainpayment.PayChannelConfig, plaintext string) error
type NotificationConnectionTester func(ctx context.Context, provider *domainnotification.NotificationProvider) error

// IDVConnectionProbe performs a read-only authenticated call against a real
// identity-verification provider. Wired at the composition root because the
// HTTP adapter lives in infra.
type IDVConnectionProbe func(ctx context.Context, provider *domain.Provider, config map[string]any) error

type UnifiedService struct {
	Clock              clock.Clock
	UUID               platformuuid.Generator
	UOW                appuow.TxRunner
	Encryptor          FieldCipher
	HTTPClient         *http.Client
	PaymentTester      PaymentConnectionTester
	NotificationTester NotificationConnectionTester
	IDVProbe           IDVConnectionProbe
	Strategies         map[domain.Family]ProviderStrategy
	// CacheInvalidator drops cached messaging configs for the organization
	// whose provider row just changed. Nil keeps the cache-only TTL behaviour.
	CacheInvalidator ProviderCacheInvalidator
}

func NewUnifiedService(c clock.Clock, u platformuuid.Generator, unitOfWork appuow.TxRunner) *UnifiedService {
	return &UnifiedService{Clock: c, UUID: u, UOW: unitOfWork}
}

func (s *UnifiedService) strategy(family domain.Family) (ProviderStrategy, error) {
	if s.Strategies != nil {
		strategy, ok := s.Strategies[family]
		if !ok {
			return nil, domain.ValidateFamily(family)
		}
		return strategy, nil
	}
	strategies := s.defaultStrategies()
	strategy, ok := strategies[family]
	if !ok {
		return nil, domain.ValidateFamily(family)
	}
	return strategy, nil
}

func (s *UnifiedService) Create(ctx context.Context, command CreateProviderCommand) (*Provider, error) {
	if s.UOW == nil {
		return nil, apperrors.NewInternal("provider unit of work is required")
	}
	var created *Provider
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		provider, err := s.newProvider(ctx, tx, command)
		if err != nil {
			return err
		}
		if err := validateDelegatedIdentityCreate(provider); err != nil {
			return err
		}
		strategy, err := s.strategy(provider.Registration.Family)
		if err != nil {
			return err
		}
		if err := strategy.Validate(provider); err != nil {
			return err
		}
		if err := s.createNative(ctx, tx, provider); err != nil {
			return err
		}
		if provider.secretType != "" {
			if err := s.persistSecretStatus(ctx, tx, provider, 1, 0); err != nil {
				return err
			}
		}
		if err := s.saveMetadata(ctx, tx, provider); err != nil {
			return err
		}
		if err := s.saveProviderSettings(ctx, tx, provider, false); err != nil {
			return err
		}
		if err := s.embedAudit(ctx, tx, provider, command.ActorID, "created", map[string]any{
			"family": provider.Registration.Family, metaKeyKind: provider.Registration.Kind, "variant": provider.Registration.Variant,
		}); err != nil {
			return err
		}
		created = provider
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidateProviderCache(created)
	return created, nil
}

// invalidateProviderCache drops messaging resolutions for the provider's
// organization after a committed write. The cache is process-local, so other
// instances converge through the TTL instead.
func (s *UnifiedService) invalidateProviderCache(provider *Provider) {
	if s.CacheInvalidator == nil || provider == nil {
		return
	}
	s.CacheInvalidator.Invalidate(provider.Registration.OrganizationID)
}

func (s *UnifiedService) Get(ctx context.Context, id, family, organizationID string) (*Provider, error) {
	repoSource, ok := s.UOW.(interface{ Providers() domain.Repository })
	if !ok {
		return nil, apperrors.NewInternal("provider registry repository is unavailable")
	}
	repo := repoSource.Providers()
	if repo == nil {
		return nil, apperrors.NewInternal("provider registry repository is unavailable")
	}
	return s.loadProvider(ctx, repo, s.UOW, id, family, organizationID, true)
}

func (s *UnifiedService) List(ctx context.Context, query ListProvidersQuery) ([]Provider, int64, error) {
	repoSource, ok := s.UOW.(interface{ Providers() domain.Repository })
	if !ok {
		return nil, 0, apperrors.NewInternal("provider registry repository is unavailable")
	}
	repo := repoSource.Providers()
	if repo == nil {
		return nil, 0, apperrors.NewInternal("provider registry repository is unavailable")
	}
	baseQuery := domain.ListQuery{
		OrganizationID: query.OrganizationID,
		Keyword:        query.Keyword,
		Category:       query.Category,
		ProviderType:   query.ProviderType,
		Page:           1,
		PageSize:       100,
	}
	var rows []*domain.Provider
	for {
		page, total, err := repo.List(ctx, baseQuery)
		if err != nil {
			return nil, 0, err
		}
		rows = append(rows, page...)
		if len(rows) >= int(total) || len(page) == 0 {
			break
		}
		baseQuery.Page++
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	metadata, err := listRegistryMetadata(ctx, repo, ids)
	if err != nil {
		return nil, 0, err
	}
	items := make([]Provider, 0, len(rows))
	for _, row := range rows {
		provider, err := providerFromRow(row, metadata[row.ID])
		if err != nil {
			// One corrupt registration row must not take the whole page down:
			// skip it with a warning so the remaining providers stay listable
			// and the operator can find the broken row in the logs.
			slog.WarnContext(ctx, "provider list: skipping invalid registration row",
				"provider_id", row.ID, "organization_id", row.OrganizationID, "error", err)
			continue
		}
		if query.Family != "" && !strings.EqualFold(string(provider.Registration.Family), query.Family) {
			continue
		}
		if query.Kind != "" && !strings.EqualFold(string(provider.Registration.Kind), query.Kind) {
			continue
		}
		if query.Variant != "" && !strings.EqualFold(string(provider.Registration.Variant), query.Variant) {
			continue
		}
		if query.Status != "" && !strings.EqualFold(string(provider.Registration.Status), query.Status) {
			continue
		}
		items = append(items, *provider)
	}
	total := int64(len(items))
	start, end := providerPageBounds(query.Page, query.PageSize, len(items))
	return items[start:end], total, nil
}

// providerPageBounds maps a 1-based page onto the half-open range of an
// in-memory provider list. The page size is clamped before the offset and the
// end are computed so both use the same width; Offset clamps the page before
// multiplying, which is what keeps a hostile page from wrapping negative.
func providerPageBounds(page, pageSize, total int) (start, end int) {
	pageSize = platformpagination.ClampPageSize(pageSize)
	start = platformpagination.Offset(page, pageSize)
	if start > total {
		start = total
	}
	end = total
	if start < total {
		end = min(start+pageSize, total)
	}
	return start, end
}

func (s *UnifiedService) persistSecretStatus(ctx context.Context, tx appuow.UnitOfWork, provider *Provider, version int64, gracePeriod time.Duration) error {
	rotationRepo, ok := tx.Providers().(secretStatusRotationRepository)
	if !ok {
		return apperrors.NewUnimplemented("provider secret version store is unavailable")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return err
	}
	now := s.Clock.Now()
	previousStatus := "rotated"
	previousExpiresAt := now
	if gracePeriod > 0 {
		previousStatus = "grace"
		previousExpiresAt = now.Add(gracePeriod)
	}
	current := domain.ProviderSecretStatus{
		ID:            id,
		ProviderID:    provider.Registration.ID,
		SecretType:    provider.secretType,
		Status:        "active",
		LastRotatedAt: now,
		UpdatedAt:     now,
		Version:       version,
	}
	if err := rotationRepo.RotateSecretStatus(ctx, current, previousStatus, previousExpiresAt); err != nil {
		return err
	}
	if provider.metadata.Secrets == nil {
		provider.metadata.Secrets = map[string]secretMetadata{}
	}
	provider.metadata.Secrets[provider.secretType] = secretMetadata{
		Fingerprint: provider.fingerprint,
		Version:     version,
		KeyVersion:  provider.keyVersion,
	}
	return nil
}

func (s *UnifiedService) nextSecretVersion(ctx context.Context, tx appuow.UnitOfWork, providerID, secretType string) (int64, error) {
	items, err := tx.Providers().ListSecretStatuses(ctx, providerID)
	if err != nil {
		return 0, err
	}
	var maxVersion int64
	for i := range items {
		if items[i].SecretType == secretType && items[i].Version > maxVersion {
			maxVersion = items[i].Version
		}
	}
	return maxVersion + 1, nil
}

func normalizeFamily(value string) domain.Family {
	return domain.Family(strings.ToLower(strings.TrimSpace(value)))
}
