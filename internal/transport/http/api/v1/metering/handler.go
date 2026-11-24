package metering

import (
	"context"
	"encoding/json"

	appmetering "github.com/guliangiam/guliangiam/internal/app/metering"
)

// Handler adapts HTTP to the metering app service.
type Handler struct {
	Service appmetering.Service
}

func (h Handler) ListEvents(ctx context.Context, q appmetering.ListEventsQuery) ([]appmetering.EventDTO, int64, error) {
	return h.Service.ListEvents(ctx, q)
}

func (h Handler) ListAggregates(ctx context.Context, q appmetering.ListAggregatesQuery) ([]appmetering.AggregateDTO, int64, error) {
	return h.Service.ListAggregates(ctx, q)
}

type IngestEventRequest struct {
	EventType      string          `json:"eventType"`
	Subject        string          `json:"subject"`
	Quantity       int64           `json:"quantity"`
	Unit           string          `json:"unit"`
	IdempotencyKey string          `json:"idempotencyKey"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

func (h Handler) Ingest(ctx context.Context, orgID string, reqs []IngestEventRequest) (inserted int, duplicates int, err error) {
	cmds := make([]appmetering.IngestEventCommand, 0, len(reqs))
	for _, r := range reqs {
		cmds = append(cmds, appmetering.IngestEventCommand{
			OrganizationID: orgID,
			EventType:      r.EventType,
			Subject:        r.Subject,
			Quantity:       r.Quantity,
			Unit:           r.Unit,
			IdempotencyKey: r.IdempotencyKey,
			Metadata:       r.Metadata,
		})
	}
	return h.Service.Ingest(ctx, orgID, cmds)
}

func (h Handler) RefreshAggregate(ctx context.Context, orgID string, period appmetering.Period, subject, metric string) (*appmetering.AggregateDTO, error) {
	return h.Service.RefreshAggregate(ctx, appmetering.RefreshAggregateCommand{
		OrganizationID: orgID,
		Period:         period,
		Subject:        subject,
		Metric:         metric,
	})
}
