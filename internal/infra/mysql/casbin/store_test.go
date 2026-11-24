package casbin

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testModel = `[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, dom, obj, act

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && r.dom == p.dom && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
`

func newMockStore(t *testing.T) (Store, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	return Store{db: db, enforcerID: "enf-1"}, mock, db
}

func anyArgs(n int) []driver.Value {
	out := make([]driver.Value, n)
	for i := range out {
		out[i] = sqlmock.AnyArg()
	}
	return out
}

func newModel(t *testing.T) casbinmodel.Model {
	t.Helper()
	m, err := casbinmodel.NewModelFromString(testModel)
	require.NoError(t, err)
	return m
}

func TestStore_LoadPolicy_SkipsJITRows(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT ptype, v0, v1, v2, v3, v4, v5 FROM `casbin_rules`")).
		WithArgs("enf-1").
		WillReturnRows(sqlmock.NewRows([]string{"ptype", "v0", "v1", "v2", "v3", "v4", "v5"}).
			AddRow("p", "admin", "built-in", "*", "*", "", "").
			AddRow("g", "alice", "admin", "built-in", "", "", ""))

	m := newModel(t)
	require.NoError(t, store.LoadPolicy(m))
	assert.Len(t, m["p"]["p"].Policy, 1)
	assert.Len(t, m["g"]["g"].Policy, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_SavePolicy_RewritesInTransaction(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `casbin_rules` WHERE enforcer_id = ?")).
		WithArgs("enf-1").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `casbin_rules`")).
		WithArgs(anyArgs(9)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	m := newModel(t)
	require.NoError(t, m.AddPolicy("p", "p", []string{"admin", "built-in", "*", "*"}))

	require.NoError(t, store.SavePolicy(m))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_AddPolicy_GeneratesID(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `casbin_rules`")).
		WithArgs(anyArgs(9)...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.AddPolicy("p", "p", []string{"alice", "built-in", "users", "read"}))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_RemovePolicy_UsesEmptyAsWildcard(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `casbin_rules` WHERE enforcer_id = ?")).
		WithArgs("enf-1", "p", "alice", "built-in", "", "", "", "", "", "", "", "").
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, store.RemovePolicy("p", "p", []string{"alice", "built-in"}))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_RemoveFilteredPolicy_OnlyAppendsProvidedFields(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `casbin_rules` WHERE enforcer_id = ? AND ptype = ? AND v2 = ?")).
		WithArgs("enf-1", "p", "*").
		WillReturnResult(sqlmock.NewResult(0, 3))

	require.NoError(t, store.RemoveFilteredPolicy("p", "p", 2, "*"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_ReplaceUserGroupings_ReplacesUserRows(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM `casbin_rules` WHERE enforcer_id = ? AND ptype = 'g' AND v0 = ?")).
		WithArgs("enf-1", "alice").
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `casbin_rules`")).
		WithArgs(anyArgs(5)...).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	require.NoError(t, store.ReplaceUserGroupings(context.Background(), "alice", "built-in", []string{"admin"}))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_EnforceJIT_EvaluatesWindowInGo(t *testing.T) {
	now := time.Now().UTC()
	meta := func(lifecycle string, start time.Time) string {
		return `{"source":"iga_jit","lifecycleId":"` + lifecycle + `","startsAt":"` + start.Format(time.RFC3339Nano) + `"}`
	}
	rows := func(values ...[]string) *sqlmock.Rows {
		out := sqlmock.NewRows([]string{"v0", "v1", "v2", "v3", "v4", "v5"})
		for _, v := range values {
			out.AddRow(v[0], v[1], v[2], v[3], v[4], v[5])
		}
		return out
	}
	const (
		sub = "alice"
		dom = "built-in"
		obj = "users"
		act = "read"
	)
	cases := []struct {
		name    string
		row     []string
		allowed bool
	}{
		{
			name:    "inside window",
			row:     []string{sub, dom, obj, act, now.Add(time.Hour).Format(time.RFC3339Nano), meta("lc-1", now.Add(-time.Minute))},
			allowed: true,
		},
		{
			name:    "expired",
			row:     []string{sub, dom, obj, act, now.Add(-time.Minute).Format(time.RFC3339Nano), meta("lc-1", now.Add(-time.Hour))},
			allowed: false,
		},
		{
			name:    "not started",
			row:     []string{sub, dom, obj, act, now.Add(time.Hour).Format(time.RFC3339Nano), meta("lc-1", now.Add(time.Minute))},
			allowed: false,
		},
		{
			name:    "other lifecycle",
			row:     []string{sub, dom, obj, act, now.Add(time.Hour).Format(time.RFC3339Nano), meta("lc-2", now.Add(-time.Minute))},
			allowed: false,
		},
		{
			name:    "broken metadata",
			row:     []string{sub, dom, obj, act, now.Add(time.Hour).Format(time.RFC3339Nano), "{not-json"},
			allowed: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, mock, db := newMockStore(t)
			defer db.Close()

			mock.ExpectQuery(regexp.QuoteMeta("SELECT v0, v1, v2, v3, v4, v5 FROM `casbin_rules`")).
				WithArgs("enf-1", sub, dom, obj, act).
				WillReturnRows(rows(tc.row))

			allowed, err := store.EnforceJIT(context.Background(), "enf-1", "lc-1", sub, dom, obj, act)
			require.NoError(t, err)
			assert.Equal(t, tc.allowed, allowed)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestStore_EnforceJIT_RequiresLifecycleAndFourValues(t *testing.T) {
	store, _, db := newMockStore(t)
	defer db.Close()

	allowed, err := store.EnforceJIT(context.Background(), "enf-1", "", "alice", "built-in", "users", "read")
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, err = store.EnforceJIT(context.Background(), "enf-1", "lc-1", "alice")
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestStore_ResolveEnforcerID(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM `enforcers`")).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("enf-1"))

	id, err := store.ResolveEnforcerID(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "enf-1", id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestStore_ResolveEnforcerID_NoRows(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM `enforcers`")).
		WithArgs("org-1").
		WillReturnError(sql.ErrNoRows)

	_, err := store.ResolveEnforcerID(context.Background(), "org-1")
	assert.Error(t, err)
}

func TestStore_ResolveEnforcerUUID_QueriesSameTable(t *testing.T) {
	store, mock, db := newMockStore(t)
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM `enforcers`")).
		WithArgs("org-1").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("enf-1"))

	id, err := store.ResolveEnforcerUUID(context.Background(), "org-1")
	require.NoError(t, err)
	assert.Equal(t, "enf-1", id)
	assert.NoError(t, mock.ExpectationsWereMet())
}
