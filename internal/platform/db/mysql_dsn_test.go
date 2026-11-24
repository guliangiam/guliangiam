package db

import (
	"testing"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeMySQLDSNForcesParseTime(t *testing.T) {
	normalized, err := NormalizeMySQLDSN("root:root@tcp(127.0.0.1:3306)/guliangiam")
	require.NoError(t, err)

	cfg, err := mysql.ParseDSN(normalized)
	require.NoError(t, err)
	assert.True(t, cfg.ParseTime, "TIMESTAMP columns are scanned into time.Time")
	assert.Equal(t, "guliangiam", cfg.DBName)
	assert.Equal(t, "root", cfg.User)
}

func TestNormalizeMySQLDSNAcceptsURLForm(t *testing.T) {
	normalized, err := NormalizeMySQLDSN("mysql://root:root@127.0.0.1:3306/guliangiam?charset=utf8mb4")
	require.NoError(t, err)

	cfg, err := mysql.ParseDSN(normalized)
	require.NoError(t, err)
	assert.True(t, cfg.ParseTime)
	assert.Equal(t, "tcp", cfg.Net)
	assert.Equal(t, "127.0.0.1:3306", cfg.Addr)
	assert.Equal(t, "guliangiam", cfg.DBName)
	assert.Equal(t, "root", cfg.User)
	assert.Equal(t, "root", cfg.Passwd)
	assert.Contains(t, normalized, "charset=utf8mb4")
}

func TestNormalizeMySQLDSNDefaultsPort(t *testing.T) {
	normalized, err := NormalizeMySQLDSN("mysql://root:root@localhost/guliangiam")
	require.NoError(t, err)

	cfg, err := mysql.ParseDSN(normalized)
	require.NoError(t, err)
	assert.Equal(t, "localhost:3306", cfg.Addr)
}

func TestNormalizeMySQLDSNRejectsUnknownSchemeAndEmpty(t *testing.T) {
	_, err := NormalizeMySQLDSN("postgres://user:pass@localhost:5432/db")
	require.Error(t, err)

	_, err = NormalizeMySQLDSN("   ")
	require.Error(t, err)
}
