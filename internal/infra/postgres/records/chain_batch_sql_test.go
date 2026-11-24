package records

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultChainSQL_ListEntriesBatchSelectsFullVerifiableFields pins the
// batch statement to the full canonical payload. A regression back to the
// link-only column set (chain_seq/prev_hash/entry_hash/chain_algo) would make
// the background worker silently unable to detect field tampering.
func TestDefaultChainSQL_ListEntriesBatchSelectsFullVerifiableFields(t *testing.T) {
	sql := DefaultChainSQL()
	require.NotEmpty(t, sql.ListEntriesBatch, "the worker needs a full-record batch statement")

	for _, field := range []string{
		"organization_id::text",
		"COALESCE(actor_id::text, '')",
		"action",
		"COALESCE(resource_type, '')",
		"COALESCE(resource_id, '')",
		"COALESCE(HOST(ip), '')",
		"COALESCE(user_agent, '')",
		"COALESCE(surface, '')",
		"COALESCE(request_id, '')",
		"COALESCE(trace_id, '')",
		"COALESCE(metadata, '{}'::jsonb)",
		"created_at",
		"chain_seq",
		"prev_hash",
		"entry_hash",
		"chain_algo",
	} {
		assert.Contains(t, sql.ListEntriesBatch, field)
	}

	assert.Contains(t, sql.ListEntriesBatch, "WHERE chain_id = $1")
	assert.Contains(t, sql.ListEntriesBatch, "entry_hash IS NOT NULL")
	assert.Contains(t, sql.ListEntriesBatch, "chain_seq > $2")
	assert.Contains(t, sql.ListEntriesBatch, "ORDER BY chain_seq ASC")
	assert.Contains(t, sql.ListEntriesBatch, "LIMIT $3")
	assert.NotEqual(t, sql.ListEntries, sql.ListEntriesBatch,
		"the link-only statement must not back the full verifier")
}
