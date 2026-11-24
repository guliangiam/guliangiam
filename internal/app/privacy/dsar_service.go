package privacy

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/privacy"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// AuditEvent 是应用层交给基础设施的审计载荷；实现必须在 tx 内写 records +
// outbox，保证状态变更与审计同事务（design/privacy §7）。
type AuditEvent struct {
	OrganizationID string
	ActorID        string
	Action         string
	ResourceType   string
	ResourceID     string
	AggregateType  string
	EventType      string
	Payload        map[string]any
}

// Tx 是一次事务内的能力集合：仓储绑定同一事务，审计也在同一事务写入。
type Tx interface {
	Privacy() domain.DsarRepository
	Consents() domain.ConsentRepository
	EmbedAudit(ctx context.Context, event AuditEvent) error
}

// TransactionRunner 负责开启事务并把 Tx 交给用例。
type TransactionRunner interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}

// Service 编排 DSAR 用例：创建、状态迁移、读取与超期巡检。归属与跨租户由
// 仓储的 ForOrganization 读取决定：缺失与越权一律 NotFound（设计 §6，防枚举）。
type Service struct {
	Clock  clock.Clock
	UUID   platformuuid.Generator
	Runner TransactionRunner
}

func NewService(ck clock.Clock, generator platformuuid.Generator, runner TransactionRunner) Service {
	return Service{Clock: ck, UUID: generator, Runner: runner}
}

type CreateDsarRequestCommand struct {
	OrganizationID string
	SubjectType    string
	SubjectID      string
	RequestType    string
	ActorID        string
}

type TransitionDsarRequestCommand struct {
	OrganizationID string
	ID             string
	Target         string
	Reason         string
	ActorID        string
	Revision       int64
}

// Create 登记请求、写 created 事件与审计；SLA 自创建时刻起算。
func (s Service) Create(ctx context.Context, command CreateDsarRequestCommand) (*domain.DsarRequest, error) {
	if command.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate dsar id")
	}
	eventID, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "generate dsar event id")
	}
	now := s.Clock.Now()
	var createdBy *string
	if command.ActorID != "" {
		actor := command.ActorID
		createdBy = &actor
	}
	request, err := domain.NewDsarRequest(id, command.OrganizationID, command.SubjectType, command.SubjectID, command.RequestType, createdBy, now)
	if err != nil {
		return nil, err
	}
	event, err := domain.NewDsarEvent(eventID, command.OrganizationID, request.ID, "created", createdBy, nil, now)
	if err != nil {
		return nil, err
	}
	err = s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		if err := tx.Privacy().Create(ctx, request, event); err != nil {
			return err
		}
		return tx.EmbedAudit(ctx, AuditEvent{
			OrganizationID: request.OrganizationID,
			ActorID:        command.ActorID,
			Action:         "dsar.request.created",
			ResourceType:   "dsar_request",
			ResourceID:     request.ID,
			AggregateType:  "DsarRequest",
			EventType:      "dsar.request.created",
			Payload:        map[string]any{"requestType": request.RequestType, "subjectType": request.SubjectType},
		})
	})
	if err != nil {
		return nil, err
	}
	return request, nil
}

// Transition 执行一次状态迁移，追加不可变事件与审计；乐观锁由 Revision 保证。
func (s Service) Transition(ctx context.Context, command TransitionDsarRequestCommand) (*domain.DsarRequest, error) {
	if command.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if command.ID == "" {
		return nil, apperrors.NewInvalidArgument("id is required")
	}
	var request *domain.DsarRequest
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		current, err := tx.Privacy().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}
		if command.Revision > 0 && current.Revision != command.Revision {
			return apperrors.NewConflict("dsar request revision mismatch")
		}
		now := s.Clock.Now()
		expectedRevision := current.Revision
		fromState := current.State
		if err := current.TransitionTo(command.Target, command.Reason, now); err != nil {
			return err
		}
		eventID, err := s.UUID.NewString()
		if err != nil {
			return apperrors.WrapInternalf(err, "generate dsar event id")
		}
		var actorID *string
		if command.ActorID != "" {
			actor := command.ActorID
			actorID = &actor
		}
		event, err := domain.NewDsarEvent(eventID, command.OrganizationID, current.ID, command.Target, actorID,
			map[string]any{"from": fromState, "to": current.State}, now)
		if err != nil {
			return err
		}
		if err := tx.Privacy().UpdateWithEvent(ctx, current, event, expectedRevision); err != nil {
			return err
		}
		request = current
		return tx.EmbedAudit(ctx, AuditEvent{
			OrganizationID: current.OrganizationID,
			ActorID:        command.ActorID,
			Action:         "dsar.request.updated",
			ResourceType:   "dsar_request",
			ResourceID:     current.ID,
			AggregateType:  "DsarRequest",
			EventType:      "dsar.request." + current.State,
			Payload:        map[string]any{"from": fromState, "to": current.State, "reason": command.Reason},
		})
	})
	if err != nil {
		return nil, err
	}
	return request, nil
}

// Get 读取单条请求；越权与缺失统一 NotFound（设计 §6，防枚举）。
func (s Service) Get(ctx context.Context, organizationID, id string) (*domain.DsarRequest, error) {
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	var request *domain.DsarRequest
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		found, err := tx.Privacy().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		request = found
		return nil
	})
	if err != nil {
		return nil, err
	}
	return request, nil
}

// List 返回租户内请求分页；limit 归一化到 [1,200]，offset 不为负。
func (s Service) List(ctx context.Context, organizationID string, limit, offset int) ([]*domain.DsarRequest, int64, error) {
	if organizationID == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId is required")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	var requests []*domain.DsarRequest
	var total int64
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		items, count, err := tx.Privacy().ListByOrganization(ctx, organizationID, limit, offset)
		if err != nil {
			return err
		}
		requests, total = items, count
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return requests, total, nil
}

// ListOverdue 返回已超 SLA 且未终结的请求（用于告警与合规指标）。
func (s Service) ListOverdue(ctx context.Context, now time.Time, limit int) ([]*domain.DsarRequest, error) {
	if limit <= 0 {
		limit = 100
	}
	var requests []*domain.DsarRequest
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		items, err := tx.Privacy().ListOverdue(ctx, now, limit)
		if err != nil {
			return err
		}
		requests = items
		return nil
	})
	if err != nil {
		return nil, err
	}
	return requests, nil
}

// Export assembles the evidence package for a fulfilled access/portability DSAR.
// Subject records and the audit event are read/written inside one transaction so
// the export manifest can never reference data outside the tenant boundary.
func (s Service) Export(ctx context.Context, organizationID, id, actorID string) (*domain.EvidencePackage, error) {
	if organizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if id == "" {
		return nil, apperrors.NewInvalidArgument("id is required")
	}
	var pkg *domain.EvidencePackage
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		request, err := tx.Privacy().GetByIDForOrganization(ctx, id, organizationID)
		if err != nil {
			return err
		}
		if request.RequestType != "access" && request.RequestType != "portability" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"only access or portability requests can be exported",
				map[string]any{"field": "requestType", "value": request.RequestType})
		}
		if request.State != "fulfilled" {
			return apperrors.NewWithMeta(apperrors.CodeConflict,
				"evidence package is available only after the request is fulfilled",
				map[string]any{"field": "state", "value": request.State})
		}
		if request.SubjectType != "user" {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"only user subjects are supported for export",
				map[string]any{"field": "subjectType", "value": request.SubjectType})
		}
		collector, ok := tx.Privacy().(domain.SubjectDataCollector)
		if !ok {
			return apperrors.NewInternal("privacy export collector is not configured")
		}
		records, region, err := collector.CollectSubjectData(ctx, organizationID, request.SubjectType, request.SubjectID)
		if err != nil {
			return err
		}
		pkg, err = domain.BuildEvidencePackage(records, region, s.Clock.Now())
		if err != nil {
			return err
		}
		return tx.EmbedAudit(ctx, AuditEvent{
			OrganizationID: organizationID,
			ActorID:        actorID,
			Action:         "privacy.data_exported",
			ResourceType:   "privacy_export",
			ResourceID:     request.ID,
			AggregateType:  "DsarRequest",
			EventType:      "privacy.data_exported",
			Payload: map[string]any{
				"requestId":   request.ID,
				"recordCount": pkg.LineCount,
				"checksum":    pkg.Checksum,
				"region":      pkg.Region,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return pkg, nil
}

// ReviewErasureCommand is the compliance review input for an erase DSAR:
// targets are proposed categories, retentions are the legal retention rules.
type ErasureTarget struct {
	Category string
	TargetID string
}

type RetentionRule struct {
	Category   string
	LegalBasis string
	Blocks     bool
}

type ReviewErasureCommand struct {
	OrganizationID string
	ID             string
	ActorID        string
	Targets        []ErasureTarget
	Retentions     []RetentionRule
}

// ReviewErasure applies the two-track erasure adjudication (design/privacy §4):
// legal retention blocks the request with 409; otherwise it returns the exact
// erase/retain scope and records the review in the same transaction. Actual
// per-table deletion remains an operational follow-up.
func (s Service) ReviewErasure(ctx context.Context, command ReviewErasureCommand) (*domain.EraseScope, error) {
	if command.OrganizationID == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if command.ID == "" {
		return nil, apperrors.NewInvalidArgument("id is required")
	}
	var scope *domain.EraseScope
	err := s.Runner.WithinTransaction(ctx, func(ctx context.Context, tx Tx) error {
		request, err := tx.Privacy().GetByIDForOrganization(ctx, command.ID, command.OrganizationID)
		if err != nil {
			return err
		}
		if request.RequestType != domain.RequestTypeErase {
			return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity,
				"only erase requests can be reviewed",
				map[string]any{"field": "requestType", "value": request.RequestType})
		}
		if request.State != domain.StateInProgress {
			return apperrors.NewWithMeta(apperrors.CodeConflict,
				"erasure review requires an in_progress request",
				map[string]any{"field": "state", "value": request.State})
		}
		targets := make([]domain.EraseTarget, 0, len(command.Targets))
		for _, target := range command.Targets {
			targets = append(targets, domain.EraseTarget{Category: target.Category, TargetID: target.TargetID})
		}
		retentions := make([]domain.RetentionItem, 0, len(command.Retentions))
		for _, retention := range command.Retentions {
			retentions = append(retentions, domain.RetentionItem{
				Category: retention.Category, LegalBasis: retention.LegalBasis, Blocks: retention.Blocks,
			})
		}
		planned, err := domain.PlanErase(targets, retentions)
		if err != nil {
			return err
		}
		scope = &planned
		retained := make([]string, 0, len(planned.Retain))
		for _, item := range planned.Retain {
			retained = append(retained, item.Category)
		}
		return tx.EmbedAudit(ctx, AuditEvent{
			OrganizationID: command.OrganizationID,
			ActorID:        command.ActorID,
			Action:         "privacy.erasure.reviewed",
			ResourceType:   "dsar_request",
			ResourceID:     request.ID,
			AggregateType:  "DsarRequest",
			EventType:      "privacy.erasure.reviewed",
			Payload: map[string]any{
				"requestId":   request.ID,
				"eraseCount":  len(planned.Erase),
				"retainCount": len(planned.Retain),
				"retained":    retained,
				"subjectType": request.SubjectType,
				"requestType": request.RequestType,
			},
		})
	})
	if err != nil {
		return nil, err
	}
	return scope, nil
}
