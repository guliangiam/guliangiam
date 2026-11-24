package webhooks

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	recordsdomain "github.com/guliangiam/guliangiam/internal/domain/records"
	recordsmock "github.com/guliangiam/guliangiam/internal/domain/records/mock"
	eventdomain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhooks"
)

// replayAuditUOW exposes a capturing Records repository on top of the shared
// hand-written test UOW so replay audit writes become observable. It is not a
// gomock type, so audit.EmbedAuditEvent does not skip it.
type replayAuditUOW struct {
	testUnitOfWork
	records *recordsmock.MockRepository
}

func (u replayAuditUOW) Records() recordsdomain.Repository { return u.records }

// Override so the transaction body sees the wrapper (with the capturing
// Records repo), not the embedded base UOW.
func (u replayAuditUOW) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx appuow.UnitOfWork) error) error {
	return fn(ctx, u)
}

// BE-B-22 regression: a replay writes exactly ONE audit record
// (webhook_event.replayed). The previous implementation fabricated
// failed+delivered records for a delivery that never happened.
func TestService_ReplayWebhookEvent_SingleReplayedAudit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	future := now.Add(1 * time.Hour)
	eventRepo := &mockEventRepo{
		events: []*eventdomain.WebhookEvent{
			{ID: "evt-1", WebhookID: "wh-1", Status: "failed", RetryCount: 3, NextRetryAt: &future, CreatedAt: now, UpdatedAt: now},
		},
	}
	recordsRepo := recordsmock.NewMockRepository(ctrl)
	var captured []*recordsdomain.Record
	recordsRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r *recordsdomain.Record) error {
			captured = append(captured, r)
			return nil
		}).Times(1)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "replay-outbox-1"},
		UOW: replayAuditUOW{
			testUnitOfWork: testUnitOfWork{
				webhookRepo: &mockWebhookRepo{webhooks: []*domain.Webhook{{ID: "wh-1", OrganizationID: "org-9"}}},
				eventRepo:   eventRepo,
				outboxRepo:  &captureOutboxRepo{},
			},
			records: recordsRepo,
		},
	}

	err := svc.ReplayWebhookEvent(platformScopeContext(), "evt-1", "")
	assert.NoError(t, err)

	if assert.Len(t, captured, 1) {
		assert.Equal(t, "webhook_event.replayed", captured[0].Action)
		assert.Equal(t, "evt-1", captured[0].ResourceID)
	}
	assert.Equal(t, "pending", eventRepo.events[0].Status)
	assert.Equal(t, 0, eventRepo.events[0].RetryCount)
	assert.Nil(t, eventRepo.events[0].NextRetryAt)
}

// BE-B-22 regression: under platform scope (empty organizationID) the replay
// audit tenant is resolved from the parent webhook instead of falling back to
// the built-in organization.
func TestService_ReplayWebhookEvent_AuditOrgFromParentWebhook(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	now := time.Now()
	eventRepo := &mockEventRepo{
		events: []*eventdomain.WebhookEvent{
			{ID: "evt-1", WebhookID: "wh-1", Status: "failed", RetryCount: 3, CreatedAt: now, UpdatedAt: now},
		},
	}
	webhookRepo := &mockWebhookRepo{
		webhooks: []*domain.Webhook{
			{ID: "wh-1", OrganizationID: "org-9"},
		},
	}
	recordsRepo := recordsmock.NewMockRepository(ctrl)
	recordsRepo.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, r *recordsdomain.Record) error {
			assert.Equal(t, "org-9", r.OrganizationID)
			assert.Equal(t, "webhook_event.replayed", r.Action)
			return nil
		}).Times(1)

	svc := Service{
		Clock: fakeClock{fixedTime: now},
		UUID:  fakeUUIDGenerator{fixedID: "replay-outbox-1"},
		UOW: replayAuditUOW{
			testUnitOfWork: testUnitOfWork{webhookRepo: webhookRepo, eventRepo: eventRepo, outboxRepo: &captureOutboxRepo{}},
			records:        recordsRepo,
		},
	}

	err := svc.ReplayWebhookEvent(platformScopeContext(), "evt-1", "")
	assert.NoError(t, err)
}
