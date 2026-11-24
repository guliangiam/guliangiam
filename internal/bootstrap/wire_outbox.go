// wire_outbox.go — Outbox Worker (ADR-008) + Idempotency Repo (ADR-018)。
// 从 runtime.go 的 NewRuntime 中抽离，保持 bootstrap 包级 private function。
package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	aigatewaydomain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	domainidem "github.com/guliangiam/guliangiam/internal/domain/idempotency"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	providerdomain "github.com/guliangiam/guliangiam/internal/domain/providers"

	appgateway "github.com/guliangiam/guliangiam/internal/app/aigateway"
	appproviders "github.com/guliangiam/guliangiam/internal/app/providers"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	appusernotifications "github.com/guliangiam/guliangiam/internal/app/usernotifications"
	appwebhooks "github.com/guliangiam/guliangiam/internal/app/webhooks"

	mysqlag "github.com/guliangiam/guliangiam/internal/infra/mysql/aigateway"
	mysqlidem "github.com/guliangiam/guliangiam/internal/infra/mysql/idempotency"
	mysqloutbox "github.com/guliangiam/guliangiam/internal/infra/mysql/outbox"
	mysqlssf "github.com/guliangiam/guliangiam/internal/infra/mysql/ssf"
	infrapg "github.com/guliangiam/guliangiam/internal/infra/postgres"
	infraaigateway "github.com/guliangiam/guliangiam/internal/infra/postgres/aigateway"
	infraidem "github.com/guliangiam/guliangiam/internal/infra/postgres/idempotency"
	infraoutbox "github.com/guliangiam/guliangiam/internal/infra/postgres/outbox"
	infrarecords "github.com/guliangiam/guliangiam/internal/infra/postgres/records"
	infrassf "github.com/guliangiam/guliangiam/internal/infra/postgres/ssf"
	infrawebhook "github.com/guliangiam/guliangiam/internal/infra/webhook"
	providerlogoutoutbound "github.com/guliangiam/guliangiam/internal/outbound/providerlogout"
	ssfoutbound "github.com/guliangiam/guliangiam/internal/outbound/ssf"
	platformclock "github.com/guliangiam/guliangiam/internal/platform/clock"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
	auditprovider "github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"
	tentxtx "github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// outboxDeps 聚合 Outbox/幂等/Webhook 投递与审计链 Worker。
type outboxDeps struct {
	OutboxWorker     *platformoutbox.Worker
	Idempotency      domainidem.Repository
	OutboxRepo       domainoutbox.Repository
	SSFSigner        *ssfoutbound.SignerHolder
	AuditChainWorker *auditprovider.ChainWorker
}

// wireOutboxDeps 构造 Outbox/幂等/Webhook/审计链依赖，聚合为 outboxDeps。
// Worker 已 Start() — Runtime.Workers 字段持有引用用于优雅关闭。
// OutboxRepo 是 Outbox Enqueue 端口（domain/outbox.Repository），
// 共享给其他模块（如 AI Gateway）写入审计事件到同一张 outbox 表。
//
// 方言边界（ADR-023 / ADR-031）：
//   - PostgreSQL：outbox / idempotency / metering / SSF / 审计链 worker 全部启用。
//   - MySQL：outbox / idempotency / metering 使用 mysql 镜像仓储；审计哈希链
//     按 ADR-031 不提供（不启动 ChainWorker）；SSF 投递无 MySQL 镜像，跳过处理器
//     并记录告警，事件保留在 outbox 等待管理员处置，而不是在 nil 连接上 panic。
func wireOutboxDeps(ctx context.Context, pool *pgxpool.Pool, mysqlDB *sql.DB, driver string, unitOfWork appuow.UnitOfWork) outboxDeps {
	// ── Outbox / Idempotency / Metering 仓储按方言选择 ──────────────────
	var (
		platformRepo platformoutbox.Repository
		outboxRepo   domainoutbox.Repository
		idempotency  domainidem.Repository
		meteringRepo aigatewaydomain.MeteringEventRepo
	)
	if driver == driverMySQL {
		mysqlOutboxRepo := mysqloutbox.NewRepository(mysqlDB)
		platformRepo = mysqlOutboxRepo
		outboxRepo = mysqlOutboxRepo
		idempotency = mysqlidem.NewRepository(mysqlDB)
		meteringRepo = mysqlag.NewMeteringEventRepo(mysqlDB)
	} else {
		pgOutboxRepo := infraoutbox.NewRepository(infrapg.NewInstrumentedDBTX(pool), pool)
		platformRepo = pgOutboxRepo
		outboxRepo = pgOutboxRepo
		idempotency = infraidem.NewRepository(infrapg.NewInstrumentedDBTX(pool), pool)
		meteringRepo = infraaigateway.NewMeteringEventRepo(pool)
	}

	// ── Outbox Worker (ADR-008) ──
	// H40：webhook 投递统一走 outbox 单轨。遗留 webhook_events 直轮询 Worker
	// 已退役；DeliveryOutboxHandler 在投递同时维护 webhook_events 台账。
	webhookDeliverySender := infrawebhook.NewHTTPDeliverySender(nil, 30*time.Second)

	// Handler Registry: aggregate_type → DeliveryHandler.
	// - webhook: 真实 HTTP 投递 + 台账维护（app/webhooks，HMAC 签名在 infra/webhook sender）
	// - llm_provider / llm_quota / llm_usage: 两层 sink
	//     (a) llm_metering_events — AI Gateway 审计表
	//     (b) metering_events — Commerce billing 计费表 (仅 llm_usage + ai.usage.recorded)
	ssfSignerHolder := ssfoutbound.NewSignerHolder()
	outboxRegistry := platformoutbox.MapRegistry{
		appproviders.ConsistencyAggregateType: appproviders.NewConsistencyOutboxHandler(slog.Default()),
		"webhook":                             appwebhooks.NewDeliveryOutboxHandler(unitOfWork, webhookDeliverySender, slog.Default()),
	}
	// 提供方登出（S-12）：登出请求只做本地吊销 + 入队；外部 POST 由本 Handler 在
	// Worker 内执行，重试/退避/死信沿用既有 Outbox 配置。无 KEK 时不会有携带
	// 令牌的事件入队；cipher 为 nil 时该 Handler 对历史事件直接死信。
	var providerLogoutCipher providerlogoutoutbound.TokenDecrypter
	if platformcrypto.FieldEncryptionConfigured() {
		providerLogoutCipher = platformcrypto.GlobalFieldEncryptor()
	}
	outboxRegistry[providerlogoutoutbound.AggregateType] = providerlogoutoutbound.NewHandler(
		systemProviderLoader{uow: unitOfWork}, providerLogoutCipher, nil,
	)
	switch {
	case driver == driverMySQL && mysqlDB != nil:
		outboxRegistry[ssfoutbound.AggregateType] = ssfoutbound.NewHandler(
			mysqlssf.NewReceiverRepository(mysqlDB), ssfSignerHolder, ssrf.SafeHTTPClient(10*time.Second),
		)
	case pool != nil:
		outboxRegistry[ssfoutbound.AggregateType] = ssfoutbound.NewHandler(
			infrassf.NewReceiverRepository(pool), ssfSignerHolder, ssrf.SafeHTTPClient(10*time.Second),
		)
	default:
		slog.Warn("ssf outbox delivery is unavailable on this database driver; events stay queued",
			"driver", driver, "aggregate_type", ssfoutbound.AggregateType)
	}
	billingRepo := unitOfWork.Metering() // Commerce metering_events repo
	// 个人收件箱事件处理器：领域事件（注册欢迎）与 AI 网关的配额/熔断告警
	// 共用同一个处理器，保证这些运维事件也出现在通知中心而不是只写审计。
	var inboxHandler *appusernotifications.InboxEventHandler
	if inboxUOW, ok := unitOfWork.(appuow.UserNotificationsUoW); ok {
		inboxService := appusernotifications.NewService(platformclock.SystemClock{}, platformuuid.V7Generator{}, inboxUOW)
		inboxService.Contacts = notificationContactResolver{uow: inboxUOW}
		inboxHandler = appusernotifications.NewInboxEventHandler(inboxService, slog.Default())
	} else {
		slog.Warn("unit of work does not expose UserNotifications; inbox event fan-out disabled")
	}
	// metering_events.id 是 UUID 列 —— Handler 生成计费事实主键必须用平台生成器，
	// 不能用 outbox_id + 后缀拼接字符串。
	for k, h := range appgateway.NewOutboxHandlers(meteringRepo, slog.Default(), billingRepo, platformuuid.V7Generator{}) {
		// 同一 aggregate 只允许一个 handler：AI 配额/熔断在计量之外还要写收件箱，
		// 用组合 handler 扇出（收件箱写入对事件重投幂等）。
		if inboxHandler != nil && (k == "llm_provider" || k == "llm_quota") {
			outboxRegistry[k] = compositeOutboxHandler{handlers: []platformoutbox.DeliveryHandler{h, inboxHandler}}
			continue
		}
		outboxRegistry[k] = h
	}
	// 领域事件（Organization/risk/user/session）扇出到租户 webhook 订阅者。
	// 这些聚合此前没有 handler，Worker 会把事件直接转死信。
	domainEventHandler := appwebhooks.NewDomainEventOutboxHandler(unitOfWork, slog.Default())
	domainHandlers := []platformoutbox.DeliveryHandler{domainEventHandler}
	// 同一条领域事件还要落个人收件箱：注册欢迎等系统消息由 outbox 驱动，
	// 与 webhook 扇出共用一次投递（收件箱写入对事件重投幂等）。
	if inboxHandler != nil {
		domainHandlers = append(domainHandlers, inboxHandler)
	}
	domainFanout := compositeOutboxHandler{handlers: domainHandlers}
	for _, aggregateType := range appwebhooks.DomainEventAggregateTypes {
		outboxRegistry[aggregateType] = domainFanout
	}
	outboxWorker := platformoutbox.NewWorker(
		platformRepo, outboxRegistry, "main",
		platformoutbox.WithPollInterval(5*time.Second),
		platformoutbox.WithBackoff(time.Second, 32*time.Second),
		platformoutbox.WithDeliveryObserver(newAuditSyslogObserver(uowSyslogProviderLookup{uow: unitOfWork})),
	)
	outboxWorker.Start(ctx)

	// ── Audit Chain Worker (ADR-031) ──
	// AuditChainWriter 持有最小 dbtx 端口（pgxpool.Pool 满足）。
	// Worker 每 5min 锚定 + 每 15min 验证链完整性。
	// MySQL 部署不提供审计哈希链（ADR-031 决策），因此不构造也不启动。
	var auditChainWorker *auditprovider.ChainWorker
	if pool != nil {
		auditChainWriter := auditprovider.NewAuditChainWriter(pool, infrarecords.DefaultChainSQL())
		auditChainWorker = auditprovider.NewChainWorker(auditChainWriter, 5*time.Minute, 15*time.Minute, slog.Default())
		auditChainWorker.Start(ctx)
	}

	return outboxDeps{
		OutboxWorker:     outboxWorker,
		Idempotency:      idempotency,
		OutboxRepo:       outboxRepo,
		SSFSigner:        ssfSignerHolder,
		AuditChainWorker: auditChainWorker,
	}
}

// compositeOutboxHandler 让同一个 aggregate_type 按顺序执行多个投递处理器：
// 既有的 webhook 扇出与个人收件箱写入互不覆盖。任一处理器返回错误即整体重投，
// 各处理器自身需对事件重投幂等。
type compositeOutboxHandler struct {
	handlers []platformoutbox.DeliveryHandler
}

func (h compositeOutboxHandler) Handle(ctx context.Context, item *platformoutbox.OutboxItem) error {
	for _, handler := range h.handlers {
		if err := handler.Handle(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// notificationContactResolver 解析用户的邮箱/手机与渠道绑定，供收件箱
// Publisher 在多渠道路由时使用。用户与绑定表都是 FORCE RLS，必须在系统
// 作用域事务内读取（outbox worker 没有请求租户）。
type notificationContactResolver struct {
	uow appuow.UserNotificationsUoW
}

func (r notificationContactResolver) ResolveContacts(ctx context.Context, organizationID, userID string) (appusernotifications.Contacts, error) {
	var contacts appusernotifications.Contacts
	err := r.uow.WithinTransaction(tentxtx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		user, err := tx.Users().GetByIDForOrganization(ctx, userID, organizationID)
		if err != nil {
			return err
		}
		contacts.Email = user.Profile.Email
		contacts.Phone = user.Profile.Phone

		scoped, ok := tx.(appuow.UserNotificationsUoW)
		if !ok {
			return nil
		}
		bindings, err := scoped.UserNotifications().ListChannelBindings(ctx, organizationID, userID)
		if err != nil {
			return err
		}
		if len(bindings) > 0 {
			contacts.Bindings = make(map[string]string, len(bindings))
			for _, binding := range bindings {
				// 只使用已校验且外部 ID 非空的绑定：未校验绑定不参与投递，
				// 防止把本人的通知推到未确认的企业微信/公众号身份。
				if !binding.Verified || binding.ExternalID == "" {
					continue
				}
				switch binding.Channel {
				case "wecom_app", "wechat_official":
					contacts.Bindings[binding.Channel] = binding.ExternalID
				}
			}
		}
		return nil
	})
	if err != nil {
		return appusernotifications.Contacts{}, err
	}
	return contacts, nil
}

// systemProviderLoader 为无请求租户上下文的 Outbox Handler 读取提供方配置。// providers 是 FORCE RLS 表，必须在系统作用域事务内读取（DEF-35/DEF-46）。
type systemProviderLoader struct {
	uow appuow.UnitOfWork
}

func (l systemProviderLoader) GetProvider(ctx context.Context, providerID string) (*providerdomain.Provider, error) {
	var provider *providerdomain.Provider
	err := l.uow.WithinTransaction(tentxtx.System(ctx), func(ctx context.Context, tx appuow.UnitOfWork) error {
		loaded, loadErr := tx.Providers().GetByID(ctx, providerID)
		if loadErr != nil {
			return loadErr
		}
		provider = loaded
		return nil
	})
	if err != nil {
		return nil, err
	}
	return provider, nil
}
