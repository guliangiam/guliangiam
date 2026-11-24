package iga

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/iga"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedLater = time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)
	testOrgID  = "00000000-0000-0000-0000-000000000001"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── Campaigns 列定义（9 列） ────────────────────────────────────

func campaignCols() []string {
	return []string{
		"id", "organization_id", "name", "status", "start_date", "end_date",
		"created_at", "updated_at", "revision",
	}
}

func newCampaignRow() []driver.Value {
	return []driver.Value{
		"cmp-001", testOrgID, "Q3 Review", "active", &fixedNow, &fixedLater,
		fixedNow, fixedNow, int64(1),
	}
}

// ── SOD Rules 列定义（8 列） ─────────────────────────────────────

func sodRuleCols() []string {
	return []string{
		"id", "organization_id", "name", "role_a_id", "role_b_id",
		"created_at", "updated_at", "revision",
	}
}

func newSODRuleRow() []driver.Value {
	return []driver.Value{
		"sod-001", testOrgID, "no-admin+finance", "role-admin", "role-finance",
		fixedNow, fixedNow, int64(1),
	}
}

// ── Lifecycle 列定义（9 列） ─────────────────────────────────────

func lifecycleCols() []string {
	return []string{
		"id", "organization_id", "user_id", "role_id", "granted_at", "expires_at",
		"status", "created_at", "revision",
	}
}

func newLifecycleRow() []driver.Value {
	return []driver.Value{
		"lc-001", testOrgID, "user-123", "role-admin", fixedNow, fixedLater,
		"active", fixedNow, int64(1),
	}
}

// ── Campaigns: ListCampaigns ─────────────────────────────────────

func TestRepository_ListCampaigns_Default(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `access_campaigns` WHERE organization_id = ?")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `access_campaigns` WHERE organization_id = ? ORDER BY updated_at DESC LIMIT ? OFFSET ?")).
		WithArgs(testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(campaignCols()).AddRow(newCampaignRow()...))

	r := NewRepository(db)
	items, total, err := r.ListCampaigns(context.Background(), domain.ListCampaignsQuery{
		OrganizationID: testOrgID, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "cmp-001", items[0].ID)
	assert.Equal(t, domain.CampaignActive, items[0].Status)
}

func TestRepository_ListCampaigns_WithStatus(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	st := domain.CampaignActive

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `access_campaigns` WHERE organization_id = ?")).
		WithArgs(testOrgID, string(st)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `access_campaigns` WHERE organization_id = ?")).
		WithArgs(testOrgID, string(st), 20, 0).
		WillReturnRows(sqlmock.NewRows(campaignCols()))

	r := NewRepository(db)
	items, total, err := r.ListCampaigns(context.Background(), domain.ListCampaignsQuery{
		OrganizationID: testOrgID, Status: &st, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── Campaigns: GetCampaign ───────────────────────────────────────

func TestRepository_GetCampaign(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qGetCampaign)).
		WithArgs("cmp-001", testOrgID).
		WillReturnRows(sqlmock.NewRows(campaignCols()).AddRow(newCampaignRow()...))

	r := NewRepository(db)
	c, err := r.GetCampaign(context.Background(), "cmp-001", testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, "cmp-001", c.ID)
	assert.Equal(t, domain.CampaignActive, c.Status)
}

// ── Campaigns: CreateCampaign ─────────────────────────────────────

func TestRepository_CreateCampaign(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qInsertCampaign)).
		WithArgs("cmp-001", testOrgID, "Q3 Review", &fixedNow, &fixedLater).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateCampaign(context.Background(), &domain.AccessCampaign{
		ID: "cmp-001", OrganizationID: testOrgID, Name: "Q3 Review",
		StartDate: &fixedNow, EndDate: &fixedLater,
	})
	assert.NoError(t, err)
}

// ── Campaigns: UpdateCampaignStatus ──────────────────────────────

func TestRepository_UpdateCampaignStatus(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qUpdateCampaignStatus)).
		WithArgs("cancelled", "cmp-001", testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.UpdateCampaignStatus(context.Background(), testOrgID, "cmp-001", domain.CampaignCancelled)
	assert.NoError(t, err)
}

// ── SOD Rules: ListSODRules ──────────────────────────────────────

func TestRepository_ListSODRules(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qCountSODRules)).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta(qListSODRules)).
		WithArgs(testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(sodRuleCols()).AddRow(newSODRuleRow()...))

	r := NewRepository(db)
	items, total, err := r.ListSODRules(context.Background(), domain.ListSODRulesQuery{
		OrganizationID: testOrgID, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "sod-001", items[0].ID)
	assert.Equal(t, "role-admin", items[0].RoleAID)
	assert.Equal(t, "role-finance", items[0].RoleBID)
}

// ── SOD Rules: GetSODRule ────────────────────────────────────────

func TestRepository_GetSODRule(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(qGetSODRule)).
		WithArgs("sod-001", testOrgID).
		WillReturnRows(sqlmock.NewRows(sodRuleCols()).AddRow(newSODRuleRow()...))

	r := NewRepository(db)
	s, err := r.GetSODRule(context.Background(), "sod-001", testOrgID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, "sod-001", s.ID)
}

// ── SOD Rules: CreateSODRule ──────────────────────────────────────

func TestRepository_CreateSODRule(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qInsertSODRule)).
		WithArgs("sod-001", testOrgID, "no-admin+finance", "role-admin", "role-finance").
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateSODRule(context.Background(), &domain.SODRule{
		ID: "sod-001", OrganizationID: testOrgID, Name: "no-admin+finance",
		RoleAID: "role-admin", RoleBID: "role-finance",
	})
	assert.NoError(t, err)
}

// ── Lifecycle: ListLifecycle ─────────────────────────────────────

func TestRepository_ListLifecycle_Default(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `access_lifecycle` WHERE organization_id = ?")).
		WithArgs(testOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `access_lifecycle` WHERE organization_id = ? ORDER BY expires_at DESC LIMIT ? OFFSET ?")).
		WithArgs(testOrgID, 20, 0).
		WillReturnRows(sqlmock.NewRows(lifecycleCols()).AddRow(newLifecycleRow()...))

	r := NewRepository(db)
	items, total, err := r.ListLifecycle(context.Background(), domain.ListLifecycleQuery{
		OrganizationID: testOrgID, Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, "lc-001", items[0].ID)
	assert.Equal(t, domain.LifecycleActive, items[0].Status)
}

func TestRepository_ListLifecycle_AllFilters(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	userID := "user-123"
	roleID := "role-admin"
	st := domain.LifecycleActive

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `access_lifecycle` WHERE organization_id = ?")).
		WithArgs(testOrgID, userID, roleID, string(st)).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	mock.ExpectQuery(regexp.QuoteMeta("FROM `access_lifecycle` WHERE organization_id = ?")).
		WithArgs(testOrgID, userID, roleID, string(st), 20, 0).
		WillReturnRows(sqlmock.NewRows(lifecycleCols()))

	r := NewRepository(db)
	items, total, err := r.ListLifecycle(context.Background(), domain.ListLifecycleQuery{
		OrganizationID: testOrgID, UserID: &userID, RoleID: &roleID, Status: &st,
		Page: 1, PageSize: 20,
	})
	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, int64(0), total)
}

// ── Lifecycle: CreateLifecycle ───────────────────────────────────

func TestRepository_CreateLifecycle(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qInsertLifecycle)).
		WithArgs("lc-001", testOrgID, "user-123", "role-admin", nil, nil, "", "", nil, fixedNow, fixedNow, fixedLater, domain.LifecycleActive).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateLifecycle(context.Background(), &domain.AccessLifecycle{
		ID: "lc-001", OrganizationID: testOrgID,
		UserID: "user-123", RoleID: "role-admin", GrantedAt: fixedNow,
		StartsAt: fixedNow, ExpiresAt: fixedLater, Status: domain.LifecycleActive,
	})
	assert.NoError(t, err)
}

// ── Lifecycle: RevokeLifecycle ───────────────────────────────────

func TestRepository_RevokeLifecycle(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qRevokeLifecycle)).
		WithArgs("lc-001", testOrgID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.RevokeLifecycle(context.Background(), "lc-001", testOrgID)
	assert.NoError(t, err)
}

// ── Lifecycle: ExpireLifecycle ────────────────────────────────────

func TestRepository_ExpireLifecycle(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(qExpireLifecycle)).
		WithArgs(testOrgID, fixedNow).
		WillReturnResult(sqlmock.NewResult(2, 42)) // 42 rows affected

	r := NewRepository(db)
	n, err := r.ExpireLifecycle(context.Background(), testOrgID, fixedNow)
	assert.NoError(t, err)
	assert.Equal(t, int64(42), n)
}

// ── 编译期接口断言 ───────────────────────────────────────────────

var _ domain.Repository = Repository{}
