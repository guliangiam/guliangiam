package webhooks

import (
	"context"
	"strings"
	"time"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domain "github.com/guliangiam/guliangiam/internal/domain/webhookevents"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ListWebhookEvents returns paginated webhook event delivery records.
func (s Service) ListWebhookEvents(ctx context.Context, webhookID, organizationID string, page, pageSize int, eventType string, status *string) ([]WebhookEventListItem, int64, error) {
	if err := requireOrganizationID(ctx, organizationID); err != nil {
		return nil, 0, err
	}
	// A regular caller must prove ownership of the parent webhook first. The
	// platform scope is authorized across tenants, so it skips the parent read
	// and lists with an empty organization filter.
	if strings.TrimSpace(organizationID) != "" {
		if _, err := loadWebhookForTenant(ctx, s.UOW.Webhooks(), webhookID, organizationID); err != nil {
			return nil, 0, err
		}
	}
	items, total, err := s.UOW.WebhookEvents().List(ctx, domain.ListQuery{
		Page: page, PageSize: pageSize, WebhookID: webhookID,
		EventType: eventType, Status: status, OrganizationID: organizationID,
	})
	if err != nil {
		return nil, 0, err
	}
	r := make([]WebhookEventListItem, 0, len(items))
	for _, i := range items {
		r = append(r, toEventListItem(i))
	}
	return r, total, nil
}

// GetWebhookEvent returns a single webhook event delivery record.
func (s Service) GetWebhookEvent(ctx context.Context, id, organizationID string) (*WebhookEventDetail, error) {
	e, err := loadWebhookEventForTenant(ctx, s.UOW.WebhookEvents(), id, organizationID)
	if err != nil {
		return nil, err
	}
	d := toEventDetail(e)
	return &d, nil
}

// ReplayWebhookEvent re-dispatches a previously recorded webhook event.
func (s Service) ReplayWebhookEvent(ctx context.Context, id, organizationID string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		e, err := loadWebhookEventForTenant(ctx, tx.WebhookEvents(), id, organizationID)
		if err != nil {
			return err
		}
		wh, err := tx.Webhooks().GetByID(ctx, e.WebhookID)
		if err != nil {
			return err
		}
		e.Status = "pending"
		e.RetryCount = 0
		e.NextRetryAt = nil
		e.UpdatedAt = time.Now()
		if err := tx.WebhookEvents().Update(ctx, e); err != nil {
			return err
		}

		// H40：重放 = 台账重置 + 同事务入队一条新 outbox 行，实际投递由
		// DeliveryOutboxHandler 执行。Headers.event_id 指回原台账行（outbox
		// 新行主键与台账不再相同）。只重置台账不入队会让事件永远滞留 pending。
		newID, err := s.UUID.NewString()
		if err != nil {
			return err
		}
		maxAttempts := wh.RetryCount
		if maxAttempts <= 0 {
			maxAttempts = 5
		}
		var orgPtr *string
		if wh.OrganizationID != "" {
			orgPtr = &wh.OrganizationID
		}
		if err := tx.Outbox().Enqueue(ctx, &domainoutbox.OutboxEvent{
			ID:             newID,
			OrganizationID: orgPtr,
			AggregateType:  "webhook",
			AggregateID:    e.WebhookID,
			EventType:      e.EventType,
			Payload:        e.Payload,
			Headers:        buildWebhookHeaders(wh.URL, wh.Secret, wh.Method, nil, e.ID),
			MaxAttempts:    maxAttempts,
		}); err != nil {
			return apperrors.WrapInternal(err, "replay webhook event: enqueue outbox")
		}

		// BE-B-22: a replay only queues the event; it does not deliver it. The
		// worker audits the real delivery outcome; only the replayed action is
		// recorded here. The audit tenant comes from the trusted caller scope;
		// for platform scope (empty organizationID) it falls back to the parent
		// webhook's organization.
		auditOrgID := organizationID
		if auditOrgID == "" {
			auditOrgID = wh.OrganizationID
		}
		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			auditOrgID, "",
			"webhook_event.replayed", "WebhookEvent", e.ID,
			"WebhookEvent", "webhook_event.replayed",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}

// DeleteWebhookEvent deletes a webhook event delivery record.
func (s Service) DeleteWebhookEvent(ctx context.Context, id, organizationID string) error {
	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if _, err := loadWebhookEventForTenant(ctx, tx.WebhookEvents(), id, organizationID); err != nil {
			return err
		}
		if err := tx.WebhookEvents().Delete(ctx, id); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx, ctx,
			"", "", "", "", "",
			"", "",
			"webhook_event.deleted", "WebhookEvent", id,
			"WebhookEvent", "webhook_event.deleted",
			map[string]any{},
		); err != nil {
			return err
		}
		return nil
	})
}
