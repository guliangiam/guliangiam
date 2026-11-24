package idempotency

// SQL constants for the idempotency_keys table (ADR-018).
// Schema defined in db/schema/10_tenant.yaml.
const (
	QueryGetByScopeKey = `
		SELECT id, organization_id, scope, idempotency_key, request_method, request_path,
		       request_hash, status, response_status, response_body, content_type,
		       completed_at, expires_at, created_at
		FROM "guliangiam".idempotency_keys
		WHERE scope = $1 AND idempotency_key = $2 AND expires_at > NOW()
		LIMIT 1
	`

	// QueryTryAcquire uses INSERT ... ON CONFLICT DO NOTHING + RETURNING
	// to atomically grab the write gate. If a row already exists, we
	// fall back to GetByScopeAndKey to read it.
	QueryTryAcquire = `
		INSERT INTO "guliangiam".idempotency_keys
			(id, organization_id, scope, idempotency_key, request_method, request_path,
			 request_hash, status, expires_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, 'in_progress', NOW() + INTERVAL '24 hours')
		ON CONFLICT (scope, idempotency_key) DO NOTHING
		RETURNING id, organization_id, scope, idempotency_key, request_method, request_path,
		          request_hash, status, response_status, response_body, content_type,
		          completed_at, expires_at, created_at
	`

	QueryComplete = `
		UPDATE "guliangiam".idempotency_keys
		SET status = $1,
		    response_status = $2,
		    response_body = $3,
		    content_type = $4,
		    completed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $5::uuid AND status = 'in_progress'
	`

	// QueryDeleteExpiredByKey reclaims a single expired (scope, key) row so a
	// new request can preempt it immediately instead of 409-ing until the
	// background TTL cleaner runs (BE-E-12). DML only — no schema change.
	QueryDeleteExpiredByKey = `
		DELETE FROM "guliangiam".idempotency_keys
		WHERE scope = $1 AND idempotency_key = $2 AND expires_at <= NOW()
	`

	// QueryReacquireFailed flips a failed terminal row back to in_progress for
	// a same-hash retry so the retry's outcome is captured (BE-E-12). The
	// response columns are cleared; a concurrent winner simply yields no row.
	QueryReacquireFailed = `
		UPDATE "guliangiam".idempotency_keys
		SET status = 'in_progress',
		    request_method = $3,
		    request_path = $4,
		    request_hash = $5,
		    response_status = NULL,
		    response_body = NULL,
		    content_type = NULL,
		    completed_at = NULL,
		    expires_at = NOW() + INTERVAL '24 hours',
		    updated_at = NOW()
		WHERE scope = $1 AND idempotency_key = $2 AND status = 'failed' AND expires_at > NOW()
		RETURNING id, organization_id, scope, idempotency_key, request_method, request_path,
		          request_hash, status, response_status, response_body, content_type,
		          completed_at, expires_at, created_at
	`
)

// QueryDeleteExpired deletes expired idempotency rows in bounded batches so
// a large backlog cannot hold one long transaction (ADR-018 TTL cleanup).

const QueryDeleteExpired = `
		DELETE FROM "guliangiam".idempotency_keys
		WHERE id IN (
			SELECT id FROM "guliangiam".idempotency_keys
			WHERE expires_at < $1
			ORDER BY expires_at
			LIMIT $2
		)
	`
