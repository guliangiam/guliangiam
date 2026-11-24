package mysql

import (
	"context"
	"database/sql"
	"strconv"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// Repository 是 MySQL 8.4+ 实现的 domain.Repository。
// 接受 *sql.DB 或 *sql.Tx（通过 DBTX 接口），事务由调用方控制。
type Repository struct {
	db         DBTX
	txBeginner txBeginner // 可选；CreateTuple 用事务包装 bump + insert
}

// NewRepository 用 *sql.DB 构造 Repository（包含事务能力）。
func NewRepository(db *sql.DB) Repository {
	return Repository{db: db, txBeginner: db}
}

// NewRepositoryWithDBTX 用任意 DBTX 构造（适合传入 *sql.Tx 复用外层事务）。
func NewRepositoryWithDBTX(db DBTX) Repository {
	return Repository{db: db}
}

// ─── Namespace ─────────────────────────────────────────────────────────────

func (r Repository) CreateNamespace(ctx context.Context, ns *domain.Namespace) error {
	defsBytes, err := marshalRelationDefs(ns.RelationDefs)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, queryCreateNamespace,
		ns.ID, ns.OrganizationID, ns.Name, ns.DisplayName, defsBytes, ns.Version,
	)
	if err != nil {
		return err
	}
	// MySQL 无 RETURNING，创建后需要回读完整行
	created, err := r.GetNamespace(ctx, ns.ID, ns.OrganizationID)
	if err != nil {
		return err
	}
	*ns = *created
	return nil
}

func (r Repository) ListNamespaces(ctx context.Context, q domain.ListNamespacesQuery) ([]*domain.Namespace, int64, error) {
	limit, offset := rebacPageWindow(q.Page, q.PageSize, 20)
	var total int64
	if err := r.db.QueryRowContext(ctx, queryListNamespacesCount, q.OrganizationID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, queryListNamespaces, q.OrganizationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Namespace
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
		out = append(out, ns)
	}
	return out, total, rows.Err()
}

func (r Repository) GetNamespace(ctx context.Context, id, orgID string) (*domain.Namespace, error) {
	var scanned dbNamespace
	if err := r.db.QueryRowContext(ctx, queryGetNamespace, id, orgID).Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.Name, &scanned.DisplayName,
		&scanned.RelationDefs, &scanned.Version, &scanned.CreatedAt, &scanned.UpdatedAt,
		&scanned.RevisionCounter,
	); err != nil {
		return nil, err
	}
	return scanned.toDomain()
}

func (r Repository) GetNamespaceByName(ctx context.Context, organizationID, name string) (*domain.Namespace, error) {
	var scanned dbNamespace
	if err := r.db.QueryRowContext(ctx, queryGetNamespaceByName, organizationID, name).Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.Name, &scanned.DisplayName,
		&scanned.RelationDefs, &scanned.Version, &scanned.CreatedAt, &scanned.UpdatedAt,
		&scanned.RevisionCounter,
	); err != nil {
		return nil, err
	}
	return scanned.toDomain()
}

func (r Repository) DeleteNamespace(ctx context.Context, id, orgID string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteNamespace, id, orgID)
	return err
}

// ─── Tuple ──────────────────────────────────────────────────────────────────

// CreateTuple 在事务内执行：UPDATE revision_counter bump → SELECT new value → INSERT IGNORE。
// 即使 INSERT IGNORE 因为唯一约束命中而忽略写入，bump 仍然发生 —— OpenFGA 兼容行为。
func (r Repository) CreateTuple(ctx context.Context, tuple *domain.Tuple) error {
	if r.txBeginner != nil {
		return r.createTupleWithTx(ctx, tuple)
	}
	return r.createTuplePlain(ctx, tuple)
}

// createTupleWithTx 包在事务里的 bump + insert。
func (r Repository) createTupleWithTx(ctx context.Context, tuple *domain.Tuple) error {
	tx, err := r.txBeginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := r.bumpRevisionInTx(ctx, tx, tuple.NamespaceID, tuple.OrganizationID); err != nil {
		return err
	}

	var newCounter int64
	if err := tx.QueryRowContext(ctx, queryReadNewRevision, tuple.NamespaceID, tuple.OrganizationID).Scan(&newCounter); err != nil {
		return err
	}

	zookie := strconv.FormatInt(newCounter, 10)
	sr := nullableString(tuple.SubjectRelation)
	if _, err := tx.ExecContext(ctx, queryInsertTuple,
		tuple.ID, tuple.OrganizationID, tuple.NamespaceID,
		tuple.Subject, sr, tuple.Relation, tuple.Object, zookie,
	); err != nil {
		return err
	}
	if err := scanTupleByKey(ctx, tx, tuple); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	return nil
}

// createTuplePlain 没有事务包装时的降级路径（当 Repository 被传入 *sql.Tx 时）。
// bump + select + insert 不原子 —— 但在已有事务里调用通常安全。
func (r Repository) createTuplePlain(ctx context.Context, tuple *domain.Tuple) error {
	if err := r.bumpRevisionInTx(ctx, r.db, tuple.NamespaceID, tuple.OrganizationID); err != nil {
		return err
	}
	var newCounter int64
	if err := r.db.QueryRowContext(ctx, queryReadNewRevision, tuple.NamespaceID, tuple.OrganizationID).Scan(&newCounter); err != nil {
		return err
	}
	zookie := strconv.FormatInt(newCounter, 10)
	sr := nullableString(tuple.SubjectRelation)
	if _, err := r.db.ExecContext(ctx, queryInsertTuple,
		tuple.ID, tuple.OrganizationID, tuple.NamespaceID,
		tuple.Subject, sr, tuple.Relation, tuple.Object, zookie,
	); err != nil {
		return err
	}
	return scanTupleByKey(ctx, r.db, tuple)
}

func scanTupleByKey(ctx context.Context, db DBTX, tuple *domain.Tuple) error {
	var scanned dbTuple
	if err := db.QueryRowContext(ctx, queryGetTupleByKey,
		tuple.OrganizationID, tuple.NamespaceID, tuple.Subject, tuple.Relation, tuple.Object,
	).Scan(
		&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
		&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
		&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
	); err != nil {
		return err
	}
	*tuple = *scanned.toDomain()
	return nil
}

func (r Repository) bumpRevisionInTx(ctx context.Context, dtx DBTX, nsID, orgID string) error {
	_, err := dtx.ExecContext(ctx, queryBumpRevision, nsID, orgID)
	return err
}

func (r Repository) ListTuples(ctx context.Context, q domain.ListTuplesQuery) ([]*domain.Tuple, int64, error) {
	limit, offset := rebacPageWindow(q.Page, q.PageSize, 50)
	var total int64
	if err := r.db.QueryRowContext(ctx, queryListTuplesCount, q.OrganizationID, q.NamespaceID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx, queryListTuples, q.OrganizationID, q.NamespaceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []*domain.Tuple
	for rows.Next() {
		var scanned dbTuple
		if err := rows.Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
			&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
			&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, scanned.toDomain())
	}
	return out, total, rows.Err()
}

func (r Repository) DeleteTuple(ctx context.Context, id, orgID string) error {
	_, err := r.db.ExecContext(ctx, queryDeleteTuple, id, orgID)
	return err
}

func (r Repository) DeleteTuplesByNamespace(ctx context.Context, nsID, orgID string) (int64, error) {
	res, err := r.db.ExecContext(ctx, queryDeleteTuplesByNamespace, nsID, orgID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ─── TupleStore (Evaluator) ─────────────────────────────────────────────────

// FetchAllTuples 支持 atZookie 快照边界 —— 与 PG 完全一致的语义。
//
//	atZookie='' → 返回全部（最新）
//	atZookie 非空 → zookie <= atZookie
func (r Repository) FetchAllTuples(ctx context.Context, orgID, nsID, atZookie string) ([]*domain.Tuple, error) {
	rows, err := r.db.QueryContext(ctx, queryFetchAllTuples, orgID, nsID, atZookie, atZookie)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.Tuple
	for rows.Next() {
		var scanned dbTuple
		if err := rows.Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
			&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
			&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, scanned.toDomain())
	}
	return out, rows.Err()
}

func (r Repository) GetLatestZookie(ctx context.Context, orgID, nsID string) (string, error) {
	var counter int64
	if err := r.db.QueryRowContext(ctx, queryGetLatestZookie, nsID, orgID).Scan(&counter); err != nil {
		return "", err
	}
	return strconv.FormatInt(counter, 10), nil
}

var _ domain.Repository = Repository{}

// rebacPageWindow keeps the per-list default size (namespaces 20, tuples 50)
// while routing the arithmetic through mysqlshared, where the page is capped
// before multiplying and the per-list default is applied only when pageSize<=0.
func rebacPageWindow(page, pageSize, defaultSize int) (limit, offset int) {
	if pageSize <= 0 {
		pageSize = defaultSize
	}
	limit = mysqlshared.ClampPageSize(pageSize)
	return limit, mysqlshared.PageOffset(page, limit)
}

// nullableString 把 *string 转成 sql.NullString（MySQL 驱动要求）。
func nullableString(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}
