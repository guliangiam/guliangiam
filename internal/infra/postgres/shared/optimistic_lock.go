package shared

import (
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var ErrConcurrentModification = apperrors.NewConflict("concurrent modification detected: revision mismatch")

func CheckAffectedRows(rowsAffected int64) error {
	if rowsAffected == 0 {
		return apperrors.NewConflict("revision conflict: record was modified by another transaction")
	}
	return nil
}
