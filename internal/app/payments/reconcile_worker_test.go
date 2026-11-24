package payments

import (
	"context"
	"strconv"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/payments"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Test doubles for reconcile worker ────────────────────────────────

type reconFixedClock struct{}

func (reconFixedClock) Now() time.Time        { return reconNow }
func (reconFixedClock) Sleep(_ time.Duration) {}

var reconNow = time.Date(2025, 10, 1, 12, 0, 0, 0, time.UTC)

type reconFixedUUID struct{ count int }

func (f *reconFixedUUID) NewString() (string, error) {
	f.count++
	return "recon-item-uuid-" + strconv.Itoa(f.count), nil
}

// ── fakeReconRepo ─────────────────────────────────────────────────────

type fakeReconRepo struct {
	batches      map[string]*domain.ReconBatch
	itemsByBatch map[string][]*domain.ReconItem
	updateErr    error
}

func newFakeReconRepo() *fakeReconRepo {
	return &fakeReconRepo{
		batches:      map[string]*domain.ReconBatch{},
		itemsByBatch: map[string][]*domain.ReconItem{},
	}
}

func (f *fakeReconRepo) Batches(_ context.Context, _ string, _, _ int) ([]*domain.ReconBatch, int64, error) {
	out := make([]*domain.ReconBatch, 0, len(f.batches))
	for _, b := range f.batches {
		cp := *b
		out = append(out, &cp)
	}
	return out, int64(len(out)), nil
}
func (f *fakeReconRepo) GetBatch(_ context.Context, id string) (*domain.ReconBatch, error) {
	b, ok := f.batches[id]
	if !ok {
		return nil, errStringError("not found")
	}
	cp := *b
	return &cp, nil
}
func (f *fakeReconRepo) GetBatchForOrganization(_ context.Context, id, organizationID string) (*domain.ReconBatch, error) {
	b, ok := f.batches[id]
	if !ok || b.OrganizationID != organizationID {
		return nil, errStringError("not found")
	}
	cp := *b
	return &cp, nil
}
func (f *fakeReconRepo) CreateBatch(_ context.Context, b *domain.ReconBatch) error {
	cp := *b
	f.batches[b.ID] = &cp
	return nil
}
func (f *fakeReconRepo) UpdateBatch(_ context.Context, b *domain.ReconBatch) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	if _, ok := f.batches[b.ID]; !ok {
		return errStringError("not found")
	}
	cp := *b
	f.batches[b.ID] = &cp
	return nil
}
func (f *fakeReconRepo) CreateItem(_ context.Context, item *domain.ReconItem) error {
	cp := *item
	f.itemsByBatch[item.BatchID] = append(f.itemsByBatch[item.BatchID], &cp)
	return nil
}
func (f *fakeReconRepo) ListItems(_ context.Context, batchID string) ([]*domain.ReconItem, error) {
	return f.itemsByBatch[batchID], nil
}
func (f *fakeReconRepo) ListItemsForOrganization(_ context.Context, batchID, organizationID string) ([]*domain.ReconItem, error) {
	b, ok := f.batches[batchID]
	if !ok || b.OrganizationID != organizationID {
		return nil, errStringError("not found")
	}
	return f.itemsByBatch[batchID], nil
}
func (f *fakeReconRepo) ResolveItem(_ context.Context, _, _ string) error { return nil }
func (f *fakeReconRepo) ResolveItemForOrganization(_ context.Context, _, _, _ string) error {
	return nil
}

// ── fakePaymentsRepo (recon-specific, supports date range) ────────────

type reconFakePaymentsRepo struct {
	items map[string]*domain.Payment
}

func newReconFakePaymentsRepo() *reconFakePaymentsRepo {
	return &reconFakePaymentsRepo{items: map[string]*domain.Payment{}}
}

func (f *reconFakePaymentsRepo) GetByID(_ context.Context, id string) (*domain.Payment, error) {
	p, ok := f.items[id]
	if !ok {
		return nil, errStringError("not found")
	}
	cp := *p
	return &cp, nil
}
func (f *reconFakePaymentsRepo) GetByIDForOrganization(_ context.Context, id, organizationID string) (*domain.Payment, error) {
	p, ok := f.items[id]
	if !ok || p.OrganizationID != organizationID {
		return nil, errStringError("not found")
	}
	cp := *p
	return &cp, nil
}
func (f *reconFakePaymentsRepo) GetByChannelOrderNo(_ context.Context, _, orderNo string) (*domain.Payment, error) {
	for _, p := range f.items {
		if p.ChannelOrderNo == orderNo {
			cp := *p
			return &cp, nil
		}
	}
	return nil, errStringError("not found")
}
func (f *reconFakePaymentsRepo) GetByOutTradeNo(_ context.Context, _, outNo string) (*domain.Payment, error) {
	for _, p := range f.items {
		if p.OutOrderId == outNo {
			cp := *p
			return &cp, nil
		}
	}
	return nil, errStringError("not found")
}
func (f *reconFakePaymentsRepo) List(_ context.Context, q domain.ListQuery) ([]*domain.Payment, int64, error) {
	var out []*domain.Payment
	for _, p := range f.items {
		if q.State != nil && p.State != *q.State {
			continue
		}
		if q.OrganizationID != "" && p.OrganizationID != q.OrganizationID {
			continue
		}
		cp := *p
		out = append(out, &cp)
	}
	return out, int64(len(out)), nil
}
func (f *reconFakePaymentsRepo) Create(_ context.Context, p *domain.Payment) error {
	cp := *p
	f.items[p.ID] = &cp
	return nil
}
func (f *reconFakePaymentsRepo) Update(_ context.Context, p *domain.Payment) error {
	cp := *p
	f.items[p.ID] = &cp
	return nil
}
func (f *reconFakePaymentsRepo) Delete(_ context.Context, _, _ string) error { return nil }

// ── fakeGateway with statement support ──────────────────────────────

type reconFakeGateway struct {
	statementLines map[string][]domain.StatementLine // key: BatchDate key
	statementErr   *domain.ChannelError
	meta           domain.ChannelMetadata
}

func newReconFakeGateway(channel string) *reconFakeGateway {
	return &reconFakeGateway{
		statementLines: map[string][]domain.StatementLine{},
		meta: domain.ChannelMetadata{
			Channel:           channel,
			SupportsStatement: true,
			SupportsCharge:    true,
		},
	}
}
func (g *reconFakeGateway) Metadata() domain.ChannelMetadata { return g.meta }
func (g *reconFakeGateway) Charge(_ context.Context, _ domain.ChargeRequest) (*domain.ChargeResult, *domain.ChannelError) {
	return nil, nil
}
func (g *reconFakeGateway) Query(_ context.Context, _ domain.QueryRequest) (*domain.QueryResult, *domain.ChannelError) {
	return nil, nil
}
func (g *reconFakeGateway) Refund(_ context.Context, _ domain.RefundRequest) (*domain.RefundResult, *domain.ChannelError) {
	return nil, nil
}
func (g *reconFakeGateway) QueryRefund(_ context.Context, _ domain.QueryRefundRequest) (*domain.QueryRefundResult, *domain.ChannelError) {
	return nil, nil
}
func (g *reconFakeGateway) Close(_ context.Context, _ domain.CloseRequest) (*domain.CloseResult, *domain.ChannelError) {
	return nil, nil
}
func (g *reconFakeGateway) ParseCallback(_ context.Context, _ domain.CallbackRequest) (*domain.CallbackEvent, error) {
	return nil, nil
}
func (g *reconFakeGateway) VerifyConfig(_ context.Context, _ domain.ChannelConfig) error { return nil }
func (g *reconFakeGateway) DownloadStatement(_ context.Context, req domain.StatementRequest) ([]domain.StatementLine, *domain.ChannelError) {
	if g.statementErr != nil {
		return nil, g.statementErr
	}
	key := req.BatchDate.Format("2006-01-02")
	return g.statementLines[key], nil
}

// ── Helpers ──────────────────────────────────────────────────────────

func newPayment(id, outNo string, price float64, state string) *domain.Payment {
	return &domain.Payment{
		ID:             id,
		OrganizationID: "org-1",
		Provider:       "alipay",
		Price:          price,
		State:          state,
		OutOrderId:     outNo,
	}
}

func newBatch(id, channel string, batchDate time.Time) *domain.ReconBatch {
	return &domain.ReconBatch{
		ID:             id,
		OrganizationID: "org-1",
		Channel:        channel,
		BatchDate:      batchDate,
		Status:         string(domain.ReconBatchStatusPending),
		CreatedAt:      reconNow,
		UpdatedAt:      reconNow,
	}
}

func newReconcileWorkerSetup(t *testing.T) (*ReconcileWorker, *reconFakePaymentsRepo, *fakeReconRepo, *reconFakeGateway) {
	t.Helper()
	payRepo := newReconFakePaymentsRepo()
	reconRepo := newFakeReconRepo()
	gw := newReconFakeGateway("alipay")
	reg := NewGatewayRegistry()
	reg.Register("alipay", gw)

	w := NewReconcileWorker(
		payRepo, reconRepo,
		reconFixedClock{}, &reconFixedUUID{},
		reg,
		0, 10,
	)
	return w, payRepo, reconRepo, gw
}

// ── Tests ────────────────────────────────────────────────────────────

func TestReconcile_ConvergesNoDiff(t *testing.T) {
	w, payRepo, reconRepo, gw := newReconcileWorkerSetup(t)
	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	// 渠道对账单 2 笔
	gw.statementLines["2025-10-01"] = []domain.StatementLine{
		{OutTradeNo: "out-1", Amount: 100.0, State: string(domain.AttemptStateSucceeded), OccurredAt: batchDate},
		{OutTradeNo: "out-2", Amount: 200.0, State: string(domain.AttemptStateSucceeded), OccurredAt: batchDate},
	}
	// 本地 2 笔完全匹配
	payRepo.items["p-1"] = newPayment("p-1", "out-1", 100.0, domain.StateSucceeded)
	payRepo.items["p-2"] = newPayment("p-2", "out-2", 200.0, domain.StateSucceeded)

	require.NoError(t, w.reconcileOne(context.Background(), batch))

	updated := reconRepo.batches["rb-1"]
	assert.Equal(t, string(domain.ReconBatchStatusCompleted), updated.Status)
	assert.Equal(t, 2, updated.TotalLocal)
	assert.Equal(t, 2, updated.TotalChannel)
	assert.Equal(t, 0, updated.DiffCount)

	items, _ := reconRepo.ListItems(context.Background(), "rb-1")
	assert.Equal(t, 0, len(items))
}

func TestReconcile_DetectsMissingLocal(t *testing.T) {
	w, payRepo, reconRepo, gw := newReconcileWorkerSetup(t)
	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	gw.statementLines["2025-10-01"] = []domain.StatementLine{
		{OutTradeNo: "out-1", Amount: 100.0, State: string(domain.AttemptStateSucceeded), OccurredAt: batchDate},
		{OutTradeNo: "out-2", Amount: 200.0, State: string(domain.AttemptStateSucceeded), OccurredAt: batchDate},
		{OutTradeNo: "out-3", Amount: 300.0, State: string(domain.AttemptStateSucceeded), OccurredAt: batchDate},
	}
	// 本地只有 2 笔 → out-3 missing_local
	payRepo.items["p-1"] = newPayment("p-1", "out-1", 100.0, domain.StateSucceeded)
	payRepo.items["p-2"] = newPayment("p-2", "out-2", 200.0, domain.StateSucceeded)

	require.NoError(t, w.reconcileOne(context.Background(), batch))

	updated := reconRepo.batches["rb-1"]
	assert.Equal(t, 1, updated.DiffCount)
	items, _ := reconRepo.ListItems(context.Background(), "rb-1")
	require.Len(t, items, 1)
	assert.Equal(t, string(domain.DiffMissingLocal), items[0].DiffType)
	assert.Equal(t, "out-3", *items[0].ChannelOrderNo)
	assert.Equal(t, 300.0, *items[0].ChannelAmount)
	assert.Nil(t, items[0].LocalPaymentID)
	assert.Nil(t, items[0].LocalAmount)
}

func TestReconcile_DetectsMissingChannel(t *testing.T) {
	w, payRepo, reconRepo, gw := newReconcileWorkerSetup(t)
	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	// 渠道只有 1 笔
	gw.statementLines["2025-10-01"] = []domain.StatementLine{
		{OutTradeNo: "out-1", Amount: 100.0, State: string(domain.AttemptStateSucceeded), OccurredAt: batchDate},
	}
	// 本地有 2 笔 → out-2 missing_channel
	payRepo.items["p-1"] = newPayment("p-1", "out-1", 100.0, domain.StateSucceeded)
	payRepo.items["p-2"] = newPayment("p-2", "out-2", 200.0, domain.StateSucceeded)

	require.NoError(t, w.reconcileOne(context.Background(), batch))

	updated := reconRepo.batches["rb-1"]
	assert.Equal(t, 1, updated.DiffCount)
	items, _ := reconRepo.ListItems(context.Background(), "rb-1")
	require.Len(t, items, 1)
	assert.Equal(t, string(domain.DiffMissingChannel), items[0].DiffType)
	assert.Equal(t, "p-2", *items[0].LocalPaymentID)
	assert.Nil(t, items[0].ChannelOrderNo)
}

func TestReconcile_DetectsAmountMismatch(t *testing.T) {
	w, payRepo, reconRepo, gw := newReconcileWorkerSetup(t)
	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	gw.statementLines["2025-10-01"] = []domain.StatementLine{
		{OutTradeNo: "out-1", Amount: 100.0, State: string(domain.AttemptStateSucceeded)},
	}
	payRepo.items["p-1"] = newPayment("p-1", "out-1", 150.0, domain.StateSucceeded)

	require.NoError(t, w.reconcileOne(context.Background(), batch))

	items, _ := reconRepo.ListItems(context.Background(), "rb-1")
	require.Len(t, items, 1)
	assert.Equal(t, string(domain.DiffAmountMismatch), items[0].DiffType)
	assert.Equal(t, 100.0, *items[0].ChannelAmount)
	assert.Equal(t, 150.0, *items[0].LocalAmount)
}

func TestReconcile_DetectsStateMismatch(t *testing.T) {
	w, payRepo, reconRepo, gw := newReconcileWorkerSetup(t)
	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	gw.statementLines["2025-10-01"] = []domain.StatementLine{
		// 渠道 succeeded, 本地 processing
		{OutTradeNo: "out-1", Amount: 100.0, State: string(domain.AttemptStateSucceeded)},
	}
	payRepo.items["p-1"] = newPayment("p-1", "out-1", 100.0, domain.StateProcessing)

	require.NoError(t, w.reconcileOne(context.Background(), batch))

	items, _ := reconRepo.ListItems(context.Background(), "rb-1")
	require.Len(t, items, 1)
	assert.Equal(t, string(domain.DiffStateMismatch), items[0].DiffType)
	assert.Equal(t, string(domain.AttemptStateSucceeded), *items[0].ChannelState)
	assert.Equal(t, domain.StateProcessing, *items[0].LocalState)
}

func TestReconcile_ChannelNotRegistered_MarksFailed(t *testing.T) {
	payRepo := newReconFakePaymentsRepo()
	reconRepo := newFakeReconRepo()
	reg := NewGatewayRegistry() // empty — no channel registered

	w := NewReconcileWorker(
		payRepo, reconRepo,
		reconFixedClock{}, &reconFixedUUID{},
		reg,
		0, 10,
	)

	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	err := w.reconcileOne(context.Background(), batch)
	assert.Error(t, err)
	updated := reconRepo.batches["rb-1"]
	assert.Equal(t, string(domain.ReconBatchStatusFailed), updated.Status)
}

func TestReconcile_DownloadError_MarksFailed(t *testing.T) {
	w, _, reconRepo, gw := newReconcileWorkerSetup(t)
	gw.statementErr = &domain.ChannelError{Code: "channel_busy", Message: "rate limited"}

	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	err := w.reconcileOne(context.Background(), batch)
	assert.Error(t, err)
	updated := reconRepo.batches["rb-1"]
	assert.Equal(t, string(domain.ReconBatchStatusFailed), updated.Status)
}

func TestReconcile_StartStop_Idempotent(t *testing.T) {
	w, _, _, _ := newReconcileWorkerSetup(t)

	w.Start(context.Background())
	w.Start(context.Background()) // 幂等 — 不 panic
	w.Stop()
	w.Stop() // 幂等
}

func TestReconcile_Tick_NoPending_Noop(t *testing.T) {
	w, _, reconRepo, _ := newReconcileWorkerSetup(t)
	// 空的 reconRepo → 没有 pending
	require.NoError(t, w.tick(context.Background()))
	assert.Equal(t, 0, len(reconRepo.batches))
}

func TestReconcile_Tick_ProcessesPending(t *testing.T) {
	w, payRepo, reconRepo, gw := newReconcileWorkerSetup(t)

	// 两个 batch: 一个 pending, 一个 completed
	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	pendingBatch := newBatch("rb-1", "alipay", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), pendingBatch))

	completedBatch := newBatch("rb-2", "alipay", batchDate)
	completedBatch.Status = string(domain.ReconBatchStatusCompleted)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), completedBatch))

	// 渠道 + 本地 2 笔匹配
	gw.statementLines["2025-10-01"] = []domain.StatementLine{
		{OutTradeNo: "out-1", Amount: 100.0, State: string(domain.AttemptStateSucceeded)},
	}
	payRepo.items["p-1"] = newPayment("p-1", "out-1", 100.0, domain.StateSucceeded)

	require.NoError(t, w.tick(context.Background()))

	// rb-1 应该变成 completed, rb-2 不变
	assert.Equal(t, string(domain.ReconBatchStatusCompleted), reconRepo.batches["rb-1"].Status)
	assert.Equal(t, string(domain.ReconBatchStatusCompleted), reconRepo.batches["rb-2"].Status)
}

func TestReconcile_ChannelDoesntSupportStatement_Fails(t *testing.T) {
	payRepo := newReconFakePaymentsRepo()
	reconRepo := newFakeReconRepo()
	gw := newReconFakeGateway("legacy")
	gw.meta.SupportsStatement = false
	reg := NewGatewayRegistry()
	reg.Register("legacy", gw)

	w := NewReconcileWorker(
		payRepo, reconRepo,
		reconFixedClock{}, &reconFixedUUID{},
		reg, 0, 10,
	)

	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "legacy", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	err := w.reconcileOne(context.Background(), batch)
	assert.Error(t, err)
	updated := reconRepo.batches["rb-1"]
	assert.Equal(t, string(domain.ReconBatchStatusFailed), updated.Status)
}

func TestReconcileTick_MarksFailedBatchAndContinues(t *testing.T) {
	payRepo := newReconFakePaymentsRepo()
	reconRepo := newFakeReconRepo()
	gw := newReconFakeGateway("legacy")
	gw.meta.SupportsStatement = false
	reg := NewGatewayRegistry()
	reg.Register("legacy", gw)
	w := NewReconcileWorker(
		payRepo, reconRepo,
		reconFixedClock{}, &reconFixedUUID{},
		reg, 0, 10,
	)

	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "legacy", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	require.NoError(t, w.tick(context.Background()))
	updated := reconRepo.batches["rb-1"]
	assert.Equal(t, string(domain.ReconBatchStatusFailed), updated.Status)
}

func TestReconcileTick_PersistFailureDoesNotAbortTick(t *testing.T) {
	payRepo := newReconFakePaymentsRepo()
	reconRepo := newFakeReconRepo()
	gw := newReconFakeGateway("legacy")
	gw.meta.SupportsStatement = false
	reg := NewGatewayRegistry()
	reg.Register("legacy", gw)
	w := NewReconcileWorker(
		payRepo, reconRepo,
		reconFixedClock{}, &reconFixedUUID{},
		reg, 0, 10,
	)

	batchDate := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)
	batch := newBatch("rb-1", "legacy", batchDate)
	require.NoError(t, reconRepo.CreateBatch(context.Background(), batch))

	reconRepo.updateErr = errStringError("persist unavailable")
	require.NoError(t, w.tick(context.Background()))
	assert.Equal(t, string(domain.ReconBatchStatusPending), reconRepo.batches["rb-1"].Status)
}

func (r *fakeReconRepo) ApproveItemForOrganization(context.Context, string, string, string) (string, error) {
	return "resolver", nil
}
