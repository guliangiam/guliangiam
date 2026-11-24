package resources

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/resources"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow9  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID9 = "00000000-0000-0000-0000-000000000001"
	testAppID9 = "00000000-0000-0000-0000-000000000002"
	testResID  = "00000000-0000-0000-0000-000000000007"
)

func newResMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// resCols: 13 列
func resCols() []string {
	return []string{
		"id", "organization_id", "application_id", "name", "display_name", "type", "tag",
		"file_path", "file_size", "mime_type", "created_at", "updated_at", "revision",
	}
}

func newResRow() []driver.Value {
	return []driver.Value{
		testResID, testOrgID9, testAppID9, "asset-001", "Asset 001", "image", "tag-a",
		"/files/img.png", int64(10240), "image/png", fixedNow9, fixedNow9, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetResourceByID)).
		WithArgs(testResID, testOrgID9).
		WillReturnRows(sqlmock.NewRows(resCols()).AddRow(newResRow()...))

	r := NewRepository(db)
	res, err := r.GetByID(context.Background(), testResID, testOrgID9)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, testResID, res.ID)
	assert.Equal(t, "image", res.Type)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetResourceByID)).
		WithArgs("bad", "org-1").
		WillReturnRows(sqlmock.NewRows(resCols()))

	r := NewRepository(db)
	res, err := r.GetByID(context.Background(), "bad", "org-1")
	assert.Error(t, err)
	assert.Nil(t, res)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + app_id×2 + keyword×3 + type×2 + tag×2 = 11
// listArgs: 11 + limit + offset = 13
func TestRepository_List_WithFilters(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{testOrgID9, testOrgID9, testAppID9, testAppID9, "kw", "kw", "kw", "image", "image", "tag-a", "tag-a"}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountResources)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListResources)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(resCols()).AddRow(newResRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID9,
		ApplicationID:  testAppID9,
		Keyword:        "kw",
		Type:           "image",
		Tag:            "tag-a",
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_Empty(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountResources)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListResources)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(resCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertResource)).
		WithArgs(
			testResID, testOrgID9, testAppID9, "asset-001", "Asset 001", "image", "tag-a",
			"/files/img.png", int64(10240), "image/png", fixedNow9, fixedNow9, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Resource{
		ID:             testResID,
		OrganizationID: testOrgID9,
		ApplicationID:  testAppID9,
		Name:           "asset-001",
		DisplayName:    "Asset 001",
		Type:           "image",
		Tag:            "tag-a",
		FilePath:       "/files/img.png",
		FileSize:       int64(10240),
		MimeType:       "image/png",
		CreatedAt:      fixedNow9,
		UpdatedAt:      fixedNow9,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	// Step 1: UPDATE ... WHERE id=? AND revision=? AND organization_id=?
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateResource)).
		WithArgs(
			"Updated", "video", "tag-b", "/files/video.mp4",
			int64(51200), "video/mp4", fixedNow9, testResID, int64(1), testOrgID9,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 2: SELECT revision
	mock.ExpectQuery(regexp.QuoteMeta(querySelectResourceRevision)).
		WithArgs(testResID, testOrgID9).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	res := &domain.Resource{
		ID:             testResID,
		OrganizationID: testOrgID9,
		DisplayName:    "Updated",
		Type:           "video",
		Tag:            "tag-b",
		FilePath:       "/files/video.mp4",
		FileSize:       int64(51200),
		MimeType:       "video/mp4",
		UpdatedAt:      fixedNow9,
		Revision:       int64(1),
	}
	err := r.Update(context.Background(), res)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), res.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Conflict(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateResource)).
		WithArgs(
			"Updated", "video", "tag-b", "/files/video.mp4",
			int64(51200), "video/mp4", fixedNow9, testResID, int64(1), testOrgID9,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	res := &domain.Resource{
		ID:             testResID,
		OrganizationID: testOrgID9,
		DisplayName:    "Updated",
		Type:           "video",
		Tag:            "tag-b",
		FilePath:       "/files/video.mp4",
		FileSize:       int64(51200),
		MimeType:       "video/mp4",
		UpdatedAt:      fixedNow9,
		Revision:       int64(1),
	}
	err := r.Update(context.Background(), res)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteResource)).
		WithArgs(testResID, testOrgID9).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testResID, testOrgID9)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newResMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteResource)).
		WithArgs(testResID, testOrgID9).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testResID, testOrgID9)
	assertNotFoundCode(t, err)
}

func assertNotFoundCode(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *apperrors.Error, got %v", err)
	}
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// 编译期接口断言
var _ domain.Repository = Repository{}
