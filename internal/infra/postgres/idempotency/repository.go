package idempotency

import (
	"context"
	stderrors "errors"
	"time"

	guuid "github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	domainidem "github.com/guliangiam/guliangiam/internal/domain/idempotency"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

var _ domainidem.Repository = (*Repository)(nil)

// Repository implements domain/idempotency.Repository against PostgreSQL.
// Follows the same pattern as outbox.Repository: shared.DBTX for tx-scoped ops,
// pgxpool.Pool for ad-hoc usage outside a UoW.
type Repository struct {
	db   shared.DBTX
	pool *pgxpool.Pool
}

func NewRepository(db shared.DBTX, pool *pgxpool.Pool) *Repository {
	return &Repository{db: db, pool: pool}
}

// withSystemTx 在系统作用域事务内执行 fn：idempotency_keys 是跨租户平台表，
// 中间件运行在 TenantFilter 之外（手冊 4.16 / ADR-018），没有请求级租户 GUC。
// 直接走 pool 会触发 RLS 拒绝并让幂等静默 fail-open（DEF-31）；因此每次读写都
// 短暂开启事务并钉住 system 角色，事务结束即释放，不污染连接池。
func (r *Repository) withSystemTx(ctx context.Context, fn func(db shared.DBTX) error) error {
	if r.pool == nil {
		return fn(r.db)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`,
	); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) GetByScopeAndKey(ctx context.Context, scope, key string) (*domainidem.Record, error) {
	var out *domainidem.Record
	err := r.withSystemTx(ctx, func(db shared.DBTX) error {
		row := db.QueryRow(ctx, QueryGetByScopeKey, scope, key)
		rec, qerr := scanRecord(row)
		if qerr != nil {
			return qerr
		}
		out = rec
		return nil
	})
	return out, err
}

func (r *Repository) TryAcquire(ctx context.Context, rec *domainidem.Record) (*domainidem.Record, bool, error) {
	if rec.ID == "" {
		id, err := guuid.NewV7()
		if err != nil {
			return nil, false, apperrors.WrapInternalf(err, "idempotency: generate id")
		}
		rec.ID = id.String()
	}
	var out *domainidem.Record
	var acquired bool
	err := r.withSystemTx(ctx, func(db shared.DBTX) error {
		recOut, acq, aerr := r.tryAcquireDB(ctx, db, rec)
		if aerr != nil {
			return aerr
		}
		out, acquired = recOut, acq
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return out, acquired, nil
}

func (r *Repository) tryAcquireDB(ctx context.Context, db shared.DBTX, rec *domainidem.Record) (*domainidem.Record, bool, error) {
	// INSERT ... ON CONFLICT DO NOTHING RETURNING — 原子获取 gate.
	created, err := r.insertAcquireDB(ctx, db, rec)
	if err == nil {
		return created, true, nil
	}
	if !isAcquireConflict(err) {
		return nil, false, apperrors.WrapInternalf(err, "idempotency: try acquire")
	}

	// 冲突：先抢占过期行（BE-E-12）。过期未清理的行会让 GetByScopeAndKey
	// （expires_at > NOW()）读不到而 409 卡死到清理周期；这里按 key 定点删除
	// 过期行后重试一次 INSERT。并发下只有一个请求删成功，其余走读路径。
	tag, derr := db.Exec(ctx, QueryDeleteExpiredByKey, rec.Scope, rec.Key)
	if derr != nil {
		return nil, false, apperrors.WrapInternalf(derr, "idempotency: reclaim expired key")
	}
	if tag.RowsAffected() > 0 {
		created, err = r.insertAcquireDB(ctx, db, rec)
		if err == nil {
			return created, true, nil
		}
		if !isAcquireConflict(err) {
			return nil, false, apperrors.WrapInternalf(err, "idempotency: try acquire after reclaim")
		}
	}

	existing, gerr := r.readExistingDB(ctx, db, rec.Scope, rec.Key)
	if gerr != nil {
		return nil, false, gerr
	}
	if existing == nil {
		return nil, false, apperrors.NewConflict("idempotency: key exists but not readable")
	}

	// failed 终态 + 同哈希 → 翻回 in_progress 重新捕获（BE-E-12）。
	// 不同哈希保持 acquired=false，由中间件按 409 处理。
	if existing.Status == domainidem.StatusFailed && existing.RequestHash == rec.RequestHash {
		row := db.QueryRow(ctx, QueryReacquireFailed,
			rec.Scope, rec.Key, rec.RequestMethod, rec.RequestPath, rec.RequestHash)
		reacquired, rerr := scanRecord(row)
		if rerr == nil {
			return reacquired, true, nil
		}
		if !stderrors.Is(rerr, pgx.ErrNoRows) {
			return nil, false, apperrors.WrapInternalf(rerr, "idempotency: reacquire failed record")
		}
		// 竞态：另一请求已抢占/完成 — 回读当前行交给中间件裁决。
		current, cerr := r.readExistingDB(ctx, db, rec.Scope, rec.Key)
		if cerr != nil {
			return nil, false, cerr
		}
		if current == nil {
			return nil, false, apperrors.NewConflict("idempotency: key exists but not readable")
		}
		return current, false, nil
	}
	return existing, false, nil
}

// insertAcquireDB 执行一次原子 INSERT gate；冲突时返回 pgx.ErrNoRows 或 23505。
func (r *Repository) insertAcquireDB(ctx context.Context, db shared.DBTX, rec *domainidem.Record) (*domainidem.Record, error) {
	orgID := orgIDArg(rec.OrganizationID)
	row := db.QueryRow(ctx, QueryTryAcquire,
		rec.ID, orgID, rec.Scope, rec.Key,
		rec.RequestMethod, rec.RequestPath, rec.RequestHash,
	)
	return scanRecord(row)
}

// isAcquireConflict 报告 INSERT gate 是否因既有行而落空
// （ON CONFLICT DO NOTHING → no-rows，或并发下唯一约束 23505）。
func isAcquireConflict(err error) bool {
	if stderrors.Is(err, pgx.ErrNoRows) {
		return true
	}
	var pgErr *pgconn.PgError
	return stderrors.As(err, &pgErr) && pgErr.Code == "23505"
}

// readExistingDB 在调用方已开启的事务内读取既有记录；不存在返回 (nil, nil)。
func (r *Repository) readExistingDB(ctx context.Context, db shared.DBTX, scope, key string) (*domainidem.Record, error) {
	row := db.QueryRow(ctx, QueryGetByScopeKey, scope, key)
	existing, gerr := scanRecord(row)
	if gerr != nil {
		if stderrors.Is(gerr, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, gerr
	}
	return existing, nil
}

func (r *Repository) Complete(ctx context.Context, id string, status domainidem.Status, respStatus int, respBody, respContentType string) error {
	return r.withSystemTx(ctx, func(db shared.DBTX) error {
		// 空 Content-Type 存 NULL：旧行为等价，回放时不设置显式头（BE-E-12）。
		var contentType any
		if respContentType != "" {
			contentType = respContentType
		}
		tag, err := db.Exec(ctx, QueryComplete,
			string(status), respStatus, respBody, contentType, id,
		)
		if err != nil {
			return apperrors.WrapInternalf(err, "idempotency: complete")
		}
		if tag.RowsAffected() == 0 {
			return apperrors.NewConflict("idempotency: record not found or not in_progress")
		}
		return nil
	})
}

func orgIDArg(s *string) any {
	if s == nil || *s == "" {
		return nil
	}
	return *s
}

// scanRecord 从 pgx.Row 扫描 Record。
//
// 可空列（organization_id / response_body / content_type / completed_at）必须用指针接收：
// INSERT 刚创建的 in_progress 行这些列都是 NULL，直接 scan 到 string 会返回
// "cannot scan NULL into *string"，使 TryAcquire 永远失败并 fail-open（DEF-31 的第二层根因）。
func scanRecord(row interface{ Scan(dest ...any) error }) (*domainidem.Record, error) {
	var rec domainidem.Record
	var orgID, respBody, contentType *string
	var respStatus *int
	var completedAt, expiresAt, createdAt *time.Time

	err := row.Scan(
		&rec.ID, &orgID, &rec.Scope, &rec.Key,
		&rec.RequestMethod, &rec.RequestPath, &rec.RequestHash,
		&rec.Status, &respStatus, &respBody, &contentType,
		&completedAt, &expiresAt, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	if orgID != nil && *orgID != "" {
		rec.OrganizationID = orgID
	}
	if respStatus != nil {
		s := *respStatus
		rec.ResponseStatus = &s
	}
	if respBody != nil && *respBody != "" {
		rec.ResponseBody = respBody
	}
	if contentType != nil && *contentType != "" {
		rec.ContentType = contentType
	}
	if completedAt != nil {
		rec.CompletedAt = completedAt
	}
	if expiresAt != nil {
		rec.ExpiresAt = *expiresAt
	}
	if createdAt != nil {
		rec.CreatedAt = *createdAt
	}
	return &rec, nil
}

// DeleteExpired removes one bounded batch of expired idempotency keys.
// Implements domainidem.ExpiredCleaner.
func (r *Repository) DeleteExpired(ctx context.Context, cutoff time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}
	tag, err := r.db.Exec(ctx, QueryDeleteExpired, cutoff, limit)
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "idempotency: delete expired")
	}
	return tag.RowsAffected(), nil
}

var _ domainidem.ExpiredCleaner = (*Repository)(nil)
