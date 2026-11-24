package gateway

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/gateway"
	"github.com/stretchr/testify/assert"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow     = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	testOrgID    = "00000000-0000-0000-0000-000000000001"
	testServerID = "00000000-0000-0000-0000-000000000002"
	testSiteID   = "00000000-0000-0000-0000-000000000003"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// ── Servers 列定义（11 列）────────────────────────────────────────

func serverCols() []string {
	return []string{
		"id", "organization_id", "name", "endpoint", "protocol", "status",
		"metadata", "created_at", "updated_at", "deleted_at", "revision",
	}
}

func newServerRow() []driver.Value {
	return []driver.Value{
		testServerID, testOrgID, "auth-server", "https://auth.example.com", "https", "active",
		`{"region":"us-east"}`, fixedNow, fixedNow, nil, int64(2),
	}
}

// ── Sites 列定义（21 列）──────────────────────────────────────────

func siteCols() []string {
	return []string{
		"id", "organization_id", "name", "host", "status", "ssl_mode", "cert_id", "force_https",
		"enable_health_check", "health_check_interval", "last_health_status", "last_health_at",
		"enable_alert", "alert_interval", "alert_try_times", "alert_provider_ids",
		"metadata", "created_at", "updated_at", "deleted_at", "revision",
	}
}

func newSiteRow() []driver.Value {
	return []driver.Value{
		testSiteID, testOrgID, "portal", "portal.example.com", "active", "auto", "cert-001", true,
		true, 30, "healthy", fixedNow,
		true, 300, 3, `["prov-001"]`,
		`{"owner":"alice"}`, fixedNow, fixedNow, nil, int64(1),
	}
}

// ── Servers: GetServerByID ───────────────────────────────────────

func TestRepository_GetServerByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetServerByID)).
		WithArgs(testServerID).
		WillReturnRows(sqlmock.NewRows(serverCols()).AddRow(newServerRow()...))

	r := NewRepository(db)
	s, err := r.GetServerByID(context.Background(), testServerID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testServerID, s.ID)
	assert.Equal(t, "active", s.Status)
	assert.Equal(t, int64(2), s.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetServerByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetServerByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(serverCols()))

	r := NewRepository(db)
	s, err := r.GetServerByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, s)
}

// ── Servers: ListServers ─────────────────────────────────────────

// List 参数 9 个：org_id×2, keyword×3, status×2, limit, offset
// Count 参数 7 个：同上去掉 limit/offset
func TestRepository_ListServers(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	listArgs := []driver.Value{
		"", "", // org_id×2
		"", "", "", // keyword×3
		"", "", // status×2
		20, 0, // limit, offset
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryListServers)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(serverCols()).AddRow(newServerRow()...))

	countArgs := []driver.Value{"", "", "", "", "", "", ""} // 7
	mock.ExpectQuery(regexp.QuoteMeta(queryCountServers)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.ListServers(context.Background(), domain.ListServersQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Servers: CreateServer ────────────────────────────────────────

func TestRepository_CreateServer(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertServer)).
		WithArgs(
			testServerID, testOrgID, "auth-server", "https://auth.example.com", "https",
			"active", `{"region":"us-east"}`, fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateServer(context.Background(), &domain.Server{
		ID:             testServerID,
		OrganizationID: testOrgID,
		Name:           "auth-server",
		Endpoint:       "https://auth.example.com",
		Protocol:       "https",
		Status:         "active",
		Metadata:       map[string]any{"region": "us-east"},
		CreatedAt:      fixedNow,
		UpdatedAt:      fixedNow,
		Revision:       int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Servers: UpdateServer ────────────────────────────────────────

func TestRepository_UpdateServer_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateServer)).
		WithArgs(
			"auth-server-updated", "https://new.example.com", "https", "active",
			`{"region":"eu-west"}`, fixedNow, testServerID, int64(2),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectServerRevision)).
		WithArgs(testServerID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(3)))

	r := NewRepository(db)
	s := &domain.Server{
		ID:        testServerID,
		Name:      "auth-server-updated",
		Endpoint:  "https://new.example.com",
		Protocol:  "https",
		Status:    "active",
		Metadata:  map[string]any{"region": "eu-west"},
		UpdatedAt: fixedNow,
		Revision:  int64(2),
	}
	err := r.UpdateServer(context.Background(), s)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), s.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateServer_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateServer)).
		WithArgs(
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), testServerID, int64(2),
		).
		WillReturnResult(sqlmock.NewResult(1, 0)) // 0 affected → conflict

	r := NewRepository(db)
	err := r.UpdateServer(context.Background(), &domain.Server{
		ID:       testServerID,
		Revision: int64(2),
	})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Servers: DeleteServer ────────────────────────────────────────

func TestRepository_DeleteServer(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteServer)).
		WithArgs(testServerID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteServer(context.Background(), testServerID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Sites: GetSiteByID ───────────────────────────────────────────

func TestRepository_GetSiteByID_Found(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetSiteByID)).
		WithArgs(testSiteID).
		WillReturnRows(sqlmock.NewRows(siteCols()).AddRow(newSiteRow()...))

	r := NewRepository(db)
	s, err := r.GetSiteByID(context.Background(), testSiteID)
	assert.NoError(t, err)
	assert.NotNil(t, s)
	assert.Equal(t, testSiteID, s.ID)
	assert.Equal(t, "portal.example.com", s.Host)
	assert.Equal(t, int64(1), s.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetSiteByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetSiteByID)).
		WithArgs("bad").
		WillReturnRows(sqlmock.NewRows(siteCols()))

	r := NewRepository(db)
	s, err := r.GetSiteByID(context.Background(), "bad")
	assert.Error(t, err)
	assert.Nil(t, s)
}

// ── Sites: ListSites ────────────────────────────────────────────

func TestRepository_ListSites(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	listArgs := []driver.Value{
		testOrgID, testOrgID,
		"my-keyword", "my-keyword", "my-keyword",
		"active", "active",
		20, 0,
	}
	mock.ExpectQuery(regexp.QuoteMeta(queryListSites)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(siteCols()).AddRow(newSiteRow()...))

	countArgs := []driver.Value{testOrgID, testOrgID, "my-keyword", "my-keyword", "my-keyword", "active", "active"}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountSites)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	r := NewRepository(db)
	items, total, err := r.ListSites(context.Background(), domain.ListSitesQuery{
		OrganizationID: testOrgID,
		Keyword:        "my-keyword",
		Status:         "active",
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Sites: CreateSite ────────────────────────────────────────────

func TestRepository_CreateSite(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	alertJSON, _ := json.Marshal([]string{"prov-001"})
	metadataJSON, _ := json.Marshal(map[string]any{"owner": "alice"})

	mock.ExpectExec(regexp.QuoteMeta(queryInsertSite)).
		WithArgs(
			testSiteID, testOrgID, "portal", "portal.example.com",
			"active", "auto", "cert-001", true,
			true, 30, "healthy", fixedNow,
			true, 300, 3, string(alertJSON),
			string(metadataJSON), fixedNow, fixedNow, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.CreateSite(context.Background(), &domain.Site{
		ID:                  testSiteID,
		OrganizationID:      testOrgID,
		Name:                "portal",
		Host:                "portal.example.com",
		Status:              "active",
		SslMode:             "auto",
		CertID:              "cert-001",
		ForceHttps:          true,
		EnableHealthCheck:   true,
		HealthCheckInterval: 30,
		LastHealthStatus:    "healthy",
		LastHealthAt:        &fixedNow,
		EnableAlert:         true,
		AlertInterval:       300,
		AlertTryTimes:       3,
		AlertProviderIDs:    []string{"prov-001"},
		Metadata:            map[string]any{"owner": "alice"},
		CreatedAt:           fixedNow,
		UpdatedAt:           fixedNow,
		Revision:            int64(1),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Sites: UpdateSite ────────────────────────────────────────────

func TestRepository_UpdateSite_Success(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateSite)).
		WithArgs(
			"portal-updated", "new-portal.example.com", "active",
			"manual", "cert-002", false,
			true, 60, "healthy", fixedNow,
			false, 0, 0, "[]",
			`{}`,
			fixedNow, testSiteID, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(regexp.QuoteMeta(querySelectSiteRevision)).
		WithArgs(testSiteID).
		WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow(int64(2)))

	r := NewRepository(db)
	s := &domain.Site{
		ID:                  testSiteID,
		Name:                "portal-updated",
		Host:                "new-portal.example.com",
		Status:              "active",
		SslMode:             "manual",
		CertID:              "cert-002",
		ForceHttps:          false,
		EnableHealthCheck:   true,
		HealthCheckInterval: 60,
		LastHealthStatus:    "healthy",
		LastHealthAt:        &fixedNow,
		EnableAlert:         false,
		AlertProviderIDs:    []string{},
		Metadata:            map[string]any{},
		UpdatedAt:           fixedNow,
		Revision:            int64(1),
	}
	err := r.UpdateSite(context.Background(), s)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), s.Revision)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_UpdateSite_Conflict(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateSite)).
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.UpdateSite(context.Background(), &domain.Site{
		ID:       testSiteID,
		Revision: int64(1),
	})
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Sites: DeleteSite ────────────────────────────────────────────

func TestRepository_DeleteSite(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteSite)).
		WithArgs(testSiteID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.DeleteSite(context.Background(), testSiteID)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// 编译期接口断言
var _ domain.Repository = Repository{}
