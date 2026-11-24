// wire_commerce_workers.go — Commerce domain background workers + history repos + ldapSync.
// 从 runtime.go NewRuntime L565-616 抽离，保持 bootstrap 包级 private function。
// 包含: meteringAggregateWorker, paymentsQueryWorker, paymentsReconWorker (均已 Start),
//
//	paymentsReconService, paymentsChannelConfigService, 3 个 history repo, ldapSyncService.
package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	appb2b "github.com/guliangiam/guliangiam/internal/app/b2b"
	appcron "github.com/guliangiam/guliangiam/internal/app/cron"
	appldapsync "github.com/guliangiam/guliangiam/internal/app/ldapsync"
	appmetering "github.com/guliangiam/guliangiam/internal/app/metering"
	apppayments "github.com/guliangiam/guliangiam/internal/app/payments"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"

	domainldaps "github.com/guliangiam/guliangiam/internal/domain/ldaps"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	infraldapsync "github.com/guliangiam/guliangiam/internal/infra/postgres/ldapsync"
	infrapgpayments "github.com/guliangiam/guliangiam/internal/infra/postgres/payments"
	infrapgproviders "github.com/guliangiam/guliangiam/internal/infra/postgres/providers"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// commerceWorkers 聚合 Commerce 域后台 Worker 与配套 repo/service。
type commerceWorkers struct {
	MeteringAggregate        *appmetering.AggregateWorker
	PaymentsRecon            apppayments.ReconService
	PaymentsChannelConfig    apppayments.ChannelConfigService
	PaymentsAttempts         infrapgpayments.AttemptsRepository
	PaymentsRefunds          infrapgpayments.RefundsRepository
	PaymentsCallbackReceipts infrapgpayments.CallbackReceiptsRepository
	LDAPSync                 *appldapsync.Service
	ProviderConsistency      *appproviders.ConsistencyWorker
}

// wireCommerceWorkers 构造并启动 Commerce 域后台 Worker + 配套 repo/service。
// 副作用: meteringAggregateWorker 以 goroutine 启动（workersCtx），
// paymentsQueryWorker / paymentsReconWorker / providerConsistencyWorker 内部 Start()，ldapSyncService.StartScheduler().
func wireCommerceWorkers(
	pool *pgxpool.Pool,
	unitOfWork appuow.UnitOfWork,
	clockProvider clock.Clock,
	uuidGenerator platformuuid.Generator,
	workersCtx context.Context,
) commerceWorkers {
	// ── metering AggregateWorker ──────────────────────────────────────────
	// 周期性扫 metering_events → 幂等 RefreshAggregate 入 metering_aggregates。
	// 与 outbox worker 独立 goroutine，metering_events 追加型 + RefreshAggregate 幂等 SUM → 安全并发。
	meteringAggregateWorker := appmetering.NewAggregateWorker(unitOfWork.Metering(), slog.Default(), 5*time.Minute)
	recover.SafeGo("metering-aggregate", func() {
		if err := meteringAggregateWorker.Start(workersCtx); err != nil {
			slog.Warn("metering aggregate worker returned", "err", err)
		}
	})

	// ── payments QueryWorker (design doc G10) ────────────────────────────
	// poll processing payments, query channel-side state, converge UNKNOWNs
	// into succeeded/failed — 遵循 "UNKNOWN 不推断" 原则。
	paymentsQueryRegistry := apppayments.NewGatewayRegistry()
	paymentsQueryWorker := apppayments.NewQueryWorker(
		infrapgpayments.NewRepository(pool),
		infrapgpayments.NewAttemptsRepository(pool),
		clockProvider, uuidGenerator,
		paymentsQueryRegistry,
		60*time.Second, 50,
	)
	paymentsQueryWorker.Start(workersCtx)

	// ── payments ReconcileWorker (design doc G11) ────────────────────────
	// poll pending recon batches, download channel statements, diff vs local,
	// write ReconItem diffs for manual disposition.
	paymentsReconRegistry := apppayments.NewGatewayRegistry()
	paymentsReconWorker := apppayments.NewReconcileWorker(
		infrapgpayments.NewRepository(pool),
		infrapgpayments.NewReconRepository(pool),
		clockProvider, uuidGenerator,
		paymentsReconRegistry,
		5*time.Minute, 10,
	)
	paymentsReconWorker.Start(workersCtx)

	providerConsistencyWorker := appproviders.NewConsistencyWorker(
		infrapgproviders.NewRepository(infrapg.NewInstrumentedDBTX(pool)),
		unitOfWork, clockProvider, uuidGenerator, 24*time.Hour, slog.Default(),
	)
	providerConsistencyWorker.Start(workersCtx)

	// ── B2B 域名自动续验 Worker ───────────────────────────────────────────
	// 2min ticker 原子认领到期 pending 域名（租约 5min）→ DNS TXT → verified/retry/failed。
	// worker.run 内部注入 tenantctx.System，ClaimPending 在事务内读写跨租户行。
	b2bVerifyWorker := appcron.NewB2BVerifyWorker(
		appb2b.NewService(clockProvider, uuidGenerator, unitOfWork),
		appcron.WithLogger(slog.Default()),
	)
	b2bVerifyWorker.Start(workersCtx)

	paymentsReconService := apppayments.NewReconService(
		infrapgpayments.NewReconRepository(pool),
		paymentsReconWorker,
		uuidGenerator,
		unitOfWork,
	)

	paymentsChannelConfigService := apppayments.NewChannelConfigService(
		infrapgpayments.NewChannelConfigsRepository(pool),
		clockProvider,
		// Reuse the process-wide field encryptor published by applyFieldEncryption.
		// It implements the app-layer ChannelConfigCipher port; no second
		// encryption abstraction is introduced.
		platformcrypto.GlobalFieldEncryptor(),
		uuidGenerator,
	)

	// ── payments history sub-resources ────────────────────────────────────
	// attempts timeline + refund list for admin UI debug.
	paymentsAttemptsRepo := infrapgpayments.NewAttemptsRepository(pool)
	paymentsRefundsRepo := infrapgpayments.NewRefundsRepository(pool)
	paymentsCallbackReceiptsRepo := infrapgpayments.NewCallbackReceiptsRepository(pool)

	// ── LDAP Sync scheduler ────────────────────────────────────────────────
	// MySQL 分支 pool=nil：LDAP Sync 仓储只有 PostgreSQL 实现。注入「能力不可用」
	// 实现并跳过 scheduler，避免 nil pool 在后台 goroutine 里 panic 导致进程退出；
	// 请求侧得到 503 + unavailable（OPEN-API-36）。
	var ldapSyncRepo domainldaps.SyncRepository
	if pool == nil {
		ldapSyncRepo = unavailableLDAPSyncRepository{}
	} else {
		ldapSyncRepo = infraldapsync.NewRepository(pool)
	}
	ldapSyncService := appldapsync.NewService(ldapSyncRepo, unitOfWork.Ldaps(), unitOfWork.Users())
	if pool != nil {
		ldapSyncService.WithLockProvider(infrapg.NewAdvisoryLockProvider(pool))
		ldapSyncService.StartScheduler()
	}

	return commerceWorkers{
		MeteringAggregate:        meteringAggregateWorker,
		PaymentsRecon:            paymentsReconService,
		PaymentsChannelConfig:    paymentsChannelConfigService,
		PaymentsAttempts:         paymentsAttemptsRepo,
		PaymentsRefunds:          paymentsRefundsRepo,
		PaymentsCallbackReceipts: paymentsCallbackReceiptsRepo,
		LDAPSync:                 ldapSyncService,
		ProviderConsistency:      providerConsistencyWorker,
	}
}
