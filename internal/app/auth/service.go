package auth

import (
	"context"
	"crypto/x509"
	"sync"
	"time"

	domainauth "github.com/guliangiam/guliangiam/internal/domain/auth"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	riskdomain "github.com/guliangiam/guliangiam/internal/domain/risk"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	tokendomain "github.com/guliangiam/guliangiam/internal/domain/tokens"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformdpop "github.com/guliangiam/guliangiam/internal/platform/dpop"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/geo"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"

	domainpermissions "github.com/guliangiam/guliangiam/internal/domain/permissions"
)

// SSFOutbox enqueues CAEP/SSF security events; implemented by the shared
// outbox repository. Optional: nil disables SSF emission (tests/dev).
type SSFOutbox interface {
	Enqueue(ctx context.Context, item *domainoutbox.OutboxEvent) error
}

// RiskEvaluator is the port the auth layer calls before session creation.
// The domain-level risk engine produces one of three actions; the auth layer
// decides what to do with each outcome. nil → risk engine is disabled, every
// login proceeds as allow (backward-compatible fallback).
type RiskEvaluator = riskdomain.LoginRiskEvaluator

type RiskLoginCommand = riskdomain.LoginRiskCommand
type RiskLoginResult = riskdomain.LoginRiskResult

type Service struct {
	Clock         clock.Clock
	UUIDGenerator platformuuid.Generator
	UOW           uow.AuthUoW
	PushNotifier  PushNotifier
	// Notifications 可选：安全提醒（密码重置、异地登录）写入个人通知中心。
	Notifications NotificationPublisher
	// RiskEvaluator is the optional risk engine. nil → every login is "allow".
	RiskEvaluator RiskEvaluator
	// Actions is the optional ADR-029 sync extension-point dispatcher
	// (pre_login / pre_token / post_registration / consent / mfa).
	// nil → extension points disabled; security triggers fail closed when
	// wired, post_registration stays fail-open.
	Actions ActionsDispatcher
	// WebAuthnAllowedOrigins 是 WebAuthn 断言 origin 校验的允许来源（CORS 白名单）。
	// 与 IssuerURL 的来源合并使用，见 webAuthnAllowedOrigins。
	WebAuthnAllowedOrigins []string
	// WebAuthnRequireUV 为真时，注册与认证都要求认证器置 UV 标志（《开发设计》
	// 31.3.2(b)）。bootstrap 从 WEBAUTHN_REQUIRE_UV 注入，默认关闭。
	WebAuthnRequireUV bool
	JWKSManager       domainauth.JWKSProvider
	Outbox            SSFOutbox
	TokenIssuer       tokendomain.TokenIssuer
	// ClientJWKSFetcher fetches a client's remote jwks_uri when no inline JWKS
	// matches the assertion (OPEN-API-61). Nil disables remote JWKS (fail closed).
	ClientJWKSFetcher ClientJWKSFetcher
	// RefreshTokens rotates a refresh token for the OAuth refresh_token grant.
	// Transport-only endpoint; nil disables the grant (fail closed).
	RefreshTokens  RefreshTokenExchanger
	GeoLookup      *geo.LookupService
	CaptchaFactory CaptchaFactory
	// TLSClientCAs verifies the certificate chain of tls_client_auth clients
	// (RFC 8705). nil → no trusted CA configured → tls_client_auth fails closed.
	// self_signed_tls_client_auth does not use this pool: it matches the
	// registered fingerprint byte-for-byte.
	TLSClientCAs *x509.CertPool
	// SAMLValidator is the domain port for SAML protocol validation. Wired to
	// infra/saml.Validator in bootstrap; nil → use package-level adapter functions.
	SAMLValidator domainauth.SAMLValidator
	// RelayStateSigner signs SP-issued RelayState with a server-held key. Wired
	// to infra/saml.RelayStateSigner in bootstrap. nil → SP-initiated issuance
	// fails closed instead of emitting an unsigned state (审计 SEC-A01).
	RelayStateSigner domainauth.RelayStateSigner
	// SAMLReplayStore one-time-consumes the server-issued state of the SAML SP
	// browser ACS. It uses the same shared store as assertion replay so a state
	// cannot complete two logins across replicas. nil → ACS fails closed.
	SAMLReplayStore domainauth.ReplayStore
	// DPoP applies the algorithm whitelist and the replay cache to a proof. It is
	// optional: ValidateDPoPProof falls back to a package-level validator when it
	// is nil, so a missed wiring in bootstrap cannot silently disable replay
	// detection (设计报告 13.1: an unassigned Service field is invisible to
	// go build and go vet).
	DPoP *platformdpop.DPoPValidator
	// ClientAssertionReplayStore catches private_key_jwt `jti` replay for the
	// assertion's remaining lifetime. Bootstrap wires the same Redis/process-local
	// store used by SAML; nil falls back to a process-local cache.
	ClientAssertionReplayStore AssertionReplayStore
	// BreachChecker checks candidate passwords against a breached-password
	// corpus (AUTH-009, HIBP k-anonymity). nil → disabled
	// (PASSWORD_BREACH_CHECK_ENABLED default off). Dependency failure fails
	// open with a breachCheck=unverified audit marker (用户裁定 G4-ab).
	BreachChecker BreachChecker
	// Federation Trust (ADR-028): lazy 注入 — nil 时 resolveJWTSubjectToken 跳过
	// federation lookup，保持原有本地 JWT 路径不变。
	federationResolver FederationTrustResolver
	federationVerifier ExternalJWTVerifier
	// IssuerURL is the configured OIDC issuer (ISSUER_URL). It is the fallback for
	// call sites that have no request to derive the issuer from, such as the
	// back-channel logout fan-out that runs after the response is written.
	IssuerURL string
	// CasbinPolicyManager resolves real Casbin p-policies for role actors.
	// Nil → permissions_query.go falls back to the hard-coded role semantics.
	CasbinPolicyManager domainpermissions.CasbinPolicyManager
	// ProviderTokenCipher seals the upstream OAuth access token kept in the
	// session for provider logout. Nil falls back to the platform encryptor
	// when a KEK is configured; without one the token is not retained.
	ProviderTokenCipher ProviderTokenCipher
	// socialOIDCStates binds generic OIDC nonce/PKCE values to an authorization
	// state. Nil uses the process-wide store; tests inject an isolated one.
	socialOIDCStates *socialOIDCStateStore

	// permissionCache memoizes GetCurrentUserPermissions results for 5 minutes
	// per (orgSlug, username). The cache key is "orgSlug|username". Stale entries
	// expire via cachedPermission.expiresAt; there is no background GC, so the
	// map only grows as long as distinct (org, user) pairs access it.
	permissionCache *sync.Map
}

// ProviderTokenCipher seals and opens the upstream provider access token that
// is retained for custom logout. It matches platform/crypto.FieldEncryptor.
type ProviderTokenCipher interface {
	EncryptField(plaintext string) (string, error)
	DecryptField(ciphertext string) (string, error)
}

type AccountQuery struct {
	OrganizationSlug string
	Username         string
	AccessToken      string
}

type LoginCommand struct {
	OrganizationSlug string
	Username         string
	Password         string
	ApplicationID    string
	ChallengeID      string
	MfaType          string
	Passcode         string
	RecoveryCode     string
	// VerificationCode and VerificationType enable the email/SMS one-time-code
	// login channel. They are optional and mutually exclusive with Password;
	// an empty VerificationCode keeps the password flow unchanged.
	VerificationCode string
	VerificationType string // "email" or "sms"
	// CaptchaToken/CaptchaID carry the human-check response. Login enforces
	// them through EnforceCaptcha once the organization is resolved, mirroring
	// the signup anti-bot gate; the MFA-completion leg (challengeId set) is
	// exempt because it already passed the gate on the first leg.
	CaptchaToken string
	CaptchaID    string
	// TrustedDeviceCredentialID identifies a device registered through F1. When
	// it matches an active trust row (same user, org and UA hash) the login skips
	// the MFA step; otherwise the normal MFA flow runs unchanged.
	//
	// 请求体来源为回退路径，移除期限 2027-05-04：到期后删除该字段与
	// LoginRequest 的请求体解析分支，cookie 成为唯一来源。
	TrustedDeviceCredentialID string
	// TrustedDeviceCredentialSource 记录凭据来源："cookie"（优先）或 "body"
	// （回退期），用于冲突审计。请求体回退移除期限 2027-05-04。
	TrustedDeviceCredentialSource string
	// TrustedDeviceCredentialConflict 为真表示 cookie 与请求体同时提供且值不同。
	// 以 cookie 为准，并通过独立事务写入 auth.trusted_device.credential_conflict 审计。
	TrustedDeviceCredentialConflict bool
	// RememberMeToken is an optional single-use token minted by
	// remember-me/issue. On a login that requires MFA it substitutes the
	// passcode, skipping the second factor and rotating the token.
	RememberMeToken   string
	EnableMfaRemember bool
	IPAddress         string
	UserAgent         string
	// HandleLoggedIn parameters
	ResponseType string
	RedirectURI  string
	State        string
	Nonce        string
	Scope        []string
	// PKCE and target-resource binding for the authorization-code response type.
	// They travel with the login request because the login endpoint is the other
	// place that issues an authorization code (设计报告 14.6.4).
	CodeChallenge     string
	PreviousSessionID string
	ChallengeMethod   string
	Resource          string
	SAMLRequest       string
	RelayState        string
	CASService        string
}

type LoginChallengeCommand struct {
	ChallengeID string
	// SessionUserID and SessionOrganizationID identify the authenticated caller
	// for approve/reject. A login challenge may only be acted on by the user it
	// was minted for; the command carries the session principal so the
	// application layer can enforce that ownership instead of trusting the
	// challenge ID alone.
	SessionUserID         string
	SessionOrganizationID string
}

type LogoutCommand struct {
	SessionID string
	// IssuerURL is the issuer the logout token must declare. It comes from the
	// request so logout_token.iss equals discovery's issuer (设计报告 14.8).
	IssuerURL string
}

type AuthorizeCommand struct {
	OrganizationSlug string
	Username         string
	ClientID         string
	RedirectURI      string
	ResponseType     string
	RequestedScopes  []string
	State            string
	Nonce            string
	CodeChallenge    string
	ChallengeMethod  string
	Resource         string
	// SessionID is the OIDC session identifier of the session this request was
	// authorized for. It is stamped onto the authorization code so the token
	// endpoint can put it in the ID token (设计报告 14.12.3).
	SessionID string
	// AuthTime is the session's establishment time, i.e. when the user actually
	// authenticated. The transport fills it from the resolved session; leaving it
	// zero makes the code fall back to the issue time (设计报告 14.8).
	AuthTime time.Time
}

type ExchangeAuthorizationCodeCommand struct {
	GrantType    string
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	CodeVerifier string
	Resource     string
	DPoPProof    string
	// ClientAuthMethod 是本次请求实际使用的客户端认证方式
	// （client_secret_basic / client_secret_post / private_key_jwt /
	// tls_client_auth / self_signed_tls_client_auth / none）。
	// 由传输层判定；空值表示没有任何凭据。
	ClientAuthMethod    string
	ClientAssertion     string
	ClientAssertionType string
	// DPoPURL is the absolute URL of the token endpoint the proof was made for,
	// i.e. the `htu` the proof must declare. It is required whenever DPoPProof is
	// set: RFC 9449 §4.3 step 5 makes matching htu mandatory, and passing an empty
	// string here made every dpop_required application fail the comparison, so no
	// such application could complete the exchange at all.
	DPoPURL string
	// IssuerURL is the issuer the ID token must declare, resolved from the same
	// source as the discovery document (设计报告 14.8).
	IssuerURL string
	// ClientCertificateThumbprint is the RFC 8705 x5t#S256 of the TLS client
	// certificate, when the transport observed one. It turns the issued token
	// into a certificate-bound token (cnf.x5t#S256).
	ClientCertificateThumbprint string
	// PeerCertificates is the client certificate chain from the in-process TLS
	// handshake. tls_client_auth / self_signed_tls_client_auth use it as their
	// only credential; reverse-proxy certificate headers are never consulted.
	PeerCertificates []*x509.Certificate
}

type GrantConsentCommand struct {
	OrganizationSlug string
	Username         string
	ApplicationID    string
	GrantedScopes    []string
}

type EvaluateConsentCommand struct {
	OrganizationSlug string
	Username         string
	ApplicationID    string
	RequestedScopes  []string
}

type RevokeConsentCommand struct {
	OrganizationSlug string
	Username         string
	ApplicationID    string
	GrantedScopes    []string
	// CurrentSessionID lets revokeSessionsForRevokedConsent skip the very
	// session the user is making the revocation from — they shouldn't
	// silently get kicked out (ADR-027 consent revocation cascade).
	CurrentSessionID string
}

// ExitImpersonate revokes the impersonation tokens for the given user, ending an
// impersonation session. It must only touch tokens minted by ImpersonateUser;
// revoking by user alone also kills the user's own sessions, which is a forced
// logout rather than an exit-impersonate.
func (s *Service) ExitImpersonate(ctx context.Context, orgSlug, username string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx uow.UnitOfWork) error {
		org, err := tx.Organizations().GetBySlug(ctx, orgSlug)
		if err != nil {
			return err
		}
		user, err := tx.Users().GetByUsername(ctx, org.ID, username)
		if err != nil {
			return err
		}
		// Impersonation tokens carry the "imp" prefix (set by ImpersonateUser).
		// The organization predicate is included as well: access_tokens has no
		// organization_id column, so RevokeByQuery binds it through the users
		// join, and MySQL (no RLS) needs that explicit tenant boundary.
		_, err = tx.Tokens().RevokeByQuery(ctx, tokendomain.RevokeQuery{
			OrganizationID: org.ID,
			UserID:         user.ID,
			TokenPrefix:    "imp",
		}, s.Clock.Now())
		if err != nil {
			return apperrors.NewInternal("revoke impersonation tokens")
		}

		// 审计：终止 impersonation 会话（action=auth.exit_impersonate，resourceType=tokens）
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "authentication", "", "",
			org.ID, "",
			"auth.exit_impersonate", "tokens", user.ID,
			"user", "auth.exit_impersonate",
			map[string]any{
				"username": username,
			},
		); err != nil {
			return err
		}
		return nil
	})
}
