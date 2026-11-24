package shared

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockDBTX_Exec_WithFn(t *testing.T) {
	called := false
	m := &MockDBTX{
		ExecFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
			called = true
			assert.Equal(t, "INSERT INTO users", sql)
			return pgconn.CommandTag{}, nil
		},
	}
	tag, err := m.Exec(context.Background(), "INSERT INTO users")
	assert.True(t, called)
	assert.NoError(t, err)
	assert.NotNil(t, tag)
}

func TestMockDBTX_Exec_Default(t *testing.T) {
	m := &MockDBTX{}
	tag, err := m.Exec(context.Background(), "SELECT 1")
	assert.NoError(t, err)
	assert.Equal(t, pgconn.CommandTag{}, tag)
}

func TestMockDBTX_Query_WithFn(t *testing.T) {
	called := false
	m := &MockDBTX{
		QueryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
			called = true
			return &MockRows{}, nil
		},
	}
	rows, err := m.Query(context.Background(), "SELECT * FROM users")
	assert.True(t, called)
	assert.NoError(t, err)
	assert.NotNil(t, rows)
}

func TestMockDBTX_Query_Default(t *testing.T) {
	m := &MockDBTX{}
	rows, err := m.Query(context.Background(), "SELECT 1")
	assert.NoError(t, err)
	assert.NotNil(t, rows)
}

func TestMockDBTX_QueryRow_WithFn(t *testing.T) {
	called := false
	m := &MockDBTX{
		QueryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
			called = true
			return NewMockRow([]any{"value"}, nil)
		},
	}
	row := m.QueryRow(context.Background(), "SELECT name FROM users")
	assert.True(t, called)
	assert.NotNil(t, row)
}

func TestMockDBTX_QueryRow_Default(t *testing.T) {
	m := &MockDBTX{}
	row := m.QueryRow(context.Background(), "SELECT 1")
	assert.NotNil(t, row)
}

func TestNewMockRow(t *testing.T) {
	row := NewMockRow([]any{"hello", 42}, nil)
	assert.NotNil(t, row)
	assert.Equal(t, []any{"hello", 42}, row.values)
	assert.Nil(t, row.err)
}

func TestNewMockRow_WithError(t *testing.T) {
	err := errors.New("db error")
	row := NewMockRow(nil, err)
	assert.NotNil(t, row)
	assert.Equal(t, err, row.err)
}

func TestMockRow_Scan_Error(t *testing.T) {
	dbErr := errors.New("connection lost")
	row := NewMockRow(nil, dbErr)
	err := row.Scan()
	assert.Equal(t, dbErr, err)
}

func TestMockRow_Scan_NotEnoughValues(t *testing.T) {
	row := NewMockRow([]any{"only_one"}, nil)
	var s1, s2 string
	err := row.Scan(&s1, &s2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough values")
}

func TestMockRow_Scan_String(t *testing.T) {
	row := NewMockRow([]any{"hello"}, nil)
	var s string
	err := row.Scan(&s)
	assert.NoError(t, err)
	assert.Equal(t, "hello", s)
}

func TestMockRow_Scan_Int(t *testing.T) {
	row := NewMockRow([]any{42}, nil)
	var n int
	err := row.Scan(&n)
	assert.NoError(t, err)
	assert.Equal(t, 42, n)
}

func TestMockRow_Scan_Int64(t *testing.T) {
	row := NewMockRow([]any{int64(999)}, nil)
	var n int64
	err := row.Scan(&n)
	assert.NoError(t, err)
	assert.Equal(t, int64(999), n)
}

func TestMockRow_Scan_Float64(t *testing.T) {
	row := NewMockRow([]any{3.14}, nil)
	var f float64
	err := row.Scan(&f)
	assert.NoError(t, err)
	assert.Equal(t, 3.14, f)
}

func TestMockRow_Scan_Bool(t *testing.T) {
	row := NewMockRow([]any{true}, nil)
	var b bool
	err := row.Scan(&b)
	assert.NoError(t, err)
	assert.True(t, b)
}

func TestMockRow_Scan_ByteSlice(t *testing.T) {
	row := NewMockRow([]any{[]byte("bytes")}, nil)
	var b []byte
	err := row.Scan(&b)
	assert.NoError(t, err)
	assert.Equal(t, []byte("bytes"), b)
}

func TestMockRow_Scan_NilValue(t *testing.T) {
	row := NewMockRow([]any{nil, "hello"}, nil)
	var s1, s2 string
	err := row.Scan(&s1, &s2)
	assert.NoError(t, err)
	assert.Equal(t, "", s1) // nil doesn't set
	assert.Equal(t, "hello", s2)
}

func TestMockRow_Scan_MultipleTypes(t *testing.T) {
	row := NewMockRow([]any{"hello", 42, true, 3.14}, nil)
	var s string
	var n int
	var b bool
	var f float64
	err := row.Scan(&s, &n, &b, &f)
	assert.NoError(t, err)
	assert.Equal(t, "hello", s)
	assert.Equal(t, 42, n)
	assert.True(t, b)
	assert.Equal(t, 3.14, f)
}

func TestNewMockRows(t *testing.T) {
	rows := NewMockRows([][]any{{"a", 1}, {"b", 2}})
	assert.NotNil(t, rows)
	assert.Equal(t, 0, rows.idx)
	assert.False(t, rows.closed)
}

func TestMockRows_Close(t *testing.T) {
	rows := NewMockRows(nil)
	rows.Close()
	assert.True(t, rows.closed)
}

func TestMockRows_Err(t *testing.T) {
	rows := NewMockRows(nil)
	assert.Nil(t, rows.Err())

	rows.err = errors.New("row error")
	assert.Equal(t, errors.New("row error"), rows.Err())
}

func TestMockRows_CommandTag(t *testing.T) {
	rows := NewMockRows(nil)
	tag := rows.CommandTag()
	assert.Equal(t, pgconn.CommandTag{}, tag)
}

func TestMockRows_FieldDescriptions(t *testing.T) {
	rows := NewMockRows(nil)
	fd := rows.FieldDescriptions()
	assert.Nil(t, fd)
}

func TestMockRows_Next(t *testing.T) {
	rows := NewMockRows([][]any{{"a"}, {"b"}})
	assert.True(t, rows.Next())
	assert.True(t, rows.Next())
	assert.False(t, rows.Next())
}

func TestMockRows_Next_Empty(t *testing.T) {
	rows := NewMockRows(nil)
	assert.False(t, rows.Next())
}

func TestMockRows_Scan(t *testing.T) {
	rows := NewMockRows([][]any{{"hello", 42}})
	require.True(t, rows.Next())
	var s string
	var n int
	err := rows.Scan(&s, &n)
	assert.NoError(t, err)
	assert.Equal(t, "hello", s)
	assert.Equal(t, 42, n)
}

func TestMockRows_Scan_NoRow(t *testing.T) {
	rows := NewMockRows([][]any{})
	err := rows.Scan()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no row to scan")
}

func TestMockRows_Scan_NotEnoughValues(t *testing.T) {
	rows := NewMockRows([][]any{{"only_one"}})
	require.True(t, rows.Next())
	var s1, s2 string
	err := rows.Scan(&s1, &s2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough values")
}

func TestMockRows_Scan_NilValue(t *testing.T) {
	rows := NewMockRows([][]any{{nil, "hello"}})
	require.True(t, rows.Next())
	var s1, s2 string
	err := rows.Scan(&s1, &s2)
	assert.NoError(t, err)
	assert.Equal(t, "", s1)
	assert.Equal(t, "hello", s2)
}

func TestMockRows_Values(t *testing.T) {
	rows := NewMockRows([][]any{{"a", 1}})
	require.True(t, rows.Next())
	vals, err := rows.Values()
	assert.NoError(t, err)
	assert.Equal(t, []any{"a", 1}, vals)
}

func TestMockRows_Values_NoRow(t *testing.T) {
	rows := NewMockRows([][]any{})
	vals, err := rows.Values()
	assert.Error(t, err)
	assert.Nil(t, vals)
}

func TestMockRows_RawValues(t *testing.T) {
	rows := NewMockRows(nil)
	rv := rows.RawValues()
	assert.Nil(t, rv)
}

func TestMockRows_Conn(t *testing.T) {
	rows := NewMockRows(nil)
	conn := rows.Conn()
	assert.Nil(t, conn)
}

func TestMockRows_Scan_AllTypes(t *testing.T) {
	rows := NewMockRows([][]any{{"hello", 42, int64(999), 3.14, true, []byte("bytes")}})
	require.True(t, rows.Next())
	var s string
	var n int
	var n64 int64
	var f float64
	var b bool
	var by []byte
	err := rows.Scan(&s, &n, &n64, &f, &b, &by)
	assert.NoError(t, err)
	assert.Equal(t, "hello", s)
	assert.Equal(t, 42, n)
	assert.Equal(t, int64(999), n64)
	assert.Equal(t, 3.14, f)
	assert.True(t, b)
	assert.Equal(t, []byte("bytes"), by)
}
