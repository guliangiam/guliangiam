package providers

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BE-B-15: one registration row whose metadata passes decoding but fails
// Registration.Validate (identity family with a payment-only variant) must be
// skipped with a warning instead of failing the whole list.
func TestUnifiedList_SkipsInvalidRegistrationRow(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "bad-1", OrganizationID: "org-1", Name: "broken", DisplayName: "Broken", Category: "OAuth", ProviderType: "github", Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1},
			{ID: "good-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Category: "OAuth", ProviderType: "github", Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1},
		},
	}

	// Decodes cleanly (family/kind/status all valid) but Validate rejects a
	// non-empty variant on an identity provider.
	badConfig, err := registryMetadata{
		Family: string(domain.FamilyIdentity), Kind: "github", Variant: string(domain.VariantProduction),
		Status: string(domain.StatusActive), DetailID: "bad-1",
	}.config("bad-1", now)
	require.NoError(t, err)

	goodRegistration := domain.Registration{ID: "good-1", OrganizationID: "org-1", Name: "github", DisplayName: "GitHub", Family: domain.FamilyIdentity, Kind: "github", Status: domain.StatusActive, CreatedAt: now, UpdatedAt: now, Revision: 1}
	goodConfig, err := metadataFromRegistration(goodRegistration, goodRegistration.ID).config(goodRegistration.ID, now)
	require.NoError(t, err)
	repo.configs = []*domain.ProviderConfig{badConfig, goodConfig}

	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{providerRepo: repo}}

	items, total, err := svc.ListProviders(context.Background(), ListProvidersQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "good-1", items[0].ID)
}

// A row without registry metadata is a legacy identity provider and must stay
// listable next to a corrupt row.
func TestUnifiedList_LegacyRowSurvivesInvalidSibling(t *testing.T) {
	now := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	repo := &mockProviderRepo{
		providers: []*domain.Provider{
			{ID: "legacy-1", OrganizationID: "org-1", Name: "legacy", DisplayName: "Legacy", Category: "OAuth", ProviderType: "github", Enabled: true, CreatedAt: now, UpdatedAt: now, Revision: 1},
			{ID: "bad-1", OrganizationID: "org-1", Name: "broken", DisplayName: "Broken", Category: "OAuth", ProviderType: "github", CreatedAt: now, UpdatedAt: now, Revision: 1},
		},
	}
	badConfig, err := registryMetadata{
		Family: string(domain.FamilyIdentity), Kind: "github", Variant: string(domain.VariantProduction),
		Status: string(domain.StatusActive), DetailID: "bad-1",
	}.config("bad-1", now)
	require.NoError(t, err)
	repo.configs = []*domain.ProviderConfig{badConfig}

	svc := Service{Clock: fakeClock{fixedTime: now}, UOW: testUnitOfWork{providerRepo: repo}}

	items, total, err := svc.ListProviders(context.Background(), ListProvidersQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, "legacy-1", items[0].ID)
	assert.True(t, items[0].Enabled)
}
