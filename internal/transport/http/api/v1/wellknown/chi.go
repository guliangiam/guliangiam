package wellknown

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"strings"

	"crypto/x509"

	"github.com/go-chi/chi/v5"
	appauth "github.com/guliangiam/guliangiam/internal/app/auth"
	appkeys "github.com/guliangiam/guliangiam/internal/app/keys"
	appscim "github.com/guliangiam/guliangiam/internal/app/scim"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	platformdpop "github.com/guliangiam/guliangiam/internal/platform/dpop"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformjwks "github.com/guliangiam/guliangiam/internal/platform/jwks"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// OIDCDiscoveryFetcher fetches OIDC Discovery documents.
type OIDCDiscoveryFetcher interface {
	GetOIDCDiscovery(ctx context.Context, issuerURL string) (*appauth.OIDCDiscoveryResponse, error)
	GetOIDCDiscoveryByApplication(ctx context.Context, issuerURL string, applicationID string) (*appauth.OIDCDiscoveryResponse, error)
	ResolveOIDCApplication(ctx context.Context, reference string) (*appauth.OIDCApplication, error)
}

// UserInfoFetcher fetches OIDC UserInfo from an access token.
type UserInfoFetcher interface {
	GetUserInfo(ctx context.Context, accessToken string) (*appauth.UserInfoResponse, error)
}

// Handler handles .well-known and OAuth2 protocol endpoints.
type Handler struct {
	JWKSManager        appkeys.JWKSProvider
	IssuerURL          string
	UserInfoFetch      UserInfoFetcher
	OIDCDiscoveryFetch OIDCDiscoveryFetcher
	SAMLMetadataFetch  SAMLMetadataFetcher
	KeyService         *appkeys.Service
}

// SAMLMetadataFetcher resolves SAML IdP metadata for an application.
type SAMLMetadataFetcher interface {
	GetSAMLMetadata(ctx context.Context, organizationSlug string, applicationID string) ([]byte, error)
}

// RegisterChiRoutes registers .well-known and OAuth2 protocol routes.
func RegisterChiRoutes(router chi.Router, handler Handler) {
	// ─────────────────────────────────────────────
	// OIDC Discovery (global)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		base, err := handler.issuerBase(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.oidcDiscoveryFetch(r.Context(), base)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// OIDC Discovery (per-application)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/{application}/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		applicationID := chi.URLParam(r, "application")
		base, err := handler.issuerBase(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		response, err := handler.oidcDiscoveryByApplication(r.Context(), base, applicationID)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// JWKS (global)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.jwksEndpoint()
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=3600")
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// JWKS (国密 SM2-SM3, dedicated extension channel)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/jwks-gm.json", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.gmJwksEndpoint()
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=3600")
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// JWKS (per-application)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/{application}/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		appSlug := chi.URLParam(r, "application")
		response, err := handler.appJwksEndpoint(r, appSlug)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=3600")
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// WebFinger (RFC 7033)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/webfinger", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.webFinger(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// OAuth 2.0 Authorization Server Metadata (RFC 8414)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.oauthAuthorizationServer(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// OAuth 2.0 Protected Resource Metadata (RFC 9728)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		response, err := handler.protectedResourceMetadata(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	router.Get("/.well-known/{application}/oauth-protected-resource", func(w http.ResponseWriter, r *http.Request) {
		// The per-application variant must actually resolve the path parameter;
		// returning the global document here would be a silent contract mismatch.
		appSlug := chi.URLParam(r, "application")
		if strings.TrimSpace(appSlug) == "" {
			route.WriteError(w, r, errBadRequest("application is required"))
			return
		}
		response, err := handler.protectedResourceMetadataByApplication(r, appSlug)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		route.WriteJSON(w, http.StatusOK, response)
	})

	// ─────────────────────────────────────────────
	// SAML metadata (RFC-less discovery convenience endpoint)
	// ─────────────────────────────────────────────

	router.Get("/.well-known/saml-metadata", func(w http.ResponseWriter, r *http.Request) {
		metadata, err := handler.samlMetadata(r)
		if err != nil {
			route.WriteError(w, r, err)
			return
		}
		w.Header().Set("Content-Type", "application/samlmetadata+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(metadata)
	})

	// ─────────────────────────────────────────────
	// SCIM service provider configuration discovery
	// ─────────────────────────────────────────────

	router.Get("/.well-known/scim-configuration", func(w http.ResponseWriter, r *http.Request) {
		// Discovery documents carry no user data; the SCIM data family itself
		// remains Bearer-authenticated (AUTH-072).
		appscim.HandleServiceProviderConfig(w, r)
	})

	// ─────────────────────────────────────────────
	// OIDC UserInfo
	// ─────────────────────────────────────────────

	router.Get("/oauth2/userinfo", func(w http.ResponseWriter, r *http.Request) {
		accessToken := bearerTokenFromRequest(r)
		if accessToken == "" {
			// RFC 6750 §3.1: a resource server answers a missing token with the
			// protocol envelope and a WWW-Authenticate challenge. The challenge must
			// be set first — a header written after WriteHeader is dropped.
			route.WriteBearerChallenge(w, route.OAuthErrInvalidToken)
			route.WriteOAuthError(w, http.StatusUnauthorized, route.OAuthErrInvalidToken, "")
			return
		}

		// token/user 查询的 System 只读定位与租户回钉在 app 层 GetUserInfo 内完成
		// （INC-BE-05），transport 不做请求级 RLS 提升。
		response, err := handler.UserInfoFetch.GetUserInfo(r.Context(), accessToken)
		if err != nil {
			// RFC 6750 §3.1: a protected resource answers a bad token with
			// `invalid_token`, never `insufficient_scope`, whichever code the
			// service raised. WriteResourceError keeps the code fixed and still
			// refuses to echo an internal message into error_description, which
			// the hand-written form here used to do verbatim.
			route.WriteResourceError(w, err, route.OAuthErrInvalidToken, http.StatusUnauthorized)
			return
		}

		route.WriteJSON(w, http.StatusOK, response)
	})
}

// issuerBase resolves the issuer identifier the discovery-family documents
// advertise. Precedence is handler configuration, then ISSUER_URL, and only
// then the request. Production must configure ISSUER_URL: deriving the issuer
// from a request header would let any client choose the value an RP caches.
//
// X-Forwarded-Host and X-Forwarded-Proto are honoured outside production only
// when the peer is listed in TRUSTED_PROXY_CIDRS — the same trust boundary the
// client-IP resolver uses (security-model S13). Without that configuration the
// headers are ignored and the request Host/scheme are the fallback, so a forged
// header cannot rewrite the issuer.
func (h Handler) issuerBase(r *http.Request) (string, error) {
	if configured := strings.TrimRight(strings.TrimSpace(h.IssuerURL), "/"); configured != "" {
		return configured, nil
	}
	if configured := platformconfig.ConfiguredIssuerURL(); configured != "" {
		return configured, nil
	}
	if platformconfig.IsProduction() {
		return "", apperrors.NewInternal("ISSUER_URL is required in production")
	}

	// One derivation for every request-derived issuer: X-Forwarded-Proto and
	// X-Forwarded-Host are judged by the platformconfig trust helpers, which
	// RequestIssuerURL (and therefore DPoP `htu`) also uses.
	scheme := platformconfig.TrustedForwardedProto(r)
	host := r.Host
	if forwarded := platformconfig.TrustedForwardedHost(r); forwarded != "" {
		host = forwarded
	}

	return scheme + "://" + host, nil
}

func (h Handler) oidcDiscoveryFetch(ctx context.Context, issuerURL string) (*appauth.OIDCDiscoveryResponse, error) {
	if h.OIDCDiscoveryFetch != nil {
		return h.OIDCDiscoveryFetch.GetOIDCDiscovery(ctx, issuerURL)
	}
	// Fallback: build a basic discovery document.
	return &appauth.OIDCDiscoveryResponse{
		Issuer:                 issuerURL,
		AuthorizationEndpoint:  issuerURL + "/api/v1/auth/authorize",
		TokenEndpoint:          issuerURL + appauth.TokenEndpointPath,
		UserinfoEndpoint:       issuerURL + "/oauth2/userinfo",
		JWKSEndpoint:           issuerURL + "/.well-known/jwks.json",
		RegistrationEndpoint:   issuerURL + "/oauth2/register",
		ScopesSupported:        []string{"openid", "profile", "email", "phone", "address", "offline_access"},
		ResponseTypesSupported: []string{"code"},
		ResponseModesSupported: []string{"query", "fragment"},
		// client_credentials is deliberately absent: it answers
		// unsupported_grant_type until token subjects are modeled as principals,
		// and advertising a grant that cannot succeed misleads client libraries.
		GrantTypesSupported: []string{
			"authorization_code",
			"refresh_token",
			"urn:ietf:params:oauth:grant-type:device_code",
			"urn:ietf:params:oauth:grant-type:token-exchange",
		},
		SubjectTypesSupported: []string{"public"},
		IDTokenSigningAlgValuesSupported: append(
			[]string(nil), platformjwks.IDTokenSigningAlgorithms...,
		),
		GMJWKSEndpoint:                    issuerURL + "/.well-known/jwks-gm.json",
		GMAlgValuesSupported:              []string{platformjwks.SigningAlgorithmSM2SM3},
		TokenEndpointAuthMethodsSupported: append([]string(nil), appauth.TokenEndpointAuthMethodsSupported...),
		ClaimsSupported:                   []string{"sub", "username", "name", "email", "email_verified", "phone_number", "picture", "org"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		IntrospectionEndpoint:             issuerURL + "/api/v1/auth/oauth2/introspect",
		RevocationEndpoint:                issuerURL + "/api/v1/auth/oauth2/revoke",
		EndSessionEndpoint:                issuerURL + "/api/v1/auth/logout",
		DeviceAuthorizationEndpoint:       issuerURL + "/api/v1/auth/device/authorize",
		// Same source as the validator's whitelist and as the app-layer document,
		// so this fallback cannot announce a set the server would reject
		// (设计报告 14.27.1).
		DPoPSigningAlgValuesSupported:      append([]string(nil), platformdpop.SupportedAlgorithms...),
		BackchannelLogoutSupported:         true,
		BackchannelLogoutSessionSupported:  true,
		FrontchannelLogoutSupported:        true,
		FrontchannelLogoutSessionSupported: true,
	}, nil
}

func (h Handler) oidcDiscoveryByApplication(ctx context.Context, issuerURL string, applicationID string) (*appauth.OIDCDiscoveryResponse, error) {
	if h.OIDCDiscoveryFetch != nil {
		return h.OIDCDiscoveryFetch.GetOIDCDiscoveryByApplication(ctx, issuerURL, applicationID)
	}
	// Fallback: return the same global discovery document.
	return h.oidcDiscoveryFetch(ctx, issuerURL)
}

func (h Handler) oauthAuthorizationServer(r *http.Request) (*OAuthAuthorizationServerDocument, error) {
	base, err := h.issuerBase(r)
	if err != nil {
		return nil, err
	}

	return &OAuthAuthorizationServerDocument{
		Issuer:                            base,
		AuthorizationEndpoint:             base + "/api/v1/auth/authorize",
		TokenEndpoint:                     base + appauth.TokenEndpointPath,
		RegistrationEndpoint:              base + "/oauth2/register",
		JWKSEndpoint:                      base + "/.well-known/jwks.json",
		ScopesSupported:                   []string{"openid", "profile", "email", "phone", "address", "offline_access"},
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		TokenEndpointAuthMethodsSupported: append([]string(nil), appauth.TokenEndpointAuthMethodsSupported...),
		RevocationEndpoint:                base + "/api/v1/auth/oauth2/revoke",
		IntrospectionEndpoint:             base + "/api/v1/auth/oauth2/introspect",
		CodeChallengeMethodsSupported:     []string{"S256"},
	}, nil
}

func (h Handler) jwksEndpoint() (*appkeys.JWKS, error) {
	if h.JWKSManager == nil {
		return nil, apperrors.NewUnavailable("platform JWKS is not configured")
	}
	return h.JWKSManager.GetJWKS()
}

// gmJwksEndpoint publishes the 国密 SM2-SM3 key set on its own path. Standard
// JOSE clients never fetch this URI (discovery points them at jwks_uri), so the
// SM2 curve cannot break their key-set parsing; GM-capable clients read
// gm_jwks_uri from discovery to verify SM2-SM3 tokens by the gm_ kid prefix.
func (h Handler) gmJwksEndpoint() (*appkeys.JWKS, error) {
	if h.JWKSManager == nil {
		return nil, apperrors.NewUnavailable("platform JWKS is not configured")
	}
	return h.JWKSManager.GetGMJWKS()
}

// appJwksEndpoint publishes the application-scoped JWKS. Every failure is
// explicit: an unknown application or an unusable application key is a 404, and
// a dependency that cannot answer (unwired service, repository failure,
// platform JWKS failure) is a 503. It never falls back to the global document:
// a caller that requested the application-specific URI and silently received
// the platform keys would verify application signatures against the wrong
// public key and fail without a diagnosable cause.
func (h Handler) appJwksEndpoint(r *http.Request, appSlug string) (*appkeys.JWKS, error) {
	if h.KeyService == nil {
		return nil, apperrors.NewUnavailable("application key service is not configured")
	}
	if h.OIDCDiscoveryFetch == nil {
		return nil, apperrors.NewUnavailable("application lookup is not configured")
	}

	application, err := h.OIDCDiscoveryFetch.ResolveOIDCApplication(r.Context(), appSlug)
	if err != nil {
		if isErrorCode(err, apperrors.CodeNotFound) {
			return nil, apperrors.NewNotFound("application not found")
		}
		return nil, apperrors.WrapUnavailable(err, "resolve application for JWKS")
	}
	if application == nil {
		return nil, apperrors.NewNotFound("application not found")
	}

	keyName := "app-signing-" + application.Slug
	key, err := h.KeyService.GetEnabledKeyByName(r.Context(), application.OrganizationID, keyName)
	if err != nil {
		if isErrorCode(err, apperrors.CodeNotFound) {
			return nil, apperrors.NewNotFound("application signing key not found")
		}
		return nil, apperrors.WrapUnavailable(err, "load application signing key")
	}

	pubKey, err := parseRSAPublicKeyPEM(key.PublicKey)
	if err != nil {
		return nil, apperrors.NewNotFound("application signing key is not a usable RSA public key")
	}

	// AC-CRYPTO-002: published kids are namespaced by signing algorithm family.
	// The application record is published as an RS256/RSA JWK, so its kid must
	// carry the rsa_ prefix; otherwise the application document would hand out a
	// kid that no family rule (and no RP prefix selector) can classify.
	kid, err := platformjwks.KidForAlgorithm(platformjwks.SigningAlgorithmRS256, key.ID)
	if err != nil {
		return nil, apperrors.WrapNotFound(err, "application signing key has no usable key id")
	}

	jwk := appkeys.JWK{
		Kty: "RSA",
		Use: "sig",
		Kid: kid,
		Alg: platformjwks.SigningAlgorithmRS256,
		N:   base64.RawURLEncoding.EncodeToString(pubKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pubKey.E)).Bytes()),
	}

	// AC-CRYPTO-002: the issuer signs with the platform keys, not with this
	// application key record, so an application-scoped document that carries
	// only the record leaves an RP without the rsa_/gm_ key the token names.
	// mergePlatformJWKS then keeps every platform kid canonical.
	platform, err := h.jwksEndpoint()
	if err != nil {
		return nil, apperrors.WrapUnavailable(err, "load platform JWKS")
	}
	return mergePlatformJWKS(&appkeys.JWKS{Keys: []appkeys.JWK{jwk}}, platform), nil
}

func isErrorCode(err error, code apperrors.Code) bool {
	var appErr *apperrors.Error
	return errors.As(err, &appErr) && appErr.Code == code
}

// mergePlatformJWKS appends the platform signing keys to an application JWKS.
// A platform key is canonical: the same public key must be published under one
// kid at every endpoint, so an application entry whose key material matches a
// platform entry is omitted instead of appearing under a second kid. An
// application entry whose kid is already owned by a platform entry is likewise
// omitted, because two different keys must never share one kid. Remaining
// application entries keep their position and value, and platform entries
// always follow unchanged.
func mergePlatformJWKS(app, platform *appkeys.JWKS) *appkeys.JWKS {
	if app == nil {
		app = &appkeys.JWKS{}
	}
	if platform == nil || len(platform.Keys) == 0 {
		return app
	}

	platformKids := make(map[string]struct{}, len(platform.Keys))
	platformMaterials := make(map[string]struct{}, len(platform.Keys))
	for _, key := range platform.Keys {
		platformKids[key.Kid] = struct{}{}
		if material := jwkMaterialKey(key); material != "" {
			platformMaterials[material] = struct{}{}
		}
	}

	merged := make([]appkeys.JWK, 0, len(app.Keys)+len(platform.Keys))
	seen := make(map[string]struct{}, len(app.Keys))
	for _, key := range app.Keys {
		if _, ok := platformKids[key.Kid]; ok {
			continue
		}
		if _, ok := seen[key.Kid]; ok {
			continue
		}
		if material := jwkMaterialKey(key); material != "" {
			if _, ok := platformMaterials[material]; ok {
				continue
			}
		}
		seen[key.Kid] = struct{}{}
		merged = append(merged, key)
	}
	for _, key := range platform.Keys {
		merged = append(merged, key)
	}
	return &appkeys.JWKS{Keys: merged}
}

// jwkMaterialKey identifies a JWK by its public key material, not its kid. It
// returns an empty string for a JWK without complete material, which callers
// must treat as non-deduplicable.
func jwkMaterialKey(key appkeys.JWK) string {
	switch key.Kty {
	case "RSA":
		if key.N == "" || key.E == "" {
			return ""
		}
		return "RSA|" + key.N + "|" + key.E
	case "EC":
		if key.Crv == "" || key.X == "" || key.Y == "" {
			return ""
		}
		return "EC|" + key.Crv + "|" + key.X + "|" + key.Y
	default:
		return ""
	}
}

// parseRSAPublicKeyPEM parses a PEM-encoded RSA public key.
func parseRSAPublicKeyPEM(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errBadRequest("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errBadRequest("key is not RSA")
	}

	return rsaPub, nil
}

func (h Handler) webFinger(r *http.Request) (*WebFingerResponse, error) {
	resource := strings.TrimSpace(r.URL.Query().Get("resource"))
	if resource == "" {
		return nil, errBadRequest("resource parameter is required")
	}

	rel := strings.TrimSpace(r.URL.Query().Get("rel"))
	if rel != "" && rel != "http://openid.net/specs/connect/1.0/issuer" {
		return nil, errBadRequest("unsupported rel value")
	}

	base, err := h.issuerBase(r)
	if err != nil {
		return nil, err
	}

	return &WebFingerResponse{
		Subject: resource,
		Links: []WebFingerLink{
			{
				Rel:  "http://openid.net/specs/connect/1.0/issuer",
				Href: base,
			},
		},
	}, nil
}

// protectedResourceMetadata returns RFC 9728 OAuth 2.0 Protected Resource Metadata.
func (h Handler) protectedResourceMetadata(r *http.Request) (*ProtectedResourceMetadata, error) {
	base, err := h.issuerBase(r)
	if err != nil {
		return nil, err
	}

	return &ProtectedResourceMetadata{
		Resource:                          base + "/oauth2/userinfo",
		AuthorizationServers:              []string{base},
		ScopesSupported:                   []string{"openid", "profile", "email", "phone", "address"},
		BearerMethodsSupported:            []string{"header"},
		ResourceSigningAlgValuesSupported: []string{"RS256"},
	}, nil
}

// protectedResourceMetadataByApplication returns RFC 9728 metadata scoped to a
// single application: the resource identifier and scopes are those advertised by
// the application's own discovery document.
func (h Handler) protectedResourceMetadataByApplication(r *http.Request, appSlug string) (*ProtectedResourceMetadata, error) {
	base, err := h.issuerBase(r)
	if err != nil {
		return nil, err
	}

	scopes := []string{"openid", "profile", "email", "phone", "address"}
	if discovery, discoveryErr := h.oidcDiscoveryByApplication(r.Context(), base, appSlug); discoveryErr == nil && discovery != nil && len(discovery.ScopesSupported) > 0 {
		scopes = discovery.ScopesSupported
	}

	return &ProtectedResourceMetadata{
		Resource:                          base + "/.well-known/" + appSlug + "/openid-configuration",
		AuthorizationServers:              []string{base},
		ScopesSupported:                   scopes,
		BearerMethodsSupported:            []string{"header"},
		ResourceSigningAlgValuesSupported: []string{"RS256"},
	}, nil
}

// samlMetadata resolves SAML IdP metadata for the application named by the
// `application` (or `applicationId`) query parameter, defaulting to the
// organization's only SAML-enabled application when unambiguous.
func (h Handler) samlMetadata(r *http.Request) ([]byte, error) {
	if h.SAMLMetadataFetch == nil {
		return nil, errBadRequest("SAML metadata is not configured")
	}

	organizationSlug := strings.TrimSpace(r.URL.Query().Get("organization"))
	applicationID := strings.TrimSpace(r.URL.Query().Get("application"))
	if applicationID == "" {
		applicationID = strings.TrimSpace(r.URL.Query().Get("applicationId"))
	}
	if organizationSlug == "" || applicationID == "" {
		return nil, errBadRequest("organization and application query parameters are required")
	}

	metadata, err := h.SAMLMetadataFetch.GetSAMLMetadata(r.Context(), organizationSlug, applicationID)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

func bearerTokenFromRequest(r *http.Request) string {
	authorization := strings.TrimSpace(r.Header.Get("Authorization"))
	if authorization == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return strings.TrimSpace(authorization[7:])
	}
	return ""
}

// OAuthAuthorizationServerDocument is the RFC 8414 authorization server metadata.
type OAuthAuthorizationServerDocument struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	JWKSEndpoint                      string   `json:"jwks_uri"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
}

// ProtectedResourceMetadata is the RFC 9728 OAuth 2.0 Protected Resource Metadata.
type ProtectedResourceMetadata struct {
	Resource                          string   `json:"resource"`
	AuthorizationServers              []string `json:"authorization_servers"`
	ScopesSupported                   []string `json:"scopes_supported"`
	BearerMethodsSupported            []string `json:"bearer_methods_supported"`
	ResourceSigningAlgValuesSupported []string `json:"resource_signing_alg_values_supported"`
}

// WebFingerResponse is the RFC 7033 WebFinger response.
type WebFingerResponse struct {
	Subject string          `json:"subject"`
	Links   []WebFingerLink `json:"links"`
}

// WebFingerLink is a single link in a WebFinger response.
type WebFingerLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// errBadRequest builds the 400 an invalid discovery/protected-resource request
// must produce. The protocol face renders through route.WriteError →
// OAuthErrorFrom, and a bare error would fall through to 500 `server_error`;
// the application namespace yields the intended 400 + `invalid_request`
// (手册 4.1.3 三面表 / 错误码注册表 §2).
func errBadRequest(msg string) error {
	return apperrors.NewInvalidArgument(msg)
}
