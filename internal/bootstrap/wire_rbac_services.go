// wire_rbac_services.go — ReBAC engine + 7 services（部分 only-uow，部分带 uuid/clock）。
// 从 runtime.go NewRuntime L443-452 抽离，保持 bootstrap 包级 private function。
package bootstrap

import (
	"context"
	"log/slog"
	"time"

	actionsapp "github.com/guliangiam/guliangiam/internal/app/actions"
	idvapp "github.com/guliangiam/guliangiam/internal/app/idv"
	igaapp "github.com/guliangiam/guliangiam/internal/app/iga"
	meteringapp "github.com/guliangiam/guliangiam/internal/app/metering"
	apprebac "github.com/guliangiam/guliangiam/internal/app/rebac"
	riskapp "github.com/guliangiam/guliangiam/internal/app/risk"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appwebhookevents "github.com/guliangiam/guliangiam/internal/app/webhookevents"
	workloadapp "github.com/guliangiam/guliangiam/internal/app/workload"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	"github.com/guliangiam/guliangiam/internal/platform/recover"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// rbacServices 聚合 ReBAC 引擎与 7 个 RBAC 域 service。
type rbacServices struct {
	RebacRepo     apprebac.Repository
	RebacEngine   *apprebac.Engine
	RebacService  *apprebac.Service
	Risk          riskapp.Service
	Actions       actionsapp.Service
	Metering      meteringapp.Service
	IGA           igaapp.Service
	Workload      workloadapp.Service
	WebhookEvents appwebhookevents.Service
	Idv           idvapp.Service
}

// wireRbacServices 构造 ReBAC 引擎 + 7 个 RBAC 域 service，聚合为 rbacServices。
// 全部构造零副作用，按依赖顺序：rebacRepo → rebacEngine → services。
func wireRbacServices(
	unitOfWork appuow.UnitOfWork,
	clockProvider clock.Clock,
	uuidGenerator platformuuid.Generator,
) rbacServices {
	// ─── ReBAC 双基线 repo 选择 ────────────────────────────────────────────────
	// driver 决定 rebac 存储后端：pgx 走 unitOfWork.Rebac()（共享 pgxpool 连接），
	// mysql 走 mysqluow.Rebac() → mysqlrebac.NewRepository（独立 *sql.DB）。
	// 两个实现都满足 rebacdomain.Repository 接口，Engine 求值器层零感知。
	rebacRepo := unitOfWork.Rebac()
	rebacEngine := apprebac.NewEngine(rebacRepo)
	// 写用例（CreateNamespace 等）走 app 层：UUID、事务与审计都在 Service 内。
	rebacService := apprebac.NewService(clockProvider, uuidGenerator, unitOfWork)
	riskService := riskapp.NewService(unitOfWork)
	actionsService := actionsapp.NewService(unitOfWork, uuidGenerator, clockProvider)
	meteringService := meteringapp.NewService(unitOfWork)
	igaService := igaapp.NewService(unitOfWork)
	workloadService := workloadapp.NewService(unitOfWork)
	webhookeventsService := appwebhookevents.NewService(unitOfWork)
	idvStoreService := idvapp.NewService(unitOfWork)
	return rbacServices{
		RebacRepo:     rebacRepo,
		RebacEngine:   rebacEngine,
		RebacService:  &rebacService,
		Risk:          riskService,
		Actions:       actionsService,
		Metering:      meteringService,
		IGA:           igaService,
		Workload:      workloadService,
		WebhookEvents: webhookeventsService,
		Idv:           idvStoreService,
	}
}

func startIGAExpireWorker(ctx context.Context, uow appuow.UnitOfWork) *igaapp.ExpireWorker {
	worker := igaapp.NewExpireWorker(uow, time.Minute, 100, nil, nil)
	recover.SafeGo("iga-expire-worker", func() {
		if err := worker.Run(ctx); err != nil && ctx.Err() == nil {
			slog.Error("iga expire worker stopped", "error", err)
		}
	})
	return &worker
}
