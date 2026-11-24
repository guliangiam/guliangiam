package webhooks

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	"github.com/guliangiam/guliangiam/internal/domain/webhooks"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

// fakeDomainOutbox 记录领域事件 fan-out 产生的 webhook 投递行。
type fakeDomainOutbox struct {
	items            []*domainoutbox.OutboxEvent
	enqueueCalls     int
	enqueueBatchCall int
}

func (f *fakeDomainOutbox) Enqueue(ctx context.Context, item *domainoutbox.OutboxEvent) error {
	f.enqueueCalls++
	f.items = append(f.items, item)
	return nil
}

func (f *fakeDomainOutbox) EnqueueBatch(ctx context.Context, items []*domainoutbox.OutboxEvent) error {
	f.enqueueBatchCall++
	f.items = append(f.items, items...)
	return nil
}

func TestDomainEventOutboxHandler_FansOutPerAggregate(t *testing.T) {
	const (
		orgID     = "11111111-1111-1111-1111-111111111111"
		webhookID = "22222222-2222-2222-2222-222222222222"
	)

	cases := []struct {
		aggregate string
		eventType string
	}{
		{AggregateOrganization, "control.tenant_registered"},
		{AggregateRisk, "risk.evaluation.completed"},
		{AggregateUser, "user.created"},
		{AggregateSession, "session.created"},
	}

	for _, tc := range cases {
		t.Run(tc.aggregate, func(t *testing.T) {
			org := orgID
			whRepo := &mockWebhookRepo{webhooks: []*webhooks.Webhook{{
				ID:             webhookID,
				OrganizationID: orgID,
				URL:            "https://hooks.example.com/outbox",
				Method:         "POST",
				Events:         []string{tc.eventType},
				IsEnabled:      true,
				RetryCount:     3,
			}}}
			evRepo := &mockEventRepo{}
			outboxRepo := &fakeDomainOutbox{}
			uow := testUnitOfWork{webhookRepo: whRepo, eventRepo: evRepo, outboxRepo: outboxRepo}
			handler := NewDomainEventOutboxHandler(uow, nil)

			item := &platformoutbox.OutboxItem{
				ID:             "event-" + tc.aggregate,
				OrganizationID: &org,
				AggregateType:  tc.aggregate,
				AggregateID:    orgID,
				EventType:      tc.eventType,
				Payload:        []byte(`{"organizationId":"` + orgID + `"}`),
			}

			if err := handler.Handle(context.Background(), item); err != nil {
				t.Fatalf("Handle returned error: %v", err)
			}
			// 租户上下文：订阅查询必须带事件所属组织与事件类型。
			if whRepo.lastOrgID != orgID || whRepo.lastEvent != tc.eventType {
				t.Fatalf("tenant/event not propagated: org=%q event=%q", whRepo.lastOrgID, whRepo.lastEvent)
			}
			if len(evRepo.events) != 1 {
				t.Fatalf("webhook_events = %d, want 1", len(evRepo.events))
			}
			if len(outboxRepo.items) != 1 {
				t.Fatalf("outbox items = %d, want 1", len(outboxRepo.items))
			}
			got := outboxRepo.items[0]
			if got.EventType != tc.eventType || got.AggregateType != "webhook" || got.AggregateID != webhookID {
				t.Fatalf("unexpected delivery: %+v", got)
			}
			if got.OrganizationID == nil || *got.OrganizationID != orgID {
				t.Fatalf("delivery lost organization_id: %+v", got.OrganizationID)
			}
			if got.MaxAttempts != 3 {
				t.Fatalf("max attempts = %d, want webhook retry_count 3", got.MaxAttempts)
			}
		})
	}
}

func TestDomainEventOutboxHandler_ReplayIsIdempotent(t *testing.T) {
	const (
		orgID     = "11111111-1111-1111-1111-111111111111"
		webhookID = "22222222-2222-2222-2222-222222222222"
	)
	cases := []struct {
		aggregate string
		eventType string
	}{
		{AggregateOrganization, "control.tenant_registered"},
		{AggregateRisk, "risk.evaluation.completed"},
		{AggregateUser, "user.created"},
		{AggregateSession, "session.created"},
	}

	for _, tc := range cases {
		t.Run(tc.aggregate, func(t *testing.T) {
			org := orgID
			whRepo := &mockWebhookRepo{webhooks: []*webhooks.Webhook{{
				ID: webhookID, OrganizationID: orgID, URL: "https://hooks.example.com",
				Method: "POST", Events: []string{tc.eventType}, IsEnabled: true, RetryCount: 3,
			}}}
			outboxRepo := &fakeDomainOutbox{}
			evRepo := &mockEventRepo{}
			uow := testUnitOfWork{webhookRepo: whRepo, eventRepo: evRepo, outboxRepo: outboxRepo}
			handler := NewDomainEventOutboxHandler(uow, nil)
			item := &platformoutbox.OutboxItem{
				ID: "event-" + tc.aggregate, OrganizationID: &org, AggregateType: tc.aggregate,
				AggregateID: orgID, EventType: tc.eventType, Payload: []byte(`{"id":"u1"}`),
			}

			if err := handler.Handle(context.Background(), item); err != nil {
				t.Fatalf("first Handle: %v", err)
			}
			detID := deterministicDeliveryID(item.ID, webhookID)
			if len(evRepo.events) != 1 || evRepo.events[0].ID != detID {
				t.Fatalf("first enqueue id = %v, want deterministic %s", evRepo.events, detID)
			}

			// 模拟 worker 重放同一条 outbox 事件。
			if err := handler.Handle(context.Background(), item); err != nil {
				t.Fatalf("replay Handle: %v", err)
			}
			if len(evRepo.events) != 1 {
				t.Fatalf("replay created duplicate webhook_events: %d", len(evRepo.events))
			}
			if len(outboxRepo.items) != 1 {
				t.Fatalf("replay created duplicate delivery: %d", len(outboxRepo.items))
			}
		})
	}
}

func TestDomainEventOutboxHandler_ClassifiesPermanentFailures(t *testing.T) {
	handler := NewDomainEventOutboxHandler(testUnitOfWork{}, nil)

	cases := []struct {
		name string
		item *platformoutbox.OutboxItem
	}{
		{"missing organization", &platformoutbox.OutboxItem{
			ID: "e1", AggregateType: AggregateUser, EventType: "user.created", Payload: []byte(`{}`),
		}},
		{"not dot namespaced", &platformoutbox.OutboxItem{
			ID: "e2", OrganizationID: ptr("org"), AggregateType: AggregateUser,
			EventType: "created", Payload: []byte(`{}`),
		}},
		{"payload not object", &platformoutbox.OutboxItem{
			ID: "e3", OrganizationID: ptr("org"), AggregateType: AggregateUser,
			EventType: "user.created", Payload: []byte(`"scalar"`),
		}},
	}
	for _, tc := range cases {
		err := handler.Handle(context.Background(), tc.item)
		if err == nil || !platformoutbox.IsPermanent(err) {
			t.Fatalf("%s: err = %v, want permanent", tc.name, err)
		}
	}
}

func TestDomainEventOutboxHandler_DependencyErrorIsRetryable(t *testing.T) {
	handler := NewDomainEventOutboxHandler(testUnitOfWork{
		webhookRepo: &mockWebhookRepo{listErr: errors.New("db unavailable")},
		eventRepo:   &mockEventRepo{},
		outboxRepo:  &fakeDomainOutbox{},
	}, nil)

	item := &platformoutbox.OutboxItem{
		ID: "e4", OrganizationID: ptr("org"), AggregateType: AggregateRisk,
		EventType: "risk.evaluation.completed", Payload: []byte(`{}`),
	}
	err := handler.Handle(context.Background(), item)
	if err == nil || platformoutbox.IsPermanent(err) {
		t.Fatalf("dependency failure must stay retryable, got %v", err)
	}
}

func TestDomainEventOutboxHandler_NoSubscribersIsNoOp(t *testing.T) {
	org := "11111111-1111-1111-1111-111111111111"
	evRepo := &mockEventRepo{}
	outboxRepo := &fakeDomainOutbox{}
	handler := NewDomainEventOutboxHandler(testUnitOfWork{
		webhookRepo: &mockWebhookRepo{},
		eventRepo:   evRepo,
		outboxRepo:  outboxRepo,
	}, nil)

	item := &platformoutbox.OutboxItem{
		ID: "e5", OrganizationID: &org, AggregateType: AggregateOrganization,
		EventType: "control.tenant_decommissioned", Payload: []byte(`{}`),
	}
	if err := handler.Handle(context.Background(), item); err != nil {
		t.Fatalf("no-subscriber event must be acknowledged: %v", err)
	}
	if len(evRepo.events) != 0 || len(outboxRepo.items) != 0 {
		t.Fatalf("no-subscriber event created side effects: events=%d outbox=%d",
			len(evRepo.events), len(outboxRepo.items))
	}
}

// TestDomainEventOutboxHandler_BatchFanOut pins LEFT-R0-07: the whole
// subscriber set is settled with exactly one ListExistingIDs pre-flight and
// one CreateBatch + EnqueueBatch inside a single transaction — no per-
// subscriber GetByID, no per-delivery transaction. Disabled subscribers are
// excluded; RetryCount<=0 falls back to the default 5 attempts.
func TestDomainEventOutboxHandler_BatchFanOut(t *testing.T) {
	const (
		orgID      = "11111111-1111-1111-1111-111111111111"
		webhookA   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		webhookB   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		webhookOff = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	org := orgID
	whRepo := &mockWebhookRepo{webhooks: []*webhooks.Webhook{
		{ID: webhookA, OrganizationID: orgID, URL: "https://hooks.example.com/a",
			Method: "POST", Events: []string{"user.created"}, IsEnabled: true, RetryCount: 0},
		{ID: webhookB, OrganizationID: orgID, URL: "https://hooks.example.com/b",
			Method: "POST", Events: []string{"user.created"}, IsEnabled: true, RetryCount: 4},
		{ID: webhookOff, OrganizationID: orgID, URL: "https://hooks.example.com/c",
			Method: "POST", Events: []string{"user.created"}, IsEnabled: false, RetryCount: 2},
	}}
	evRepo := &mockEventRepo{}
	outboxRepo := &fakeDomainOutbox{}
	handler := NewDomainEventOutboxHandler(
		testUnitOfWork{webhookRepo: whRepo, eventRepo: evRepo, outboxRepo: outboxRepo}, nil)

	item := &platformoutbox.OutboxItem{
		ID: "event-batch", OrganizationID: &org, AggregateType: AggregateUser,
		AggregateID: orgID, EventType: "user.created", Payload: []byte(`{"id":"u1"}`),
	}
	require.NoError(t, handler.Handle(context.Background(), item))

	assert.Equal(t, 1, evRepo.listExistingCalls, "one batched idempotency pre-flight")
	assert.Equal(t, 0, evRepo.getByIDCalls, "fan-out must not do per-subscriber GetByID")
	assert.Equal(t, 1, evRepo.createBatchCalls, "one CreateBatch for all missing deliveries")
	assert.Equal(t, 1, outboxRepo.enqueueBatchCall, "one EnqueueBatch for all deliveries")
	assert.Equal(t, 0, outboxRepo.enqueueCalls, "fan-out must not do per-item Enqueue")

	require.Len(t, evRepo.events, 2, "disabled webhook must not be fanned out")
	require.Len(t, outboxRepo.items, 2)
	byWebhook := map[string]int{}
	for _, d := range outboxRepo.items {
		byWebhook[d.AggregateID] = d.MaxAttempts
	}
	assert.Equal(t, map[string]int{webhookA: 5, webhookB: 4}, byWebhook,
		"RetryCount<=0 falls back to 5, explicit retry count preserved")
}

// TestDomainEventOutboxHandler_PartialReplayEnqueuesOnlyMissing pins the
// replay semantics of the batch path: when one subscriber's deterministic
// delivery id already exists, only the missing one is written.
func TestDomainEventOutboxHandler_PartialReplayEnqueuesOnlyMissing(t *testing.T) {
	const (
		orgID    = "11111111-1111-1111-1111-111111111111"
		webhookA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		webhookB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	org := orgID
	item := &platformoutbox.OutboxItem{
		ID: "event-partial", OrganizationID: &org, AggregateType: AggregateUser,
		AggregateID: orgID, EventType: "user.created", Payload: []byte(`{"id":"u1"}`),
	}
	whRepo := &mockWebhookRepo{webhooks: []*webhooks.Webhook{
		{ID: webhookA, OrganizationID: orgID, URL: "https://hooks.example.com/a",
			Method: "POST", Events: []string{"user.created"}, IsEnabled: true, RetryCount: 3},
		{ID: webhookB, OrganizationID: orgID, URL: "https://hooks.example.com/b",
			Method: "POST", Events: []string{"user.created"}, IsEnabled: true, RetryCount: 3},
	}}
	evRepo := &mockEventRepo{events: []*eventdomain.WebhookEvent{
		{ID: deterministicDeliveryID(item.ID, webhookA), WebhookID: webhookA},
	}}
	outboxRepo := &fakeDomainOutbox{}
	handler := NewDomainEventOutboxHandler(
		testUnitOfWork{webhookRepo: whRepo, eventRepo: evRepo, outboxRepo: outboxRepo}, nil)

	require.NoError(t, handler.Handle(context.Background(), item))

	require.Len(t, evRepo.events, 2, "only the missing delivery is created")
	assert.Equal(t, deterministicDeliveryID(item.ID, webhookB), evRepo.events[1].ID)
	require.Len(t, outboxRepo.items, 1)
	assert.Equal(t, webhookB, outboxRepo.items[0].AggregateID)
}

func ptr(s string) *string { return &s }
