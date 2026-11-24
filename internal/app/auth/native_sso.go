package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// ActorTokenTypeDeviceSecret is the actor token type for Native SSO (RFC 8693 extension).
	ActorTokenTypeDeviceSecret = "urn:openid:params:token-type:device-secret" //nolint:gosec // G101: token-type URN, not a credential

	// DeviceSsoScope is the scope that triggers device secret generation.
	DeviceSsoScope = "device_sso"
)

// NativeSsoResult contains the result of a Native SSO token exchange.
type NativeSsoResult struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int64
	Scope        string
	RefreshToken string
	IDToken      string
}

// ExchangeNativeSsoToken performs a Native SSO token exchange.
// It validates the device-secret binding and issues a new token for the target application.
func (s *Service) ExchangeNativeSsoToken(
	ctx context.Context,
	subjectToken, subjectTokenType, actorToken, actorTokenType, scope, clientID, clientSecret string,
) (*NativeSsoResult, error) {
	// Validate subject token type.
	if subjectTokenType != SubjectTokenTypeAccessToken {
		return nil, apperrors.NewInvalidArgument("subject_token_type must be access_token for native SSO")
	}

	// Validate actor token type.
	if actorTokenType != ActorTokenTypeDeviceSecret {
		return nil, apperrors.NewInvalidArgument("actor_token_type must be urn:openid:params:token-type:device-secret for native SSO")
	}

	// Look up the device secret token.
	deviceSecretHash := platformcrypto.HashTokenValue(actorToken)
	dsToken, err := s.UOW.Tokens().GetByDeviceSecretHash(ctx, deviceSecretHash)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid device secret")
	}

	// Check device secret token is not expired or revoked.
	now := s.Clock.Now()
	if dsToken.RevokedAt != nil || now.After(dsToken.ExpiresAt) {
		return nil, apperrors.NewUnauthorized("device secret has expired or been revoked")
	}

	// Look up the subject token.
	subjectTokenHash := platformcrypto.HashTokenValue(subjectToken)
	subToken, err := s.UOW.Tokens().GetByTokenHash(ctx, subjectTokenHash)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid subject token")
	}

	// Verify both tokens belong to the same user.
	if subToken.UserID != dsToken.UserID {
		return nil, apperrors.NewUnauthorized("subject token and device secret must belong to the same user")
	}

	// Cross-organization exchange requires client_secret.
	if subToken.OrganizationID != dsToken.OrganizationID {
		if strings.TrimSpace(clientSecret) == "" {
			return nil, apperrors.NewUnauthorized("client_secret is required for cross-organization native SSO")
		}
	}

	// Remove device_sso scope from the resulting scope (it's internal-only).
	resultScopes := filterScope(subToken.Scopes, DeviceSsoScope)
	if scope != "" {
		requestedScopes := strings.Split(scope, " ")
		resultScopes = intersectScopes(resultScopes, requestedScopes)
	}

	return &NativeSsoResult{
		AccessToken: subjectToken,
		TokenType:   tokenTypeBearer,
		ExpiresIn:   int64(time.Until(subToken.ExpiresAt).Seconds()),
		Scope:       strings.Join(resultScopes, " "),
	}, nil
}

// AddDeviceSecret generates a device secret for the given token if the scope includes device_sso.
// Returns the plaintext device secret, or empty string if not applicable.
func (s *Service) AddDeviceSecret(ctx context.Context, tx appuow.UnitOfWork, token *tokendomain.Token) (string, error) {
	if !containsScope(token.Scopes, DeviceSsoScope) {
		return "", nil
	}

	deviceSecret, err := generateDeviceSecret()
	if err != nil {
		return "", apperrors.NewInternal("generate device secret")
	}

	deviceSecretHash := platformcrypto.HashTokenValue(deviceSecret)
	expiresIn := int(time.Until(token.ExpiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}

	// The access token row is inserted before this call, so the device secret
	// hash must be persisted with an UPDATE. Previously this issued a second
	// INSERT that always failed on the primary key and the error was discarded,
	// leaving the returned secret unusable.
	if err := tx.Tokens().UpdateDeviceSecret(ctx, token.ID, deviceSecretHash, expiresIn); err != nil {
		return "", err
	}

	// Keep the in-memory domain object consistent for callers inside the
	// same transaction.
	token.DeviceSecretHash = deviceSecretHash
	token.DeviceSecretExpiresIn = expiresIn

	return deviceSecret, nil
}

// CompleteNativeSso completes a Native SSO flow by verifying the access token
// was issued via token exchange and completing the login process.
func (s *Service) CompleteNativeSso(ctx context.Context, accessToken, applicationID string) error {
	tokenHash := platformcrypto.HashTokenValue(accessToken)
	token, err := s.UOW.Tokens().GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return apperrors.NewUnauthorized("invalid access token")
	}

	// Verify the token was issued via token exchange (has device_sso in original scope metadata).
	if token.DeviceSecretHash == "" {
		return apperrors.NewUnauthorized("token was not issued via native SSO")
	}

	// Verify the application matches.
	if token.ApplicationID != applicationID {
		return apperrors.NewUnauthorized("token application mismatch")
	}

	return nil
}

func generateDeviceSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", apperrors.WrapInternal(err, "generate device secret")
	}
	return hex.EncodeToString(bytes), nil
}

func filterScope(scopes []string, exclude string) []string {
	result := make([]string, 0, len(scopes))
	for _, s := range scopes {
		if s != exclude {
			result = append(result, s)
		}
	}
	return result
}

func intersectScopes(available, requested []string) []string {
	availableSet := make(map[string]struct{}, len(available))
	for _, s := range available {
		availableSet[s] = struct{}{}
	}
	result := make([]string, 0)
	for _, s := range requested {
		if _, ok := availableSet[s]; ok {
			result = append(result, s)
		}
	}
	return result
}

func containsScope(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}
