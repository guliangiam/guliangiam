package outbox

import (
	"context"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// OutboxItem 是 guliangiam.outbox 的 Domain 层表示。
//
// 写入方式：业务事务内同时写 outbox 行（与业务事实同事务）。
// 读取方式：Worker 通过 Repository.LockBatch() 租约取件（SELECT ... FOR UPDATE SKIP LOCKED）。
type OutboxItem struct {
	ID             string
	OrganizationID *string
	AggregateType  string
	AggregateID    string
	EventType      string
	EventVersion   int
	Payload        []byte
	Headers        []byte
	Status         Status
	Attempts       int
	MaxAttempts    int
	AvailableAt    time.Time
	LockedBy       *string
	LockedAt       *time.Time
	ProcessedAt    *time.Time
	LastError      *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Revision       int64
}

// Validate 校验必填字段 + 状态机合法性 + attempts/max_attempts 边界。
func (o *OutboxItem) Validate() error {
	if o.ID == "" {
		return apperrors.NewInvalidArgument("outbox.id is required")
	}
	if o.AggregateType == "" {
		return apperrors.NewInvalidArgument("outbox.aggregateType is required")
	}
	if o.AggregateID == "" {
		return apperrors.NewInvalidArgument("outbox.aggregateId is required")
	}
	if o.EventType == "" {
		return apperrors.NewInvalidArgument("outbox.eventType is required")
	}
	if !IsValidStatus(string(o.Status)) {
		return apperrors.NewInvalidArgument("outbox.status must be one of pending|processing|succeeded|dead|failed")
	}
	if o.MaxAttempts <= 0 {
		return apperrors.NewInvalidArgument("outbox.maxAttempts must be > 0")
	}
	if o.Attempts < 0 || o.Attempts > o.MaxAttempts {
		return apperrors.NewInvalidArgument("outbox.attempts out of range [0, maxAttempts]")
	}
	return nil
}

// IsDead 报告该 item 是否已进入死信（attempts >= max）。
func (o *OutboxItem) IsDead() bool {
	return o.Attempts >= o.MaxAttempts
}

// NextAvailableTime 计算下一次可用时间（指数退避 + base 从 1s 起）。
func (o *OutboxItem) NextAvailableTime(now time.Time, base, maxBackoff time.Duration) time.Time {
	return now.Add(BackoffDuration(o.Attempts, base, maxBackoff))
}

// Repository 是 outbox 仓储端口。
//
// 关键语义：
//   - LockBatch   : Worker 租约取件（SELECT ... FOR UPDATE SKIP LOCKED + UPDATE status=processing）
//   - MarkSuccess : 标记投递成功
//   - MarkRetry   : 递增 attempts + 设 available_at = 退避后时间 + 回到 pending
//   - MarkDead    : attempts >= max → 写 outbox_dead_letters 并置 dead
//   - Insert      : 业务事务内调用（同 Transaction）
type Repository interface {
	// Insert 在业务事务内写入 outbox 行。与业务事实同生共死。
	Insert(ctx context.Context, item *OutboxItem) error

	// LockBatch 租约取件：一次最多 limit 个 pending 行，设 locked_by + locked_at + status=processing。
	// 事务内执行 SELECT ... FOR UPDATE SKIP LOCKED 保证多 worker 不重复取。
	LockBatch(ctx context.Context, limit int, workerID string, lockTTL time.Duration) ([]*OutboxItem, error)

	// MarkSuccess 标记投递成功。
	MarkSuccess(ctx context.Context, id string, processedAt time.Time) error

	// MarkRetry 递增 attempts + 设置 available_at（指数退避后）+ 回到 pending。
	// attempts 达到 max 后 Repository 应自动走 MarkDead 流程。
	MarkRetry(ctx context.Context, id string, errMsg string, now, availableAt time.Time) error

	// MarkDead 移至死信状态：写 outbox_dead_letters + 置 outbox.status=dead。
	MarkDead(ctx context.Context, id string, errMsg string, deadAt time.Time) error
}

// DeliveryHandler 是 Worker 调用的投递接口。
// 每个 aggregate_type 对应一个 Handler（webhook / sync / notification）。
// 由 app 层注册，Worker 只知道这个接口。
type DeliveryHandler interface {
	Handle(ctx context.Context, item *OutboxItem) error
}

// DeliveryHandlerRegistry 是 Handler 注册表。aggregate_type → Handler。
// Worker 通过 Registry 查找 Handler，找不到的 aggregate_type 会被 MarkDead。
type DeliveryHandlerRegistry interface {
	Register(aggregateType string, handler DeliveryHandler)
	Get(aggregateType string) (DeliveryHandler, bool)
}

// DeliveryObserver 在一条事件成功投递后被调用，用于旁路 sink（例如审计
// syslog）。它不参与投递成败：Observer 返回错误只记录告警，不回滚已成功的
// outbox 行，避免业务事件因旁路不可用而被重复投递。
type DeliveryObserver interface {
	Observe(ctx context.Context, item *OutboxItem) error
}

// MapRegistry is a simple map-based DeliveryHandlerRegistry.
// Bootstrap can use map[string]DeliveryHandler directly via &MapRegistry{...}.
type MapRegistry map[string]DeliveryHandler

func (r MapRegistry) Register(aggregateType string, h DeliveryHandler) { r[aggregateType] = h }
func (r MapRegistry) Get(aggregateType string) (DeliveryHandler, bool) {
	h, ok := r[aggregateType]
	return h, ok
}
