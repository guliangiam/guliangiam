package certs

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID  = "00000000-0000-0000-0000-000000000001"
	testCertID = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// certCols: 25 列
func certCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "scope", "type", "crypto_algorithm", "bit_size", "expire_in_years",
		"expire_time", "certificate", "private_key",
		"authority_public_key", "authority_root_public_key",
		"acme_issuer", "acme_endpoint",
		"provider", "account", "access_key", "access_secret", "domain",
		"domain_expire_time",
		"created_at", "updated_at", "revision",
	}
}

const (
	certPlaintextKey    = "-----BEGIN KEY-----"
	certPlaintextSecret = "secret-456"
)

// sealCertSecrets 用生产 mapper 生成密文，保证断言的是真实 envelope 令牌。
func sealCertSecrets(t *testing.T, privateKey, accessSecret string) (string, string) {
	t.Helper()
	sealedKey, err := certEncryptedMapper.EncryptColumn(certAAD(testCertID, certColumnPrivateKey), privateKey)
	assert.NoError(t, err)
	sealedSecret, err := certEncryptedMapper.EncryptColumn(certAAD(testCertID, certColumnAccessSecret), accessSecret)
	assert.NoError(t, err)
	return sealedKey, sealedSecret
}

// sealedCertSecret 断言写入值是 envelope 密文，且解回指定的明文。
// 密文带随机 DEK/nonce，无法在期望值里预先固定。
type sealedCertSecret struct {
	plaintext string
	column    string
}

func (a sealedCertSecret) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok {
		return false
	}
	opened, err := certEncryptedMapper.DecryptColumn(certAAD(testCertID, a.column), stored)
	return err == nil && opened == a.plaintext
}

func newCertRow(t *testing.T) []driver.Value {
	domainJSON, _ := json.Marshal([]string{"example.com", "api.example.com"})
	sealedKey, sealedSecret := sealCertSecrets(t, certPlaintextKey, certPlaintextSecret)
	return []driver.Value{
		testCertID, testOrgID, "my-cert", "My Certificate", "global", "x509", "RSA", 2048, 1,
		fixedNow, "-----BEGIN CERT-----", sealedKey,
		"pub-key", "root-pub-key",
		"letsencrypt", "https://acme-v02.api.letsencrypt.org/directory",
		"acme", "account@example.com", "key-123", sealedSecret, string(domainJSON),
		fixedNow,
		fixedNow, fixedNow, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetCertByID)).
		WithArgs(testCertID, testOrgID).
		WillReturnRows(sqlmock.NewRows(certCols()).AddRow(newCertRow(t)...))

	r := NewRepository(db)
	c, err := r.GetByID(context.Background(), testCertID, testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, testCertID, c.ID)
	assert.Equal(t, "RSA", c.CryptoAlgorithm)
	assert.Equal(t, int64(1), c.Revision)
	assert.Equal(t, certPlaintextKey, c.PrivateKey, "读取时应解密回明文")
	assert.Equal(t, certPlaintextSecret, c.AccessSecret, "读取时应解密回明文")
	assert.Equal(t, []string{"example.com", "api.example.com"}, c.Domain)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetCertByID)).
		WithArgs("bad", testOrgID).
		WillReturnRows(sqlmock.NewRows(certCols()))

	r := NewRepository(db)
	c, err := r.GetByID(context.Background(), "bad", testOrgID)
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestRepository_GetByID_ForeignOrganizationNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetCertByID)).
		WithArgs(testCertID, "foreign-org").
		WillReturnRows(sqlmock.NewRows(certCols()))

	r := NewRepository(db)
	c, err := r.GetByID(context.Background(), testCertID, "foreign-org")
	assert.Error(t, err)
	assert.Nil(t, c)
}

// ── List ──────────────────────────────────────────────────────────

// listArgs: org_id×2 + keyword×3 + scope×2 + type×2 + limit + offset = 11
// countArgs: org_id×2 + keyword×3 + scope×2 + type×2 = 9
func TestRepository_List(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{testOrgID, testOrgID, "kw", "kw", "kw", "global", "global", "x509", "x509"}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountCerts)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListCerts)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(certCols()).AddRow(newCertRow(t)...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID,
		Keyword:        "kw",
		Scope:          "global",
		Type:           "x509",
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_EmptyFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountCerts)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListCerts)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(certCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	domainJSON, _ := json.Marshal([]string{"example.com"})

	mock.ExpectExec(regexp.QuoteMeta(queryInsertCert)).
		WithArgs(
			testCertID, testOrgID, "my-cert", "My Certificate",
			"global", "x509", "RSA", 2048, 1,
			fixedNow, "-----BEGIN CERT-----", sealedCertSecret{plaintext: certPlaintextKey, column: certColumnPrivateKey},
			"pub-key", "root-pub-key",
			"letsencrypt", "https://acme-v02.api.letsencrypt.org/directory",
			"acme", "account@example.com", "key-123", sealedCertSecret{plaintext: certPlaintextSecret, column: certColumnAccessSecret}, string(domainJSON),
			fixedNow,
			fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	entity := &domain.Cert{
		ID:                     testCertID,
		OrganizationID:         testOrgID,
		Name:                   "my-cert",
		DisplayName:            "My Certificate",
		Scope:                  "global",
		Type:                   "x509",
		CryptoAlgorithm:        "RSA",
		BitSize:                2048,
		ExpireInYears:          1,
		ExpireTime:             &fixedNow,
		Certificate:            "-----BEGIN CERT-----",
		PrivateKey:             certPlaintextKey,
		AuthorityPublicKey:     "pub-key",
		AuthorityRootPublicKey: "root-pub-key",
		AcmeIssuer:             "letsencrypt",
		AcmeEndpoint:           "https://acme-v02.api.letsencrypt.org/directory",
		Provider:               "acme",
		Account:                "account@example.com",
		AccessKey:              "key-123",
		AccessSecret:           certPlaintextSecret,
		Domain:                 []string{"example.com"},
		DomainExpireTime:       &fixedNow,
		CreatedAt:              fixedNow,
		UpdatedAt:              fixedNow,
		Revision:               int64(1),
	}
	err := r.Create(context.Background(), entity)
	assert.NoError(t, err)
	assert.Equal(t, certPlaintextKey, entity.PrivateKey, "加密不得修改 domain 对象")
	assert.Equal(t, certPlaintextSecret, entity.AccessSecret, "加密不得修改 domain 对象")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

// 乐观锁契约与 PG / 其他 MySQL 仓储一致：app 层在调用 Update 前已 Revision++，
// 因此 WHERE revision 必须比对自增前（Revision-1）的旧值。
// queryUpdateCert 参数：19 个 SET 值 + updated_at + id + old_revision + organization_id = 23
func TestRepository_Update_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	domainJSON, _ := json.Marshal([]string{"new.example.com"})

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateCert)).
		WithArgs(
			"My Updated Cert", "tenant", "ca", "EC", 256, 2,
			fixedNow, "-----BEGIN CERT 2-----", sealedCertSecret{plaintext: "-----BEGIN KEY 2-----", column: certColumnPrivateKey},
			"new-pub", "new-root-pub",
			"letsencrypt", "https://acme-v02.api.letsencrypt.org/directory",
			"acme", "new-account@example.com", "new-key",
			sealedCertSecret{plaintext: "new-secret", column: certColumnAccessSecret}, string(domainJSON), fixedNow,
			fixedNow, testCertID, int64(1), testOrgID, // entity.Revision=2 → guard 旧值 1
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectCertRevision)).
		WithArgs(testCertID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	c := &domain.Cert{
		ID:                     testCertID,
		OrganizationID:         testOrgID,
		DisplayName:            "My Updated Cert",
		Scope:                  "tenant",
		Type:                   "ca",
		CryptoAlgorithm:        "EC",
		BitSize:                256,
		ExpireInYears:          2,
		ExpireTime:             &fixedNow,
		Certificate:            "-----BEGIN CERT 2-----",
		PrivateKey:             "-----BEGIN KEY 2-----",
		AuthorityPublicKey:     "new-pub",
		AuthorityRootPublicKey: "new-root-pub",
		AcmeIssuer:             "letsencrypt",
		AcmeEndpoint:           "https://acme-v02.api.letsencrypt.org/directory",
		Provider:               "acme",
		Account:                "new-account@example.com",
		AccessKey:              "new-key",
		AccessSecret:           "new-secret",
		Domain:                 []string{"new.example.com"},
		DomainExpireTime:       &fixedNow,
		UpdatedAt:              fixedNow,
		Revision:               int64(2),
	}
	err := r.Update(context.Background(), c)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), c.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 钉住守卫参数：传入新 revision 时 WHERE 用 revision-1。取 5→4 避免与
// “新值 2 / 旧值 1” 的巧合混淆（若误传新值，期望会拿到 5）。
func TestRepository_Update_GuardUsesPreviousRevision(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateCert)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testCertID, int64(4), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectCertRevision)).
		WithArgs(testCertID, testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(5)))

	r := NewRepository(db)
	c := &domain.Cert{
		ID:             testCertID,
		OrganizationID: testOrgID,
		Revision:       int64(5),
	}
	err := r.Update(context.Background(), c)
	require.NoError(t, err)
	assert.Equal(t, int64(5), c.Revision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateCert)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testCertID, int64(1), testOrgID,
		).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → conflict

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Cert{
		ID:             testCertID,
		OrganizationID: testOrgID,
		Revision:       int64(2),
	})
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeConflict, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// The UPDATE predicate must carry the trusted organization: a foreign-tenant
// row is invisible to the statement, so zero affected rows surface as conflict.
func TestRepository_Update_ForeignOrganizationConflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateCert)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			testCertID, int64(1), "foreign-org",
		).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.Cert{
		ID:             testCertID,
		OrganizationID: "foreign-org",
		Revision:       int64(2),
	})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteCert)).
		WithArgs(testCertID, testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testCertID, testOrgID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ForeignOrganizationNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteCert)).
		WithArgs(testCertID, "foreign-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testCertID, "foreign-org")
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
