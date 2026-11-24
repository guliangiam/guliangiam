package applications

import (
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/applications"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   uow.AuthUoW
}

// requireOrganizationID fails fast when a management-route use case reaches the
// app layer without a trusted tenant. HTTP handlers resolve this via
// TrustedOrganizationID; the guard also covers non-HTTP callers.
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

// 租户作用域说明：本包不做 RLS 提升。applications 的 organizationId 经
// auth.TrustedOrganizationID 收敛（chi.go list/create/import，以及管理面 by-ID
// 的 GET/PATCH/DELETE/export/provider-bindings）：仅 super_admin/system 作用域可
// 采用显式值，否则回落到会话组织；管理面 by-ID 用例另以 GetByIDForOrganization
// 做显式租户读取，仓储再以 organization_id 谓词与 RLS 过滤。
type CreateApplicationCommand struct {
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID                 string
	Slug                    string
	DisplayName             string
	Category                string
	AppType                 string
	Description             string
	HomepageURL             string
	Domain                  string
	Status                  string
	ClientID                string
	ClientSecret            string
	TokenEndpointAuthMethod string
	TokenSigningMethod      string
	RedirectURIs            []string
	GrantTypes              []string
	CustomScopes            []ApplicationCustomScopeCommand
	// JWKS 是内嵌公钥集合（RFC 7517 §5），仅用于 private_key_jwt 验签。
	// nil 表示请求未携带；非 nil 时必须通过 domain.ParseClientJWKS 校验。
	JWKS []byte
	// JWKSURI 指向远端 JWKS（与内嵌 JWKS 二选一或并存）；验签时按需拉取。
	JWKSURI string
	// ClientCertificates 是 mTLS 认证的初始证书集合，与 application 同事务写入。
	// 认证方式为 mTLS 时至少一张；其他方式必须为空。
	ClientCertificates []ClientCertificateInput
}

type ApplicationCustomScopeCommand struct {
	Scope       string
	DisplayName string
	Description string
}

// NewApplication 保留历史签名（丢弃一次性明文），仅供不需要返回凭据的内部调用与旧测试。
// 管理面创建与 DCR 必须使用 NewApplicationWithSecret，确保明文只出现一次。
func (s *Service) NewApplication(command CreateApplicationCommand) (*domain.Application, error) {
	application, _, err := s.NewApplicationWithSecret(command)
	return application, err
}

// ClientSecretMaterial 是一次性明文与其哈希。明文只允许进入创建/轮换响应，
// 哈希写入 app_client_secrets；两者都不得进入日志或读接口。
type ClientSecretMaterial struct {
	Plain string
	Hash  string
}

// NewApplicationWithSecret 构建应用实体并返回一次性明文 secret（未生成时为空）。
// 哈希不再写入 legacy 列 app_oauth_config.client_secret_hash（TODO(deprecate)），
// 由调用方在同一个事务内写入 app_client_secrets。
func (s *Service) NewApplicationWithSecret(command CreateApplicationCommand) (*domain.Application, *ClientSecretMaterial, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, nil, err
	}

	now := s.Clock.Now()

	// Auto-generate client_id only if not supplied in command.
	// Discipline: naming.yaml §credential_prefixes → gl_cid_ + base64url.
	// Frontend creates MUST NOT presuppose a value (empty string passes here).
	// DCR explicitly supplies its own generated clientID — respected here.
	var clientID string
	if command.ClientID != "" {
		clientID = command.ClientID
	} else {
		generated, err := generateOAuthClientID()
		if err != nil {
			return nil, nil, err
		}
		clientID = generated
	}

	// Resolve auth method first — normalization depends on whether a clientSecret exists.
	// Only an unspecified method falls back by app type; an explicit `none` is a
	// deliberate public-client registration and must not be upgraded to basic.
	rawSecret := normalizeOptionalString(command.ClientSecret)
	effectiveMethod := command.TokenEndpointAuthMethod
	if effectiveMethod == "" {
		switch strings.ToLower(strings.TrimSpace(command.AppType)) {
		case "web", "machine":
			effectiveMethod = domain.TokenEndpointAuthMethodClientSecretBasic
		default:
			effectiveMethod = domain.TokenEndpointAuthMethodNone
		}
	}
	authMethod := normalizeTokenEndpointAuthMethod(effectiveMethod, rawSecret)

	// Auto-generate clientSecret only for the two HTTP-secret methods. JWT and
	// mTLS clients hold their credentials elsewhere (JWKS / certificate), so
	// minting an unused secret would be misleading.
	if (authMethod == domain.TokenEndpointAuthMethodClientSecretBasic || authMethod == domain.TokenEndpointAuthMethodClientSecretPost) && rawSecret == "" {
		generated, err := generateOAuthClientSecret()
		if err != nil {
			return nil, nil, err
		}
		rawSecret = generated
	}

	// Hash the client secret (user-provided or auto-generated) for the one-shot
	// write into app_client_secrets. The legacy app_oauth_config column is no
	// longer written (TODO(deprecate)); the caller persists the row in the same tx.
	var clientSecretHash string
	if rawSecret != "" {
		hash, err := platformcrypto.HashClientSecret(rawSecret)
		if err != nil {
			return nil, nil, err
		}
		clientSecretHash = hash
	}

	var clientSecret *ClientSecretMaterial
	if rawSecret != "" {
		clientSecret = &ClientSecretMaterial{Plain: rawSecret, Hash: clientSecretHash}
	}

	return &domain.Application{
		ID:                      id,
		OrganizationID:          command.OrganizationID,
		Slug:                    command.Slug,
		DisplayName:             command.DisplayName,
		Category:                command.Category,
		AppType:                 command.AppType,
		Description:             command.Description,
		HomepageURL:             command.HomepageURL,
		Domain:                  command.Domain,
		Status:                  command.Status,
		ClientID:                clientID,
		TokenEndpointAuthMethod: authMethod,
		TokenSigningMethod:      normalizeTokenSigningMethod(command.TokenSigningMethod),
		RedirectURIs:            normalizeStringList(command.RedirectURIs),
		GrantTypes:              normalizeStringList(command.GrantTypes),
		CustomScopes:            toDomainCustomScopes(command.CustomScopes),
		Tags:                    []string{},
		CreatedAt:               now,
		UpdatedAt:               now,
		Revision:                1,
	}, clientSecret, nil
}

func toDomainCustomScopes(items []ApplicationCustomScopeCommand) []domain.ApplicationCustomScope {
	result := make([]domain.ApplicationCustomScope, 0, len(items))
	for _, item := range items {
		result = append(result, domain.ApplicationCustomScope{
			Scope:       item.Scope,
			DisplayName: item.DisplayName,
			Description: item.Description,
		})
	}
	return result
}

func normalizeStringList(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := normalizeOptionalString(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func normalizeOptionalString(value string) string {
	return strings.TrimSpace(value)
}

func normalizeTokenEndpointAuthMethod(method, clientSecret string) string {
	normalizedMethod := strings.ToLower(strings.TrimSpace(method))
	if domain.IsRegisteredTokenEndpointAuthMethod(normalizedMethod) {
		return normalizedMethod
	}
	if normalizeOptionalString(clientSecret) != "" {
		return domain.TokenEndpointAuthMethodClientSecretBasic
	}
	return domain.TokenEndpointAuthMethodNone
}
