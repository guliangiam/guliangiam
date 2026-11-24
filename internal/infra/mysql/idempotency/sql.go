package idempotency

import (
	"github.com/guliangiam/guliangiam/internal/infra/dialect"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
)

// MySQL 8.4+ SQL for idempotency_keys.
// Mirrors internal/infra/postgres/idempotency/sql.go.

const (
	QueryGetByScopeKey = `
			SELECT id, organization_id, scope, idempotency_key,
			       request_method, request_path, request_hash,
			       status, response_status, response_body, content_type,
			       completed_at, expires_at, created_at
			FROM guliangiam.idempotency_keys
			WHERE scope = ? AND idempotency_key = ?
			LIMIT 1`

	QueryComplete = `
			UPDATE guliangiam.idempotency_keys
			SET status = ?, response_status = ?, response_body = CAST(? AS JSON),
			    content_type = ?,
			    completed_at = NOW(), updated_at = NOW()
			WHERE id = ? AND status = 'in_progress'`
)

// QueryTryAcquire — 方言敏感：UPSERT DoNothing 经 mysqlshared.Dialect.Upsert 生成
// MySQL 无 RETURNING，调用方需在 Execute 后 SELECT 回读
var QueryTryAcquire = `
			INSERT INTO guliangiam.idempotency_keys
					(id, organization_id, scope, idempotency_key,
					 request_method, request_path, request_hash,
					 status, expires_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 'in_progress', DATE_ADD(NOW(), INTERVAL 24 HOUR))
` + mysqlshared.Dialect.Upsert(dialect.DialectUpsert{DoNothing: true}, nil) + `
`

// QueryDeleteExpired removes expired idempotency rows in bounded batches.

const QueryDeleteExpired = `
		DELETE FROM idempotency_keys
		WHERE expires_at < ?
		ORDER BY expires_at
		LIMIT ?
	`
