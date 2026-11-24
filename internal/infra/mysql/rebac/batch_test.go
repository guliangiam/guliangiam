package mysql

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// BE-D-18 回归：MySQL 批量镜像 = 事务内一次 bump + 单条多行 INSERT IGNORE + 逐键回读。

func TestMySQLRepository_CreateTuplesBatch_Transactional(t *testing.T) {
	rawDB, mock := newMockDB(t)
	defer rawDB.Close()
	r := NewRepository(rawDB)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE relation_namespaces SET revision_counter").
		WithArgs("ns-1", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT revision_counter FROM relation_namespaces").
		WithArgs("ns-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{"revision_counter"}).AddRow(int64(7)))

	// 单条多行 INSERT IGNORE：两行共享同一个 bump 后 zookie "7"。
	mock.ExpectExec("INSERT IGNORE INTO relation_tuples").
		WithArgs(
			"id-a", "org-1", "ns-1", "user:ann", sql.NullString{}, "member", "doc:1", "7",
			"id-b", "org-1", "ns-1", "user:bob", sql.NullString{Valid: true, String: "member"}, "member", "doc:2", "7",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	cols := []string{"id", "organization_id", "namespace_id", "subject",
		"subject_relation", "relation", "object", "zookie", "created_at"}
	// tuple A 新插入；tuple B 命中唯一约束，回读保留原 zookie "3"。
	mock.ExpectQuery("CAST\\(n\\.revision_counter AS CHAR\\)").
		WithArgs("org-1", "ns-1", "user:ann", "member", "doc:1").
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow("id-a", "org-1", "ns-1", "user:ann", nil, "member", "doc:1", "7", time.Now()))
	mock.ExpectQuery("CAST\\(n\\.revision_counter AS CHAR\\)").
		WithArgs("org-1", "ns-1", "user:bob", "member", "doc:2").
		WillReturnRows(sqlmock.NewRows(cols).
			AddRow("id-b-existing", "org-1", "ns-1", "user:bob", "member", "member", "doc:2", "3", time.Now()))
	mock.ExpectCommit()

	tuples := []*domain.Tuple{
		{ID: "id-a", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:ann", Relation: "member", Object: "doc:1"},
		{ID: "id-b", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:bob", SubjectRelation: strPtr("member"), Relation: "member", Object: "doc:2"},
	}
	require.NoError(t, r.CreateTuplesBatch(context.Background(), tuples))

	assert.Equal(t, "7", tuples[0].Zookie)
	assert.Equal(t, "id-a", tuples[0].ID)
	assert.Equal(t, "id-b-existing", tuples[1].ID, "冲突行回读存量记录")
	assert.Equal(t, "3", tuples[1].Zookie, "冲突行保留原 zookie")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLRepository_CreateTuplesBatch_RejectsMixedScope(t *testing.T) {
	rawDB, mock := newMockDB(t)
	defer rawDB.Close()
	r := NewRepository(rawDB)

	err := r.CreateTuplesBatch(context.Background(), []*domain.Tuple{
		{ID: "a", OrganizationID: "org-1", NamespaceID: "ns-1", Subject: "user:a", Relation: "member", Object: "doc:1"},
		{ID: "b", OrganizationID: "org-2", NamespaceID: "ns-1", Subject: "user:b", Relation: "member", Object: "doc:1"},
	})
	require.Error(t, err)
	assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	assert.NoError(t, mock.ExpectationsWereMet(), "混合 org 必须在开事务前被拒绝")
}

func TestMySQLRepository_CreateTuplesBatch_Empty(t *testing.T) {
	rawDB, mock := newMockDB(t)
	defer rawDB.Close()
	r := NewRepository(rawDB)
	require.NoError(t, r.CreateTuplesBatch(context.Background(), nil))
	assert.NoError(t, mock.ExpectationsWereMet())
}
