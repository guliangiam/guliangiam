// admin_quota.go — Quota / PriceVersion 聚合 CRUD 及 DTO↔Domain 转换。
package aigateway

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// QuotaPolicy CRUD
// ---------------------------------------------------------------------------

func (s *AdminService) CreateQuota(ctx context.Context, req QuotaCreate) (*QuotaResponse, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternalf("generate quota id: %v", err)
	}
	now := s.Clock.Now()
	q := &domain.LlmQuotaPolicy{
		ID:             id,
		OrganizationID: req.OrganizationID,
		Scope:          req.Scope,
		ScopeRef:       req.ScopeRef,
		LimitType:      req.LimitType,
		LimitValue:     req.LimitValue,
		ResetCycle:     req.ResetCycle,
		Status:         req.Status,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	if err := s.Quotas.Create(ctx, q); err != nil {
		return nil, err
	}
	return toQuotaResponse(q), nil
}

func (s *AdminService) ListQuotas(ctx context.Context, orgID string) ([]QuotaResponse, error) {
	qs, err := s.Quotas.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]QuotaResponse, 0, len(qs))
	for _, q := range qs {
		out = append(out, *toQuotaResponse(q))
	}
	return out, nil
}

// scopedQuotas resolves the management-plane quota contract. The runtime
// repository satisfies QuotaPoliciesRepo only; management routes must fail
// closed rather than fall back to an unscoped read.
func (s *AdminService) scopedQuotas() (domain.TenantScopedQuotaPoliciesRepo, error) {
	repo, ok := s.Quotas.(domain.TenantScopedQuotaPoliciesRepo)
	if !ok {
		return nil, apperrors.NewInternal("quota repository does not implement tenant scoping")
	}
	return repo, nil
}

func (s *AdminService) GetQuota(ctx context.Context, orgID, id string) (*QuotaResponse, error) {
	repo, err := s.scopedQuotas()
	if err != nil {
		return nil, err
	}
	q, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return toQuotaResponse(q), nil
}

func (s *AdminService) UpdateQuota(ctx context.Context, orgID, id string, req QuotaUpdate) (*QuotaResponse, error) {
	repo, err := s.scopedQuotas()
	if err != nil {
		return nil, err
	}
	q, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	if req.ScopeRef != nil {
		q.ScopeRef = req.ScopeRef
	}
	if req.LimitType != nil {
		q.LimitType = *req.LimitType
	}
	if req.LimitValue != nil {
		q.LimitValue = *req.LimitValue
	}
	if req.ResetCycle != nil {
		q.ResetCycle = *req.ResetCycle
	}
	if req.Status != nil {
		q.Status = *req.Status
	}
	q.UpdatedAt = now

	if err := q.Validate(); err != nil {
		return nil, err
	}
	if err := repo.UpdateForOrganization(ctx, q, orgID); err != nil {
		return nil, err
	}
	return toQuotaResponse(q), nil
}

func (s *AdminService) DeleteQuota(ctx context.Context, orgID, id string) error {
	repo, err := s.scopedQuotas()
	if err != nil {
		return err
	}
	// The scoped soft delete carries the tenant predicate itself, so no
	// read-then-write window exists between ownership check and deletion.
	return repo.DeleteForOrganization(ctx, orgID, id)
}

// ---------------------------------------------------------------------------
// Price Version management
// ---------------------------------------------------------------------------

func (s *AdminService) ListPrices(ctx context.Context, orgID string) ([]PriceVersionResponse, error) {
	versions, err := s.Prices.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]PriceVersionResponse, 0, len(versions))
	for _, v := range versions {
		out = append(out, *toPriceVersionResponse(v))
	}
	return out, nil
}

// scopedPrices resolves the management-plane price contract. The runtime
// repository satisfies PriceVersionsRepo only; management routes must fail
// closed rather than fall back to an unscoped read.
func (s *AdminService) scopedPrices() (domain.TenantScopedPriceVersionsRepo, error) {
	repo, ok := s.Prices.(domain.TenantScopedPriceVersionsRepo)
	if !ok {
		return nil, apperrors.NewInternal("price repository does not implement tenant scoping")
	}
	return repo, nil
}

func (s *AdminService) GetPrice(ctx context.Context, orgID, id string) (*PriceVersionResponse, error) {
	repo, err := s.scopedPrices()
	if err != nil {
		return nil, err
	}
	v, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return toPriceVersionResponse(v), nil
}

func (s *AdminService) CreatePrice(ctx context.Context, req PriceVersionCreate) (*PriceVersionResponse, error) {
	// Validate model belongs to this org
	m, err := s.Models.GetByID(ctx, req.ModelID)
	if err != nil {
		return nil, apperrors.NewInvalidArgument("model not found")
	}
	if m.OrganizationID != req.OrganizationID {
		return nil, apperrors.NewInvalidArgument("model does not belong to this organization")
	}

	// Compute next version_no for this model
	existing, _ := s.Prices.ListByModel(ctx, req.ModelID)
	nextVersion := 1
	for _, ev := range existing {
		if ev.VersionNo >= nextVersion {
			nextVersion = ev.VersionNo + 1
		}
	}

	effFrom, err := time.Parse(time.RFC3339, req.EffectiveFrom)
	if err != nil {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "effectiveFrom must be RFC3339", map[string]any{"field": "effectiveFrom", "value": req.EffectiveFrom, "format": "RFC3339"})
	}
	if req.InputPricePer1M < 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "prices cannot be negative", map[string]any{"field": "inputPricePer1M", "value": req.InputPricePer1M, "min": 0})
	}
	if req.OutputPricePer1M < 0 {
		return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "prices cannot be negative", map[string]any{"field": "outputPricePer1M", "value": req.OutputPricePer1M, "min": 0})
	}

	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternalf("generate price version id: %v", err)
	}
	now := s.Clock.Now().UTC()
	currency := req.Currency
	if currency == "" {
		currency = "USD"
	}

	v := &domain.LlmPriceVersion{
		ID:               id,
		OrganizationID:   req.OrganizationID,
		ModelID:          req.ModelID,
		VersionNo:        nextVersion,
		InputPricePer1M:  req.InputPricePer1M,
		OutputPricePer1M: req.OutputPricePer1M,
		Currency:         currency,
		EffectiveFrom:    effFrom.UTC(),
		CreatedAt:        now,
		UpdatedAt:        now,
		Revision:         0,
	}
	if err := s.Prices.Create(ctx, v); err != nil {
		return nil, err
	}
	return toPriceVersionResponse(v), nil
}

func (s *AdminService) UpdatePrice(ctx context.Context, orgID, id string, req PriceVersionUpdate) (*PriceVersionResponse, error) {
	repo, err := s.scopedPrices()
	if err != nil {
		return nil, err
	}
	v, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	if req.InputPricePer1M != nil {
		if *req.InputPricePer1M < 0 {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "inputPricePer1M cannot be negative", map[string]any{"field": "inputPricePer1M", "value": *req.InputPricePer1M, "min": 0})
		}
		v.InputPricePer1M = *req.InputPricePer1M
	}
	if req.OutputPricePer1M != nil {
		if *req.OutputPricePer1M < 0 {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "outputPricePer1M cannot be negative", map[string]any{"field": "outputPricePer1M", "value": *req.OutputPricePer1M, "min": 0})
		}
		v.OutputPricePer1M = *req.OutputPricePer1M
	}
	if req.Currency != nil {
		v.Currency = *req.Currency
	}
	if req.EffectiveFrom != nil {
		t, err := time.Parse(time.RFC3339, *req.EffectiveFrom)
		if err != nil {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "effectiveFrom must be RFC3339", map[string]any{"field": "effectiveFrom", "value": *req.EffectiveFrom, "format": "RFC3339"})
		}
		v.EffectiveFrom = t.UTC()
	}
	v.UpdatedAt = s.Clock.Now().UTC()

	if err := repo.UpdateForOrganization(ctx, v, orgID); err != nil {
		return nil, err
	}
	return toPriceVersionResponse(v), nil
}

func (s *AdminService) DeletePrice(ctx context.Context, orgID, id string) error {
	repo, err := s.scopedPrices()
	if err != nil {
		return err
	}
	// The scoped soft delete carries the tenant predicate itself, so no
	// read-then-write window exists between ownership check and deletion.
	return repo.DeleteForOrganization(ctx, orgID, id)
}

// ---------------------------------------------------------------------------
// DTO ↔ Domain converter
// ---------------------------------------------------------------------------

func toQuotaResponse(q *domain.LlmQuotaPolicy) *QuotaResponse {
	return &QuotaResponse{
		ID:             q.ID,
		OrganizationID: q.OrganizationID,
		Scope:          q.Scope,
		ScopeRef:       q.ScopeRef,
		LimitType:      q.LimitType,
		LimitValue:     q.LimitValue,
		ResetCycle:     q.ResetCycle,
		Status:         q.Status,
		CreatedAt:      q.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      q.UpdatedAt.Format(time.RFC3339),
	}
}

func toPriceVersionResponse(v *domain.LlmPriceVersion) *PriceVersionResponse {
	return &PriceVersionResponse{
		ID:               v.ID,
		OrganizationID:   v.OrganizationID,
		ModelID:          v.ModelID,
		VersionNo:        v.VersionNo,
		InputPricePer1M:  v.InputPricePer1M,
		OutputPricePer1M: v.OutputPricePer1M,
		Currency:         v.Currency,
		EffectiveFrom:    v.EffectiveFrom.Format(time.RFC3339),
		CreatedAt:        v.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        v.UpdatedAt.Format(time.RFC3339),
	}
}
