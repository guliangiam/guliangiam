package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/guliangiam/guliangiam/internal/platform/config"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"

	mysql "github.com/go-sql-driver/mysql" // 注册 "mysql" driver，并提供 DSN 解析
)

// MySQLConfig 是 MySQL 连接配置，与 PostgresConfig 形状一致但字段匹配 sql.DB 能力。
type MySQLConfig struct {
	DataSourceName string
	MaxOpenConns   int
	MaxIdleConns   int
	MaxLifetime    time.Duration
}

// NewMySQLPool 打开并 ping 一个 *sql.DB（go-sql-driver/mysql）。
func NewMySQLPool(ctx context.Context, cfg MySQLConfig) (*sql.DB, error) {
	if cfg.DataSourceName == "" {
		return nil, apperrors.NewUnavailable("mysql DSN not configured")
	}

	normalized, err := NormalizeMySQLDSN(cfg.DataSourceName)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "normalize mysql DSN")
	}

	db, err := sql.Open("mysql", normalized)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "open mysql")
	}

	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.MaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.MaxLifetime)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, apperrors.WrapInternalf(err, "ping mysql")
	}
	return db, nil
}

// MySQLConfigFromEnv 从环境变量读取 MySQL 连接配置。
// DB_DSN（统一连接串）优先，其次 MYSQL_DSN 或 MYSQL_URL。连接池参数从 config 包
// 的统一 getter 读取（已内置方言别名 fallback），无则用 defaultMySQLMaxConns() 默认值。
func MySQLConfigFromEnv() (MySQLConfig, error) {
	var dataSourceName string
	if dsn := config.DBDSN(); dsn != "" {
		dataSourceName = dsn
	} else if dsn := config.MySQLDSN(); dsn != "" {
		dataSourceName = dsn
	} else if dsn := config.MySQLURL(); dsn != "" {
		dataSourceName = dsn
	}
	if dataSourceName == "" {
		return MySQLConfig{}, apperrors.NewUnavailable("mysql DSN not configured: set DB_DSN, MYSQL_DSN or MYSQL_URL")
	}

	maxOpen := config.DBPoolMaxOpen()
	if maxOpen <= 0 {
		maxOpen = defaultMySQLMaxConns()
	}
	maxIdle := config.DBPoolMaxIdle()
	if maxIdle <= 0 {
		maxIdle = maxOpen / 4
		if maxIdle < 2 {
			maxIdle = 2
		}
	}

	return MySQLConfig{
		DataSourceName: dataSourceName,
		MaxOpenConns:   maxOpen,
		MaxIdleConns:   maxIdle,
		MaxLifetime:    config.DBConnMaxLifetime(),
	}, nil
}

func defaultMySQLMaxConns() int {
	n := runtime.NumCPU()*2 + 1
	if n < 10 {
		n = 10
	}
	if n > 50 {
		n = 50
	}
	return n
}

// NormalizeMySQLDSN 把连接串统一成 go-sql-driver DSN，并强制 parseTime=true。
//
// 两个运行期故障由此消除：
//   - 仓储把 TIMESTAMP 列 Scan 到 time.Time；缺 parseTime 时驱动返回 []uint8，
//     报 "unsupported Scan, storing driver.Value type []uint8 into type *time.Time"。
//   - MYSQL_URL 文档写明是 URL 格式，但 sql.Open("mysql") 只接受 DSN 格式，
//     直接透传会让 URL 形式永远连不上。
func NormalizeMySQLDSN(raw string) (string, error) {
	dsn := strings.TrimSpace(raw)
	if dsn == "" {
		return "", errors.New("mysql DSN is empty")
	}

	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", err
		}
		if u.Scheme != "mysql" {
			return "", fmt.Errorf("unsupported mysql URL scheme %q", u.Scheme)
		}

		cfg := mysql.NewConfig()
		if u.User != nil {
			cfg.User = u.User.Username()
			cfg.Passwd, _ = u.User.Password()
		}
		cfg.Net = "tcp"
		cfg.Addr = u.Host
		if cfg.Addr == "" {
			cfg.Addr = "127.0.0.1:3306"
		}
		if !strings.Contains(cfg.Addr, ":") {
			cfg.Addr += ":3306"
		}
		cfg.DBName = strings.TrimPrefix(u.Path, "/")
		if cfg.Params == nil {
			cfg.Params = make(map[string]string)
		}
		query, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return "", err
		}
		for key, values := range query {
			if len(values) > 0 {
				cfg.Params[key] = values[len(values)-1]
			}
		}
		cfg.ParseTime = true
		cfg.Params["parseTime"] = "true"
		if cfg.Loc == nil {
			cfg.Loc = time.UTC
		}
		return cfg.FormatDSN(), nil
	}

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}
	cfg.ParseTime = true
	if cfg.Loc == nil {
		cfg.Loc = time.UTC
	}
	return cfg.FormatDSN(), nil
}
