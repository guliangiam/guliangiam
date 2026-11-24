package rebac

import (
	"context"
	"strings"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/platform/clock"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformuuid "github.com/guliangiam/guliangiam/internal/platform/uuid"
)

// Service implements the ReBAC use cases. It delegates persistence to the
// UoW so each write command is atomic — namespace creation and tuple creation
// can be composed into a single transaction by callers (e.g. bootstrap code
// that pre-seeds a namespace with its initial relation vocabulary).
type (
	Repository    = domain.Repository
	CheckRequest  = domain.CheckRequest
	ExpandRequest = domain.ExpandRequest
	ExpandNode    = domain.ExpandNode
	TupleKey      = domain.TupleKey
	Tuple         = domain.Tuple
	Namespace     = domain.Namespace
)

type Service struct {
	Clock clock.Clock
	UUID  platformuuid.Generator
	UOW   appuow.AuthzUoW
}

func NewService(c clock.Clock, u platformuuid.Generator, uow appuow.UnitOfWork) Service {
	return Service{Clock: c, UUID: u, UOW: uow}
}

// ─── Commands ───────────────────────────────────────────────────────────────

type CreateNamespaceCommand = domain.CreateNamespaceCommand
type CreateTupleCommand = domain.CreateTupleCommand

// ─── Queries ─────────────────────────────────────────────────────────────────

type ListNamespacesQuery = domain.ListNamespacesQuery
type ListTuplesQuery = domain.ListTuplesQuery

// ─── DTOs (wire format for transport) ────────────────────────────────────────

type NamespaceDTO struct {
	ID             string         `json:"id"`
	OrganizationID string         `json:"organizationId"`
	Name           string         `json:"name"`
	DisplayName    string         `json:"displayName"`
	RelationDefs   map[string]any `json:"relationDefs"`
	Version        int            `json:"version"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
	Revision       int64          `json:"revision"`
}

type TupleDTO struct {
	ID              string  `json:"id"`
	OrganizationID  string  `json:"organizationId"`
	NamespaceID     string  `json:"namespaceId"`
	Subject         string  `json:"subject"`
	SubjectRelation *string `json:"subjectRelation,omitempty"`
	Relation        string  `json:"relation"`
	Object          string  `json:"object"`
	Zookie          string  `json:"zookie"`
	CreatedAt       string  `json:"createdAt"`
}

// ─── Namespace ───────────────────────────────────────────────────────────────

func (s *Service) CreateNamespace(ctx context.Context, cmd CreateNamespaceCommand) (*NamespaceDTO, error) {
	if strings.TrimSpace(cmd.OrganizationID) == "" {
		return nil, apperrors.NewInvalidArgument("organizationId is required")
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return nil, apperrors.NewInvalidArgument("name is required")
	}
	if !hasDeclaredRelation(cmd.RelationDefs) {
		// 语义规则不满足（请求已解析）→ 422（DEF-04）。
		return nil, apperrors.NewUnprocessableEntity("relationDefs must declare at least one role or permission")
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, err
	}

	ns := &domain.Namespace{
		ID:             id,
		OrganizationID: cmd.OrganizationID,
		Name:           cmd.Name,
		DisplayName:    firstNonEmpty(cmd.DisplayName, cmd.Name),
		RelationDefs:   cmd.RelationDefs,
		Version:        1,
	}

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Rebac().CreateNamespace(ctx, ns); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			ns.OrganizationID,
			"rebac.namespace.created",
			"namespace",
			ns.ID,
			"namespace",
			"rebac.namespace.created",
			map[string]any{
				"namespaceId": ns.ID,
				fieldName:     ns.Name,
				fieldOrgID:    ns.OrganizationID,
			},
		); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return toNamespaceDTO(ns), nil
}

func (s *Service) ListNamespaces(ctx context.Context, q ListNamespacesQuery) ([]*NamespaceDTO, int64, error) {
	if strings.TrimSpace(q.OrganizationID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId is required")
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	items, total, err := s.UOW.Rebac().ListNamespaces(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	return mapNamespaceList(items), total, nil
}

func (s *Service) GetNamespace(ctx context.Context, id, organizationID string) (*NamespaceDTO, error) {
	ns, err := s.UOW.Rebac().GetNamespace(ctx, id, organizationID)
	if err != nil {
		return nil, err
	}
	return toNamespaceDTO(ns), nil
}

// ─── Tuple ───────────────────────────────────────────────────────────────────

func (s *Service) CreateTuple(ctx context.Context, cmd CreateTupleCommand) (*TupleDTO, error) {
	tuples, err := s.CreateTuples(ctx, []CreateTupleCommand{cmd})
	if err != nil {
		return nil, err
	}
	return tuples[0], nil
}

// maxCreateTuplesBatch caps one CreateTuples call (BE-D-18), matching the
// BatchCheck ceiling so a single request cannot fan out into unbounded writes.
const maxCreateTuplesBatch = 100

func (s *Service) CreateTuples(ctx context.Context, commands []CreateTupleCommand) ([]*TupleDTO, error) {
	if len(commands) > maxCreateTuplesBatch {
		return nil, apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "too many tuples in batch",
			map[string]any{"field": "tuples", "max": maxCreateTuplesBatch, "got": len(commands)})
	}
	tuples := make([]*domain.Tuple, len(commands))
	for i, cmd := range commands {
		if strings.TrimSpace(cmd.OrganizationID) == "" {
			return nil, apperrors.NewInvalidArgument("organizationId is required")
		}
		if strings.TrimSpace(cmd.NamespaceID) == "" {
			return nil, apperrors.NewInvalidArgument("namespaceId is required")
		}
		if strings.TrimSpace(cmd.Subject) == "" || strings.TrimSpace(cmd.Relation) == "" || strings.TrimSpace(cmd.Object) == "" {
			return nil, apperrors.NewInvalidArgument("subject, relation and object are required")
		}
		id, err := s.UUID.NewString()
		if err != nil {
			return nil, err
		}
		tuples[i] = &domain.Tuple{
			ID:              id,
			OrganizationID:  cmd.OrganizationID,
			NamespaceID:     cmd.NamespaceID,
			Subject:         cmd.Subject,
			SubjectRelation: cmd.SubjectRelation,
			Relation:        cmd.Relation,
			Object:          cmd.Object,
		}
	}

	if err := s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 按 (org, namespace) 分组批量写入：每组一次 bump + 一条多行 INSERT。
		groups := make(map[string][]*domain.Tuple)
		order := make([]string, 0)
		for _, tuple := range tuples {
			key := tuple.OrganizationID + "\x00" + tuple.NamespaceID
			if _, ok := groups[key]; !ok {
				order = append(order, key)
			}
			groups[key] = append(groups[key], tuple)
		}
		for _, key := range order {
			if err := tx.Rebac().CreateTuplesBatch(ctx, groups[key]); err != nil {
				return err
			}
		}
		for _, tuple := range tuples {
			if err := s.embedAuditEvent(
				tx,
				ctx,
				tuple.OrganizationID,
				"rebac.tuple.created",
				"tuple",
				tuple.ID,
				"tuple",
				"rebac.tuple.created",
				map[string]any{
					"tupleId":  tuple.ID,
					fieldName:  tuple.Subject,
					fieldOrgID: tuple.OrganizationID,
				},
			); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return mapTupleList(tuples), nil
}

func (s *Service) ListTuples(ctx context.Context, q ListTuplesQuery) ([]*TupleDTO, int64, error) {
	if strings.TrimSpace(q.OrganizationID) == "" || strings.TrimSpace(q.NamespaceID) == "" {
		return nil, 0, apperrors.NewInvalidArgument("organizationId and namespaceId are required")
	}
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 50
	}
	items, total, err := s.UOW.Rebac().ListTuples(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	return mapTupleList(items), total, nil
}

// ─── DeleteNamespace ────────────────────────────────────────────────────────

// DeleteNamespace cascades tuples first, then the namespace itself, inside a
// single transaction so a mid-flight failure (e.g. concurrent tuple write)
// cannot leave the database half-cleaned.
func (s *Service) DeleteNamespace(ctx context.Context, id, organizationID string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("id and organizationId are required")
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		// 级联删除关联的所有 tuples
		if _, err := tx.Rebac().DeleteTuplesByNamespace(ctx, id, organizationID); err != nil {
			return err
		}
		// 删 namespace 本体
		if err := tx.Rebac().DeleteNamespace(ctx, id, organizationID); err != nil {
			return err
		}

		if err := s.embedAuditEvent(
			tx,
			ctx,
			organizationID,
			"rebac.namespace.deleted",
			"namespace",
			id,
			"namespace",
			"rebac.namespace.deleted",
			map[string]any{
				"namespaceId": id,
				fieldName:     nil,
				fieldOrgID:    organizationID,
			},
		); err != nil {
			return err
		}
		return nil
	})
}

// ─── DeleteTuple ─────────────────────────────────────────────────────────────

func (s *Service) DeleteTuple(ctx context.Context, id, organizationID string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(organizationID) == "" {
		return apperrors.NewInvalidArgument("id and organizationId are required")
	}

	return s.UOW.WithinTransaction(ctx, func(ctx context.Context, tx appuow.UnitOfWork) error {
		if err := tx.Rebac().DeleteTuple(ctx, id, organizationID); err != nil {
			return err
		}
		if err := s.embedAuditEvent(
			tx,
			ctx,
			organizationID,
			"rebac.tuple.deleted",
			"tuple",
			id,
			"tuple",
			"rebac.tuple.deleted",
			map[string]any{
				"tupleId":  id,
				fieldName:  nil,
				fieldOrgID: organizationID,
			},
		); err != nil {
			return err
		}
		return nil
	})
}

// ─── Mappers ─────────────────────────────────────────────────────────────────

func toNamespaceDTO(ns *domain.Namespace) *NamespaceDTO {
	return &NamespaceDTO{
		ID:             ns.ID,
		OrganizationID: ns.OrganizationID,
		Name:           ns.Name,
		DisplayName:    ns.DisplayName,
		RelationDefs:   ns.RelationDefs,
		Version:        ns.Version,
		CreatedAt:      ns.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      ns.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		Revision:       ns.Revision,
	}
}

func mapNamespaceList(items []*domain.Namespace) []*NamespaceDTO {
	out := make([]*NamespaceDTO, 0, len(items))
	for _, ns := range items {
		out = append(out, toNamespaceDTO(ns))
	}
	return out
}

func toTupleDTO(t *domain.Tuple) *TupleDTO {
	return &TupleDTO{
		ID:              t.ID,
		OrganizationID:  t.OrganizationID,
		NamespaceID:     t.NamespaceID,
		Subject:         t.Subject,
		SubjectRelation: t.SubjectRelation,
		Relation:        t.Relation,
		Object:          t.Object,
		Zookie:          t.Zookie,
		CreatedAt:       t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func mapTupleList(items []*domain.Tuple) []*TupleDTO {
	out := make([]*TupleDTO, 0, len(items))
	for _, t := range items {
		out = append(out, toTupleDTO(t))
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// hasDeclaredRelation reports whether the namespace declares a relation
// vocabulary. Check/Expand/BatchCheck reject relations outside this vocabulary,
// so a namespace without one could never be evaluated safely.
func hasDeclaredRelation(defs map[string]any) bool {
	for _, key := range []string{"roles", "permissions"} {
		if len(stringList(defs[key])) > 0 {
			return true
		}
	}
	return false
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				out = append(out, strings.TrimSpace(text))
			}
		}
		return out
	default:
		return nil
	}
}
