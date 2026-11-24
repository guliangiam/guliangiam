package rebac

import (
	"context"
	"fmt"
	"strings"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/pgerror"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// CreateTuplesBatch 用单条多行 CTE 完成同 namespace 批量写入（BE-D-18）：
// revision_counter 只 bump 一次；ON CONFLICT DO NOTHING 保持逐键幂等，
// 冲突行保留原 zookie 并按键回读——与单条 CreateTuple 的回退路径一致。
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

	var values strings.Builder
	args := make([]any, 0, 2+5*len(tuples))
	args = append(args, nsID, orgID)
	for i, t := range tuples {
		if i > 0 {
			values.WriteString(", ")
		}
		base := 3 + 5*i
		fmt.Fprintf(&values, "($%d, $2, $1, $%d, $%d, $%d, $%d, (SELECT bump.revision_counter FROM bump), now())",
			base, base+1, base+2, base+3, base+4)
		args = append(args, t.ID, t.Subject, t.SubjectRelation, t.Relation, t.Object)
	}
	// INSERT 与 ON CONFLICT 必须留在同一 SQL 字面量内：重放门禁的仲裁目标扫描
	// 按字面量独立解析，拆成两段会让冲突子句无法归属表名（validate_schema_replay）。
	query := fmt.Sprintf(`
WITH bump AS (
  UPDATE guliangiam.relation_namespaces
  SET revision_counter = revision_counter + 1
  WHERE id = $1 AND organization_id = $2
  RETURNING revision_counter
)
INSERT INTO guliangiam.relation_tuples
  (id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at)
VALUES %s
ON CONFLICT (organization_id, namespace_id, subject, relation, object) DO NOTHING
RETURNING id, organization_id, namespace_id, subject, subject_relation, relation, object, zookie, created_at`, values.String())

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return pgerror.Map(err, tupleConstraints...)
	}
	defer rows.Close()
	inserted := make(map[string]*domain.Tuple, len(tuples))
	for rows.Next() {
		var scanned dbTuple
		if err := rows.Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
			&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
			&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
		); err != nil {
			return pgerror.Map(err, tupleConstraints...)
		}
		d := scanned.toDomain()
		inserted[tupleBatchKey(d)] = d
	}
	if err := rows.Err(); err != nil {
		return pgerror.Map(err, tupleConstraints...)
	}

	for _, t := range tuples {
		if found, ok := inserted[tupleBatchKey(t)]; ok {
			*t = *found
			continue
		}
		var scanned dbTuple
		err := r.db.QueryRow(ctx, queryGetTupleByKey, orgID, nsID, t.Subject, t.Relation, t.Object).Scan(
			&scanned.ID, &scanned.OrganizationID, &scanned.NamespaceID,
			&scanned.Subject, &scanned.SubjectRelation, &scanned.Relation,
			&scanned.Object, &scanned.Zookie, &scanned.CreatedAt,
		)
		if err != nil {
			return pgerror.Map(err, tupleConstraints...)
		}
		*t = *scanned.toDomain()
	}
	return nil
}

func tupleBatchKey(t *domain.Tuple) string {
	sr := ""
	if t.SubjectRelation != nil {
		sr = *t.SubjectRelation
	}
	return t.Subject + "\x00" + sr + "\x00" + t.Relation + "\x00" + t.Object
}
