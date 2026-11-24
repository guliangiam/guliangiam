package auth

import (
	"context"
	"crypto/x509"
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	domainworkload "github.com/guliangiam/guliangiam/internal/domain/workload"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	clientCredentialsAccessTokenTTL      = 1 * time.Hour
	defaultServiceAccountTokenTTLSeconds = 900
	minimumServiceAccountTokenTTLSeconds = 60
	maximumServiceAccountTokenTTLSeconds = 3600
	clientCredentialsGrantType           = "client_credentials"
)

// ExchangeClientCredentialsCommand represents a client_credentials grant request.
type ExchangeClientCredentialsCommand struct {
	ClientID     string
	ClientSecret string
	Scope        string
	// ClientAuthMethod 是本次请求实际使用的 HTTP 客户端认证方式
	// （client_secret_basic / client_secret_post / private_key_jwt）。由传输层
	// 解析；为空表示非 RFC 表单入口（JSON 变体），此时跳过方式匹配。
	ClientAuthMethod    string
	ClientAssertion     string
	ClientAssertionType string
	// PeerCertificates is the in-process TLS client certificate chain for the
	// RFC 8705 mTLS methods. Reverse-proxy headers are never read.
	PeerCertificates []*x509.Certificate
	// TokenEndpointURL 是 private_key_jwt 断言必须声明的 aud。
	TokenEndpointURL string
}

type clientCredentialsProtocolError struct {
	status      int
	code        string
	description string
}

func (e *clientCredentialsProtocolError) Error() string        { return e.description }
func (e *clientCredentialsProtocolError) ProtocolCode() string { return e.code }
func (e *clientCredentialsProtocolError) HTTPStatus() int      { return e.status }

func newClientCredentialsProtocolError(status int, code, description string) error {
	return &clientCredentialsProtocolError{status: status, code: code, description: description}
}

func invalidClientCredentials() error {
	return newClientCredentialsProtocolError(http.StatusUnauthorized, "invalid_client", "client authentication failed")
}

// clientAuthMismatchAudit 携带客户端认证方式不匹配的审计字段。认证失败会让
// 所在事务回滚，因此审计必须在独立事务中落库，不能与失败事务共用 tx。
type clientAuthMismatchAudit struct {
	OrganizationID   string
	ApplicationID    string
	ClientID         string
	PresentedMethod  string
	RegisteredMethod string
	Reason           string
}

// clientAuthMismatchError 是 RFC 6749 §5.2 的 invalid_client（401），并携带
// 可选的审计上下文，供调用方在事务外补写审计。
type clientAuthMismatchError struct {
	description string
	audit       *clientAuthMismatchAudit
}

func (e *clientAuthMismatchError) Error() string        { return e.description }
func (e *clientAuthMismatchError) ProtocolCode() string { return "invalid_client" }
func (e *clientAuthMismatchError) HTTPStatus() int      { return http.StatusUnauthorized }

// NewInvalidClientError 供传输层在解析阶段（Basic 头非法、Basic 与 body 混用等）
// 直接产出 RFC 6749 §5.2 的 invalid_client，而不是通用的 invalid_token。
func NewInvalidClientError(description string) error {
	return &clientAuthMismatchError{description: description}
}

// newClientAuthMismatchError 构造带审计上下文的 invalid_client。application 为
// nil（客户端不存在）时没有可归属的租户，不写入审计。
func newClientAuthMismatchError(application *applicationdomain.Application, presented, registered, reason string) error {
	err := &clientAuthMismatchError{description: reason}
	if application != nil {
		err.audit = &clientAuthMismatchAudit{
			OrganizationID:   application.OrganizationID,
			ApplicationID:    application.ID,
			ClientID:         application.ClientID,
			PresentedMethod:  presented,
			RegisteredMethod: registered,
			Reason:           reason,
		}
	}
	return err
}

type clientCredentialsPrincipal struct {
	serviceAccount *domainworkload.ServiceAccount
	application    *applicationdomain.Application
	scopes         []string
	ttl            time.Duration
}

func (p *clientCredentialsPrincipal) organizationID() string {
	if p == nil {
		return ""
	}
	if p.serviceAccount != nil {
		return p.serviceAccount.OrganizationID
	}
	if p.application != nil {
		return p.application.OrganizationID
	}
	return ""
}

// ExchangeClientCredentials implements the OAuth 2.0 client_credentials grant type (RFC 6749 §4.4).
// Service accounts take precedence over applications with the same globally unique client_id.
// Neither principal receives a refresh token or an ID token.
func (s *Service) ExchangeClientCredentials(ctx context.Context, command ExchangeClientCredentialsCommand) (*ExchangeAuthorizationCodeResponse, error) {
	clientID := strings.TrimSpace(command.ClientID)
	if clientID == "" {
		return nil, apperrors.NewInvalidArgument("clientId is required")
	}

	principal, err := s.authenticateClientCredentials(ctx, clientID, command)
	if err != nil {
		s.persistClientAuthMismatchAudit(ctx, err)
		return nil, err
	}
	if principal.organizationID() == "" {
		return nil, apperrors.NewInternal("client credentials principal organization is required")
	}

	// Neither principal type can be persisted as a token subject yet:
	//   - application: user_id stays empty and the INSERT binds $2::uuid, so
	//     PostgreSQL rejects the empty string (22P02);
	//   - service account: its id is written into user_id, which carries
	//     FK -> users(id) (23503), and application_id is never set (NOT NULL).
	// Both branches fail at the write, and no real database path exercises
	// them (the existing unit tests use fake repositories). Authentication has
	// already run, so an unknown client or a wrong secret still yields
	// invalid_client; an authenticated caller gets the truthful answer that the
	// grant is not supported until token subjects are modeled as principals.
	// See .trae/documents/client-auth-reachability/adr-principal-model.md.
	return nil, newClientCredentialsProtocolError(
		http.StatusBadRequest,
		"unsupported_grant_type",
		"client_credentials is unsupported: token subjects are limited to users",
	)
}

func (s *Service) authenticateClientCredentials(ctx context.Context, clientID string, command ExchangeClientCredentialsCommand) (*clientCredentialsPrincipal, error) {
	var principal *clientCredentialsPrincipal
	// client_id is globally unique, so the anonymous request must first locate
	// the credential under RLS. Keep this system transaction read-only and limit
	// it to authentication; all writes use the principal's tenant context.
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		if workloadRepo := tx.Workload(); workloadRepo != nil {
			account, err := workloadRepo.GetAccountByClientID(ctx, clientID)
			if err == nil && account != nil {
				validated, err := authenticateServiceAccount(account, command)
				if err != nil {
					return err
				}
				principal = validated
				return nil
			}
			if err != nil && !isWorkloadNotFound(err) {
				return apperrors.WrapInternal(err, "lookup service account")
			}
		}

		application, err := tx.Applications().GetByClientID(ctx, clientID)
		if err != nil {
			// Equalise timing with the wrong-secret path.
			platformcrypto.BurnClientSecretVerification(command.ClientSecret)
			return invalidClientCredentials()
		}
		validated, err := s.authenticateApplicationClient(ctx, tx, application, command)
		if err != nil {
			return err
		}
		principal = validated
		return nil
	})
	if err != nil {
		return nil, err
	}
	if principal == nil {
		return nil, invalidClientCredentials()
	}
	return principal, nil
}

func authenticateServiceAccount(account *domainworkload.ServiceAccount, command ExchangeClientCredentialsCommand) (*clientCredentialsPrincipal, error) {
	if account == nil || account.Status != domainworkload.SAStatusActive {
		return nil, invalidClientCredentials()
	}
	secret := strings.TrimSpace(command.ClientSecret)
	if secret == "" || strings.TrimSpace(account.ClientSecretHash) == "" {
		return nil, invalidClientCredentials()
	}
	match, err := platformcrypto.VerifyClientSecret(secret, account.ClientSecretHash)
	if err != nil || !match {
		return nil, invalidClientCredentials()
	}

	ttlSeconds := account.AccessTokenTTLSeconds
	if ttlSeconds == 0 {
		ttlSeconds = defaultServiceAccountTokenTTLSeconds
	}
	if ttlSeconds < minimumServiceAccountTokenTTLSeconds || ttlSeconds > maximumServiceAccountTokenTTLSeconds {
		return nil, apperrors.NewInternal("service account access token TTL is invalid")
	}
	scopes := normalizeScopesFromString(command.Scope)
	if !isScopeSubset(scopes, account.Scopes) {
		return nil, newClientCredentialsProtocolError(http.StatusBadRequest, "invalid_scope", "requested scope exceeds service account scopes")
	}
	return &clientCredentialsPrincipal{
		serviceAccount: account,
		scopes:         scopes,
		ttl:            time.Duration(ttlSeconds) * time.Second,
	}, nil
}

func (s *Service) authenticateApplicationClient(ctx context.Context, tx appuow.UnitOfWork, application *applicationdomain.Application, command ExchangeClientCredentialsCommand) (*clientCredentialsPrincipal, error) {
	if application == nil {
		return nil, invalidClientCredentials()
	}
	if !application.IsConfidential() {
		return nil, newClientCredentialsProtocolError(http.StatusBadRequest, "unsupported_grant_type", "client_credentials grant requires a confidential client")
	}
	if presented := strings.TrimSpace(command.ClientAuthMethod); presented != "" {
		if registered := application.EffectiveTokenEndpointAuthMethod(); presented != registered {
			return nil, newClientAuthMismatchError(application, presented, registered, "client authentication method does not match the registered token_endpoint_auth_method")
		}
	}

	if method := application.EffectiveTokenEndpointAuthMethod(); method == applicationdomain.TokenEndpointAuthMethodTLSClientAuth ||
		method == applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth {
		// mTLS: certificate only; a secret or assertion alongside it is a mismatch.
		if strings.TrimSpace(command.ClientSecret) != "" || strings.TrimSpace(command.ClientAssertion) != "" {
			return nil, invalidClientCredentials()
		}
		if !s.clientCertificateMatches(ctx, tx, application, command.PeerCertificates) {
			return nil, invalidClientCredentials()
		}
		if !containsString(application.GrantTypes, clientCredentialsGrantType) {
			return nil, newClientCredentialsProtocolError(http.StatusBadRequest, "unsupported_grant_type", "client_credentials grant type is not allowed for this application")
		}
		return &clientCredentialsPrincipal{
			application: application,
			scopes:      normalizeScopesFromString(command.Scope),
			ttl:         clientCredentialsAccessTokenTTL,
		}, nil
	}

	if application.EffectiveTokenEndpointAuthMethod() == applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT {
		if strings.TrimSpace(command.ClientSecret) != "" {
			return nil, invalidClientCredentials()
		}
		if strings.TrimSpace(command.ClientAssertion) == "" ||
			strings.TrimSpace(command.ClientAssertionType) != ClientAssertionTypeJWT {
			return nil, invalidClientCredentials()
		}
		if err := s.verifyPrivateKeyJWTAssertion(
			ctx,
			tx,
			application,
			strings.TrimSpace(command.ClientAssertion),
			strings.TrimSpace(command.ClientAssertionType),
			command.TokenEndpointURL,
		); err != nil {
			return nil, invalidClientCredentials()
		}
		if !containsString(application.GrantTypes, clientCredentialsGrantType) {
			return nil, newClientCredentialsProtocolError(http.StatusBadRequest, "unsupported_grant_type", "client_credentials grant type is not allowed for this application")
		}
		return &clientCredentialsPrincipal{
			application: application,
			scopes:      normalizeScopesFromString(command.Scope),
			ttl:         clientCredentialsAccessTokenTTL,
		}, nil
	}

	clientSecret := strings.TrimSpace(command.ClientSecret)
	if clientSecret == "" {
		return nil, invalidClientCredentials()
	}
	matched, err := s.verifyApplicationClientSecret(ctx, tx, application, clientSecret)
	if err != nil {
		return nil, err
	}
	if !matched {
		return nil, invalidClientCredentials()
	}
	if !containsString(application.GrantTypes, clientCredentialsGrantType) {
		return nil, newClientCredentialsProtocolError(http.StatusBadRequest, "unsupported_grant_type", "client_credentials grant type is not allowed for this application")
	}
	return &clientCredentialsPrincipal{
		application: application,
		scopes:      normalizeScopesFromString(command.Scope),
		ttl:         clientCredentialsAccessTokenTTL,
	}, nil
}

func isWorkloadNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows) {
		return true
	}
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == apperrors.CodeNotFound
}

// normalizeScopesFromString splits a space-delimited scope string into a slice.
func normalizeScopesFromString(scopeString string) []string {
	if strings.TrimSpace(scopeString) == "" {
		return []string{}
	}
	parts := strings.Fields(scopeString)
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			result = append(result, p)
		}
	}
	return result
}
