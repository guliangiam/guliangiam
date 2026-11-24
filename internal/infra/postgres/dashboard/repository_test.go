package dashboard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
)

// ---------------------------------------------------------------------------
// fakePool / fakeTx：dashboard 只读事务的测试替身
// ---------------------------------------------------------------------------

type fakeCall struct {
	sql  string
	args []any
}

type fakeTx struct {
	pgx.Tx // 未实现的方法保持 nil 内嵌（被测路径不会触达）

	execCalls     []fakeCall
	queryCalls    []fakeCall
	queryRowCalls []fakeCall

	execFn     func(sql string, args ...any) (pgconn.CommandTag, error)
	queryFn    func(sql string, args ...any) (pgx.Rows, error)
	queryRowFn func(sql string, args ...any) pgx.Row

	commitCalls   int
	rollbackCalls int
	commitErr     error
}

func (t *fakeTx) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("nested begin not supported")
}

func (t *fakeTx) Commit(context.Context) error {
	t.commitCalls++
	return t.commitErr
}

func (t *fakeTx) Rollback(context.Context) error {
	t.rollbackCalls++
	return nil
}

func (t *fakeTx) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.execCalls = append(t.execCalls, fakeCall{sql: sql, args: args})
	if t.execFn != nil {
		return t.execFn(sql, args...)
	}
	return pgconn.CommandTag{}, nil
}

func (t *fakeTx) Query(_ context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.queryCalls = append(t.queryCalls, fakeCall{sql: sql, args: args})
	if t.queryFn != nil {
		return t.queryFn(sql, args...)
	}
	return nil, fmt.Errorf("unexpected query: %s", sql)
}

func (t *fakeTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	t.queryRowCalls = append(t.queryRowCalls, fakeCall{sql: sql, args: args})
	if t.queryRowFn != nil {
		return t.queryRowFn(sql, args...)
	}
	return shared.NewMockRow(nil, fmt.Errorf("unexpected query row: %s", sql))
}

type fakePool struct {
	tx         *fakeTx
	beginErr   error
	beginCalls int
}

func (p *fakePool) Begin(context.Context) (pgx.Tx, error) {
	p.beginCalls++
	if p.beginErr != nil {
		return nil, p.beginErr
	}
	return p.tx, nil
}

func newTestRepo(tx *fakeTx) (Repository, *fakePool) {
	pool := &fakePool{tx: tx}
	return NewRepository(pool), pool
}

// queryRowErrorRepo 构造一个 QueryRow 恒返回错误的仓储（覆盖 GUC 成功、查询失败路径）。
func queryRowErrorRepo() Repository {
	repo, _ := newTestRepo(&fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	})
	return repo
}

func queryErrorRepo() Repository {
	repo, _ := newTestRepo(&fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) {
			return nil, errors.New("db error")
		},
	})
	return repo
}

// ---------------------------------------------------------------------------
// RLS 上下文与事务生命周期
// ---------------------------------------------------------------------------

const testOrgUUID = "11111111-1111-1111-1111-111111111111"

// 空 organizationID（平台跨租户视图）必须以事务级 super_admin GUC 执行，
// 否则 FORCE RLS fail-closed 会把所有行遮蔽为空。
func TestRepository_RLSContext_EmptyOrgUsesSuperAdmin(t *testing.T) {
	tx := &fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row { return shared.NewMockRow([]any{int64(1)}, nil) },
	}
	repo, _ := newTestRepo(tx)
	_, err := repo.CountApplications(context.Background(), "")
	require.NoError(t, err)

	require.Len(t, tx.execCalls, 1)
	assert.Contains(t, tx.execCalls[0].sql, "'super_admin'")
	assert.Contains(t, tx.execCalls[0].sql, "true)") // LOCAL 事务级，禁止 SESSION 污染 pool
	assert.Equal(t, 1, tx.commitCalls)
}

// 合法 UUID 租户必须以 member + set_tenant_context 执行。
func TestRepository_RLSContext_UUIDOrgUsesMember(t *testing.T) {
	tx := &fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row { return shared.NewMockRow([]any{int64(1)}, nil) },
	}
	repo, _ := newTestRepo(tx)
	_, err := repo.CountApplications(context.Background(), testOrgUUID)
	require.NoError(t, err)

	require.Len(t, tx.execCalls, 1)
	assert.Contains(t, tx.execCalls[0].sql, "guliangiam.set_tenant_context")
	require.Len(t, tx.execCalls[0].args, 1)
	assert.Equal(t, testOrgUUID, tx.execCalls[0].args[0])
	assert.Equal(t, 1, tx.commitCalls)
}

// slug 形式的 organizationID 只有 platform scope 请求能到达（trustedOrgID 裁决），
// 必须映射为 super_admin，由 SQL 的 resolve_org_id 收窄。
func TestRepository_RLSContext_SlugOrgUsesSuperAdmin(t *testing.T) {
	tx := &fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row { return shared.NewMockRow([]any{int64(1)}, nil) },
	}
	repo, _ := newTestRepo(tx)
	_, err := repo.CountApplications(context.Background(), "built-in")
	require.NoError(t, err)

	require.Len(t, tx.execCalls, 1)
	assert.Contains(t, tx.execCalls[0].sql, "'super_admin'")
}

// GUC 设置失败必须中止查询并回滚，不得继续执行 fn。
func TestRepository_RLSContext_SetErrorAborts(t *testing.T) {
	tx := &fakeTx{
		execFn: func(sql string, args ...any) (pgconn.CommandTag, error) {
			return pgconn.CommandTag{}, errors.New("set config failed")
		},
	}
	repo, _ := newTestRepo(tx)
	_, err := repo.CountApplications(context.Background(), testOrgUUID)
	require.Error(t, err)
	assert.Equal(t, 0, tx.commitCalls)
	assert.Equal(t, 1, tx.rollbackCalls)
	assert.Empty(t, tx.queryRowCalls)
}

// Begin 失败直接返回错误。
func TestRepository_BeginError(t *testing.T) {
	repo, pool := newTestRepo(&fakeTx{})
	pool.beginErr = errors.New("pool exhausted")
	_, err := repo.CountUsers(context.Background(), testOrgUUID)
	require.Error(t, err)
	assert.Equal(t, 1, pool.beginCalls)
}

// 查询错误路径：不提交，defer 回滚。
func TestRepository_QueryErrorRollsBack(t *testing.T) {
	tx := &fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("db error"))
		},
	}
	repo, _ := newTestRepo(tx)
	_, err := repo.CountUsers(context.Background(), testOrgUUID)
	require.Error(t, err)
	assert.Equal(t, 0, tx.commitCalls)
	assert.Equal(t, 1, tx.rollbackCalls)
}

// Commit 失败必须向上传播。
func TestRepository_CommitError(t *testing.T) {
	tx := &fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row { return shared.NewMockRow([]any{int64(3)}, nil) },
		commitErr:  errors.New("commit failed"),
	}
	repo, _ := newTestRepo(tx)
	_, err := repo.CountApplications(context.Background(), testOrgUUID)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// SQL 形状：登录活动数据源修正（回归护栏）
// ---------------------------------------------------------------------------

// 登录活动/热力图/趋势的"登录"必须来自 sessions——records 中不存在
// action='login' 的行，成功登录也不写 records（历史缺陷：恒 0）。
func TestRepository_SQL_LoginSourcesUseSessions(t *testing.T) {
	rows := shared.NewMockRows([][]any{{"2026-11-17", int64(1)}})
	tx := &fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) { return rows, nil },
	}
	repo, _ := newTestRepo(tx)

	_, err := repo.GetActivityHeatmap(context.Background(), testOrgUUID)
	require.NoError(t, err)
	_, err = repo.GetTrends(context.Background(), testOrgUUID)
	require.NoError(t, err)

	require.Len(t, tx.queryCalls, 2)
	for _, call := range tx.queryCalls {
		assert.Contains(t, call.sql, "guliangiam.sessions")
		assert.NotContains(t, call.sql, "action = 'login'")
		assert.NotContains(t, call.sql, "guliangiam.records")
	}

	// login heatmap（小时分布）同样以 sessions 为源。
	tx2 := &fakeTx{
		queryFn: func(sql string, args ...any) (pgx.Rows, error) {
			return shared.NewMockRows([][]any{{9, int64(1)}}), nil
		},
	}
	repo2, _ := newTestRepo(tx2)
	_, err = repo2.GetLoginHeatmap(context.Background(), testOrgUUID)
	require.NoError(t, err)
	require.Len(t, tx2.queryCalls, 1)
	assert.Contains(t, tx2.queryCalls[0].sql, "guliangiam.sessions")
	assert.NotContains(t, tx2.queryCalls[0].sql, "action = 'login'")
}

// 失败登录计数必须用审计事件名 auth.login.failed（不存在 'login-failed'）。
func TestRepository_SQL_FailedLoginsUsesAuditEventName(t *testing.T) {
	tx := &fakeTx{
		queryRowFn: func(sql string, args ...any) pgx.Row { return shared.NewMockRow([]any{int64(2)}, nil) },
	}
	repo, _ := newTestRepo(tx)
	_, err := repo.CountFailedLogins24h(context.Background(), testOrgUUID)
	require.NoError(t, err)

	require.Len(t, tx.queryRowCalls, 1)
	assert.Contains(t, tx.queryRowCalls[0].sql, "auth.login.failed")
	assert.False(t, strings.Contains(tx.queryRowCalls[0].sql, "'login-failed'"))
}

// ---------------------------------------------------------------------------
// 各方法错误路径（保持原有覆盖）
// ---------------------------------------------------------------------------

func TestRepository_CountUsers_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountUsers(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountOrganizations_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountOrganizations(context.Background())
	assert.Error(t, err)
}

func TestRepository_CountApplications_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountApplications(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountRoles_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountRoles(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountGroups_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountGroups(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountPermissions_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountPermissions(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountProviders_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountProviders(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountActiveSessions_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountActiveSessions(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountTokens_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountTokens(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_CountFailedLogins24h_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().CountFailedLogins24h(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_GetMfaCoverageStats_QueryRowError(t *testing.T) {
	_, err := queryRowErrorRepo().GetMfaCoverageStats(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_GetLoginHeatmap_QueryError(t *testing.T) {
	_, err := queryErrorRepo().GetLoginHeatmap(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_GetProviderDistribution_QueryError(t *testing.T) {
	_, err := queryErrorRepo().GetProviderDistribution(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_GetTrends_QueryError(t *testing.T) {
	_, err := queryErrorRepo().GetTrends(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_GetActivityHeatmap_QueryError(t *testing.T) {
	_, err := queryErrorRepo().GetActivityHeatmap(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestRepository_GetOrgUserDistribution_QueryError(t *testing.T) {
	_, err := queryErrorRepo().GetOrgUserDistribution(context.Background())
	assert.Error(t, err)
}
