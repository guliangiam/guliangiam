package providers

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformoutbox "github.com/guliangiam/guliangiam/internal/platform/outbox"
)

const (
	ConsistencyAggregateType   = "provider_consistency"
	ConsistencyDifferenceEvent = "provider.consistency.difference"
	ConsistencyCompletedEvent  = "provider.consistency.completed"
)

func (w *ConsistencyWorker) consistencyEvents(report ConsistencyReport) ([]*domainoutbox.OutboxEvent, error) {
	events := make([]*domainoutbox.OutboxEvent, 0, len(report.Differences)+1)
	for _, difference := range report.Differences {
		id, err := w.UUID.NewString()
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(struct {
			RunID      string                `json:"runId"`
			CheckedAt  string                `json:"checkedAt"`
			Digest     string                `json:"digest"`
			Difference ConsistencyDifference `json:"difference"`
		}{report.RunID, report.CheckedAt.Format(time.RFC3339Nano), report.Digest, difference})
		if err != nil {
			return nil, err
		}
		headers, err := json.Marshal(map[string]any{
			"run_id": report.RunID, "organization_id": difference.OrganizationID, "severity": "warning",
		})
		if err != nil {
			return nil, err
		}
		// aggregate_id is a UUID column. Each difference event is its own
		// aggregate, so the per-event UUID is the natural aggregate id; the
		// run id stays in payload/headers for correlation.
		var organizationID *string
		if difference.OrganizationID != "" {
			value := difference.OrganizationID
			organizationID = &value
		}
		events = append(events, &domainoutbox.OutboxEvent{
			ID:             id,
			OrganizationID: organizationID,
			AggregateType:  ConsistencyAggregateType,
			AggregateID:    id,
			EventType:      ConsistencyDifferenceEvent,
			EventVersion:   1,
			Payload:        payload,
			Headers:        headers,
			MaxAttempts:    5,
			AvailableAt:    report.CheckedAt,
		})
	}

	id, err := w.UUID.NewString()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(report)
	if err != nil {
		return nil, err
	}
	headers, err := json.Marshal(map[string]any{
		"run_id": report.RunID, "difference_count": report.DifferenceCount, "digest": report.Digest,
	})
	if err != nil {
		return nil, err
	}
	events = append(events, &domainoutbox.OutboxEvent{
		ID:            id,
		AggregateType: ConsistencyAggregateType,
		AggregateID:   report.RunID,
		EventType:     ConsistencyCompletedEvent,
		EventVersion:  1,
		Payload:       payload,
		Headers:       headers,
		MaxAttempts:   5,
		AvailableAt:   report.CheckedAt,
	})
	return events, nil
}

type consistencyOutboxHandler struct {
	logger *slog.Logger
}

func NewConsistencyOutboxHandler(logger *slog.Logger) platformoutbox.DeliveryHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &consistencyOutboxHandler{logger: logger.With("component", "provider-consistency-outbox")}
}

func (h *consistencyOutboxHandler) Handle(ctx context.Context, item *platformoutbox.OutboxItem) error {
	var envelope struct {
		RunID           string                 `json:"runId"`
		Digest          string                 `json:"digest"`
		DifferenceCount int                    `json:"differenceCount"`
		Difference      *ConsistencyDifference `json:"difference"`
	}
	if err := json.Unmarshal(item.Payload, &envelope); err != nil {
		return apperrors.WrapInternal(err, "decode provider consistency outbox payload")
	}
	if envelope.RunID == "" {
		return apperrors.NewInternal("provider consistency outbox runId is required")
	}
	switch item.EventType {
	case ConsistencyDifferenceEvent:
		if envelope.Difference == nil {
			return apperrors.NewInternal("provider consistency difference payload is required")
		}
		difference := envelope.Difference
		h.logger.WarnContext(ctx, "provider consistency difference",
			"run_id", envelope.RunID,
			"provider_id", difference.ProviderID,
			"detail_id", difference.DetailID,
			"family", difference.Family,
			"code", difference.Code,
			"expected", difference.Expected,
			"actual", difference.Actual,
			"digest", envelope.Digest,
		)
	case ConsistencyCompletedEvent:
		h.logger.InfoContext(ctx, "provider consistency run recorded",
			"run_id", envelope.RunID, "differences", envelope.DifferenceCount, "digest", envelope.Digest)
	default:
		// The event type is written by this service, so an unknown value is an
		// internal contract mismatch, not a caller's 4xx (手册 4.14.1).
		return apperrors.NewInternalf("unsupported provider consistency event: %s", item.EventType)
	}
	return nil
}
