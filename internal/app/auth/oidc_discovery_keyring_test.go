package auth

import (
	"context"
	"errors"
	"testing"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	"github.com/guliangiam/guliangiam/internal/domain/auth/mock"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// INC-BE-01 回归：Discovery 宣告的签名算法与国密通道必须与 keyring 实际
// 发布的密钥族一致，不得无条件宣告 GM 能力。
func TestGetOIDCDiscovery_RSAOnlyRingDoesNotAnnounceGM(t *testing.T) {
	ctrl := gomock.NewController(t)
	jwks := mock.NewMockJWKSProvider(ctrl)
	jwks.EXPECT().GetJWKS().Return(&domainauth.JWKS{Keys: []domainauth.JWK{
		{Kid: "rsa-1", Alg: platformjwks.SigningAlgorithmRS256},
	}}, nil)
	jwks.EXPECT().GetGMJWKS().Return(&domainauth.JWKS{}, nil)

	svc := Service{JWKSManager: jwks}
	resp, err := svc.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{platformjwks.SigningAlgorithmRS256}, resp.IDTokenSigningAlgValuesSupported)
	assert.Empty(t, resp.GMJWKSEndpoint)
	assert.Empty(t, resp.GMAlgValuesSupported)
}

func TestGetOIDCDiscovery_GMOnlyRingDoesNotAnnounceRS256(t *testing.T) {
	ctrl := gomock.NewController(t)
	jwks := mock.NewMockJWKSProvider(ctrl)
	jwks.EXPECT().GetJWKS().Return(&domainauth.JWKS{}, nil)
	jwks.EXPECT().GetGMJWKS().Return(&domainauth.JWKS{Keys: []domainauth.JWK{
		{Kid: "gm-1", Alg: platformjwks.SigningAlgorithmSM2SM3},
	}}, nil)

	svc := Service{JWKSManager: jwks}
	resp, err := svc.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{platformjwks.SigningAlgorithmSM2SM3}, resp.IDTokenSigningAlgValuesSupported)
	assert.Equal(t, "https://auth.example.com/.well-known/jwks-gm.json", resp.GMJWKSEndpoint)
	assert.Equal(t, []string{platformjwks.SigningAlgorithmSM2SM3}, resp.GMAlgValuesSupported)
}

func TestGetOIDCDiscovery_DualFamilyRingAnnouncesBoth(t *testing.T) {
	ctrl := gomock.NewController(t)
	jwks := mock.NewMockJWKSProvider(ctrl)
	jwks.EXPECT().GetJWKS().Return(&domainauth.JWKS{Keys: []domainauth.JWK{
		{Kid: "rsa-1", Alg: platformjwks.SigningAlgorithmRS256},
	}}, nil)
	jwks.EXPECT().GetGMJWKS().Return(&domainauth.JWKS{Keys: []domainauth.JWK{
		{Kid: "gm-1", Alg: platformjwks.SigningAlgorithmSM2SM3},
	}}, nil)

	svc := Service{JWKSManager: jwks}
	resp, err := svc.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	require.NoError(t, err)
	assert.Equal(t,
		[]string{platformjwks.SigningAlgorithmRS256, platformjwks.SigningAlgorithmSM2SM3},
		resp.IDTokenSigningAlgValuesSupported)
	assert.NotEmpty(t, resp.GMJWKSEndpoint)
	assert.NotEmpty(t, resp.GMAlgValuesSupported)
}

func TestGetOIDCDiscovery_KeyRingErrorFailsClosed(t *testing.T) {
	ctrl := gomock.NewController(t)
	jwks := mock.NewMockJWKSProvider(ctrl)
	wantErr := errors.New("no publishable keys")
	jwks.EXPECT().GetJWKS().Return(nil, wantErr)

	svc := Service{JWKSManager: jwks}
	resp, err := svc.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, wantErr)
}

func TestGetOIDCDiscovery_NilManagerKeepsStaticAnnouncement(t *testing.T) {
	svc := Service{}
	resp, err := svc.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	require.NoError(t, err)
	assert.Equal(t, platformjwks.IDTokenSigningAlgorithms, resp.IDTokenSigningAlgValuesSupported)
	assert.NotEmpty(t, resp.GMJWKSEndpoint)
}
