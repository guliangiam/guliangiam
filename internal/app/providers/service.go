package providers

import (
	"strings"

	"github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type Service struct {
	Clock   clock.Clock
	UUID    platformuuid.Generator
	UOW     uow.AuthUoW
	Unified *UnifiedService
}

func (s Service) unified() *UnifiedService {
	if s.Unified != nil {
		return s.Unified
	}
	return NewUnifiedService(s.Clock, s.UUID, s.UOW)
}

// requireOrganizationID fails fast when a by-ID management call reaches the app
// layer without a trusted tenant. HTTP handlers resolve this via
// TrustedOrganizationID; the guard keeps non-HTTP callers from degrading to a
// cross-tenant lookup.
func requireOrganizationID(organizationID string) error {
	if strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	return nil
}

// 租户作用域说明：本包不做 RLS 提升。列表类查询的 organizationId 由 transport 的
// trustedOrgID 解析（普通调用者伪造的 query/body 值被忽略）；by-ID 读写先经
// GetByIDForOrganization 在仓储层绑定 organization_id，跨租户按 not found 处理，
// ensureProviderTenantScope 仅作为二次断言。
// 全局列表 /api/v1/global/providers 以空 organizationId 查询（chi.go）；provider_cache
// 读取当前作用域，System/SuperAdmin 时跳过按租户隔离的缓存键（provider_cache.go:170-171）。
type CreateProviderCommand struct {
	ID             string
	OrganizationID string
	// ActorID is the operator user id (empty for system actions).
	ActorID       string
	Name          string
	DisplayName   string
	Family        string
	Kind          string
	Variant       string
	Category      string
	ProviderType  string
	SubType       string
	Method        string
	ClientID      string
	ClientSecret  string
	ClientID2     string
	ClientSecret2 string
	ProviderURL   string
	AuthUrl       string
	TokenUrl      string
	UserInfoUrl   string
	CallbackUrl   string
	SigningAlg    string
	Scopes        []string
	IdPEntityId   string
	SsoUrl        string
	SloUrl        string
	Metadata      string
	IssuerUrl     string
	Region        string
	Host          string
	Port          int
	DatabaseType  string
	UserMapping   map[string]any
	Template      string
	CustomApi     string
	CustomHeader  string
	CustomBody    string
	Admin         string
	Password      string
	AppId         string
	AgentID       string
	ContentType   string
	Sender        string
	Receiver      string

	// Casdoor-compatible options persisted in provider_configs.
	EmailRegex                  string
	EnablePkce                  bool
	EnableProxy                 bool
	HttpHeaders                 map[string]string
	RequireMessageAuthenticator bool
	EnableSignAuthnRequest      bool
	CustomLogoutUrl             string
	CustomLogo                  string
	Cert                        string
	State                       string

	AI           *AIProviderConfig
	Payment      *PaymentProviderConfig
	Notification *NotificationProviderConfig
}

func (s Service) NewProvider(command CreateProviderCommand) (*domain.Provider, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	// ARRAY 列 NOT NULL 保护：repo 显式 INSERT scopes (ARRAY NOT NULL DEFAULT '{}')
	// Go nil slice → pgx 传 NULL → 触发 NOT NULL 约束。
	// CreateProviderCommand 里没有 Scopes（provider_configs 子表另行存储），
	// 这里显式设空数组对齐 DB DEFAULT。
	now := s.Clock.Now()
	return &domain.Provider{
		ID:             id,
		OrganizationID: command.OrganizationID,
		Name:           command.Name,
		DisplayName:    command.DisplayName,
		Category:       domain.ProviderCategory(command.Category),
		ProviderType:   domain.ProviderType(command.ProviderType),
		ClientID:       command.ClientID,
		ClientSecret:   command.ClientSecret,
		ProviderURL:    command.ProviderURL,
		Scopes:         []string{},
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}, nil
}
