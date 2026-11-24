package providers

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domain "github.com/guliangiam/guliangiam/internal/domain/providers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
)

var fixedNow = time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

// makeProviderRow 返回精确 45 个类型正确的 driver.Value（GetByID/List Scan 字段数）。
func makeProviderRow(id, orgID, name string) []driver.Value {
	vals := make([]driver.Value, 45)
	vals[0] = ""            // ID
	vals[1] = ""            // OrganizationID
	vals[2] = ""            // Name
	vals[3] = ""            // DisplayName
	vals[4] = ""            // Category
	vals[5] = ""            // ProviderType
	vals[6] = ""            // SubType
	vals[7] = ""            // Method
	vals[8] = ""            // ClientID
	vals[9] = ""            // ClientSecret
	vals[10] = ""           // ClientID2
	vals[11] = ""           // ClientSecret2
	vals[12] = ""           // ProviderURL
	vals[13] = ""           // AuthUrl
	vals[14] = ""           // TokenUrl
	vals[15] = ""           // UserInfoUrl
	vals[16] = ""           // CallbackUrl
	vals[17] = ""           // SigningAlg
	vals[18] = []byte("[]") // Scopes (JSON)
	vals[19] = ""           // IdPEntityId
	vals[20] = ""           // SsoUrl
	vals[21] = ""           // SloUrl
	vals[22] = ""           // Metadata
	vals[23] = ""           // IssuerUrl
	vals[24] = ""           // Region
	vals[25] = ""           // Host
	vals[26] = 0            // Port
	vals[27] = ""           // DatabaseType
	vals[28] = []byte("{}") // UserMapping (JSON)
	vals[29] = ""           // Template
	vals[30] = ""           // CustomApi
	vals[31] = ""           // CustomHeader
	vals[32] = ""           // CustomBody
	vals[33] = ""           // Admin
	vals[34] = ""           // Password
	vals[35] = ""           // AppId
	vals[36] = ""           // AppSecret
	vals[37] = ""           // AgentID
	vals[38] = ""           // ContentType
	vals[39] = ""           // Sender
	vals[40] = ""           // Receiver
	vals[41] = false        // Enabled
	vals[42] = time.Time{}  // CreatedAt
	vals[43] = time.Time{}  // UpdatedAt
	vals[44] = int64(0)     // Revision
	// 覆盖关键字段
	vals[0] = id
	vals[1] = orgID
	vals[2] = name
	vals[3] = "Test Provider"
	vals[4] = "oauth2"
	vals[41] = true
	vals[42] = fixedNow
	vals[43] = fixedNow
	vals[44] = int64(1)
	return vals
}

func providerRowCols(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "c"
	}
	return out
}

func newMockDB(t *testing.T) (sqlmock.Sqlmock, Repository) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	repo := NewRepository(db)
	return mock, repo
}

func providerWriteArgs(count int) []driver.Value {
	args := make([]driver.Value, count)
	for i := range args {
		args[i] = sqlmock.AnyArg()
	}
	return args
}

// envelopeSecretArg matches a v2 envelope token bound to the provider row and
// verifies it decrypts back to the expected plaintext.
type envelopeSecretArg struct {
	plaintext string
	column    string
	pk        string
}

func (e envelopeSecretArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	ctx := crypto.AADContext{Table: providerTable, Column: e.column, PKValue: e.pk}
	opened, err := crypto.GlobalFieldEncryptor().DecryptEnvelopeFieldWithAAD(ctx, stored)
	return err == nil && opened == e.plaintext
}

// ─── GetByID ──────────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectQuery(regexp.QuoteMeta("FROM `providers` WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("prov-1", "", "").
		WillReturnRows(sqlmock.NewRows(providerRowCols(45)).AddRow(makeProviderRow("prov-1", "org-1", "github")...))

	p, err := repo.GetByID(context.Background(), "prov-1")
	require.NoError(t, err)
	assert.Equal(t, "prov-1", p.ID)
	assert.Equal(t, "org-1", p.OrganizationID)
	assert.Equal(t, "github", p.Name)
	assert.Equal(t, int64(1), p.Revision)
	assert.True(t, p.Enabled)
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectQuery(regexp.QuoteMeta("FROM `providers` WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("missing", "", "").
		WillReturnRows(sqlmock.NewRows(providerRowCols(45)))

	_, err := repo.GetByID(context.Background(), "missing")
	require.Error(t, err)
}

// ─── List ──────────────────────────────────────────────────────────

func TestRepository_List(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectQuery(regexp.QuoteMeta("FROM `providers` WHERE (? = '' OR organization_id = ?)")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "", 20, 0).
		WillReturnRows(sqlmock.NewRows(providerRowCols(45)).AddRow(makeProviderRow("prov-1", "org-1", "github")...))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `providers`")).
		WithArgs("org-1", "org-1", "", "", "", "", "", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	items, total, err := repo.List(context.Background(), domain.ListQuery{OrganizationID: "org-1", PageSize: 20})
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
}

// ─── Create ──────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	args := providerWriteArgs(48)
	args[45] = "payment"
	args[46] = "alipay"
	args[47] = "sandbox"
	mock.ExpectExec(regexp.QuoteMeta(queryInsertProvider)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Create(context.Background(), &domain.Provider{
		ID:             "prov-new",
		OrganizationID: "org-1",
		Name:           "new-provider",
		Revision:       1,
		Family:         domain.FamilyPayment,
		Kind:           domain.Kind("alipay"),
		Variant:        domain.VariantSandbox,
	})
	require.NoError(t, err)
}

func TestRepository_Create_EncryptsSecrets(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	args := providerWriteArgs(48)
	args[9] = envelopeSecretArg{plaintext: "client-secret", column: providerColumnClientSecret, pk: "prov-secret"}
	args[11] = envelopeSecretArg{plaintext: "client-secret-2", column: providerColumnClientSecret2, pk: "prov-secret"}
	args[34] = envelopeSecretArg{plaintext: "db-password", column: providerColumnPassword, pk: "prov-secret"}
	args[36] = envelopeSecretArg{plaintext: "app-secret", column: providerColumnAppSecret, pk: "prov-secret"}
	args[45] = "identity"
	args[46] = "twilio_sms"
	args[47] = ""
	mock.ExpectExec(regexp.QuoteMeta(queryInsertProvider)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Create(context.Background(), &domain.Provider{
		ID:             "prov-secret",
		OrganizationID: "org-1",
		Name:           "secret-provider",
		Revision:       1,
		Family:         domain.FamilyIdentity,
		Kind:           domain.Kind("twilio_sms"),
		ClientSecret:   "client-secret",
		ClientSecret2:  "client-secret-2",
		Password:       "db-password",
		AppSecret:      "app-secret",
	})
	require.NoError(t, err)
}

// ─── Update ──────────────────────────────────────────────────────────

func TestRepository_Update_Success(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	args := providerWriteArgs(48)
	args[40] = "payment"
	args[41] = "alipay"
	args[42] = "sandbox"
	args[43] = "renamed-provider"
	args[44] = "prov-1"
	args[45] = int64(1)
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateProvider)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT revision FROM `providers` WHERE id = ?")).
		WithArgs("prov-1").
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(3)))

	err := repo.Update(context.Background(), &domain.Provider{
		ID:       "prov-1",
		Name:     "renamed-provider",
		Revision: 2,
		Family:   domain.FamilyPayment,
		Kind:     domain.Kind("alipay"),
		Variant:  domain.VariantSandbox,
	})
	require.NoError(t, err)
}

func TestRepository_Update_EncryptsSecrets(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	args := providerWriteArgs(48)
	args[6] = envelopeSecretArg{plaintext: "rotated-client-secret", column: providerColumnClientSecret, pk: "prov-secret"}
	args[8] = envelopeSecretArg{plaintext: "rotated-client-secret-2", column: providerColumnClientSecret2, pk: "prov-secret"}
	args[31] = envelopeSecretArg{plaintext: "rotated-password", column: providerColumnPassword, pk: "prov-secret"}
	args[33] = envelopeSecretArg{plaintext: "rotated-app-secret", column: providerColumnAppSecret, pk: "prov-secret"}
	args[40] = "identity"
	args[41] = "twilio_sms"
	args[42] = ""
	args[43] = "secret-provider"
	args[44] = "prov-secret"
	args[45] = int64(1)
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateProvider)).
		WithArgs(args...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT revision FROM `providers` WHERE id = ?")).
		WithArgs("prov-secret").
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	err := repo.Update(context.Background(), &domain.Provider{
		ID:            "prov-secret",
		Name:          "secret-provider",
		Revision:      2,
		Family:        domain.FamilyIdentity,
		Kind:          domain.Kind("twilio_sms"),
		ClientSecret:  "rotated-client-secret",
		ClientSecret2: "rotated-client-secret-2",
		Password:      "rotated-password",
		AppSecret:     "rotated-app-secret",
	})
	require.NoError(t, err)
}

func TestRepository_Update_RevisionConflict(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE `providers` SET")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.Update(context.Background(), &domain.Provider{
		ID:       "prov-1",
		Revision: 99,
	})
	require.Error(t, err)
}

func TestRepository_GetByID_DecryptsSecrets(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	row := makeProviderRow("prov-1", "org-1", "github")
	sealed, err := crypto.GlobalFieldEncryptor().EncryptField("stored-secret")
	require.NoError(t, err)
	row[9] = sealed
	mock.ExpectQuery(regexp.QuoteMeta("FROM `providers` WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("prov-1", "", "").
		WillReturnRows(sqlmock.NewRows(providerRowCols(45)).AddRow(row...))

	p, err := repo.GetByID(context.Background(), "prov-1")
	require.NoError(t, err)
	assert.Equal(t, "stored-secret", p.ClientSecret)
}

func TestRepository_GetByID_RejectsPlaintextSecret(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	row := makeProviderRow("prov-1", "org-1", "github")
	row[9] = "plaintext-secret"
	mock.ExpectQuery(regexp.QuoteMeta("FROM `providers` WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("prov-1", "", "").
		WillReturnRows(sqlmock.NewRows(providerRowCols(45)).AddRow(row...))

	_, err := repo.GetByID(context.Background(), "prov-1")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not envelope-encrypted")
}

// ─── Delete ──────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	mock, repo := newMockDB(t)
	defer func() { assert.NoError(t, mock.ExpectationsWereMet()) }()

	mock.ExpectExec(regexp.QuoteMeta("UPDATE `providers` SET deleted_at = NOW(), updated_at = NOW() WHERE id = ? AND (? = '' OR organization_id = ?) AND deleted_at IS NULL")).
		WithArgs("prov-1", "", "").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.Delete(context.Background(), "prov-1")
	require.NoError(t, err)
}
