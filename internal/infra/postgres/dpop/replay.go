package dpop

import (
	"context"

	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// PostgresReplayStore is a shared ReplayStore backed by PostgreSQL.
// Uses INSERT ... ON CONFLICT DO NOTHING for atomic jti recording.
// The unique index uq_dpop_replay_jti ensures replays are caught across all
// replicas; background cleanup keeps the table bounded (ProofMaxAge window
// + skew buffer ≈ 3 minutes).
type PostgresReplayStore struct{ db shared.DBTX }

func NewPostgresReplayStore(db shared.DBTX) PostgresReplayStore {
	return PostgresReplayStore{db: db}
}

// Check returns true if this is the first time jti is seen (inserted).
// Returns false on conflict (replay detected).
func (s PostgresReplayStore) Check(ctx context.Context, jti string) (bool, error) {
	tag, err := s.db.Exec(ctx, qInsertJTI, jti)
	if err != nil {
		return false, apperrors.WrapUnavailable(err, "record DPoP proof jti")
	}
	return tag.RowsAffected() == 1, nil
}
