package payments

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// ─── 内联 stub ─────────────────────────────────────────────────────────

type stubReceiptsRepo struct {
	existing    *domain.CallbackReceipt
	lookupErr   error
	createErr   error
	updateErr   error
	createdRec  *domain.CallbackReceipt
	updatedRecs []*domain.CallbackReceipt
}

func (s *stubReceiptsRepo) Create(ctx context.Context, r *domain.CallbackReceipt) error {
	cp := *r
	s.createdRec = &cp
	return s.createErr
}
func (s *stubReceiptsRepo) GetByPayloadHash(ctx context.Context, channel, payloadHash string) (*domain.CallbackReceipt, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	if s.existing == nil {
		return nil, nil
	}
	cp := *s.existing
	return &cp, nil
}
func (s *stubReceiptsRepo) GetByEventKey(ctx context.Context, channel, eventKey string) (*domain.CallbackReceipt, error) {
	return nil, nil
}
func (s *stubReceiptsRepo) Update(ctx context.Context, r *domain.CallbackReceipt) error {
	cp := *r
	s.updatedRecs = append(s.updatedRecs, &cp)
	return s.updateErr
}
func (s *stubReceiptsRepo) List(ctx context.Context, orgID string, limit, offset int) ([]*domain.CallbackReceipt, int64, error) {
	return nil, 0, nil
}

type stubPaymentsRepo struct {
	payment     *domain.Payment
	getErr      error
	updateErrs  []error
	updateCalls int
	updated     []*domain.Payment
}

func (s *stubPaymentsRepo) GetByID(ctx context.Context, id string) (*domain.Payment, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.clonePayment(), nil
}
func (s *stubPaymentsRepo) GetByIDForOrganization(ctx context.Context, id, organizationID string) (*domain.Payment, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	p := s.clonePayment()
	if p == nil || p.OrganizationID != organizationID {
		return nil, errors.New("not found")
	}
	return p, nil
}
func (s *stubPaymentsRepo) GetByChannelOrderNo(ctx context.Context, channel, orderNo string) (*domain.Payment, error) {
	return s.GetByID(ctx, orderNo)
}
func (s *stubPaymentsRepo) GetByOutTradeNo(ctx context.Context, channel, outTradeNo string) (*domain.Payment, error) {
	return s.GetByID(ctx, outTradeNo)
}
func (s *stubPaymentsRepo) List(ctx context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
	return nil, 0, nil
}
func (s *stubPaymentsRepo) Create(ctx context.Context, p *domain.Payment) error { return nil }
func (s *stubPaymentsRepo) Update(ctx context.Context, p *domain.Payment) error {
	cp := *p
	s.updated = append(s.updated, &cp)
	if s.updateCalls < len(s.updateErrs) {
		err := s.updateErrs[s.updateCalls]
		s.updateCalls++
		return err
	}
	return nil
}
func (s *stubPaymentsRepo) Delete(ctx context.Context, id, organizationID string) error { return nil }
func (s *stubPaymentsRepo) clonePayment() *domain.Payment {
	if s.payment == nil {
		return nil
	}
	p := *s.payment
	return &p
}

type stubGateway struct {
	parseFunc  func(ctx context.Context, req domain.CallbackRequest) (*domain.CallbackEvent, error)
	refundFunc func(ctx context.Context, req domain.RefundRequest) (*domain.RefundResult, *domain.ChannelError)
	domain.ChannelGateway
}

func (s *stubGateway) Refund(ctx context.Context, req domain.RefundRequest) (*domain.RefundResult, *domain.ChannelError) {
	if s.refundFunc != nil {
		return s.refundFunc(ctx, req)
	}
	return &domain.RefundResult{State: domain.AttemptStateSucceeded, ChannelRefundNo: "ch-refund-1"}, nil
}

func (s *stubGateway) ParseCallback(ctx context.Context, req domain.CallbackRequest) (*domain.CallbackEvent, error) {
	if s.parseFunc != nil {
		return s.parseFunc(ctx, req)
	}
	return &domain.CallbackEvent{
		OutTradeNo:     "order-123",
		ChannelOrderNo: "ch-456",
		State:          domain.AttemptStateSucceeded,
		RawStatus:      "paid",
	}, nil
}
func (s *stubGateway) Metadata() domain.ChannelMetadata {
	return domain.ChannelMetadata{Channel: "stub", SupportsCharge: true, SupportsRefund: true, SupportsQuery: true, SupportsCallback: true}
}

func strPtr(s string) *string { return &s }

// ─── 测试辅助 ─────────────────────────────────────────────────────────────

func newPipeline(receipts domain.CallbackReceiptsRepo, payments domain.Repository, registry *GatewayRegistry) *CallbackPipeline {
	return &CallbackPipeline{
		Clock:    clock.SystemClock{},
		UUID:     uuid.V7Generator{},
		Payments: payments,
		Receipts: receipts,
		Registry: registry,
	}
}

// ─── 五阶段管线测试 ────────────────────────────────────────────────────────

func TestCallbackPipeline_NormalPath(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-1",
			OutOrderId:     "order-xyz",
			State:          domain.StateProcessing,
			Provider:       "stripe",
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	p := newPipeline(receipts, payments, reg)

	payload := []byte(`{"out_trade_no":"order-xyz"}`)
	receiptID, advanced, err := p.Process(context.Background(), "stripe", payload, nil, "127.0.0.1", "org-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receiptID == "" {
		t.Error("receiptID should not be empty")
	}
	if !advanced {
		t.Error("payment should advance from processing → succeeded")
	}
	// Create 被调用（initial state = received）
	if receipts.createdRec == nil {
		t.Fatal("receipt Create not called")
	}
	if receipts.createdRec.ProcessState != domain.ReceiptStateReceived {
		t.Errorf("receipt initial state = %s, want received", receipts.createdRec.ProcessState)
	}
	if receipts.createdRec.CreatedAt.IsZero() || receipts.createdRec.UpdatedAt.IsZero() {
		t.Error("receipt audit timestamps must be initialized")
	}
	if receipts.createdRec.Revision != 1 {
		t.Errorf("receipt initial revision = %d, want 1", receipts.createdRec.Revision)
	}
	// 最终阶段 5 Update 进 accepted
	lastUpdate := receipts.updatedRecs[len(receipts.updatedRecs)-1]
	if lastUpdate.ProcessState != domain.ReceiptStateAccepted {
		t.Errorf("receipt final state = %s, want accepted", lastUpdate.ProcessState)
	}
}

func TestCallbackPipeline_DuplicatePayload(t *testing.T) {
	payload := []byte(`{"out_trade_no":"dup"}`)
	h := sha256.Sum256(payload)
	sha := hex.EncodeToString(h[:])
	receipts := &stubReceiptsRepo{
		existing: &domain.CallbackReceipt{
			ID:            "existing-1",
			PayloadSHA256: sha,
			ProcessState:  domain.ReceiptStateAccepted,
		},
	}
	payments := &stubPaymentsRepo{}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	p := newPipeline(receipts, payments, reg)
	receiptID, advanced, err := p.Process(context.Background(), "stripe", payload, nil, "", "org-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if receiptID != "existing-1" {
		t.Errorf("receiptID = %s, want existing-1 (short-circuit)", receiptID)
	}
	if advanced {
		t.Error("duplicate should not advance state machine")
	}
	// Create 不应被调用 — 幂等短路
	if receipts.createdRec != nil {
		t.Error("duplicate payload should NOT create new receipt")
	}
}

func TestCallbackPipeline_NoAdapter(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{}
	reg := NewGatewayRegistry() // 空

	p := newPipeline(receipts, payments, reg)
	_, _, err := p.Process(context.Background(), "unknown-channel", []byte(`{}`), nil, "", "org-1")
	if err == nil {
		t.Fatal("expected ErrChannelUnavailable")
	}
	if err != domain.ErrChannelUnavailable {
		t.Errorf("err = %v, want ErrChannelUnavailable", err)
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodePaymentChannelUnavailable {
		t.Errorf("code = %s, want %s", code, apperrors.CodePaymentChannelUnavailable)
	}
	// 阶段 1 收件应该已完成（Create 被调用）
	if receipts.createdRec == nil {
		t.Fatal("receipt should be created (stage1) even when adapter missing")
	}
	if receipts.updatedRecs == nil {
		t.Fatal("receipt should have been transitioned to rejected")
	}
	last := receipts.updatedRecs[len(receipts.updatedRecs)-1]
	if last.ProcessState != domain.ReceiptStateRejected {
		t.Errorf("final state = %s, want rejected", last.ProcessState)
	}
}

func TestCallbackPipeline_ParseCallbackFails(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{
		parseFunc: func(ctx context.Context, req domain.CallbackRequest) (*domain.CallbackEvent, error) {
			return nil, errors.New("signature mismatch")
		},
	})

	p := newPipeline(receipts, payments, reg)
	_, _, err := p.Process(context.Background(), "stripe", []byte(`{}`), nil, "", "org-1")
	if err == nil {
		t.Fatal("expected error from ParseCallback")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodePaymentVerificationFailed {
		t.Fatalf("code = %s, want %s", code, apperrors.CodePaymentVerificationFailed)
	}
	last := receipts.updatedRecs[len(receipts.updatedRecs)-1]
	if last.ProcessState != domain.ReceiptStateRejected {
		t.Errorf("final state = %s, want rejected", last.ProcessState)
	}
	if last.ErrorMessage == nil || *last.ErrorMessage == "" {
		t.Error("receipt ErrorMessage should be populated")
	}
}

func TestCallbackPipeline_PaymentNotFound(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{} // nil payment → clonePayment 返回 nil → Process 会调 GetByOutTradeNo

	// 直接让 GetByOutTradeNo 返回错误
	payments.getErr = errors.New("payment not found")

	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	p := newPipeline(receipts, payments, reg)
	_, _, err := p.Process(context.Background(), "stripe", []byte(`{}`), nil, "", "org-1")
	if err == nil {
		t.Fatal("expected payment not found")
	}
}

func TestCallbackPipeline_StateConflict(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-1",
			OutOrderId:     "order-xyz",
			State:          domain.StateRefunded, // terminal
			Provider:       "stripe",
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	p := newPipeline(receipts, payments, reg)
	_, _, err := p.Process(context.Background(), "stripe", []byte(`{}`), nil, "", "org-1")
	if err == nil {
		t.Fatal("expected state machine conflict error")
	}
}

func TestCallbackPipeline_AlreadyAtTargetIdempotent(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-1",
			OutOrderId:     "order-xyz",
			State:          domain.StateSucceeded, // already target
			Provider:       "stripe",
		},
	}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	p := newPipeline(receipts, payments, reg)
	_, advanced, err := p.Process(context.Background(), "stripe", []byte(`{}`), nil, "", "org-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if advanced {
		t.Error("already at target state should report advanced=false")
	}
}

// ─── advancePaymentFromEvent 纯函数 ──────────────────────────────────────

func TestAdvancePaymentFromEvent(t *testing.T) {
	cases := []struct {
		name      string
		state     string
		event     domain.AttemptState
		wantAdv   bool
		wantState string
		wantErr   bool
	}{
		{"processing→succeeded", domain.StateProcessing, domain.AttemptStateSucceeded, true, domain.StateSucceeded, false},
		{"processing→failed", domain.StateProcessing, domain.AttemptStateFailed, true, domain.StateFailed, false},
		{"succeeded→succeeded no-op", domain.StateSucceeded, domain.AttemptStateSucceeded, false, domain.StateSucceeded, false},
		{"succeeded→failed conflict", domain.StateSucceeded, domain.AttemptStateFailed, false, "", true},
		{"refunded→succeeded blocked", domain.StateRefunded, domain.AttemptStateSucceeded, false, "", true},
		{"pending no-op", domain.StateProcessing, domain.AttemptStatePending, false, "", false},
		{"unknown no-op", domain.StateProcessing, domain.AttemptStateUnknown, false, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &domain.Payment{State: c.state}
			evt := &domain.CallbackEvent{State: c.event}
			gotAdv, err := advancePaymentFromEvent(p, evt)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr=%v", err, c.wantErr)
			}
			if gotAdv != c.wantAdv {
				t.Errorf("advanced = %v, want %v", gotAdv, c.wantAdv)
			}
			if c.wantState != "" && p.State != c.wantState {
				t.Errorf("state = %s, want %s", p.State, c.wantState)
			}
		})
	}
}

func TestAttemptStateToPaymentState(t *testing.T) {
	cases := []struct {
		state domain.AttemptState
		want  string
	}{
		{domain.AttemptStateSucceeded, domain.StateSucceeded},
		{domain.AttemptStateFailed, domain.StateFailed},
		{domain.AttemptStatePending, domain.StateProcessing},
		{domain.AttemptStateRejected, ""},
		{domain.AttemptStateUnknown, ""},
	}
	for _, c := range cases {
		if got := attemptStateToPaymentState(c.state); got != c.want {
			t.Errorf("attemptStateToPaymentState(%q) = %q, want %q", c.state, got, c.want)
		}
	}
}

// ─── GatewayRegistry ──────────────────────────────────────────────────────

func TestGatewayRegistry_RegisterGetNames(t *testing.T) {
	reg := NewGatewayRegistry()
	gw := &stubGateway{}
	reg.Register("stripe", gw)
	reg.Register("alipay", gw)

	if g, ok := reg.Get("stripe"); !ok || g == nil {
		t.Error("Get(stripe) should return registered adapter")
	}
	if _, ok := reg.Get("unknown"); ok {
		t.Error("Get(unknown) should return false")
	}
	names := reg.Names()
	if len(names) != 2 {
		t.Errorf("Names() = %v, want 2 entries", names)
	}
}

// 确保 fmt 在错误场景被触发时有 import
var _ = fmt.Sprintf

func TestCallbackPipeline_ReceiptStorageFailed(t *testing.T) {
	receipts := &stubReceiptsRepo{createErr: errors.New("db down")}
	payments := &stubPaymentsRepo{}
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{})

	p := newPipeline(receipts, payments, reg)
	_, _, err := p.Process(context.Background(), "stripe", []byte("{}"), nil, "", "org-1")
	if err == nil {
		t.Fatal("expected receipt storage error")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodePaymentReceiptStorageFailed {
		t.Fatalf("code = %s, want %s", code, apperrors.CodePaymentReceiptStorageFailed)
	}
	if receipts.createdRec == nil {
		t.Fatal("Create should have been attempted")
	}
}

func TestCallbackPipeline_AmountMismatch(t *testing.T) {
	receipts := &stubReceiptsRepo{}
	payments := &stubPaymentsRepo{
		payment: &domain.Payment{
			ID:             "pay-1",
			OrganizationID: "org-1",
			OutOrderId:     "order-xyz",
			State:          domain.StateProcessing,
			Provider:       "stripe",
			Price:          100,
		},
	}
	wrong := 99.0
	reg := NewGatewayRegistry()
	reg.Register("stripe", &stubGateway{
		parseFunc: func(ctx context.Context, req domain.CallbackRequest) (*domain.CallbackEvent, error) {
			return &domain.CallbackEvent{
				OutTradeNo:     "order-xyz",
				ChannelOrderNo: "ch-456",
				Amount:         &wrong,
				State:          domain.AttemptStateSucceeded,
				RawStatus:      "paid",
			}, nil
		},
	})

	p := newPipeline(receipts, payments, reg)
	_, advanced, err := p.Process(context.Background(), "stripe", []byte("{}"), nil, "", "org-1")
	if err == nil {
		t.Fatal("expected consistency mismatch")
	}
	if advanced {
		t.Fatal("mismatched amount must not advance the payment state")
	}
	if code := apperrors.FromError(err).Code; code != apperrors.CodePaymentConsistencyMismatch {
		t.Fatalf("code = %s, want %s", code, apperrors.CodePaymentConsistencyMismatch)
	}
	last := receipts.updatedRecs[len(receipts.updatedRecs)-1]
	if last.ProcessState != domain.ReceiptStateRejected {
		t.Fatalf("receipt final state = %s, want rejected", last.ProcessState)
	}
}
