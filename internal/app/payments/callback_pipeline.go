package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// CallbackPipeline 实现五阶段回调管线：
//
//	阶段 1 收件：Transport 层 notify 端点 → 写 pay_callback_receipts
//	阶段 2 验签：adapter.ParseCallback 返回已归一化 CallbackEvent
//	阶段 3 去重：payload_sha256 唯一键 / event_key 唯一键
//	阶段 4 归一化：adapter 输出 → CallbackEvent 结构
//	阶段 5 状态迁移：advance payment 状态机 + Outbox 事件
//
// 每个阶段失败都写入 ErrorMessage 并终止后续。
type CallbackPipeline struct {
	Clock    clock.Clock
	UUID     uuid.Generator
	Payments domain.Repository
	Receipts domain.CallbackReceiptsRepo
	Registry *GatewayRegistry
}

// GatewayRegistry 维护 channel name → domain.ChannelGateway SPI 的映射。
// bootstrap 时调用 RegisterAll 注入 infra/payment factory 创建的 adapter。
type GatewayRegistry struct {
	adapters map[string]domain.ChannelGateway
}

func NewGatewayRegistry() *GatewayRegistry {
	return &GatewayRegistry{adapters: map[string]domain.ChannelGateway{}}
}

func (r *GatewayRegistry) Register(name string, gw domain.ChannelGateway) {
	r.adapters[name] = gw
}

func (r *GatewayRegistry) Get(name string) (domain.ChannelGateway, bool) {
	gw, ok := r.adapters[name]
	return gw, ok
}

func (r *GatewayRegistry) Names() []string {
	names := make([]string, 0, len(r.adapters))
	for n := range r.adapters {
		names = append(names, n)
	}
	return names
}

// Process 是管线入口 — 对应 Transport 层 notify endpoint。
// 返回 receipt.ID 和是否推进了 payment 状态机（调用方据此返回 HTTP 200 OK / 409 Dup）。
func (p *CallbackPipeline) Process(ctx context.Context, channel string, payload []byte, headers map[string][]string, clientIP, organizationID string) (receiptID string, advanced bool, err error) {
	now := p.Clock.Now()
	// ─── 阶段 1 收件 ────────────────────────────────────────────────────
	h := sha256.Sum256(payload)
	sha := hex.EncodeToString(h[:])

	// 先查是否已存在 — 幂等短路。NotFound 才是"首次投递"；其他错误必须上抛，
	// 否则去重失效、重复 payload 会被再次处理（BE-C-08）。
	existing, lookupErr := p.Receipts.GetByPayloadHash(ctx, channel, sha)
	if lookupErr != nil && apperrors.FromError(lookupErr).Code != apperrors.CodeNotFound {
		return "", false, apperrors.WrapInternalf(lookupErr, "lookup callback receipt by payload hash")
	}
	if existing != nil {
		return existing.ID, false, nil // 200 OK，重复请求不报错
	}

	id, err := p.UUID.NewString()
	if err != nil {
		return "", false, apperrors.WrapInternalf(err, "generate receipt id")
	}
	receipt := &domain.CallbackReceipt{
		ID:             id,
		OrganizationID: organizationID,
		Channel:        channel,
		PayloadSHA256:  sha,
		Payload:        payload,
		ReceivedAt:     now,
		ProcessState:   domain.ReceiptStateReceived,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if clientIP != "" {
		receipt.ClientIP = &clientIP
	}
	if err := p.Receipts.Create(ctx, receipt); err != nil {
		return "", false, apperrors.Wrap(apperrors.CodePaymentReceiptStorageFailed, err, "create callback receipt")
	}

	// ─── 阶段 2 验签 ────────────────────────────────────────────────────
	gw, ok := p.Registry.Get(channel)
	if !ok {
		p.markFailed(ctx, receipt, fmt.Sprintf("no adapter registered for channel %s", channel))
		return receipt.ID, false, domain.ErrChannelUnavailable
	}

	cbReq := domain.CallbackRequest{
		Channel:  channel,
		Payload:  payload,
		Headers:  headers,
		ClientIP: clientIP,
	}
	event, parseErr := gw.ParseCallback(ctx, cbReq)
	if parseErr != nil {
		p.markFailed(ctx, receipt, fmt.Sprintf("parse callback failed: %v", parseErr))
		return receipt.ID, false, apperrors.Wrapf(apperrors.CodePaymentVerificationFailed, parseErr, "parse callback: %v", parseErr)
	}

	// ─── 阶段 3+4 归一化 ─────────────────────────────────────────────────
	evtKey := event.EventKey
	outTradeNo := event.OutTradeNo
	receipt.EventKey = &evtKey
	receipt.OutTradeNo = &outTradeNo
	receipt.RawEvent = map[string]any{
		"rawStatus": event.RawStatus,
		"state":     string(event.State),
	}
	if err := receipt.TransitionTo(domain.ReceiptStateNormalized); err != nil {
		return receipt.ID, false, apperrors.WrapInternalf(err, "transition receipt to normalized")
	}
	receipt.UpdatedAt = p.Clock.Now()
	if err := p.Receipts.Update(ctx, receipt); err != nil {
		return receipt.ID, false, apperrors.WrapInternalf(err, "update receipt normalized")
	}

	// ─── 阶段 5 状态迁移 ────────────────────────────────────────────────
	// 按 outTradeNo + channel 查 payment
	payment, err := p.Payments.GetByOutTradeNo(ctx, channel, outTradeNo)
	if err != nil {
		p.markFailed(ctx, receipt, fmt.Sprintf("payment not found: %v", err))
		return receipt.ID, false, apperrors.WrapInternalf(err, "payment lookup failed")
	}

	// 状态机推进
	advanced, advanceErr := advancePaymentFromEvent(payment, event)
	if advanceErr != nil {
		p.markFailed(ctx, receipt, fmt.Sprintf("advance payment: %v", advanceErr))
		return receipt.ID, false, advanceErr
	}

	if advanced {
		// BE-C-08: the state advance must be persisted, not just mutated in
		// memory — an Accepted receipt over a still-`created` payment row is a
		// lost settlement. The payment write lands before the receipt flips to
		// accepted; a retry of the same payload is a no-op on the payment and
		// converges the receipt, so the ordering is crash-safe.
		payment.UpdatedAt = p.Clock.Now()
		payment.Revision++
		if err := p.Payments.Update(ctx, payment); err != nil {
			p.markFailed(ctx, receipt, fmt.Sprintf("persist payment advance: %v", err))
			return receipt.ID, false, apperrors.WrapInternalf(err, "persist payment state advance")
		}
		if err := receipt.TransitionTo(domain.ReceiptStateAccepted); err != nil {
			return receipt.ID, true, apperrors.WrapInternalf(err, "transition receipt to accepted")
		}
		receipt.UpdatedAt = p.Clock.Now()
		if err := p.Receipts.Update(ctx, receipt); err != nil {
			return receipt.ID, true, apperrors.WrapInternalf(err, "update receipt accepted")
		}
	} else {
		if err := receipt.TransitionTo(domain.ReceiptStateDuplicated); err != nil {
			return receipt.ID, false, apperrors.WrapInternalf(err, "transition receipt to duplicated")
		}
		receipt.UpdatedAt = p.Clock.Now()
		if err := p.Receipts.Update(ctx, receipt); err != nil {
			slog.WarnContext(ctx, "callback receipt update failed",
				"receipt_id", receipt.ID, "state", receipt.ProcessState, "error", err)
		}
	}

	return receipt.ID, advanced, nil
}

// markFailed 把 receipt 标记为 rejected 并写入 ErrorMessage。
func (p *CallbackPipeline) markFailed(ctx context.Context, receipt *domain.CallbackReceipt, msg string) {
	if err := receipt.TransitionTo(domain.ReceiptStateRejected); err != nil {
		slog.WarnContext(ctx, "callback receipt reject transition failed",
			"receipt_id", receipt.ID, "state", receipt.ProcessState, "error", err)
		return
	}
	receipt.ErrorMessage = &msg
	receipt.UpdatedAt = p.Clock.Now()
	if err := p.Receipts.Update(ctx, receipt); err != nil {
		slog.WarnContext(ctx, "callback receipt mark-failed update failed",
			"receipt_id", receipt.ID, "error", err)
	}
}

// advancePaymentFromEvent 根据 channel 回调事件推进 payment 状态机。
// 返回 (是否实际变更, error)。幂等：如果 payment 已在目标状态则返回 false。
func advancePaymentFromEvent(payment *domain.Payment, event *domain.CallbackEvent) (bool, error) {
	if event.Amount != nil && !amountConsistent(*event.Amount, payment.Price) {
		return false, apperrors.NewWithMeta(
			apperrors.CodePaymentConsistencyMismatch,
			"callback amount does not match payment amount",
			map[string]any{"field": "amount", "value": *event.Amount, "expected": payment.Price},
		)
	}
	target := attemptStateToPaymentState(event.State)
	if target == "" || target == payment.State {
		return false, nil // 无变更
	}
	if err := validatePaymentTransition(payment, target); err != nil {
		return false, err
	}
	payment.State = target
	payment.ChannelOrderNo = event.ChannelOrderNo
	payment.Message = event.RawStatus
	return true, nil
}

// attemptStateToPaymentState 把 adapter 的 AttemptState 映射到 Payment.State。
func attemptStateToPaymentState(s domain.AttemptState) string {
	switch s {
	case domain.AttemptStateSucceeded:
		return domain.StateSucceeded
	case domain.AttemptStateFailed:
		return domain.StateFailed
	case domain.AttemptStatePending:
		return domain.StateProcessing
	case domain.AttemptStateRejected, domain.AttemptStateUnknown:
		return "" // 不推进状态机
	}
	return ""
}

// 保留 ReceiptNormalized 给后续 refactor 可能使用的归一化 JSON。
func (p *CallbackPipeline) ReceiptNormalized(receipt *domain.CallbackReceipt) (string, error) {
	b, err := json.Marshal(receipt.RawEvent)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func amountConsistent(a, b float64) bool {
	diff := a - b
	return diff <= refundTolerance && diff >= -refundTolerance
}
