package applications

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	applicationdomain "github.com/guliangiam/guliangiam/internal/domain/applications"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Credential prefix constants — registered in contracts/registries/naming.yaml
const (
	clientIDPrefix      = "gl_cid_"
	clientSecretPrefix  = "gl_cs_"
	clientIDEntropy     = 24 // bytes → 32 chars after base64url
	clientSecretEntropy = 32 // bytes → 43 chars after base64url
)

// DCRRequest represents an RFC 7591 Dynamic Client Registration request.
type DCRRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	Scope                   string   `json:"scope"`
	ClientURI               string   `json:"client_uri"`
	LogoURI                 string   `json:"logo_uri"`
	OrganizationSlug        string   `json:"organization_slug"` // Extension: which org to register under
	// JWKS 是 private_key_jwt 的内嵌公钥集合；JWKSURI 指向远端 JWKS。
	// 注册边界只校验 URI 形状，拉取时的 SSRF/超时/缓存由 infra 执行。
	JWKS    json.RawMessage `json:"jwks"`
	JWKSURI string          `json:"jwks_uri"`
}

// DCRResponse represents an RFC 7591 Dynamic Client Registration response.
type DCRResponse struct {
	ClientID              string `json:"client_id"`
	ClientSecret          string `json:"client_secret,omitempty"`
	ClientIDIssuedAt      int64  `json:"client_id_issued_at"`
	ClientSecretExpiresAt int64  `json:"client_secret_expires_at"` // 0 = never expires
	// RegistrationAccessToken is the management credential RFC 7591 §3.2.1
	// requires for the read/update/delete operations on this client. Without it
	// the registration endpoints had no authentication at all (设计报告 14.28).
	RegistrationAccessToken string   `json:"registration_access_token,omitempty"`
	RegistrationClientURI   string   `json:"registration_client_uri,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	ClientName              string   `json:"client_name"`
	ClientURI               string   `json:"client_uri,omitempty"`
	LogoURI                 string   `json:"logo_uri,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
}

// RegisterDynamicClient implements RFC 7591 Dynamic Client Registration.
func (s *Service) RegisterDynamicClient(ctx context.Context, request DCRRequest) (*DCRResponse, error) {
	if err := validateDCRRequest(request); err != nil {
		return nil, err
	}
	parsedJWKS, jwksErr := parseOptionalClientJWKS(request.JWKS)
	if jwksErr != nil {
		return nil, newInvalidClientMetadataError("jwks is invalid")
	}
	jwksURI := strings.TrimSpace(request.JWKSURI)
	if err := validateClientJWKSURI(jwksURI); err != nil {
		return nil, err
	}
	// private_key_jwt needs verifiable material: an inline JWKS or a remote
	// jwks_uri. A client with neither could never authenticate at the token
	// endpoint (update has the same check).
	if strings.ToLower(strings.TrimSpace(request.TokenEndpointAuthMethod)) == applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT &&
		len(parsedJWKS) == 0 && jwksURI == "" {
		return nil, newInvalidClientMetadataError("jwks or jwks_uri is required for private_key_jwt")
	}

	// Resolve the organization.
	organization, err := s.UOW.Organizations().GetBySlug(ctx, request.OrganizationSlug)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("organization not found")
	}

	// Auto-generate client_id.
	clientID, err := generateOAuthClientID()
	if err != nil {
		return nil, apperrors.NewInternal("generate client_id")
	}

	// Determine the auth method before minting any credential: a public client
	// (`none`) must not receive an unused secret, and JWT/mTLS clients hold their
	// credentials elsewhere (JWKS / certificate).
	authMethod := strings.ToLower(strings.TrimSpace(request.TokenEndpointAuthMethod))
	if authMethod == "" {
		authMethod = applicationdomain.TokenEndpointAuthMethodClientSecretBasic
	}
	var clientSecret string
	if authMethod == applicationdomain.TokenEndpointAuthMethodClientSecretBasic ||
		authMethod == applicationdomain.TokenEndpointAuthMethodClientSecretPost {
		clientSecret, err = generateOAuthClientSecret()
		if err != nil {
			return nil, apperrors.NewInternal("generate client_secret")
		}
	}

	// Normalize grant types, defaulting to authorization_code if not specified.
	grantTypes := normalizeStringList(request.GrantTypes)
	if len(grantTypes) == 0 {
		grantTypes = []string{grantTypeAuthorizationCode, grantTypeRefreshToken}
	}

	// Normalize redirect URIs.
	redirectURIs := normalizeStringList(request.RedirectURIs)
	if len(redirectURIs) == 0 {
		return nil, apperrors.NewInvalidArgument("redirect_uris is required")
	}

	// Create the application using the existing factory.
	// slug = channel marker + clientID suffix. Slugs are resource identifiers
	// (see naming.yaml concepts:application), NOT credentials — the "零业务语义"
	// discipline in §credential_prefixes applies to client_id/client_secret only.
	slug := "dcr-" + clientID[:8]
	displayName := request.ClientName
	if displayName == "" {
		displayName = "DCR Client " + clientID[:8]
	}

	command := CreateApplicationCommand{
		OrganizationID:          organization.ID,
		Slug:                    slug,
		DisplayName:             displayName,
		Category:                "dcr",
		AppType:                 "web",
		Description:             "Dynamically registered OAuth2 client",
		HomepageURL:             request.ClientURI,
		Status:                  "active",
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		TokenEndpointAuthMethod: authMethod,
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		JWKS:                    request.JWKS,
	}

	application, clientSecretMaterial, err := s.NewApplicationWithSecret(command)
	if err != nil {
		return nil, err
	}

	// The management credential is generated before the row is written so the
	// hash lands in the same transaction as the client itself: a client that
	// exists without a credential would be permanently unmanageable.
	registrationAccessToken, err := generateRegistrationAccessToken()
	if err != nil {
		return nil, err
	}
	registrationTokenHash := platformcrypto.HashTokenValue(registrationAccessToken)

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if createErr := tx.Applications().Create(ctx, application); createErr != nil {
			return createErr
		}
		if clientSecretMaterial != nil {
			row, err := s.buildClientSecretRow(application.ID, *clientSecretMaterial, s.Clock.Now())
			if err != nil {
				return err
			}
			if err := tx.Applications().CreateClientSecret(ctx, application.OrganizationID, row); err != nil {
				return err
			}
		}
		if jwksURI != "" {
			uriRepo, ok := tx.Applications().(applicationdomain.ClientJWKSURIRepository)
			if !ok {
				return apperrors.NewInternal("client jwks_uri repository is not configured")
			}
			if err := uriRepo.SetClientJWKSURI(ctx, application.OrganizationID, application.ID, jwksURI, s.Clock.Now()); err != nil {
				return err
			}
		}
		// DCR 的 private_key_jwt 公钥与客户端同事务落库。
		if len(parsedJWKS) > 0 {
			now := s.Clock.Now()
			rows, err := s.buildClientJWKRows(application.ID, application.OrganizationID, parsedJWKS, now)
			if err != nil {
				return err
			}
			if err := tx.Applications().ReplaceClientJWKs(ctx, application.OrganizationID, application.ID, rows, now); err != nil {
				return err
			}
		}
		if err := tx.Applications().SetRegistrationAccessToken(ctx, application.ID, registrationTokenHash); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			"",
			"application.dynamic_client.created",
			application.ID,
			"application.dynamic_client.created",
			map[string]any{},
		); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			"",
			"application.dynamic_client.token_set",
			application.ID,
			"application.dynamic_client.token_set",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	response := &DCRResponse{
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		ClientIDIssuedAt:        now.Unix(),
		ClientSecretExpiresAt:   0, // Never expires
		RegistrationAccessToken: registrationAccessToken,
		RegistrationClientURI:   registrationClientURIPath(clientID),
		RedirectURIs:            redirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           normalizeStringList(request.ResponseTypes),
		TokenEndpointAuthMethod: authMethod,
		ClientName:              displayName,
		ClientURI:               request.ClientURI,
		LogoURI:                 request.LogoURI,
		Scope:                   request.Scope,
	}

	return response, nil
}

// GetDynamicClient implements RFC 7591 client read. registrationAccessToken is
// the Bearer credential presented by the caller and must match the one issued at
// registration (RFC 7591 §2.3, §3.2.1).
func (s *Service) GetDynamicClient(ctx context.Context, clientID, registrationAccessToken string) (*DCRResponse, error) {
	application, err := s.loadManageableDynamicClient(ctx, clientID, registrationAccessToken)
	if err != nil {
		return nil, err
	}

	return applicationToDCRResponse(application), nil
}

// UpdateDynamicClient implements RFC 7591 client update.
func (s *Service) UpdateDynamicClient(ctx context.Context, clientID, registrationAccessToken string, request DCRRequest) (*DCRResponse, error) {
	application, err := s.loadManageableDynamicClient(ctx, clientID, registrationAccessToken)
	if err != nil {
		return nil, err
	}

	if err := validateDCRRequest(request); err != nil {
		return nil, err
	}
	parsedJWKS, jwksErr := parseOptionalClientJWKS(request.JWKS)
	if jwksErr != nil {
		return nil, newInvalidClientMetadataError("jwks is invalid")
	}
	jwksURI := strings.TrimSpace(request.JWKSURI)
	if err := validateClientJWKSURI(jwksURI); err != nil {
		return nil, err
	}

	// Update mutable fields.
	redirectURIs := normalizeStringList(request.RedirectURIs)
	grantTypes := normalizeStringList(request.GrantTypes)
	if len(redirectURIs) == 0 {
		return nil, apperrors.NewInvalidArgument("redirect_uris is required")
	}
	if len(grantTypes) == 0 {
		grantTypes = []string{grantTypeAuthorizationCode, grantTypeRefreshToken}
	}

	authMethod := strings.ToLower(strings.TrimSpace(request.TokenEndpointAuthMethod))
	if authMethod == "" {
		authMethod = application.TokenEndpointAuthMethod
	}

	displayName := request.ClientName
	if displayName == "" {
		displayName = application.DisplayName
	}

	now := s.Clock.Now()
	application.DisplayName = displayName
	application.RedirectURIs = redirectURIs
	application.GrantTypes = grantTypes
	application.TokenEndpointAuthMethod = authMethod
	application.HomepageURL = request.ClientURI
	application.UpdatedAt = now
	application.Revision++

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if authMethod == applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT {
			finalHasJWKS := len(parsedJWKS) > 0 || jwksURI != ""
			if !finalHasJWKS {
				keys, listErr := tx.Applications().ListActiveClientJWKs(ctx, application.OrganizationID, application.ID)
				if listErr != nil {
					return listErr
				}
				finalHasJWKS = len(keys) > 0
				if !finalHasJWKS {
					if uriRepo, ok := tx.Applications().(applicationdomain.ClientJWKSURIRepository); ok {
						existing, getErr := uriRepo.GetClientJWKSURI(ctx, application.OrganizationID, application.ID)
						if getErr != nil && apperrors.FromError(getErr).Code != apperrors.CodeNotFound {
							return getErr
						}
						finalHasJWKS = existing != nil
					}
				}
			}
			if !finalHasJWKS {
				return jwksRequiredError(authMethod)
			}
		}
		// DCR 的 update 请求不携带 secret；切到 secret 方法时，客户端必须已经
		// 持有可验签的 app_client_secrets 行（或 legacy hash），否则写入后就
		// 没有凭据可用。register 会为新客户端生成 secret，因此只需守住 update。
		if requiresClientSecretMethod(authMethod) {
			usableSecrets, listErr := tx.Applications().ListUsableClientSecrets(ctx, application.OrganizationID, application.ID, now)
			if listErr != nil {
				return listErr
			}
			if len(usableSecrets) == 0 && strings.TrimSpace(application.ClientSecretHash) == "" {
				return newInvalidClientMetadataError("token_endpoint_auth_method requires a client secret; register a new client or rotate the secret in the console")
			}
		}
		// 显式切换 mTLS 已在 validateDCRRequest 拒绝；这里覆盖“省略
		// token_endpoint_auth_method、沿用当前 mTLS 方法”的更新：只有此前通过
		// 控制台登记过匹配证书的客户端才允许继续原地更新。
		if isMTLSMethod(authMethod) {
			certificates, listErr := tx.Applications().ListActiveClientCertificates(ctx, application.OrganizationID, application.ID)
			if listErr != nil {
				return listErr
			}
			usable := false
			for _, certificate := range certificates {
				if certificate.AuthMethod == authMethod {
					usable = true
					break
				}
			}
			if !usable {
				return newInvalidClientMetadataError("token_endpoint_auth_method requires client certificates registered through the console")
			}
		}
		if jwksURI != "" {
			uriRepo, ok := tx.Applications().(applicationdomain.ClientJWKSURIRepository)
			if !ok {
				return apperrors.NewInternal("client jwks_uri repository is not configured")
			}
			if err := uriRepo.SetClientJWKSURI(ctx, application.OrganizationID, application.ID, jwksURI, now); err != nil {
				return err
			}
		}
		if len(parsedJWKS) > 0 {
			rows, err := s.buildClientJWKRows(application.ID, application.OrganizationID, parsedJWKS, now)
			if err != nil {
				return err
			}
			if err := tx.Applications().ReplaceClientJWKs(ctx, application.OrganizationID, application.ID, rows, now); err != nil {
				return err
			}
		}
		if err := tx.Applications().Update(ctx, application); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			"",
			"application.dynamic_client.updated",
			application.ID,
			"application.dynamic_client.updated",
			map[string]any{"jwksKidCount": len(parsedJWKS)},
		); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return applicationToDCRResponse(application), nil
}

// DeleteDynamicClient implements RFC 7591 client delete.
func (s *Service) DeleteDynamicClient(ctx context.Context, clientID, registrationAccessToken string) error {
	application, err := s.loadManageableDynamicClient(ctx, clientID, registrationAccessToken)
	if err != nil {
		return err
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// DCR 协议路由已用 registration_access_token 验证客户端；删除仍以实体
		// 自身组织作为仓储谓词，避免 id 复用导致的跨租户删除。
		if err := tx.Applications().Delete(ctx, application.ID, application.OrganizationID); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			application.OrganizationID,
			"",
			"application.dynamic_client.deleted",
			application.ID,
			"application.dynamic_client.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// loadManageableDynamicClient resolves a dynamically registered client and
// proves the caller holds its registration_access_token.
//
// Both failure modes answer with the same 401: an unknown client_id and a valid
// client_id with a wrong token must be indistinguishable, otherwise the endpoint
// becomes an oracle for enumerating registered clients.
func (s *Service) loadManageableDynamicClient(ctx context.Context, clientID, registrationAccessToken string) (*applicationdomain.Application, error) {
	unauthorized := apperrors.NewUnauthorized("registration access token is invalid")

	if strings.TrimSpace(registrationAccessToken) == "" {
		return nil, unauthorized
	}

	application, err := s.UOW.Applications().GetByClientID(ctx, clientID)
	if err != nil {
		return nil, unauthorized
	}
	if application.Category != "dcr" {
		return nil, unauthorized
	}

	storedHash, err := s.UOW.Applications().GetRegistrationAccessTokenHash(ctx, clientID)
	if err != nil {
		return nil, unauthorized
	}
	if storedHash == "" {
		return nil, unauthorized
	}
	if !platformcrypto.VerifyTokenValue(registrationAccessToken, storedHash) {
		return nil, unauthorized
	}

	return application, nil
}

// invalidClientMetadataError 是 RFC 7591 §3.2.2 的 invalid_client_metadata
// （HTTP 400）。error_description 只说明可修正的通用原因，不回显内部判定细节。
type invalidClientMetadataError struct{ description string }

func (e *invalidClientMetadataError) Error() string        { return e.description }
func (e *invalidClientMetadataError) ProtocolCode() string { return "invalid_client_metadata" }
func (e *invalidClientMetadataError) HTTPStatus() int      { return http.StatusBadRequest }

func newInvalidClientMetadataError(description string) error {
	return &invalidClientMetadataError{description: description}
}

func validateDCRRequest(request DCRRequest) error {
	if len(request.RedirectURIs) == 0 {
		return apperrors.NewInvalidArgument("redirect_uris is required")
	}
	for i, uri := range request.RedirectURIs {
		if err := validateOptionalURL(uri, "redirect_uris"); err != nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "redirect_uris must contain valid absolute URLs", map[string]any{metaKeyIndex: i, metaKeyValue: uri})
		}
		// A dynamically registered client must face the same matcher the
		// authorize endpoint applies, so a wildcard is rejected here rather than
		// stored as a registration that can never match (设计报告 14.28.2).
		if err := applicationdomain.ValidateRedirectURI(uri); err != nil {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "redirect_uris contains an unusable value", map[string]any{metaKeyIndex: i, metaKeyValue: uri})
		}
	}

	// The per-URI checks above bound the shape of one entry, not the list: the
	// column is VARCHAR(255)[] and the request slice has no declared
	// cardinality. Reuse the console's limit check instead of keeping a second
	// copy that drifts — otherwise this path only fails at the INSERT.
	if err := validateRedirectURILimits("redirect_uris", request.RedirectURIs); err != nil {
		return err
	}

	for i, gt := range request.GrantTypes {
		normalized := strings.TrimSpace(gt)
		if normalized == "" {
			continue
		}
		if !slices.Contains(allowedDCRGrantTypes, normalized) {
			return apperrors.NewWithMeta(
				apperrors.CodeUnprocessableEntity,
				"grant_types contains unsupported value",
				map[string]any{
					metaKeyIndex: i,
					"grantType":  normalized,
					metaKeyField: "grant_types",
					metaKeyValue: normalized,
					"allowed":    append([]string{}, allowedDCRGrantTypes...),
				},
			)
		}
	}

	if request.TokenEndpointAuthMethod != "" {
		normalized := strings.ToLower(strings.TrimSpace(request.TokenEndpointAuthMethod))
		// client_secret_jwt 不在支持闭集内（OPEN-API-60），按 RFC 7591 返回
		// 400 invalid_client_metadata；描述不暴露内部原因。
		if !slices.Contains(allowedTokenEndpointAuthMethods, normalized) {
			return newInvalidClientMetadataError("token_endpoint_auth_method is not supported")
		}
		// RFC 7591 的注册请求没有客户端证书字段，DCR 无法管理
		// app_client_certificates。接受 mTLS 只会得到一个永远无法在 token
		// endpoint 认证的客户端，因此在这里拒绝并引导走控制台登记证书。
		if isMTLSMethod(normalized) {
			return newInvalidClientMetadataError("token_endpoint_auth_method requires client certificates registered through the console")
		}
		if normalized == applicationdomain.TokenEndpointAuthMethodNone {
			// none 只适用于公开客户端，其安全性依赖授权码流程的强制 PKCE；
			// 因此必须保留 authorization_code，且不能拿它跑 client_credentials。
			grants := normalizeStringList(request.GrantTypes)
			if len(grants) == 0 {
				grants = []string{grantTypeAuthorizationCode, grantTypeRefreshToken}
			}
			if !slices.Contains(grants, grantTypeAuthorizationCode) {
				return newInvalidClientMetadataError("token_endpoint_auth_method none requires the authorization_code grant with PKCE")
			}
			if slices.Contains(grants, "client_credentials") {
				return newInvalidClientMetadataError("token_endpoint_auth_method none cannot be combined with the client_credentials grant")
			}
		}
	}

	// jwks_uri 只做注册边界形状校验；出站白名单/超时/缓存/降级由
	// infra fetcher 在验签时执行（OPEN-API-61）。
	if raw := strings.TrimSpace(request.JWKSURI); raw != "" {
		if err := validateClientJWKSURI(raw); err != nil {
			return newInvalidClientMetadataError("jwks_uri is invalid")
		}
	}
	parsedJWKS, jwksErr := parseOptionalClientJWKS(request.JWKS)
	if jwksErr != nil {
		return newInvalidClientMetadataError("jwks is invalid")
	}
	if strings.ToLower(strings.TrimSpace(request.TokenEndpointAuthMethod)) == applicationdomain.TokenEndpointAuthMethodPrivateKeyJWT && len(parsedJWKS) == 0 && strings.TrimSpace(request.JWKSURI) == "" {
		return newInvalidClientMetadataError("jwks is required when token_endpoint_auth_method is private_key_jwt")
	}

	if request.ClientURI != "" {
		if err := validateOptionalURL(request.ClientURI, "client_uri"); err != nil {
			return err
		}
	}
	if request.LogoURI != "" {
		if err := validateOptionalURL(request.LogoURI, "logo_uri"); err != nil {
			return err
		}
	}

	return nil
}

func applicationToDCRResponse(app *applicationdomain.Application) *DCRResponse {
	var issuedAt int64
	if !app.CreatedAt.IsZero() {
		issuedAt = app.CreatedAt.Unix()
	}
	return &DCRResponse{
		ClientID:                app.ClientID,
		ClientIDIssuedAt:        issuedAt,
		ClientSecretExpiresAt:   0,
		RegistrationClientURI:   registrationClientURIPath(app.ClientID),
		RedirectURIs:            app.RedirectURIs,
		GrantTypes:              app.GrantTypes,
		TokenEndpointAuthMethod: app.TokenEndpointAuthMethod,
		ClientName:              app.DisplayName,
		ClientURI:               app.HomepageURL,
	}
}

// registrationClientURIPath is the relative management endpoint RFC 7591
// §3.2.1 reports as registration_client_uri. It stays relative so the value is
// correct regardless of which host the client reached us on (the issuer rule of
// 14.13.1 governs absolute URLs, and a management URI is not one of them).
func registrationClientURIPath(clientID string) string {
	if strings.TrimSpace(clientID) == "" {
		return ""
	}
	return "/oauth2/register/" + clientID
}

// generateRegistrationAccessToken generates the RFC 7591 management credential.
// It is a bearer secret, so it gets 32 bytes of crypto/rand and is stored only
// as a SHA-256 hash, exactly like an access token (手册 9.5).
func generateRegistrationAccessToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", apperrors.WrapInternal(err, "generate registration_access_token")
	}
	return "gl_rat_" + hex.EncodeToString(buf), nil
}

// generateOAuthClientID generates a OAuth2 client_id with gl_cid_ prefix.
// 24 bytes → base64url (32 chars after prefix). Zero business semantics.
// Discipline: naming.yaml §credential_prefixes; any "app identifier in client_id"
// variant requires ADR review.
func generateOAuthClientID() (string, error) {
	bytes := make([]byte, clientIDEntropy)
	if _, err := rand.Read(bytes); err != nil {
		return "", apperrors.WrapInternal(err, "generate client_id")
	}
	return clientIDPrefix + base64.RawURLEncoding.EncodeToString(bytes), nil
}

// generateOAuthClientSecret generates a OAuth2 client_secret with gl_cs_ prefix.
// 32 bytes → base64url (43 chars after prefix). Only the hash + prefix is stored;
// the plaintext appears exactly once in create/rotate response.
func generateOAuthClientSecret() (string, error) {
	bytes := make([]byte, clientSecretEntropy)
	if _, err := rand.Read(bytes); err != nil {
		return "", apperrors.WrapInternal(err, "generate client_secret")
	}
	return clientSecretPrefix + base64.RawURLEncoding.EncodeToString(bytes), nil
}

// ensure time.Time import is available.
var _ time.Time
