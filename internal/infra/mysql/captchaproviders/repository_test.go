package captchaproviders

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/captchaproviders"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow2         = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID2        = "00000000-0000-0000-0000-000000000001"
	testCaptchaID     = "00000000-0000-0000-0000-000000000002"
	captchaConfigJSON = []byte(`{"siteKey":"test","threshold":0.5}`)
)

func newMockDB2(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// captchaCols: 11 列
func captchaCols() []string {
	return []string{
		"id", "organization_id", "name", "provider_type", "site_key", "secret_key",
		"config", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newCaptchaRow(t *testing.T) []driver.Value {
	return []driver.Value{
		testCaptchaID, testOrgID2, "my-captcha", "recaptcha", "site-key-123", sealCaptchaSecret(t, captchaPlaintextSecret),
		captchaConfigJSON, true, fixedNow2, fixedNow2, int64(1),
	}
}

const captchaPlaintextSecret = "secret-key-456"

// sealCaptchaSecret 用生产 mapper 生成密文，保证断言的是真实 envelope 令牌。
func sealCaptchaSecret(t *testing.T, plaintext string) string {
	t.Helper()
	sealed, err := captchaProviderEncryptedMapper.EncryptColumn(captchaProviderAAD(testCaptchaID), plaintext)
	assert.NoError(t, err)
	return sealed
}

// sealedCaptchaSecret 断言写入值是 envelope 密文，且解回指定的明文。
// 密文带随机 DEK/nonce，无法在期望值里预先固定。
type sealedCaptchaSecret struct{ plaintext string }

func (a sealedCaptchaSecret) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok {
		return false
	}
	opened, err := captchaProviderEncryptedMapper.DecryptColumn(captchaProviderAAD(testCaptchaID), stored)
	return err == nil && opened == a.plaintext
}

// ── GetByID ────────────────────────────────────────────────────

func TestCaptcha_GetByID_Found(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetCaptchaProviderByID)).
		WithArgs(testCaptchaID, testOrgID2).
		WillReturnRows(sqlmock.NewRows(captchaCols()).AddRow(newCaptchaRow(t)...))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testCaptchaID, testOrgID2)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, testCaptchaID, p.ID)
	assert.Equal(t, "recaptcha", p.ProviderType)
	assert.True(t, p.IsEnabled)
	assert.Equal(t, captchaPlaintextSecret, p.SecretKey, "读取时应解密回明文")
	assert.Equal(t, map[string]any{"siteKey": "test", "threshold": 0.5}, p.Config)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptcha_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetCaptchaProviderByID)).
		WithArgs("bad", testOrgID2).
		WillReturnRows(sqlmock.NewRows(captchaCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), "bad", testOrgID2)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestCaptcha_GetByID_ForeignOrganizationNotFound(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetCaptchaProviderByID)).
		WithArgs(testCaptchaID, "foreign-org").
		WillReturnRows(sqlmock.NewRows(captchaCols()))

	r := NewRepository(db)
	p, err := r.GetByID(context.Background(), testCaptchaID, "foreign-org")
	assert.Error(t, err)
	assert.Nil(t, p)
}

// ── List ────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 9
func TestCaptcha_List_WithEnabled(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgID2, testOrgID2, "kw", "kw", "kw", enabled, enabled}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountCaptchaProviders)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListCaptchaProviders)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(captchaCols()).AddRow(newCaptchaRow(t)...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID2,
		Keyword:        "kw",
		IsEnabled:      &enabled,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptcha_List_NilEnabled(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountCaptchaProviders)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListCaptchaProviders)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(captchaCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ──────────────────────────────────────────────────────

func TestCaptcha_Create(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertCaptchaProvider)).
		WithArgs(
			testCaptchaID, testOrgID2, "my-captcha", "recaptcha",
			"site-key-123", sealedCaptchaSecret{captchaPlaintextSecret}, captchaConfigJSON, true,
			fixedNow2, fixedNow2, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	entity := &domain.CaptchaProvider{
		ID:             testCaptchaID,
		OrganizationID: testOrgID2,
		Name:           "my-captcha",
		ProviderType:   "recaptcha",
		SiteKey:        "site-key-123",
		SecretKey:      captchaPlaintextSecret,
		Config:         map[string]any{"siteKey": "test", "threshold": 0.5},
		IsEnabled:      true,
		CreatedAt:      fixedNow2,
		UpdatedAt:      fixedNow2,
		Revision:       int64(1),
	}
	err := r.Create(context.Background(), entity)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, captchaPlaintextSecret, entity.SecretKey, "加密不得修改 domain 对象")
}

// ── Update ──────────────────────────────────────────────────────

func TestCaptcha_Update(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	updatedConfig := []byte(`{"siteKey":"updated"}`)
	// UPDATE ... WHERE id=? AND revision=? AND organization_id=? → revision 传 revision-1 = 1
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateCaptchaProvider)).
		WithArgs("Updated", "hcaptcha", "new-site", sealedCaptchaSecret{"new-secret"}, updatedConfig, false, fixedNow2, testCaptchaID, int64(1), testOrgID2).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// SELECT revision = 2
	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevisionCaptchaProvider)).
		WithArgs(testCaptchaID, testOrgID2).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	p := &domain.CaptchaProvider{
		ID:             testCaptchaID,
		OrganizationID: testOrgID2,
		Name:           "Updated",
		ProviderType:   "hcaptcha",
		SiteKey:        "new-site",
		SecretKey:      "new-secret",
		Config:         map[string]any{"siteKey": "updated"},
		IsEnabled:      false,
		UpdatedAt:      fixedNow2,
		Revision:       int64(2), // Update 时 revision 已经是期望的新值？不，看上面 applications 的模式...
		// 等下 PG 版 Update 时 revision 是传入的 revision-1 来 WHERE。我们传 2，WHERE revision=1
		// 然后 SET revision = revision + 1 → 2
		// 再 SELECT revision → 2，赋值给 provider.Revision
	}
	err := r.Update(context.Background(), p)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), p.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestCaptcha_Update_ZeroAffectedRowsIsConflict pin the 0-row semantics: a
// scoped read already rejected foreign tenants, so an UPDATE that matches no
// row can only be a revision mismatch and stays a conflict (not NotFound).
func TestCaptcha_Update_ZeroAffectedRowsIsConflict(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateCaptchaProvider)).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.CaptchaProvider{
		ID: testCaptchaID, OrganizationID: "foreign-org", Revision: 2,
	})
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ──────────────────────────────────────────────────────

func TestCaptcha_Delete(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteCaptchaProvider)).
		WithArgs(testCaptchaID, testOrgID2).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testCaptchaID, testOrgID2)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestCaptcha_Delete_ForeignOrganizationNotFound(t *testing.T) {
	db, mock := newMockDB2(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteCaptchaProvider)).
		WithArgs(testCaptchaID, "foreign-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testCaptchaID, "foreign-org")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
