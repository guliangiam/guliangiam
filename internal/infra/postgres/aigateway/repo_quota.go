package aigateway

import (
	"context"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/jackc/pgx/v5"
)

// ============================================================================
// QuotaPoliciesRepo
// ============================================================================

type QuotaPoliciesRepository struct {
	db shared.DBTX
}

func NewQuotaPoliciesRepository(db shared.DBTX) QuotaPoliciesRepository {
	return QuotaPoliciesRepository{db: db}
}

var _ domain.QuotaPoliciesRepo = QuotaPoliciesRepository{}
var _ domain.TenantScopedQuotaPoliciesRepo = QuotaPoliciesRepository{}
var _ domain.ActiveQuotaPoliciesLister = QuotaPoliciesRepository{}

func (r QuotaPoliciesRepository) GetByID(ctx context.Context, id string) (*domain.LlmQuotaPolicy, error) {
	row := r.db.QueryRow(ctx, queryQuotaPolicyGetByID, id)
	q, err := scanLlmQuotaPolicy(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("quota policy not found")
		}
		return nil, err
	}
	return q, nil
}

// GetByIDForOrganization is the management by-ID read. The trusted tenant is
// an explicit predicate, so a foreign or missing id is NotFound.
func (r QuotaPoliciesRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.LlmQuotaPolicy, error) {
	row := r.db.QueryRow(ctx, queryQuotaPolicyGetByIDForOrganization, id, organizationID)
	q, err := scanLlmQuotaPolicy(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("quota policy not found")
		}
		return nil, err
	}
	return q, nil
}

func (r QuotaPoliciesRepository) List(ctx context.Context, organizationID string) ([]*domain.LlmQuotaPolicy, error) {
	return r.listByQuery(ctx, queryQuotaPolicyList, organizationID)
}

// ListActive 只返回 status='active' 的策略，供数据面 enforcement 使用；
// 管理面 List 保留全状态（管理 UI 需要看到 disabled 策略）。
func (r QuotaPoliciesRepository) ListActive(ctx context.Context, organizationID string) ([]*domain.LlmQuotaPolicy, error) {
	return r.listByQuery(ctx, queryQuotaPolicyListActive, organizationID)
}

func (r QuotaPoliciesRepository) listByQuery(ctx context.Context, query, organizationID string) ([]*domain.LlmQuotaPolicy, error) {
	rows, err := r.db.Query(ctx, query, organizationID)
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

func (r QuotaPoliciesRepository) Create(ctx context.Context, q *domain.LlmQuotaPolicy) error {
	_, err := r.db.Exec(ctx, queryQuotaPolicyInsert,
		q.ID, q.OrganizationID, q.Scope, q.ScopeRef,
		q.LimitType, q.LimitValue, q.ResetCycle, q.Status,
		q.CreatedAt, q.UpdatedAt, q.DeletedAt, q.Revision,
	)
	return err
}

func (r QuotaPoliciesRepository) Update(ctx context.Context, q *domain.LlmQuotaPolicy) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryQuotaPolicyUpdate,
		q.Scope, q.ScopeRef, q.LimitType, q.LimitValue, q.ResetCycle, q.Status,
		q.UpdatedAt, q.DeletedAt,
		q.Revision-1, q.ID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("quota policy revision conflict")
		}
		return err
	}
	q.Revision = newRevision
	return nil
}

func (r QuotaPoliciesRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, queryQuotaPolicyDelete, id)
	return err
}

// UpdateForOrganization persists the aggregate only inside organizationID.
// The current revision is the guard; a concurrent ownership/revision change
// surfaces as the same conflict as a stale write, never a cross-tenant update.
func (r QuotaPoliciesRepository) UpdateForOrganization(ctx context.Context, q *domain.LlmQuotaPolicy, organizationID string) error {
	var newRevision int64
	err := r.db.QueryRow(ctx, queryQuotaPolicyUpdateForOrganization,
		q.Scope, q.ScopeRef, q.LimitType, q.LimitValue, q.ResetCycle, q.Status,
		q.UpdatedAt, q.DeletedAt,
		q.Revision, q.ID, organizationID,
	).Scan(&newRevision)
	if err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.NewConflict("quota policy revision conflict")
		}
		return err
	}
	q.Revision = newRevision
	return nil
}

// DeleteForOrganization soft-deletes only a row owned by organizationID. A
// foreign or already deleted id affects no rows and is reported as NotFound.
func (r QuotaPoliciesRepository) DeleteForOrganization(ctx context.Context, organizationID, id string) error {
	tag, err := r.db.Exec(ctx, queryQuotaPolicyDeleteForOrganization, id, organizationID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperrors.NewNotFound("quota policy not found")
	}
	return nil
}

// ============================================================================
// ProviderHealthRepo (tenant-less)
// ============================================================================

type ProviderHealthRepository struct {
	db shared.DBTX
}

func NewProviderHealthRepository(db shared.DBTX) ProviderHealthRepository {
	return ProviderHealthRepository{db: db}
}

var _ domain.ProviderHealthRepo = ProviderHealthRepository{}

func (r ProviderHealthRepository) GetByID(ctx context.Context, id string) (*domain.ProviderHealth, error) {
	row := r.db.QueryRow(ctx, queryProviderHealthGetByID, id)
	h, err := scanProviderHealth(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("provider health not found")
		}
		return nil, err
	}
	return h, nil
}

func (r ProviderHealthRepository) GetByProvider(ctx context.Context, providerID string) (*domain.ProviderHealth, error) {
	row := r.db.QueryRow(ctx, queryProviderHealthGetByProvider, providerID)
	h, err := scanProviderHealth(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("provider health not found")
		}
		return nil, err
	}
	return h, nil
}

// GetByIDForOrganization resolves tenant ownership through
// llm_provider_health.provider_id → llm_providers.id; a foreign record yields
// the same 0-row result as a missing one.
func (r ProviderHealthRepository) GetByIDForOrganization(ctx context.Context, organizationID, id string) (*domain.ProviderHealth, error) {
	row := r.db.QueryRow(ctx, queryProviderHealthGetByIDForOrganization, id, organizationID)
	h, err := scanProviderHealth(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, apperrors.NewNotFound("provider health not found")
		}
		return nil, err
	}
	return h, nil
}

func (r ProviderHealthRepository) Upsert(ctx context.Context, h *domain.ProviderHealth) error {
	_, err := r.db.Exec(ctx, queryProviderHealthUpsert,
		h.ID, h.ProviderID, h.State, h.ConsecutiveFailures,
		h.OpenUntil, h.LastCheckedAt, h.LastLatencyMs, h.UpdatedAt,
	)
	return err
}

func (r ProviderHealthRepository) ListOpen(ctx context.Context) ([]*domain.ProviderHealth, error) {
	rows, err := r.db.Query(ctx, queryProviderHealthListOpen)
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

func (r ProviderHealthRepository) ListAll(ctx context.Context) ([]*domain.ProviderHealth, error) {
	rows, err := r.db.Query(ctx, queryProviderHealthListAll)
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
func (r ProviderHealthRepository) ListByOrganization(ctx context.Context, organizationID string) ([]*domain.ProviderHealth, error) {
	rows, err := r.db.Query(ctx, queryProviderHealthListByOrganization, organizationID)
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
	db shared.DBTX
}

func NewUsageLogsRepository(db shared.DBTX) UsageLogsRepository {
	return UsageLogsRepository{db: db}
}

var _ domain.UsageLogsRepo = UsageLogsRepository{}

func (r UsageLogsRepository) Create(ctx context.Context, u *domain.UsageLog) error {
	_, err := r.db.Exec(ctx, queryUsageLogInsert,
		u.ID, u.OrganizationID, u.APIKeyID, u.AliasID, u.ProviderID, u.ModelID,
		u.PriceVersionID, u.ProtocolFace, u.AttemptNo,
		u.InputTokens, u.OutputTokens, u.TotalTokens,
		u.CostAmount, u.Currency, u.Status,
		u.ErrorCode, u.TTFTMs, u.TotalLatencyMs,
		u.UpstreamRequestID, u.FailoverFrom, u.RequestID, u.CreatedAt,
	)
	return err
}

func (r UsageLogsRepository) List(ctx context.Context, organizationID string, limit, offset int) ([]*domain.UsageLog, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, queryUsageLogCount, organizationID).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, queryUsageLogList, organizationID, limit, offset)
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

// ============================================================================
// MeteringEventRepo — outbox 审计落表
// ============================================================================

type meteringEventsRepo struct {
	db shared.DBTX
}

// NewMeteringEventRepo 接受任意 shared.DBTX（pgxpool.Pool / pgx.Tx 均可）。
// Outbox Worker handler 用 pool 做长连接，unit-of-work 事务内也可以传 tx。
func NewMeteringEventRepo(db shared.DBTX) domain.MeteringEventRepo {
	return &meteringEventsRepo{db: db}
}

func (r *meteringEventsRepo) Save(ctx context.Context, e *domain.MeteringEvent) error {
	if e == nil {
		return nil
	}
	if e.ID == "" {
		return apperrors.NewInvalidArgument("metering event id required")
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO guliangiam.llm_metering_events
			(id, outbox_id, organization_id, aggregate_type, aggregate_id, event_type,
			 provider_id, model_id, payload_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO NOTHING
	`,
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

// ============================================================================
// UsageStats — admin dashboard aggregation
// ============================================================================

// QueryStats 实现 domain.UsageLogsRepo.QueryStats。
// 用 4 个简单聚合查询（totals + by_provider + by_day + by_protocol）——
// 每个查询都走 RLS FORCE + organization_id 过滤。
// days > 0 时加 WHERE created_at >= NOW() - interval 'N days'。
func (r UsageLogsRepository) QueryStats(ctx context.Context, orgID string, days int) (*domain.UsageStats, error) {
	stats := &domain.UsageStats{}
	if orgID == "" {
		return stats, nil
	}

	var whereSQL string
	var args []any
	args = append(args, orgID)
	if days > 0 {
		whereSQL = "WHERE organization_id = $1 AND created_at >= NOW() - $2::INTERVAL"
		args = append(args, fmt.Sprintf("%d days", days))
	} else {
		whereSQL = "WHERE organization_id = $1"
	}

	// ── 1. Totals ──
	row := r.db.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'succeeded'),
			COUNT(*) FILTER (WHERE status IN ('failed','throttled')),
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0),
			AVG(ttft_ms),
			AVG(total_latency_ms)
		FROM guliangiam.llm_usage_logs `+whereSQL, args...)
	if err := row.Scan(
		&stats.TotalRequests,
		&stats.SucceededCount,
		&stats.FailedCount,
		&stats.TotalInputTokens,
		&stats.TotalOutputTokens,
		&stats.TotalTokens,
		&stats.TotalCostUSD,
		&stats.AvgTTFTMs,
		&stats.AvgLatencyMs,
	); err != nil {
		return nil, err
	}

	// ── 2. By Provider ──
	providerRows, err := r.db.Query(ctx, `
		SELECT provider_id,
		       COUNT(*),
		       COALESCE(SUM(total_tokens), 0),
		       COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0)
		FROM guliangiam.llm_usage_logs `+whereSQL+`
		GROUP BY provider_id
		ORDER BY SUM(total_tokens) DESC
		LIMIT 20
	`, args...)
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

	// ── 3. By Day ──
	dayRows, err := r.db.Query(ctx, `
		SELECT DATE_TRUNC('day', created_at)::date as d,
		       COALESCE(SUM(total_tokens), 0),
		       COALESCE(SUM(CASE WHEN currency = 'USD' THEN cost_amount ELSE 0 END), 0)
		FROM guliangiam.llm_usage_logs `+whereSQL+`
		GROUP BY d
		ORDER BY d DESC
		LIMIT 30
	`, args...)
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

	// ── 4. By Protocol ──
	protoRows, err := r.db.Query(ctx, `
		SELECT protocol_face,
		       COUNT(*),
		       COALESCE(SUM(total_tokens), 0)
		FROM guliangiam.llm_usage_logs `+whereSQL+`
		GROUP BY protocol_face
		ORDER BY COUNT(*) DESC
	`, args...)
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

	return stats, nil
}
