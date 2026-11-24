// admin_usage.go — UsageLogs / ProviderHealth 只读查询及 DTO↔Domain 转换。
package aigateway

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
)

// ---------------------------------------------------------------------------
// UsageLogs (只读)
// ---------------------------------------------------------------------------

func (s *AdminService) ListUsageLogs(ctx context.Context, orgID string, limit, offset int) ([]UsageLogResponse, int64, error) {
	// Same bound as every other served list: the transport middleware already
	// rejects an oversized query, and this is the repository-side net for
	// direct callers. Keeping a second 50/100 pair here made the service's
	// default disagree with the API's DefaultPageSize.
	limit = platformpagination.ClampPageSize(limit)
	logs, total, err := s.UsageLogs.List(ctx, orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	out := make([]UsageLogResponse, 0, len(logs))
	for _, u := range logs {
		out = append(out, *toUsageLogResponse(u))
	}
	return out, total, nil
}

// GetUsageStats returns aggregated usage statistics for the dashboard.
// days = 0 means no lookback filter (all-time); 7 = last 7 days.
func (s *AdminService) GetUsageStats(ctx context.Context, orgID string, days int) (*domain.UsageStats, error) {
	return s.UsageLogs.QueryStats(ctx, orgID, days)
}

// ---------------------------------------------------------------------------
// Provider Health (read-only admin view)
// ---------------------------------------------------------------------------

// ListHealth returns the circuit-breaker state of the providers owned by orgID.
// llm_provider_health has no organization_id by design, so the repository
// resolves ownership through llm_providers.organization_id; rows belonging to
// other tenants never leave the database.
func (s *AdminService) ListHealth(ctx context.Context, orgID string) ([]ProviderHealthResponse, error) {
	healths, err := s.Health.ListByOrganization(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderHealthResponse, 0, len(healths))
	for _, h := range healths {
		out = append(out, *toProviderHealthResponse(h))
	}
	return out, nil
}

// GetHealthForOrganization returns the health record addressed by id inside
// orgID. The console addresses the provider (GET /ai/health/{providerId}), so
// the provider is resolved through the tenant-scoped repository first: a
// foreign or unknown provider answers 404 before the tenant-less health table
// is queried without an organization predicate. The list payload exposes the
// health record id as well, so that address stays readable through the org-scoped
// fallback, which carries the same provider→organization predicate in SQL.
func (s *AdminService) GetHealthForOrganization(ctx context.Context, orgID, id string) (*ProviderHealthResponse, error) {
	repo, err := s.scopedProviders()
	if err != nil {
		return nil, err
	}
	p, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		if apperrors.FromError(err).Code == apperrors.CodeNotFound {
			return s.getHealthRecordForOrganization(ctx, orgID, id)
		}
		return nil, err
	}
	h, err := s.Health.GetByProvider(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	return toProviderHealthResponse(h), nil
}

// getHealthRecordForOrganization answers the legacy health-record address.
// The scoped read decides ownership, so a cross-tenant record is not found.
func (s *AdminService) getHealthRecordForOrganization(ctx context.Context, orgID, id string) (*ProviderHealthResponse, error) {
	h, err := s.Health.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return toProviderHealthResponse(h), nil
}

// ---------------------------------------------------------------------------
// DTO ↔ Domain converters
// ---------------------------------------------------------------------------

func toUsageLogResponse(u *domain.UsageLog) *UsageLogResponse {
	return &UsageLogResponse{
		ID:                u.ID,
		OrganizationID:    u.OrganizationID,
		APIKeyID:          u.APIKeyID,
		AliasID:           u.AliasID,
		ProviderID:        u.ProviderID,
		ModelID:           u.ModelID,
		ProtocolFace:      u.ProtocolFace,
		AttemptNo:         u.AttemptNo,
		InputTokens:       u.InputTokens,
		OutputTokens:      u.OutputTokens,
		TotalTokens:       u.TotalTokens,
		CostAmount:        u.CostAmount,
		Currency:          u.Currency,
		Status:            u.Status,
		ErrorCode:         u.ErrorCode,
		TTFTMs:            u.TTFTMs,
		TotalLatencyMs:    u.TotalLatencyMs,
		UpstreamRequestID: u.UpstreamRequestID,
		FailoverFrom:      u.FailoverFrom,
		RequestID:         u.RequestID,
		CreatedAt:         u.CreatedAt.Format(time.RFC3339),
	}
}

func toProviderHealthResponse(h *domain.ProviderHealth) *ProviderHealthResponse {
	r := &ProviderHealthResponse{
		ID:                  h.ID,
		ProviderID:          h.ProviderID,
		State:               h.State,
		ConsecutiveFailures: h.ConsecutiveFailures,
		UpdatedAt:           h.UpdatedAt.Format(time.RFC3339),
	}
	if h.OpenUntil != nil {
		s := h.OpenUntil.Format(time.RFC3339)
		r.OpenUntil = &s
	}
	if h.LastCheckedAt != nil {
		s := h.LastCheckedAt.Format(time.RFC3339)
		r.LastCheckedAt = &s
	}
	if h.LastLatencyMs != nil {
		ms := *h.LastLatencyMs
		r.LastLatencyMs = &ms
	}
	return r
}
