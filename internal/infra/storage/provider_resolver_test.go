package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainproviders "github.com/guliangiam/guliangiam/internal/domain/providers"
	domainresources "github.com/guliangiam/guliangiam/internal/domain/resources"
)

type stubProviderLister struct {
	rows []*domainproviders.Provider
	err  error
}

func (s stubProviderLister) List(context.Context, domainproviders.ListQuery) ([]*domainproviders.Provider, int64, error) {
	return s.rows, int64(len(s.rows)), s.err
}

func TestProviderResolver_S3Row(t *testing.T) {
	resolver := NewProviderResolver(stubProviderLister{rows: []*domainproviders.Provider{{
		ID: "p1", Revision: 1, Enabled: true,
		ProviderType: domainproviders.ProviderAWSS3, Category: domainproviders.CategoryStorage,
		AppId: "bucket", Region: "us-east-1", Admin: "key", Password: "secret",
	}}})
	provider, err := resolver.Resolve(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, provider)
}

func TestProviderResolver_MissingFallsBack(t *testing.T) {
	resolver := NewProviderResolver(stubProviderLister{})
	_, err := resolver.Resolve(context.Background(), "org-1")
	require.ErrorIs(t, err, domainresources.ErrStorageProviderUnconfigured)
}

func TestProviderResolver_DisabledIsUnavailable(t *testing.T) {
	resolver := NewProviderResolver(stubProviderLister{rows: []*domainproviders.Provider{{
		ID: "p1", Name: "s3", Revision: 1, Enabled: false,
		ProviderType: domainproviders.ProviderAWSS3, Category: domainproviders.CategoryStorage,
		AppId: "bucket", Admin: "key", Password: "secret",
	}}})
	_, err := resolver.Resolve(context.Background(), "org-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disabled")
}

func TestProviderResolver_InvalidConfigIsUnavailable(t *testing.T) {
	resolver := NewProviderResolver(stubProviderLister{rows: []*domainproviders.Provider{{
		ID: "p1", Name: "s3", Revision: 1, Enabled: true,
		ProviderType: domainproviders.ProviderAWSS3, Category: domainproviders.CategoryStorage,
	}}})
	_, err := resolver.Resolve(context.Background(), "org-1")
	require.Error(t, err)
	assert.False(t, errors.Is(err, domainresources.ErrStorageProviderUnconfigured))
	assert.Contains(t, err.Error(), "requires bucket")
}

func TestProviderResolver_UnsupportedTypeFallsBack(t *testing.T) {
	resolver := NewProviderResolver(stubProviderLister{rows: []*domainproviders.Provider{{
		ID: "p1", Revision: 1, Enabled: true,
		ProviderType: domainproviders.ProviderLocal, Category: domainproviders.CategoryStorage,
		AppId: "bucket",
	}}})
	_, err := resolver.Resolve(context.Background(), "org-1")
	require.ErrorIs(t, err, domainresources.ErrStorageProviderUnconfigured)
}

func TestProviderResolver_CUCloudDerivesEndpoint(t *testing.T) {
	resolver := NewProviderResolver(stubProviderLister{rows: []*domainproviders.Provider{{
		ID: "p1", Revision: 1, Enabled: true,
		ProviderType: domainproviders.ProviderCUCloud, Category: domainproviders.CategoryStorage,
		AppId: "bucket", Region: "cn-north-1", Admin: "key", Password: "secret",
	}}})
	provider, err := resolver.Resolve(context.Background(), "org-1")
	require.NoError(t, err)
	require.NotNil(t, provider)
}
