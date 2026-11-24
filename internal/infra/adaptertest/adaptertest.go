// Package adaptertest probes external database connectivity for the adapters
// module. DSN construction, driver registration and outbound-address checks
// are infrastructure concerns; the app layer reaches this package through its
// ConnectionTester port (composition-root wiring).
package adaptertest

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/guliangiam/guliangiam/internal/platform/ssrf"

	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib" // register the PostgreSQL driver
)

// supportedDatabaseTypes is the closed set advertised in failure messages.
// Aliases (postgres/pg/sqlserver) remain accepted inputs. mssql is only
// usable when a driver has been registered by the deployment; the binary does
// not bundle one (no new dependency is added for it).
var supportedDatabaseTypes = []string{"postgresql", "mysql", "mssql"}

// Tester implements the app/adapters.ConnectionTester port.
type Tester struct{}

// New returns a connection tester.
func New() Tester { return Tester{} }

// TestDatabaseConnection attempts a short-lived connection to the target
// database. It returns (connected, failureReason); failureReason is empty on
// success. Invalid input and blocked targets are reported as failures rather
// than panics or silent hangs.
func (Tester) TestDatabaseConnection(ctx context.Context, databaseType, host string, port int, username, password, databaseName string) (bool, string) {
	if strings.TrimSpace(host) == "" {
		return false, "host is required"
	}

	dsn, driver, err := buildDSN(databaseType, host, port, username, password, databaseName)
	if err != nil {
		return false, err.Error()
	}

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// SSRF guard: the probe must not be usable to reach loopback, link-local
	// (cloud metadata) or other non-public addresses through the server.
	if err := validateOutboundHost(testCtx, host); err != nil {
		return false, err.Error()
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return false, fmt.Sprintf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(testCtx); err != nil {
		return false, fmt.Sprintf("connection failed: %v", err)
	}
	return true, ""
}

// validateOutboundHost resolves the host and fails closed when any resolved
// address is non-public (ssrf.IsSafeIP) or resolution itself fails.
func validateOutboundHost(ctx context.Context, host string) error {
	var ips []net.IPAddr
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IPAddr{{IP: ip}}
	} else {
		resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return apperrors.WrapInvalidArgumentf(err, "resolve host %q", host)
		}
		ips = resolved
	}
	if len(ips) == 0 {
		return apperrors.NewInvalidArgumentf("host %q resolved to no addresses", host)
	}
	for _, a := range ips {
		if !ssrf.IsSafeIP(a.IP) {
			return apperrors.NewForbiddenf("blocked target %s (%s): non-public address", host, a.IP)
		}
	}
	return nil
}

// buildDSN constructs a data source name with properly escaped credentials.
// Raw fmt.Sprintf interpolation broke passwords containing @ : / ? # and let
// caller input reach the sslmode segment; both are fixed here.
func buildDSN(databaseType, host string, port int, username, password, databaseName string) (dsn, driver string, err error) {
	switch databaseType {
	case "postgresql", "postgres", "pg", "":
		if port == 0 {
			port = 5432
		}
		if err := validatePort(port); err != nil {
			return "", "", err
		}
		u := &url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(username, password),
			Host:   net.JoinHostPort(host, strconv.Itoa(port)),
			Path:   databaseName,
			// sslmode is a fixed policy constant, never caller-interpolated.
			RawQuery: "sslmode=disable",
		}
		return u.String(), "pgx", nil
	case "mysql":
		if port == 0 {
			port = 3306
		}
		if err := validatePort(port); err != nil {
			return "", "", err
		}
		cfg := mysqldriver.NewConfig()
		cfg.User = username
		cfg.Passwd = password
		cfg.Net = "tcp"
		cfg.Addr = net.JoinHostPort(host, strconv.Itoa(port))
		cfg.DBName = databaseName
		cfg.Timeout = 10 * time.Second
		return cfg.FormatDSN(), "mysql", nil
	case "mssql", "sqlserver":
		if !driverRegistered("sqlserver", "mssql") {
			return "", "", apperrors.NewUnimplemented(
				"unsupported driver: mssql/sqlserver driver is not registered (supported: postgresql, mysql)")
		}
		if port == 0 {
			port = 1433
		}
		if err := validatePort(port); err != nil {
			return "", "", err
		}
		query := url.Values{}
		query.Set("database", databaseName)
		query.Set("connection timeout", "10")
		u := &url.URL{
			Scheme:   "sqlserver",
			User:     url.UserPassword(username, password),
			Host:     net.JoinHostPort(host, strconv.Itoa(port)),
			RawQuery: query.Encode(),
		}
		return u.String(), "sqlserver", nil
	default:
		return "", "", apperrors.NewWithMeta(
			apperrors.CodeUnprocessableEntity,
			fmt.Sprintf("unsupported database type: %s (supported: postgresql, mysql, mssql)", databaseType),
			map[string]any{"field": "databaseType", "value": databaseType, "allowed": append([]string{}, supportedDatabaseTypes...)},
		)
	}
}

func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return apperrors.NewInvalidArgument("port must be between 1 and 65535")
	}
	return nil
}

func driverRegistered(names ...string) bool {
	registered := sql.Drivers()
	for _, name := range names {
		for _, r := range registered {
			if r == name {
				return true
			}
		}
	}
	return false
}
