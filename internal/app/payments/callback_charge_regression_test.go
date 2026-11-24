package payments

import (
	"context"
	"errors"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	pmock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"go.uber.org/mock/gomock"
)

// ─── BE-C-08：回调推进必须落库 payments.Update ─────────────────────────────

// 状态推进必须持久化：payment 写入 succeeded 且 revision 递增，receipt 才进 accepted。
func TestCallbackPipeline_PersistsPaymentAdvance(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID: "pay-1", OrganizationID: "org-1", OutOrderId: "order-1",
			State: domain.StateProcessing, Provider: "stripe", Revision: 3,
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})
	p := newPipeline(receipts, payments, reg)

	_, advanced, err := p.Process(context.Background(), "stripe", []byte(`{"ok":1}`), nil, "", "org-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !advanced {
		t.Fatal("expected advance")
	}
	if len(payments.updated) != 1 {
		t.Fatalf("payment Update must be persisted exactly once, got %d", len(payments.updated))
	}
	saved := payments.updated[0]
	if saved.State != domain.StateSucceeded {
		t.Errorf("persisted state = %s, want succeeded", saved.State)
	}
	if saved.Revision != 4 {
		t.Errorf("persisted revision = %d, want 4", saved.Revision)
	}
	if saved.UpdatedAt.IsZero() {
		t.Error("persisted updated_at must be set")
	}
	last := receipts.updatedRecs[len(receipts.updatedRecs)-1]
	if last.ProcessState != domain.ReceiptStateAccepted {
		t.Errorf("receipt final state = %s, want accepted", last.ProcessState)
	}
}

// payment 落库失败 → receipt 不得进 accepted（否则结算丢失且重试短路）。
func TestCallbackPipeline_PaymentPersistFailureRejectsReceipt(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID: "pay-1", OrganizationID: "org-1", OutOrderId: "order-1",
			State: domain.StateProcessing, Provider: "stripe",
		},
		updateErrs: []error{errors.New("db down")},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})
	p := newPipeline(receipts, payments, reg)

	_, advanced, err := p.Process(context.Background(), "stripe", []byte(`{"ok":1}`), nil, "", "org-1")
	if err == nil {
		t.Fatal("expected error when payment persistence fails")
	}
	if advanced {
		t.Error("advanced must be false when the advance was not persisted")
	}
	last := receipts.updatedRecs[len(receipts.updatedRecs)-1]
	if last.ProcessState == domain.ReceiptStateAccepted {
		t.Fatal("receipt must NOT be accepted when payment write failed")
	}
	if last.ProcessState != domain.ReceiptStateRejected {
		t.Errorf("receipt final state = %s, want rejected", last.ProcessState)
	}
}

// 去重查询非 NotFound 错误必须上抛，不得当成首次投递重复处理。
func TestCallbackPipeline_DedupeLookupErrorPropagates(t *testing.T) {
	receipts := &stubReceiptsRepo{lookupErr: errors.New("connection reset")}
	payments := &stubPaymentsRepo{}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})
	p := newPipeline(receipts, payments, reg)

	_, _, err := p.Process(context.Background(), "stripe", []byte(`{"ok":1}`), nil, "", "org-1")
	if err == nil {
		t.Fatal("dedupe lookup error must propagate")
	}
	if receipts.createdRec != nil {
		t.Error("must not create a receipt when dedupe lookup failed")
	}
}

// NotFound 仍是首次投递，正常走管线。
func TestCallbackPipeline_DedupeNotFoundProceeds(t *testing.T) {
	receipts := &stubReceiptsRepo{lookupErr: apperrors.NewNotFound("receipt not found")}
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID: "pay-1", OrganizationID: "org-1", OutOrderId: "order-1",
			State: domain.StateProcessing, Provider: "stripe",
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})
	p := newPipeline(receipts, payments, reg)

	_, advanced, err := p.Process(context.Background(), "stripe", []byte(`{"ok":1}`), nil, "", "org-1")
	if err != nil {
		t.Fatalf("NotFound must be treated as first delivery: %v", err)
	}
	if !advanced {
		t.Error("expected advance on first delivery")
	}
}

// ─── BE-C-09：退款编排器 org 归属断言 ──────────────────────────────────────

func TestRefundOrchestrator_Execute_CrossOrgRejected(t *testing.T) {
	refunds := &refundsTracker{}
	reg := NewGatewayRegistry()
	reg.Register("alipay", &stubChannelGateway{name: "alipay"})
	orch := &RefundOrchestrator{
		Clock:    fixedClock{},
		UUID:     fixedUUID{s: "r-x"},
		Payments: refundOrgPayments("org-other"),
		Refunds:  refunds,
		Registry: reg,
		LockPayment: func(ctx context.Context, pid string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{Price: 100, Status: domain.StateSucceeded, Provider: "alipay"}, nil
		},
	}
	_, err := orch.Execute(context.Background(), "p-1", "org-1", 20, "")
	if apperrors.FromError(err).Code != apperrors.CodeForbidden {
		t.Fatalf("err = %v, want forbidden for cross-org refund", err)
	}
	if refunds.called != 0 {
		t.Error("cross-org refund must not create a refund row")
	}
}

// ─── BE-C-10：charge 前 CAS 认领 ───────────────────────────────────────────

// chargeObservingGateway 记录 gw.Charge 被调时 payment 已持久化的状态。
type chargeObservingGateway struct {
	stubChannelGateway
	persistedState *string
	calls          int
	stateAtCharge  string
}

func (g *chargeObservingGateway) Charge(ctx context.Context, req domain.ChargeRequest) (*domain.ChargeResult, *domain.ChannelError) {
	g.calls++
	if g.persistedState != nil {
		g.stateAtCharge = *g.persistedState
	}
	return g.stubChannelGateway.Charge(ctx, req)
}

// gw.Charge 被调用时，created→processing 的认领必须已经提交。
func TestChargeOrchestrator_Execute_ClaimCommittedBeforeCharge(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := pmock.NewMockRepository(ctrl)
	payment := &domain.Payment{
		ID: "p-claim", OrganizationID: "org-1", Price: 10,
		Provider: "stripe", State: domain.StateCreated,
	}
	persisted := ""
	repo.EXPECT().GetByID(gomock.Any(), "p-claim").Return(payment, nil).Times(3)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *domain.Payment) error {
			persisted = p.State
			return nil
		},
	).Times(2)

	gw := &chargeObservingGateway{
		stubChannelGateway: stubChannelGateway{
			name:   "stripe",
			result: &domain.ChargeResult{State: domain.AttemptStateSucceeded, ChannelOrderNo: "ch-1"},
		},
		persistedState: &persisted,
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", gw)

	orch := &ChargeOrchestrator{
		Payments: repo, Registry: reg, Clock: fixedClock{}, UUID: fixedUUID{s: "u"},
		Configs: stubChannelConfigs{cfg: activeChannelConfig()}, Runner: newStubChargeRunner(repo),
	}
	if _, err := orch.Execute(context.Background(), "p-claim", "org-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gw.calls != 1 {
		t.Fatalf("gw.Charge calls = %d, want 1", gw.calls)
	}
	if gw.stateAtCharge != domain.StateProcessing {
		t.Fatalf("state at gw.Charge = %q, claim must be committed first", gw.stateAtCharge)
	}
}

// 并发双 charge：事务内重读发现已被认领（processing）→ 拒绝且不再调渠道。
func TestChargeOrchestrator_Execute_ConcurrentClaimRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := pmock.NewMockRepository(ctrl)
	created := &domain.Payment{
		ID: "p-race", OrganizationID: "org-1", Price: 10,
		Provider: "stripe", State: domain.StateCreated,
	}
	claimedByOther := &domain.Payment{
		ID: "p-race", OrganizationID: "org-1", Price: 10,
		Provider: "stripe", State: domain.StateProcessing, AttemptCount: 1,
	}
	gomock.InOrder(
		repo.EXPECT().GetByID(gomock.Any(), "p-race").Return(created, nil),
		repo.EXPECT().GetByID(gomock.Any(), "p-race").Return(claimedByOther, nil),
	)
	// Update 不允许被调用：认领失败必须整体回滚。

	gw := &chargeObservingGateway{stubChannelGateway: stubChannelGateway{
		name:   "stripe",
		result: &domain.ChargeResult{State: domain.AttemptStateSucceeded, ChannelOrderNo: "ch-2"},
	}}
	reg := NewGatewayRegistry()
	reg.Register("stripe", gw)

	orch := &ChargeOrchestrator{
		Payments: repo, Registry: reg, Clock: fixedClock{}, UUID: fixedUUID{s: "u"},
		Configs: stubChannelConfigs{cfg: activeChannelConfig()}, Runner: newStubChargeRunner(repo),
	}
	_, err := orch.Execute(context.Background(), "p-race", "org-1")
	if apperrors.FromError(err).Code != apperrors.CodePaymentInvalidTransition {
		t.Fatalf("err = %v, want invalid transition on lost claim", err)
	}
	if gw.calls != 0 {
		t.Fatal("loser of the claim race must never call gw.Charge")
	}
}
