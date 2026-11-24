// Package mysql 是 ReBAC 的 MySQL 8.4+ 存储层实现。
//
// 与 postgres 子包的关键差异：
//   - 用 database/sql + go-sql-driver/mysql，接口类型是通用接口而非 pgx 专用类型
//   - CreateTuple 用事务两步（UPDATE bump → SELECT new revision → INSERT IGNORE）
//     而非 PG 的 CTE (UPDATE RETURNING + INSERT)
//   - 幂等写入用 INSERT IGNORE 而非 ON CONFLICT DO NOTHING
//   - 不处理 RLS（MySQL 无此机制）
//
// SQL 与 pgx 版本的语义保持一致，revision_counter/zookie 机制完全相同。

package mysql

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/rebac"
)

// DBTX 是 MySQL 存储层使用的通用数据库接口，与 database/sql 的
// *sql.DB / *sql.Tx 都兼容。签名刻意保持与 postgres/shared.DBTX 相似，
// 方便未来重构共享接口层——但当前 MySQL 不导入 postgres/shared，
// 避免把 pgconn.CommandTag 等 pgx 专用类型泄漏到 MySQL 代码。
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// 同时接受 *sql.DB 用于事务管理
type txBeginner interface {
	DBTX
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

// dbNamespace/dbTuple 与 postgres 包的同构，但用 database/sql 扫描类型。

type dbNamespace struct {
	ID              string
	OrganizationID  string
	Name            string
	DisplayName     string
	RelationDefs    []byte
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	RevisionCounter int64
}

type dbTuple struct {
	ID              string
	OrganizationID  string
	NamespaceID     string
	Subject         string
	SubjectRelation sql.NullString
	Relation        string
	Object          string
	Zookie          sql.NullString
	CreatedAt       time.Time
}

func (db dbNamespace) toDomain() (*domain.Namespace, error) {
	defs := map[string]any{}
	if len(db.RelationDefs) > 0 {
		if err := jsonUnmarshal(db.RelationDefs, &defs); err != nil {
			return nil, err
		}
	}
	return &domain.Namespace{
		ID:             db.ID,
		OrganizationID: db.OrganizationID,
		Name:           db.Name,
		DisplayName:    db.DisplayName,
		RelationDefs:   defs,
		Version:        db.Version,
		CreatedAt:      db.CreatedAt,
		UpdatedAt:      db.UpdatedAt,
		Revision:       db.RevisionCounter,
	}, nil
}

func (db dbTuple) toDomain() *domain.Tuple {
	var sr *string
	if db.SubjectRelation.Valid {
		s := db.SubjectRelation.String
		sr = &s
	}
	var zookie string
	if db.Zookie.Valid {
		zookie = db.Zookie.String
	}
	return &domain.Tuple{
		ID:              db.ID,
		OrganizationID:  db.OrganizationID,
		NamespaceID:     db.NamespaceID,
		Subject:         db.Subject,
		SubjectRelation: sr,
		Relation:        db.Relation,
		Object:          db.Object,
		Zookie:          zookie,
		CreatedAt:       db.CreatedAt,
	}
}
