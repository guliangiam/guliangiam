package crypto

import (
	"context"
	"database/sql"
)

// PayloadBatch is one keyset-page request for a registered encrypted column.
// AfterPK is empty for the first page, which makes the implementation omit the
// cursor predicate: comparing a UUID primary key against an empty string is
// rejected by PostgreSQL.
type PayloadBatch struct {
	Table    string
	Column   string
	PKColumn string
	AfterPK  string
	Limit    int
}

// PayloadRow is one stored value returned by a PayloadBatchSource. Value may be
// NULL or empty ("not configured"); the classifier counts those as skipped and
// never treats them as ciphertext.
type PayloadRow struct {
	PK    string
	Value sql.NullString
}

// PayloadBatchSource is the minimal persistence port for the read-only payload
// scans. The SQL implementation lives in internal/infra/postgres so platform
// code never owns SQL; crypto keeps classification, queueing and cursor
// semantics. The constructor is wired in internal/bootstrap.
type PayloadBatchSource interface {
	FetchPayloadBatch(ctx context.Context, batch PayloadBatch) ([]PayloadRow, error)
}

// SealedPayloadWrite is one compare-and-swap update of a registered sealed
// payload column. Expected is the exact value that was read; the storage
// implementation must refuse the write when the stored value no longer equals
// it, so a concurrent writer is never overwritten. Sealed is the replacement
// token (plaintext values are never written).
type SealedPayloadWrite struct {
	Column   SealedPayloadColumn
	PK       string
	Expected string
	Sealed   string
}

// SealedPayloadBackfillSource extends the read-only payload port with the single
// write the sealed-payload backfill needs. The SQL (SELECT keyset page and
// UPDATE ... WHERE pk = $2 AND column = $3) lives in
// internal/infra/postgres/cryptopayload; crypto owns classification, AAD
// verification and idempotency.
type SealedPayloadBackfillSource interface {
	PayloadBatchSource
	UpdateSealedPayload(ctx context.Context, write SealedPayloadWrite) (int64, error)
}
