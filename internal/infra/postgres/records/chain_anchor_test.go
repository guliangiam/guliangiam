package records

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/guliangiam/guliangiam/internal/platform/outbox/providers/audit"
)

const testAnchorOrganizationID = "01a0ea76-0c00-7000-8000-000000000001"

// Golden proof of "chain_id=<org>\nanchor_seq=7\nentry_hash=<32×01>" with SHA-256;
// the platform package has an equivalent recipe test, this one proves the SQL
// path stores the same value.
const goldenAnchorProof = "sha256:bee3fb5ca2b8123f4d5d61661f6cef4ea888358927f714bcca4a76dfb701dd39"

func TestQueryCreateRecord_WritesChainedStatus(t *testing.T) {
	assert.Contains(t, queryCreateRecord, "chain_status")
	assert.Contains(t, queryCreateRecord, "'chained'", "a new record must be chained at birth, not left for the worker")
	assert.NotContains(t, queryCreateRecord, "'pending'")
}

func TestDefaultChainSQL_AnchorAndAdvanceStatements(t *testing.T) {
	sql := DefaultChainSQL()

	require.Contains(t, sql.InsertAnchorAndAdvance, "INSERT INTO guliangiam.audit_chain_anchors")
	require.Contains(t, sql.InsertAnchorAndAdvance, "UPDATE guliangiam.records")
	require.Contains(t, sql.InsertAnchorAndAdvance, "SET chain_status = 'anchored'")
	require.Contains(t, sql.InsertAnchorAndAdvance, "chain_status = 'chained'", "pending/broken rows must not be masked as anchored")
	require.Contains(t, sql.InsertAnchorAndAdvance, "chain_seq <= $4")

	require.Contains(t, sql.AdvanceChainedStatus, "UPDATE guliangiam.records")
	require.Contains(t, sql.AdvanceChainedStatus, "SET chain_status = 'anchored'")
	require.Contains(t, sql.AdvanceChainedStatus, "chain_status = 'chained'")
	require.Contains(t, sql.AdvanceChainedStatus, "chain_seq <= $2")
}

type capturedExec struct {
	sql  string
	args []any
}

func newAnchorTestDB(tailSeq int64, tailHash []byte, maxAnchorSeq int64) (*shared.MockDBTX, *[]capturedExec) {
	var execs []capturedExec
	db := &shared.MockDBTX{
		ExecFn: func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			execs = append(execs, capturedExec{sql: sql, args: args})
			return pgconn.NewCommandTag("UPDATE 1"), nil
		},
		QueryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			switch sql {
			case chainSQLLoadLastEntry:
				return shared.NewMockRow([]any{tailSeq, tailHash, "sha256"}, nil)
			case chainSQLLoadAnchorSeq:
				return shared.NewMockRow([]any{maxAnchorSeq}, nil)
			default:
				return shared.NewMockRow(nil, errors.New("unexpected QueryRow: "+sql))
			}
		},
	}
	return db, &execs
}

func TestAuditChainWriter_Anchor_NewTailUsesRealSQL(t *testing.T) {
	tailHash := bytes.Repeat([]byte{0x01}, 32)
	db, execs := newAnchorTestDB(7, tailHash, 3)
	writer := audit.NewAuditChainWriter(db, DefaultChainSQL())

	require.NoError(t, writer.Anchor(context.Background(), testAnchorOrganizationID, testAnchorOrganizationID, ""))

	require.Len(t, *execs, 2)
	assert.Equal(t, chainSQLAdvisoryLock, (*execs)[0].sql)
	assert.Equal(t, chainSQLInsertAnchorAndAdvance, (*execs)[1].sql)

	args := (*execs)[1].args
	require.Len(t, args, 6)
	assert.Equal(t, testAnchorOrganizationID, args[2])
	assert.Equal(t, int64(7), args[3], "anchor_seq must close the covered tail chain_seq")
	assert.Equal(t, tailHash, args[4])
	assert.Equal(t, goldenAnchorProof, args[5], "the worker path must persist the derivable proof, not an empty string")
}

func TestDefaultChainSQL_CountPendingUsesExplicitStatusFilter(t *testing.T) {
	sql := DefaultChainSQL()

	require.Contains(t, sql.CountPending, "COUNT(*)")
	require.Contains(t, sql.CountPending, "FROM guliangiam.records")
	require.Contains(t, sql.CountPending, "WHERE chain_status = 'pending'")
	assert.NotContains(t, sql.CountPending, "entry_hash IS NULL",
		"pending must be filtered by chain_status, not inferred from a missing hash")
}

func TestAuditChainWriter_CountPendingUsesSQL(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, sql string, _ ...any) pgx.Row {
			require.Equal(t, chainSQLCountPending, sql)
			return shared.NewMockRow([]any{int64(9)}, nil)
		},
	}
	writer := audit.NewAuditChainWriter(db, DefaultChainSQL())

	pending, err := writer.CountPending(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(9), pending)
}

func TestAuditChainWriter_CountPendingPropagatesError(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
			return shared.NewMockRow(nil, errors.New("count failed"))
		},
	}
	writer := audit.NewAuditChainWriter(db, DefaultChainSQL())

	_, err := writer.CountPending(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count pending audit chain entries")
}

func TestAuditChainWriter_Anchor_UnchangedTailUsesBackfillSQLOnly(t *testing.T) {
	tailHash := bytes.Repeat([]byte{0x01}, 32)
	db, execs := newAnchorTestDB(7, tailHash, 7)
	writer := audit.NewAuditChainWriter(db, DefaultChainSQL())

	require.NoError(t, writer.Anchor(context.Background(), testAnchorOrganizationID, testAnchorOrganizationID, ""))

	require.Len(t, *execs, 2, "an unchanged tail must not insert another anchor row")
	assert.Equal(t, chainSQLAdvisoryLock, (*execs)[0].sql)
	assert.Equal(t, chainSQLAdvanceChainedStatus, (*execs)[1].sql)

	args := (*execs)[1].args
	require.Len(t, args, 2)
	assert.Equal(t, testAnchorOrganizationID, args[0])
	assert.Equal(t, int64(7), args[1])
}
