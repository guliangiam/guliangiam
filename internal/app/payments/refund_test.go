package payments

import (
	"context"
	"errors"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ─── stub payments repo (already in callback_pipeline_test.go) ────────────
// 使用上面已定义的 stubPaymentsRepo + stubReceiptsRepo + stubGateway

// stubRefundsRepo
type stubRefundsRepo struct {
	createErr error
	updateErr error
	created   *domain.Refund
	updated   *domain.Refund
}

func (s *stubRefundsRepo) Create(ctx context.Context, r *domain.Refund) error {
	s.created = r
	return s.createErr
}
func (s *stubRefundsRepo) GetByID(ctx context.Context, id string) (*domain.Refund, error) {
	return nil, errors.New("not found")
}
func (s *stubRefundsRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Refund, error) {
	return nil, errors.New("not found")
}
func (s *stubRefundsRepo) GetByOutRefundNo(ctx context.Context, channel, outRefundNo string) (*domain.Refund, error) {
	return nil, errors.New("not found")
}
func (s *stubRefundsRepo) ListByPayment(ctx context.Context, paymentID string) ([]*domain.Refund, error) {
	return nil, nil
}
func (s *stubRefundsRepo) ListByPaymentForOrganization(ctx context.Context, paymentID, organizationID string) ([]*domain.Refund, error) {
	return nil, nil
}
func (s *stubRefundsRepo) List(ctx context.Context, orgID string, limit, offset int) ([]*domain.Refund, int64, error) {
	return nil, 0, nil
}
func (s *stubRefundsRepo) Update(ctx context.Context, r *domain.Refund) error {
	s.updated = r
	return s.updateErr
}

// ─── ChargeOrchestrator ────────────────────────────────────────────────────

// 测试 Charge 状态门禁（不依赖 stub.Charge 实现）
func TestChargeOrchestrator_StateGate(t *testing.T) {
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-1",
			State:          domain.StateSucceeded, // 已完成 → 不能再 charge
			Provider:       "stripe",
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	o := NewChargeOrchestrator(clock.SystemClock{}, uuid.V7Generator{}, payments, reg, stubChannelConfigs{cfg: activeChannelConfig()}, newStubChargeRunner(payments))
	_, err := o.Execute(context.Background(), "pay-1", "org-1")
	if err == nil {
		t.Fatal("expected state gate error for non-created payment")
	}
}

func TestChargeOrchestrator_WrongOrg(t *testing.T) {
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-other",
			State:          domain.StateCreated,
			Provider:       "stripe",
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	o := NewChargeOrchestrator(clock.SystemClock{}, uuid.V7Generator{}, payments, reg, stubChannelConfigs{cfg: activeChannelConfig()}, newStubChargeRunner(payments))
	_, err := o.Execute(context.Background(), "pay-1", "org-1")
	if err == nil {
		t.Fatal("expected Forbidden error for wrong org")
	}
}

func TestChargeOrchestrator_ChannelUnavailable(t *testing.T) {
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-1",
			State:          domain.StateCreated,
			Provider:       "unknown-channel",
		},
	}
	reg := NewGatewayRegistry() // empty

	o := NewChargeOrchestrator(clock.SystemClock{}, uuid.V7Generator{}, payments, reg, stubChannelConfigs{cfg: activeChannelConfig()}, newStubChargeRunner(payments))
	_, err := o.Execute(context.Background(), "pay-1", "org-1")
	if err != domain.ErrChannelUnavailable {
		t.Errorf("err = %v, want ErrChannelUnavailable", err)
	}
}

// ─── RefundOrchestrator ────────────────────────────────────────────────────

func TestRefundOrchestrator_InvalidAmount(t *testing.T) {
	o := &RefundOrchestrator{}
	_, err := o.Execute(context.Background(), "pay-1", "org-1", 0, "")
	if err == nil {
		t.Fatal("expected error for zero amount")
	}

	_, err = o.Execute(context.Background(), "pay-1", "org-1", -1.0, "")
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestRefundOrchestrator_LockPaymentError(t *testing.T) {
	lockCalled := false
	o := &RefundOrchestrator{
		LockPayment: func(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
			lockCalled = true
			return nil, errors.New("lock failed")
		},
	}
	_, err := o.Execute(context.Background(), "pay-1", "org-1", 10.0, "")
	if err == nil {
		t.Fatal("expected lock error")
	}
	if !lockCalled {
		t.Error("LockPayment should be called")
	}
}

func TestRefundOrchestrator_NonSettledPayment(t *testing.T) {
	o := &RefundOrchestrator{
		Payments: refundOrgPayments("org-1"),
		LockPayment: func(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{Price: 100, Status: domain.StateProcessing}, nil
		},
	}
	_, err := o.Execute(context.Background(), "pay-1", "org-1", 10.0, "")
	if err == nil {
		t.Fatal("expected error for non-settled payment")
	}
}

func TestRefundOrchestrator_ExceedsAvailable(t *testing.T) {
	o := &RefundOrchestrator{
		Payments: refundOrgPayments("org-1"),
		LockPayment: func(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{RefundedAmount: 30, RefundingAmount: 5, Price: 100, Status: domain.StateSucceeded}, nil // 可用额度 65
		},
	}
	_, err := o.Execute(context.Background(), "pay-1", "org-1", 70.0, "")
	if err == nil {
		t.Fatal("expected error for exceeding available")
	}
}

func TestRefundOrchestrator_Success(t *testing.T) {
	refs := &stubRefundsRepo{}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})
	o := &RefundOrchestrator{
		UUID:     uuid.V7Generator{},
		Clock:    clock.SystemClock{},
		Payments: refundOrgPayments("org-1"),
		Refunds:  refs,
		Registry: reg,
		LockPayment: func(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{
				RefundedAmount: 30, RefundingAmount: 5, Price: 100,
				Status: domain.StateSucceeded, Provider: "stripe",
				OutTradeNo: "order-1", ChannelOrderNo: "ch-1", Currency: "USD",
			}, nil // 可用额度 65
		},
	}
	refund, err := o.Execute(context.Background(), "pay-1", "org-1", 50.0, "test refund")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if refund == nil {
		t.Fatal("expected refund")
	}
	if refund.Amount != 50.0 {
		t.Errorf("amount = %v, want 50", refund.Amount)
	}
	if refund.Channel != "stripe" {
		t.Errorf("channel = %s, want stripe", refund.Channel)
	}
	if refund.State != domain.RefundStateSucceeded {
		t.Errorf("state = %s, want succeeded", refund.State)
	}
	if refund.OutRefundNo == "" {
		t.Error("OutRefundNo should be set")
	}
	if refs.updated == nil || refs.updated.State != domain.RefundStateSucceeded {
		t.Errorf("persisted refund not advanced to succeeded: %+v", refs.updated)
	}
}

func TestRefundOrchestrator_ChannelFailureMarksRefundFailed(t *testing.T) {
	refs := &stubRefundsRepo{}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{
		refundFunc: func(ctx context.Context, req domain.RefundRequest) (*domain.RefundResult, *domain.ChannelError) {
			return nil, &domain.ChannelError{Code: "INSUFFICIENT_FUNDS", Message: "balance too low"}
		},
	})
	o := &RefundOrchestrator{
		UUID:     uuid.V7Generator{},
		Clock:    clock.SystemClock{},
		Payments: refundOrgPayments("org-1"),
		Refunds:  refs,
		Registry: reg,
		LockPayment: func(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{Price: 100, Status: domain.StateSucceeded, Provider: "stripe"}, nil
		},
	}
	if _, err := o.Execute(context.Background(), "pay-1", "org-1", 20.0, ""); err == nil {
		t.Fatal("expected channel failure error")
	}
	if refs.updated == nil {
		t.Fatal("failed refund was not persisted")
	}
	if refs.updated.State != domain.RefundStateFailed {
		t.Errorf("state = %s, want failed", refs.updated.State)
	}
	if refs.updated.ErrorCode == nil || *refs.updated.ErrorCode != "INSUFFICIENT_FUNDS" {
		t.Errorf("error code = %v, want INSUFFICIENT_FUNDS", refs.updated.ErrorCode)
	}
	if refs.updated.CompletedAt == nil {
		t.Error("CompletedAt should be set for a terminal state")
	}
}

func TestRefundOrchestrator_ChannelUnavailable(t *testing.T) {
	o := &RefundOrchestrator{
		Payments: refundOrgPayments("org-1"),
		Refunds:  &stubRefundsRepo{},
		Registry: NewGatewayRegistry(),
		LockPayment: func(ctx context.Context, paymentID string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{Price: 100, Status: domain.StateSucceeded, Provider: "unknown"}, nil
		},
	}
	_, err := o.Execute(context.Background(), "pay-1", "org-1", 10.0, "")
	if err != domain.ErrChannelUnavailable {
		t.Fatalf("err = %v, want ErrChannelUnavailable", err)
	}
}

type stubChargeTx struct {
	repo   domain.Repository
	audits []ChargeAuditEvent
}

func (t *stubChargeTx) PaymentRepo() domain.Repository { return t.repo }

func (t *stubChargeTx) EmbedAudit(ctx context.Context, event ChargeAuditEvent) error {
	t.audits = append(t.audits, event)
	return nil
}

type stubChargeRunner struct{ tx *stubChargeTx }

func (r stubChargeRunner) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx ChargeTx) error) error {
	return fn(ctx, r.tx)
}

func newStubChargeRunner(repo domain.Repository) stubChargeRunner {
	return stubChargeRunner{tx: &stubChargeTx{repo: repo}}
}

type stubChannelConfigs struct {
	cfg *domain.PayChannelConfig
	err error
}

func (s stubChannelConfigs) ResolveActive(ctx context.Context, organizationID, channel string) (*domain.PayChannelConfig, error) {
	return s.cfg, s.err
}

func activeChannelConfig() *domain.PayChannelConfig {
	return &domain.PayChannelConfig{Status: domain.ChannelConfigStatusActive}
}

func TestChargeOrchestrator_ChannelNotActivated(t *testing.T) {
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-1",
			State:          domain.StateCreated,
			Provider:       "stripe",
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	o := NewChargeOrchestrator(clock.SystemClock{}, uuid.V7Generator{}, payments, reg, stubChannelConfigs{cfg: &domain.PayChannelConfig{Status: domain.ChannelConfigStatusDraft}}, newStubChargeRunner(payments))
	_, err := o.Execute(context.Background(), "pay-1", "org-1")
	if err == nil {
		t.Fatal("expected not-activated error for draft channel config")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodePaymentChannelNotActivated {
		t.Fatalf("code = %s, want %s", code, apperrors.CodePaymentChannelNotActivated)
	}
}
