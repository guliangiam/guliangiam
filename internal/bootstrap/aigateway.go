// bootstrap/aigateway.go — AI Gateway wiring.
//
// 9 Repo instances + RouterEngine + Upstream Adapter Factory + QuotaMeter → GatewayService
// (数据平面)；同一个 Repo 集 → AdminService (管理面 CRUD)。
// 与 runtime.go 中其他模块的构造保持相同的风格：零值 clock/uuid，
// Repo 全部共享同一个 shared.DBTX。QuotaMeter 独立 Redis client，
// Fail-Open：Redis 不可用时跳过配额治理（migration-friendly）。
package bootstrap

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	infragw "github.com/guliangiam/guliangiam/internal/infra/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/aiupstream"
	pgag "github.com/guliangiam/guliangiam/internal/infra/postgres/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
	"github.com/redis/go-redis/v9"
)

// aiGatewayRepos 聚合 AI Gateway 的 9 个仓储实例（数据平面与管理面共用）。
type aiGatewayRepos struct {
	Providers      pgag.ProvidersRepository
	Models         pgag.ModelsRepository
	Aliases        pgag.AliasesRepository
	Routes         pgag.RoutesRepository
	APIKeys        pgag.ApiKeysRepository
	QuotaPolicies  pgag.QuotaPoliciesRepository
	PriceVersions  pgag.PriceVersionsRepository
	ProviderHealth pgag.ProviderHealthRepository
	UsageLogs      pgag.UsageLogsRepository
}

// newAIGatewayRepos 构造 AI Gateway 全部 9 个 Repo 实例并聚合为 aiGatewayRepos。
// 两个 Service（数据平面 + 管理面）共用同一个 Repo 集——事务边界和 DB 连接
// 都由 shared.DBTX 控制，无需各自维护。
func newAIGatewayRepos(db shared.DBTX) aiGatewayRepos {
	return aiGatewayRepos{
		Providers:      pgag.NewProvidersRepository(db),
		Models:         pgag.NewModelsRepository(db),
		Aliases:        pgag.NewAliasesRepository(db),
		Routes:         pgag.NewRoutesRepository(db),
		APIKeys:        pgag.NewApiKeysRepository(db),
		QuotaPolicies:  pgag.NewQuotaPoliciesRepository(db),
		PriceVersions:  pgag.NewPriceVersionsRepository(db),
		ProviderHealth: pgag.NewProviderHealthRepository(db),
		UsageLogs:      pgag.NewUsageLogsRepository(db),
	}
}

// wireQuotaMeter 构造 RedisQuotaMeter（Round 9 配额治理）。
// Fail-Open：依次尝试 RATE_LIMIT_REDIS_ADDR → REDIS_ADDR → nil。
// Redis 不可用时返回 nil，GatewayService 在 Meter == nil 时跳过所有配额检查。
// 这样可以在没有 Redis 的开发环境里正常运行，不阻断 CI/本地调试。
func wireQuotaMeter() domain.QuotaMeter {
	addr := platformconfig.RateLimitRedisAddr()
	if addr == "" {
		addr = platformconfig.RedisAddr()
	}
	if addr == "" {
		slog.Info("aigateway quota meter: no Redis configured, quota enforcement disabled")
		return nil
	}

	password := platformconfig.RateLimitRedisPassword()
	if password == "" {
		password = platformconfig.RedisPassword()
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           platformconfig.RateLimitRedisDB(), // 与 session cache DB=0 隔离
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		slog.Warn("aigateway quota meter Redis ping failed; quota enforcement disabled",
			"addr", addr, "err", err)
		return nil
	}

	slog.Info("aigateway quota meter initialized", "addr", addr)
	return aiupstream.NewRedisQuotaMeter(client)
}

// NewAIGatewayService wires the AI Gateway Core with all its Repo/Adapter
// dependencies and returns a fully constructed GatewayService (data plane).
//
// db 是共享的 shared.DBTX — 生产环境传 pgx.InstrumentedDBTX，
// 测试可传 *pgxmock.PgxPool 或 *sqlmock 适配层。
// meter / outbox 为 nil 时对应能力关闭（Fail-Open）。
func NewAIGatewayService(db shared.DBTX, meter domain.QuotaMeter, outbox domainoutbox.Repository) *appgateway.GatewayService {
	repos := newAIGatewayRepos(db)

	routerEngine := infragw.NewDefaultRouterEngine(
		repos.Aliases, repos.Routes, repos.Providers, repos.Models, repos.ProviderHealth,
	)

	factory := aiupstream.NewAdapterFactory()
	aiupstream.RegisterDefaults(factory)

	return appgateway.NewGatewayService(
		clock.SystemClock{},
		platformuuid.V7Generator{},
		routerEngine, factory,
		repos.QuotaPolicies, repos.PriceVersions, repos.UsageLogs, repos.APIKeys, repos.Aliases, repos.Providers, repos.ProviderHealth,
		meter,
		outbox,
	)
}

// NewAIGatewayAdminService wires the management-plane AdminService with the
// same Repo set. 管理面不关心 RouterEngine 或 Upstream adapters，但需要
// Route 和 QuotaPolicy Repo（数据平面也共用这两个）。
func NewAIGatewayAdminService(db shared.DBTX) *appgateway.AdminService {
	repos := newAIGatewayRepos(db)

	adminSvc := appgateway.NewAdminService(
		platformuuid.V7Generator{},
		clock.SystemClock{},
		nil, // Encryptor 为 nil → service 内部用 GlobalFieldEncryptor()
		repos.Providers,
		repos.Models,
		repos.Aliases,
		repos.Routes,
		repos.APIKeys,
		repos.QuotaPolicies,
		repos.UsageLogs,
		repos.PriceVersions,
		repos.ProviderHealth,
	)
	// TestProvider 复用与数据平面相同的 AdapterFactory 构建路径：
	// BuildAdapter 内部完成 provider 凭据解密与协议归一化，
	// decryptor 复用进程级 GlobalFieldEncryptor，不新建单例。
	factory := aiupstream.NewAdapterFactory()
	aiupstream.RegisterDefaults(factory)
	factory.Decryptor = platformcrypto.GlobalFieldEncryptor()
	adminSvc.Upstreams = factory
	return adminSvc
}

// NewHealthProbeWorker 构造 AI Gateway 半开探测后台 Worker（设计文档 §2.4）。
// 构造过程：
//  1. 重新使用 pgag.NewProvidersRepository + pgag.NewProviderHealthRepository
//     —— 与 NewAIGatewayService / NewAIGatewayAdminService 不同，此处不需要
//     RouterEngine / Adapter Factory，Worker 只需要 ProviderHealthRepo（列 Open）
//     和 ProvidersRepo（拿 BaseURL）。
//  2. HTTP client 使用 aiupstream.NewSSRFClient —— 拒绝探测 loopback /
//     RFC1918 / link-local 等内部网段，防 SSRF 反连。
//  3. outbox 可以为 nil（Fail-Open）——有配置时熔断器状态转换事件会投递
//     ai.provider.circuit_opened / ai.provider.circuit_closed（设计文档 §13）。
//  4. 返回 *appgateway.HealthProbeWorker，调用 Start() 启动 goroutine。
func NewHealthProbeWorker(db shared.DBTX, outbox domainoutbox.Repository, logger *slog.Logger) *appgateway.HealthProbeWorker {
	providersRepo := pgag.NewProvidersRepository(db)
	healthRepo := pgag.NewProviderHealthRepository(db)

	httpClient := aiupstream.NewSSRFClient(10 * time.Second)
	interval := resolveProbeInterval()
	return appgateway.NewHealthProbeWorker(
		healthRepo,
		providersRepo,
		clock.SystemClock{},
		httpClient,
		outbox,
		interval,
		logger,
	)
}

// resolveProbeInterval reads AI_GATEWAY_PROBE_INTERVAL env var with a default of 30s.
// Accepts plain integers (seconds), "30s", "1m", "500ms" — time.ParseDuration compatible.
// Falls back to 30s on any parse failure (logged at Warn by runtime caller).
func resolveProbeInterval() time.Duration {
	const envKey = "AI_GATEWAY_PROBE_INTERVAL"
	const fallback = 30 * time.Second

	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return fallback
	}
	// If it parses as a bare integer, treat as seconds (friendly default).
	if secs, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if secs <= 0 {
			return fallback
		}
		return time.Duration(secs) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	return fallback
}
