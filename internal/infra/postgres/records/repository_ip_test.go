package records

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// 回归：records.ip 是 PostgreSQL inet（OID 869）。pgx 不能把 inet 的二进制格式
// 扫进 *string，会报 `cannot scan inet ... into **string`，导致审计记录列表 500。
// SELECT 必须显式转成 text。
func TestList_SelectsIPAsText(t *testing.T) {
	var listSQL string
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			listSQL = sql
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}

	_, _, err := NewRepository(db).List(context.Background(), domain.ListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Contains(t, listSQL, "ip::text")
}

func TestSearch_SelectsIPAsText(t *testing.T) {
	var searchSQL string
	db := &shared.MockDBTX{
		QueryFn: func(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
			searchSQL = sql
			return shared.NewMockRows(nil), nil
		},
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(0)}, nil)
		},
	}

	_, _, err := NewRepository(db).Search(context.Background(), domain.SearchCriteria{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Contains(t, searchSQL, "ip::text")
}

// 回归：records.ip 是 inet，空字符串不是合法 inet（`invalid input syntax for
// type inet: ""`，SQLSTATE 22P02）。没有客户端 IP 的写路径（如创建用户时嵌入
// 的审计事件）必须在 INSERT 里传 SQL NULL，而不是空串。
func TestCreate_EmptyIPWritesNull(t *testing.T) {
	var insertArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO guliangiam.records") {
				insertArgs = append([]any(nil), args...)
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			// 空链：chain_seq=0、无前序哈希。
			return shared.NewMockRow([]any{int64(0), nil, ""}, nil)
		},
	}

	record := &domain.Record{
		ID:             "0199c000-0000-7000-8000-000000000010",
		OrganizationID: "01a0ea76-0c00-7000-8000-000000000001",
		Action:         "user.created",
		ResourceType:   "User",
		ResourceID:     "0199c000-0000-7000-8000-000000000011",
		CreatedAt:      time.Now().UTC(),
	}
	require.NoError(t, NewRepository(db).Create(context.Background(), record))
	require.Len(t, insertArgs, 18)
	require.Nil(t, insertArgs[6], "empty ip must be written as SQL NULL for the inet column")
}

// 非空 IP 必须原样传给 INSERT，归一化不能把真实地址也吞成 NULL。
func TestCreate_NonEmptyIPIsPassedThrough(t *testing.T) {
	var insertArgs []any
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			if strings.Contains(sql, "INSERT INTO guliangiam.records") {
				insertArgs = append([]any(nil), args...)
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
		QueryRowFn: func(context.Context, string, ...any) pgx.Row {
			return shared.NewMockRow([]any{int64(0), nil, ""}, nil)
		},
	}

	record := &domain.Record{
		ID:             "0199c000-0000-7000-8000-000000000012",
		OrganizationID: "01a0ea76-0c00-7000-8000-000000000001",
		Action:         "user.created",
		ResourceType:   "User",
		ResourceID:     "0199c000-0000-7000-8000-000000000011",
		IP:             "203.0.113.10",
		CreatedAt:      time.Now().UTC(),
	}
	require.NoError(t, NewRepository(db).Create(context.Background(), record))
	require.Equal(t, "203.0.113.10", insertArgs[6])
}
