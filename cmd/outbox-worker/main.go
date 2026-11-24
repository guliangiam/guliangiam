// ============================================================================
// outbox-worker — 独立 Outbox 投递 Worker
// ============================================================================
// Phase A2 里程碑产物（§9.5）：将 Outbox Worker 从主进程 goroutine 抽离为
// 独立进程。通过 FOR UPDATE SKIP LOCKED 与主进程 Worker 并行不会重复取件
// （ADR-008）。
//
// 启动：
//   go run ./cmd/outbox-worker
//   或构建独立二进制：go build -o outbox-worker ./cmd/outbox-worker
//
// K8s 部署：见 k8s.yaml outbox-worker Deployment（Phase A2 新增）
// ============================================================================

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	appwebhooks "github.com/guliangiam/guliangiam/internal/app/webhooks"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	infraoutbox "github.com/guliangiam/guliangiam/internal/infra/postgres/outbox"
	postgresuow "github.com/guliangiam/guliangiam/internal/infra/postgres/uow"
	infrawebhook "github.com/guliangiam/guliangiam/internal/infra/webhook"
	platformconfig "github.com/guliangiam/guliangiam/internal/platform/config"
	platformdb "github.com/guliangiam/guliangiam/internal/platform/db"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

func main() {
	os.Exit(run())
}

func run() int {
	logger := slog.Default()
	logger.Info("outbox-worker starting")

	// ── Config 校验（复用 bootstrap ValidateOrPanic，fail-fast） ──
	platformconfig.ValidateOrPanic()

	// ── DB Pool ──
	// 必须用 platformdb.NewPostgresPool：它装配 AfterConnect(fail-closed)、
	// PrepareConn（按 ctx 的 TenantScope 设置 app.current_role/org_id 的 SESSION
	// GUC）与 AfterRelease reset。直接用 pgxpool.New 不会设置 GUC，在 FORCE RLS
	// 下应用账号执行 LockBatch 只能看到 0 行（X2 未验证边界即由此暴露）。
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := platformdb.NewPostgresPool(ctx, platformdb.PostgresConfig{
		DataSourceName: platformdb.PostgresDataSourceName(),
	})
	if err != nil {
		logger.Error("db pool init failed", "error", err)
		return 1
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("db ping failed", "error", err)
		return 1
	}
	logger.Info("db pool ready")

	// ── Outbox Repository ──
	outboxRepo := infraoutbox.NewRepository(infrapg.NewInstrumentedDBTX(pool), pool)

	// ── Delivery Registry ──
	// H40：webhook 投递统一走 outbox 单轨，DeliveryOutboxHandler 投递并维护
	// webhook_events 台账；领域事件（Organization/risk/user/session）扇出到
	// 租户 webhook 订阅者。缺少这些 handler 时 Worker 会把事件直接转死信。
	uow := postgresuow.New(infrapg.NewInstrumentedDBTX(pool), pool)
	webhookSender := infrawebhook.NewHTTPDeliverySender(nil, 30*time.Second)
	outboxRegistry := platformoutbox.MapRegistry{
		"webhook": appwebhooks.NewDeliveryOutboxHandler(uow, webhookSender, logger),
	}
	domainEventHandler := appwebhooks.NewDomainEventOutboxHandler(uow, logger)
	for _, aggregateType := range appwebhooks.DomainEventAggregateTypes {
		outboxRegistry[aggregateType] = domainEventHandler
	}

	// ── Worker 构造 + 启动 ──
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown"
	}
	workerID := host + "-outbox"
	worker := platformoutbox.NewWorker(
		outboxRepo,
		outboxRegistry,
		workerID,
		platformoutbox.WithPollInterval(5*time.Second),
		platformoutbox.WithBackoff(time.Second, 32*time.Second),
	)

	// ⚠️ TenantFilterSystem 注入：
	//   独立进程 Worker 不经过 HTTP middleware，直接创建 TenantFilterSystem ctx。
	//   AfterConnect 改 fail-closed（Phase D1）后，此注入保证 Worker RLS 放行。
	rootCtx, rootCancel := context.WithCancel(
		infrapg.WithTenantScope(context.Background(), "", "", infrapg.TenantFilterSystem),
	)
	defer rootCancel()

	// 独立 Worker：与主进程 Worker 共享 DB 但进程独立
	worker.Start(rootCtx)
	logger.Info("outbox-worker started", "worker_id", workerID)

	// ── 优雅退出 ──
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	logger.Info("shutting down", "signal", sig)

	rootCancel()
	worker.Stop()
	logger.Info("outbox-worker stopped")
	return 0
}
