package auth

import (
	"context"
	"crypto/x509"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// RefreshTokenExchangeRequest is the trusted input for rotation. OrganizationID
// and ApplicationID are resolved server-side, never taken from the request body.
type RefreshTokenExchangeRequest struct {
	OrganizationID string
	ApplicationID  string
	RefreshToken   string
}

// RefreshTokenExchangeResult is the rotated token pair.
type RefreshTokenExchangeResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scopes       []string
	TokenType    string
}

// RefreshTokenExchanger rotates a refresh token. Implemented in the composition
// root on top of app/tokens so the auth service keeps no cross-app import.
type RefreshTokenExchanger interface {
	ExchangeRefreshToken(ctx context.Context, request RefreshTokenExchangeRequest) (RefreshTokenExchangeResult, error)
}

// ExchangeRefreshTokenCommand represents the RFC 6749 §6 refresh_token grant.
type ExchangeRefreshTokenCommand struct {
	RefreshToken string
	ClientID     string
	ClientSecret string
	Scope        string

	ClientAuthMethod    string
	ClientAssertion     string
	ClientAssertionType string
	// PeerCertificates is the TLS client chain resolved by the transport layer.
	PeerCertificates []*x509.Certificate
	TokenEndpointURL string
}

// invalidGrantError renders RFC 6749 §5.2 invalid_grant (400).
type invalidGrantError struct{ description string }

func (e *invalidGrantError) Error() string        { return e.description }
func (e *invalidGrantError) ProtocolCode() string { return "invalid_grant" }
func (e *invalidGrantError) HTTPStatus() int      { return 400 }

func newInvalidGrantError(description string) error {
	return &invalidGrantError{description: description}
}

// ExchangeRefreshToken authenticates the client first (same decision tree as
// the token endpoint), requires the registered refresh_token grant, then rotates
// the token pair through the injected exchanger.
func (s *Service) ExchangeRefreshToken(ctx context.Context, command ExchangeRefreshTokenCommand) (*ExchangeAuthorizationCodeResponse, error) {
	if strings.TrimSpace(command.RefreshToken) == "" {
		return nil, newInvalidGrantError("refresh_token is required")
	}
	clientID := strings.TrimSpace(command.ClientID)
	if clientID == "" {
		return nil, NewInvalidClientError("client authentication failed")
	}
	material := clientAuthMaterial{
		ClientSecret:        command.ClientSecret,
		ClientAssertion:     command.ClientAssertion,
		ClientAssertionType: command.ClientAssertionType,
		ClientAuthMethod:    command.ClientAuthMethod,
		PeerCertificates:    command.PeerCertificates,
	}
	var app *applicationdomain.Application
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, lookupErr := tx.Applications().GetByClientID(ctx, clientID)
		if lookupErr != nil {
			platformcrypto.BurnClientSecretVerification(command.ClientSecret)
			return NewInvalidClientError("client authentication failed")
		}
		if authErr := s.authenticateProtocolClient(ctx, tx, material, application, command.TokenEndpointURL); authErr != nil {
			return authErr
		}
		app = application
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !containsString(app.GrantTypes, oauthRefreshToken) {
		return nil, newInvalidGrantError("client is not allowed to use the refresh_token grant")
	}
	if s.RefreshTokens == nil {
		return nil, apperrors.NewUnavailable("refresh token exchange is not configured")
	}

	// ADR-029 pre_token: fail-closed policy gate before rotation. The user
	// is only known after the exchanger validates the refresh token, so the
	// input carries client/tenant metadata only — never the token value.
	if gateErr := s.enforceActionsPreToken(ctx, app.OrganizationID, "", "", app.ID,
		oauthRefreshToken, normalizeScopesFromString(command.Scope),
		newInvalidGrantError("token issuance denied by policy")); gateErr != nil {
		return nil, gateErr
	}

	result, err := s.RefreshTokens.ExchangeRefreshToken(applicationTenantScope(ctx, app), RefreshTokenExchangeRequest{
		OrganizationID: app.OrganizationID,
		ApplicationID:  app.ID,
		RefreshToken:   command.RefreshToken,
	})
	if err != nil {
		if appErr, ok := err.(*apperrors.Error); ok {
			switch appErr.Code {
			case apperrors.CodeInvalidArgument, apperrors.CodeUnauthorized, apperrors.CodeNotFound, apperrors.CodeForbidden:
				return nil, newInvalidGrantError("refresh token is invalid or expired")
			}
		}
		return nil, apperrors.WrapInternal(err, "refresh token exchange")
	}

	tokenType := strings.TrimSpace(result.TokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}
	expiresIn := int64(0)
	if !result.ExpiresAt.IsZero() {
		expiresIn = int64(result.ExpiresAt.Sub(s.Clock.Now()).Seconds())
		if expiresIn < 0 {
			expiresIn = 0
		}
	}
	return &ExchangeAuthorizationCodeResponse{
		Status:       "issued",
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    tokenType,
		ExpiresIn:    expiresIn,
		Scope:        strings.Join(result.Scopes, " "),
	}, nil
}
