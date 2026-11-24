package trusteddevices

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/trusteddevices"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var fixedNow = time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() = %v", err)
	}
	return db, mock
}

// TestRevokeByUser_ScopesToTenantAndReportsRows 与 Postgres 实现对齐：
// 参数顺序 (revoked_at, updated_at, organization_id, user_id) 与 SQL 一致。
func TestRevokeByUser_ScopesToTenantAndReportsRows(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeByUser)).
		WithArgs(fixedNow, fixedNow, "org-1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 2))

	affected, err := NewRepository(db).RevokeByUser(context.Background(), "org-1", "user-1", fixedNow)
	if err != nil {
		t.Fatalf("RevokeByUser() error = %v", err)
	}
	if affected != 2 {
		t.Fatalf("RowsAffected = %d, want 2", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet sqlmock expectations: %v", err)
	}
}

func TestRevokeByUser_ZeroRowsIsNotAnError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryRevokeByUser)).
		WithArgs(fixedNow, fixedNow, "org-1", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	affected, err := NewRepository(db).RevokeByUser(context.Background(), "org-1", "user-1", fixedNow)
	if err != nil {
		t.Fatalf("RevokeByUser() error = %v", err)
	}
	if affected != 0 {
		t.Fatalf("RowsAffected = %d, want 0 (idempotent re-revoke)", affected)
	}
}

func TestGetByID_NotFoundMapsToNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs("device-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := NewRepository(db).GetByID(context.Background(), "device-1")
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("GetByID() error = %v, want NotFound", err)
	}
}

func TestCreate_RejectsPlaintextCredentialBeforeSQL(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	err := NewRepository(db).Create(context.Background(), &domain.TrustedDevice{
		ID:                 "device-1",
		OrganizationID:     "org-1",
		UserID:             "user-1",
		DeviceCredentialID: "plaintext-credential",
	})

	var appErr *apperrors.Error
	if !errors.As(err, &appErr) || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("Create() error = %v, want UnprocessableEntity", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("validation must run before SQL: %v", err)
	}
}
