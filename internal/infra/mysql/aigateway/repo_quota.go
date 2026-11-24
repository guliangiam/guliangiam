package aigateway

import (
	"context"
	"database/sql"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	mysqlshared "github.com/guliangiam/guliangiam/internal/infra/mysql/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ============================================================================
// QuotaPoliciesRepo
// ============================================================================

type QuotaPoliciesRepository struct {
	db mysqlshared.DBTX
}

func NewQuotaPoliciesRepository(db mysqlshared.DBTX) QuotaPoliciesRepository {
	return QuotaPoliciesRepository{db: db}
}

var _ domain.QuotaPoliciesRepo = (*QuotaPoliciesRepository)(nil)
var _ domain.TenantScopedQuotaPoliciesRepo = (*QuotaPoliciesRepository)(nil)

func (r *QuotaPoliciesRepository) GetByID(ctx context.Context, id string) (*domain.LlmQuotaPolicy, error) {
	row := r.db.QueryRowContext(ctx, queryQuotaPolicyGetByID, id)
	q, err := scanLlmQuotaPolicy(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("quota policy not found")
		}
		return nil, err
	}
	return q, nil
}

// GetByIDForOrganization is the management by-ID read. The tenant predicate is
// explicit because MySQL has no RLS, so a foreign or missing id is NotFound.
func (r *QuotaPoliciesRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmQuotaPolicy, error) {
	row := r.db.QueryRowContext(ctx, queryQuotaPolicyGetByIDForOrganization, id, organizationID)
	q, err := scanLlmQuotaPolicy(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("quota policy not found")
		}
		return nil, err
	}
	return q, nil
}

func (r *QuotaPoliciesRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmQuotaPolicy, error) {
	rows, err := r.db.QueryContext(ctx, queryQuotaPolicyList, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	policies := make([]*domain.LlmQuotaPolicy, 0, 8)
	for rows.Next() {
		q, err := scanLlmQuotaPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, q)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return policies, nil
}

func (r *QuotaPoliciesRepository) Create(ctx context.Context, q *domain.LlmQuotaPolicy) error {
	_, err := r.db.ExecContext(ctx, queryQuotaPolicyInsert,
		q.ID, q.OrganizationID, q.Scope, q.ScopeRef,
		q.LimitType, q.LimitValue, q.ResetCycle, q.Status,
		q.CreatedAt, q.UpdatedAt, q.DeletedAt, q.Revision,
	)
	return err
}

func (r *QuotaPoliciesRepository) Update(ctx context.Context, q *domain.LlmQuotaPolicy) error {
	res, err := r.db.ExecContext(ctx, queryQuotaPolicyUpdate,
		q.Scope, q.ScopeRef, q.LimitType, q.LimitValue, q.ResetCycle, q.Status,
		q.UpdatedAt, q.DeletedAt,
		q.ID, q.Revision,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("quota policy revision conflict (id=%s)", q.ID)
	}
	q.Revision++
	return nil
}

func (r *QuotaPoliciesRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, queryQuotaPolicyDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// The explicit tenant predicate plus current-revision guard means a foreign id
// is a conflict, never a cross-tenant update.
func (r *QuotaPoliciesRepository) UpdateForOrganization(ctx context.Context, q *domain.LlmQuotaPolicy, organizationID string) error {
	res, err := r.db.ExecContext(ctx, queryQuotaPolicyUpdateForOrganization,
		q.Scope, q.ScopeRef, q.LimitType, q.LimitValue, q.ResetCycle, q.Status,
		q.UpdatedAt, q.DeletedAt,
		q.ID, q.Revision, organizationID,
	)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewConflictf("quota policy revision conflict (id=%s)", q.ID)
	}
	q.Revision++
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r *QuotaPoliciesRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	res, err := r.db.ExecContext(ctx, queryQuotaPolicyDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return apperrors.NewNotFound("quota policy not found")
	}
	return nil
}

// ============================================================================
// ProviderHealthRepo（tenant-less）
// ============================================================================

type ProviderHealthRepository struct {
	db mysqlshared.DBTX
}

func NewProviderHealthRepository(db mysqlshared.DBTX) ProviderHealthRepository {
	return ProviderHealthRepository{db: db}
}

var _ domain.ProviderHealthRepo = (*ProviderHealthRepository)(nil)

func (r *ProviderHealthRepository) GetByID(ctx context.Context, id string) (*domain.ProviderHealth, error) {
	row := r.db.QueryRowContext(ctx, queryProviderHealthGetByID, id)
	h, err := scanProviderHealth(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider health not found")
		}
		return nil, err
	}
	return h, nil
}

func (r *ProviderHealthRepository) GetByProvider(ctx context.Context, providerID string) (*domain.ProviderHealth, error) {
	row := r.db.QueryRowContext(ctx, queryProviderHealthGetByProvider, providerID)
	h, err := scanProviderHealth(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider health not found")
		}
		return nil, err
	}
	return h, nil
}

// GetByIDForOrganization resolves tenant ownership through
// llm_provider_health.provider_id → llm_providers.id; a foreign record yields
// the same 0-row result as a missing one (MySQL has no RLS to fall back on).
func (r *ProviderHealthRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.ProviderHealth, error) {
	row := r.db.QueryRowContext(ctx, queryProviderHealthGetByIDForOrganization, id, organizationID)
	h, err := scanProviderHealth(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, apperrors.NewNotFound("provider health not found")
		}
		return nil, err
	}
	return h, nil
}

func (r *ProviderHealthRepository) Upsert(ctx context.Context, h *domain.ProviderHealth) error {
	_, err := r.db.ExecContext(ctx, queryProviderHealthUpsert,
		h.ID, h.ProviderID, h.State, h.ConsecutiveFailures,
		h.OpenUntil, h.LastCheckedAt, h.LastLatencyMs, h.UpdatedAt,
	)
	return err
}

func (r *ProviderHealthRepository) ListOpen(ctx context.Context) ([]*domain.ProviderHealth, error) {
	rows, err := r.db.QueryContext(ctx, queryProviderHealthListOpen)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.ProviderHealth, 0, 16)
	for rows.Next() {
		h, err := scanProviderHealth(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return out, nil
}

func (r *ProviderHealthRepository) ListAll(ctx context.Context) ([]*domain.ProviderHealth, error) {
	rows, err := r.db.QueryContext(ctx, queryProviderHealthListAll)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.ProviderHealth, 0, 32)
	for rows.Next() {
		h, err := scanProviderHealth(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ListByOrganization returns only rows whose provider belongs to organizationID.
func (r *ProviderHealthRepository) ListByOrganization(ctx context.Context, organizationID string) ([]*domain.ProviderHealth, error) {
	rows, err := r.db.QueryContext(ctx, queryProviderHealthListByOrganization, organizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.ProviderHealth, 0, 32)
	for rows.Next() {
		h, err := scanProviderHealth(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ============================================================================
// UsageLogsRepo
// ============================================================================

type UsageLogsRepository struct {
	db mysqlshared.DBTX
}

func NewUsageLogsRepository(db mysqlshared.DBTX) UsageLogsRepository {
	return UsageLogsRepository{db: db}
}

var _ domain.UsageLogsRepo = (*UsageLogsRepository)(nil)

func (r *UsageLogsRepository) Create(ctx context.Context, u *domain.UsageLog) error {
	_, err := r.db.ExecContext(ctx, queryUsageLogInsert,
		u.ID, u.OrganizationID, u.APIKeyID, u.AliasID, u.ProviderID, u.ModelID,
		u.PriceVersionID, u.ProtocolFace, u.AttemptNo,
		u.InputTokens, u.OutputTokens, u.TotalTokens,
		u.CostAmount, u.Currency, u.Status,
		u.ErrorCode, u.TTFTMs, u.TotalLatencyMs,
		u.UpstreamRequestID, u.FailoverFrom, u.RequestID, u.CreatedAt,
	)
	return err
}

func (r *UsageLogsRepository) List(ctx context.Context, organizationID string, limit, offset int) ([]*domain.UsageLog, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, queryUsageLogCount, organizationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, queryUsageLogList, organizationID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := make([]*domain.UsageLog, 0, limit)
	for rows.Next() {
		u, err := scanUsageLog(rows)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, u)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, 0, rowsErr
	}
	return logs, total, nil
}

// QueryStats 实现 domain.UsageLogsRepo.QueryStats。
// MySQL 版：FILTER → SUM(CASE WHEN), DATE_TRUNC → DATE(), INTERVAL → DATE_SUB
// days>0 时加 DATE_SUB(NOW(), INTERVAL ? DAY) 过滤。
func (r *UsageLogsRepository) QueryStats(ctx context.Context, orgID string, days int) (*domain.UsageStats, error) {
	stats := &domain.UsageStats{}
	if orgID == "" {
		return stats, nil
	}

	// ── 1. Totals ──
	var totalsSQL string
	var totalsArgs []any
	if days > 0 {
		totalsSQL = queryStatsTotals + queryStatsWhereOrgDays
		totalsArgs = []any{orgID, days}
	} else {
		totalsSQL = queryStatsTotals + queryStatsWhereOrg
		totalsArgs = []any{orgID}
	}

	var avgTTFTMs sql.NullFloat64
	var avgLatencyMs sql.NullFloat64
	row := r.db.QueryRowContext(ctx, totalsSQL, totalsArgs...)
	if err := row.Scan(
		&stats.TotalRequests,
		&stats.SucceededCount,
		&stats.FailedCount,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalTokens,
		&stats.TotalCostUSD,
		&avgTTFTMs,
		&avgLatencyMs,
	); err != nil {
		return nil, err
	}
	if avgTTFTMs.Valid {
		v := avgTTFTMs.Float64
		stats.AvgTTFTMs = &v
	}
	if avgLatencyMs.Valid {
		v := avgLatencyMs.Float64
		stats.AvgLatencyMs = &v
	}

	// ── 2. By Provider ──
	var byProviderSQL string
	var byProviderArgs []any
	if days > 0 {
		byProviderSQL = queryStatsByProviderDays
		byProviderArgs = []any{orgID, days}
	} else {
		byProviderSQL = queryStatsByProvider
		byProviderArgs = []any{orgID}
	}

	providerRows, err := r.db.QueryContext(ctx, byProviderSQL, byProviderArgs...)
	if err != nil {
		return nil, err
	}
	defer providerRows.Close()
	for providerRows.Next() {
		var b domain.ProviderUsageBreakdown
		if err := providerRows.Scan(&b.ProviderID, &b.Requests, &b.Tokens, &b.CostUSD); err != nil {
			return nil, err
		}
		stats.ByProvider = append(stats.ByProvider, b)
	}
	if err := providerRows.Err(); err != nil {
		return nil, err
	}

	// ── 3. By Day ──
	var byDaySQL string
	var byDayArgs []any
	if days > 0 {
		byDaySQL = queryStatsByDayDays
		byDayArgs = []any{orgID, days}
	} else {
		byDaySQL = queryStatsByDay
		byDayArgs = []any{orgID}
	}

	dayRows, err := r.db.QueryContext(ctx, byDaySQL, byDayArgs...)
	if err != nil {
		return nil, err
	}
	defer dayRows.Close()
	for dayRows.Next() {
		var d domain.DailyUsageBreakdown
		var t time.Time
		if err := dayRows.Scan(&t, &d.Tokens, &d.Cost); err != nil {
			return nil, err
		}
		d.Date = t.Format("2006-01-02")
		stats.ByDay = append(stats.ByDay, d)
	}
	if err := dayRows.Err(); err != nil {
		return nil, err
	}

	// ── 4. By Protocol ──
	var byProtocolSQL string
	var byProtocolArgs []any
	if days > 0 {
		byProtocolSQL = queryStatsByProtocolDays
		byProtocolArgs = []any{orgID, days}
	} else {
		byProtocolSQL = queryStatsByProtocol
		byProtocolArgs = []any{orgID}
	}

	protoRows, err := r.db.QueryContext(ctx, byProtocolSQL, byProtocolArgs...)
	if err != nil {
		return nil, err
	}
	defer protoRows.Close()
	for protoRows.Next() {
		var p domain.ProtocolUsageBreakdown
		if err := protoRows.Scan(&p.ProtocolFace, &p.Requests, &p.Tokens); err != nil {
			return nil, err
		}
		stats.ByProtocol = append(stats.ByProtocol, p)
	}
	if err := protoRows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

// ============================================================================
// MeteringEventRepo — outbox 审计落表
// ============================================================================

type meteringEventsRepo struct {
	db mysqlshared.DBTX
}

// NewMeteringEventRepo 接受任意 mysqlshared.DBTX（*sql.DB / *sql.Tx 均可）。
func NewMeteringEventRepo(db mysqlshared.DBTX) domain.MeteringEventRepo {
	return &meteringEventsRepo{db: db}
}

func (r *meteringEventsRepo) Save(ctx context.Context, e *domain.MeteringEvent) error {
	if e == nil {
		return nil
	}
	if e.ID == "" {
		return apperrors.NewInvalidArgument("metering event id required")
	}

	_, err := r.db.ExecContext(ctx, queryMeteringEventSave,
		e.ID,
		e.OutboxID,
		e.OrganizationID,
		e.AggregateType,
		e.AggregateID,
		e.EventType,
		e.ProviderID,
		e.ModelID,
		e.PayloadJSON,
	)
	return err
}
