package auth

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/radiusauth"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
)

const sessionTTL = 30 * 24 * time.Hour
const shortSessionTTL = 24 * time.Hour
const mfaChallengeTTL = 10 * time.Minute

const (
	LoginStatusSuccess     = "success"
	LoginStatusMfaRequired = "mfa_required"
	LoginStatusMfaSetup    = "required_mfa_setup"
	mfaTypeTotp            = "totp"
	mfaTypeEmail           = "email"
	mfaTypeSms             = "sms"
	mfaTypeRadius          = "radius"
	mfaTypePush            = "push"
	mfaTypeRecovery        = "recovery"
	defaultRadiusPort      = 1812
	radiusRequestTimeout   = 10 * time.Second
	tokenPrefixMaxLength   = 18
)

type radiusProviderConfig struct {
	host                        string
	port                        int
	sharedSecret                string
	requireMessageAuthenticator bool
}

type pushProviderConfig struct {
	providerType  string
	clientID      string
	clientSecret  string
	clientID2     string
	clientSecret2 string
	appID         string
	method        string
	title         string
	metadata      string
	regionID      string
}

var resolveRadiusProvider = func(ctx context.Context, tx appuow.UnitOfWork, providerID string) (radiusProviderConfig, error) {
	if tx == nil {
		return radiusProviderConfig{}, apperrors.NewUnauthorized("radius mfa provider is not configured for the user")
	}

	provider, err := tx.Providers().GetByID(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return radiusProviderConfig{}, err
	}
	config, err := tx.Providers().GetConfig(ctx, provider.ID)
	if err != nil {
		return radiusProviderConfig{}, err
	}

	host := firstNonEmpty(
		stringConfigValue(config.Config, "host"),
		hostFromProviderURL(provider.ProviderURL),
	)
	if host == "" {
		return radiusProviderConfig{}, apperrors.NewUnauthorized("radius mfa provider host is not configured")
	}

	secret := firstNonEmpty(
		strings.TrimSpace(provider.ClientSecret),
		stringConfigValue(config.Config, "sharedSecret"),
	)
	if secret == "" {
		return radiusProviderConfig{}, apperrors.NewUnauthorized("radius mfa provider shared secret is not configured")
	}

	port := intConfigValue(config.Config, "port")
	if port <= 0 {
		port = portFromProviderURL(provider.ProviderURL)
	}
	if port <= 0 {
		port = defaultRadiusPort
	}

	return radiusProviderConfig{
		host:                        host,
		port:                        port,
		sharedSecret:                secret,
		requireMessageAuthenticator: provider.RequireMessageAuthenticator,
	}, nil
}

var verifyRadiusCredentials = func(ctx context.Context, config radiusProviderConfig, username string, password string) error {
	packet := radius.New(radius.CodeAccessRequest, []byte(config.sharedSecret))
	if err := rfc2865.UserName_SetString(packet, strings.TrimSpace(username)); err != nil {
		return apperrors.NewInternal("set radius username")
	}
	if err := rfc2865.UserPassword_SetString(packet, strings.TrimSpace(password)); err != nil {
		return apperrors.NewInternal("set radius password")
	}
	if config.requireMessageAuthenticator {
		if err := radiusauth.Sign(packet); err != nil {
			return apperrors.NewInternal("sign radius request")
		}
	}

	requestCtx, cancel := context.WithTimeout(ctx, radiusRequestTimeout)
	defer cancel()

	response, err := radius.Exchange(requestCtx, packet, net.JoinHostPort(config.host, strconv.Itoa(config.port)))
	if err != nil {
		return apperrors.NewInternal("radius authentication failed")
	}
	if config.requireMessageAuthenticator {
		if err := radiusauth.VerifyResponse(response, packet); err != nil {
			return apperrors.NewUnauthorized("radius response failed Message-Authenticator verification")
		}
	}
	if response.Code != radius.CodeAccessAccept {
		return apperrors.NewUnauthorized("radius authentication rejected")
	}

	return nil
}

var resolvePushProvider = func(ctx context.Context, tx appuow.UnitOfWork, providerID string) (pushProviderConfig, error) {
	if tx == nil {
		return pushProviderConfig{}, apperrors.NewUnauthorized("push mfa provider is not configured for the user")
	}

	provider, err := tx.Providers().GetByID(ctx, strings.TrimSpace(providerID))
	if err != nil {
		return pushProviderConfig{}, err
	}
	config, err := tx.Providers().GetConfig(ctx, provider.ID)
	if err != nil {
		return pushProviderConfig{}, err
	}

	providerType := firstNonEmpty(string(provider.ProviderType), stringConfigValue(config.Config, "type"))
	if providerType == "" {
		return pushProviderConfig{}, apperrors.NewUnauthorized("push mfa provider type is not configured")
	}

	return pushProviderConfig{
		providerType:  providerType,
		clientID:      firstNonEmpty(provider.ClientID, stringConfigValue(config.Config, "clientId")),
		clientSecret:  firstNonEmpty(provider.ClientSecret, stringConfigValue(config.Config, "clientSecret")),
		clientID2:     stringConfigValue(config.Config, "clientId2"),
		clientSecret2: stringConfigValue(config.Config, "clientSecret2"),
		appID:         stringConfigValue(config.Config, "appId"),
		method:        stringConfigValue(config.Config, "method"),
		title:         stringConfigValue(config.Config, "title"),
		metadata:      stringConfigValue(config.Config, "metadata"),
		regionID:      stringConfigValue(config.Config, "regionId"),
	}, nil
}

var sendPushAuthentication = func(ctx context.Context, config pushProviderConfig, receiver string, title string, message string) error {
	notifier := NewDefaultPushNotifier(nil)
	return notifier.Send(ctx, config.providerType, config, receiver, title, message)
}

type LoginResult struct {
	SessionID string
	Response  LoginResponse
	// RememberMeToken is the rotated/new plaintext handed to the client exactly
	// once. Empty when no remember-me flow took part in this login.
	RememberMeToken     string
	RememberMeExpiresAt *time.Time
}
