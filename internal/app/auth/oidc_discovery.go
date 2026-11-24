package auth

import (
	"context"
	"strings"

	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	platformdpop "github.com/guliangiam/guliangiam/internal/platform/dpop"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
)

// TokenEndpointPath is the one path the authorization-server token endpoint is
// registered at (internal/transport/http/api/v1/auth/chi_protocol.go). Discovery,
// the well-known fallback and the DPoP `htu` all derive from this constant so a
// client that follows the published document cannot land on a 404.
const TokenEndpointPath = "/api/v1/auth/tokens/exchange"

// OIDCApplication is the application identity a per-application discovery/JWKS
// request resolves to. Only the fields the protocol needs are exposed.
type OIDCApplication struct {
	ID             string
	Slug           string
	ClientID       string
	OrganizationID string
}

// OIDCDiscoveryResponse represents the OIDC Discovery document
// as defined in OpenID Connect Discovery 1.0.
type OIDCDiscoveryResponse struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	UserinfoEndpoint                 string   `json:"userinfo_endpoint"`
	JWKSEndpoint                     string   `json:"jwks_uri"`
	RegistrationEndpoint             string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                  []string `json:"scopes_supported"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	ResponseModesSupported           []string `json:"response_modes_supported"`
	GrantTypesSupported              []string `json:"grant_types_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	// GMJWKSEndpoint is the GuliangIAM extension pointer to the dedicated 国密
	// JWKS channel. Standard clients only read jwks_uri (international keys);
	// GM-capable clients read gm_jwks_uri to fetch SM2-SM3 public keys.
	GMJWKSEndpoint string `json:"gm_jwks_uri,omitempty"`
	// GMAlgValuesSupported announces the 国密 signing algorithms whose public
	// keys are published at gm_jwks_uri.
	GMAlgValuesSupported              []string `json:"gm_alg_values_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	EndSessionEndpoint                string   `json:"end_session_endpoint"`
	DeviceAuthorizationEndpoint       string   `json:"device_authorization_endpoint,omitempty"`
	// DPoPSigningAlgValuesSupported announces the algorithms a DPoP proof may
	// use (RFC 9449 §5.1). Its value comes from the same slice the validator
	// enforces, so the advertised set cannot drift from the accepted one — the
	// drift 14.27.1 records as part of.
	DPoPSigningAlgValuesSupported []string `json:"dpop_signing_alg_values_supported,omitempty"`
	// BackchannelLogoutSupported announces that this OP delivers logout tokens to
	// a relying party's registered back-channel URI (OpenID Connect Back-Channel
	// Logout 1.0 §2).
	BackchannelLogoutSupported bool `json:"backchannel_logout_supported,omitempty"`
	// BackchannelLogoutSessionSupported announces that those tokens carry a `sid`,
	// i.e. that a relying party can destroy one session instead of all of them.
	// It is true only because the sid is now a real per-session identifier rather
	// than the internal primary key (设计报告 14.12.3).
	BackchannelLogoutSessionSupported bool `json:"backchannel_logout_session_supported,omitempty"`
	// FrontchannelLogoutSupported announces the OP front-channel logout page.
	FrontchannelLogoutSupported bool `json:"frontchannel_logout_supported,omitempty"`
	// FrontchannelLogoutSessionSupported announces that sid is included when the
	// application requires session identification.
	FrontchannelLogoutSessionSupported bool `json:"frontchannel_logout_session_supported,omitempty"`
}

// UserInfoResponse represents the OIDC UserInfo claims.
type UserInfoResponse struct {
	Sub           string `json:"sub"`
	Username      string `json:"username,omitempty"`
	DisplayName   string `json:"name,omitempty"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	Phone         string `json:"phone_number,omitempty"`
	PhoneVerified bool   `json:"phone_number_verified,omitempty"`
	AvatarURL     string `json:"picture,omitempty"`
	Organization  string `json:"org,omitempty"`
}

// TokenEndpointAuthMethodsSupported is the single source for the discovery
// document and the well-known fallback. It must list exactly the methods the
// token endpoint verifies today: none, the two client-secret methods,
// private_key_jwt and the two RFC 8705 mTLS methods. client_secret_jwt stays
// absent because the repo has no DB-backed key facility (OPEN-API-60).
var TokenEndpointAuthMethodsSupported = []string{
	applicationdomain.TokenEndpointAuthMethodNone,
	applicationdomain.TokenEndpointAuthMethodClientSecretBasic,
	applicationdomain.TokenEndpointAuthMethodClientSecretPost,
	applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT,
	applicationdomain.TokenEndpointAuthMethodTLSClientAuth,
	applicationdomain.TokenEndpointAuthMethodSelfSignedTLSClientAuth,
}

// GetOIDCDiscovery returns the OIDC Discovery document for this issuer.
func (s *Service) GetOIDCDiscovery(ctx context.Context, issuerURL string) (*OIDCDiscoveryResponse, error) {
	base := strings.TrimRight(issuerURL, "/")

	discovery := &OIDCDiscoveryResponse{
		Issuer:                base,
		AuthorizationEndpoint: base + "/api/v1/auth/authorize",
		TokenEndpoint:         base + TokenEndpointPath,
		UserinfoEndpoint:      base + "/oauth2/userinfo",
		JWKSEndpoint:          base + "/.well-known/jwks.json",
		RegistrationEndpoint:  base + "/oauth2/register",
		ScopesSupported: []string{
			scopeOpenID,
			scopeProfile,
			scopeEmail,
			scopePhone,
			"address",
			scopeOfflineAccess,
		},
		ResponseTypesSupported: []string{
			fieldCode,
		},
		ResponseModesSupported: []string{
			"query",
			"fragment",
		},
		// client_credentials is deliberately absent: no principal type can be
		// persisted as a token subject yet, so the grant answers
		// unsupported_grant_type. Announcing it would send client libraries into
		// a call that is guaranteed to fail (see the Principal model ADR).
		GrantTypesSupported: append([]string{}, discoveryGrantTypesSupported...),
		SubjectTypesSupported: []string{
			"public",
		},
		// Same single source as the well-known fallback: discovery must not
		// announce a signing set the issuer does not implement.
		IDTokenSigningAlgValuesSupported: append([]string(nil), platformjwks.IDTokenSigningAlgorithms...),
		// 国密 extension channel: the standard jwks_uri carries only international
		// keys, so SM2-SM3 public keys are advertised separately for GM clients.
		GMJWKSEndpoint:       base + "/.well-known/jwks-gm.json",
		GMAlgValuesSupported: []string{platformjwks.SigningAlgorithmSM2SM3},
		// Must list exactly the methods the token endpoint verifies. Adding a
		// method here before its verifier exists would publish a capability the
		// server rejects; client_secret_jwt is intentionally absent because the
		// repo has no DB-backed key facility (see OPEN-API-60).
		TokenEndpointAuthMethodsSupported: append([]string(nil), TokenEndpointAuthMethodsSupported...),
		ClaimsSupported: []string{
			"sub",
			"username",
			"name",
			scopeEmail,
			"email_verified",
			"phone_number",
			"phone_number_verified",
			"picture",
			"org",
		},
		CodeChallengeMethodsSupported: []string{
			"S256",
		},
		IntrospectionEndpoint:       base + "/api/v1/auth/oauth2/introspect",
		RevocationEndpoint:          base + "/api/v1/auth/oauth2/revoke",
		EndSessionEndpoint:          base + "/api/v1/auth/logout",
		DeviceAuthorizationEndpoint: base + "/api/v1/auth/device/authorize",
		// Copy the slice: the discovery document is serialized outside the
		// Service, and handing out the package-level slice would let a caller
		// mutate the validator's source of truth through a response struct.
		DPoPSigningAlgValuesSupported: append([]string(nil), platformdpop.SupportedAlgorithms...),
		// The OP does deliver logout tokens (14.12) and they do carry a per-session
		// sid, so both capability flags are honest. `frontchannel_logout_supported`
		// stays unannounced: the front-channel iframe delivery is still [Target]
		// (14.12.4 / T11), and advertising it would be the same kind of drift this
		// round is fixing.
		BackchannelLogoutSupported:         true,
		BackchannelLogoutSessionSupported:  true,
		FrontchannelLogoutSupported:        true,
		FrontchannelLogoutSessionSupported: true,
	}
	if s.JWKSManager != nil {
		if err := s.narrowSigningCapabilitiesToKeyRing(discovery); err != nil {
			return nil, err
		}
	}
	return discovery, nil
}

// narrowSigningCapabilitiesToKeyRing restricts the announced signing algorithms
// and the 国密 extension channel to what the keyring actually publishes
// (INC-BE-01). An RSA-only ring must not advertise gm_jwks_uri/SM2-SM3 and a
// GM-only ring must not advertise RS256: discovery and the JWKS endpoints read
// the same manager, so the document cannot drift from the published key set.
// A keyring with no publishable key fails closed here exactly like the JWKS
// endpoints do, instead of announcing capabilities the issuer cannot honor.
func (s *Service) narrowSigningCapabilitiesToKeyRing(discovery *OIDCDiscoveryResponse) error {
	international, err := s.JWKSManager.GetJWKS()
	if err != nil {
		return err
	}
	gm, err := s.JWKSManager.GetGMJWKS()
	if err != nil {
		return err
	}
	published := make(map[string]bool, len(international.Keys)+len(gm.Keys))
	for _, key := range international.Keys {
		published[key.Alg] = true
	}
	for _, key := range gm.Keys {
		published[key.Alg] = true
	}
	algorithms := make([]string, 0, len(platformjwks.IDTokenSigningAlgorithms))
	for _, alg := range platformjwks.IDTokenSigningAlgorithms {
		if published[alg] {
			algorithms = append(algorithms, alg)
		}
	}
	discovery.IDTokenSigningAlgValuesSupported = algorithms
	if !published[platformjwks.SigningAlgorithmSM2SM3] {
		discovery.GMJWKSEndpoint = ""
		discovery.GMAlgValuesSupported = nil
	}
	return nil
}

// ResolveOIDCApplication maps the `{application}` path parameter of a
// per-application discovery/JWKS request onto an application row.
//
// The protocol accepts the client_id and the application slug. A client_id is
// globally unique (partial unique index), so it is tried first. Slugs are only
// unique inside an organization, and the well-known URL carries no organization
// segment; the slug is therefore accepted only when exactly one application
// matches it. Zero matches and ambiguous matches are both a 404: returning the
// global document or guessing an organization would hand the caller a document
// for a different application.
func (s *Service) ResolveOIDCApplication(ctx context.Context, reference string) (*OIDCApplication, error) {
	ref := strings.TrimSpace(reference)
	if ref == "" {
		return nil, apperrors.NewInvalidArgument("application is required")
	}
	if s.UOW == nil {
		return nil, apperrors.NewUnavailable("application lookup is unavailable")
	}

	// The per-application well-known URL carries no organization segment. The
	// lookup is public metadata (the same information discovery already
	// publishes), and the tenant used for the subsequent key lookup comes from
	// the matched row — never from the request. A system-scoped, read-only
	// lookup is therefore what lets a client_id or slug resolve regardless of
	// which organization owns the application; without it the public request
	// would only see the built-in organization's rows.
	lookupCtx := tenantctx.System(ctx)
	apps := s.UOW.Applications()
	application, err := apps.GetByClientID(lookupCtx, ref)
	if err == nil {
		if application == nil {
			return nil, apperrors.NewNotFound("application not found")
		}
		return oidcApplicationView(application), nil
	}
	if !isNotFoundError(err) {
		return nil, err
	}

	const pageSize = 100
	matches := make([]*applicationdomain.Application, 0, 1)
	for page := 1; ; page++ {
		items, total, listErr := apps.List(lookupCtx, applicationdomain.ListQuery{
			Page: page, PageSize: pageSize, Keyword: ref,
		})
		if listErr != nil {
			return nil, listErr
		}
		for _, item := range items {
			if item != nil && strings.TrimSpace(item.Slug) == ref {
				matches = append(matches, item)
			}
		}
		if len(items) == 0 || int64(page*pageSize) >= total {
			break
		}
	}
	if len(matches) != 1 {
		return nil, apperrors.NewNotFound("application not found")
	}
	return oidcApplicationView(matches[0]), nil
}

func oidcApplicationView(application *applicationdomain.Application) *OIDCApplication {
	return &OIDCApplication{
		ID:             application.ID,
		Slug:           application.Slug,
		ClientID:       application.ClientID,
		OrganizationID: application.OrganizationID,
	}
}

// GetOIDCDiscoveryByApplication returns an OIDC Discovery document customized
// for a specific application. The document reflects the application's
// configured grant types, scopes, and signing algorithm.
func (s *Service) GetOIDCDiscoveryByApplication(ctx context.Context, issuerURL, applicationID string) (*OIDCDiscoveryResponse, error) {
	base := strings.TrimRight(issuerURL, "/")

	discovery, err := s.GetOIDCDiscovery(ctx, issuerURL)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(applicationID) == "" {
		return discovery, nil
	}

	application, err := s.ResolveOIDCApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}

	// Advertise the canonical slug even when the caller addressed the
	// application by client_id: both identifiers resolve, but a cached document
	// should not depend on which spelling happened to be used first.
	discovery.JWKSEndpoint = base + "/.well-known/" + application.Slug + "/jwks.json"
	return discovery, nil
}

// GetUserInfo returns OIDC UserInfo claims for the given user identified by access token.
//
// Per OIDC Core §5.3 the endpoint requires the `openid` scope — an access token
// that does not carry it is not an OIDC access token and must not unlock the
// user's profile. §5.4 then makes every non-`sub` claim conditional on the scope
// that owns it, and says an un-granted claim is omitted, not blanked: returning
// an empty string tells the client "the value is empty" when the truth is "you
// were not allowed to see it" (设计报告 14.9).
func (s *Service) GetUserInfo(ctx context.Context, accessToken string) (*UserInfoResponse, error) {
	if strings.TrimSpace(accessToken) == "" {
		return nil, apperrors.NewUnauthorized("access token is required")
	}

	tokenHash := platformcrypto.HashTokenValue(strings.TrimSpace(accessToken))
	// INC-BE-05：token 哈希定位是会话无关的只读查询，匿名请求在 fail-closed
	// RLS 下必须用 System 作用域；System 提升收敛在 app 层，transport 不提升。
	token, err := s.UOW.Tokens().GetByTokenHash(tenantctx.System(ctx), tokenHash)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid access token")
	}
	if token.RevokedAt != nil {
		return nil, apperrors.NewUnauthorized("access token has been revoked")
	}
	if !token.ExpiresAt.After(s.Clock.Now().UTC()) {
		return nil, apperrors.NewUnauthorized("access token has expired")
	}

	granted := newScopeSet(token.Scopes)
	if !granted.has(scopeOpenID) {
		return nil, apperrors.NewForbidden("access token was not issued for the openid scope")
	}

	// 用户读取回钉到 token 自身租户，而不是继续留在 System 作用域。
	userCtx := tenantctx.WithTenantScope(ctx, token.OrganizationID, "", tenantctx.TenantFilterStrict)
	user, err := s.UOW.Users().GetByID(userCtx, token.UserID)
	if err != nil {
		return nil, err
	}

	// `openid` guarantees only `sub`.
	response := &UserInfoResponse{Sub: user.ID}
	if granted.has(scopeProfile) {
		response.Username = user.Username
		response.DisplayName = user.Profile.DisplayName
		response.AvatarURL = user.Profile.AvatarURL
		response.Organization = token.OrganizationSlug
	}
	if granted.has(scopeEmail) {
		response.Email = user.Profile.Email
		response.EmailVerified = user.Profile.EmailVerified
	}
	if granted.has(scopePhone) {
		response.Phone = user.Profile.Phone
		response.PhoneVerified = user.Profile.PhoneVerified
	}
	return response, nil
}
