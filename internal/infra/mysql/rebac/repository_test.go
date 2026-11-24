package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
	"github.com/stretchr/testify/assert"
)

// newMockDB 返回 sql.DB + sqlmock，方便各测试独立构造期望序列。
func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ─── CreateNamespace ─────────────────────────────────────────────────────

func TestMySQLRepository_CreateNamespace(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	r := NewRepositoryWithDBTX(db)

	// CreateNamespace 先 INSERT，再 SELECT 回读（MySQL 无 RETURNING）
	rels, _ := json.Marshal(map[string]any{"roles": []string{"admin", "member"}})

	mock.ExpectExec("INSERT INTO relation_namespaces").
		WithArgs("ns-1", "org-1", "doc", "Doc Namespace", rels, 1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery("SELECT .*relation_namespaces").
		WithArgs("ns-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "name", "display_name",
			"relation_defs", "version", "created_at", "updated_at", "revision_counter"}).
			AddRow("ns-1", "org-1", "doc", "Doc Namespace", rels, 1,
				time.Now(), time.Now(), 0))

	ns := &domain.Namespace{
		ID: "ns-1", OrganizationID: "org-1", Name: "doc",
		DisplayName:  "Doc Namespace",
		RelationDefs: map[string]any{"roles": []string{"admin", "member"}},
		Version:      1,
	}
	err := r.CreateNamespace(context.Background(), ns)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), ns.Revision, "初始 revision_counter 应该是 0")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── CreateTuple（事务两步） ───────────────────────────────────────────────
// 核心！验证 MySQL 事务内的 UPDATE bump → SELECT → INSERT IGNORE 三步。

func TestMySQLRepository_CreateTuple_Transactional(t *testing.T) {
	// 用 *sql.DB 开启事务，验证 BeginTx + 三步序列
	rawDB, mock := newMockDB(t)
	defer rawDB.Close()
	r := NewRepository(rawDB)

	// 1. BeginTx
	mock.ExpectBegin()

	// 2. UPDATE bump（revision_counter + 1）
	mock.ExpectExec("UPDATE relation_namespaces SET revision_counter").
		WithArgs("ns-1", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 3. SELECT 新的 revision
	mock.ExpectQuery("SELECT revision_counter FROM relation_namespaces").
		WithArgs("ns-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{"revision_counter"}).AddRow(int64(7)))

	// 4. INSERT IGNORE
	mock.ExpectExec("INSERT IGNORE INTO relation_tuples").
		WithArgs("tuple-1", "org-1", "ns-1", "user:u1", sql.NullString{Valid: true, String: "member"},
			"write", "folder:f1", "7").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 5. Read the persisted tuple and the namespace's current Zookie.
	mock.ExpectQuery("CAST\\(n\\.revision_counter AS CHAR\\)").
		WithArgs("org-1", "ns-1", "user:u1", "write", "folder:f1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "namespace_id", "subject",
			"subject_relation", "relation", "object", "zookie", "created_at"}).
			AddRow("tuple-1", "org-1", "ns-1", "user:u1", "member", "write", "folder:f1", "7", time.Now()))

	// 6. Commit
	mock.ExpectCommit()

	tuple := &domain.Tuple{
		ID: "tuple-1", OrganizationID: "org-1", NamespaceID: "ns-1",
		Subject: "user:u1", Relation: "write", Object: "folder:f1",
		SubjectRelation: strPtr("member"),
	}
	err := r.CreateTuple(context.Background(), tuple)
	assert.NoError(t, err)
	assert.Equal(t, "7", tuple.Zookie, "zookie 应该是 UPDATE bump 后的 revision")
	assert.Equal(t, "tuple-1", tuple.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLRepository_CreateTuple_Idempotent_BumpStillHappens(t *testing.T) {
	// INSERT IGNORE 命中唯一约束时，仍会 bump revision_counter
	rawDB, mock := newMockDB(t)
	defer rawDB.Close()
	r := NewRepository(rawDB)

	mock.ExpectBegin()

	// bump 发生
	mock.ExpectExec("UPDATE relation_namespaces SET revision_counter").
		WithArgs("ns-1", "org-1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// SELECT 拿到 revision 8（第 2 次幂等调用）
	mock.ExpectQuery("SELECT revision_counter FROM relation_namespaces").
		WithArgs("ns-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{"revision_counter"}).AddRow(int64(8)))

	// INSERT IGNORE —— RowsAffected = 0（幂等命中）
	mock.ExpectExec("INSERT IGNORE INTO relation_tuples").
		WithArgs("tuple-1", "org-1", "ns-1", "user:u1", sql.NullString{},
			"write", "folder:f1", "8").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mock.ExpectQuery("CAST\\(n\\.revision_counter AS CHAR\\)").
		WithArgs("org-1", "ns-1", "user:u1", "write", "folder:f1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "namespace_id", "subject",
			"subject_relation", "relation", "object", "zookie", "created_at"}).
			AddRow("original-tuple-id", "org-1", "ns-1", "user:u1", nil, "write", "folder:f1", "8", time.Now()))

	mock.ExpectCommit()

	tuple := &domain.Tuple{
		ID: "tuple-1", OrganizationID: "org-1", NamespaceID: "ns-1",
		Subject: "user:u1", Relation: "write", Object: "folder:f1",
	}
	err := r.CreateTuple(context.Background(), tuple)
	assert.NoError(t, err)
	assert.Equal(t, "8", tuple.Zookie, "幂等写入也 bump zookie")
	assert.Equal(t, "original-tuple-id", tuple.ID, "idempotent writes must return the persisted tuple ID")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLRepository_CreateTuple_RollsBackOnBumpError(t *testing.T) {
	rawDB, mock := newMockDB(t)
	defer rawDB.Close()
	r := NewRepository(rawDB)

	boom := errors.New("bump failed")
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE relation_namespaces SET revision_counter").
		WithArgs("ns-1", "org-1").
		WillReturnError(boom)
	mock.ExpectRollback()

	tuple := &domain.Tuple{
		ID: "tuple-1", OrganizationID: "org-1", NamespaceID: "ns-1",
		Subject: "user:u1", Relation: "write", Object: "folder:f1",
	}
	err := r.CreateTuple(context.Background(), tuple)
	assert.ErrorIs(t, err, boom)
	assert.NoError(t, mock.ExpectationsWereMet(), "失败路径必须回滚事务")
}

// ─── FetchAllTuples + atZookie 快照 ───────────────────────────────────────

func TestMySQLRepository_FetchAllTuples_WithZookieFilter(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	r := NewRepositoryWithDBTX(db)

	// atZookie = "5" → 只返回 zookie <= 5 的元组
	mock.ExpectQuery("SELECT .* FROM relation_tuples").
		WithArgs("org-1", "ns-1", "5", "5").
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "namespace_id",
			"subject", "subject_relation", "relation", "object", "zookie", "created_at"}).
			AddRow("t1", "org-1", "ns-1", "user:u1", nil, "read", "folder:f1", "3", time.Now()).
			AddRow("t2", "org-1", "ns-1", "user:u2", nil, "write", "folder:f1", "5", time.Now()))

	tuples, err := r.FetchAllTuples(context.Background(), "org-1", "ns-1", "5")
	assert.NoError(t, err)
	assert.Len(t, tuples, 2)
	assert.Equal(t, "3", tuples[0].Zookie)
	assert.Equal(t, "5", tuples[1].Zookie)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMySQLRepository_FetchAllTuples_LatestSnapshot(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	r := NewRepositoryWithDBTX(db)

	// atZookie = "" → 返回全量（最新快照）
	mock.ExpectQuery("SELECT .* FROM relation_tuples").
		WithArgs("org-1", "ns-1", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "namespace_id",
			"subject", "subject_relation", "relation", "object", "zookie", "created_at"}).
			AddRow("t1", "org-1", "ns-1", "user:u1", nil, "read", "folder:f1", "99", time.Now()))

	tuples, err := r.FetchAllTuples(context.Background(), "org-1", "ns-1", "")
	assert.NoError(t, err)
	assert.Len(t, tuples, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── GetLatestZookie ──────────────────────────────────────────────────────

func TestMySQLRepository_GetLatestZookie(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()
	r := NewRepositoryWithDBTX(db)

	mock.ExpectQuery("SELECT revision_counter FROM relation_namespaces").
		WithArgs("ns-1", "org-1").
		WillReturnRows(sqlmock.NewRows([]string{"revision_counter"}).AddRow(int64(12)))

	z, err := r.GetLatestZookie(context.Background(), "org-1", "ns-1")
	assert.NoError(t, err)
	assert.Equal(t, "12", z)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ─── 接口实现断言 ────────────────────────────────────────────────────────

func TestMySQLRepository_ImplementsDomainInterface(t *testing.T) {
	var _ domain.Repository = Repository{}
	var _ domain.TupleStore = Repository{}
}

// ─── helpers ──────────────────────────────────────────────────────────────

func strPtr(s string) *string { return &s }
