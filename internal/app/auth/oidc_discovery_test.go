package auth

import (
	"context"
	"testing"

	appuowmock "github.com/guliangiam/guliangiam/internal/app/uow/mock"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	appmock "github.com/guliangiam/guliangiam/internal/domain/applications/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestGetOIDCDiscovery(t *testing.T) {
	svc := Service{}
	resp, err := svc.GetOIDCDiscovery(context.Background(), "https://auth.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Issuer != "https://auth.example.com" {
		t.Fatalf("got %s", resp.Issuer)
	}
	if resp.AuthorizationEndpoint != "https://auth.example.com/api/v1/auth/authorize" {
		t.Fatalf("got %s", resp.AuthorizationEndpoint)
	}
	if resp.TokenEndpoint != "https://auth.example.com"+TokenEndpointPath {
		t.Fatalf("got %s", resp.TokenEndpoint)
	}
	if resp.JWKSEndpoint != "https://auth.example.com/.well-known/jwks.json" {
		t.Fatalf("got %s", resp.JWKSEndpoint)
	}
	if len(resp.ScopesSupported) != 6 {
		t.Fatalf("got %d", len(resp.ScopesSupported))
	}
	if len(resp.GrantTypesSupported) != 5 {
		t.Fatalf("got %d", len(resp.GrantTypesSupported))
	}
	// client_credentials answers unsupported_grant_type until token subjects
	// become principals, so discovery must not advertise it. refresh_token is
	// implemented and advertised.
	for _, grant := range resp.GrantTypesSupported {
		if grant == "client_credentials" {
			t.Fatal("discovery must not advertise client_credentials")
		}
	}
	if len(resp.ClaimsSupported) != 9 {
		t.Fatalf("got %d", len(resp.ClaimsSupported))
	}
}

func TestGetOIDCDiscovery_TrailingSlash(t *testing.T) {
	svc := Service{}
	resp, err := svc.GetOIDCDiscovery(context.Background(), "https://auth.example.com/")
	if err != nil {
		t.Fatal(err)
	}
	if resp.Issuer != "https://auth.example.com" {
		t.Fatalf("got %s", resp.Issuer)
	}
}

func TestGetOIDCDiscoveryByApplication_ResolvesClientID(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := appmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	repo.EXPECT().GetByClientID(gomock.Any(), "client-1").Return(&applicationdomain.Application{
		ID: "app-id-1", Slug: "app1", ClientID: "client-1", OrganizationID: "org-1",
	}, nil)

	svc := Service{UOW: uow}
	resp, err := svc.GetOIDCDiscoveryByApplication(context.Background(), "https://auth.example.com", "client-1")
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com/.well-known/app1/jwks.json", resp.JWKSEndpoint,
		"the document must name the canonical slug regardless of the identifier used to ask")
}

func TestGetOIDCDiscoveryByApplication_ResolvesSlug(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := appmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	repo.EXPECT().GetByClientID(gomock.Any(), "app1").Return(nil, apperrors.NewNotFound("application not found"))
	repo.EXPECT().List(gomock.Any(), applicationdomain.ListQuery{
		Page: 1, PageSize: 100, Keyword: "app1",
	}).Return([]*applicationdomain.Application{{
		ID: "app-id-1", Slug: "app1", ClientID: "client-1", OrganizationID: "org-1",
	}}, int64(1), nil)

	svc := Service{UOW: uow}
	resp, err := svc.GetOIDCDiscoveryByApplication(context.Background(), "https://auth.example.com", "app1")
	require.NoError(t, err)
	assert.Equal(t, "https://auth.example.com/.well-known/app1/jwks.json", resp.JWKSEndpoint)
}

func TestGetOIDCDiscoveryByApplication_NotFoundDoesNotFallBackToGlobal(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := appmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	repo.EXPECT().GetByClientID(gomock.Any(), "missing").Return(nil, apperrors.NewNotFound("application not found"))
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*applicationdomain.Application{}, int64(0), nil)

	svc := Service{UOW: uow}
	resp, err := svc.GetOIDCDiscoveryByApplication(context.Background(), "https://auth.example.com", "missing")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, apperrors.CodeNotFound, err.(*apperrors.Error).Code)
}

func TestResolveOIDCApplication_AmbiguousSlugIsNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	uow := appuowmock.NewMockAuthUoW(ctrl)
	repo := appmock.NewMockRepository(ctrl)
	uow.EXPECT().Applications().Return(repo).AnyTimes()
	repo.EXPECT().GetByClientID(gomock.Any(), "app1").Return(nil, apperrors.NewNotFound("application not found"))
	repo.EXPECT().List(gomock.Any(), gomock.Any()).Return([]*applicationdomain.Application{
		{Slug: "app1", OrganizationID: "org-1"},
		{Slug: "app1", OrganizationID: "org-2"},
	}, int64(2), nil)

	svc := Service{UOW: uow}
	_, err := svc.ResolveOIDCApplication(context.Background(), "app1")
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeNotFound, err.(*apperrors.Error).Code)
}
