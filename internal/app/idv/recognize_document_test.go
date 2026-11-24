package idv

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ocrResolverStub returns a per-provider-name result so the OCR fallback and
// error-surfacing logic can be exercised without a database.
type ocrResolverStub struct {
	byName map[string]error
	creds  domain.ProviderCredentials
}

func (r ocrResolverStub) ResolveIDVProvider(_ context.Context, _ string, name string) (*domain.ProviderCredentials, error) {
	if err := r.byName[name]; err != nil {
		return nil, err
	}
	cp := r.creds
	return &cp, nil
}

func incompleteErr() error {
	return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "aliyun_idv: provider credentials are incomplete (missing password)", map[string]any{"field": "password"})
}

func noProviderErr() error {
	return apperrors.NewInvalidArgument("no enabled identity-verification provider is configured for this organization")
}

func TestResolveOCRProviderCredentials_SurfacesIncompleteInsteadOfUnavailable(t *testing.T) {
	s := Service{Providers: ocrResolverStub{byName: map[string]error{
		"aliyun_idv":  incompleteErr(),
		"tencent_idv": noProviderErr(),
	}}}
	_, err := s.resolveOCRProviderCredentials(context.Background(), "org-1")
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "password", appErr.Meta["field"])
}

func TestResolveOCRProviderCredentials_FallsBackToCompleteTencent(t *testing.T) {
	complete := domain.ProviderCredentials{Provider: "tencent_idv", APIKey: "ak", APISecret: "sk"}
	s := Service{Providers: ocrResolverStub{
		byName: map[string]error{"aliyun_idv": incompleteErr()},
		creds:  complete,
	}}
	creds, err := s.resolveOCRProviderCredentials(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "tencent_idv", creds.Provider)
}

func TestResolveOCRProviderCredentials_UnconfiguredIsUnavailable(t *testing.T) {
	s := Service{Providers: ocrResolverStub{byName: map[string]error{
		"aliyun_idv":  noProviderErr(),
		"tencent_idv": noProviderErr(),
	}}}
	_, err := s.resolveOCRProviderCredentials(context.Background(), "org-1")
	require.Error(t, err)
	appErr, ok := err.(*apperrors.Error)
	require.True(t, ok)
	assert.Equal(t, apperrors.CodeUnavailable, appErr.Code)
}
