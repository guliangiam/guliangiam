package auth

import (
	"context"
	"crypto/x509"
	"errors"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// resolveClientApplication locates a client_id for a request that has no session
// and therefore no tenant scope. RLS is fail-closed outside a transaction, so
// the lookup gets its own system transaction and nothing else runs inside it:
// the transaction must stay read-only and must not carry token reads or writes.
func (s *Service) resolveClientApplication(ctx context.Context, clientID string) (*domain.Application, error) {
	var application *domain.Application
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		looked, err := tx.Applications().GetByClientID(ctx, strings.TrimSpace(clientID))
		if err != nil {
			return err
		}
		application = looked
		return nil
	})
	if err != nil {
		return nil, err
	}
	if application == nil {
		return nil, apperrors.NewNotFound("application not found")
	}
	return application, nil
}

// applicationTenantScope pins follow-up work to the application's own tenant.
// Strict mode panics when the organization is empty, which turns a malformed
// application row into a loud failure instead of a cross-tenant read. RLS stays
// the last line of defense: callers also compare row organization explicitly.
func applicationTenantScope(ctx context.Context, application *domain.Application) context.Context {
	return tenantctx.WithTenantScope(ctx, application.OrganizationID, "", tenantctx.TenantFilterStrict)
}

// AuthenticateClientCommand contains parameters for client authentication.
type AuthenticateClientCommand struct {
	ClientID            string // Client identifier
	ClientSecret        string // Client secret (for client_secret_basic / client_secret_post)
	ClientAssertion     string // JWT assertion (for private_key_jwt)
	ClientAssertionType string // Assertion type identifier
	// PeerCertificates is the client certificate chain resolved by the transport
	// boundary: the in-process TLS state by default, or a header forwarded by an
	// explicitly configured trusted proxy (MTLS_PROXY_CLIENT_CERT_HEADER). The
	// app layer applies the same chain verification and fingerprint match to
	// both sources.
	PeerCertificates []*x509.Certificate
	// TokenEndpointURL is the absolute URL of the token endpoint. private_key_jwt
	// assertions must declare it as their `aud`.
	TokenEndpointURL string
}

// ClientAuthResult contains the result of a client authentication attempt.
type ClientAuthResult struct {
	Authenticated bool
	ClientID      string
	ApplicationID string
}

// clientAuthFailureMessage 是所有客户端认证失败的统一对外文案。客户端不存在、
// 密钥错误、方式不匹配、断言非法都必须返回同一错误码与同一文案，避免调用方
// 通过 error_description 区分失败原因。
const clientAuthFailureMessage = "client authentication failed"

// AuthenticateClient performs enhanced client authentication supporting:
// - HTTP Basic Auth (client_secret_basic)
// - client_secret_post
// - client_assertion JWT (private_key_jwt per OIDC Core 1.0 Section 9)
func (s *Service) AuthenticateClient(ctx context.Context, command AuthenticateClientCommand) (*ClientAuthResult, error) {
	if strings.TrimSpace(command.ClientID) == "" {
		return nil, apperrors.NewInvalidArgument("clientId is required")
	}

	var result *ClientAuthResult
	err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		application, err := tx.Applications().GetByClientID(ctx, strings.TrimSpace(command.ClientID))
		if err != nil {
			return apperrors.NewUnauthorized(clientAuthFailureMessage)
		}

		authMethod := application.EffectiveTokenEndpointAuthMethod()

		switch authMethod {
		case "client_secret_basic", "client_secret_post":
			// Authenticate using client_secret. New rows live in
			// app_client_secrets (active + grace window); the legacy column is a
			// read-only fallback for pre-migration data (TODO(deprecate)).
			clientSecret := strings.TrimSpace(command.ClientSecret)
			if clientSecret == "" {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}
			matched, verifyErr := s.verifyApplicationClientSecret(ctx, tx, application, clientSecret)
			if verifyErr != nil {
				return verifyErr
			}
			if !matched {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}

		case "none":
			// Public client - no authentication needed.
			if strings.TrimSpace(command.ClientSecret) != "" || strings.TrimSpace(command.ClientAssertion) != "" {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}

		case domain.TokenEndpointAuthMethodPrivateKeyJWT:
			// private_key_jwt: the assertion is the only credential. A client
			// secret alongside it is a mismatch, not a fallback.
			if strings.TrimSpace(command.ClientSecret) != "" {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}
			if strings.TrimSpace(command.ClientAssertion) == "" ||
				strings.TrimSpace(command.ClientAssertionType) != ClientAssertionTypeJWT {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}
			if err := s.verifyPrivateKeyJWTAssertion(
				ctx,
				tx,
				application,
				strings.TrimSpace(command.ClientAssertion),
				strings.TrimSpace(command.ClientAssertionType),
				command.TokenEndpointURL,
			); err != nil {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}

		case domain.TokenEndpointAuthMethodTLSClientAuth, domain.TokenEndpointAuthMethodSelfSignedTLSClientAuth:
			// mTLS: the TLS client certificate is the only credential. A secret or
			// a JWT assertion alongside it is a method mismatch, not a fallback.
			if strings.TrimSpace(command.ClientSecret) != "" || strings.TrimSpace(command.ClientAssertion) != "" {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}
			if !s.clientCertificateMatches(ctx, tx, application, command.PeerCertificates) {
				return apperrors.NewUnauthorized(clientAuthFailureMessage)
			}

		default:
			// Unsupported or downgraded methods (client_secret_jwt) are rejected.
			return apperrors.NewUnauthorized(clientAuthFailureMessage)
		}

		result = &ClientAuthResult{
			Authenticated: true,
			ClientID:      application.ClientID,
			ApplicationID: application.ID,
		}
		return nil
	})
	if err != nil {
		var platformErr *apperrors.Error
		if errors.As(err, &platformErr) {
			return nil, err
		}
		return nil, apperrors.WrapInternal(err, "client authentication transaction")
	}

	return result, nil
}

// verifyApplicationClientSecret checks the presented secret against every
// usable row (active + grace window) in app_client_secrets, then falls back to
// the legacy app_oauth_config.client_secret_hash column for pre-migration data.
// Any repository error is returned unwrapped so callers can map it to 401
// invalid_client instead of a 500.
func (s *Service) verifyApplicationClientSecret(ctx context.Context, tx appuow.UnitOfWork, application *domain.Application, presented string) (bool, error) {
	now := s.Clock.Now()
	secrets, err := tx.Applications().ListUsableClientSecrets(ctx, application.OrganizationID, application.ID, now)
	if err != nil {
		return false, err
	}
	attempted := false
	for _, secret := range secrets {
		if secret.SecretHash == "" {
			continue
		}
		attempted = true
		match, verifyErr := platformcrypto.VerifyClientSecret(presented, secret.SecretHash)
		if verifyErr == nil && match {
			return true, nil
		}
	}
	// TODO(deprecate): legacy hash is read-only compatibility; remove once all
	// installations have migrated every application to app_client_secrets.
	if strings.TrimSpace(application.ClientSecretHash) == "" {
		if !attempted {
			// No stored credential to compare against: spend one same-cost
			// verification so the caller cannot distinguish this from a
			// wrong-secret attempt by response time.
			platformcrypto.BurnClientSecretVerification(presented)
		}
		return false, nil
	}
	match, verifyErr := platformcrypto.VerifyClientSecret(presented, application.ClientSecretHash)
	if verifyErr != nil {
		return false, nil
	}
	return match, nil
}
