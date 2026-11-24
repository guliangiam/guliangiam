package rebac

import (
	"context"
	"strings"
	"time"

	rebacapp "github.com/guliangiam/guliangiam/internal/app/rebac"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/transport/http/presenter"
)

// Handler 是 ReBAC 全部传输接口的聚合。它组合 Repository（CRUD）
// 和 Evaluator（求值）两个域能力，HTTP handler 层只做请求/响应转换。
// Namespace 与 tuple 写命令走 App Service，以复用 UUID、事务与审计。
type Handler struct {
	Repo    rebacapp.Repository
	App     *rebacapp.Engine
	Service *rebacapp.Service
}

func NewHandler(repo rebacapp.Repository, engine *rebacapp.Engine, service *rebacapp.Service) Handler {
	return Handler{Repo: repo, App: engine, Service: service}
}

// ─── Namespace ────────────────────────────────────────────────────────────

func (h Handler) CreateNamespace(ctx context.Context, req CreateNamespaceRequest) (*CreateNamespaceResponse, error) {
	ns, err := h.Service.CreateNamespace(ctx, rebacapp.CreateNamespaceCommand{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		DisplayName:    req.DisplayName,
		RelationDefs:   req.RelationDefs,
	})
	if err != nil {
		return nil, err
	}
	// NamespaceDTO 带 revision，但对外契约只有 id/organizationId/name/
	// displayName/relationDefs/version/createdAt/updatedAt，故逐字段映射。
	resp := NamespaceResponse{
		ID:             ns.ID,
		OrganizationID: ns.OrganizationID,
		Name:           ns.Name,
		DisplayName:    ns.DisplayName,
		RelationDefs:   ns.RelationDefs,
		Version:        ns.Version,
		CreatedAt:      ns.CreatedAt,
		UpdatedAt:      ns.UpdatedAt,
	}
	return &CreateNamespaceResponse{Namespace: resp}, nil
}

func (h Handler) ListNamespaces(ctx context.Context, req ListNamespacesRequest) (*ListNamespacesResponse, error) {
	items, total, err := h.Repo.ListNamespaces(ctx, rebacapp.ListNamespacesQuery{
		OrganizationID: req.OrganizationID,
		Page:           req.Page,
		PageSize:       req.PageSize,
	})
	if err != nil {
		return nil, err
	}
	resp := presenter.NewListResponse(make([]NamespaceResponse, 0, len(items)), req.Page, req.PageSize, total)
	for _, ns := range items {
		resp.Items = append(resp.Items, NamespaceResponse{
			ID:             ns.ID,
			OrganizationID: ns.OrganizationID,
			Name:           ns.Name,
			DisplayName:    ns.DisplayName,
			RelationDefs:   ns.RelationDefs,
			Version:        ns.Version,
			CreatedAt:      ns.CreatedAt.Format(time.RFC3339),
			UpdatedAt:      ns.UpdatedAt.Format(time.RFC3339),
		})
	}
	return &resp, nil
}

func (h Handler) GetNamespace(ctx context.Context, id, orgID string) (*NamespaceResponse, error) {
	ns, err := h.Repo.GetNamespace(ctx, id, orgID)
	if err != nil {
		return nil, err
	}
	resp := NamespaceResponse{
		ID:             ns.ID,
		OrganizationID: ns.OrganizationID,
		Name:           ns.Name,
		DisplayName:    ns.DisplayName,
		RelationDefs:   ns.RelationDefs,
		Version:        ns.Version,
		CreatedAt:      ns.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      ns.UpdatedAt.Format(time.RFC3339),
	}
	return &resp, nil
}

func (h Handler) DeleteNamespace(ctx context.Context, id, orgID string) error {
	return h.Service.DeleteNamespace(ctx, id, orgID)
}

// ─── Tuple ────────────────────────────────────────────────────────────────

func (h Handler) CreateTuple(ctx context.Context, orgID, nsID string, req CreateTupleRequest) (*TupleResponse, error) {
	tuple, err := h.Service.CreateTuple(ctx, rebacapp.CreateTupleCommand{
		OrganizationID:  orgID,
		NamespaceID:     nsID,
		Subject:         req.Subject,
		SubjectRelation: req.SubjectRelation,
		Relation:        req.Relation,
		Object:          req.Object,
	})
	if err != nil {
		return nil, err
	}
	return tupleResponse(tuple), nil
}

func (h Handler) WriteTuples(ctx context.Context, req WriteTuplesRequest) (*WriteTuplesResponse, error) {
	commands := make([]rebacapp.CreateTupleCommand, 0, len(req.Tuples))
	for _, k := range req.Tuples {
		commands = append(commands, rebacapp.CreateTupleCommand{
			OrganizationID:  req.OrganizationID,
			NamespaceID:     req.NamespaceID,
			Subject:         k.Subject,
			SubjectRelation: k.SubjectRelation,
			Relation:        k.Relation,
			Object:          k.Object,
		})
	}
	tuples, err := h.Service.CreateTuples(ctx, commands)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(tuples))
	for i, tuple := range tuples {
		ids[i] = tuple.ID
	}
	return &WriteTuplesResponse{IDs: ids}, nil
}

func (h Handler) DeleteTuple(ctx context.Context, id, orgID string) error {
	return h.Service.DeleteTuple(ctx, id, orgID)
}

func tupleResponse(tuple *rebacapp.TupleDTO) *TupleResponse {
	return &TupleResponse{
		ID:              tuple.ID,
		OrganizationID:  tuple.OrganizationID,
		NamespaceID:     tuple.NamespaceID,
		Subject:         tuple.Subject,
		SubjectRelation: tuple.SubjectRelation,
		Relation:        tuple.Relation,
		Object:          tuple.Object,
		Zookie:          tuple.Zookie,
		CreatedAt:       tuple.CreatedAt,
	}
}

func (h Handler) ListTuples(ctx context.Context, orgID, nsID string, page, pageSize int) ([]TupleResponse, int64, error) {
	items, total, err := h.Repo.ListTuples(ctx, rebacapp.ListTuplesQuery{
		OrganizationID: orgID, NamespaceID: nsID,
		Page: page, PageSize: pageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	resp := make([]TupleResponse, 0, len(items))
	for _, t := range items {
		resp = append(resp, TupleResponse{
			ID:              t.ID,
			OrganizationID:  t.OrganizationID,
			NamespaceID:     t.NamespaceID,
			Subject:         t.Subject,
			SubjectRelation: t.SubjectRelation,
			Relation:        t.Relation,
			Object:          t.Object,
			Zookie:          t.Zookie,
			CreatedAt:       t.CreatedAt.Format(time.RFC3339),
		})
	}
	return resp, total, nil
}

// ─── ADR-024 核心求值接口 ──────────────────────────────────────────────────

func (h Handler) Check(ctx context.Context, req CheckRequest) (*CheckResponse, error) {
	if err := h.validateEvaluationTarget(ctx, req.OrganizationID, req.NamespaceID, req.Relation, req.Subject); err != nil {
		return nil, err
	}
	res, err := h.App.Check(ctx, rebacapp.CheckRequest{
		OrganizationID: req.OrganizationID,
		NamespaceID:    req.NamespaceID,
		Tuple: rebacapp.TupleKey{
			Subject:  req.Subject,
			Relation: req.Relation,
			Object:   req.Object,
		},
		DepthLimit: req.DepthLimit,
		AtZookie:   req.Zookie,
	})
	if err != nil {
		return nil, err
	}
	return &CheckResponse{Allowed: res.Allowed, Trace: res.Trace, Zookie: res.Zookie}, nil
}

func (h Handler) Expand(ctx context.Context, req ExpandRequest) (*ExpandResponse, error) {
	if err := h.validateEvaluationTarget(ctx, req.OrganizationID, req.NamespaceID, req.Relation, ""); err != nil {
		return nil, err
	}
	res, err := h.App.Expand(ctx, rebacapp.ExpandRequest{
		OrganizationID: req.OrganizationID,
		NamespaceID:    req.NamespaceID,
		Relation:       req.Relation,
		Object:         req.Object,
		DepthLimit:     req.DepthLimit,
		AtZookie:       req.Zookie,
	})
	if err != nil {
		return nil, err
	}
	return &ExpandResponse{Root: convertExpandNode(res.Root), Zookie: res.Zookie}, nil
}

func convertExpandNode(n rebacapp.ExpandNode) ExpandNodeResponse {
	node := ExpandNodeResponse{Subject: n.Subject, Expanded: n.Expanded, SubjectRel: n.SubjectRel}
	for _, c := range n.Children {
		node.Children = append(node.Children, convertExpandNode(c))
	}
	return node
}

// maxBatchCheckItems caps a single BatchCheck request (BE-D-18): without a
// ceiling one request could fan out into unbounded evaluation work.
const maxBatchCheckItems = 100

func (h Handler) BatchCheck(ctx context.Context, req BatchCheckRequest) (*BatchCheckResponse, error) {
	if len(req.Items) > maxBatchCheckItems {
		return nil, apperrors.NewWithMeta(apperrors.CodeInvalidArgument, "too many items in batch check", map[string]any{"field": "items", "max": maxBatchCheckItems, "got": len(req.Items)})
	}
	// 同一 namespace 只取一次（BE-D-18）：逐项 GetNamespace 是 N+1。
	namespace, err := h.Repo.GetNamespace(ctx, req.NamespaceID, req.OrganizationID)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("namespace is not registered")
	}
	for _, item := range req.Items {
		if err := validateAgainstNamespace(namespace.RelationDefs, item.Relation, item.Subject); err != nil {
			return nil, err
		}
	}
	keys := make([]rebacapp.TupleKey, len(req.Items))
	for i, item := range req.Items {
		keys[i] = rebacapp.TupleKey{Subject: item.Subject, Relation: item.Relation, Object: item.Object}
	}
	results, err := h.App.BatchCheck(ctx, req.OrganizationID, req.NamespaceID, keys, req.Zookie)
	if err != nil {
		return nil, err
	}
	out := make([]BatchCheckItem, len(results))
	for i, r := range results {
		out[i] = BatchCheckItem{
			Subject: req.Items[i].Subject, Relation: req.Items[i].Relation,
			Object: req.Items[i].Object, Allowed: r.Allowed,
		}
	}
	// 所有 item 共享同一个 zookie（BatchCheck 单次快照）
	var zookie string
	if len(results) > 0 {
		zookie = results[0].Zookie
	}
	return &BatchCheckResponse{Results: out, Zookie: zookie}, nil
}

// validateEvaluationTarget rejects unregistered namespaces, relations and
// subject types before evaluation (AC-REBAC-001: 400 invalid_argument).
func (h Handler) validateEvaluationTarget(ctx context.Context, organizationID, namespaceID, relation, subject string) error {
	namespace, err := h.Repo.GetNamespace(ctx, namespaceID, organizationID)
	if err != nil {
		return apperrors.NewInvalidArgument("namespace is not registered")
	}
	return validateAgainstNamespace(namespace.RelationDefs, relation, subject)
}

func validateAgainstNamespace(defs map[string]any, relation, subject string) error {
	relations := declaredVocabulary(defs, "roles", "permissions")
	if len(relations) > 0 && !relations[relation] {
		return apperrors.NewInvalidArgument("relation is not registered in the namespace")
	}
	subjectTypes := declaredVocabulary(defs, "subjectTypes")
	if len(subjectTypes) > 0 && subject != "" {
		subjectType := subject
		if idx := strings.Index(subject, ":"); idx > 0 {
			subjectType = subject[:idx]
		}
		if !subjectTypes[subjectType] {
			return apperrors.NewInvalidArgument("subject type is not registered in the namespace")
		}
	}
	return nil
}

func declaredVocabulary(defs map[string]any, keys ...string) map[string]bool {
	out := map[string]bool{}
	for _, key := range keys {
		switch typed := defs[key].(type) {
		case []string:
			for _, item := range typed {
				out[item] = true
			}
		case []any:
			for _, item := range typed {
				if text, ok := item.(string); ok {
					out[text] = true
				}
			}
		}
	}
	return out
}
