package aigateway

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

// unscopedModelsRepo deliberately implements only ModelsRepo. The management
// service must refuse to use its unscoped methods.
type unscopedModelsRepo struct {
	getByIDCalls int
	updateCalls  int
	deleteCalls  int
}

func (r *unscopedModelsRepo) GetByID(context.Context, string) (*domain.LlmModel, error) {
	r.getByIDCalls++
	return nil, apperrors.NewNotFound("model not found")
}
func (r *unscopedModelsRepo) List(context.Context, string) ([]*domain.LlmModel, error) {
	return nil, nil
}
func (r *unscopedModelsRepo) ListByProvider(context.Context, string) ([]*domain.LlmModel, error) {
	return nil, nil
}
func (r *unscopedModelsRepo) Create(context.Context, *domain.LlmModel) error { return nil }
func (r *unscopedModelsRepo) Update(context.Context, *domain.LlmModel) error {
	r.updateCalls++
	return nil
}
func (r *unscopedModelsRepo) Delete(context.Context, string) error {
	r.deleteCalls++
	return nil
}

type unscopedAliasesRepo struct {
	emptyAliasesRepo
	getByIDCalls int
	updateCalls  int
	deleteCalls  int
}

func (r *unscopedAliasesRepo) GetByID(context.Context, string) (*domain.LlmModelAlias, error) {
	r.getByIDCalls++
	return nil, apperrors.NewNotFound("alias not found")
}
func (r *unscopedAliasesRepo) Update(context.Context, *domain.LlmModelAlias) error {
	r.updateCalls++
	return nil
}
func (r *unscopedAliasesRepo) Delete(context.Context, string) error {
	r.deleteCalls++
	return nil
}

// unscopedQuotasRepo deliberately implements only QuotaPoliciesRepo.
type unscopedQuotasRepo struct {
	getByIDCalls int
	updateCalls  int
	deleteCalls  int
}

func (r *unscopedQuotasRepo) GetByID(context.Context, string) (*domain.LlmQuotaPolicy, error) {
	r.getByIDCalls++
	return nil, apperrors.NewNotFound("quota not found")
}
func (r *unscopedQuotasRepo) List(context.Context, string) ([]*domain.LlmQuotaPolicy, error) {
	return nil, nil
}
func (r *unscopedQuotasRepo) Create(context.Context, *domain.LlmQuotaPolicy) error { return nil }
func (r *unscopedQuotasRepo) Update(context.Context, *domain.LlmQuotaPolicy) error {
	r.updateCalls++
	return nil
}
func (r *unscopedQuotasRepo) Delete(context.Context, string) error {
	r.deleteCalls++
	return nil
}

// unscopedPricesRepo deliberately implements only PriceVersionsRepo.
type unscopedPricesRepo struct {
	getByIDCalls int
	updateCalls  int
	deleteCalls  int
}

func (r *unscopedPricesRepo) GetByID(context.Context, string) (*domain.LlmPriceVersion, error) {
	r.getByIDCalls++
	return nil, apperrors.NewNotFound("price version not found")
}
func (r *unscopedPricesRepo) GetByModel(context.Context, string) (*domain.LlmPriceVersion, error) {
	return nil, nil
}
func (r *unscopedPricesRepo) List(context.Context, string) ([]*domain.LlmPriceVersion, error) {
	return nil, nil
}
func (r *unscopedPricesRepo) ListByModel(context.Context, string) ([]*domain.LlmPriceVersion, error) {
	return nil, nil
}
func (r *unscopedPricesRepo) Create(context.Context, *domain.LlmPriceVersion) error { return nil }
func (r *unscopedPricesRepo) Update(context.Context, *domain.LlmPriceVersion) error {
	r.updateCalls++
	return nil
}
func (r *unscopedPricesRepo) Delete(context.Context, string) error {
	r.deleteCalls++
	return nil
}

// TestAdminService_ManagementByIDFailsClosedWithoutScopedRepos pins that a
// repository missing the tenant-scoped contract yields Internal for all six
// by-ID operations. There is no fallback to GetByID/Update/Delete.
func TestAdminService_ManagementByIDFailsClosedWithoutScopedRepos(t *testing.T) {
	models := &unscopedModelsRepo{}
	aliases := &unscopedAliasesRepo{}
	svc := &AdminService{Models: models, Aliases: aliases}
	ctx := context.Background()

	_, err := svc.GetModel(ctx, "org-1", "m-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	_, err = svc.UpdateModel(ctx, "org-1", "m-1", ModelUpdate{})
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	err = svc.DeleteModel(ctx, "org-1", "m-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)

	_, err = svc.GetAlias(ctx, "org-1", "a-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	_, err = svc.UpdateAlias(ctx, "org-1", "a-1", AliasUpdate{})
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	err = svc.DeleteAlias(ctx, "org-1", "a-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)

	require.Zero(t, models.getByIDCalls, "model reads must not fall back to unscoped GetByID")
	require.Zero(t, models.updateCalls, "model writes must not fall back to unscoped Update")
	require.Zero(t, models.deleteCalls, "model deletes must not fall back to unscoped Delete")
	require.Zero(t, aliases.getByIDCalls, "alias reads must not fall back to unscoped GetByID")
	require.Zero(t, aliases.updateCalls, "alias writes must not fall back to unscoped Update")
	require.Zero(t, aliases.deleteCalls, "alias deletes must not fall back to unscoped Delete")
}

// TestAdminService_QuotaPriceFailsClosedWithoutScopedRepos pins the same
// fail-closed contract for the quota/price by-ID entries.
func TestAdminService_QuotaPriceFailsClosedWithoutScopedRepos(t *testing.T) {
	quotas := &unscopedQuotasRepo{}
	prices := &unscopedPricesRepo{}
	svc := &AdminService{Quotas: quotas, Prices: prices}
	ctx := context.Background()

	_, err := svc.GetQuota(ctx, "org-1", "q-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	_, err = svc.UpdateQuota(ctx, "org-1", "q-1", QuotaUpdate{})
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	err = svc.DeleteQuota(ctx, "org-1", "q-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)

	_, err = svc.GetPrice(ctx, "org-1", "pv-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	_, err = svc.UpdatePrice(ctx, "org-1", "pv-1", PriceVersionUpdate{})
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)
	err = svc.DeletePrice(ctx, "org-1", "pv-1")
	require.Equal(t, apperrors.CodeInternal, apperrors.FromError(err).Code)

	require.Zero(t, quotas.getByIDCalls, "quota reads must not fall back to unscoped GetByID")
	require.Zero(t, quotas.updateCalls, "quota writes must not fall back to unscoped Update")
	require.Zero(t, quotas.deleteCalls, "quota deletes must not fall back to unscoped Delete")
	require.Zero(t, prices.getByIDCalls, "price reads must not fall back to unscoped GetByID")
	require.Zero(t, prices.updateCalls, "price writes must not fall back to unscoped Update")
	require.Zero(t, prices.deleteCalls, "price deletes must not fall back to unscoped Delete")
}
