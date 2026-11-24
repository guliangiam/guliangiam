package dpop

import (
	"context"

	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// MySQLReplayStore is a shared ReplayStore backed by MySQL.
// Mirrors internal/infra/postgres/dpop/replay.go — same interface, same semantics.
// Uses INSERT ... ON DUPLICATE KEY UPDATE jti = jti for atomic jti recording:
// first insert returns RowsAffected=1 (Check -> true, fresh),
// duplicate returns RowsAffected=0 (Check -> false, replay).
type MySQLReplayStore struct{ db mysqlshared.DBTX }

func NewMySQLReplayStore(db mysqlshared.DBTX) MySQLReplayStore {
	return MySQLReplayStore{db: db}
}

// Check returns true if this is the first time jti is seen (inserted).
// Returns false on conflict (replay detected) and on storage errors:
// a storage outage must reject DPoP proofs rather than silently disabling
// replay detection (security-model S-14).
func (s MySQLReplayStore) Check(ctx context.Context, jti string) (bool, error) {
	tag, err := s.db.ExecContext(ctx, qInsertJTI, jti)
	if err != nil {
		return false, apperrors.WrapUnavailable(err, "record DPoP proof jti")
	}
	n, err := tag.RowsAffected()
	if err != nil {
		return false, apperrors.WrapUnavailable(err, "read DPoP replay insert result")
	}
	return n == 1, nil
}
