package mcpservers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/mcpservers"
	"github.com/guliangiam/guliangiam/internal/platform/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow5  = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID5 = "00000000-0000-0000-0000-000000000001"
	testMcpID  = "00000000-0000-0000-0000-000000000002"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// mcpCols: 12 列
func mcpCols() []string {
	return []string{
		"id", "organization_id", "name", "display_name", "url", "token", "application",
		"tools", "is_enabled", "created_at", "updated_at", "revision",
	}
}

func newMcpRow() []driver.Value {
	toolsJSON, _ := json.Marshal([]domain.Tool{
		{Name: "search", Description: "Search tool", IsAllowed: true},
	})
	return []driver.Value{
		testMcpID, testOrgID5, "my-mcp", "My MCP", "https://mcp.example.com", "tok-123", "app-y",
		string(toolsJSON), true, fixedNow5, fixedNow5, int64(1),
	}
}

func newMcpRowWithToken(token string) []driver.Value {
	row := newMcpRow()
	row[5] = token
	return row
}

// sealedMcpTokenArg asserts that the value written for token is a v2 envelope
// bound to the row id and opens back to the expected plaintext.
type sealedMcpTokenArg struct {
	plaintext string
	serverID  string
}

func (a sealedMcpTokenArg) Match(value driver.Value) bool {
	stored, ok := value.(string)
	if !ok || !crypto.IsEnvelopeToken(stored) {
		return false
	}
	header, _, _ := strings.Cut(stored, ":")
	if !strings.HasSuffix(header, "-aad") {
		return false
	}
	opened, err := crypto.GlobalFieldEncryptor().DecryptColumn(mcpServerAAD(a.serverID), stored)
	return err == nil && opened == a.plaintext
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetMcpServerByID)).
		WithArgs(testMcpID).
		WillReturnRows(sqlmock.NewRows(mcpCols()).AddRow(newMcpRow()...))

	r := NewRepository(db)
	s, err := r.GetByID(context.Background(), testMcpID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testMcpID, s.ID)
	assert.Len(t, s.Tools, 1)
	assert.Equal(t, "search", s.Tools[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetMcpServerByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(mcpCols()))

	r := NewRepository(db)
	s, err := r.GetByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, s)
}

func TestRepository_GetByID_SealedTokenDecrypts(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(mcpServerAAD(testMcpID), "tok-123")
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(queryGetMcpServerByID)).
		WithArgs(testMcpID).
		WillReturnRows(sqlmock.NewRows(mcpCols()).AddRow(newMcpRowWithToken(sealed)...))

	r := NewRepository(db)
	s, err := r.GetByID(context.Background(), testMcpID)
	require.NoError(t, err)
	assert.Equal(t, "tok-123", s.Token)
}

func TestRepository_GetByID_RejectsForeignRowBinding(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	sealed, err := crypto.GlobalFieldEncryptor().EncryptColumn(mcpServerAAD("other-server"), "tok-123")
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(queryGetMcpServerByID)).
		WithArgs(testMcpID).
		WillReturnRows(sqlmock.NewRows(mcpCols()).AddRow(newMcpRowWithToken(sealed)...))

	r := NewRepository(db)
	_, err = r.GetByID(context.Background(), testMcpID)
	require.Error(t, err)
	assert.ErrorIs(t, err, crypto.ErrAADMismatch)
}

func TestRepository_Create_EmptyTokenStaysEmpty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertMcpServer)).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "",
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.McpServer{ID: testMcpID, OrganizationID: testOrgID5})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + is_enabled×2 = 7
// listArgs: 7 + limit + offset = 9
func TestRepository_List_WithEnabled(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	enabled := true
	countArgs := []driver.Value{testOrgID5, testOrgID5, "kw", "kw", "kw", enabled, enabled}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountMcpServers)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListMcpServers)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(mcpCols()).AddRow(newMcpRow()...))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID5,
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

func TestRepository_List_NilEnabled(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", nil, nil}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountMcpServers)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListMcpServers)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(mcpCols()))

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

	toolsJSON, _ := json.Marshal([]domain.Tool{
		{Name: "search", Description: "Search tool", IsAllowed: true},
	})

	mock.ExpectExec(regexp.QuoteMeta(queryInsertMcpServer)).
		WithArgs(
			testMcpID, testOrgID5, "my-mcp", "My MCP", "https://mcp.example.com",
			sealedMcpTokenArg{plaintext: "tok-123", serverID: testMcpID}, "app-y",
			string(toolsJSON), true, fixedNow5, fixedNow5, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Create(context.Background(), &domain.McpServer{
		ID:             testMcpID,
		OrganizationID: testOrgID5,
		Name:           "my-mcp",
		DisplayName:    "My MCP",
		URL:            "https://mcp.example.com",
		Token:          "tok-123",
		Application:    "app-y",
		Tools: []domain.Tool{
			{Name: "search", Description: "Search tool", IsAllowed: true},
		},
		IsEnabled: true,
		CreatedAt: fixedNow5,
		UpdatedAt: fixedNow5,
		Revision:  int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	toolsJSON, _ := json.Marshal([]domain.Tool{
		{Name: "write", Description: "Write tool", IsAllowed: false},
	})

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateMcpServer)).
		WithArgs(
			"Updated", "https://new-mcp.example.com",
			sealedMcpTokenArg{plaintext: "new-tok", serverID: testMcpID}, "new-app",
			string(toolsJSON), false, fixedNow5, int64(2), testMcpID, testOrgID5,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.McpServer{
		ID:             testMcpID,
		OrganizationID: testOrgID5,
		DisplayName:    "Updated",
		URL:            "https://new-mcp.example.com",
		Token:          "new-tok",
		Application:    "new-app",
		Tools: []domain.Tool{
			{Name: "write", Description: "Write tool", IsAllowed: false},
		},
		IsEnabled: false,
		UpdatedAt: fixedNow5,
		Revision:  int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteMcpServer)).
		WithArgs(testMcpID, testOrgID5).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testMcpID, testOrgID5)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
