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
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// IntrospectTokenRFCCommand represents an RFC 7662 Token Introspection request.
type IntrospectTokenRFCCommand struct {
	Token         string
	TokenTypeHint string
	ClientID      string
	ClientSecret  string
	// Client authentication material (RFC 7662 §2.1 authenticates the client).
	ClientAuthMethod    string
	ClientAssertion     string
	ClientAssertionType string
	PeerCertificates    []*x509.Certificate
	TokenEndpointURL    string
	// IssuerURL is the issuer the response must report. It resolves through the
	// same helper as the ID token and the discovery document, because a hardcoded
	// literal here made the introspection `iss` disagree with what the token
	// itself declares (设计报告 14.8).
	IssuerURL string
}

// TokenIntrospectionRFCResponse represents an RFC 7662 Token Introspection response.
type TokenIntrospectionRFCResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Aud       string `json:"aud,omitempty"`
	Iss       string `json:"iss,omitempty"`
	Jti       string `json:"jti,omitempty"`
	Cnf       *Cnf   `json:"cnf,omitempty"`
}

// Cnf represents the confirmation claim for sender-constrained tokens:
// DPoP uses jkt (RFC 9449), mTLS uses x5t#S256 (RFC 8705).
type Cnf struct {
	Jkt     string `json:"jkt,omitempty"`
	X5tS256 string `json:"x5t#S256,omitempty"`
}

// IntrospectTokenRFC performs RFC 7662 compliant token introspection.
// The client must be authenticated via HTTP Basic Auth or request body credentials.
func (s *Service) IntrospectTokenRFC(ctx context.Context, command IntrospectTokenRFCCommand) (*TokenIntrospectionRFCResponse, error) {
	if strings.TrimSpace(command.Token) == "" {
		return &TokenIntrospectionRFCResponse{Active: false}, nil
	}

	// Authenticate the client with the same decision tree as the token endpoint.
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
			slog.DebugContext(ctx, "token introspection client lookup failed", "error", lookupErr)
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

	tokenHash := hashTokenForIntrospection(command.Token)
	tokenTypeHint := strings.TrimSpace(command.TokenTypeHint)

	// Token rows live in the client's tenant, so the lookup runs under a strict
	// tenant scope. RLS hides other tenants; the organization comparison inside
	// the helper is the explicit invariant that does not rely on it.
	response := &TokenIntrospectionRFCResponse{Active: false}
	err = s.UOW.WithinTransaction(applicationTenantScope(ctx, app), func(ctx context.Context, tx appuow.UnitOfWork) error {
		response = s.introspectTokenWithinApplication(ctx, tx, app.OrganizationID, app.ClientID, command, tokenHash, tokenTypeHint)
		return nil
	})
	if err != nil {
		slog.WarnContext(ctx, "token introspection lookup failed", "client_id", clientID, "error", err)
		return &TokenIntrospectionRFCResponse{Active: false}, nil
	}
	return response, nil
}

// introspectTokenWithinApplication reads the token row under the application's
// strict tenant scope. A row whose organization differs from the client's is
// reported inactive rather than relying on RLS alone.
func (s *Service) introspectTokenWithinApplication(ctx context.Context, tx appuow.UnitOfWork, organizationID, clientID string, command IntrospectTokenRFCCommand, tokenHash, tokenTypeHint string) *TokenIntrospectionRFCResponse {
	inactive := &TokenIntrospectionRFCResponse{Active: false}

	// Try access_token first.
	if tokenTypeHint == "" || tokenTypeHint == "access_token" {
		token, err := tx.Tokens().GetByTokenHash(ctx, tokenHash)
		if err == nil {
			if token.OrganizationID != organizationID {
				return inactive
			}
			now := s.Clock.Now()
			if token.RevokedAt != nil || !token.ExpiresAt.After(now) {
				return inactive
			}

			resp := &TokenIntrospectionRFCResponse{
				Active:    true,
				Scope:     strings.Join(token.Scopes, " "),
				ClientID:  clientID,
				Username:  token.Username,
				TokenType: tokenTypeBearer,
				Exp:       token.ExpiresAt.Unix(),
				Iat:       token.CreatedAt.Unix(),
				Sub:       token.UserID,
				Aud:       strings.Join(token.Audience, " "),
				Iss:       s.resolveIssuer(command.IssuerURL),
				Jti:       token.ID,
			}

			// RFC 9449 §6: a DPoP-bound token reports `cnf.jkt` so the resource
			// server can compare an incoming proof against it, and its type is DPoP
			// rather than Bearer so the client knows a proof is required. This used
			// to report the device secret hash as `cnf.jkt`, which is a different
			// value entirely — it would have told a resource server to accept proofs
			// for a key the token was never bound to.
			if jkt := dpopThumbprintFromMetadata(token.Metadata); jkt != "" {
				resp.Cnf = &Cnf{Jkt: jkt}
				resp.TokenType = "DPoP"
			} else if x5t := platformmtls.ClientCertificateThumbprintFromMetadata(token.Metadata); x5t != "" {
				// RFC 8705 §3: a certificate-bound token stays Bearer but reports
				// cnf.x5t#S256 so the resource server can verify the client cert.
				resp.Cnf = &Cnf{X5tS256: x5t}
			}

			return resp
		}
	}

	// Try refresh_token.
	if tokenTypeHint == "" || tokenTypeHint == oauthRefreshToken {
		refreshToken, err := tx.Tokens().GetRefreshByTokenHash(ctx, tokenHash)
		if err == nil {
			if refreshToken.OrganizationID != organizationID {
				return inactive
			}
			now := s.Clock.Now()
			if refreshToken.RevokedAt != nil || !refreshToken.ExpiresAt.After(now) {
				return inactive
			}

			return &TokenIntrospectionRFCResponse{
				Active:    true,
				Scope:     strings.Join(refreshToken.Scopes, " "),
				ClientID:  clientID,
				Username:  refreshToken.Username,
				TokenType: oauthRefreshToken,
				Exp:       refreshToken.ExpiresAt.Unix(),
				Iat:       refreshToken.CreatedAt.Unix(),
				Sub:       refreshToken.UserID,
				Aud:       strings.Join(refreshToken.Audience, " "),
				Iss:       s.resolveIssuer(command.IssuerURL),
				Jti:       refreshToken.ID,
			}
		}
	}

	return inactive
}

func hashTokenForIntrospection(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// dpopThumbprintFromMetadata reads the confirmation key a token was bound to.
// BindTokenToDPoP writes it as metadata.cnf.jkt (RFC 9449 §6). An unbound token
// yields "" so the introspection response omits `cnf` rather than reporting a
// value it was never bound to.
func dpopThumbprintFromMetadata(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}
	cnf, ok := metadata["cnf"].(map[string]any)
	if !ok {
		return ""
	}
	jkt, _ := cnf["jkt"].(string)
	return strings.TrimSpace(jkt)
}
