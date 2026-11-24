package wellknown

import (
	"context"

	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
)

// OIDCDiscoveryAdapter adapts the auth.Service to the wellknown.OIDCDiscoveryFetcher interface.
type OIDCDiscoveryAdapter struct {
	Service *appauth.Service
}

// GetOIDCDiscovery fetches the global OIDC Discovery document.
func (a OIDCDiscoveryAdapter) GetOIDCDiscovery(ctx context.Context, issuerURL string) (*appauth.OIDCDiscoveryResponse, error) {
	return a.Service.GetOIDCDiscovery(ctx, issuerURL)
}

// GetOIDCDiscoveryByApplication fetches a per-application OIDC Discovery document.
func (a OIDCDiscoveryAdapter) GetOIDCDiscoveryByApplication(ctx context.Context, issuerURL string, applicationID string) (*appauth.OIDCDiscoveryResponse, error) {
	return a.Service.GetOIDCDiscoveryByApplication(ctx, issuerURL, applicationID)
}

// ResolveOIDCApplication resolves the slug/client_id a per-application
// well-known request addressed.
func (a OIDCDiscoveryAdapter) ResolveOIDCApplication(ctx context.Context, reference string) (*appauth.OIDCApplication, error) {
	return a.Service.ResolveOIDCApplication(ctx, reference)
}

// UserInfoAdapter adapts the auth.Service to the wellknown.UserInfoFetcher interface.
type UserInfoAdapter struct {
	Service *appauth.Service
}

// GetUserInfo fetches OIDC UserInfo for the given access token.
func (a UserInfoAdapter) GetUserInfo(ctx context.Context, accessToken string) (*appauth.UserInfoResponse, error) {
	return a.Service.GetUserInfo(ctx, accessToken)
}

// SAMLMetadataAdapter adapts the auth.Service to the wellknown.SAMLMetadataFetcher interface.
type SAMLMetadataAdapter struct {
	Service *appauth.Service
}

// GetSAMLMetadata renders the SAML IdP metadata document for an application.
func (a SAMLMetadataAdapter) GetSAMLMetadata(ctx context.Context, organizationSlug string, applicationID string) ([]byte, error) {
	return a.Service.GetSAMLMetadata(ctx, organizationSlug, applicationID)
}
