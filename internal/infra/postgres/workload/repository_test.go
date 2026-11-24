package workload

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/workload"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var postgresWorkloadNow = time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)

type workloadRow struct {
	values []any
	err    error
}

func (r workloadRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return scanWorkloadValues(r.values, dest)
}

type workloadRows struct {
	rows [][]any
	idx  int
}

func (r *workloadRows) Close()                                       {}
func (r *workloadRows) Err() error                                   { return nil }
func (r *workloadRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *workloadRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *workloadRows) TypeMap() *pgtype.Map                         { return nil }
func (r *workloadRows) Conn() *pgx.Conn                              { return nil }
func (r *workloadRows) RawValues() [][]byte                          { return nil }
func (r *workloadRows) Next() bool {
	if r.idx >= len(r.rows) {
		return false
	}
	r.idx++
	return true
}
func (r *workloadRows) Scan(dest ...any) error {
	if r.idx == 0 || r.idx > len(r.rows) {
		return errors.New("no current row")
	}
	return scanWorkloadValues(r.rows[r.idx-1], dest)
}
func (r *workloadRows) Values() ([]any, error) {
	if r.idx == 0 || r.idx > len(r.rows) {
		return nil, errors.New("no current row")
	}
	return r.rows[r.idx-1], nil
}

func scanWorkloadValues(values, dest []any) error {
	if len(values) != len(dest) {
		return fmt.Errorf("got %d scan values for %d destinations", len(values), len(dest))
	}
	for i, value := range values {
		if value == nil {
			continue
		}
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return fmt.Errorf("destination %d is not a pointer", i)
		}
		if err := setWorkloadValue(target.Elem(), reflect.ValueOf(value)); err != nil {
			return fmt.Errorf("scan column %d: %w", i, err)
		}
	}
	return nil
}

func setWorkloadValue(target, source reflect.Value) error {
	if target.Kind() == reflect.Pointer {
		value := reflect.New(target.Type().Elem())
		if err := setWorkloadValue(value.Elem(), source); err != nil {
			return err
		}
		target.Set(value)
		return nil
	}
	if source.Type().AssignableTo(target.Type()) {
		target.Set(source)
		return nil
	}
	if source.Type().ConvertibleTo(target.Type()) {
		target.Set(source.Convert(target.Type()))
		return nil
	}
	return fmt.Errorf("cannot assign %s to %s", source.Type(), target.Type())
}

func postgresTrustValues() []any {
	return []any{"trust", "org", "pool", "trust", "https://issuer.example.com", "https://issuer.example.com/jwks", []string{"api"}, []string{"repo:org/*"}, "active", postgresWorkloadNow, postgresWorkloadNow, int64(1)}
}

func postgresAccountValues() []any {
	return []any{"sa", "org", "pool", "robot", "CI bot", "gl_cid_test", "hash", []string{"orders:read"}, 900, postgresWorkloadNow, "active", postgresWorkloadNow, postgresWorkloadNow, int64(2)}
}

func TestRepository_CreateTrustSQL(t *testing.T) {
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
		assert.Equal(t, qInsertTrust, statement)
		assert.Equal(t, []any{"trust", "org", "pool", "trust", "https://issuer.example.com", "https://issuer.example.com/jwks", []string{"api"}, []string{"repo:org/*"}}, args)
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}}
	err := NewRepository(db).CreateTrust(context.Background(), &domain.FederationTrust{ID: "trust", OrganizationID: "org", PoolID: "pool", Name: "trust", Issuer: "https://issuer.example.com", JWKSURI: "https://issuer.example.com/jwks", Audience: []string{"api"}, Subjects: []string{"repo:org/*"}})
	require.NoError(t, err)
}

func TestRepository_ListTrustsWithoutResolverFilters(t *testing.T) {
	db := &shared.MockDBTX{
		QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
			assert.Equal(t, qCountTrusts, statement)
			assert.Equal(t, []any{"", ""}, args)
			return workloadRow{values: []any{int64(1)}}
		},
		QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
			assert.Equal(t, qListTrusts+" LIMIT 20 OFFSET 0", statement)
			assert.Equal(t, []any{"", ""}, args)
			return &workloadRows{rows: [][]any{postgresTrustValues()}}, nil
		},
	}
	items, total, err := NewRepository(db).ListTrusts(context.Background(), domain.ListTrustsQuery{Page: 1, PageSize: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	assert.Equal(t, []string{"repo:org/*"}, items[0].Subjects)
}

func TestListTrustsSQLHasOptionalFilters(t *testing.T) {
	assert.Contains(t, qListTrusts, "organization_id = NULLIF($1, '')::uuid")
	assert.Contains(t, qListTrusts, "pool_id = NULLIF($2, '')::uuid")
}

func TestRepository_FindActiveTrustsByIssuer(t *testing.T) {
	db := &shared.MockDBTX{QueryFn: func(_ context.Context, statement string, args ...any) (pgx.Rows, error) {
		assert.Equal(t, qFindActiveTrustsByIssuer, statement)
		assert.Equal(t, []any{"https://issuer.example.com"}, args)
		return &workloadRows{rows: [][]any{postgresTrustValues()}}, nil
	}}
	items, err := NewRepository(db).FindActiveTrustsByIssuer(context.Background(), "https://issuer.example.com")
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "trust", items[0].ID)
}

func TestRepository_CreateAccountSQL(t *testing.T) {
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
		assert.Equal(t, qInsertAccount, statement)
		assert.Equal(t, []any{"sa", "org", "pool", "robot", "CI bot", "gl_cid_test", "hash", []string{"orders:read"}, 900}, args)
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}}
	err := NewRepository(db).CreateAccount(context.Background(), &domain.ServiceAccount{ID: "sa", OrganizationID: "org", PoolID: "pool", Name: "robot", Description: "CI bot", ClientID: "gl_cid_test", ClientSecretHash: "hash", Scopes: []string{"orders:read"}, AccessTokenTTLSeconds: 900})
	require.NoError(t, err)
}

func TestRepository_GetAccountByClientIDScan(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
		assert.Equal(t, qGetAccountByClientID, statement)
		assert.Equal(t, []any{"gl_cid_test"}, args)
		return workloadRow{values: postgresAccountValues()}
	}}
	account, err := NewRepository(db).GetAccountByClientID(context.Background(), "gl_cid_test")
	require.NoError(t, err)
	assert.Equal(t, "CI bot", account.Description)
	assert.Equal(t, []string{"orders:read"}, account.Scopes)
	assert.Equal(t, 900, account.AccessTokenTTLSeconds)
	require.NotNil(t, account.LastRotatedAt)
	assert.Equal(t, postgresWorkloadNow, *account.LastRotatedAt)
	assert.Equal(t, "hash", account.ClientSecretHash)
}

func TestRepository_GetAccountByNameScan(t *testing.T) {
	db := &shared.MockDBTX{QueryRowFn: func(_ context.Context, statement string, args ...any) pgx.Row {
		assert.Equal(t, qGetAccountByName, statement)
		assert.Equal(t, []any{"org", "robot"}, args)
		return workloadRow{values: postgresAccountValues()}
	}}
	account, err := NewRepository(db).GetAccountByName(context.Background(), "org", "robot")
	require.NoError(t, err)
	assert.Equal(t, "sa", account.ID)
	assert.Equal(t, "robot", account.Name)
}

func TestRepository_AccountLifecycleSQL(t *testing.T) {
	rotatedAt := postgresWorkloadNow.Add(time.Hour)
	tests := []struct {
		name      string
		statement string
		args      []any
		call      func(Repository) error
	}{
		{name: "status", statement: qUpdateAccountStatus, args: []any{"disabled", "sa", "org", int64(2)}, call: func(r Repository) error {
			return r.SetAccountStatus(context.Background(), "sa", "org", domain.SAStatusDisabled, 2)
		}},
		{name: "rotate", statement: qRotateAccountSecret, args: []any{"new-hash", rotatedAt, "sa", "org", int64(2)}, call: func(r Repository) error {
			return r.RotateAccountSecret(context.Background(), "sa", "org", "new-hash", rotatedAt, 2)
		}},
		{name: "delete", statement: qDeleteAccount, args: []any{"sa", "org", int64(3)}, call: func(r Repository) error {
			return r.DeleteAccount(context.Background(), "sa", "org", 3)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
				assert.Equal(t, test.statement, statement)
				assert.Equal(t, test.args, args)
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}}
			require.NoError(t, test.call(NewRepository(db)))
		})
	}
}

func TestRepository_AccountLifecycleRevisionConflict(t *testing.T) {
	db := &shared.MockDBTX{ExecFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
		return pgconn.NewCommandTag("UPDATE 0"), nil
	}}
	err := NewRepository(db).SetAccountStatus(context.Background(), "sa", "org", domain.SAStatusActive, 1)
	assert.ErrorContains(t, err, "revision conflict")
}

func TestRepository_TrustStatusAndDeleteSQL(t *testing.T) {
	calls := 0
	db := &shared.MockDBTX{ExecFn: func(_ context.Context, statement string, args ...any) (pgconn.CommandTag, error) {
		calls++
		if calls == 1 {
			assert.Equal(t, qUpdateTrustStatus, statement)
			assert.Equal(t, []any{"disabled", "trust", "org"}, args)
		} else {
			assert.Equal(t, qDeleteTrust, statement)
			assert.Equal(t, []any{"trust", "org"}, args)
		}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	}}
	repository := NewRepository(db)
	require.NoError(t, repository.SetTrustStatus(context.Background(), "trust", "org", domain.TrustStatusDisabled))
	require.NoError(t, repository.DeleteTrust(context.Background(), "trust", "org"))
}
