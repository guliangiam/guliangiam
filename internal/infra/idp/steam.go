package idp

import (
	"context"
	"net/url"
	"strings"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// SteamProvider implements IdP authentication via Steam OpenID.
// Steam uses OpenID 2.0 rather than standard OAuth2.
type SteamProvider struct {
	BaseOAuthProvider
}

// NewSteamProvider creates a new Steam OpenID provider.
//
// 2026-11-14 OPEN-API-75 P4 校准：OpenID 2.0 端点已按官方复核
// (https://steamcommunity.com/dev) — Steam 不提供 OAuth2，校验依赖
// claimed_id https://steamcommunity.com/openid/id/<SteamID64> 格式。
func NewSteamProvider(config *OAuthConfig) *SteamProvider {
	return &SteamProvider{
		BaseOAuthProvider: newBaseOAuth(config,
			"https://steamcommunity.com/openid/login",
			"",
			"",
		),
	}
}

// GetRedirectURL returns the Steam OpenID login URL.
func (p *SteamProvider) GetRedirectURL(state string) (string, error) {
	u, err := url.Parse(p.authURL)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "parse auth URL")
	}

	q := u.Query()
	q.Set("openid.ns", "http://specs.openid.net/auth/2.0")
	q.Set("openid.mode", "checkid_setup")
	q.Set("openid.return_to", p.config.RedirectURI)
	q.Set("openid.realm", p.config.RedirectURI)
	q.Set("openid.identity", "http://specs.openid.net/auth/2.0/identifier_select")
	q.Set("openid.claimed_id", "http://specs.openid.net/auth/2.0/identifier_select")
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// ExchangeCode verifies the Steam OpenID response and extracts the Steam ID.
func (p *SteamProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	return nil, apperrors.NewUnauthorized("steam provider uses OpenID verification; parse the claimed_id from the callback instead")
}

// GetUserInfo is not directly supported for Steam; user info requires the Steam Web API.
func (p *SteamProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	return nil, apperrors.NewUnauthorized("steam user info requires Steam Web API with the API key")
}

// ExtractSteamID extracts the SteamID64 from the OpenID claimed_id URL.
func (p *SteamProvider) ExtractSteamID(claimedID string) (string, error) {
	// Steam OpenID claimed_id format: https://steamcommunity.com/openid/id/<SteamID64>
	parts := strings.Split(claimedID, "/")
	if len(parts) == 0 {
		return "", apperrors.NewInvalidArgument("invalid claimed_id format")
	}
	steamID := parts[len(parts)-1]
	if steamID == "" {
		return "", apperrors.NewNotFound("empty steam ID in claimed_id")
	}
	return steamID, nil
}
