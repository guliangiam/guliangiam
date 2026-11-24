package rebac

import (
	"context"
	"errors"
	"strconv"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

var namespaceConstraints = []string{
	"uq_relation_namespaces_org_name",
	"fk_relation_namespaces_organization_id_organizations",
}

var tupleConstraints = []string{
	"uq_relation_tuples_org_ns_sub_rel_obj",
	"fk_relation_tuples_namespace_id_relation_namespaces",
	"fk_relation_tuples_organization_id_organizations",
}

// Repository is the postgres-backed implementation of domain.Repository.
// The constructor accepts any shared.DBTX so callers can use it with either a
// plain connection or a transaction — transactions are scoped by the UOW.
type Repository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

// ─── Namespace ─────────────────────────────────────────────────────────────

func (r Repository) CreateNamespace(ctx context.Context, ns *domain.Namespace) error {
	defsBytes, err := marshalRelationDefs(ns.RelationDefs)
	if err != nil {
		return err
	}
	var scanned dbNamespace
	if err := r.db.QueryRow(ctx, queryCreateNamespace,
		ns.ID, ns.OrganizationID, ns.Name, ns.DisplayName, defsBytes, ns.Version,
	).Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.Name, &scanned.DisplayName,
		&scanned.RelationDefs, &scanned.Version, &scanned.CreatedAt, &scanned.UpdatedAt,
		&scanned.RevisionCounter,
	); err != nil {
		return pgerror.Map(err, namespaceConstraints...)
	}
	domainNS, err := scanned.toDomain()
	if err != nil {
		return err
	}
	*ns = *domainNS
	return nil
}

func (r Repository) ListNamespaces(ctx context.Context, query domain.ListNamespacesQuery) ([]*domain.Namespace, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, queryListNamespacesCount, query.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	pageSize := shared.ClampPageSize(query.PageSize)
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListNamespaces, query.OrganizationID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]*domain.Namespace, 0, pageSize)
	for rows.Next() {
		var scanned dbNamespace
		if err := rows.Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.Name, &scanned.DisplayName,
			&scanned.RelationDefs, &scanned.Version, &scanned.CreatedAt, &scanned.UpdatedAt,
			&scanned.RevisionCounter,
		); err != nil {
			return nil, 0, err
		}
		ns, err := scanned.toDomain()
		if err != nil {
			return nil, 0, err
		}
		result = append(result, ns)
	}
	return result, total, rows.Err()
}

func (r Repository) GetNamespace(ctx context.Context, id, organizationID string) (*domain.Namespace, error) {
	var scanned dbNamespace
	if err := r.db.QueryRow(ctx, queryGetNamespace, id, organizationID).Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.Name, &scanned.DisplayName,
		&scanned.RelationDefs, &scanned.Version, &scanned.CreatedAt, &scanned.UpdatedAt,
		&scanned.RevisionCounter,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("rebac namespace not found")
		}
		return nil, err
	}
	return scanned.toDomain()
}

func (r Repository) GetNamespaceByName(ctx context.Context, organizationID, name string) (*domain.Namespace, error) {
	var scanned dbNamespace
	if err := r.db.QueryRow(ctx, queryGetNamespaceByName, organizationID, name).Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.Name, &scanned.DisplayName,
		&scanned.RelationDefs, &scanned.Version, &scanned.CreatedAt, &scanned.UpdatedAt,
		&scanned.RevisionCounter,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.NewNotFound("rebac namespace not found")
		}
		return nil, err
	}
	return scanned.toDomain()
}

// ─── Tuple ──────────────────────────────────────────────────────────────────

// CreateTuple 用 CTE 原子地 bump namespace 的 revision_counter 并写入 tuple。
// zookie = bump 后的 revision_counter 值。ON CONFLICT DO NOTHING 保证幂等——
// 但幂等写入也会 bump revision_counter（OpenFGA 兼容行为）。
func (r Repository) CreateTuple(ctx context.Context, tuple *domain.Tuple) error {
	var scanned dbTuple
	err := r.db.QueryRow(ctx, queryCreateTuple,
		tuple.NamespaceID, tuple.OrganizationID,
		tuple.ID, tuple.OrganizationID,
		tuple.Subject, tuple.SubjectRelation, tuple.Relation, tuple.Object,
	).Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
		&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
		&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		err = r.db.QueryRow(ctx, queryGetTupleByKey,
			tuple.OrganizationID, tuple.NamespaceID, tuple.Subject, tuple.Relation, tuple.Object,
		).Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
			&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
			&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
		)
	}
	if err != nil {
		return pgerror.Map(err, tupleConstraints...)
	}
	*tuple = *scanned.toDomain()
	return nil
}

func (r Repository) ListTuples(ctx context.Context, query domain.ListTuplesQuery) ([]*domain.Tuple, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, queryListTuplesCount, query.OrganizationID, query.NamespaceID).Scan(&total); err != nil {
		return nil, 0, err
	}
	pageSize := shared.ClampPageSize(query.PageSize)
	offset := shared.PageOffset(query.Page, query.PageSize)
	rows, err := r.db.Query(ctx, queryListTuples, query.OrganizationID, query.NamespaceID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	result := make([]*domain.Tuple, 0, pageSize)
	for rows.Next() {
		var scanned dbTuple
		if err := rows.Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
			&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
			&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		result = append(result, scanned.toDomain())
	}
	return result, total, rows.Err()
}

// ─── Delete ────────────────────────────────────────────────────────────────

func (r Repository) DeleteNamespace(ctx context.Context, id, organizationID string) error {
	_, err := r.db.Exec(ctx, queryDeleteNamespace, id, organizationID)
	return err
}

// DeleteTuple 用 CTE 原子地删除 tuple 并 bump namespace 的 revision_counter，
// 保证删除后旧 zookie 快照不会与新写入混淆（与 CreateTuple 对称）。
// 未命中任何行时返回 NotFound（跨租户/不存在的 id 同样按 404 处理）。
func (r Repository) DeleteTuple(ctx context.Context, id, organizationID string) error {
	var namespaceID string
	err := r.db.QueryRow(ctx, queryDeleteTuple, id, organizationID).Scan(&namespaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperrors.NewNotFound("rebac tuple not found")
	}
	return pgerror.Map(err, tupleConstraints...)
}

func (r Repository) DeleteTuplesByNamespace(ctx context.Context, namespaceID, organizationID string) (int64, error) {
	tag, err := r.db.Exec(ctx, queryDeleteTuplesByNamespace, namespaceID, organizationID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ─── TupleStore (Evaluator path) ────────────────────────────────────────────

// FetchAllTuples 拉取 namespace 下的全部元组。
// atZookie 控制快照边界：
//   - ”  → 返回全部元组（最新快照）
//   - 非空 → 只返回 zookie ≤ atZookie 的元组
func (r Repository) FetchAllTuples(ctx context.Context, organizationID, namespaceID, atZookie string) ([]*domain.Tuple, error) {
	rows, err := r.db.Query(ctx, queryFetchAllTuples, organizationID, namespaceID, atZookie)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*domain.Tuple
	for rows.Next() {
		var scanned dbTuple
		if err := rows.Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
			&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
			&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, scanned.toDomain())
	}
	return result, rows.Err()
}

// GetLatestZookie 返回 namespace 当前 revision_counter 的字符串形式。
// 用于在 Check/Expand 响应里返回本次求值基于的快照 revision。
func (r Repository) GetLatestZookie(ctx context.Context, organizationID, namespaceID string) (string, error) {
	var counter int64
	if err := r.db.QueryRow(ctx, queryGetLatestZookie, namespaceID, organizationID).Scan(&counter); err != nil {
		return "", err
	}
	return strconv.FormatInt(counter, 10), nil
}

var _ domain.Repository = Repository{}
