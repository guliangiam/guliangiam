package idp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	"github.com/markbates/goth"
	// Import commonly used Goth providers.
	// Providers already implemented natively (github, google, etc.) are NOT imported here
	// to avoid conflicts; those use the native implementations in the factory.
	"github.com/markbates/goth/providers/battlenet"
	"github.com/markbates/goth/providers/box"
	"github.com/markbates/goth/providers/cloudfoundry"
	"github.com/markbates/goth/providers/dailymotion"
	"github.com/markbates/goth/providers/deezer"
	"github.com/markbates/goth/providers/digitalocean"
	"github.com/markbates/goth/providers/dropbox"
	"github.com/markbates/goth/providers/eveonline"
	"github.com/markbates/goth/providers/fitbit"
	"github.com/markbates/goth/providers/gitea"
	"github.com/markbates/goth/providers/heroku"
	"github.com/markbates/goth/providers/influxcloud"
	"github.com/markbates/goth/providers/instagram"
	"github.com/markbates/goth/providers/intercom"
	"github.com/markbates/goth/providers/kakao"
	"github.com/markbates/goth/providers/lastfm"
	"github.com/markbates/goth/providers/mailru"
	"github.com/markbates/goth/providers/meetup"
	"github.com/markbates/goth/providers/microsoftonline"
	"github.com/markbates/goth/providers/naver"
	"github.com/markbates/goth/providers/nextcloud"
	"github.com/markbates/goth/providers/onedrive"
	"github.com/markbates/goth/providers/oura"
	"github.com/markbates/goth/providers/patreon"
	"github.com/markbates/goth/providers/paypal"
	"github.com/markbates/goth/providers/salesforce"
	"github.com/markbates/goth/providers/shopify"
	"github.com/markbates/goth/providers/soundcloud"
	"github.com/markbates/goth/providers/spotify"
	"github.com/markbates/goth/providers/strava"
	"github.com/markbates/goth/providers/stripe"
	"github.com/markbates/goth/providers/tiktok"
	"github.com/markbates/goth/providers/tumblr"
	"github.com/markbates/goth/providers/twitch"
	"github.com/markbates/goth/providers/typetalk"
	"github.com/markbates/goth/providers/uber"
	"github.com/markbates/goth/providers/vk"
	"github.com/markbates/goth/providers/wepay"
	"github.com/markbates/goth/providers/xero"
	"github.com/markbates/goth/providers/yahoo"
	"github.com/markbates/goth/providers/yammer"
	"github.com/markbates/goth/providers/yandex"
	"github.com/markbates/goth/providers/zoom"
)

// GothProvider wraps a goth.Provider to implement the idp.Provider interface.
// This enables 50+ OAuth2 providers via the Goth framework.
type GothProvider struct {
	gothProv goth.Provider
	session  goth.Session
}

// NewGothProvider creates a new Goth-backed IdP provider.
func NewGothProvider(providerType string, config *OAuthConfig) (*GothProvider, error) {
	clientID := config.ClientID
	clientSecret := config.ClientSecret
	redirectURL := config.RedirectURI
	hostURL := config.Host

	provider, session, err := createGothProvider(providerType, clientID, clientSecret, redirectURL, hostURL)
	if err != nil {
		return nil, err
	}

	return &GothProvider{
		gothProv: provider,
		session:  session,
	}, nil
}

// createGothProvider instantiates a goth.Provider and its session based on the type string.
func createGothProvider(providerType, clientID, clientSecret, redirectURL, hostURL string) (goth.Provider, goth.Session, error) {
	// normalizeGothType lowercases and strips separators, matching the case
	// labels below.
	normalType := normalizeGothType(providerType)

	// normalizeGothType lowercases and strips separators, so every case label
	// below must use the same normalized form.
	switch normalType {
	case "battlenet":
		return battlenet.New(clientID, clientSecret, redirectURL), &battlenet.Session{}, nil
	case "box":
		return box.New(clientID, clientSecret, redirectURL), &box.Session{}, nil
	case "cloudfoundry":
		return cloudfoundry.New("", clientID, clientSecret, redirectURL), &cloudfoundry.Session{}, nil
	case "dailymotion":
		return dailymotion.New(clientID, clientSecret, redirectURL), &dailymotion.Session{}, nil
	case "deezer":
		return deezer.New(clientID, clientSecret, redirectURL), &deezer.Session{}, nil
	case "digitalocean":
		return digitalocean.New(clientID, clientSecret, redirectURL), &digitalocean.Session{}, nil
	case "dropbox":
		return dropbox.New(clientID, clientSecret, redirectURL), &dropbox.Session{}, nil
	case "eveonline":
		return eveonline.New(clientID, clientSecret, redirectURL), &eveonline.Session{}, nil
	case "fitbit":
		return fitbit.New(clientID, clientSecret, redirectURL), &fitbit.Session{}, nil
	case "gitea":
		if hostURL != "" {
			return gitea.NewCustomisedURL(clientID, clientSecret, redirectURL, hostURL+"/login/oauth/authorize", hostURL+"/login/oauth/access_token", hostURL+"/api/v1/user"), &gitea.Session{}, nil
		}
		return gitea.New(clientID, clientSecret, redirectURL), &gitea.Session{}, nil
	case "heroku":
		return heroku.New(clientID, clientSecret, redirectURL), &heroku.Session{}, nil
	case "influxcloud":
		return influxcloud.New(clientID, clientSecret, redirectURL), &influxcloud.Session{}, nil
	case "instagram":
		return instagram.New(clientID, clientSecret, redirectURL), &instagram.Session{}, nil
	case "intercom":
		return intercom.New(clientID, clientSecret, redirectURL), &intercom.Session{}, nil
	case "kakao":
		return kakao.New(clientID, clientSecret, redirectURL), &kakao.Session{}, nil
	case "lastfm":
		return lastfm.New(clientID, clientSecret, redirectURL), &lastfm.Session{}, nil
	case "mailru":
		return mailru.New(clientID, clientSecret, redirectURL), &mailru.Session{}, nil
	case "meetup":
		return meetup.New(clientID, clientSecret, redirectURL), &meetup.Session{}, nil
	case "microsoftonline":
		return microsoftonline.New(clientID, clientSecret, redirectURL), &microsoftonline.Session{}, nil
	case "naver":
		return naver.New(clientID, clientSecret, redirectURL), &naver.Session{}, nil
	case "nextcloud":
		if hostURL != "" {
			return nextcloud.NewCustomisedDNS(clientID, clientSecret, redirectURL, hostURL), &nextcloud.Session{}, nil
		}
		return nextcloud.New(clientID, clientSecret, redirectURL), &nextcloud.Session{}, nil
	case "onedrive":
		return onedrive.New(clientID, clientSecret, redirectURL), &onedrive.Session{}, nil
	case "oura":
		return oura.New(clientID, clientSecret, redirectURL), &oura.Session{}, nil
	case "patreon":
		return patreon.New(clientID, clientSecret, redirectURL), &patreon.Session{}, nil
	case "paypal":
		return paypal.New(clientID, clientSecret, redirectURL), &paypal.Session{}, nil
	case "salesforce":
		return salesforce.New(clientID, clientSecret, redirectURL), &salesforce.Session{}, nil
	case "shopify":
		return shopify.New(clientID, clientSecret, redirectURL), &shopify.Session{}, nil
	case "soundcloud":
		return soundcloud.New(clientID, clientSecret, redirectURL), &soundcloud.Session{}, nil
	case "spotify":
		return spotify.New(clientID, clientSecret, redirectURL), &spotify.Session{}, nil
	case "strava":
		return strava.New(clientID, clientSecret, redirectURL), &strava.Session{}, nil
	case "stripe":
		return stripe.New(clientID, clientSecret, redirectURL), &stripe.Session{}, nil
	case "tiktok":
		return tiktok.New(clientID, clientSecret, redirectURL), &tiktok.Session{}, nil
	case "tumblr":
		return tumblr.New(clientID, clientSecret, redirectURL), &tumblr.Session{}, nil
	case "twitch":
		return twitch.New(clientID, clientSecret, redirectURL), &twitch.Session{}, nil
	case "typetalk":
		return typetalk.New(clientID, clientSecret, redirectURL), &typetalk.Session{}, nil
	case "uber":
		return uber.New(clientID, clientSecret, redirectURL), &uber.Session{}, nil
	case "vk":
		return vk.New(clientID, clientSecret, redirectURL), &vk.Session{}, nil
	case "wepay":
		return wepay.New(clientID, clientSecret, redirectURL), &wepay.Session{}, nil
	case "xero":
		return xero.New(clientID, clientSecret, redirectURL), &xero.Session{}, nil
	case "yahoo":
		return yahoo.New(clientID, clientSecret, redirectURL), &yahoo.Session{}, nil
	case "yammer":
		return yammer.New(clientID, clientSecret, redirectURL), &yammer.Session{}, nil
	case "yandex":
		return yandex.New(clientID, clientSecret, redirectURL), &yandex.Session{}, nil
	case "zoom":
		return zoom.New(clientID, clientSecret, redirectURL), &zoom.Session{}, nil
	default:
		return nil, nil, apperrors.NewInvalidArgumentf("unsupported Goth provider type: %s", providerType)
	}
}

// GetRedirectURL returns the authorization URL for the Goth provider.
func (p *GothProvider) GetRedirectURL(state string) (string, error) {
	sess, err := p.gothProv.BeginAuth(state)
	if err != nil {
		return "", apperrors.WrapInternalf(err, "begin goth auth")
	}
	p.session = sess

	authURL, err := sess.GetAuthURL()
	if err != nil {
		return "", apperrors.WrapInternalf(err, "get goth auth URL")
	}
	return authURL, nil
}

// ExchangeCode exchanges the authorization code for an OAuth token via the Goth session.
func (p *GothProvider) ExchangeCode(ctx context.Context, code string) (*OAuthToken, error) {
	// Goth uses url.Values for the authorize step
	value := url.Values{}
	value.Add("code", code)

	accessToken, err := p.session.Authorize(p.gothProv, value)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "goth authorize")
	}

	// Try to get expiry from session via reflection
	var expiresIn int64
	valueOfExpire := reflect.ValueOf(p.session).Elem().FieldByName("ExpiresAt")
	if valueOfExpire.IsValid() {
		if expireAt, ok := valueOfExpire.Interface().(time.Time); ok && !expireAt.IsZero() {
			expiresIn = int64(time.Until(expireAt).Seconds())
			if expiresIn < 0 {
				expiresIn = 0
			}
		}
	}

	return &OAuthToken{
		AccessToken: accessToken,
		ExpiresIn:   expiresIn,
	}, nil
}

// GetUserInfo retrieves user information from the Goth provider.
func (p *GothProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*UserInfo, error) {
	gothUser, err := p.gothProv.FetchUser(p.session)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "goth fetch user")
	}

	info := &UserInfo{
		ID:          gothUser.UserID,
		Username:    gothUser.Name,
		DisplayName: gothUser.NickName,
		Email:       gothUser.Email,
		AvatarURL:   gothUser.AvatarURL,
	}

	// Fallback for missing username
	if info.Username == "" {
		if gothUser.FirstName != "" && gothUser.LastName != "" {
			info.Username = fmt.Sprintf("%s %s", gothUser.FirstName, gothUser.LastName)
		} else {
			info.Username = gothUser.NickName
		}
	}

	if info.DisplayName == "" {
		if gothUser.FirstName != "" && gothUser.LastName != "" {
			info.DisplayName = fmt.Sprintf("%s %s", gothUser.FirstName, gothUser.LastName)
		} else {
			info.DisplayName = info.Username
		}
	}

	return info, nil
}

// SupportedGothTypes returns all Goth provider types that can be used.
//
// 2026-11-14 OPEN-API-75 P4 裁定：Kakao/Naver/PayPal/Zoom/Yandex/Dropbox 等
// 增量候选以 Goth 目录覆盖（kind=goth + provider_type 路由），不另立 dedicated
// kind；authorize 端点由 TestGothCatalog_P4IncrementalProviders 按官方文档钉住。
func SupportedGothTypes() []string {
	return []string{
		"battlenet", "box", "cloudfoundry", "dailymotion", "deezer",
		"digitalocean", "dropbox", "eveonline", "fitbit", "gitea",
		"heroku", "influxcloud", "instagram", "intercom", "kakao",
		"lastfm", "mailru", "meetup", "microsoftonline", "naver", "nextcloud",
		"onedrive", "oura", "patreon", "paypal", "salesforce", "shopify",
		"soundcloud", "spotify", "strava", "stripe", "tiktok",
		"tumblr", "twitch", "typetalk", "uber", "vk",
		"wepay", "xero", "yahoo", "yammer", "yandex", "zoom",
	}
}

// IsGothProvider checks if a provider type is supported by the Goth framework.
func IsGothProvider(providerType string) bool {
	normalType := normalizeGothType(providerType)
	for _, t := range SupportedGothTypes() {
		if normalizeGothType(t) == normalType {
			return true
		}
	}
	return false
}

// normalizeGothType normalizes a provider type string for comparison.
func normalizeGothType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

// SetHTTPClient sets a custom HTTP client on the Goth provider via reflection.
func (p *GothProvider) SetHTTPClient(client *http.Client) {
	idpClient := reflect.ValueOf(p.gothProv).Elem().FieldByName("HTTPClient")
	if idpClient.IsValid() {
		idpClient.Set(reflect.ValueOf(client))
	}
}

// ensure GothProvider implements Provider interface at compile time
var _ Provider = (*GothProvider)(nil)
