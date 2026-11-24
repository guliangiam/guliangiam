package radiusaccountings

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	domain "github.com/guliangiam/guliangiam/internal/domain/radiusaccountings"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── 基础设施 ────────────────────────────────────────────────────

var (
	fixedNow8   = time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	fixedStop   = time.Date(2026, 9, 24, 10, 30, 0, 0, time.UTC)
	testOrgID8  = "00000000-0000-0000-0000-000000000001"
	testRAID    = "00000000-0000-0000-0000-000000000006"
	sessionStop = fixedStop
)

func newRAMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	return db, mock
}

// raColsFull: 25 列（GetByID / Create / Update）
func raColsFull() []string {
	return []string{
		"id", "organization_id", "name", "username", "service_type", "nas_id", "nas_ip_addr",
		"nas_port_id", "nas_port_type", "nas_port", "framed_ip_addr", "framed_ip_netmask",
		"acct_session_id", "acct_session_time", "acct_input_total", "acct_output_total",
		"acct_input_packets", "acct_output_packets", "acct_terminate_cause", "acct_start_time",
		"acct_stop_time", "last_update", "created_at", "updated_at", "revision",
	}
}

// raColsList: 7 列（List 摘要）
func raColsList() []string {
	return []string{
		"id", "organization_id", "name", "username", "acct_session_id", "acct_start_time", "created_at",
	}
}

func newRARow() []driver.Value {
	return []driver.Value{
		testRAID, testOrgID8, "ra-001", "alice", int64(2), "nas-1", "10.0.0.1",
		"port-id-01", int64(15), int64(1), "192.168.1.100", "255.255.255.0",
		"sess-001", int64(1800), int64(1024000), int64(2048000),
		int64(1000), int64(2000), int64(1), fixedNow8,
		sessionStop, fixedNow8, fixedNow8, fixedNow8, int64(1),
	}
}

// ── GetByID ───────────────────────────────────────────────────────

func TestRepository_GetByID_Found(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRAByID)).
		WithArgs(testRAID, testOrgID8).
		WillReturnRows(sqlmock.NewRows(raColsFull()).AddRow(newRARow()...))

	r := NewRepository(db)
	ra, err := r.GetByID(context.Background(), testRAID, testOrgID8)
	assert.NoError(t, err)
	assert.NotNil(t, ra)
	assert.Equal(t, testRAID, ra.ID)
	assert.Equal(t, "alice", ra.Username)
	assert.NotNil(t, ra.AcctStopTime)
	assert.Equal(t, fixedStop, *ra.AcctStopTime)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_GetByID_NotFound(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta(queryGetRAByID)).
		WithArgs("bad", "other-org").
		WillReturnRows(sqlmock.NewRows(raColsFull()))

	r := NewRepository(db)
	ra, err := r.GetByID(context.Background(), "bad", "other-org")
	assert.Error(t, err)
	assert.Nil(t, ra)
}

// ── List ──────────────────────────────────────────────────────────

// countArgs: org_id×2 + keyword×3 + username×2 = 7
// listArgs: 7 + limit + offset = 9
func TestRepository_List_WithAllFilters(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{testOrgID8, testOrgID8, "kw", "kw", "kw", "alice", "alice"}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRA)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(1)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRA)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(raColsList()).AddRow(
			testRAID, testOrgID8, "ra-001", "alice", "sess-001", fixedNow8, fixedNow8,
		))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{
		OrganizationID: testOrgID8,
		Keyword:        "kw",
		Username:       "alice",
		Page:           1,
		PageSize:       20,
	})
	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, int64(1), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_List_Empty(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	countArgs := []driver.Value{"", "", "", "", "", "", ""}
	mock.ExpectQuery(regexp.QuoteMeta(queryCountRA)).WithArgs(countArgs...).
		WillReturnRows(sqlmock.NewRows([]string{"c"}).AddRow(int64(0)))

	listArgs := append(append([]driver.Value{}, countArgs...), 20, 0)
	mock.ExpectQuery(regexp.QuoteMeta(queryListRA)).WithArgs(listArgs...).
		WillReturnRows(sqlmock.NewRows(raColsList()))

	r := NewRepository(db)
	items, total, err := r.List(context.Background(), domain.ListQuery{Page: 1, PageSize: 20})
	assert.NoError(t, err)
	assert.Len(t, items, 0)
	assert.Equal(t, int64(0), total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Create ────────────────────────────────────────────────────────

func TestRepository_Create(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryInsertRA)).
		WithArgs(
			testRAID, testOrgID8, "ra-001", "alice", int64(2), "nas-1", "10.0.0.1",
			"port-id-01", int64(15), int64(1), "192.168.1.100", "255.255.255.0",
			"sess-001", int64(1800), int64(1024000), int64(2048000),
			int64(1000), int64(2000), int64(1), fixedNow8,
			&sessionStop, fixedNow8, fixedNow8, fixedNow8, int64(1),
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	ra := &domain.RadiusAccounting{
		ID:                 testRAID,
		OrganizationID:     testOrgID8,
		Name:               "ra-001",
		Username:           "alice",
		ServiceType:        int64(2),
		NasID:              "nas-1",
		NasIPAddr:          "10.0.0.1",
		NasPortID:          "port-id-01",
		NasPortType:        int64(15),
		NasPort:            int64(1),
		FramedIPAddr:       "192.168.1.100",
		FramedIPNetmask:    "255.255.255.0",
		AcctSessionID:      "sess-001",
		AcctSessionTime:    int64(1800),
		AcctInputTotal:     int64(1024000),
		AcctOutputTotal:    int64(2048000),
		AcctInputPackets:   int64(1000),
		AcctOutputPackets:  int64(2000),
		AcctTerminateCause: int64(1),
		AcctStartTime:      fixedNow8,
		AcctStopTime:       &sessionStop,
		LastUpdate:         fixedNow8,
		CreatedAt:          fixedNow8,
		UpdatedAt:          fixedNow8,
		Revision:           int64(1),
	}
	err := r.Create(context.Background(), ra)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Update ────────────────────────────────────────────────────────

func TestRepository_Update(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryUpdateRA)).
		WithArgs(
			int64(3600), int64(2048000), int64(4096000), int64(2000), int64(4000),
			int64(6), &sessionStop, fixedNow8, fixedNow8, int64(2), testRAID, testOrgID8,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Update(context.Background(), &domain.RadiusAccounting{
		ID:                 testRAID,
		OrganizationID:     testOrgID8,
		AcctSessionTime:    int64(3600),
		AcctInputTotal:     int64(2048000),
		AcctOutputTotal:    int64(4096000),
		AcctInputPackets:   int64(2000),
		AcctOutputPackets:  int64(4000),
		AcctTerminateCause: int64(6),
		AcctStopTime:       &sessionStop,
		LastUpdate:         fixedNow8,
		UpdatedAt:          fixedNow8,
		Revision:           int64(2),
	})
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── Delete ────────────────────────────────────────────────────────

func TestRepository_Delete(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRA)).
		WithArgs(testRAID, testOrgID8).
		WillReturnResult(sqlmock.NewResult(1, 1))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRAID, testOrgID8)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRepository_Delete_ForeignOrganizationIsNotFound(t *testing.T) {
	db, mock := newRAMockDB(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta(queryDeleteRA)).
		WithArgs(testRAID, "other-org").
		WillReturnResult(sqlmock.NewResult(1, 0))

	r := NewRepository(db)
	err := r.Delete(context.Background(), testRAID, "other-org")
	var appErr *apperrors.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, apperrors.CodeNotFound, appErr.Code)
}

// 编译期接口断言
var _ domain.Repository = Repository{}
