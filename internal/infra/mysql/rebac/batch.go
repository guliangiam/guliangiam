package mysql

import (
	"context"
	"strconv"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CreateTuplesBatch 是 CreateTuple 的批量镜像（BE-D-18）：一次 bump +
// 单条多行 INSERT IGNORE + 逐键回读。MySQL 无 RETURNING，回读同时覆盖
// 新插入与冲突保留行，语义与单条路径一致。
func (r Repository) CreateTuplesBatch(ctx context.Context, tuples []*domain.Tuple) error {
	if len(tuples) == 0 {
		return nil
	}
	orgID, nsID := tuples[0].OrganizationID, tuples[0].NamespaceID
	for _, t := range tuples[1:] {
		if t.OrganizationID != orgID || t.NamespaceID != nsID {
			return apperrors.NewInvalidArgument("batch tuples must share one organization and namespace")
		}
	}
	if r.txBeginner != nil {
		tx, err := r.txBeginner.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		if err := r.createTuplesBatchOn(ctx, tx, tuples); err != nil {
			return err
		}
		return tx.Commit()
	}
	return r.createTuplesBatchOn(ctx, r.db, tuples)
}

func (r Repository) createTuplesBatchOn(ctx context.Context, dtx DBTX, tuples []*domain.Tuple) error {
	orgID, nsID := tuples[0].OrganizationID, tuples[0].NamespaceID
	if err := r.bumpRevisionInTx(ctx, dtx, nsID, orgID); err != nil {
		return err
	}
	var newCounter int64
	if err := dtx.QueryRowContext(ctx, queryReadNewRevision, nsID, orgID).Scan(&newCounter); err != nil {
		return err
	}
	zookie := strconv.FormatInt(newCounter, 10)

	var sb strings.Builder
	sb.WriteString(`INSERT IGNORE INTO relation_tuples
  (id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at)
VALUES `)
	args := make([]any, 0, 8*len(tuples))
	for i, t := range tuples {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, NOW())")
		args = append(args, t.ID, t.OrganizationID, t.NamespaceID, t.Subject, nullableString(t.SubjectRelation), t.Relation, t.Object, zookie)
	}
	if _, err := dtx.ExecContext(ctx, sb.String(), args...); err != nil {
		return err
	}
	for _, t := range tuples {
		if err := scanTupleByKey(ctx, dtx, t); err != nil {
			return err
		}
	}
	return nil
}
