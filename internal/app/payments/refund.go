package payments

import (
	"context"
	"log/slog"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ChargeOrchestrator 编排创建支付流程：
// 1. 校验 payment 可用（state = created/processing）
// 2. 从 registry 取 channel adapter
// 3. 调 Charge → 写 pay_attempts + 更新 payment.channel_order_no
// 4. 返回 PayURL / ChannelOrderNo 给调用方
// ChannelConfigResolver resolves the active channel configuration that a
// charge is allowed to use. Implementations live in the infra layer; the app
// layer only depends on this narrow port so the activation rule stays testable
// (AC-PAY-006: charge with a draft config must answer payment_channel_not_activated).
type ChannelConfigResolver interface {
	ResolveActive(ctx context.Context, organizationID, channel string) (*domain.PayChannelConfig, error)
}

// ChargeAuditEvent 是 charge 路径交给事务适配器的审计载荷；审计与 payment
// 状态更新必须同事务提交（design/commerce/payment.md §…「审计: records 同事务写」）。
type ChargeAuditEvent struct {
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	AggregateType  string
	EventType      string
	Payload        map[string]any
}

// ChargeTx 是一次事务内的支付仓储 + 审计能力。
type ChargeTx interface {
	PaymentRepo() domain.Repository
	EmbedAudit(ctx context.Context, event ChargeAuditEvent) error
}

// ChargeRunner 负责开启事务并把 Tx 交给编排；网络调用必须在事务之外完成。
type ChargeRunner interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx ChargeTx) error) error
}

type ChargeOrchestrator struct {
	Clock    clock.Clock
	UUID     uuid.Generator
	Payments domain.Repository
	Registry *GatewayRegistry
	Configs  ChannelConfigResolver
	Runner   ChargeRunner
}

func NewChargeOrchestrator(ck clock.Clock, u uuid.Generator, paymentsRepo domain.Repository, reg *GatewayRegistry, configs ChannelConfigResolver, runner ChargeRunner) *ChargeOrchestrator {
	return &ChargeOrchestrator{Clock: ck, UUID: u, Payments: paymentsRepo, Registry: reg, Configs: configs, Runner: runner}
}

// Execute 发起支付。返回新的 payment 引用（已更新 ChannelOrderNo/State）。
func (c *ChargeOrchestrator) Execute(ctx context.Context, paymentID, organizationID string) (*domain.Payment, error) {
	payment, err := c.Payments.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if payment.OrganizationID != organizationID {
		return nil, apperrors.NewForbidden("payment not belongs to this organization")
	}
	// 状态门禁：只有 created 可以发起 charge
	if err := validateChargeState(payment); err != nil {
		return nil, err
	}
	// AC-PAY-006：渠道未激活（draft/testing/disabled 或缺少配置）一律拒绝 Charge。
	if c.Configs == nil {
		return nil, apperrors.New(apperrors.CodePaymentChannelNotActivated, "channel config resolver is not wired")
	}
	cfg, cfgErr := c.Configs.ResolveActive(ctx, payment.OrganizationID, payment.Provider)
	if cfgErr != nil || cfg == nil || cfg.Status != domain.ChannelConfigStatusActive {
		status := ""
		if cfg != nil {
			status = cfg.Status
		}
		return nil, apperrors.NewWithMeta(
			apperrors.CodePaymentChannelNotActivated,
			"payment channel is not activated",
			map[string]any{"field": "channel", "value": payment.Provider, "status": status},
		)
	}

	gw, ok := c.Registry.Get(payment.Provider)
	if !ok {
		return nil, domain.ErrChannelUnavailable
	}

	// BE-C-10: claim the payment BEFORE any network charge. The CAS
	// created→processing is committed first, so of two concurrent Execute
	// calls exactly one reaches gw.Charge; the loser is rejected on the
	// state gate and never produces a second channel debit. `processing` is
	// the registry's in-flight state (§1.2) — there is no separate `charging`
	// state in the schema, and claiming it is what closes the double-charge
	// window that a post-charge recheck left open.
	claimed := payment
	if err := c.persistChargeOutcome(ctx, payment.ID, organizationID, func(current *domain.Payment) error {
		if err := validateChargeState(current); err != nil {
			return err
		}
		current.State = domain.StateProcessing
		current.AttemptCount++
		claimed = current
		return nil
	}, ChargeAuditEvent{
		Action:        "payment.updated",
		EventType:     "payment.updated",
		ResourceType:  "payment",
		AggregateType: "Payment",
		Payload:       map[string]any{"channel": payment.Provider, "phase": "charge_claimed"},
	}); err != nil {
		return nil, err
	}

	chargeReq := domain.ChargeRequest{
		PaymentID:      claimed.ID,
		OrganizationID: claimed.OrganizationID,
		OutTradeNo:     claimed.OutOrderId,
		Amount:         claimed.Price,
		Currency:       claimed.Currency,
		Subject:        claimed.Name,
		ReturnURL:      claimed.SuccessUrl,
	}
	res, chErr := gw.Charge(ctx, chargeReq)
	if chErr != nil {
		if err := c.persistChargeOutcome(ctx, payment.ID, organizationID, func(current *domain.Payment) error {
			current.State = domain.StateFailed
			current.LastErrorCode = chErr.Code
			current.LastErrorMessage = chErr.Message
			return nil
		}, ChargeAuditEvent{
			Action:        "payment.updated",
			EventType:     "payment.charge.failed",
			ResourceType:  "payment",
			AggregateType: "Payment",
			Payload:       map[string]any{"channel": payment.Provider, "errorCode": chErr.Code, "errorMessage": chErr.Message},
		}); err != nil {
			slog.WarnContext(ctx, "refund: persist failed payment state",
				"payment_id", payment.ID, "error", err)
		}
		return nil, apperrors.NewUnavailablef("channel charge failed: %s", chErr.Message)
	}

	// 推进状态：状态更新 + 审计同事务。The claim already moved the row to
	// processing; this write only attaches the channel's trade references, and
	// re-asserts the claimed state so a concurrent cancel/failure is never
	// overwritten by a late charge result.
	updated := claimed
	err = c.persistChargeOutcome(ctx, payment.ID, organizationID, func(current *domain.Payment) error {
		if current.State != domain.StateProcessing {
			return apperrors.NewWithMeta(
				apperrors.CodePaymentInvalidTransition,
				"payment "+current.ID+" is in state "+current.State+", charge result cannot attach",
				map[string]any{"field": "state", "value": current.State, "expected": domain.StateProcessing},
			)
		}
		current.ChannelOrderNo = res.ChannelOrderNo
		current.PayUrl = res.PayURL
		updated = current
		return nil
	}, ChargeAuditEvent{
		Action:        "payment.captured",
		EventType:     "payment.captured",
		ResourceType:  "payment",
		AggregateType: "Payment",
		Payload:       map[string]any{"channel": payment.Provider, "channelOrderNo": res.ChannelOrderNo, "attemptCount": claimed.AttemptCount},
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// persistChargeOutcome 在事务内重读 payment、应用状态变更、提交更新并写审计。
func (c *ChargeOrchestrator) persistChargeOutcome(ctx context.Context, paymentID, organizationID string, mutate func(*domain.Payment) error, auditChargeEvent ChargeAuditEvent) error {
	if c.Runner == nil {
		return apperrors.NewUnimplemented("payment charge transaction runner is not wired")
	}
	return c.Runner.WithinTransaction(ctx, func(ctx context.Context, tx ChargeTx) error {
		repo := tx.PaymentRepo()
		if repo == nil {
			return apperrors.NewUnimplemented("payment repository is not bound to the transaction")
		}
		current, err := repo.GetByID(ctx, paymentID)
		if err != nil {
			return err
		}
		if current.OrganizationID != organizationID {
			return apperrors.NewForbidden("payment not belongs to this organization")
		}
		if err := mutate(current); err != nil {
			return err
		}
		current.UpdatedAt = c.Clock.Now()
		current.Revision++
		if err := repo.Update(ctx, current); err != nil {
			return apperrors.WrapInternalf(err, "update payment after charge")
		}
		auditChargeEvent.OrganizationID = current.OrganizationID
		auditChargeEvent.ResourceID = current.ID
		return tx.EmbedAudit(ctx, auditChargeEvent)
	})
}

// RefundExecutor 是 transport 层发起退款的 app 端口：实现方（bootstrap）负责
// 事务边界与 tx 绑定的仓储/锁注入，编排逻辑复用 RefundOrchestrator。
type RefundExecutor interface {
	ExecuteRefund(ctx context.Context, paymentID, organizationID string, amount float64, reason string) (*domain.Refund, error)
}

// RefundDetail 是退款读侧 DTO（transport 不直接暴露 domain 聚合）。
type RefundDetail struct {
	ID              string     `json:"id"`
	PaymentID       string     `json:"paymentId"`
	OutRefundNo     string     `json:"outRefundNo"`
	Channel         string     `json:"channel"`
	Amount          float64    `json:"amount"`
	Currency        string     `json:"currency"`
	State           string     `json:"state"`
	Reason          string     `json:"reason,omitempty"`
	ChannelRefundNo string     `json:"channelRefundNo,omitempty"`
	ErrorCode       *string    `json:"errorCode,omitempty"`
	ErrorMessage    *string    `json:"errorMessage,omitempty"`
	RequestedAt     time.Time  `json:"requestedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

func NewRefundDetail(r *domain.Refund) *RefundDetail {
	return &RefundDetail{
		ID:              r.ID,
		PaymentID:       r.PaymentID,
		OutRefundNo:     r.OutRefundNo,
		Channel:         r.Channel,
		Amount:          r.Amount,
		Currency:        r.Currency,
		State:           r.State,
		Reason:          r.Reason,
		ChannelRefundNo: r.ChannelRefundNo,
		ErrorCode:       r.ErrorCode,
		ErrorMessage:    r.ErrorMessage,
		RequestedAt:     r.RequestedAt,
		CompletedAt:     r.CompletedAt,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// ─── RefundOrchestrator ────────────────────────────────────────────────

// RefundOrchestrator 编排退款：
// 1. SELECT FOR UPDATE 锁住 payment
// 2. 计算 AvailableForRefund 确认额度
// 3. 写 pay_refunds 初始状态 (requested)
// 4. 调 Channel.Refund
// 5. 根据渠道结果推进 refund state
//
// UoW 边界由调用方控制 — 本 orchestrator 假设整个流程在一个事务里。
type RefundOrchestrator struct {
	Clock    clock.Clock
	UUID     uuid.Generator
	Payments domain.Repository
	Refunds  domain.RefundsRepo
	Registry *GatewayRegistry
	// LockPayment 是 SELECT FOR UPDATE 的注入点 — postgres/mysql 实现不同。
	LockPayment func(ctx context.Context, paymentID string) (*domain.LockedPayment, error)
}

// Execute 执行一次退款请求。
//
// amount > 0, reason 可选。
func (r *RefundOrchestrator) Execute(ctx context.Context, paymentID, organizationID string, amount float64, reason string) (*domain.Refund, error) {
	if err := validateRefundAmount(amount); err != nil {
		return nil, err
	}

	// ① SELECT FOR UPDATE — 并发互斥
	locked, err := r.LockPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	// BE-C-09: LockedPayment 不携带租户列，锁后必须回读 payment 断言 org 归属，
	// 否则跨租户 paymentID 可以借本租户上下文退款。
	payment, err := r.Payments.GetByID(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	if payment.OrganizationID != organizationID {
		return nil, apperrors.NewForbidden("payment not belongs to this organization")
	}
	if err := validateRefundState(paymentID, locked.Status); err != nil {
		return nil, err
	}
	available := locked.Price - locked.RefundedAmount - locked.RefundingAmount
	if err := validateRefundAvailable(amount, available); err != nil {
		return nil, err
	}

	// ② 解析渠道 adapter
	if r.Registry == nil {
		return nil, domain.ErrChannelUnavailable
	}
	gw, ok := r.Registry.Get(locked.Provider)
	if !ok || !gw.Metadata().SupportsRefund {
		return nil, domain.ErrChannelUnavailable
	}

	// ③ 创建 Refund aggregate（requested 先落库，保证渠道调用可审计）
	now := r.Clock.Now()
	refundID, err := r.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate refund id")
	}
	refund := &domain.Refund{
		ID:             refundID,
		OrganizationID: organizationID,
		PaymentID:      paymentID,
		OutRefundNo:    refundID,
		Channel:        locked.Provider,
		Amount:         amount,
		Currency:       locked.Currency,
		State:          domain.RefundStateRequested,
		Reason:         reason,
		RequestedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := r.Refunds.Create(ctx, refund); err != nil {
		return nil, apperrors.WrapInternalf(err, "persist refund")
	}

	// ④ 调渠道退款 — 失败必须落 failed 终态，不能静默留下 requested
	result, chErr := gw.Refund(ctx, domain.RefundRequest{
		PaymentID:      paymentID,
		OutTradeNo:     locked.OutTradeNo,
		OutRefundNo:    refund.OutRefundNo,
		ChannelOrderNo: locked.ChannelOrderNo,
		Amount:         amount,
		Reason:         reason,
	})
	if chErr != nil {
		markRefundFailed(ctx, r.Refunds, refund, chErr.Code, chErr.Message)
		return nil, apperrors.NewUnavailablef("channel refund failed: %s", chErr.Message)
	}
	if result == nil {
		markRefundFailed(ctx, r.Refunds, refund, "EMPTY_CHANNEL_RESULT", "channel returned no refund result")
		return nil, apperrors.NewInternal("channel refund returned no result")
	}

	// ⑤ 按渠道结果推进状态
	refund.ChannelRefundNo = result.ChannelRefundNo
	switch result.State {
	case domain.AttemptStateSucceeded:
		refund.State = domain.RefundStateSucceeded
		completed := now
		refund.CompletedAt = &completed
	case domain.AttemptStateFailed, domain.AttemptStateRejected:
		refund.State = domain.RefundStateFailed
		completed := now
		refund.CompletedAt = &completed
		setRefundError(refund, result.ErrorCode, result.ErrorMessage)
	case domain.AttemptStatePending, domain.AttemptStateUnknown:
		// pending / unknown — 保持非终态，等待查询或回调收敛（UNKNOWN 不推断）
		refund.State = domain.RefundStateProcessing
	}
	refund.UpdatedAt = r.Clock.Now()
	if err := r.Refunds.Update(ctx, refund); err != nil {
		return nil, apperrors.WrapInternalf(err, "update refund after channel call")
	}
	return refund, nil
}

// markRefundFailed records a channel-side failure and persists it. A persistence
// failure is logged but does not mask the original channel error.
func markRefundFailed(ctx context.Context, repo domain.RefundsRepo, refund *domain.Refund, code, message string) {
	refund.State = domain.RefundStateFailed
	completed := refund.RequestedAt
	refund.CompletedAt = &completed
	setRefundError(refund, code, message)
	refund.UpdatedAt = completed
	if err := repo.Update(ctx, refund); err != nil {
		slog.WarnContext(ctx, "refund: persist failed state",
			"refund_id", refund.ID, "payment_id", refund.PaymentID, "error", err)
	}
}

func setRefundError(refund *domain.Refund, code, message string) {
	if code != "" {
		c := code
		refund.ErrorCode = &c
	}
	if message != "" {
		m := message
		refund.ErrorMessage = &m
	}
}

// ─── 辅助 ──────────────────────────────────────────────────────────────

// nowPtr 把 time.Time 转成 *time.Time 给 nullable 字段。
func nowPtr(t time.Time) *time.Time { return &t }
