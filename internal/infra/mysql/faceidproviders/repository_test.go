package faceidproviders

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/faceidproviders"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNowFID   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgIDFID  = "00000000-0000-0000-0000-000000000001"
	testFIDID     = "00000000-0000-0000-0000-000000000002"
	fidConfigJSON = []byte(`{"region":"cn-hangzhou"}`)
)

func newMockDBFID(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// faceIDCols: 12 列
func faceIDCols() []string {
	return []string{
		"id", "organization_id", "name", "provider_type", "access_key_id", "access_key_secret",
		"endpoint", "config", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newFaceIDRow() []driver.Value {
	return []driver.Value{
		testFIDID, testOrgIDFID, "my-faceid", "Aliyun", "AKID123", "AKS456",
		"https://faceid.aliyun.com", fidConfigJSON, true, fixedNowFID, fixedNowFID, int64(1),
	}
}

func newFaceIDRowWithSecret(secret string) []driver.Value {
	row := newFaceIDRow()
	row[5] = secret
	return row
}

// sealedFaceIdSecret asserts that the value written for access_key_secret is a
// v2 envelope bound to the row id and opens back to the expected plaintext.
type sealedFaceIdSecret struct {
	plaintext  string
	providerID string
}

func (a sealedFaceIdSecret) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	header, _, _ := strings.Cut(stored, ":")
	if !strings.HasSuffix(header, "-aad") {
		return false
	}
	opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(
		crypto.AADContext{Table: faceIdProviderTable, Column: faceIdProviderColumnAccessKeySecret, PKValue: a.providerID},
		stored,
	)
	return err == nil && opened == a.plaintext
}

// ── GetByID ────────────────────────────────────────────────────

func TestFID_GetByID_Found(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetFaceIdProviderByID)).
		WithArgs(testFIDID, testOrgIDFID).
		WillReturnRows(sqlmock.NewRows(faceIDCols()).AddRow(newFaceIDRow()...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testFIDID, testOrgIDFID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "Aliyun", p.ProviderType)
	assert.Equal(t, map[string]any{"region": "cn-hangzhou"}, p.Config)
	assert.Equal(t, "AKS456", p.AccessKeySecret)
	assert.True(t, p.SecretNeedsRotation, "legacy plaintext must be marked for rotation")
}

func TestFID_GetByID_SealedSecretDecrypts(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(
		crypto.AADContext{Table: faceIdProviderTable, Column: faceIdProviderColumnAccessKeySecret, PKValue: testFIDID},
		"AKS456",
	)
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(queryGetFaceIdProviderByID)).
		WithArgs(testFIDID, testOrgIDFID).
		WillReturnRows(sqlmock.NewRows(faceIDCols()).AddRow(newFaceIDRowWithSecret(sealed)...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testFIDID, testOrgIDFID)
	require.NoError(t, err)
	assert.Equal(t, "AKS456", p.AccessKeySecret)
	assert.False(t, p.SecretNeedsRotation)
}

func TestFID_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetFaceIdProviderByID)).
		WithArgs("bad", testOrgIDFID).
		WillReturnRows(sqlmock.NewRows(faceIDCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), "bad", testOrgIDFID)
	assert.Error(t, err)
	assert.Nil(t, p)
}

// ── List ────────────────────────────────────────────────────────

// countArgs: org_id×2 + is_enabled×2 = 4
// listArgs: 6
func TestFID_List_WithEnabled(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgIDFID, testOrgIDFID, enabled, enabled}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountFaceIdProviders)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListFaceIdProviders)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(faceIDCols()).AddRow(newFaceIDRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgIDFID, IsEnabled: &enabled, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

func TestFID_List_NilEnabled(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountFaceIdProviders)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListFaceIdProviders)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(faceIDCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
}

// ── Create ──────────────────────────────────────────────────────

func TestFID_Create(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertFaceIdProvider)).
		WithArgs(testFIDID, testOrgIDFID, "my-faceid", "Aliyun", "AKID123",
			sealedFaceIdSecret{plaintext: "AKS456", providerID: testFIDID},
			"https://faceid.aliyun.com", fidConfigJSON, true, fixedNowFID, fixedNowFID, int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.FaceIdProvider{
		ID: testFIDID, OrganizationID: testOrgIDFID, Name: "my-faceid", ProviderType: "Aliyun",
		AccessKeyID: "AKID123", AccessKeySecret: "AKS456", Endpoint: "https://faceid.aliyun.com",
		Config: map[string]any{"region": "cn-hangzhou"}, IsEnabled: true,
		CreatedAt: fixedNowFID, UpdatedAt: fixedNowFID, Revision: int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ──────────────────────────────────────────────────────

func TestFID_Update(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	updatedConfig := []byte(`{"region":"cn-shanghai"}`)
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateFaceIdProvider)).
		WithArgs("Updated", "Aliyun", "NewAK",
			sealedFaceIdSecret{plaintext: "NewAS", providerID: testFIDID},
			"https://new.endpoint", updatedConfig, false, fixedNowFID, testFIDID, int64(1), testOrgIDFID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevisionFaceIdProvider)).
		WithArgs(testFIDID, testOrgIDFID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	p := &domain.FaceIdProvider{
		ID: testFIDID, OrganizationID: testOrgIDFID, Name: "Updated", ProviderType: "Aliyun",
		AccessKeyID: "NewAK", AccessKeySecret: "NewAS", Endpoint: "https://new.endpoint",
		Config: map[string]any{"region": "cn-shanghai"}, IsEnabled: false,
		UpdatedAt: fixedNowFID, Revision: int64(2),
	}
	err := r.Update(context.Background(), p)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), p.Revision)
	assert.Contains(t, queryUpdateFaceIdProvider, "organization_id = ?", "update must stay tenant scoped")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ──────────────────────────────────────────────────────

func TestFID_Delete(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteFaceIdProvider)).
		WithArgs(testFIDID, testOrgIDFID).WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testFIDID, testOrgIDFID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFID_Delete_ForeignOrganizationIsNotFound(t *testing.T) {
	db, mock := newMockDBFID(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteFaceIdProvider)).
		WithArgs(testFIDID, "other-org").WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testFIDID, "other-org")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

var _ domain.Repository = Repository{}
