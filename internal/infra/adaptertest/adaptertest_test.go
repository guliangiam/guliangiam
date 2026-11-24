package adaptertest

import (
	"context"
	"net/url"
	"strings"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDSN_PostgreSQL(t *testing.T) {
	dsn, driver, err := buildDSN("postgresql", "db.example.com", 5433, "admin", "secret", "testdb")
	require.NoError(t, err)
	assert.Equal(t, "pgx", driver)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	assert.Equal(t, "postgres", u.Scheme)
	assert.Equal(t, "db.example.com:5433", u.Host)
	assert.Equal(t, "/testdb", u.Path)
	assert.Equal(t, "sslmode=disable", u.RawQuery)
	password, _ := u.User.Password()
	assert.Equal(t, "secret", password)
}

// Special characters in credentials must survive the DSN round trip; the old
// fmt.Sprintf interpolation produced unparseable or mis-parsed DSNs.
func TestBuildDSN_PostgreSQL_EscapesCredentials(t *testing.T) {
	password := "p@ss:w/ord?#%&=+"
	dsn, _, err := buildDSN("postgres", "db.example.com", 5432, "user:name@corp", password, "my db")
	require.NoError(t, err)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	gotPassword, ok := u.User.Password()
	assert.True(t, ok)
	assert.Equal(t, password, gotPassword)
	assert.Equal(t, "user:name@corp", u.User.Username())
	assert.Equal(t, "sslmode=disable", u.RawQuery)
}

func TestBuildDSN_PostgreSQL_DefaultPort(t *testing.T) {
	dsn, _, err := buildDSN("", "db.example.com", 0, "u", "p", "db")
	require.NoError(t, err)
	assert.Contains(t, dsn, "db.example.com:5432")
}

func TestBuildDSN_MySQL_RoundTrip(t *testing.T) {
	password := "p@ss/w:ord?#"
	dsn, driver, err := buildDSN("mysql", "db.example.com", 3307, "root", password, "mydb")
	require.NoError(t, err)
	assert.Equal(t, "mysql", driver)
	cfg, err := mysqldriver.ParseDSN(dsn)
	require.NoError(t, err)
	assert.Equal(t, "root", cfg.User)
	assert.Equal(t, password, cfg.Passwd)
	assert.Equal(t, "tcp", cfg.Net)
	assert.Equal(t, "db.example.com:3307", cfg.Addr)
	assert.Equal(t, "mydb", cfg.DBName)
}

func TestBuildDSN_MySQL_DefaultPort(t *testing.T) {
	dsn, _, err := buildDSN("mysql", "db.example.com", 0, "root", "pass", "mydb")
	require.NoError(t, err)
	cfg, err := mysqldriver.ParseDSN(dsn)
	require.NoError(t, err)
	assert.Equal(t, "tcp", cfg.Net)
	assert.Equal(t, "db.example.com:3306", cfg.Addr)
}

// The binary bundles no mssql driver: the probe must say so explicitly instead
// of failing with a generic "unknown driver" at ping time.
func TestBuildDSN_MSSQL_UnregisteredDriver(t *testing.T) {
	_, _, err := buildDSN("mssql", "db.example.com", 1433, "sa", "pass", "mydb")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported driver")
	assert.Equal(t, apperrors.CodeUnimplemented, apperrors.FromError(err).Code)
}

func TestBuildDSN_UnsupportedType(t *testing.T) {
	_, _, err := buildDSN("oracle", "db.example.com", 1521, "u", "p", "db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database type")
	appErr := apperrors.FromError(err)
	assert.Equal(t, apperrors.CodeUnprocessableEntity, appErr.Code)
	assert.Equal(t, "databaseType", appErr.Meta["field"])
	assert.NotNil(t, appErr.Meta["allowed"])
}

func TestBuildDSN_PortRange(t *testing.T) {
	for _, port := range []int{-1, 65536} {
		_, _, err := buildDSN("postgresql", "db.example.com", port, "u", "p", "db")
		require.Error(t, err, "port %d", port)
		assert.Equal(t, apperrors.CodeInvalidArgument, apperrors.FromError(err).Code)
	}
}

// SSRF guard: loopback, link-local (cloud metadata) and private targets are
// rejected before any dial. IP literals keep this test offline.
func TestTestDatabaseConnection_BlocksNonPublicTargets(t *testing.T) {
	tester := New()
	for _, host := range []string{"127.0.0.1", "169.254.169.254", "10.0.0.1", "192.168.1.10", "::1"} {
		connected, failure := tester.TestDatabaseConnection(context.Background(), "postgresql", host, 5432, "u", "p", "db")
		assert.False(t, connected, "host %s", host)
		assert.Contains(t, failure, "blocked target", "host %s", host)
	}
}

func TestTestDatabaseConnection_HostRequired(t *testing.T) {
	connected, failure := New().TestDatabaseConnection(context.Background(), "postgresql", "  ", 5432, "u", "p", "db")
	assert.False(t, connected)
	assert.Equal(t, "host is required", failure)
}

func TestTestDatabaseConnection_UnsupportedTypeIsResult(t *testing.T) {
	connected, failure := New().TestDatabaseConnection(context.Background(), "oracle", "db.example.com", 1521, "u", "p", "db")
	assert.False(t, connected)
	assert.True(t, strings.Contains(failure, "unsupported database type"))
}
