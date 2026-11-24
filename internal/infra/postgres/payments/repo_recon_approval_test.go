package payments

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domainoutbox "github.com/guliangiam/guliangiam/internal/domain/outbox"
	domainrecords "github.com/guliangiam/guliangiam/internal/domain/records"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── BE-C-18 harness ──────────────────────────────────────────────────────

type reconExec struct {
	sql  string
	args []any
}

// newReconApprovalDB：approve RETURNING 一行；审计链为空链（ErrNoRows）；
// execErrOn 命中的 INSERT 返回错误，其余 Exec 成功。
func newReconApprovalDB(execs *[]reconExec, execErrOn string) *shared.MockDBTX {
	return &shared.MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			*execs = append(*execs, reconExec{sql: sql, args: args})
			if execErrOn != "" && strings.Contains(sql, execErrOn) {
				return pgconn.CommandTag{}, errors.New("audit write failed")
			}
			return pgconn.NewCommandTag("INSERT 1"), nil
		},
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			if sql == ReconItemApproveForOrganization {
				return shared.NewMockRow([]any{"resolver-1"}, nil)
			}
			// 审计链 LoadLastEntry → 空链
			return shared.NewMockRow(nil, pgx.ErrNoRows)
		},
	}
}

func reconApprovalAudit(original string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error) {
	org := "org-1"
	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	return &domainrecords.Record{
		ID: "audit-1", OrganizationID: org, ActorID: "approver-1",
		Action: "payment.recon.item.approved", ResourceType: "pay_recon_item",
		ResourceID: "item-1", Surface: domainrecords.SurfaceManagement,
		Metadata: map[string]any{"originalOperator": original}, CreatedAt: now,
	}, &domainoutbox.OutboxEvent{
		ID: "audit-1", OrganizationID: &org,
		AggregateType: "pay_recon_item", AggregateID: "item-1",
		EventType: "payment.recon.item.approved", EventVersion: 1,
		Payload: []byte(`{}`), MaxAttempts: 3, AvailableAt: now,
	}, nil
}

// 已绑定调用方事务（db 无 Begin）：approve → records → outbox 顺序复用同一 db。
func TestRecon_ApproveItemWithAudit_TxBoundWritesAuditSequentially(t *testing.T) {
	var execs []reconExec
	db := newReconApprovalDB(&execs, "")
	repo := NewReconRepository(db)

	built := 0
	_, err := repo.ApproveItemWithAudit(context.Background(), "item-1", "org-1", "approver-1", func(original string) (*domainrecords.Record, *domainoutbox.OutboxEvent, error) {
		built++
		return reconApprovalAudit(original)
	})
	require.NoError(t, err)
	assert.Equal(t, 1, built)

	var recordIdx, outboxIdx = -1, -1
	for i, e := range execs {
		if strings.Contains(e.sql, "INSERT INTO guliangiam.records") {
			recordIdx = i
		}
		if strings.Contains(e.sql, "INSERT INTO guliangiam.outbox") {
			outboxIdx = i
		}
	}
	require.GreaterOrEqual(t, recordIdx, 0, "audit record must be inserted")
	require.GreaterOrEqual(t, outboxIdx, 0, "outbox event must be inserted")
	assert.Less(t, recordIdx, outboxIdx)
}

// 审计写入失败必须上抛（事务绑定路径由调用方回滚；不得吞错）。
func TestRecon_ApproveItemWithAudit_AuditFailurePropagates(t *testing.T) {
	var execs []reconExec
	db := newReconApprovalDB(&execs, "INSERT INTO guliangiam.records")
	repo := NewReconRepository(db)

	_, err := repo.ApproveItemWithAudit(context.Background(), "item-1", "org-1", "approver-1", reconApprovalAudit)
	require.Error(t, err)
	for _, e := range execs {
		assert.NotContains(t, e.sql, "INSERT INTO guliangiam.outbox", "outbox must not be written after record failure")
	}
}

// ─── 自开事务路径（db 可 Begin）───────────────────────────────────────────

type reconFakeTx struct {
	pgx.Tx
	db         *shared.MockDBTX
	committed  bool
	rolledBack bool
}

func (t *reconFakeTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return t.db.Exec(ctx, sql, args...)
}
func (t *reconFakeTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return t.db.Query(ctx, sql, args...)
}
func (t *reconFakeTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return t.db.QueryRow(ctx, sql, args...)
}
func (t *reconFakeTx) Commit(ctx context.Context) error   { t.committed = true; return nil }
func (t *reconFakeTx) Rollback(ctx context.Context) error { t.rolledBack = true; return nil }

type reconFakePool struct {
	*shared.MockDBTX
	tx *reconFakeTx
}

func (p *reconFakePool) Begin(ctx context.Context) (pgx.Tx, error) { return p.tx, nil }

// 成功路径：系统作用域 set_config → approve/审计 → Commit。
func TestRecon_ApproveItemWithAudit_PoolPathCommitsAtomically(t *testing.T) {
	var execs []reconExec
	db := newReconApprovalDB(&execs, "")
	tx := &reconFakeTx{db: db}
	repo := NewReconRepository(&reconFakePool{MockDBTX: db, tx: tx})

	_, err := repo.ApproveItemWithAudit(context.Background(), "item-1", "org-1", "approver-1", reconApprovalAudit)
	require.NoError(t, err)
	assert.True(t, tx.committed, "approval + audit must commit")
	require.NotEmpty(t, execs)
	assert.Contains(t, execs[0].sql, "set_config('app.current_role', 'super_admin', true)",
		"FORCE RLS audit tables need system scope inside the tx")
}

// 审计失败：Rollback，approve 不得单独提交。
func TestRecon_ApproveItemWithAudit_PoolPathRollsBackOnAuditFailure(t *testing.T) {
	var execs []reconExec
	db := newReconApprovalDB(&execs, "INSERT INTO guliangiam.outbox")
	tx := &reconFakeTx{db: db}
	repo := NewReconRepository(&reconFakePool{MockDBTX: db, tx: tx})

	_, err := repo.ApproveItemWithAudit(context.Background(), "item-1", "org-1", "approver-1", reconApprovalAudit)
	require.Error(t, err)
	assert.False(t, tx.committed, "must not commit when audit fails")
	assert.True(t, tx.rolledBack, "approve must roll back with the audit")
}
