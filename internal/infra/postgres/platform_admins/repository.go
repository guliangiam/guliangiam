package platformadmins

import (
	"context"

	domain "github.com/guliangiam/guliangiam/internal/domain/platform_admins"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository 实现 PlatformAdminRepository（平台管理员 CRUD）。
// platform_admins RLS policy: `platform_admins_super_admin: is_super_admin()`
// → 只有 super_admin role 才能读写。
//
// ⚠️ pgxpool GUC 陷阱：
//
//	pgxpool.Exec(ctx, sql)        → Acquire conn A → exec → Release A
//	pgxpool.QueryRow(ctx, sql)    → Acquire conn B → query  ← conn B 没有 set_config 副作用！
//	set_config('app.current_role','super_admin', false) 是 session 级，
//	只在执行它的那个连接上生效。pgxpool 每次 Exec/QueryRow 可能拿到不同连接，
//	所以必须在**同一个连接**上先 Exec set_config，再 Exec/QueryRow 主 SQL。
//
//	两种 shared.DBTX 实现：
//	  - pgxpool.Pool   → 必须 Acquire 后在同一 *pgxpool.Conn 上串
//	  - pgx.Tx          → 事务内所有 SQL 天然在同一连接上，Exec set_config 后直接用 tx 即可
//
//	runOnConn 自动处理两种情况。
type Repository struct {
	db shared.DBTX
}

type ScopeRepository struct {
	db shared.DBTX
}

func NewRepository(db shared.DBTX) Repository {
	return Repository{db: db}
}

func NewScopeRepository(db shared.DBTX) ScopeRepository {
	return ScopeRepository{db: db}
}

// runOnConn 在同一连接上先 grant super_admin role，再执行 fn。
// db 是 pgxpool.Pool → Acquire 独占连接，fn 内用 *pgxpool.Conn（也是 shared.DBTX）
// db 是 pgx.Tx      → 直接用 tx，事务内 Exec 后同一 tx 继续用
//
// ⚠️ instrumentedDBTX 解包：
// NewInstrumentedDBTX 把 pgxpool.Pool 包成 instrumentedDBTX（OTel + Prometheus）。
// runOnConn 需要先解包到底层 *pgxpool.Pool / pgx.Tx 才能 type switch。
func runOnConn(ctx context.Context, db shared.DBTX, fn func(ctx context.Context, conn shared.DBTX) error) error {
	// 递归解包所有 DBTX wrapper（instrumentedDBTX 等）
	for {
		if u, ok := db.(interface{ Unwrap() shared.DBTX }); ok {
			db = u.Unwrap()
			continue
		}
		break
	}

	switch t := db.(type) {
	case *pgxpool.Pool:
		conn, err := t.Acquire(ctx)
		if err != nil {
			return err
		}
		defer conn.Release()
		if _, err := conn.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', false)`); err != nil {
			return err
		}
		return fn(ctx, conn)
	case pgx.Tx:
		if _, err := t.Exec(ctx, `SELECT set_config('app.current_role', 'super_admin', true)`); err != nil {
			return err
		}
		return fn(ctx, t)
	default:
		return apperrors.NewInternalf("unsupported DBTX type %T", db)
	}
}

// ─── PlatformAdminRepository ──────────────────────────────────────────────

var _ domain.PlatformAdminRepository = Repository{}

func (r Repository) GetByID(ctx context.Context, id string) (*domain.PlatformAdmin, error) {
	var admin domain.PlatformAdmin
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		return conn.QueryRow(ctx, queryGetByID, id).Scan(
			&admin.ID, &admin.UserID, &admin.CreatedBy, &admin.IsActive,
			&admin.CreatedAt, &admin.UpdatedAt, &admin.Revision,
		)
	})
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("platform admin not found")
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r Repository) GetByUserID(ctx context.Context, userID string) (*domain.PlatformAdmin, error) {
	var admin domain.PlatformAdmin
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		return conn.QueryRow(ctx, queryGetByUserID, userID).Scan(
			&admin.ID, &admin.UserID, &admin.CreatedBy, &admin.IsActive,
			&admin.CreatedAt, &admin.UpdatedAt, &admin.Revision,
		)
	})
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("platform admin not found")
	}
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r Repository) List(ctx context.Context, opts domain.ListOptions) ([]*domain.PlatformAdmin, int64, error) {
	items := make([]*domain.PlatformAdmin, 0, shared.ClampPageSize(opts.PageSize))
	var total int64
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		pageSize := shared.ClampPageSize(opts.PageSize)
		offset := shared.PageOffset(opts.Page, pageSize)
		var activeArg any
		if opts.Active != nil {
			activeArg = *opts.Active
		}
		var keyword any = ""
		if opts.Keyword != "" {
			keyword = opts.Keyword
		}
		rows, err := conn.Query(ctx, queryList, activeArg, keyword, pageSize, offset)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var a domain.PlatformAdmin
			if err := rows.Scan(
				&a.ID, &a.UserID, &a.CreatedBy, &a.IsActive,
				&a.CreatedAt, &a.UpdatedAt, &a.Revision,
			); err != nil {
				return err
			}
			items = append(items, &a)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return conn.QueryRow(ctx, queryCount, activeArg, keyword).Scan(&total)
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r Repository) Create(ctx context.Context, admin *domain.PlatformAdmin) error {
	return runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		_, err := conn.Exec(ctx, queryInsert,
			admin.ID, admin.UserID, admin.CreatedBy, admin.IsActive,
			admin.CreatedAt, admin.UpdatedAt, admin.Revision,
		)
		return err
	})
}

func (r Repository) Update(ctx context.Context, admin *domain.PlatformAdmin) error {
	var newRevision int64
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		return conn.QueryRow(ctx, queryUpdate,
			admin.CreatedBy, admin.IsActive, admin.UpdatedAt,
			admin.ID, admin.Revision,
		).Scan(&newRevision)
	})
	if err == pgx.ErrNoRows {
		return apperrors.NewConflict("platform admin revision conflict")
	}
	if err != nil {
		return err
	}
	admin.Revision = newRevision
	return nil
}

func (r Repository) SetActive(ctx context.Context, id string, active bool) error {
	var ra int64
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		res, err := conn.Exec(ctx, querySetActive, active, id)
		if err != nil {
			return err
		}
		ra = res.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if ra == 0 {
		return apperrors.NewNotFound("platform admin not found")
	}
	return nil
}

func (r Repository) Delete(ctx context.Context, id string) error {
	var ra int64
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		res, err := conn.Exec(ctx, queryDelete, id)
		if err != nil {
			return err
		}
		ra = res.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if ra == 0 {
		return apperrors.NewNotFound("platform admin not found")
	}
	return nil
}

// ─── PlatformAdminScopeRepository ──────────────────────────────────────────

var _ domain.PlatformAdminScopeRepository = ScopeRepository{}

func (r ScopeRepository) GetByID(ctx context.Context, id string) (*domain.PlatformAdminScope, error) {
	var scope domain.PlatformAdminScope
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		return conn.QueryRow(ctx, queryGetScopeByID, id).Scan(
			&scope.ID, &scope.PlatformAdminID, &scope.ScopeType,
			&scope.TargetTenants, &scope.ApprovedBy, &scope.ApprovedAt,
			&scope.IsBootstrap, &scope.ExpiresAt, &scope.CreatedAt,
		)
	})
	if err == pgx.ErrNoRows {
		return nil, apperrors.NewNotFound("platform admin scope not found")
	}
	if err != nil {
		return nil, err
	}
	return &scope, nil
}

func (r ScopeRepository) ListByPlatformAdmin(ctx context.Context, adminID string, opts domain.ScopeListOptions) ([]*domain.PlatformAdminScope, error) {
	items := make([]*domain.PlatformAdminScope, 0)
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		rows, err := conn.Query(ctx, queryListScopesByAdmin, adminID, opts.OnlyActive)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var s domain.PlatformAdminScope
			if err := rows.Scan(
				&s.ID, &s.PlatformAdminID, &s.ScopeType,
				&s.TargetTenants, &s.ApprovedBy, &s.ApprovedAt,
				&s.IsBootstrap, &s.ExpiresAt, &s.CreatedAt,
			); err != nil {
				return err
			}
			items = append(items, &s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r ScopeRepository) Create(ctx context.Context, scope *domain.PlatformAdminScope) error {
	return runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		_, err := conn.Exec(ctx, queryInsertScope,
			scope.ID, scope.PlatformAdminID, scope.ScopeType,
			scope.TargetTenants, scope.ApprovedBy, scope.ApprovedAt,
			scope.IsBootstrap, scope.ExpiresAt, scope.CreatedAt,
		)
		return err
	})
}

func (r ScopeRepository) Approve(ctx context.Context, scopeID, approverID string) error {
	var ra int64
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		res, err := conn.Exec(ctx, queryApproveScope, approverID, scopeID)
		if err != nil {
			return err
		}
		ra = res.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if ra == 0 {
		return apperrors.NewForbidden("scope already approved or not found")
	}
	return nil
}

func (r ScopeRepository) Delete(ctx context.Context, id string) error {
	var ra int64
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		res, err := conn.Exec(ctx, queryDeleteScope, id)
		if err != nil {
			return err
		}
		ra = res.RowsAffected()
		return nil
	})
	if err != nil {
		return err
	}
	if ra == 0 {
		return apperrors.NewNotFound("platform admin scope not found")
	}
	return nil
}

// HasActiveScopeForTenant 查询指定 admin 是否有覆盖目标租户的有效 scope。
// ScopeAuthorizer Gate 7 hot path 专用。
func (r ScopeRepository) HasActiveScopeForTenant(ctx context.Context, adminID, tenantID string) (bool, error) {
	var has bool
	err := runOnConn(ctx, r.db, func(ctx context.Context, conn shared.DBTX) error {
		return conn.QueryRow(ctx, queryHasActiveScopeForTenant, adminID, tenantID).Scan(&has)
	})
	return has, err
}
