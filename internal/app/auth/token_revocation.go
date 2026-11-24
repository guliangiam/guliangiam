package auth

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// RevokeTokenCommand represents an RFC 7009 Token Revocation request.
type RevokeTokenCommand struct {
	Token         string
	TokenTypeHint string // "access_token" or oauthRefreshToken
	ClientID      string
	ClientSecret  string
	// Client authentication material (RFC 7009 endpoint authenticates the
	// client like the token endpoint: secret, private_key_jwt or mTLS).
	ClientAuthMethod    string
	ClientAssertion     string
	ClientAssertionType string
	PeerCertificates    []*x509.Certificate
	TokenEndpointURL    string
}

// RevokeToken revokes a token per RFC 7009.
// Per Section 2.2, the endpoint always returns 200 OK even if the token
// does not exist or belongs to another client, to prevent information leakage.
func (s *Service) RevokeToken(ctx context.Context, command RevokeTokenCommand) error {
	if strings.TrimSpace(command.Token) == "" {
		return nil // Per RFC 7009, return success even for empty tokens.
	}

	// Authenticate the client with the same decision tree as the token
	// endpoint (secret / private_key_jwt / mTLS / public none).
	clientID := strings.TrimSpace(command.ClientID)
	if clientID == "" {
		// RFC 7009 §2.2.1: client authentication failures are 401 invalid_client.
		// The same message is used for every failure mode so an unknown client is
		// indistinguishable from a wrong secret (no enumeration oracle).
		return NewInvalidClientError("client authentication failed")
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
			// Equalise timing with the wrong-secret path.
			platformcrypto.BurnClientSecretVerification(command.ClientSecret)
			slog.DebugContext(ctx, "token revocation client lookup failed", "error", lookupErr)
			return NewInvalidClientError("client authentication failed")
		}
		if authErr := s.authenticateProtocolClient(ctx, tx, material, application, command.TokenEndpointURL); authErr != nil {
			return authErr
		}
		app = application
		return nil
	})
	if err != nil {
		return err
	}

	tokenHash := hashTokenValueForRevocation(command.Token)
	tokenTypeHint := strings.TrimSpace(command.TokenTypeHint)

	// Token rows live in the client's tenant, so the lookup and the revoke run
	// under a strict tenant scope. RLS hides other tenants; the organization
	// comparison below is the explicit invariant that does not rely on it.
	err = s.UOW.WithinTransaction(applicationTenantScope(ctx, app), func(ctx context.Context, tx appuow.UnitOfWork) error {
		now := s.Clock.Now()

		// Try access_token first if hint is empty or "access_token".
		if tokenTypeHint == "" || tokenTypeHint == "access_token" {
			token, lookupErr := tx.Tokens().GetByTokenHash(ctx, tokenHash)
			if lookupErr == nil {
				// Verify the token belongs to this client's application.
				if token.OrganizationID == app.OrganizationID && token.ApplicationID == app.ID {
					return tx.Tokens().Revoke(ctx, token.ID, now)
				}
				return nil
			}
		}

		// Try refresh_token if hint is "refresh_token" or access_token lookup failed.
		if tokenTypeHint == "" || tokenTypeHint == oauthRefreshToken {
			refreshToken, lookupErr := tx.Tokens().GetRefreshByTokenHash(ctx, tokenHash)
			if lookupErr == nil {
				if refreshToken.OrganizationID == app.OrganizationID && refreshToken.ApplicationID == app.ID {
					return tx.Tokens().RevokeRefresh(ctx, refreshToken.ID, now)
				}
			}
		}

		return nil
	})
	if err != nil {
		// RFC 7009 still answers 200, but a failed revoke means the token stays
		// live — the failure must stay observable.
		slog.WarnContext(ctx, "token revocation write failed", "client_id", clientID, "error", err)
	}
	return nil
}

func hashTokenValueForRevocation(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
