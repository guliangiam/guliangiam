package idv

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/idv"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testVerifID  = "00000000-0000-0000-0000-000000000001"
	testOrgID    = "00000000-0000-0000-0000-000000000002"
	testUserID   = "00000000-0000-0000-0000-000000000003"
	testProvider = "jumio"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── 列定义（21 列，与 scanOne 的 Scan 参数顺序一致） ──

func verifCols() []string {
	return []string{
		"id", "organization_id", "user_id", "provider", "status",
		"redirect_url", "session_token", "provider_ref",
		"first_name", "last_name", "date_of_birth", "nationality",
		"id_type", "id_number", "confidence", "reason",
		"raw_result", "initiated_at", "completed_at", "created_at", "updated_at",
	}
}

func newVerifRow() []driver.Value {
	redirectURL := "https://example.com/redirect"
	sessionToken := "sess-token-abc"
	providerRef := "jumio-ref-123"
	firstName := "John"
	lastName := "Doe"
	dob := "1990-01-01"
	nationality := "CN"
	idType := "passport"
	idNumber := "AB1234567"
	confidence := 0.95
	reason := "matched"
	raw := json.RawMessage(`{"jumio":"ok"}`)
	return []driver.Value{
		testVerifID, testOrgID, testUserID, testProvider, "approved",
		&redirectURL, &sessionToken, &providerRef,
		&firstName, &lastName, &dob, &nationality,
		&idType, &idNumber, &confidence, &reason,
		raw, fixedNow, fixedNow, fixedNow, fixedNow,
	}
}

func newVerifRowWithSessionToken(token *string) []driver.Value {
	row := newVerifRow()
	row[6] = token
	return row
}

func Test_colCounts_Idv(t *testing.T) {
	assert.Equal(t, 21, len(verifCols()))
	assert.Equal(t, 21, len(newVerifRow()))
}

// ── GetByID ──────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testVerifID, testOrgID).
		WillReturnRows(sqlmock.NewRows(verifCols()).AddRow(newVerifRow()...))

	r := NewRepository(db)
	v, err := r.GetByID(context.Background(), testVerifID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.Equal(t, testVerifID, v.ID)
	assert.Equal(t, testOrgID, v.OrganizationID)
	assert.Equal(t, testUserID, *v.UserID)
	assert.Equal(t, domain.StatusApproved, v.Status)
	assert.Equal(t, testProvider, v.Provider)
	assert.Equal(t, "John", *v.FirstName)
	assert.Equal(t, "AB1234567", *v.IDNumber)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs("unknown-id", testOrgID).
		WillReturnRows(sqlmock.NewRows(verifCols()))

	r := NewRepository(db)
	v, err := r.GetByID(context.Background(), "unknown-id", testOrgID)
	assert.Error(t, err)
	assert.Nil(t, v)
}

func TestRepository_GetByID_SealedSessionTokenDecrypts(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(identityVerificationAAD(testVerifID), "sess-token-abc")
	require.NoError(t, err)
	sealedPtr := sealed

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testVerifID, testOrgID).
		WillReturnRows(sqlmock.NewRows(verifCols()).AddRow(newVerifRowWithSessionToken(&sealedPtr)...))

	r := NewRepository(db)
	v, err := r.GetByID(context.Background(), testVerifID, testOrgID)
	require.NoError(t, err)
	require.NotNil(t, v.SessionToken)
	assert.Equal(t, "sess-token-abc", *v.SessionToken)
}

func TestRepository_GetByID_RejectsForeignRowBinding(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(identityVerificationAAD("other-verification"), "sess-token-abc")
	require.NoError(t, err)
	sealedPtr := sealed

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testVerifID, testOrgID).
		WillReturnRows(sqlmock.NewRows(verifCols()).AddRow(newVerifRowWithSessionToken(&sealedPtr)...))

	r := NewRepository(db)
	_, err = r.GetByID(context.Background(), testVerifID, testOrgID)
	require.Error(t, err)
	assert.ErrorIs(t, err, crypto.ErrAADMismatch)
}

func TestRepository_GetByID_NullSessionTokenStaysNil(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByID)).
		WithArgs(testVerifID, testOrgID).
		WillReturnRows(sqlmock.NewRows(verifCols()).AddRow(newVerifRowWithSessionToken(nil)...))

	r := NewRepository(db)
	v, err := r.GetByID(context.Background(), testVerifID, testOrgID)
	require.NoError(t, err)
	assert.Nil(t, v.SessionToken)
}

// ── GetByProviderRef ─────────────────────────────────────────────

func TestRepository_GetByProviderRef_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByProviderRef)).
		WithArgs(testProvider, "jumio-ref-123").
		WillReturnRows(sqlmock.NewRows(verifCols()).AddRow(newVerifRow()...))

	r := NewRepository(db)
	v, err := r.GetByProviderRef(context.Background(), testProvider, "jumio-ref-123")
	assert.NoError(t, err)
	assert.NotNil(t, v)
	assert.Equal(t, testVerifID, v.ID)
}

func TestRepository_GetByProviderRef_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetByProviderRef)).
		WithArgs(testProvider, "no-such-ref").
		WillReturnRows(sqlmock.NewRows(verifCols()))

	r := NewRepository(db)
	v, err := r.GetByProviderRef(context.Background(), testProvider, "no-such-ref")
	assert.Error(t, err)
	assert.Nil(t, v)
}

// ── List ─────────────────────────────────────────────────────────
// queryList 参数：orgID, userFilter×2, providerFilter×2, statusFilter×2, limit, offset = 9
// queryCount 参数：orgID, userFilter×2, providerFilter×2, statusFilter×2 = 7

func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 默认 ListQuery：nil userID → userFilter=""，nil provider → providerFilter=""，nil status → statusFilter=nil
	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs(testOrgID, "", "", "", "", nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows(verifCols()).AddRow(newVerifRow()...))

	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs(testOrgID, "", "", "", "", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, testVerifID, items[0].ID)
}

func TestRepository_List_WithFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	status := domain.StatusApproved
	userID := testUserID

	mock.ExpectQuery(regexp.QuoteMeta(queryList)).
		WithArgs(testOrgID, userID, userID, testProvider, testProvider, string(status), string(status), 10, 20).
		WillReturnRows(sqlmock.NewRows(verifCols()))

	mock.ExpectQuery(regexp.QuoteMeta(queryCount)).
		WithArgs(testOrgID, userID, userID, testProvider, testProvider, string(status), string(status)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
		UserID:         &userID,
		Provider:       &testProvider,
		Status:         &status,
		Page:           3,
		PageSize:       10,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── Create ───────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsert)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	v := &domain.Verification{
		ID:             testVerifID,
		OrganizationID: testOrgID,
		Provider:       testProvider,
		Status:         domain.StatusPending,
		InitiatedAt:    fixedNow,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
	}
	err := r.Create(context.Background(), v)
	assert.NoError(t, err)
}

// sealedSessionTokenArg matches the session_token argument: it must be a v2
// envelope bound to the verification id and open back to the plaintext.
type sealedSessionTokenArg struct {
	plaintext      string
	verificationID string
}

func (a sealedSessionTokenArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(
		identityVerificationAAD(a.verificationID), stored)
	return err == nil && opened == a.plaintext
}

func TestRepository_Create_SealsSessionToken(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsert)).
		WithArgs(
			testVerifID, testOrgID, nil, testProvider, sqlmock.AnyArg(),
			nil, sealedSessionTokenArg{plaintext: "sess-token-abc", verificationID: testVerifID}, nil,
			nil, nil, nil, nil,
			nil, nil, nil, nil,
			sqlmock.AnyArg(), fixedNow, nil, fixedNow, fixedNow,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	raw := json.RawMessage(`{}`)
	v := &domain.Verification{
		ID:             testVerifID,
		OrganizationID: testOrgID,
		Provider:       testProvider,
		Status:         domain.StatusPending,
		SessionToken:   stringPtr("sess-token-abc"),
		RawResult:      raw,
		InitiatedAt:    fixedNow,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
	}
	require.NoError(t, r.Create(context.Background(), v))
	require.NoError(t, mock.ExpectationsWereMet())
}

func stringPtr(value string) *string { return &value }

func TestRepository_Create_NilRawResult(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// 最后一个参数（raw_result）应为空对象
	mock.ExpectExec(regexp.QuoteMeta(queryInsert)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	v := &domain.Verification{
		ID:             testVerifID,
		OrganizationID: testOrgID,
		Provider:       testProvider,
		Status:         domain.StatusPending,
		InitiatedAt:    fixedNow,
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		RawResult:      nil,
	}
	err := r.Create(context.Background(), v)
	assert.NoError(t, err)
}

// ── Update ───────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdate)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(),
			testVerifID, testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	v := &domain.Verification{
		ID:             testVerifID,
		OrganizationID: testOrgID,
		Status:         domain.StatusApproved,
		UpdatedAt:      fixedNow,
	}
	err := r.Update(context.Background(), v)
	assert.NoError(t, err)
}

// ── Delete ───────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDelete)).
		WithArgs(testVerifID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testVerifID, testOrgID)
	assert.NoError(t, err)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
