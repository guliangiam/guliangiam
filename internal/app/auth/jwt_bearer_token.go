package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

const (
	// jwtBearerGrantType is the RFC 7523 §2.1 grant type. It is not a client
	// authentication method: the same signed assertion must also satisfy the
	// client-assertion verifier against this client's registered JWKS.
	jwtBearerGrantType = "urn:ietf:params:oauth:grant-type:jwt-bearer"

	// ClientAssertionTypeJWT identifies the JWT bearer assertion type per RFC 7523.
	ClientAssertionTypeJWT = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
)

// GetJwtBearerTokenCommand contains parameters for the JWT-bearer grant type (RFC 7523).
type GetJwtBearerTokenCommand struct {
	ClientAssertionType string // Must be "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	ClientAssertion     string // The JWT assertion
	Scope               string // Optional requested scope
	// TokenEndpointURL is the absolute URL of the token endpoint. It is the
	// `aud` value the assertion must declare (RFC 7523 §3).
	TokenEndpointURL string
}

// GetJwtBearerToken implements the OAuth 2.0 JWT-bearer grant type (RFC 7523).
//
// The assertion is first verified as a client assertion for the issuer
// (client_id): its kid must resolve in that client's app_client_jwks, iss/sub
// must equal the client_id, aud must be this token endpoint, and jti must not
// have been seen. The verification transaction commits so the jti stays
// consumed (replay protection is preserved even though issuance is blocked).
//
// Issuance is blocked exactly like ExchangeClientCredentials (ADR-038 step 1):
// the grant has no user subject, so persisting a token with an empty UserID
// fails at the write ($2::uuid rejects the empty string, 22P02), and RFC 7523
// client-only tokens must not receive refresh tokens (R1 limits refresh to
// user-bound flows). An authenticated caller therefore gets a truthful
// unsupported_grant_type instead of a 500. Authentication failures still yield
// 401. Declaration/capability convergence in discovery awaits D1/D2.
// See .trae/documents/client-auth-reachability/adr-principal-model.md.
func (s *Service) GetJwtBearerToken(ctx context.Context, command GetJwtBearerTokenCommand) (*ExchangeAuthorizationCodeResponse, error) {
	clientAssertion := strings.TrimSpace(command.ClientAssertion)
	if clientAssertion == "" {
		return nil, apperrors.NewInvalidArgument("client_assertion is required")
	}
	if strings.TrimSpace(command.ClientAssertionType) != ClientAssertionTypeJWT {
		return nil, apperrors.NewInvalidArgument("client_assertion_type must be urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	}

	// The issuer identifies the client. It is unverified at this point; the
	// verifier below rejects any mismatch.
	issuer, err := unverifiedIssuer(clientAssertion)
	if err != nil || issuer == "" {
		return nil, apperrors.NewUnauthorized(clientAuthFailureMessage)
	}

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByClientID(ctx, issuer)
		if err != nil {
			return apperrors.NewUnauthorized(clientAuthFailureMessage)
		}
		if !containsString(application.GrantTypes, jwtBearerGrantType) {
			return apperrors.NewUnauthorized(clientAuthFailureMessage)
		}
		if err := s.verifyPrivateKeyJWTAssertion(
			ctx,
			tx,
			application,
			clientAssertion,
			strings.TrimSpace(command.ClientAssertionType),
			strings.TrimSpace(command.TokenEndpointURL),
		); err != nil {
			return apperrors.NewUnauthorized(clientAuthFailureMessage)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return nil, newClientCredentialsProtocolError(
		http.StatusBadRequest,
		"unsupported_grant_type",
		"jwt-bearer is unsupported: token subjects are limited to users",
	)
}

// unverifiedIssuer extracts iss from a JWT without checking its signature. It
// is used only to locate the candidate client row; verifyPrivateKeyJWTAssertion
// re-checks iss after the signature succeeds.
func unverifiedIssuer(tokenString string) (string, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", apperrors.NewInvalidArgument("invalid token format")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", apperrors.NewInvalidArgument("invalid token payload")
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", apperrors.NewInvalidArgument("invalid token claims")
	}
	issuer, _ := claims["iss"].(string)
	return strings.TrimSpace(issuer), nil
}
