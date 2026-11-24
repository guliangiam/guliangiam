package bootstrap

import (
	"database/sql"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/cryptopayload"
	platformcrypto "github.com/guliangiam/guliangiam/internal/platform/crypto"
)

// NewPayloadBatchSource wires the PostgreSQL keyset scan used by the read-only
// crypto payload classification commands. platform/crypto owns the port,
// classification and cursor semantics; the SQL stays in infra.
func NewPayloadBatchSource(db *sql.DB) platformcrypto.PayloadBatchSource {
	return cryptopayload.NewSource(db)
}
