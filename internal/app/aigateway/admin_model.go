// admin_model.go — Model / Alias 聚合 CRUD 及 DTO↔Domain 转换。
package aigateway

import (
	"context"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// ---------------------------------------------------------------------------
// Model CRUD
// ---------------------------------------------------------------------------

func (s *AdminService) CreateModel(ctx context.Context, req ModelCreate) (*ModelResponse, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternalf("generate model id: %v", err)
	}
	now := s.Clock.Now()
	m := &domain.LlmModel{
		ID:                id,
		OrganizationID:    req.OrganizationID,
		ProviderID:        req.ProviderID,
		ProviderModelName: req.ProviderModelName,
		DisplayName:       req.DisplayName,
		Capabilities:      req.Capabilities,
		MaxContextTokens:  req.MaxContextTokens,
		MaxOutputTokens:   req.MaxOutputTokens,
		Status:            req.Status,
		CreatedAt:         now,
		UpdatedAt:         now,
		Revision:          1,
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := s.Models.Create(ctx, m); err != nil {
		return nil, err
	}
	return toModelResponse(m), nil
}

func (s *AdminService) ListModels(ctx context.Context, orgID string) ([]ModelResponse, error) {
	models, err := s.Models.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]ModelResponse, 0, len(models))
	for _, m := range models {
		out = append(out, *toModelResponse(m))
	}
	return out, nil
}

// scopedModels resolves the management-plane model contract. The runtime
// repository satisfies ModelsRepo only; management routes must fail closed
// rather than fall back to an unscoped read.
func (s *AdminService) scopedModels() (domain.TenantScopedModelsRepo, error) {
	repo, ok := s.Models.(domain.TenantScopedModelsRepo)
	if !ok {
		return nil, apperrors.NewInternal("model repository does not implement tenant scoping")
	}
	return repo, nil
}

func (s *AdminService) GetModel(ctx context.Context, orgID, id string) (*ModelResponse, error) {
	repo, err := s.scopedModels()
	if err != nil {
		return nil, err
	}
	m, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return toModelResponse(m), nil
}

func (s *AdminService) UpdateModel(ctx context.Context, orgID, id string, req ModelUpdate) (*ModelResponse, error) {
	repo, err := s.scopedModels()
	if err != nil {
		return nil, err
	}
	m, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	if req.ProviderModelName != "" {
		m.ProviderModelName = req.ProviderModelName
	}
	if req.DisplayName != nil {
		m.DisplayName = *req.DisplayName
	}
	if req.Capabilities != nil {
		m.Capabilities = req.Capabilities
	}
	if req.MaxContextTokens != nil {
		m.MaxContextTokens = req.MaxContextTokens
	}
	if req.MaxOutputTokens != nil {
		m.MaxOutputTokens = req.MaxOutputTokens
	}
	if req.Status != nil {
		m.Status = *req.Status
	}
	m.UpdatedAt = now

	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := repo.UpdateForOrganization(ctx, m, orgID); err != nil {
		return nil, err
	}
	return toModelResponse(m), nil
}

func (s *AdminService) DeleteModel(ctx context.Context, orgID, id string) error {
	repo, err := s.scopedModels()
	if err != nil {
		return err
	}
	// The scoped soft delete carries the tenant predicate itself, so no
	// read-then-write window exists between ownership check and deletion.
	return repo.DeleteForOrganization(ctx, orgID, id)
}

// ---------------------------------------------------------------------------
// Alias CRUD
// ---------------------------------------------------------------------------

func (s *AdminService) CreateAlias(ctx context.Context, req AliasCreate) (*AliasResponse, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternalf("generate alias id: %v", err)
	}
	now := s.Clock.Now()
	a := &domain.LlmModelAlias{
		ID:             id,
		OrganizationID: req.OrganizationID,
		Alias:          req.Alias,
		DisplayName:    req.DisplayName,
		Status:         req.Status,
		CreatedAt:      now,
		UpdatedAt:      now,
		Revision:       1,
	}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	if err := s.Aliases.Create(ctx, a); err != nil {
		return nil, err
	}
	return toAliasResponse(a), nil
}

func (s *AdminService) ListAliases(ctx context.Context, orgID string) ([]AliasResponse, error) {
	aliases, err := s.Aliases.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]AliasResponse, 0, len(aliases))
	for _, a := range aliases {
		out = append(out, *toAliasResponse(a))
	}
	return out, nil
}

// scopedAliases resolves the management-plane alias contract. The runtime
// repository satisfies AliasesRepo only; management routes must fail closed
// rather than fall back to an unscoped read.
func (s *AdminService) scopedAliases() (domain.TenantScopedAliasesRepo, error) {
	repo, ok := s.Aliases.(domain.TenantScopedAliasesRepo)
	if !ok {
		return nil, apperrors.NewInternal("alias repository does not implement tenant scoping")
	}
	return repo, nil
}

func (s *AdminService) GetAlias(ctx context.Context, orgID, id string) (*AliasResponse, error) {
	repo, err := s.scopedAliases()
	if err != nil {
		return nil, err
	}
	a, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return toAliasResponse(a), nil
}

func (s *AdminService) UpdateAlias(ctx context.Context, orgID, id string, req AliasUpdate) (*AliasResponse, error) {
	repo, err := s.scopedAliases()
	if err != nil {
		return nil, err
	}
	a, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	if req.Alias != nil {
		a.Alias = *req.Alias
	}
	if req.DisplayName != nil {
		a.DisplayName = *req.DisplayName
	}
	if req.Status != nil {
		a.Status = *req.Status
	}
	a.UpdatedAt = now

	if err := a.Validate(); err != nil {
		return nil, err
	}
	if err := repo.UpdateForOrganization(ctx, a, orgID); err != nil {
		return nil, err
	}
	return toAliasResponse(a), nil
}

func (s *AdminService) DeleteAlias(ctx context.Context, orgID, id string) error {
	repo, err := s.scopedAliases()
	if err != nil {
		return err
	}
	// The scoped soft delete carries the tenant predicate itself, so no
	// read-then-write window exists between ownership check and deletion.
	return repo.DeleteForOrganization(ctx, orgID, id)
}

// ---------------------------------------------------------------------------
// DTO ↔ Domain helpers
// ---------------------------------------------------------------------------

func toModelResponse(m *domain.LlmModel) *ModelResponse {
	return &ModelResponse{
		ID:                m.ID,
		OrganizationID:    m.OrganizationID,
		ProviderID:        m.ProviderID,
		ProviderModelName: m.ProviderModelName,
		DisplayName:       m.DisplayName,
		Capabilities:      m.Capabilities,
		MaxContextTokens:  m.MaxContextTokens,
		MaxOutputTokens:   m.MaxOutputTokens,
		Status:            m.Status,
		CreatedAt:         m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         m.UpdatedAt.Format(time.RFC3339),
	}
}

func toAliasResponse(a *domain.LlmModelAlias) *AliasResponse {
	return &AliasResponse{
		ID:             a.ID,
		OrganizationID: a.OrganizationID,
		Alias:          a.Alias,
		DisplayName:    a.DisplayName,
		Status:         a.Status,
		CreatedAt:      a.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      a.UpdatedAt.Format(time.RFC3339),
	}
}
