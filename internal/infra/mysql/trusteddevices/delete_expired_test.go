package trusteddevices

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDeleteExpiredBatchesByLimit(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectExec("DELETE FROM .*trusted_devices.*expires_at").
		WithArgs(fixedNow, 100).
		WillReturnResult(sqlmock.NewResult(0, 4))
	repo := NewRepository(db)

	deleted, err := repo.DeleteExpired(context.Background(), fixedNow, 100)
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}
	if deleted != 4 {
		t.Fatalf("deleted = %d, want 4", deleted)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestDeleteExpiredZeroLimitSkipsSQL(t *testing.T) {
	db, _ := newMockDB(t)
	repo := NewRepository(db)
	deleted, err := repo.DeleteExpired(context.Background(), time.Now(), 0)
	if err != nil || deleted != 0 {
		t.Fatalf("DeleteExpired(zero limit) = (%d, %v), want (0, nil)", deleted, err)
	}
}
