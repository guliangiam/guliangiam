package payments

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	pmock "github.com/guliangiam/guliangiam/internal/domain/payments/mock"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
	"go.uber.org/mock/gomock"
)

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) }

var _ clock.Clock = fixedClock{}

type fixedUUID struct{ s string }

func (f fixedUUID) NewString() (string, error) { return f.s, nil }

var _ uuid.Generator = fixedUUID{}

// refundOrgPayments 提供 BE-C-09 租户回读所需的 Payments 仓储 stub。
func refundOrgPayments(org string) *stubPaymentsRepo {
	return &stubPaymentsRepo{payment: &domain.Payment{ID: "pay-1", OrganizationID: org}}
}

// ─── mock ChannelGateway ──────────────────────────────────────────

type stubChannelGateway struct {
	name   string
	result *domain.ChargeResult
	cErr   *domain.ChannelError
}

func (s *stubChannelGateway) Metadata() domain.ChannelMetadata {
	return domain.ChannelMetadata{Channel: s.name, SupportsCharge: true, SupportsRefund: true}
}
func (s *stubChannelGateway) Charge(ctx context.Context, req domain.ChargeRequest) (*domain.ChargeResult, *domain.ChannelError) {
	return s.result, s.cErr
}
func (s *stubChannelGateway) Query(ctx context.Context, req domain.QueryRequest) (*domain.QueryResult, *domain.ChannelError) {
	return nil, nil
}
func (s *stubChannelGateway) Refund(ctx context.Context, req domain.RefundRequest) (*domain.RefundResult, *domain.ChannelError) {
	return &domain.RefundResult{State: domain.AttemptStateSucceeded, ChannelRefundNo: "ch-refund-1"}, nil
}
func (s *stubChannelGateway) QueryRefund(ctx context.Context, req domain.QueryRefundRequest) (*domain.QueryRefundResult, *domain.ChannelError) {
	return nil, nil
}
func (s *stubChannelGateway) Close(ctx context.Context, req domain.CloseRequest) (*domain.CloseResult, *domain.ChannelError) {
	return nil, nil
}
func (s *stubChannelGateway) ParseCallback(ctx context.Context, req domain.CallbackRequest) (*domain.CallbackEvent, error) {
	return nil, nil
}
func (s *stubChannelGateway) VerifyConfig(ctx context.Context, cfg domain.ChannelConfig) error {
	return nil
}
func (s *stubChannelGateway) DownloadStatement(ctx context.Context, req domain.StatementRequest) ([]domain.StatementLine, *domain.ChannelError) {
	return nil, nil
}

// ─── ChargeOrchestrator.Execute ───────────────────────────────────

func TestChargeOrchestrator_Execute_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := pmock.NewMockRepository(ctrl)
	payment := &domain.Payment{
		ID:             "p-1",
		OrganizationID: "org-1",
		Name:           "test-plan",
		OutOrderId:     "out-order-1",
		Price:          99.00,
		Currency:       "USD",
		Provider:       "alipay",
		SuccessUrl:     "https://return",
		State:          domain.StateCreated,
	}

	gw := &stubChannelGateway{
		name: "alipay",
		result: &domain.ChargeResult{
			State:          domain.AttemptStateSucceeded,
			ChannelOrderNo: "ch-order-abc",
			PayURL:         "https://pay.alipay.com/xxx",
		},
	}

	reg := NewGatewayRegistry()
	reg.Register("alipay", gw)

	repo.EXPECT().GetByID(gomock.Any(), "p-1").Return(payment, nil).Times(3)
	// BE-C-10：第一次 Update 是 charge 前的 CAS 认领（created→processing），
	// 第二次 Update 才附带渠道单号/PayURL。
	updates := 0
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *domain.Payment) error {
			updates++
			switch updates {
			case 1:
				if p.State != domain.StateProcessing {
					t.Errorf("claim: expected state processing, got %s", p.State)
				}
				if p.ChannelOrderNo != "" {
					t.Errorf("claim must run before gw.Charge, channel order already set: %s", p.ChannelOrderNo)
				}
				if p.AttemptCount != 1 {
					t.Errorf("claim: expected attempt count 1, got %d", p.AttemptCount)
				}
			case 2:
				if p.State != domain.StateProcessing {
					t.Errorf("expected state processing, got %s", p.State)
				}
				if p.ChannelOrderNo != "ch-order-abc" {
					t.Errorf("expected channel order ch-order-abc, got %s", p.ChannelOrderNo)
				}
				if p.PayUrl != "https://pay.alipay.com/xxx" {
					t.Errorf("expected pay url, got %s", p.PayUrl)
				}
				if p.AttemptCount != 1 {
					t.Errorf("expected attempt count 1, got %d", p.AttemptCount)
				}
			default:
				t.Errorf("unexpected extra Update call #%d", updates)
			}
			return nil
		},
	).Times(2)

	orch := &ChargeOrchestrator{
		Payments: repo,
		Registry: reg,
		Clock:    fixedClock{},
		UUID:     fixedUUID{s: "uuid-1"},
		Configs:  stubChannelConfigs{cfg: activeChannelConfig()},
		Runner:   newStubChargeRunner(repo),
	}

	got, err := orch.Execute(context.Background(), "p-1", "org-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil {
		t.Fatal("expected payment")
	}
	if got.State != domain.StateProcessing {
		t.Errorf("expected processing, got %s", got.State)
	}
}

func TestChargeOrchestrator_Execute_ChannelChargeFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := pmock.NewMockRepository(ctrl)
	payment := &domain.Payment{
		ID:             "p-2",
		OrganizationID: "org-1",
		Price:          100.0,
		Currency:       "USD",
		Provider:       "alipay",
		State:          domain.StateCreated,
	}

	gw := &stubChannelGateway{
		name: "alipay",
		cErr: &domain.ChannelError{Code: "TIMEOUT", Message: "provider timeout"},
	}
	reg := NewGatewayRegistry()
	reg.Register("alipay", gw)

	repo.EXPECT().GetByID(gomock.Any(), "p-2").Return(payment, nil).Times(3)
	// 先 CAS 认领 processing，渠道失败后再写 failed + 错误码。
	updates := 0
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, p *domain.Payment) error {
			updates++
			switch updates {
			case 1:
				if p.State != domain.StateProcessing {
					t.Errorf("claim: expected state processing, got %s", p.State)
				}
			case 2:
				if p.State != domain.StateFailed {
					t.Errorf("expected state failed, got %s", p.State)
				}
				if p.LastErrorCode != "TIMEOUT" {
					t.Errorf("expected error code TIMEOUT, got %s", p.LastErrorCode)
				}
			default:
				t.Errorf("unexpected extra Update call #%d", updates)
			}
			return nil
		},
	).Times(2)

	orch := &ChargeOrchestrator{Payments: repo, Registry: reg, Clock: fixedClock{}, Configs: stubChannelConfigs{cfg: activeChannelConfig()}, Runner: newStubChargeRunner(repo)}
	_, err := orch.Execute(context.Background(), "p-2", "org-1")
	if err == nil {
		t.Fatal("expected error on channel charge failure")
	}
}

func TestChargeOrchestrator_Execute_UpdateAfterChargeFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := pmock.NewMockRepository(ctrl)
	payment := &domain.Payment{
		ID:             "p-3",
		OrganizationID: "org-1",
		Price:          100.0,
		Currency:       "USD",
		Provider:       "alipay",
		State:          domain.StateCreated,
	}
	gw := &stubChannelGateway{
		name:   "alipay",
		result: &domain.ChargeResult{State: domain.AttemptStateSucceeded, ChannelOrderNo: "ch-ok"},
	}
	reg := NewGatewayRegistry()
	reg.Register("alipay", gw)

	repo.EXPECT().GetByID(gomock.Any(), "p-3").Return(payment, nil).Times(3)
	// 认领提交成功，charge 之后的结果落库失败必须报错（不能吞掉）。
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(apperrors.NewInternal("db locked"))

	orch := &ChargeOrchestrator{Payments: repo, Registry: reg, Clock: fixedClock{}, Configs: stubChannelConfigs{cfg: activeChannelConfig()}, Runner: newStubChargeRunner(repo)}
	_, err := orch.Execute(context.Background(), "p-3", "org-1")
	if err == nil {
		t.Fatal("expected error when update fails after charge")
	}
}

func TestChargeOrchestrator_Execute_PaymentNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := pmock.NewMockRepository(ctrl)
	repo.EXPECT().GetByID(gomock.Any(), "p-missing").Return(nil, apperrors.NewNotFound("payment not found"))

	orch := &ChargeOrchestrator{Payments: repo, Registry: NewGatewayRegistry(), Configs: stubChannelConfigs{cfg: activeChannelConfig()}, Runner: newStubChargeRunner(repo)}
	_, err := orch.Execute(context.Background(), "p-missing", "org-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ─── RefundsRepo 最小 stub ─────────────────────────────────────

type refundsTracker struct {
	createFn func(ctx context.Context, r *domain.Refund) error
	called   int
	last     *domain.Refund
}

func (s *refundsTracker) GetByID(ctx context.Context, id string) (*domain.Refund, error) {
	return nil, nil
}
func (s *refundsTracker) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Refund, error) {
	return nil, nil
}
func (s *refundsTracker) GetByOutRefundNo(ctx context.Context, ch, no string) (*domain.Refund, error) {
	return nil, nil
}
func (s *refundsTracker) ListByPayment(ctx context.Context, pid string) ([]*domain.Refund, error) {
	return nil, nil
}
func (s *refundsTracker) ListByPaymentForOrganization(ctx context.Context, pid, organizationID string) ([]*domain.Refund, error) {
	return nil, nil
}
func (s *refundsTracker) List(ctx context.Context, org string, limit, offset int) ([]*domain.Refund, int64, error) {
	return nil, 0, nil
}
func (s *refundsTracker) Create(ctx context.Context, r *domain.Refund) error {
	s.called++
	s.last = r
	if s.createFn != nil {
		return s.createFn(ctx, r)
	}
	return nil
}
func (s *refundsTracker) Update(ctx context.Context, r *domain.Refund) error { return nil }

// ─── RefundOrchestrator.Execute ───────────────────────────────────

func TestRefundOrchestrator_Execute_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := pmock.NewMockRepository(ctrl)
	refunds := &refundsTracker{}
	reg := NewGatewayRegistry()
	reg.Register("alipay", &stubChannelGateway{name: "alipay"})
	repo.EXPECT().GetByID(gomock.Any(), "p-1").Return(&domain.Payment{ID: "p-1", OrganizationID: "org-1"}, nil)

	orch := &RefundOrchestrator{
		Clock:    fixedClock{},
		UUID:     fixedUUID{s: "refund-uuid-1"},
		Payments: repo,
		Refunds:  refunds,
		Registry: reg,
		LockPayment: func(ctx context.Context, pid string) (*domain.LockedPayment, error) {
			// 已结算，未退款过，价格 100
			return &domain.LockedPayment{
				Price: 100, Status: domain.StateSucceeded,
				Provider: "alipay", OutTradeNo: "out-1", ChannelOrderNo: "ch-1", Currency: "CNY",
			}, nil
		},
	}

	ref, err := orch.Execute(context.Background(), "p-1", "org-1", 20.00, "customer-requested")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ref == nil {
		t.Fatal("expected refund")
	}
	if ref.Amount != 20.00 {
		t.Errorf("expected 20.00, got %v", ref.Amount)
	}
	if ref.State != domain.RefundStateSucceeded {
		t.Errorf("expected state succeeded, got %s", ref.State)
	}
	if ref.ChannelRefundNo == "" {
		t.Error("expected channel refund no from the adapter")
	}
	if refunds.called != 1 {
		t.Errorf("expected refund created once, got %d", refunds.called)
	}
}

func TestRefundOrchestrator_Execute_NonPositiveAmount(t *testing.T) {
	orch := &RefundOrchestrator{LockPayment: func(ctx context.Context, pid string) (*domain.LockedPayment, error) {
		return &domain.LockedPayment{Price: 100, Status: domain.StateSucceeded, Provider: "alipay"}, nil
	}}

	_, err := orch.Execute(context.Background(), "p-1", "org-1", 0, "")
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
	_, err = orch.Execute(context.Background(), "p-1", "org-1", -10, "")
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestRefundOrchestrator_Execute_StateGateRejected(t *testing.T) {
	orch := &RefundOrchestrator{
		Payments: refundOrgPayments("org-1"),
		Refunds:  &refundsTracker{},
		LockPayment: func(ctx context.Context, pid string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{Price: 100, Status: domain.StateCreated}, nil // 未结算
		},
	}
	_, err := orch.Execute(context.Background(), "p-1", "org-1", 20, "")
	if err == nil {
		t.Fatal("expected error when payment not settled")
	}
}

func TestRefundOrchestrator_Execute_ExceedsAvailable(t *testing.T) {
	orch := &RefundOrchestrator{
		Payments: refundOrgPayments("org-1"),
		Refunds:  &refundsTracker{},
		LockPayment: func(ctx context.Context, pid string) (*domain.LockedPayment, error) {
			// 价格 100，已退款 90，剩余 10
			return &domain.LockedPayment{RefundedAmount: 90, Price: 100, Status: domain.StatePartiallyRefunded}, nil
		},
	}
	_, err := orch.Execute(context.Background(), "p-1", "org-1", 50, "") // 要 50，只有 10
	if err == nil {
		t.Fatal("expected error when refund exceeds available")
	}
}

func TestRefundOrchestrator_Execute_ExactlyAtCeiling_IsOk(t *testing.T) {
	reg := NewGatewayRegistry()
	reg.Register("alipay", &stubChannelGateway{name: "alipay"})
	orch := &RefundOrchestrator{
		Clock:    fixedClock{},
		UUID:     fixedUUID{s: "u-1"},
		Payments: refundOrgPayments("org-1"),
		Refunds:  &refundsTracker{},
		Registry: reg,
		LockPayment: func(ctx context.Context, pid string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{RefundedAmount: 80, Price: 100, Status: domain.StateSucceeded, Provider: "alipay"}, nil // 可用额度 20
		},
	}
	_, err := orch.Execute(context.Background(), "p-1", "org-1", 20, "")
	if err != nil {
		t.Fatalf("exactly at ceiling should be ok: %v", err)
	}
}

func TestRefundOrchestrator_Execute_PartiallyRefundedSourceState(t *testing.T) {
	// partially_refunded 也应该可以发起新的 refund
	reg := NewGatewayRegistry()
	reg.Register("alipay", &stubChannelGateway{name: "alipay"})
	orch := &RefundOrchestrator{
		Clock:    fixedClock{},
		UUID:     fixedUUID{s: "u-2"},
		Payments: refundOrgPayments("org-1"),
		Refunds:  &refundsTracker{},
		Registry: reg,
		LockPayment: func(ctx context.Context, pid string) (*domain.LockedPayment, error) {
			return &domain.LockedPayment{RefundedAmount: 30, Price: 100, Status: domain.StatePartiallyRefunded, Provider: "alipay"}, nil // 可用额度 70
		},
	}
	ref, err := orch.Execute(context.Background(), "p-1", "org-1", 25, "extra")
	if err != nil {
		t.Fatalf("partially_refunded source state should allow refund: %v", err)
	}
	if ref.Amount != 25 {
		t.Errorf("expected 25, got %v", ref.Amount)
	}
}
