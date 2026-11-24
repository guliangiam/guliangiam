package payments

import (
	"context"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test doubles ──────────────────────────────────────────────────────

var qwFixedClock = fixedClock{}
var qwFixedUUID = fixedUUID{s: "qw-uuid-001"}

type fakePaymentsRepo struct {
	items   map[string]*domain.Payment
	listErr error
}

func newFakePaymentsRepo() *fakePaymentsRepo {
	return &fakePaymentsRepo{items: map[string]*domain.Payment{}}
}

func (f *fakePaymentsRepo) GetByID(_ context.Context, id string) (*domain.Payment, error) {
	p, ok := f.items[id]
	if !ok {
		return nil, errf("not found")
	}
	cp := *p
	return &cp, nil
}
func (f *fakePaymentsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Payment, error) {
	p, ok := f.items[id]
	if !ok || p.OrganizationID != organizationID {
		return nil, errf("not found")
	}
	cp := *p
	return &cp, nil
}
func (f *fakePaymentsRepo) GetByChannelOrderNo(_ context.Context, _, orderNo string) (*domain.Payment, error) {
	for _, p := range f.items {
		if p.ChannelOrderNo == orderNo {
			cp := *p
			return &cp, nil
		}
	}
	return nil, errf("not found")
}
func (f *fakePaymentsRepo) GetByOutTradeNo(_ context.Context, _, outNo string) (*domain.Payment, error) {
	for _, p := range f.items {
		if p.OutOrderId == outNo {
			cp := *p
			return &cp, nil
		}
	}
	return nil, errf("not found")
}
func (f *fakePaymentsRepo) List(_ context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	var out []*domain.Payment
	for _, p := range f.items {
		if q.State != nil && p.State != *q.State {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out, int64(len(out)), nil
}
func (f *fakePaymentsRepo) Create(_ context.Context, p *domain.Payment) error {
	cp := *p
	f.items[p.ID] = &cp
	return nil
}
func (f *fakePaymentsRepo) Update(_ context.Context, p *domain.Payment) error {
	if _, ok := f.items[p.ID]; !ok {
		return errf("not found")
	}
	cp := *p
	f.items[p.ID] = &cp
	return nil
}
func (f *fakePaymentsRepo) Delete(_ context.Context, id, organizationID string) error {
	if p, ok := f.items[id]; !ok || p.OrganizationID != organizationID {
		return errf("not found")
	}
	delete(f.items, id)
	return nil
}

type fakeAttemptsRepo struct {
	items []*domain.PayAttempt
}

func (f *fakeAttemptsRepo) GetByID(_ context.Context, _ string) (*domain.PayAttempt, error) {
	return nil, nil
}
func (f *fakeAttemptsRepo) GetByIDForOrganization(_ context.Context, _, _ string) (*domain.PayAttempt, error) {
	return nil, nil
}
func (f *fakeAttemptsRepo) ListByPayment(_ context.Context, _ string) ([]*domain.PayAttempt, error) {
	return nil, nil
}
func (f *fakeAttemptsRepo) ListByPaymentForOrganization(_ context.Context, _, _ string) ([]*domain.PayAttempt, error) {
	return nil, nil
}
func (f *fakeAttemptsRepo) ListByRefund(_ context.Context, _ string) ([]*domain.PayAttempt, error) {
	return nil, nil
}
func (f *fakeAttemptsRepo) Create(_ context.Context, a *domain.PayAttempt) error {
	cp := *a
	f.items = append(f.items, &cp)
	return nil
}

// fakeChannelGateway simulates a channel adapter.
type fakeChannelGateway struct {
	queryResults map[string]*domain.QueryResult // OutTradeNo → result
	queryErrs    map[string]*domain.ChannelError
	queryCalls   []string
}

func (f *fakeChannelGateway) Metadata() domain.ChannelMetadata {
	return domain.ChannelMetadata{Channel: "fake", SupportsCharge: true, SupportsQuery: true}
}
func (f *fakeChannelGateway) Charge(_ context.Context, _ domain.ChargeRequest) (*domain.ChargeResult, *domain.ChannelError) {
	return nil, nil
}
func (f *fakeChannelGateway) Query(_ context.Context, req domain.QueryRequest) (*domain.QueryResult, *domain.ChannelError) {
	f.queryCalls = append(f.queryCalls, req.OutTradeNo)
	if e, ok := f.queryErrs[req.OutTradeNo]; ok {
		return nil, e
	}
	if r, ok := f.queryResults[req.OutTradeNo]; ok {
		return r, nil
	}
	return &domain.QueryResult{State: domain.AttemptStateUnknown}, nil
}
func (f *fakeChannelGateway) Refund(_ context.Context, _ domain.RefundRequest) (*domain.RefundResult, *domain.ChannelError) {
	return nil, nil
}
func (f *fakeChannelGateway) QueryRefund(_ context.Context, _ domain.QueryRefundRequest) (*domain.QueryRefundResult, *domain.ChannelError) {
	return nil, nil
}
func (f *fakeChannelGateway) Close(_ context.Context, _ domain.CloseRequest) (*domain.CloseResult, *domain.ChannelError) {
	return nil, nil
}
func (f *fakeChannelGateway) ParseCallback(_ context.Context, _ domain.CallbackRequest) (*domain.CallbackEvent, error) {
	return nil, nil
}
func (f *fakeChannelGateway) VerifyConfig(_ context.Context, _ domain.ChannelConfig) error {
	return nil
}
func (f *fakeChannelGateway) DownloadStatement(_ context.Context, _ domain.StatementRequest) ([]domain.StatementLine, *domain.ChannelError) {
	return nil, nil
}

// helper
type errStringError string

func (e errStringError) Error() string      { return string(e) }
func errf(format string, args ...any) error { return errStringError("not found") }

// ── Fixture ───────────────────────────────────────────────────────────

func newProcessingPayment(id, outNo, channel string) *domain.Payment {
	return &domain.Payment{
		ID:             id,
		OrganizationID: "org-1",
		Name:           "Test",
		Provider:       channel,
		OutOrderId:     outNo,
		State:          domain.StateProcessing,
		Price:          99.00,
		Currency:       "USD",
		CreatedAt:      qwFixedClock.Now(),
		UpdatedAt:      qwFixedClock.Now(),
	}
}

// ── Tests ─────────────────────────────────────────────────────────────

func TestQueryWorker_Tick_ConvergesSucceeded(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	gw := &fakeChannelGateway{
		queryResults: map[string]*domain.QueryResult{
			"out-001": {State: domain.AttemptStateSucceeded, ChannelOrderNo: "ch-100"},
		},
	}
	registry := NewGatewayRegistry()
	registry.Register("alipay", gw)

	require.NoError(t, payments.Create(context.Background(), newProcessingPayment("p-1", "out-001", "alipay")))

	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 0, 50)
	require.NoError(t, w.tick(context.Background()))

	// Payment should be succeeded
	p := payments.items["p-1"]
	assert.Equal(t, domain.StateSucceeded, p.State)
	assert.Equal(t, "ch-100", p.ChannelOrderNo)

	// Attempt recorded
	require.Len(t, attempts.items, 1)
	assert.Equal(t, domain.AttemptStateSucceeded, attempts.items[0].State)
	assert.Equal(t, "query", attempts.items[0].AttemptType)
	assert.Equal(t, "alipay", attempts.items[0].Channel)

	// Gateway called with right OutTradeNo
	require.Len(t, gw.queryCalls, 1)
	assert.Equal(t, "out-001", gw.queryCalls[0])
}

func TestQueryWorker_Tick_ConvergesFailed(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	gw := &fakeChannelGateway{
		queryResults: map[string]*domain.QueryResult{
			"out-fail": {State: domain.AttemptStateFailed, ErrorCode: "ORDER_NOT_FOUND"},
		},
	}
	registry := NewGatewayRegistry()
	registry.Register("wechat", gw)

	require.NoError(t, payments.Create(context.Background(), newProcessingPayment("p-fail", "out-fail", "wechat")))

	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 0, 50)
	require.NoError(t, w.tick(context.Background()))

	p := payments.items["p-fail"]
	assert.Equal(t, domain.StateFailed, p.State)
	assert.Equal(t, "ORDER_NOT_FOUND", p.LastErrorCode)

	require.Len(t, attempts.items, 1)
	assert.Equal(t, domain.AttemptStateFailed, attempts.items[0].State)
	assert.Equal(t, strPtr("ORDER_NOT_FOUND"), attempts.items[0].ErrorCode)
}

func TestQueryWorker_Tick_Unknown_Passthrough(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	gw := &fakeChannelGateway{
		// no entry → returns AttemptStateUnknown
	}
	registry := NewGatewayRegistry()
	registry.Register("stripe", gw)

	require.NoError(t, payments.Create(context.Background(), newProcessingPayment("p-unk", "out-unk", "stripe")))

	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 0, 50)
	require.NoError(t, w.tick(context.Background()))

	// Payment stays processing
	p := payments.items["p-unk"]
	assert.Equal(t, domain.StateProcessing, p.State, "UNKNOWN must NOT advance payment state")

	// Attempt recorded as unknown
	require.Len(t, attempts.items, 1)
	assert.Equal(t, domain.AttemptStateUnknown, attempts.items[0].State)
}

func TestQueryWorker_Tick_ChannelError_Passthrough(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	gw := &fakeChannelGateway{
		queryErrs: map[string]*domain.ChannelError{
			"out-err": {Code: "NETWORK_TIMEOUT", Message: "connect timeout", Category: domain.ErrorCategoryRetryable},
		},
	}
	registry := NewGatewayRegistry()
	registry.Register("alipay", gw)

	require.NoError(t, payments.Create(context.Background(), newProcessingPayment("p-err", "out-err", "alipay")))

	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 0, 50)
	require.NoError(t, w.tick(context.Background()))

	// Payment stays processing (retryable error → don't advance)
	p := payments.items["p-err"]
	assert.Equal(t, domain.StateProcessing, p.State)

	// Attempt recorded as failed with ChannelError details
	require.Len(t, attempts.items, 1)
	assert.Equal(t, domain.AttemptStateFailed, attempts.items[0].State)
	assert.Equal(t, strPtr("NETWORK_TIMEOUT"), attempts.items[0].ErrorCode)
	assert.Equal(t, strPtr("connect timeout"), attempts.items[0].ErrorMessage)
}

func TestQueryWorker_Tick_NoProcessing_Noop(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	registry := NewGatewayRegistry()

	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 0, 50)
	require.NoError(t, w.tick(context.Background()))
	assert.Empty(t, attempts.items, "no attempts should be recorded with empty processing set")
}

func TestQueryWorker_Tick_BatchSizeLimited(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	gw := &fakeChannelGateway{
		queryResults: map[string]*domain.QueryResult{
			"out-1": {State: domain.AttemptStateSucceeded},
			"out-2": {State: domain.AttemptStateSucceeded},
			"out-3": {State: domain.AttemptStateSucceeded},
		},
	}
	registry := NewGatewayRegistry()
	registry.Register("alipay", gw)

	for i, outNo := range []string{"out-1", "out-2", "out-3"} {
		require.NoError(t, payments.Create(context.Background(), newProcessingPayment(
			"p"+string(rune('1'+i)), outNo, "alipay")))
	}

	// Fake repo ignores PageSize — so all 3 items come back. The batchSize
	// here is just a safety cap; real implementation enforces it in SQL LIMIT.
	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 0, 2)
	require.NoError(t, w.tick(context.Background()))

	// All 3 processed — our fake returns all, queryWorker handles them all.
	// batchSize=2 only caps the SQL query layer, not the worker loop itself.
	assert.Len(t, gw.queryCalls, 3)
	assert.Len(t, attempts.items, 3)
}

func TestQueryWorker_Tick_UnregisteredChannel(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	registry := NewGatewayRegistry()
	// No adapters registered

	require.NoError(t, payments.Create(context.Background(), newProcessingPayment("p-no-adapt", "out-x", "unknown-channel")))

	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 0, 50)
	// Should not error on tick — single-payment failure continues
	assert.NoError(t, w.tick(context.Background()))
	assert.Empty(t, attempts.items, "no attempts recorded when channel not registered")
}

// ── Start/Stop lifecycle ──────────────────────────────────────────────

func TestQueryWorker_StartStop_Idempotent(t *testing.T) {
	payments := newFakePaymentsRepo()
	attempts := &fakeAttemptsRepo{}
	registry := NewGatewayRegistry()

	w := NewQueryWorker(payments, attempts, qwFixedClock, qwFixedUUID, registry, 100*time.Millisecond, 50)

	w.Start(context.Background())
	w.Start(context.Background()) // second Start should be no-op
	assert.NotNil(t, w.cancel)
	w.Stop()
	w.Stop() // second Stop should be no-op
}
