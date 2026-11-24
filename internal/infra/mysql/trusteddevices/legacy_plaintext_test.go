package trusteddevices

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRevokeLegacyPlaintextUsesDigestRegexp(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectExec("UPDATE .*trusted_devices.*NOT REGEXP").
		WithArgs(fixedNow, fixedNow).
		WillReturnResult(sqlmock.NewResult(0, 2))
	repo := NewRepository(db)

	revoked, err := repo.RevokeLegacyPlaintext(context.Background(), fixedNow)
	if err != nil {
		t.Fatalf("RevokeLegacyPlaintext() error = %v", err)
	}
	if revoked != 2 {
		t.Fatalf("revoked = %d, want 2", revoked)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
