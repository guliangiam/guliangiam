package aigateway

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// GatewayService is the Core of the AI Gateway data plane.
//
// It orchestrates the full request lifecycle:
//
//  1. RouterEngine.Resolve  — alias → concrete route (health filtered)
//  2. QuotaMeter.Get + LlmQuotaPolicy.CheckLimit — per-API-key / per-org RPM/TPM/cost
//  3. UpstreamAdapter.Chat/ChatStream/Embedding  — vendor call
//  4. PriceVersion.CalculateCost + UsageLogsRepo.Create  — 用量计量
//  5. QuotaMeter.Incr — 成功后递增 Redis 计数器
//
// Every dependency is a Domain Port — infra implementations are injected at
// bootstrap time; the Core never imports an infra package. See ADR-035.
// Meter 字段为 nil 时所有配额检查跳过（Fail-Open，开发环境友好）。
type GatewayService struct {
	Clock clock.Clock
	UUID  uuid.Generator

	Routers   domain.RouterEngine
	Upstreams domain.ProviderRegistry
	Quotas    domain.QuotaPoliciesRepo
	Prices    domain.PriceVersionsRepo
	UsageLogs domain.UsageLogsRepo
	ApiKeys   domain.ApiKeysRepo
	Aliases   domain.AliasesRepo
	Providers domain.ProvidersRepo
	Health    domain.ProviderHealthRepo

	// Meter 是 Redis 配额计数器端口。nil 表示配额治理未启用——enforceQuota
	// 和 incrementCounters 会直接跳过（Fail-Open，开发环境友好；生产应配置）。
	Meter domain.QuotaMeter

	// Outbox 是审计事件投递端口。nil 表示审计投递关闭——Enqueue 事件会
	// 被静默跳过（Fail-Open，开发环境友好；生产应配置以便 metering_events
	// 和通知下游消费）。
	Outbox domainoutbox.Repository

	// ToolExecutor 是工具调用执行端口。nil 时 tool_use 块原样透传给
	// 调用方（由调用方手动执行）；非 nil 时 GatewayService 自动执行后
	// 替换为 tool_result 块（Fail-Open）。见设计文档 §7。
	ToolExecutor domain.ToolExecutor
}

// NewGatewayService wires the Core with all its Domain Port dependencies.
// meter / outbox 可以为 nil——此时对应能力分别关闭（Fail-Open）。
func NewGatewayService(
	ck clock.Clock,
	uid uuid.Generator,
	routers domain.RouterEngine,
	upstreams domain.ProviderRegistry,
	quotas domain.QuotaPoliciesRepo,
	prices domain.PriceVersionsRepo,
	usageLogs domain.UsageLogsRepo,
	apiKeys domain.ApiKeysRepo,
	aliases domain.AliasesRepo,
	providers domain.ProvidersRepo,
	health domain.ProviderHealthRepo,
	meter domain.QuotaMeter,
	outbox domainoutbox.Repository,
) *GatewayService {
	return &GatewayService{
		Clock:     ck,
		UUID:      uid,
		Routers:   routers,
		Upstreams: upstreams,
		Quotas:    quotas,
		Prices:    prices,
		UsageLogs: usageLogs,
		ApiKeys:   apiKeys,
		Aliases:   aliases,
		Providers: providers,
		Health:    health,
		Meter:     meter,
		Outbox:    outbox,
	}
}

// authenticate 校验 API Key：hash 格式 → DB 查 → 状态 + 过期检查。
//
// key 查询是认证步骤，此时请求还没有可信租户 —— 用 System 作用域执行
// （hash 本身是全库唯一凭据）；认证通过后由 bindKeyTenant 把后续所有
// 业务查询绑定到 key.OrganizationID（BE-C-05）。
func (s *GatewayService) authenticate(ctx context.Context, hash string) (*domain.LlmApiKey, error) {
	if hash == "" {
		return nil, apperrors.NewUnauthorized("api key required")
	}
	if s.ApiKeys == nil {
		return nil, apperrors.NewUnavailable("api key repository not configured")
	}
	key, err := s.ApiKeys.GetByHash(tenantctx.WithTenantScope(ctx, "", "", tenantctx.TenantFilterSystem), hash)
	if err != nil {
		return nil, apperrors.NewUnauthorized("invalid api key")
	}
	if key == nil {
		return nil, apperrors.NewUnauthorized("invalid api key")
	}
	if key.OrganizationID == "" {
		// 没有租户归属的 key 无法建立可信租户上下文 → fail-closed。
		return nil, apperrors.NewUnauthorized("invalid api key")
	}
	if !key.IsActive(s.Clock.Now()) {
		switch key.Status {
		case domain.APIKeyStatusRevoked:
			return nil, apperrors.NewUnauthorized("api key has been revoked")
		case domain.APIKeyStatusExpired:
			return nil, apperrors.NewUnauthorized("api key has expired")
		default:
			return nil, apperrors.NewUnauthorized("api key is not active")
		}
	}
	return key, nil
}

// bindKeyTenant 把数据面请求上下文绑定到 API key 所属组织（Strict 模式）。
// 数据面路由不经过 TenantFilter 中间件，租户可信来源只能是认证得到的 key；
// 之后的路由解析、配额查询、用量落库、Outbox 全部继承该作用域。
func bindKeyTenant(ctx context.Context, key *domain.LlmApiKey) context.Context {
	return tenantctx.WithTenantScope(ctx, key.OrganizationID, "", tenantctx.TenantFilterStrict)
}

// ListAliases returns available model aliases for the GET /models endpoint.
// It maps domain Alias objects into App-layer AliasSummary DTOs so the
// transport layer never touches domain types.
// ListAliasesForAPIKey 先 authenticate → 拿 org_id → ListAliases。
// 给 transport 层调用，不用它自己管 org_id。
func (s *GatewayService) ListAliasesForAPIKey(ctx context.Context, apiKeyHash string) ([]AliasSummary, error) {
	key, err := s.authenticate(ctx, apiKeyHash)
	if err != nil {
		return nil, err
	}
	return s.ListAliases(bindKeyTenant(ctx, key), key.OrganizationID)
}

func (s *GatewayService) ListAliases(ctx context.Context, orgID string) ([]AliasSummary, error) {
	if s.Aliases == nil {
		return nil, apperrors.NewUnavailable("aliases repo not configured")
	}
	aliases, err := s.Aliases.List(ctx, orgID)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "list aliases")
	}
	out := make([]AliasSummary, 0, len(aliases))
	for _, a := range aliases {
		if a == nil || !a.IsAvailable() {
			continue
		}
		out = append(out, AliasSummary{
			Alias:       a.Alias,
			DisplayName: a.DisplayName,
			Status:      a.Status,
			CreatedUnix: a.CreatedAt.Unix(),
		})
	}
	return out, nil
}

// aliasFromResolution 从 RouteResolution 拿到 aliasID。
func aliasFromResolution(res *domain.RouteResolution) string {
	if res == nil || res.Route == nil {
		return ""
	}
	return res.Route.AliasID
}
