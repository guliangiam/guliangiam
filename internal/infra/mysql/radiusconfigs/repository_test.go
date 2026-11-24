package radiusconfigs

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusconfigs"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNowRC  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgIDRC = "00000000-0000-0000-0000-000000000001"
	testRCID    = "00000000-0000-0000-0000-000000000002"
)

func newMockDBRC(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// rcCols: 11 列
func rcCols() []string {
	return []string{
		"id", "organization_id", "name", "host", "port", "shared_secret",
		"auth_protocol", "is_enabled", "created_at", "updated_at", "revision",
	}
}

const rcPlaintextSecret = "secret-key-123"

// sealRCSecret 用生产 mapper 生成密文，保证断言的是真实 envelope 令牌。
func sealRCSecret(t *testing.T, plaintext string) string {
	t.Helper()
	sealed, err := radiusConfigEncryptedMapper.EncryptColumn(radiusConfigAAD(testRCID), plaintext)
	assert.NoError(t, err)
	return sealed
}

// sealedRCSecret 断言写入值是 envelope 密文，且解回指定的明文。
// 密文带随机 DEK/nonce，无法在期望值里预先固定。
type sealedRCSecret struct{ plaintext string }

func (a sealedRCSecret) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok {
		return false
	}
	opened, err := radiusConfigEncryptedMapper.DecryptColumn(radiusConfigAAD(testRCID), stored)
	return err == nil && opened == a.plaintext
}

func newRCRow(t *testing.T) []driver.Value {
	return []driver.Value{
		testRCID, testOrgIDRC, "my-radius", "10.0.0.1", 1812, sealRCSecret(t, rcPlaintextSecret),
		"PAP", true, fixedNowRC, fixedNowRC, int64(1),
	}
}

// ── GetByID ────────────────────────────────────────────────────

func TestRC_GetByID_Found(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRadiusConfigByID)).
		WithArgs(testRCID, testOrgIDRC).
		WillReturnRows(sqlmock.NewRows(rcCols()).AddRow(newRCRow(t)...))

	r := NewRepository(db)
	c, err := r.GetByID(context.Background(), testRCID, testOrgIDRC)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, "PAP", c.AuthProtocol)
	assert.Equal(t, 1812, c.Port)
	assert.Equal(t, rcPlaintextSecret, c.SharedSecret, "读取时应解密回明文")
}

func TestRC_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRadiusConfigByID)).
		WithArgs("bad", testOrgIDRC).
		WillReturnRows(sqlmock.NewRows(rcCols()))

	r := NewRepository(db)
	c, err := r.GetByID(context.Background(), "bad", testOrgIDRC)
	assert.Error(t, err)
	assert.Nil(t, c)
}

// ── List ────────────────────────────────────────────────────────

// countArgs: org_id×2 + is_enabled×2 + auth_protocol×2 = 6
// listArgs: 8
func TestRC_List_WithAllFilters(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgIDRC, testOrgIDRC, enabled, enabled, "PAP", "PAP"}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRadiusConfigs)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRadiusConfigs)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(rcCols()).AddRow(newRCRow(t)...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgIDRC, IsEnabled: &enabled, AuthProtocol: "PAP", Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

func TestRC_List_NilEnabledEmptyProtocol(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", nil, nil, "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRadiusConfigs)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRadiusConfigs)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(rcCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
}

// ── Create ──────────────────────────────────────────────────────

func TestRC_Create(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertRadiusConfig)).
		WithArgs(testRCID, testOrgIDRC, "my-radius", "10.0.0.1", 1812, sealedRCSecret{rcPlaintextSecret},
			"PAP", true, fixedNowRC, fixedNowRC, int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	entity := &domain.RadiusConfig{
		ID: testRCID, OrganizationID: testOrgIDRC, Name: "my-radius",
		Host: "10.0.0.1", Port: 1812, SharedSecret: rcPlaintextSecret,
		AuthProtocol: "PAP", IsEnabled: true,
		CreatedAt: fixedNowRC, UpdatedAt: fixedNowRC, Revision: int64(1),
	}
	err := r.Create(context.Background(), entity)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
	assert.Equal(t, rcPlaintextSecret, entity.SharedSecret, "加密不得修改 domain 对象")
}

// ── Update ──────────────────────────────────────────────────────

func TestRC_Update(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateRadiusConfig)).
		WithArgs("Updated", "10.0.0.2", 1813, sealedRCSecret{"new-secret"}, "CHAP", false, fixedNowRC, testRCID, int64(1), testOrgIDRC).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectRevisionRadiusConfig)).
		WithArgs(testRCID, testOrgIDRC).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	c := &domain.RadiusConfig{
		ID: testRCID, OrganizationID: testOrgIDRC, Name: "Updated", Host: "10.0.0.2", Port: 1813,
		SharedSecret: "new-secret", AuthProtocol: "CHAP", IsEnabled: false,
		UpdatedAt: fixedNowRC, Revision: int64(2),
	}
	err := r.Update(context.Background(), c)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), c.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ──────────────────────────────────────────────────────

func TestRC_Delete(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRadiusConfig)).
		WithArgs(testRCID, testOrgIDRC).WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRCID, testOrgIDRC)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRC_Delete_ForeignOrganizationIsNotFound(t *testing.T) {
	db, mock := newMockDBRC(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRadiusConfig)).
		WithArgs(testRCID, "other-org").WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRCID, "other-org")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

var _ domain.Repository = Repository{}
