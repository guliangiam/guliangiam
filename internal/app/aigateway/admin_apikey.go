// admin_apikey.go — APIKey / Route 聚合 CRUD 及 DTO↔Domain 转换。
package aigateway

import (
	"context"
	"fmt"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// API Key CRUD
// ---------------------------------------------------------------------------

func (s *AdminService) CreateAPIKey(ctx context.Context, req APIKeyCreate) (*APIKeyResponse, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternalf("generate api key id: %v", err)
	}

	key := &domain.LlmApiKey{
		ID:             id,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		AllowedAliases: req.AllowedAliases,
		RPMLimit:       req.RPMLimit,
		TPMLimit:       req.TPMLimit,
		MonthlyBudget:  req.MonthlyBudget,
		Currency:       req.Currency,
		Status:         domain.APIKeyStatusActive,
		CreatedAt:      s.Clock.Now(),
		UpdatedAt:      s.Clock.Now(),
		Revision:       1,
	}
	if req.ExpiresAt != nil {
		t, perr := time.Parse(time.RFC3339, *req.ExpiresAt)
		if perr != nil {
			return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("expiresAt must be RFC3339: %v", perr), map[string]any{"field": "expiresAt", "value": *req.ExpiresAt, "format": "RFC3339"})
		}
		key.ExpiresAt = &t
	}

	plaintext, err := key.GenerateKey()
	if err != nil {
		return nil, err
	}
	if err := key.Validate(); err != nil {
		return nil, err
	}
	if err := s.ApiKeys.Create(ctx, key); err != nil {
		return nil, err
	}

	resp := toAPIKeyResponse(key)
	resp.Plaintext = plaintext // 仅 Create 响应携带一次
	return resp, nil
}

func (s *AdminService) ListAPIKeys(ctx context.Context, orgID string) ([]APIKeyResponse, error) {
	keys, err := s.ApiKeys.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		out = append(out, *toAPIKeyResponse(k))
	}
	return out, nil
}

func (s *AdminService) GetAPIKey(ctx context.Context, orgID, id string) (*APIKeyResponse, error) {
	k, err := s.ApiKeys.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if k.OrganizationID != orgID {
		return nil, apperrors.NewNotFound("api key not found")
	}
	return toAPIKeyResponse(k), nil
}

func (s *AdminService) UpdateAPIKey(ctx context.Context, orgID, id string, req APIKeyUpdate) (*APIKeyResponse, error) {
	k, err := s.ApiKeys.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if k.OrganizationID != orgID {
		return nil, apperrors.NewNotFound("api key not found")
	}

	now := s.Clock.Now()
	if req.Name != nil {
		k.Name = *req.Name
	}
	if req.AllowedAliases != nil {
		k.AllowedAliases = req.AllowedAliases
	}
	if req.RPMLimit != nil {
		k.RPMLimit = req.RPMLimit
	}
	if req.TPMLimit != nil {
		k.TPMLimit = req.TPMLimit
	}
	if req.MonthlyBudget != nil {
		k.MonthlyBudget = req.MonthlyBudget
	}
	if req.Currency != nil {
		k.Currency = *req.Currency
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			k.ExpiresAt = nil
		} else {
			t, perr := time.Parse(time.RFC3339, *req.ExpiresAt)
			if perr != nil {
				return nil, apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, fmt.Sprintf("expiresAt must be RFC3339: %v", perr), map[string]any{"field": "expiresAt", "value": *req.ExpiresAt, "format": "RFC3339"})
			}
			k.ExpiresAt = &t
		}
	}
	if req.Status != nil {
		k.Status = *req.Status
	}
	k.UpdatedAt = now

	if err := k.Validate(); err != nil {
		return nil, err
	}
	if err := s.ApiKeys.Update(ctx, k); err != nil {
		return nil, err
	}
	return toAPIKeyResponse(k), nil
}

func (s *AdminService) DeleteAPIKey(ctx context.Context, orgID, id string) error {
	k, err := s.ApiKeys.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if k.OrganizationID != orgID {
		return apperrors.NewNotFound("api key not found")
	}
	return s.ApiKeys.Delete(ctx, id)
}

// RevokeAPIKey 软撤销 —— 撤销后不可恢复，应通过新建替换。
func (s *AdminService) RevokeAPIKey(ctx context.Context, orgID, id string) error {
	k, err := s.ApiKeys.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if k.OrganizationID != orgID {
		return apperrors.NewNotFound("api key not found")
	}
	if err := k.Revoke(); err != nil {
		return err
	}
	k.UpdatedAt = s.Clock.Now()
	return s.ApiKeys.Update(ctx, k)
}

// ---------------------------------------------------------------------------
// Route CRUD
// ---------------------------------------------------------------------------

func (s *AdminService) CreateRoute(ctx context.Context, req RouteCreate) (*RouteResponse, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternalf("generate route id: %v", err)
	}
	now := s.Clock.Now()
	rt := &domain.LlmRoute{
		ID:             id,
		OrganizationID: req.OrganizationID,
		AliasID:        req.AliasID,
		ModelID:        req.ModelID,
		Priority:       req.Priority,
		Weight:         req.Weight,
		Status:         req.Status,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	if err := s.Routes.Create(ctx, rt); err != nil {
		return nil, err
	}
	return toRouteResponse(ctx, rt, s.Aliases, s.Models, s.Providers)
}

func (s *AdminService) ListRoutes(ctx context.Context, orgID string) ([]RouteResponse, error) {
	routes, err := s.Routes.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]RouteResponse, 0, len(routes))
	for _, rt := range routes {
		resp, err := toRouteResponse(ctx, rt, s.Aliases, s.Models, s.Providers)
		if err != nil {
			return nil, err
		}
		out = append(out, *resp)
	}
	return out, nil
}

func (s *AdminService) GetRoute(ctx context.Context, orgID, id string) (*RouteResponse, error) {
	rt, err := s.Routes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rt.OrganizationID != orgID {
		return nil, apperrors.NewNotFound("route not found")
	}
	return toRouteResponse(ctx, rt, s.Aliases, s.Models, s.Providers)
}

func (s *AdminService) UpdateRoute(ctx context.Context, orgID, id string, req RouteUpdate) (*RouteResponse, error) {
	rt, err := s.Routes.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if rt.OrganizationID != orgID {
		return nil, apperrors.NewNotFound("route not found")
	}

	now := s.Clock.Now()
	if req.ModelID != nil {
		rt.ModelID = *req.ModelID
	}
	if req.Priority != nil {
		rt.Priority = *req.Priority
	}
	if req.Weight != nil {
		rt.Weight = *req.Weight
	}
	if req.Status != nil {
		rt.Status = *req.Status
	}
	rt.UpdatedAt = now

	if err := rt.Validate(); err != nil {
		return nil, err
	}
	if err := s.Routes.Update(ctx, rt); err != nil {
		return nil, err
	}
	return toRouteResponse(ctx, rt, s.Aliases, s.Models, s.Providers)
}

func (s *AdminService) DeleteRoute(ctx context.Context, orgID, id string) error {
	rt, err := s.Routes.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if rt.OrganizationID != orgID {
		return apperrors.NewNotFound("route not found")
	}
	return s.Routes.Delete(ctx, id)
}

// ---------------------------------------------------------------------------
// DTO ↔ Domain helpers
// ---------------------------------------------------------------------------

func toAPIKeyResponse(k *domain.LlmApiKey) *APIKeyResponse {
	resp := &APIKeyResponse{
		ID:             k.ID,
		Name:           k.Name,
		KeyPrefix:      k.KeyPrefix,
		AllowedAliases: k.AllowedAliases,
		RPMLimit:       k.RPMLimit,
		TPMLimit:       k.TPMLimit,
		MonthlyBudget:  k.MonthlyBudget,
		Currency:       k.Currency,
		Status:         k.Status,
		CreatedAt:      k.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      k.UpdatedAt.Format(time.RFC3339),
	}
	if k.ExpiresAt != nil {
		t := k.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &t
	}
	return resp
}

// toRouteResponse 把 domain LlmRoute 转成管理面 DTO。
// 附带查询 alias/model/provider 的展示名称 —— 查不到时留空（omitempty），
// 前端可 fallback 展示 ID。
func toRouteResponse(
	ctx context.Context,
	rt *domain.LlmRoute,
	aliases domain.AliasesRepo,
	models domain.ModelsRepo,
	providers domain.ProvidersRepo,
) (*RouteResponse, error) {
	resp := &RouteResponse{
		ID:             rt.ID,
		OrganizationID: rt.OrganizationID,
		AliasID:        rt.AliasID,
		ModelID:        rt.ModelID,
		Priority:       rt.Priority,
		Weight:         rt.Weight,
		Status:         rt.Status,
		CreatedAt:      rt.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      rt.UpdatedAt.Format(time.RFC3339),
	}

	// Alias 名称
	if a, err := aliases.GetByID(ctx, rt.AliasID); err == nil {
		resp.AliasName = a.Alias
	}
	// Model + Provider 名称
	if m, err := models.GetByID(ctx, rt.ModelID); err == nil {
		resp.ModelName = m.ProviderModelName
		resp.ProviderID = m.ProviderID
		if p, err := providers.GetByID(ctx, m.ProviderID); err == nil {
			resp.ProviderName = p.Name
		}
	}
	return resp, nil
}
