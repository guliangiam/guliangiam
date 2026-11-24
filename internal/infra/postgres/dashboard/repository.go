package dashboard

import (
	"context"

	"github.com/jackc/pgx/v5"

	domaindashboard "github.com/guliangiam/guliangiam/internal/domain/dashboard"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// Beginner 是 dashboard 只读事务所需的最小连接池能力（*pgxpool.Pool 天然满足）。
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

type Repository struct {
	pool Beginner
}

func NewRepository(pool Beginner) Repository {
	return Repository{pool: pool}
}

// setDashboardRLSContext 在事务内（LOCAL）设置 RLS GUC。
//
// 判定与 transport 层 trustedOrgID 的可信裁决严格对齐：
//   - organizationID 为 UUID → member + 该组织：RLS 只放行本租户行，
//     SQL 中显式 organization_id 过滤保留作纵深防御。
//   - organizationID 为空或非 UUID（slug）→ super_admin：仅
//     super_admin/system scope 的请求可能到达（handler 已裁决），
//     对应跨租户总览或 slug 收窄视图；跨租户语义由 SQL 的 $1 过滤实现。
//
// 必须用 set_config(..., true)（事务级）——SESSION 级会污染 pool 连接，
// 后续请求继承错误 GUC（见 platform/db/rls.go 的 AfterConnect 教训）。
func setDashboardRLSContext(ctx context.Context, tx pgx.Tx, organizationID string) error {
	if organizationID == "" || len(organizationID) != 36 {
		if _, err := tx.Exec(ctx,
			`SELECT set_config('app.current_role', 'super_admin', true), set_config('app.current_organization_id', '', true)`,
		); err != nil {
			return apperrors.WrapInternal(err, "dashboard: set super_admin rls context")
		}
		return nil
	}
	if _, err := tx.Exec(ctx,
		`SELECT guliangiam.set_tenant_context($1::uuid, 'member')`,
		organizationID,
	); err != nil {
		return apperrors.WrapInternal(err, "dashboard: set tenant rls context")
	}
	return nil
}

// withRLSTx 在带租户 RLS 上下文的事务里执行只读查询。
// dashboard 查询的 records/sessions/users/applications 等表全部 FORCE ROW
// LEVEL SECURITY，pool 裸读会被 fail-closed 策略遮蔽为空——必须显式开事务
// 并设置 GUC（参照 infra/postgres/scim reconciliation 适配器的自管模式）。
func (r Repository) withRLSTx(ctx context.Context, organizationID string, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return apperrors.WrapInternal(err, "dashboard: begin read transaction")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := setDashboardRLSContext(ctx, tx, organizationID); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return apperrors.WrapInternal(err, "dashboard: commit read transaction")
	}
	return nil
}

func (r Repository) CountUsers(ctx context.Context, organizationID string) (*domaindashboard.UserCountResult, error) {
	var result domaindashboard.UserCountResult
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountUsers, organizationID).Scan(
			&result.Total, &result.Active, &result.Disabled, &result.NewToday,
			&result.New7Days, &result.New30Days,
		)
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "count users")
	}
	return &result, nil
}

func (r Repository) CountOrganizations(ctx context.Context) (int64, error) {
	var total int64
	err := r.withRLSTx(ctx, "", func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountOrganizations).Scan(&total)
	})
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "count organizations")
	}
	return total, nil
}

func (r Repository) CountApplications(ctx context.Context, organizationID string) (int64, error) {
	var total int64
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountApplications, organizationID).Scan(&total)
	})
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "count applications")
	}
	return total, nil
}

func (r Repository) CountRoles(ctx context.Context, organizationID string) (int64, error) {
	var total int64
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountRoles, organizationID).Scan(&total)
	})
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "count roles")
	}
	return total, nil
}

func (r Repository) CountGroups(ctx context.Context, organizationID string) (int64, error) {
	var total int64
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountGroups, organizationID).Scan(&total)
	})
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "count groups")
	}
	return total, nil
}

func (r Repository) CountPermissions(ctx context.Context, organizationID string) (int64, error) {
	var total int64
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountPermissions, organizationID).Scan(&total)
	})
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "count permissions")
	}
	return total, nil
}

func (r Repository) CountProviders(ctx context.Context, organizationID string) (int64, error) {
	var total int64
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountProviders, organizationID).Scan(&total)
	})
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "count providers")
	}
	return total, nil
}

func (r Repository) CountActiveSessions(ctx context.Context, organizationID string) (*domaindashboard.SessionCountResult, error) {
	var result domaindashboard.SessionCountResult
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountActiveSessions, organizationID).Scan(
			&result.Total, &result.Active,
		)
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "count active sessions")
	}
	return &result, nil
}

func (r Repository) CountTokens(ctx context.Context, organizationID string) (*domaindashboard.TokenCountResult, error) {
	var result domaindashboard.TokenCountResult
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountTokens, organizationID).Scan(
			&result.Total, &result.Active,
		)
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "count tokens")
	}
	return &result, nil
}

func (r Repository) CountFailedLogins24h(ctx context.Context, organizationID string) (int64, error) {
	var count int64
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryCountFailedLogins24h, organizationID).Scan(&count)
	})
	if err != nil {
		return 0, apperrors.WrapInternalf(err, "count failed logins 24h")
	}
	return count, nil
}

func (r Repository) GetLoginHeatmap(ctx context.Context, organizationID string) ([]domaindashboard.HeatmapPoint, error) {
	var result []domaindashboard.HeatmapPoint
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, queryGetLoginHeatmap, organizationID)
		if err != nil {
			return err
		}
		defer rows.Close()

		hourCounts := make(map[int]int64)
		for rows.Next() {
			var h int
			var c int64
			if err := rows.Scan(&h, &c); err != nil {
				return err
			}
			hourCounts[h] = c
		}
		if err := rows.Err(); err != nil {
			return err
		}

		result = make([]domaindashboard.HeatmapPoint, 24)
		for h := 0; h < 24; h++ {
			result[h] = domaindashboard.HeatmapPoint{Hour: h, Count: hourCounts[h]}
		}
		return nil
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get login heatmap")
	}
	return result, nil
}

func (r Repository) GetMfaCoverageStats(ctx context.Context, organizationID string) (*domaindashboard.MfaCoverageResult, error) {
	var result domaindashboard.MfaCoverageResult
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, queryGetMfaCoverageStats, organizationID).Scan(
			&result.Total, &result.WithMfa,
		)
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get mfa coverage stats")
	}
	return &result, nil
}

func (r Repository) GetProviderDistribution(ctx context.Context, organizationID string) ([]domaindashboard.ProviderDistPoint, error) {
	var result []domaindashboard.ProviderDistPoint
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, queryGetProviderDistribution, organizationID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p domaindashboard.ProviderDistPoint
			if err := rows.Scan(&p.Category, &p.Count); err != nil {
				return err
			}
			result = append(result, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get provider distribution")
	}
	return result, nil
}

func (r Repository) GetTrends(ctx context.Context, organizationID string) ([]domaindashboard.TrendPoint, error) {
	var result []domaindashboard.TrendPoint
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, queryGetTrends, organizationID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p domaindashboard.TrendPoint
			if err := rows.Scan(&p.Date, &p.Users, &p.Applications, &p.Providers, &p.Logins); err != nil {
				return err
			}
			result = append(result, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get trends")
	}
	return result, nil
}

func (r Repository) GetActivityHeatmap(ctx context.Context, organizationID string) ([]domaindashboard.ActivityPoint, error) {
	var result []domaindashboard.ActivityPoint
	err := r.withRLSTx(ctx, organizationID, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, queryGetActivityHeatmap, organizationID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p domaindashboard.ActivityPoint
			if err := rows.Scan(&p.Date, &p.Count); err != nil {
				return err
			}
			result = append(result, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get activity heatmap")
	}
	return result, nil
}

func (r Repository) GetOrgUserDistribution(ctx context.Context) ([]domaindashboard.OrgUserDistPoint, error) {
	var result []domaindashboard.OrgUserDistPoint
	err := r.withRLSTx(ctx, "", func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, queryGetOrgUserDistribution)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var p domaindashboard.OrgUserDistPoint
			if err := rows.Scan(&p.OrganizationName, &p.UserCount); err != nil {
				return err
			}
			result = append(result, p)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "get org user distribution")
	}
	return result, nil
}

var _ domaindashboard.StatsRepository = Repository{}
