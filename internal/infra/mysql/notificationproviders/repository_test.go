package notificationproviders

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/notificationproviders"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNowNP   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgIDNP  = "00000000-0000-0000-0000-000000000001"
	testNPID     = "00000000-0000-0000-0000-000000000002"
	npConfigJSON = []byte(`{"botToken":"xxx"}`)
)

func newMockDBNP(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// npCols: 11 列
func npCols() []string {
	return []string{
		"id", "organization_id", "name", "provider_type", "webhook_url", "access_token",
		"config", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newNPRow(t *testing.T) []driver.Value {
	t.Helper()
	sealed, err := crypto.GlobalFieldEncryptor().EncryptField("token-123")
	require.NoError(t, err)
	return []driver.Value{
		testNPID, testOrgIDNP, "my-lark", "lark",
		"https://open.larksuite.com/webhook", sealed,
		npConfigJSON, true, fixedNowNP, fixedNowNP, int64(1),
	}
}

// envelopeSecretArg matches a stored envelope token and verifies it decrypts
// back to the expected plaintext.
type envelopeSecretArg struct {
	plaintext string
}

func (e envelopeSecretArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	ctx := crypto.AADContext{
		Table:   notificationProviderTable,
		Column:  notificationProviderColumnAccessToken,
		PKValue: testNPID,
	}
	opened, err := crypto.GlobalFieldEncryptor().DecryptEnvelopeFieldWithAAD(ctx, stored)
	return err == nil && opened == e.plaintext
}

// ── GetByID ────────────────────────────────────────────────────

func TestNP_GetByID_Found(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationProviderByID)).
		WithArgs(testNPID).
		WillReturnRows(sqlmock.NewRows(npCols()).AddRow(newNPRow(t)...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testNPID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "lark", p.ProviderType)
	assert.Equal(t, "token-123", p.AccessToken)
	assert.Equal(t, map[string]any{"botToken": "xxx"}, p.Config)
}

func TestNP_GetByID_PlaintextFailsClosed(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	row := newNPRow(t)
	row[5] = "legacy-plaintext"
	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationProviderByID)).
		WithArgs(testNPID).
		WillReturnRows(sqlmock.NewRows(npCols()).AddRow(row...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testNPID)
	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "encrypt-provider-secrets")
	assert.NotContains(t, err.Error(), "legacy-plaintext")
}

func TestNP_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetNotificationProviderByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(npCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, p)
}

// ── List ────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 9
func TestNP_List_WithEnabled(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgIDNP, testOrgIDNP, "kw", "kw", "kw", enabled, enabled}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountNotificationProviders)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListNotificationProviders)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(npCols()).AddRow(newNPRow(t)...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgIDNP, Keyword: "kw", IsEnabled: &enabled, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "token-123", items[0].AccessToken)
}

func TestNP_List_NilEnabled(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountNotificationProviders)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListNotificationProviders)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(npCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
}

// ── Create ──────────────────────────────────────────────────────

func TestNP_Create(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertNotificationProvider)).
		WithArgs(testNPID, testOrgIDNP, testNPID, "my-lark", "lark",
			"https://open.larksuite.com/webhook", envelopeSecretArg{plaintext: "token-123"},
			npConfigJSON, true, fixedNowNP, fixedNowNP, int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.NotificationProvider{
		ID: testNPID, OrganizationID: testOrgIDNP, Name: "my-lark", ProviderType: "lark",
		WebhookURL: "https://open.larksuite.com/webhook", AccessToken: "token-123",
		Config: map[string]any{"botToken": "xxx"}, IsEnabled: true,
		CreatedAt: fixedNowNP, UpdatedAt: fixedNowNP, Revision: int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ──────────────────────────────────────────────────────

func TestNP_Update(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	updatedConfig := []byte(`{"botToken":"new"}`)
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateNotificationProvider)).
		WithArgs("Updated", "dingtalk", "https://new", envelopeSecretArg{plaintext: "new-token"},
			updatedConfig, false, fixedNowNP, testNPID, int64(1), testOrgIDNP).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevisionNotificationProvider)).
		WithArgs(testNPID, testOrgIDNP).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	p := &domain.NotificationProvider{
		ID: testNPID, OrganizationID: testOrgIDNP, Name: "Updated", ProviderType: "dingtalk",
		WebhookURL: "https://new", AccessToken: "new-token",
		Config: map[string]any{"botToken": "new"}, IsEnabled: false,
		UpdatedAt: fixedNowNP, Revision: int64(2),
	}
	err := r.Update(context.Background(), p)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), p.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ──────────────────────────────────────────────────────

func TestNP_Delete(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteNotificationProvider)).
		WithArgs(testNPID, testOrgIDNP).WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testNPID, testOrgIDNP)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNP_Delete_ZeroRowsNotFound(t *testing.T) {
	db, mock := newMockDBNP(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteNotificationProvider)).
		WithArgs(testNPID, "other-org").WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testNPID, "other-org")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

var _ domain.Repository = Repository{}
