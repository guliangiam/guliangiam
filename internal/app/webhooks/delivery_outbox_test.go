package webhooks

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	recordsmock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

type mockDeliverySender struct {
	result domain.DeliveryResult

	gotURL     string
	gotHeaders map[string]string
	gotPayload []byte
	gotSecret  string
}

func (m *mockDeliverySender) Send(ctx context.Context, url string, headers map[string]string, payload []byte, secret string) domain.DeliveryResult {
	m.gotURL = url
	m.gotHeaders = headers
	m.gotPayload = payload
	m.gotSecret = secret
	return m.result
}

func deliveryTestItem(attempts, maxAttempts int) *platformoutbox.OutboxItem {
	return &platformoutbox.OutboxItem{
		ID:            "evt-1",
		AggregateType: "webhook",
		AggregateID:   "wh-1",
		EventType:     "user.created",
		Payload:       []byte(`{"id":"u1"}`),
		Attempts:      attempts,
		MaxAttempts:   maxAttempts,
	}
}

func deliveryTestLedger() *mockEventRepo {
	now := time.Now()
	return &mockEventRepo{
		events: []*eventdomain.WebhookEvent{
			{ID: "evt-1", WebhookID: "wh-1", EventType: "user.created", Payload: json.RawMessage(`{"id":"u1"}`), Status: "pending", CreatedAt: now, UpdatedAt: now},
		},
	}
}

func enabledDeliveryWebhook() *mockWebhookRepo {
	return &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-9", IsEnabled: true, URL: "https://example.com/hook", Secret: "test-signing-secret"},
		},
	}
}

func TestDeliveryOutbox_Success_MarksLedgerDelivered(t *testing.T) {
	eventRepo := deliveryTestLedger()
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 200, ResponseBody: "ok"}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: eventRepo}, sender, nil)

	require.NoError(t, h.Handle(context.Background(), deliveryTestItem(0, 5)))
	e := eventRepo.events[0]
	assert.Equal(t, "delivered", e.Status)
	assert.Equal(t, 200, e.ResponseCode)
	assert.Equal(t, "ok", e.ResponseBody)
	assert.Nil(t, e.NextRetryAt)
	assert.Equal(t, "https://example.com/hook", sender.gotURL)
	assert.Equal(t, "test-signing-secret", sender.gotSecret)
	assert.Equal(t, "evt-1", sender.gotHeaders["X-GuliangIAM-Delivery"])
	assert.Equal(t, "user.created", sender.gotHeaders["X-GuliangIAM-Event"])
	assert.Equal(t, "application/json", sender.gotHeaders["Content-Type"])
}

// H40 回归：重放路径 outbox 是新行，Headers.event_id 必须指回原台账行。
func TestDeliveryOutbox_ReplayHeadersEventID_MapsToLedger(t *testing.T) {
	eventRepo := deliveryTestLedger()
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 204}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: eventRepo}, sender, nil)

	item := deliveryTestItem(0, 5)
	item.ID = "replay-outbox-row"
	item.Headers = []byte(`{"event_id":"evt-1"}`)
	require.NoError(t, h.Handle(context.Background(), item))
	assert.Equal(t, "delivered", eventRepo.events[0].Status)
	assert.Equal(t, "evt-1", sender.gotHeaders["X-GuliangIAM-Delivery"])
}

func TestDeliveryOutbox_LedgerMissing_IsPermanent(t *testing.T) {
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 200}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: &mockEventRepo{}}, sender, nil)

	err := h.Handle(context.Background(), deliveryTestItem(0, 5))
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
	assert.Empty(t, sender.gotURL, "must not deliver when the ledger row is gone")
}

// BE-B-8 口径保留：父配置已删除 → 台账终结为 failed，不留 pending。
func TestDeliveryOutbox_ConfigGone_FinalizesFailed(t *testing.T) {
	eventRepo := deliveryTestLedger()
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: &mockWebhookRepo{}, eventRepo: eventRepo}, nil, nil)

	err := h.Handle(context.Background(), deliveryTestItem(0, 5))
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
	e := eventRepo.events[0]
	assert.Equal(t, "failed", e.Status)
	assert.Equal(t, "webhook config not found", e.ResponseBody)
	assert.Nil(t, e.NextRetryAt)
}

func TestDeliveryOutbox_DisabledWebhook_FinalizesFailedWithAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventRepo := deliveryTestLedger()
	webhookRepo := enabledDeliveryWebhook()
	webhookRepo.webhooks[0].IsEnabled = false

	recordsRepo := recordsmock.NewMockRepository(ctrl)
	recordsRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r *recordsdomain.Record) error {
			assert.Equal(t, "webhook_event.failed", r.Action)
			assert.Equal(t, "evt-1", r.ResourceID)
			assert.Equal(t, "org-9", r.OrganizationID)
			return nil
		}).Times(1)

	uow := replayAuditUOW{
		testUnitOfWork: testUnitOfWork{webhookRepo: webhookRepo, eventRepo: eventRepo},
		records:        recordsRepo,
	}
	h := NewDeliveryOutboxHandler(uow, nil, nil)

	err := h.Handle(context.Background(), deliveryTestItem(0, 5))
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
	assert.Equal(t, "failed", eventRepo.events[0].Status)
	assert.Equal(t, "webhook is disabled", eventRepo.events[0].ResponseBody)
}

func TestDeliveryOutbox_EmptySecret_RefusesDelivery(t *testing.T) {
	eventRepo := deliveryTestLedger()
	webhookRepo := enabledDeliveryWebhook()
	webhookRepo.webhooks[0].Secret = "  "
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 200}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: webhookRepo, eventRepo: eventRepo}, sender, nil)

	err := h.Handle(context.Background(), deliveryTestItem(0, 5))
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
	assert.Empty(t, sender.gotURL)
	assert.Equal(t, "failed", eventRepo.events[0].Status)
	assert.Contains(t, eventRepo.events[0].ResponseBody, "no signing secret")
}

// DEF-28 口径保留：不可重试 4xx（含 410）立即 failed，不消耗重试预算。
func TestDeliveryOutbox_NonRetryableStatus_FinalizesFailed(t *testing.T) {
	eventRepo := deliveryTestLedger()
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 410, ResponseBody: "gone"}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: eventRepo}, sender, nil)

	err := h.Handle(context.Background(), deliveryTestItem(0, 5))
	require.Error(t, err)
	assert.True(t, platformoutbox.IsPermanent(err))
	e := eventRepo.events[0]
	assert.Equal(t, "failed", e.Status)
	assert.Equal(t, 410, e.ResponseCode)
	assert.Contains(t, e.ResponseBody, "410")
}

// H40 核心回归：可重试失败必须同步台账（pending + retry_count=attempts+1），
// 调度交给 outbox（next_retry_at 留空），错误可重试让 Worker MarkRetry。
func TestDeliveryOutbox_RetryableStatus_RecordsAttempt(t *testing.T) {
	eventRepo := deliveryTestLedger()
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 503, ResponseBody: "unavailable"}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: eventRepo}, sender, nil)

	err := h.Handle(context.Background(), deliveryTestItem(1, 5))
	require.Error(t, err)
	assert.False(t, platformoutbox.IsPermanent(err))
	e := eventRepo.events[0]
	assert.Equal(t, "pending", e.Status)
	assert.Equal(t, 2, e.RetryCount)
	assert.Equal(t, 503, e.ResponseCode)
	assert.Equal(t, "unavailable", e.ResponseBody)
	assert.Nil(t, e.NextRetryAt, "scheduling facts live in outbox.available_at")
}

// 最后一次尝试（attempts+1 >= max_attempts）必须终结台账 —— Worker 的
// MarkRetry 到达上限即死信，不会再有下一轮 Handle 来补写 failed。
func TestDeliveryOutbox_LastAttemptExhausted_FinalizesFailedWithAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	eventRepo := deliveryTestLedger()
	recordsRepo := recordsmock.NewMockRepository(ctrl)
	recordsRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r *recordsdomain.Record) error {
			assert.Equal(t, "webhook_event.failed", r.Action)
			assert.Equal(t, "org-9", r.OrganizationID)
			return nil
		}).Times(1)
	uow := replayAuditUOW{
		testUnitOfWork: testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: eventRepo},
		records:        recordsRepo,
	}
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 500, ResponseBody: "boom"}}
	h := NewDeliveryOutboxHandler(uow, sender, nil)

	err := h.Handle(context.Background(), deliveryTestItem(4, 5))
	require.Error(t, err)
	e := eventRepo.events[0]
	assert.Equal(t, "failed", e.Status)
	assert.Equal(t, 500, e.ResponseCode)
	assert.Nil(t, e.NextRetryAt)
}

func TestDeliveryOutbox_SendError_RecordsAttempt(t *testing.T) {
	eventRepo := deliveryTestLedger()
	sender := &mockDeliverySender{result: domain.DeliveryResult{Error: assert.AnError}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: eventRepo}, sender, nil)

	err := h.Handle(context.Background(), deliveryTestItem(0, 5))
	require.Error(t, err)
	assert.False(t, platformoutbox.IsPermanent(err))
	e := eventRepo.events[0]
	assert.Equal(t, "pending", e.Status)
	assert.Equal(t, 1, e.RetryCount)
	assert.Contains(t, e.ResponseBody, "delivery error")
}

// BE-B-8 口径保留：瞬时配置读取失败按可重试处理并记录尝试进度，不热循环。
func TestDeliveryOutbox_TransientLookupError_RecordsAttemptAndRetries(t *testing.T) {
	eventRepo := deliveryTestLedger()
	webhookRepo := enabledDeliveryWebhook()
	webhookRepo.getByIDErr = assert.AnError
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: webhookRepo, eventRepo: eventRepo}, nil, nil)

	err := h.Handle(context.Background(), deliveryTestItem(1, 5))
	require.Error(t, err)
	assert.False(t, platformoutbox.IsPermanent(err))
	e := eventRepo.events[0]
	assert.Equal(t, "pending", e.Status)
	assert.Equal(t, 2, e.RetryCount)
	assert.Contains(t, e.ResponseBody, "webhook config lookup failed")
}

func TestDeliveryOutbox_TransientLedgerReadError_Retryable(t *testing.T) {
	eventRepo := deliveryTestLedger()
	eventRepo.getByIDErr = assert.AnError
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: enabledDeliveryWebhook(), eventRepo: eventRepo}, nil, nil)

	err := h.Handle(context.Background(), deliveryTestItem(0, 5))
	require.Error(t, err)
	assert.False(t, platformoutbox.IsPermanent(err))
}

func TestDeliveryOutbox_CustomBodyAndHeaders(t *testing.T) {
	eventRepo := deliveryTestLedger()
	webhookRepo := enabledDeliveryWebhook()
	webhookRepo.webhooks[0].CustomBody = `{"custom":true}`
	webhookRepo.webhooks[0].Headers = `{"X-Api-Key":"k"}`
	webhookRepo.webhooks[0].ContentType = "application/cloudevents+json"
	sender := &mockDeliverySender{result: domain.DeliveryResult{StatusCode: 200}}
	h := NewDeliveryOutboxHandler(testUnitOfWork{webhookRepo: webhookRepo, eventRepo: eventRepo}, sender, nil)

	require.NoError(t, h.Handle(context.Background(), deliveryTestItem(0, 5)))
	assert.Equal(t, `{"custom":true}`, string(sender.gotPayload))
	assert.Equal(t, "k", sender.gotHeaders["X-Api-Key"])
	assert.Equal(t, "application/cloudevents+json", sender.gotHeaders["Content-Type"])
}
