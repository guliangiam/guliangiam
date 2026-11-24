package rules

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/rules"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow10  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID10 = "00000000-0000-0000-0000-000000000001"
	testRuleID  = "00000000-0000-0000-0000-000000000008"
)

func newRuleMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ruleCols: 12 列
func ruleCols() []string {
	return []string{
		"id", "organization_id", "name", "scope", "rule_type", "conditions", "action",
		"priority", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newRuleRow() []driver.Value {
	return []driver.Value{
		testRuleID, testOrgID10, "rate-limit-01", "org", "RateLimit",
		`[{"field":"ip","op":"eq","value":"1.2.3.4"}]`,
		"Block", 10, true, fixedNow10, fixedNow10, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRuleByID)).
		WithArgs(testRuleID, testOrgID10).
		WillReturnRows(sqlmock.NewRows(ruleCols()).AddRow(newRuleRow()...))

	r := NewRepository(db)
	rule, err := r.GetByID(context.Background(), testRuleID, testOrgID10)
	assert.NoError(t, err)
	assert.NotNil(t, rule)
	assert.Equal(t, testRuleID, rule.ID)
	assert.Equal(t, "RateLimit", rule.RuleType)
	assert.True(t, rule.IsEnabled)
	assert.Len(t, rule.Conditions, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRuleByID)).
		WithArgs("bad", testOrgID10).
		WillReturnRows(sqlmock.NewRows(ruleCols()))

	r := NewRepository(db)
	rule, err := r.GetByID(context.Background(), "bad", testOrgID10)
	assert.Error(t, err)
	assert.Nil(t, rule)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + scope×2 + rule_type×2 + is_enabled×2 = 8
// listArgs: 8 + limit + offset = 10
func TestRepository_List_WithAllFilters(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgID10, testOrgID10, "org", "org", "RateLimit", "RateLimit", enabled, enabled}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRules)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRules)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(ruleCols()).AddRow(newRuleRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID10,
		Scope:          "org",
		RuleType:       "RateLimit",
		IsEnabled:      &enabled,
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_NilEnabled(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRules)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRules)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(ruleCols()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertRule)).
		WithArgs(
			testRuleID, testOrgID10, "rate-limit-01", "org", "RateLimit",
			[]byte(`[{"field":"ip","op":"eq","value":"1.2.3.4"}]`),
			"Block", 10, true, fixedNow10, fixedNow10, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.Rule{
		ID:             testRuleID,
		OrganizationID: testOrgID10,
		Name:           "rate-limit-01",
		Scope:          "org",
		RuleType:       "RateLimit",
		Conditions: []map[string]any{
			{"field": "ip", "op": "eq", "value": "1.2.3.4"},
		},
		Action:    "Block",
		Priority:  10,
		IsEnabled: true,
		CreatedAt: fixedNow10,
		UpdatedAt: fixedNow10,
		Revision:  int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update_Success(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	// Step 1: UPDATE ... WHERE id=? AND revision=?
	mock.ExpectExec(regexp.QuoteMeta(queryUpdateRule)).
		WithArgs(
			"rate-limit-02", "app", "WAF",
			[]byte(`[{"field":"path","op":"prefix","value":"/admin"}]`),
			"Captcha", 5, false, fixedNow10,
			testRuleID, int64(1), testOrgID10,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Step 2: SELECT revision
	mock.ExpectQuery(regexp.QuoteMeta(querySelectRuleRevision)).
		WithArgs(testRuleID, testOrgID10).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	rule := &domain.Rule{
		ID:             testRuleID,
		OrganizationID: testOrgID10,
		Name:           "rate-limit-02",
		Scope:          "app",
		RuleType:       "WAF",
		Conditions: []map[string]any{
			{"field": "path", "op": "prefix", "value": "/admin"},
		},
		Action:    "Captcha",
		Priority:  5,
		IsEnabled: false,
		UpdatedAt: fixedNow10,
		Revision:  int64(1),
	}
	err := r.Update(context.Background(), rule)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), rule.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Update_Conflict(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateRule)).
		WithArgs(
			"rate-limit-02", "app", "WAF",
			[]byte(`[{"field":"path","op":"prefix","value":"/admin"}]`),
			"Captcha", 5, false, fixedNow10,
			testRuleID, int64(1), testOrgID10,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	rule := &domain.Rule{
		ID:             testRuleID,
		OrganizationID: testOrgID10,
		Name:           "rate-limit-02",
		Scope:          "app",
		RuleType:       "WAF",
		Conditions: []map[string]any{
			{"field": "path", "op": "prefix", "value": "/admin"},
		},
		Action:    "Captcha",
		Priority:  5,
		IsEnabled: false,
		UpdatedAt: fixedNow10,
		Revision:  int64(1),
	}
	err := r.Update(context.Background(), rule)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRule)).
		WithArgs(testRuleID, testOrgID10).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRuleID, testOrgID10)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}

func TestRepository_Delete_ZeroRowsIsNotFound(t *testing.T) {
	db, mock := newRuleMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRule)).
		WithArgs(testRuleID, testOrgID10).
		WillReturnResult(sqlmock.NewResult(0, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRuleID, testOrgID10)
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}
