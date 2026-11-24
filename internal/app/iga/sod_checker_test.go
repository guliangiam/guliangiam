package iga_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/guliangiam/guliangiam/internal/app/iga"
	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	domainiga "github.com/guliangiam/guliangiam/internal/domain/iga"
	mysqliga "github.com/guliangiam/guliangiam/internal/infra/mysql/iga"
	"github.com/stretchr/testify/assert"
)

// ── 基础 fixtures ────────────────────────────────────────────────

var (
	sodFixedNow  = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	sodTestOrgID = "00000000-0000-0000-0000-000000000001"
	sodUserID    = "user-alice"
	roleAdmin    = "role-admin"
	roleFinance  = "role-finance"
	roleReadonly = "role-readonly"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── SODRules 列（8 列） ──────────────────────────────────────────

func sodRuleCols() []string {
	return []string{
		"id", "organization_id", "name", "role_a_id", "role_b_id",
		"created_at", "updated_at", "revision",
	}
}

func newSODRuleRow() []driver.Value {
	return []driver.Value{
		"sod-001", sodTestOrgID, "no-admin+finance", roleAdmin, roleFinance,
		sodFixedNow, sodFixedNow, int64(1),
	}
}

// ── 构造一个 sqlmock 驱动的 UOW ───────────────────────────────────

type mockUOW struct {
	appuow.UnitOfWork
	igaRepo domainiga.Repository
}

func (m *mockUOW) IGA() domainiga.Repository { return m.igaRepo }

var _ appuow.UnitOfWork = (*mockUOW)(nil)

func newSODMockUOW(db *sql.DB) *mockUOW {
	return &mockUOW{igaRepo: mysqliga.NewRepository(db)}
}

// expectEffectiveRoles 匹配 BE-D-17 后的第一步查询：先取用户当前有效角色。
func expectEffectiveRoles(mock sqlmock.Sqlmock, roleIDs ...string) {
	rows := sqlmock.NewRows([]string{"role_id"})
	for _, id := range roleIDs {
		rows.AddRow(id)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT DISTINCT b.role_id FROM")).
		WithArgs(sodTestOrgID, sodUserID, sqlmock.AnyArg(), sqlmock.AnyArg(), sodTestOrgID, sodUserID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(rows)
}

// expectSODRulesPage 匹配 MySQL 镜像回退路径：count + 分页 select。
func expectSODRulesPage(mock sqlmock.Sqlmock, total int64, ruleRows ...[]driver.Value) {
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `sod_rules` WHERE organization_id = ?")).
		WithArgs(sodTestOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(total))
	rows := sqlmock.NewRows(sodRuleCols())
	for _, r := range ruleRows {
		rows.AddRow(r...)
	}
	mock.ExpectQuery(regexp.QuoteMeta("FROM `sod_rules` WHERE organization_id = ? ORDER BY name DESC LIMIT ? OFFSET ?")).
		WithArgs(sodTestOrgID, 100, 0).
		WillReturnRows(rows)
}

// ── ValidateConflict Tests ───────────────────────────────────────

func TestSODChecker_ValidateConflict_NoRules(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// user 持有 finance，但 org 下没有 SOD rules
	expectEffectiveRoles(mock, roleFinance)
	expectSODRulesPage(mock, 0)

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	err := checker.ValidateConflict(context.Background(), sodTestOrgID, roleAdmin, sodUserID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSODChecker_ValidateConflict_NoHeldRoles(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// user 当前没有有效角色 → 短路返回，不再查询 SOD rules
	expectEffectiveRoles(mock)

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	err := checker.ValidateConflict(context.Background(), sodTestOrgID, roleAdmin, sodUserID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSODChecker_ValidateConflict_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	// user 已持有 finance；1 条 SOD 规则：admin vs finance
	expectEffectiveRoles(mock, roleFinance)
	expectSODRulesPage(mock, 1, newSODRuleRow())

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	// 授予 admin，冲突
	err := checker.ValidateConflict(context.Background(), sodTestOrgID, roleAdmin, sodUserID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, iga.ErrSODConflict)

	var ce *iga.SODConflictError
	if errors.As(err, &ce) {
		assert.Equal(t, roleAdmin, ce.RoleAID) // 实际返回的是规则里的 RoleA=admin
		assert.Equal(t, roleFinance, ce.RoleBID)
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSODChecker_ValidateConflict_ReverseRole(t *testing.T) {
	// 反向：user 已有 admin，授予 finance → 也应该冲突
	db, mock := newMockDB(t)
	defer db.Close()

	expectEffectiveRoles(mock, roleAdmin)
	expectSODRulesPage(mock, 1, newSODRuleRow())

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	err := checker.ValidateConflict(context.Background(), sodTestOrgID, roleFinance, sodUserID)
	assert.Error(t, err)
	assert.ErrorIs(t, err, iga.ErrSODConflict)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSODChecker_ValidateConflict_NoConflict(t *testing.T) {
	// user 已有 readonly，授予 admin → 规则里只有 admin vs finance，所以无冲突
	db, mock := newMockDB(t)
	defer db.Close()

	expectEffectiveRoles(mock, roleReadonly)
	expectSODRulesPage(mock, 1, newSODRuleRow())

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	err := checker.ValidateConflict(context.Background(), sodTestOrgID, roleAdmin, sodUserID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSODChecker_ValidateConflict_EmptyArgs(t *testing.T) {
	uow := newSODMockUOW(nil)
	checker := iga.NewSODChecker(uow)

	err := checker.ValidateConflict(context.Background(), "", roleAdmin, sodUserID)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "orgID")
}

// ── ValidateConflictRoles Tests ───────────────────────────────────

func TestSODChecker_ValidateConflictRoles_Match(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `sod_rules` WHERE organization_id = ?")).
		WithArgs(sodTestOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta("FROM `sod_rules` WHERE organization_id = ? ORDER BY name DESC LIMIT ? OFFSET ?")).
		WithArgs(sodTestOrgID, 100, 0).
		WillReturnRows(sqlmock.NewRows(sodRuleCols()).AddRow(newSODRuleRow()...))

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	// 正向匹配
	err := checker.ValidateConflictRoles(context.Background(), sodTestOrgID, roleAdmin, roleFinance)
	assert.Error(t, err)
	assert.ErrorIs(t, err, iga.ErrSODConflict)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSODChecker_ValidateConflictRoles_ReverseMatch(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `sod_rules` WHERE organization_id = ?")).
		WithArgs(sodTestOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta("FROM `sod_rules` WHERE organization_id = ? ORDER BY name DESC LIMIT ? OFFSET ?")).
		WithArgs(sodTestOrgID, 100, 0).
		WillReturnRows(sqlmock.NewRows(sodRuleCols()).AddRow(newSODRuleRow()...))

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	// 反向也应匹配（无序对）
	err := checker.ValidateConflictRoles(context.Background(), sodTestOrgID, roleFinance, roleAdmin)
	assert.Error(t, err)
	assert.ErrorIs(t, err, iga.ErrSODConflict)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestSODChecker_ValidateConflictRoles_NoMatch(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `sod_rules` WHERE organization_id = ?")).
		WithArgs(sodTestOrgID).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))
	mock.ExpectQuery(regexp.QuoteMeta("FROM `sod_rules` WHERE organization_id = ? ORDER BY name DESC LIMIT ? OFFSET ?")).
		WithArgs(sodTestOrgID, 100, 0).
		WillReturnRows(sqlmock.NewRows(sodRuleCols()).AddRow(newSODRuleRow()...))

	uow := newSODMockUOW(db)
	checker := iga.NewSODChecker(uow)

	err := checker.ValidateConflictRoles(context.Background(), sodTestOrgID, roleAdmin, roleReadonly)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func sodRulesWithConflictOnSecondPage() []*domainiga.SODRule {
	rules := make([]*domainiga.SODRule, 101)
	for i := 0; i < 100; i++ {
		rules[i] = &domainiga.SODRule{OrganizationID: sodTestOrgID, RoleAID: "role-other-a", RoleBID: "role-other-b"}
	}
	rules[100] = &domainiga.SODRule{OrganizationID: sodTestOrgID, RoleAID: roleAdmin, RoleBID: roleFinance}
	return rules
}

func TestSODChecker_ValidateConflict_ReadsAllRulePages(t *testing.T) {
	repo := &workflowRepo{sodRules: sodRulesWithConflictOnSecondPage(), effectiveRoleIDs: []string{roleFinance}}
	checker := iga.NewSODChecker(&mockUOW{igaRepo: repo})

	err := checker.ValidateConflict(context.Background(), sodTestOrgID, roleAdmin, sodUserID)
	assert.ErrorIs(t, err, iga.ErrSODConflict)
	assert.Equal(t, []int{1, 2}, repo.sodRulePages)
	assert.Equal(t, []int{100, 100}, repo.sodRulePageSizes)
}

func TestSODChecker_ValidateConflictRoles_ReadsAllRulePages(t *testing.T) {
	repo := &workflowRepo{sodRules: sodRulesWithConflictOnSecondPage()}
	checker := iga.NewSODChecker(&mockUOW{igaRepo: repo})

	err := checker.ValidateConflictRoles(context.Background(), sodTestOrgID, roleAdmin, roleFinance)
	assert.ErrorIs(t, err, iga.ErrSODConflict)
	assert.Equal(t, []int{1, 2}, repo.sodRulePages)
	assert.Equal(t, []int{100, 100}, repo.sodRulePageSizes)
}
