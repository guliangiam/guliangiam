package workload

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID  = "00000000-0000-0000-0000-000000000001"
	testPoolID = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── Pools 列定义（8 列） ────────────────────────────────────────

func poolCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "description",
		"created_at", "updated_at", "revision",
	}
}

func newPoolRow() []driver.Value {
	return []driver.Value{
		"pool-001", testOrgID, "prod-pool", "Production Pool", "Prod identity pool",
		fixedNow, fixedNow, int64(1),
	}
}

// ── Trusts 列定义（12 列） ─────────────────────────────────────

func trustCols() []string {
	return []string{
		"id", "organization_id", "pool_id", "name", "issuer", "jwks_uri", "audience", "subjects",
		"status", "created_at", "updated_at", "revision",
	}
}

func newTrustRow() []driver.Value {
	return []driver.Value{
		"trust-001", testOrgID, testPoolID, "corp-trust",
		"https://corp.example.com", "https://corp.example.com/.well-known/jwks.json",
		`["urn:app:guliangiam","urn:app:other"]`, `["repo:org/*"]`,
		"active", fixedNow, fixedNow, int64(1),
	}
}

// ── Accounts 列定义（14 列） ────────────────────────────────────

func accountCols() []string {
	return []string{
		"id", "organization_id", "pool_id", "name", "description", "client_id", "client_secret_hash",
		"scopes", "access_token_ttl_seconds", "last_rotated_at", "status", "created_at", "updated_at", "revision",
	}
}

func newAccountRow() []driver.Value {
	return []driver.Value{
		"sa-001", testOrgID, testPoolID, "ci-bot", "CI bot", "ci-bot-client-id", "hash-of-secret",
		`["orders:read"]`, 900, fixedNow, "active", fixedNow, fixedNow, int64(1),
	}
}

// ── Pools: ListPools ────────────────────────────────────────────

func TestRepository_ListPools(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qCountPools)).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta(qListPools)).
		WithArgs(testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(poolCols()).AddRow(newPoolRow()...))

	r := NewRepository(db)
	items, total, err := r.ListPools(context.Background(), domain.ListPoolsQuery{
		OrganizationID: testOrgID, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "pool-001", items[0].ID)
	assert.Equal(t, "Production Pool", items[0].DisplayName)
}

// ── Pools: GetPool ──────────────────────────────────────────────

func TestRepository_GetPool(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qGetPool)).
		WithArgs("pool-001", testOrgID).
		WillReturnRows(sqlmock.NewRows(poolCols()).AddRow(newPoolRow()...))

	r := NewRepository(db)
	p, err := r.GetPool(context.Background(), "pool-001", testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "pool-001", p.ID)
	assert.Equal(t, int64(1), p.Revision)
}

// ── Pools: CreatePool ────────────────────────────────────────────

func TestRepository_CreatePool(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qInsertPool)).
		WithArgs("pool-001", testOrgID, "prod-pool", "Production Pool", "Prod identity pool").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreatePool(context.Background(), &domain.Pool{
		ID: "pool-001", OrganizationID: testOrgID,
		Name: "prod-pool", DisplayName: "Production Pool", Description: "Prod identity pool",
	})
	assert.NoError(t, err)
}

// ── Trusts: ListTrusts ──────────────────────────────────────────

func TestRepository_ListTrusts(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qCountTrusts)).
		WithArgs(testOrgID, testOrgID, testPoolID, testPoolID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta(qListTrusts)).
		WithArgs(testOrgID, testOrgID, testPoolID, testPoolID, 20, 0).
		WillReturnRows(sqlmock.NewRows(trustCols()).AddRow(newTrustRow()...))

	r := NewRepository(db)
	items, total, err := r.ListTrusts(context.Background(), domain.ListTrustsQuery{
		OrganizationID: testOrgID, PoolID: testPoolID, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "trust-001", items[0].ID)
	assert.Equal(t, domain.TrustStatusActive, items[0].Status)
	assert.Equal(t, []string{"urn:app:guliangiam", "urn:app:other"}, items[0].Audience)
	assert.Equal(t, []string{"repo:org/*"}, items[0].Subjects)
}

// ── Trusts: GetTrust ────────────────────────────────────────────

func TestRepository_GetTrust(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qGetTrust)).
		WithArgs("trust-001", testOrgID).
		WillReturnRows(sqlmock.NewRows(trustCols()).AddRow(newTrustRow()...))

	r := NewRepository(db)
	t2, err := r.GetTrust(context.Background(), "trust-001", testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, t2)
	assert.Equal(t, "trust-001", t2.ID)
	assert.Equal(t, []string{"urn:app:guliangiam", "urn:app:other"}, t2.Audience)
}

// ── Trusts: CreateTrust ─────────────────────────────────────────

func TestRepository_CreateTrust(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qInsertTrust)).
		WithArgs(
			"trust-001", testOrgID, testPoolID, "corp-trust",
			"https://corp.example.com", "https://corp.example.com/.well-known/jwks.json",
			`["urn:app:guliangiam","urn:app:other"]`, `["repo:org/*"]`,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateTrust(context.Background(), &domain.FederationTrust{
		ID: "trust-001", OrganizationID: testOrgID, PoolID: testPoolID,
		Name: "corp-trust", Issuer: "https://corp.example.com",
		JWKSURI:  "https://corp.example.com/.well-known/jwks.json",
		Audience: []string{"urn:app:guliangiam", "urn:app:other"},
		Subjects: []string{"repo:org/*"},
	})
	assert.NoError(t, err)
}

// ── Accounts: ListAccounts ────────────────────────────────────────

func TestRepository_ListAccounts(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qCountAccounts)).
		WithArgs(testOrgID, testPoolID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta(qListAccounts)).
		WithArgs(testOrgID, testPoolID, 20, 0).
		WillReturnRows(sqlmock.NewRows(accountCols()).AddRow(newAccountRow()...))

	r := NewRepository(db)
	items, total, err := r.ListAccounts(context.Background(), domain.ListAccountsQuery{
		OrganizationID: testOrgID, PoolID: testPoolID, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "sa-001", items[0].ID)
	assert.Equal(t, domain.SAStatusActive, items[0].Status)
}

// ── Accounts: GetAccount ────────────────────────────────────────

func TestRepository_GetAccount(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qGetAccount)).
		WithArgs("sa-001", testOrgID).
		WillReturnRows(sqlmock.NewRows(accountCols()).AddRow(newAccountRow()...))

	r := NewRepository(db)
	a, err := r.GetAccount(context.Background(), "sa-001", testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, a)
	assert.Equal(t, "sa-001", a.ID)
	assert.Equal(t, "ci-bot", a.Name)
	assert.Equal(t, "CI bot", a.Description)
	assert.Equal(t, []string{"orders:read"}, a.Scopes)
	assert.Equal(t, 900, a.AccessTokenTTLSeconds)
	assert.NotNil(t, a.LastRotatedAt)
}

// ── Accounts: CreateAccount ────────────────────────────────────

func TestRepository_CreateAccount(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qInsertAccount)).
		WithArgs("sa-001", testOrgID, testPoolID, "ci-bot", "CI bot", "ci-bot-client-id", "hash-of-secret", `["orders:read"]`, 900).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateAccount(context.Background(), &domain.ServiceAccount{
		ID: "sa-001", OrganizationID: testOrgID, PoolID: testPoolID,
		Name: "ci-bot", Description: "CI bot", ClientID: "ci-bot-client-id", ClientSecretHash: "hash-of-secret",
		Scopes: []string{"orders:read"}, AccessTokenTTLSeconds: 900,
	})
	assert.NoError(t, err)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
