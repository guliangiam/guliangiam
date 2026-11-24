package webauthncredentials

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/webauthn"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow      = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testCredID    = "00000000-0000-0000-0000-000000000001" // naming-allow: legacy test var
	testUserID    = "00000000-0000-0000-0000-000000000002"
	credentialID  = []byte{0xDE, 0xAD, 0xBE, 0xEF}
	publicKey     = []byte{0xCA, 0xFE, 0xBA, 0xBE}
	transportJSON = []byte(`["hybrid","usb"]`)
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// credCols: 12 列
func credCols() []string {
	return []string{
		"id", "user_id", "credential_id", "public_key", "attestation_type",
		"transport", "sign_count", "name", "aaguid", "status", "created_at", "updated_at",
	}
}

func newCredRow() []driver.Value {
	return []driver.Value{
		testCredID, testUserID, credentialID, publicKey, "none", // naming-allow: legacy test var
		transportJSON, int64(42), "my-passkey", "00000000-0000-0000-0000-000000000003",
		"active", fixedNow, fixedNow,
	}
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertCredential)).
		WithArgs(
			testCredID, testUserID, credentialID, publicKey, "none", // naming-allow: legacy test var
			transportJSON, int64(42), "my-passkey",
			"00000000-0000-0000-0000-000000000003", "active", fixedNow, fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Credential{
		ID:              testCredID, // naming-allow: legacy test var
		UserID:          testUserID,
		CredentialID:    credentialID,
		PublicKey:       publicKey,
		AttestationType: "none",
		Transport:       []string{"hybrid", "usb"},
		SignCount:       42,
		Name:            "my-passkey",
		AAGUID:          "00000000-0000-0000-0000-000000000003",
		Status:          domain.CredentialStatusActive,
		CreatedAt:       fixedNow,
		UpdatedAt:       fixedNow,
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── GetByCredentialID ────────────────────────────────────────────

func TestRepository_GetByCredentialID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByCredentialID)).
		WithArgs(credentialID).
		WillReturnRows(sqlmock.NewRows(credCols()).AddRow(newCredRow()...))

	r := NewRepository(db)
	c, err := r.GetByCredentialID(context.Background(), credentialID)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, testCredID, c.ID) // naming-allow: legacy test var
	assert.Equal(t, credentialID, c.CredentialID)
	assert.Equal(t, []string{"hybrid", "usb"}, c.Transport)
	assert.Equal(t, domain.CredentialStatusActive, c.Status)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByCredentialID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByCredentialID)).
		WithArgs([]byte{0x00, 0x00}).
		WillReturnRows(sqlmock.NewRows(credCols()))

	r := NewRepository(db)
	c, err := r.GetByCredentialID(context.Background(), []byte{0x00, 0x00})
	assert.Error(t, err)
	assert.Nil(t, c)
}

// ── ListByUserID ─────────────────────────────────────────────────

func TestRepository_ListByUserID(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryListByUserID)).
		WithArgs(testUserID).
		WillReturnRows(sqlmock.NewRows(credCols()).AddRow(newCredRow()...))

	r := NewRepository(db)
	items, err := r.ListByUserID(context.Background(), testUserID)
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, testCredID, items[0].ID) // naming-allow: legacy test var
	assert.Equal(t, []string{"hybrid", "usb"}, items[0].Transport)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── UpdateSignCount ───────────────────────────────────────────────

func TestRepository_UpdateSignCount_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// time.Now() 会传入，我们用 AnyTime() 或直接期望有值
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateSignCount)).
		WithArgs(int64(100), sqlmock.AnyArg(), testCredID, int64(100)). // naming-allow: legacy test var
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpdateSignCount(context.Background(), testCredID, 100) // naming-allow: legacy test var
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateSignCount_NotMonotonic(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateSignCount)).
		WithArgs(int64(10), sqlmock.AnyArg(), testCredID, int64(10)). // naming-allow: legacy test var
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.UpdateSignCount(context.Background(), testCredID, 10) // naming-allow: legacy test var
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "monotonic")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteCredential)).
		WithArgs(testCredID). // naming-allow: legacy test var
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testCredID) // naming-allow: legacy test var
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
