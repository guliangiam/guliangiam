package metering

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/metering"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

type UnitOfWork = appuow.UnitOfWork

// Domain alias re-exports
type (
	MeteringEvent           = domain.MeteringEvent
	MeteringAggregate       = domain.MeteringAggregate
	Period                  = domain.Period
	ListEventsQuery         = domain.ListEventsQuery
	ListAggregatesQuery     = domain.ListAggregatesQuery
	IngestEventCommand      = domain.IngestEventCommand
	RefreshAggregateCommand = domain.RefreshAggregateCommand
)

// Service is the metering use-case facade. Events are append-only, so the only
// write path is Ingest; RefreshAggregate recomputes a single (org, period,
// subject, metric) cell from the raw events.
//
// Clock/UUID 走注入端口（BE-C-21）：此前 Ingest 直接调 time.Now()/uuid.NewString()
// 绕过了组合根注入的时钟与 ID 生成器，测试无法固定时间/ID。
type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.MeteringUoW
}

func NewService(uow UnitOfWork) Service {
	return Service{Clock: clock.SystemClock{}, UUID: platformuuid.V7Generator{}, UOW: uow}
}

// maxIngestBatchSize 限制单次上报的事件条数（BE-C-12）：无上限的批量让
// 一个请求就能写爆事件表并长时间占住配额事务。
const maxIngestBatchSize = 1000

// ─── DTOs ──────────────────────────────────────────────────────────────────

type EventDTO struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	EventType      string          `json:"eventType"`
	Subject        string          `json:"subject"`
	Quantity       int64           `json:"quantity"`
	Unit           string          `json:"unit"`
	OccurredAt     string          `json:"occurredAt"`
	Metadata       json.RawMessage `json:"metadata"`
	CreatedAt      string          `json:"createdAt"`
}

type AggregateDTO struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	Period         string `json:"period"`
	Subject        string `json:"subject"`
	Metric         string `json:"metric"`
	Quantity       int64  `json:"quantity"`
	Unit           string `json:"unit"`
	CalculatedAt   string `json:"calculatedAt"`
	CreatedAt      string `json:"createdAt"`
}

// ─── Use cases ─────────────────────────────────────────────────────────────

func (s *Service) ListEvents(ctx context.Context, q ListEventsQuery) ([]EventDTO, int64, error) {
	if q.OrganizationID == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId is required")
	}
	items, total, err := s.UOW.Metering().ListEvents(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]EventDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toEventDTO(it))
	}
	return dtos, total, nil
}

func (s *Service) ListAggregates(ctx context.Context, q ListAggregatesQuery) ([]AggregateDTO, int64, error) {
	if q.OrganizationID == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId is required")
	}
	items, total, err := s.UOW.Metering().ListAggregates(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	dtos := make([]AggregateDTO, 0, len(items))
	for _, it := range items {
		dtos = append(dtos, toAggregateDTO(it))
	}
	return dtos, total, nil
}

// GetEvent returns one metering event scoped to the caller's organization.
func (s *Service) GetEvent(ctx context.Context, organizationID, eventID string) (*EventDTO, error) {
	if err := validateLookup(organizationID, eventID); err != nil {
		return nil, err
	}
	reader, ok := s.UOW.Metering().(domain.DetailReader)
	if !ok {
		return nil, apperrors.NewUnimplemented("metering event detail is not supported by this storage backend")
	}
	event, err := reader.GetEventByID(ctx, eventID, organizationID)
	if err != nil {
		return nil, err
	}
	dto := toEventDTO(event)
	return &dto, nil
}

// GetAggregate returns one pre-rolled aggregate scoped to the caller's organization.
func (s *Service) GetAggregate(ctx context.Context, organizationID, aggregateID string) (*AggregateDTO, error) {
	if err := validateLookup(organizationID, aggregateID); err != nil {
		return nil, err
	}
	reader, ok := s.UOW.Metering().(domain.DetailReader)
	if !ok {
		return nil, apperrors.NewUnimplemented("metering aggregate detail is not supported by this storage backend")
	}
	aggregate, err := reader.GetAggregateByID(ctx, aggregateID, organizationID)
	if err != nil {
		return nil, err
	}
	dto := toAggregateDTO(aggregate)
	return &dto, nil
}

// Ingest accepts a batch of commands. Each row is idempotent by
// (organization_id, idempotency_key) — duplicates are silently skipped and
// the response reports how many were actually inserted vs deduped.
func (s *Service) Ingest(ctx context.Context, orgID string, cmds []IngestEventCommand) (inserted, duplicates int, err error) {
	if orgID == "" {
		return 0, 0, apperrors.NewInvalidArgument("organizationId is required")
	}
	if len(cmds) == 0 {
		return 0, 0, apperrors.NewInvalidArgument("at least one event required")
	}
	if len(cmds) > maxIngestBatchSize {
		return 0, 0, apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "ingest batch too large", map[string]any{
			"field": "events", "value": len(cmds), "max": maxIngestBatchSize,
		})
	}

	now := s.Clock.Now().UTC()
	events := make([]*domain.MeteringEvent, 0, len(cmds))
	for i := range cmds {
		c := &cmds[i]
		if c.EventType == "" || c.IdempotencyKey == "" {
			return 0, 0, apperrors.NewInvalidArgument("each event requires eventType + idempotencyKey")
		}
		// 用量只能为正（BE-C-12）：负数 Quantity 会经聚合减小 used_value，
		// 等于让上报方给自己“充值”配额、绕过限额。
		if c.Quantity <= 0 {
			return 0, 0, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "quantity must be positive", map[string]any{
				"field": "quantity", "value": c.Quantity, "min": 1,
			})
		}
		occurredAt := c.OccurredAt
		if occurredAt.IsZero() {
			occurredAt = now
		}
		metadata := c.Metadata
		if metadata == nil {
			metadata = json.RawMessage(`{}`)
		}
		id, idErr := s.UUID.NewString()
		if idErr != nil {
			return 0, 0, apperrors.WrapInternal(idErr, "generate metering event id")
		}
		events = append(events, &domain.MeteringEvent{
			ID:             id,
			OrganizationID: orgID,
			EventType:      c.EventType,
			Subject:        c.Subject,
			Quantity:       c.Quantity,
			Unit:           c.Unit,
			IdempotencyKey: c.IdempotencyKey,
			OccurredAt:     occurredAt,
			Metadata:       metadata,
			CreatedAt:      now,
		})
	}

	err = s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx UnitOfWork) error {
		var e error
		inserted, duplicates, e = tx.Metering().IngestBatch(ctx, events)
		if e != nil {
			e = apperrors.WrapInternal(e, "ingest batch")
		}

		// Tenant quota (DEF-22): the counter lives in tenant_quotas and is
		// consumed inside the same transaction as the events, so a rollback
		// never leaves usage behind. Only rows actually inserted are counted:
		// replayed batches are deduped by (organization_id, idempotency_key)
		// and must not advance the counter, so an all-duplicate batch consumes
		// nothing. Missing capability (MySQL mirror/tests) means "no quotas
		// configured" rather than a silent bypass of a configured quota.
		if e == nil && inserted > 0 {
			if consumer, ok := tx.Metering().(domain.QuotaConsumer); ok {
				status, qerr := consumer.ConsumeQuota(ctx, orgID, domain.QuotaKeyEvents, int64(inserted))
				if qerr != nil {
					return qerr
				}
				if !status.Allowed {
					return apperrors.NewQuotaExceeded("organization metering event quota exceeded", map[string]any{
						"quota":     domain.QuotaKeyEvents,
						"limit":     status.Limit,
						"used":      status.Used,
						"requested": inserted,
					})
				}
			}
		}

		if err := s.embedAuditEvent(

			tx, ctx,

			"", "", "", "", "",

			orgID, "",

			"metering.events.ingested", "metering_event", events[0].ID,

			"metering_event", "metering.events.ingested",

			map[string]any{

				"orgId": orgID,

				"eventCount": len(events),

				"inserted": inserted,

				"duplicates": duplicates,
			},
		); err != nil {

			return apperrors.WrapInternal(err, "ingest audit embed")

		}

		return e
	})
	if err != nil {
		slog.Error("metering ingest failed", "error", err, "organization_id", orgID, "events", len(events))
	}
	return inserted, duplicates, err
}

func (s *Service) RefreshAggregate(ctx context.Context, cmd RefreshAggregateCommand) (*AggregateDTO, error) {
	if cmd.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if cmd.Period == "" || cmd.Metric == "" {
		return nil, apperrors.NewInvalidArgument("period and metric are required")
	}
	agg, err := s.UOW.Metering().RefreshAggregate(ctx, cmd)
	if err != nil {
		return nil, err
	}
	dto := toAggregateDTO(agg)
	return &dto, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────

func validateLookup(organizationID, id string) error {
	organizationID = strings.TrimSpace(organizationID)
	id = strings.TrimSpace(id)
	if organizationID == "" {
		return apperrors.NewInvalidArgument("organizationId is required")
	}
	if id == "" {
		return apperrors.NewInvalidArgument("id is required")
	}
	if !platformuuid.IsValid(id) {
		return apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			"id must be a UUID",
			map[string]any{"field": "id", "value": id, "format": "uuid"},
		)
	}
	return nil
}

func toEventDTO(e *domain.MeteringEvent) EventDTO {
	return EventDTO{
		ID:             e.ID,
		OrganizationID: e.OrganizationID,
		EventType:      e.EventType,
		Subject:        e.Subject,
		Quantity:       e.Quantity,
		Unit:           e.Unit,
		OccurredAt:     e.OccurredAt.Format("2006-01-02T15:04:05.000Z07:00"),
		Metadata:       e.Metadata,
		CreatedAt:      e.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}
}

func toAggregateDTO(a *domain.MeteringAggregate) AggregateDTO {
	return AggregateDTO{
		ID:             a.ID,
		OrganizationID: a.OrganizationID,
		Period:         string(a.Period),
		Subject:        a.Subject,
		Metric:         a.Metric,
		Quantity:       a.Quantity,
		Unit:           a.Unit,
		CalculatedAt:   a.CalculatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
		CreatedAt:      a.CreatedAt.Format("2006-01-02T15:04:05.000Z07:00"),
	}
}
