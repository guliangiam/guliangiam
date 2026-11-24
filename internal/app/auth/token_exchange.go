package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	authorizationcodedomain "github.com/guliangiam/guliangiam/internal/domain/authorizationcodes"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformmtls "github.com/guliangiam/guliangiam/internal/platform/mtls"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

const (
	authorizationCodeAccessTokenTTL  = time.Hour
	authorizationCodeRefreshTokenTTL = 30 * 24 * time.Hour
)

// dpopTokenEndpointURL resolves the absolute URL a DPoP proof must name as its
// `htu` for the authorization-code exchange (RFC 9449 §4.3 step 5). The transport
// derives it from the request; the issuer fallback covers callers with no request
// and mirrors the `token_endpoint` the discovery document advertises, so a client
// that read discovery and a server with no request agree on the string.
//
// Returning "" here — as this path used to — makes urlMatches fail against every
// proof, so a dpop_required application could never complete the exchange.
func (s *Service) dpopTokenEndpointURL(command ExchangeAuthorizationCodeCommand) string {
	if trimmed := strings.TrimSpace(command.DPoPURL); trimmed != "" {
		return trimmed
	}
	issuer := strings.TrimRight(strings.TrimSpace(s.resolveIssuer(command.IssuerURL)), "/")
	if issuer == "" {
		return ""
	}
	return issuer + TokenEndpointPath
}

func (s *Service) ExchangeAuthorizationCode(ctx context.Context, command ExchangeAuthorizationCodeCommand) (*ExchangeAuthorizationCodeResponse, error) {
	if err := validateExchangeAuthorizationCodeCommand(command); err != nil {
		return nil, err
	}

	now := s.Clock.Now().UTC()

	// INC-BE-05 两段式（与 refresh_token_grant 同范式）：匿名请求先在 System
	// 只读事务里定位应用并完成客户端认证；事务体只做查询，不做任何写入。
	var application *applicationdomain.Application
	err := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		var lookupErr error
		application, lookupErr = tx.Applications().GetByClientID(ctx, strings.TrimSpace(command.ClientID))
		if lookupErr != nil {
			if isAppNotFound(lookupErr) {
				// Spend one argon2 verification so "client not found" and
				// "wrong secret" cost the same at the token endpoint, and answer
				// with the same invalid_client body: a 400 here would let a caller
				// tell an unregistered client_id from a bad credential.
				platformcrypto.BurnClientSecretVerification(command.ClientSecret)
				return invalidClientCredentials()
			}
			return lookupErr
		}
		if !containsString(application.GrantTypes, grantAuthorizationCode) {
			return apperrors.NewInvalidArgument("application does not allow authorization_code grant")
		}
		return s.validateAuthorizationCodeClientAuthentication(ctx, tx, command, application)
	})
	if err != nil {
		s.persistClientAuthMismatchAudit(ctx, err)
		return nil, err
	}

	var response *ExchangeAuthorizationCodeResponse
	var deviceSecret string
	// 码核销、发 token、重用吊销全部回钉到应用自身租户：读写混合事务不得
	// 运行在 System 作用域（tenantctx 包治理约定）。
	err = s.UOW.WithinTransaction(applicationTenantScope(ctx, application), func(ctx context.Context, tx appuow.UnitOfWork) error {
		// dpopBinding carries the proof thumbprint from validation to token
		// creation. RFC 9449 §6: the AS records the confirmation key in the token
		// so a resource server can compare an incoming proof against it.
		var dpopBinding *DPoPProofResult

		authorizationCode, err := tx.AuthorizationCodes().GetByCode(ctx, strings.TrimSpace(command.Code))
		if err != nil {
			if isAuthorizationCodeNotFound(err) {
				return apperrors.NewInvalidArgument("authorization code is invalid or expired")
			}
			return err
		}

		// A second redemption of an already redeemed code means the code leaked
		// (OAuth 2.1 I-D §4.1.3, 设计报告 14.6.4). Ownership is checked first so a
		// client cannot make another application's tokens disappear.
		if authorizationCode.ApplicationID == application.ID && authorizationCode.UsedAt != nil {
			if err := revokeTokensIssuedFromAuthorizationCode(ctx, tx, authorizationCode.ID, now); err != nil {
				return err
			}
			return apperrors.NewInvalidArgument("authorization code is invalid or expired")
		}

		if err := validateAuthorizationCodeExchange(command, application, authorizationCode, now); err != nil {
			return err
		}

		if application.DpopRequired {
			if strings.TrimSpace(command.DPoPProof) == "" {
				return apperrors.NewInvalidArgument("DPoP proof is required for this application")
			}
			// RFC 9449 §4.3 step 5 requires the proof's `htu` to equal the URL the
			// request was made to. This used to pass "" — and urlMatches is an
			// equality test, so every proof failed here and a dpop_required
			// application could never exchange a code.
			dpopResult, err := s.ValidateDPoPProof(ctx, ValidateDPoPProofCommand{
				DPoPHeader: strings.TrimSpace(command.DPoPProof),
				Method:     http.MethodPost,
				URL:        s.dpopTokenEndpointURL(command),
			})
			if err != nil {
				return err
			}
			// Keep the thumbprint: dropping it is what broke the binding — the
			// token was issued without `cnf`, so a resource server had nothing to
			// compare an incoming proof against.
			dpopBinding = dpopResult
		}

		organization, err := tx.Organizations().GetByID(ctx, authorizationCode.OrganizationID)
		if err != nil {
			return err
		}
		user, err := tx.Users().GetByID(ctx, authorizationCode.UserID)
		if err != nil {
			return err
		}

		// ADR-029 pre_token: fail-closed policy gate before issuance.
		// The authorization code itself never enters the action input.
		if gateErr := s.enforceActionsPreToken(ctx, organization.ID, user.ID, user.Username, application.ID,
			grantAuthorizationCode, authorizationCode.Scopes,
			newInvalidGrantError("token issuance denied by policy")); gateErr != nil {
			return gateErr
		}

		accessTokenValue, accessTokenPrefix, accessTokenHash, err := s.newAccessTokenSecret()
		if err != nil {
			return err
		}
		accessTokenID, err := s.UUIDGenerator.NewString()
		if err != nil {
			return apperrors.NewInternal("generate access token id")
		}

		scopes := normalizeScopes(authorizationCode.Scopes)
		audience := makeAudience(authorizationCode.Resource)
		tokenMetadata := newAuthorizationCodeTokenMetadata(authorizationCode)
		if dpopBinding != nil {
			tokenMetadata = s.BindTokenToDPoP(tokenMetadata, dpopBinding.JKT)
		} else if command.ClientCertificateThumbprint != "" {
			// RFC 8705 §3: bind the access token to the TLS client certificate so a
			// stolen token cannot be replayed without the private key.
			tokenMetadata = platformmtls.BindTokenToClientCertificate(tokenMetadata, command.ClientCertificateThumbprint)
		}
		accessToken := &tokendomain.Token{
			ID:               accessTokenID,
			UserID:           user.ID,
			Username:         user.Username,
			OrganizationID:   organization.ID,
			OrganizationSlug: organization.Slug,
			ApplicationID:    application.ID,
			ApplicationSlug:  application.Slug,
			TokenPrefix:      accessTokenPrefix,
			TokenHash:        accessTokenHash,
			Scopes:           append([]string(nil), scopes...),
			Audience:         audience,
			Metadata:         tokenMetadata,
			ExpiresAt:        now.Add(authorizationCodeAccessTokenTTL),
			CreatedAt:        now,
			UpdatedAt:        now,
			Revision:         1,
		}
		if err := s.embedAuditOutboxPlain(
			tx,
			ctx,
			"",
			"",
			"",
			accessToken.OrganizationID,
			accessToken.UserID,
			"token.created",
			"token",
			accessToken.ID,
			"token",
			"token.created",
			map[string]any{},
		); err != nil {
			return err
		}
		if err := tx.Tokens().Create(ctx, accessToken); err != nil {
			return err
		}

		// Generate device secret if the scope includes device_sso (Native SSO).
		deviceSecret, err = s.AddDeviceSecret(ctx, tx, accessToken)
		if err != nil {
			return err
		}

		// OIDC Core §3.1.3.3: the authorization-code flow must return an ID token
		// when the granted scopes contain `openid`. This is now the only ID token
		// issuance path, since the implicit flow was retired.
		idToken := ""
		if containsString(scopes, scopeOpenID) {
			idToken, err = s.buildIDToken(application, user, organization, idTokenIssuance{
				Issuer:      s.resolveIssuer(command.IssuerURL),
				AuthTime:    authorizationCode.AuthTime,
				Nonce:       authorizationCode.Nonce,
				Scopes:      scopes,
				AccessToken: accessTokenValue,
				// The session the code was issued for. It travels on the code because
				// the token endpoint has no session in hand.
				SessionID: authorizationCode.Sid,
			})
			if err != nil {
				return err
			}
		}

		refreshTokenValue := ""
		if shouldIssueAuthorizationCodeRefreshToken(application.GrantTypes, scopes) {
			refreshTokenSecret, refreshTokenHash, err := s.newRefreshTokenSecret()
			if err != nil {
				return err
			}
			refreshTokenID, err := s.UUIDGenerator.NewString()
			if err != nil {
				return apperrors.NewInternal("generate refresh token id")
			}

			refreshToken := &tokendomain.RefreshToken{
				ID:               refreshTokenID,
				AccessTokenID:    accessToken.ID,
				UserID:           user.ID,
				Username:         user.Username,
				OrganizationID:   organization.ID,
				OrganizationSlug: organization.Slug,
				ApplicationID:    application.ID,
				ApplicationSlug:  application.Slug,
				RefreshTokenHash: refreshTokenHash,
				Scopes:           append([]string(nil), scopes...),
				Audience:         append([]string(nil), audience...),
				// First generation is its own family root: family_id has a
				// self-referencing FK, so a random unrelated UUID would fail
				// the INSERT with 23503.
				FamilyID:  refreshTokenID,
				ExpiresAt: now.Add(authorizationCodeRefreshTokenTTL),
				CreatedAt: now,
				UpdatedAt: now,
				Revision:  1,
			}
			if err := tx.Tokens().CreateRefresh(ctx, refreshToken); err != nil {
				return err
			}
			refreshTokenValue = refreshTokenSecret
		}

		if err := tx.AuthorizationCodes().MarkUsed(ctx, authorizationCode.ID, now); err != nil {
			if isAuthorizationCodeNotFound(err) {
				return apperrors.NewInvalidArgument("authorization code is invalid or expired")
			}
			return err
		}

		response = &ExchangeAuthorizationCodeResponse{
			Status:       paramIssued,
			AccessToken:  accessTokenValue,
			RefreshToken: refreshTokenValue,
			IdToken:      idToken,
			TokenType:    tokenTypeBearer,
			ExpiresIn:    int64(authorizationCodeAccessTokenTTL / time.Second),
			Scope:        strings.Join(scopes, " "),
			DeviceSecret: deviceSecret,
		}
		return nil
	})
	if err != nil {
		s.persistClientAuthMismatchAudit(ctx, err)
		return nil, err
	}

	return response, nil
}

type clientAuthenticationRequirement int

const (
	clientAuthRequirementNone clientAuthenticationRequirement = iota
	clientAuthRequirementSecret
	clientAuthRequirementAssertion
	clientAuthRequirementCertificate
)

// validateClientAuthenticationShape 是无副作用的认证方式裁决：
// 返回值表明还需要执行哪一种凭据验证。它不查询仓储，便于单测覆盖方式
// 匹配与混用拒绝。
// clientAuthMaterial is the credential shape a protocol endpoint presents for
// client authentication, independent of the grant/endpoint carrying it. Revoke
// and introspection reuse the same decision tree as the token endpoint.
type clientAuthMaterial struct {
	ClientSecret        string
	ClientAssertion     string
	ClientAssertionType string
	ClientAuthMethod    string
	PeerCertificates    []*x509.Certificate
}

func clientAuthMaterialFromExchange(command ExchangeAuthorizationCodeCommand) clientAuthMaterial {
	return clientAuthMaterial{
		ClientSecret:        command.ClientSecret,
		ClientAssertion:     command.ClientAssertion,
		ClientAssertionType: command.ClientAssertionType,
		ClientAuthMethod:    command.ClientAuthMethod,
		PeerCertificates:    command.PeerCertificates,
	}
}

func validateClientAuthenticationShape(material clientAuthMaterial, application *applicationdomain.Application) (clientAuthenticationRequirement, error) {
	if application == nil {
		return clientAuthRequirementNone, invalidClientCredentials()
	}

	registered := application.EffectiveTokenEndpointAuthMethod()
	presented := strings.TrimSpace(material.ClientAuthMethod)

	switch registered {
	case applicationdomain.TokenEndpointAuthMethodNone:
		if presented != "" || strings.TrimSpace(material.ClientSecret) != "" || strings.TrimSpace(material.ClientAssertion) != "" {
			return clientAuthRequirementNone, NewInvalidClientError("public client must not present credentials")
		}
		return clientAuthRequirementNone, nil

	case applicationdomain.TokenEndpointAuthMethodClientSecretBasic, applicationdomain.TokenEndpointAuthMethodClientSecretPost:
		if presented != registered {
			return clientAuthRequirementNone, NewInvalidClientError("client authentication method does not match the registered token_endpoint_auth_method")
		}
		if strings.TrimSpace(material.ClientAssertion) != "" {
			return clientAuthRequirementNone, NewInvalidClientError("client_assertion is not allowed with a client-secret method")
		}
		if strings.TrimSpace(material.ClientSecret) == "" {
			return clientAuthRequirementNone, invalidClientCredentials()
		}
		return clientAuthRequirementSecret, nil

	case applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT:
		if presented != registered {
			return clientAuthRequirementNone, NewInvalidClientError("client authentication method does not match the registered token_endpoint_auth_method")
		}
		if strings.TrimSpace(material.ClientSecret) != "" {
			return clientAuthRequirementNone, NewInvalidClientError("client_secret is not allowed with private_key_jwt")
		}
		if strings.TrimSpace(material.ClientAssertion) == "" ||
			strings.TrimSpace(material.ClientAssertionType) != ClientAssertionTypeJWT {
			return clientAuthRequirementNone, invalidClientCredentials()
		}
		return clientAuthRequirementAssertion, nil

	case applicationdomain.TokenEndpointAuthMethodTLSClientAuth, applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth:
		// mTLS is orthogonal to the grant type: an authorization-code client
		// registered for a certificate method authenticates with the TLS chain,
		// exactly like client_credentials. A secret or assertion alongside the
		// certificate is a method mismatch, not a fallback. presentMethod is empty
		// when the transport observed no Basic/body/assertion credential, which is
		// the normal shape of an mTLS request.
		if presented != "" && presented != registered {
			return clientAuthRequirementNone, NewInvalidClientError("client authentication method does not match the registered token_endpoint_auth_method")
		}
		if strings.TrimSpace(material.ClientSecret) != "" || strings.TrimSpace(material.ClientAssertion) != "" {
			return clientAuthRequirementNone, NewInvalidClientError("client_secret and client_assertion are not allowed with an mTLS method")
		}
		if len(material.PeerCertificates) == 0 {
			return clientAuthRequirementNone, invalidClientCredentials()
		}
		return clientAuthRequirementCertificate, nil

	default:
		// Unsupported or downgraded methods (client_secret_jwt) are rejected.
		return clientAuthRequirementNone, NewInvalidClientError("registered token_endpoint_auth_method is not implemented")
	}
}

func (s *Service) validateAuthorizationCodeClientAuthentication(ctx context.Context, tx appuow.UnitOfWork, command ExchangeAuthorizationCodeCommand, application *applicationdomain.Application) error {
	return s.authenticateProtocolClient(ctx, tx, clientAuthMaterialFromExchange(command), application, s.dpopTokenEndpointURL(command))
}

// authenticateProtocolClient applies the shared client-authentication decision
// tree (none / secret / private_key_jwt / mTLS) for any protocol endpoint.
func (s *Service) authenticateProtocolClient(ctx context.Context, tx appuow.UnitOfWork, material clientAuthMaterial, application *applicationdomain.Application, tokenEndpointURL string) error {
	requirement, shapeErr := validateClientAuthenticationShape(material, application)
	if shapeErr != nil {
		registered := ""
		if application != nil {
			registered = application.EffectiveTokenEndpointAuthMethod()
		}
		return newClientAuthMismatchError(application, strings.TrimSpace(material.ClientAuthMethod), registered, shapeErr.Error())
	}

	switch requirement {
	case clientAuthRequirementNone:
		return nil

	case clientAuthRequirementSecret:
		matched, err := s.verifyApplicationClientSecret(ctx, tx, application, strings.TrimSpace(material.ClientSecret))
		if err != nil {
			// A repository failure must not turn a rejected client into a 500.
			return invalidClientCredentials()
		}
		if !matched {
			return invalidClientCredentials()
		}
		return nil

	case clientAuthRequirementAssertion:
		if err := s.verifyPrivateKeyJWTAssertion(
			ctx,
			tx,
			application,
			strings.TrimSpace(material.ClientAssertion),
			strings.TrimSpace(material.ClientAssertionType),
			tokenEndpointURL,
		); err != nil {
			return invalidClientCredentials()
		}
		return nil

	case clientAuthRequirementCertificate:
		// Same four gates as client_credentials: in-process chain only, trusted
		// CA for tls_client_auth, fingerprint row registered for the same method,
		// fail-closed when no CA is configured.
		if !s.clientCertificateMatches(ctx, tx, application, material.PeerCertificates) {
			return invalidClientCredentials()
		}
		return nil

	default:
		return invalidClientCredentials()
	}
}

// persistClientAuthMismatchAudit records a rejected client-authentication attempt
// in a standalone transaction. The exchange transaction is rolled back when the
// attempt is rejected, so writing the audit there would discard it; the request
// context also carries no tenant, and the system scope keeps the records insert
// from being blocked by RLS.
func (s *Service) persistClientAuthMismatchAudit(ctx context.Context, err error) {
	var mismatch *clientAuthMismatchError
	if !errors.As(err, &mismatch) || mismatch.audit == nil {
		return
	}
	audit := mismatch.audit
	if auditErr := s.UOW.WithinTransaction(tenantctx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		return s.embedAuditEvent(
			tx, ctx,
			"", "", "protocol", "", "",
			audit.OrganizationID, audit.ClientID,
			"oauth.client_auth.method_mismatch", "application", audit.ApplicationID,
			"application", "oauth.client_auth.method_mismatch",
			map[string]any{
				"presented_method":  audit.PresentedMethod,
				"registered_method": audit.RegisteredMethod,
				"reason":            audit.Reason,
			},
		)
	}); auditErr != nil {
		slog.WarnContext(ctx, "persist client auth mismatch audit failed", "error", auditErr)
	}
}

func validateAuthorizationCodeExchange(command ExchangeAuthorizationCodeCommand, application *applicationdomain.Application, authorizationCode *authorizationcodedomain.AuthorizationCode, now time.Time) error {
	if authorizationCode.ApplicationID != application.ID {
		return apperrors.NewInvalidArgument("authorization code is invalid or expired")
	}
	if !authorizationCode.ExpiresAt.After(now) {
		return apperrors.NewInvalidArgument("authorization code is invalid or expired")
	}
	if strings.TrimSpace(authorizationCode.RedirectURI) != strings.TrimSpace(command.RedirectURI) {
		return apperrors.NewInvalidArgument("authorization code is invalid or expired")
	}
	if resource := strings.TrimSpace(command.Resource); resource != "" && resource != strings.TrimSpace(authorizationCode.Resource) {
		return apperrors.NewInvalidArgument("resource does not match authorization code")
	}

	// PKCE is mandatory for every client — public and confidential alike (OAuth 2.1
	// I-D §4.1.1, ADR-004). A code without a challenge can only come from a path
	// that ignored the rule, so it is refused rather than exchanged.
	if strings.TrimSpace(authorizationCode.CodeChallenge) == "" {
		return apperrors.NewInvalidArgument("authorization code is not bound to a PKCE challenge")
	}
	if strings.TrimSpace(authorizationCode.ChallengeMethod) != pkceChallengeMethodS256 {
		return apperrors.NewInvalidArgument("authorization code challenge method is unsupported")
	}
	if strings.TrimSpace(command.CodeVerifier) == "" {
		return apperrors.NewInvalidArgument("codeVerifier is required for this authorization code")
	}
	if buildPKCEChallenge(command.CodeVerifier) != authorizationCode.CodeChallenge {
		return apperrors.NewInvalidArgument("codeVerifier is invalid")
	}
	return nil
}

// revokeTokensIssuedFromAuthorizationCode revokes every token the given code
// produced. It runs inside the exchange transaction so the revocation and the
// reuse rejection commit together.
func revokeTokensIssuedFromAuthorizationCode(ctx context.Context, tx appuow.UnitOfWork, authorizationCodeID string, now time.Time) error {
	affected, err := tx.Tokens().RevokeByAuthorizationCodeID(ctx, authorizationCodeID, now)
	if err != nil {
		return apperrors.WrapInternal(err, "revoke tokens issued from a reused authorization code")
	}
	slog.WarnContext(ctx, "authorization code reuse detected",
		"auth_code_id", authorizationCodeID,
		"revoked_count", affected,
	)
	return nil
}

func shouldIssueAuthorizationCodeRefreshToken(grantTypes, scopes []string) bool {
	if !containsString(grantTypes, oauthRefreshToken) {
		return false
	}
	for _, scope := range scopes {
		if scope == scopeOfflineAccess {
			return true
		}
	}
	return false
}

func buildPKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(verifier)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func makeAudience(resource string) []string {
	trimmed := strings.TrimSpace(resource)
	if trimmed == "" {
		return []string{}
	}
	return []string{trimmed}
}

func newAuthorizationCodeTokenMetadata(authorizationCode *authorizationcodedomain.AuthorizationCode) map[string]any {
	metadata := map[string]any{
		paramGrantType:        grantAuthorizationCode,
		"authorizationCodeId": authorizationCode.ID,
	}
	if strings.TrimSpace(authorizationCode.Nonce) != "" {
		metadata["nonce"] = strings.TrimSpace(authorizationCode.Nonce)
	}
	if strings.TrimSpace(authorizationCode.Resource) != "" {
		metadata["resource"] = strings.TrimSpace(authorizationCode.Resource)
	}
	return metadata
}

// newAccessTokenSecret issues a 256-bit crypto/rand bearer secret (URL-safe).
// UUIDv7-derived secrets only carried ~74 random bits plus a guessable time
// prefix, below the 128-bit convention for bearer credentials.
func (s *Service) newAccessTokenSecret() (value, prefix, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", apperrors.WrapInternal(err, "generate access token secret")
	}
	tokenValue := "gl_pat_" + base64.RawURLEncoding.EncodeToString(buf)
	prefixLength := len(tokenValue)
	if prefixLength > tokenPrefixMaxLength {
		prefixLength = tokenPrefixMaxLength
	}
	tokenPrefix := tokenValue[:prefixLength]

	return tokenValue, tokenPrefix, platformcrypto.HashTokenValue(tokenValue), nil
}

func (s *Service) newRefreshTokenSecret() (value, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", apperrors.WrapInternal(err, "generate refresh token secret")
	}
	tokenValue := "gl_prt_" + base64.RawURLEncoding.EncodeToString(buf)
	return tokenValue, platformcrypto.HashTokenValue(tokenValue), nil
}

func isAuthorizationCodeNotFound(err error) bool {
	typed, ok := err.(*apperrors.Error)
	return ok && typed.Code == apperrors.CodeNotFound
}

func isAppNotFound(err error) bool {
	typed, ok := err.(*apperrors.Error)
	return ok && typed.Code == apperrors.CodeNotFound
}
