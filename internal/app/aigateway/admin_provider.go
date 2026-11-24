// admin_provider.go — Provider 聚合 CRUD 及 DTO↔Domain 转换。
package aigateway

import (
	"context"
	"errors"
	"time"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	platformssrf "github.com/guliangiam/guliangiam/internal/platform/ssrf"
)

// ---------------------------------------------------------------------------
// Provider CRUD
// ---------------------------------------------------------------------------

func (s *AdminService) CreateProvider(ctx context.Context, req ProviderCreate) (*ProviderResponse, error) {
	id, err := s.UUID.NewString()
	if err != nil {
		return nil, apperrors.NewInternalf("generate provider id: %v", err)
	}

	sealed, err := s.encryptAPIKey(req.APIKey)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	p := &domain.LlmProvider{
		ID:                 id,
		OrganizationID:     req.OrganizationID,
		Name:               req.Name,
		DisplayName:        req.DisplayName,
		ProviderType:       req.ProviderType,
		BaseURL:            req.BaseURL,
		APIKeySealed:       sealed,
		APIKeyFingerprint:  fingerprintAPIKey(req.APIKey),
		Status:             req.Status,
		SupportedProtocols: req.SupportedProtocols,
		PreferredProtocol:  req.PreferredProtocol,
		RequestTimeoutMs:   req.RequestTimeoutMs,
		MaxRetries:         req.MaxRetries,
		Priority:           req.Priority,
		Weight:             req.Weight,
		RPMLimit:           req.RPMLimit,
		TPMLimit:           req.TPMLimit,
		CreatedAt:          now,
		UpdatedAt:          now,
		Revision:           1,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := s.Providers.Create(ctx, p); err != nil {
		return nil, err
	}
	return toProviderResponse(p), nil
}

func (s *AdminService) ListProviders(ctx context.Context, orgID string) ([]ProviderResponse, error) {
	providers, err := s.Providers.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := make([]ProviderResponse, 0, len(providers))
	for _, p := range providers {
		out = append(out, *toProviderResponse(p))
	}
	return out, nil
}

// scopedProviders resolves the management-plane provider contract. The
// runtime repository satisfies ProvidersRepo only; management routes must
// fail closed rather than fall back to an unscoped read.
func (s *AdminService) scopedProviders() (domain.TenantScopedProvidersRepo, error) {
	repo, ok := s.Providers.(domain.TenantScopedProvidersRepo)
	if !ok {
		return nil, apperrors.NewInternal("provider repository does not implement tenant scoping")
	}
	return repo, nil
}

func (s *AdminService) GetProvider(ctx context.Context, orgID, id string) (*ProviderResponse, error) {
	repo, err := s.scopedProviders()
	if err != nil {
		return nil, err
	}
	p, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return toProviderResponse(p), nil
}

func (s *AdminService) UpdateProvider(ctx context.Context, orgID, id string, req ProviderUpdate) (*ProviderResponse, error) {
	repo, err := s.scopedProviders()
	if err != nil {
		return nil, err
	}
	p, err := repo.GetByIDForOrganization(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	now := s.Clock.Now()
	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.DisplayName != nil {
		p.DisplayName = *req.DisplayName
	}
	if req.ProviderType != nil {
		p.ProviderType = *req.ProviderType
	}
	if req.BaseURL != nil {
		p.BaseURL = *req.BaseURL
	}
	if req.APIKey != nil && *req.APIKey != "" {
		sealed, err := s.encryptAPIKey(*req.APIKey)
		if err != nil {
			return nil, err
		}
		p.APIKeySealed = sealed
		p.APIKeyFingerprint = fingerprintAPIKey(*req.APIKey)
	}
	if req.Status != nil {
		if err := p.TransitionTo(*req.Status); err != nil {
			return nil, err
		}
	}
	if req.SupportedProtocols != nil {
		p.SupportedProtocols = req.SupportedProtocols
	}
	if req.PreferredProtocol != nil {
		p.PreferredProtocol = *req.PreferredProtocol
	}
	if req.RequestTimeoutMs != nil {
		p.RequestTimeoutMs = *req.RequestTimeoutMs
	}
	if req.MaxRetries != nil {
		p.MaxRetries = *req.MaxRetries
	}
	if req.Priority != nil {
		p.Priority = *req.Priority
	}
	if req.Weight != nil {
		p.Weight = *req.Weight
	}
	if req.RPMLimit != nil {
		p.RPMLimit = req.RPMLimit
	}
	if req.TPMLimit != nil {
		p.TPMLimit = req.TPMLimit
	}
	p.UpdatedAt = now

	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := repo.UpdateForOrganization(ctx, p, orgID); err != nil {
		return nil, err
	}
	return toProviderResponse(p), nil
}

func (s *AdminService) DeleteProvider(ctx context.Context, orgID, id string) error {
	repo, err := s.scopedProviders()
	if err != nil {
		return err
	}
	// The scoped soft delete carries the tenant predicate itself, so no
	// read-then-write window exists between ownership check and deletion.
	return repo.DeleteForOrganization(ctx, orgID, id)
}

// TestProvider 通过统一 AdapterFactory 构建的上游适配器发起一次只读连接
// 探测（列出模型）。构建阶段必须经过 provider 凭据解密与协议归一化，因此
// 探测结论与实际调用一致：未配置适配器、凭据无法解密、上游鉴权失败都会
// 显式返回，而不是误报可达。
func (s *AdminService) TestProvider(ctx context.Context, orgID, providerID string) (*ProviderTestResult, error) {
	repo, err := s.scopedProviders()
	if err != nil {
		return nil, err
	}
	p, err := repo.GetByIDForOrganization(ctx, orgID, providerID)
	if err != nil {
		return nil, err
	}
	if s.Upstreams == nil {
		return nil, apperrors.NewUnavailable("upstream adapter factory not configured")
	}
	// 与数据平面一致先做 base URL 基础校验；IP 级 SSRF 防护由适配器
	// 自带的安全 client 在 dial 阶段执行。
	if err := platformssrf.ValidateURL(p.BaseURL); err != nil {
		return nil, apperrors.NewInvalidArgumentf("invalid baseUrl: %v", err)
	}

	start := s.Clock.Now()
	upstream, err := s.Upstreams.BuildAdapter(p, p.PreferredProtocol)
	if err != nil {
		if errors.Is(err, domain.ErrUnsupportedProtocol) {
			return nil, apperrors.NewUnavailablef(
				"no upstream adapter for provider type %q (protocol %q)",
				p.ProviderType, p.PreferredProtocol,
			)
		}
		// 解密失败等已由 BuildAdapter 包装为带 CodeInternal 的结构化错误；
		// 其消息与 Cause 都不包含密文或明文凭据。
		return nil, err
	}
	tester, ok := upstream.(domain.ConnectionTester)
	if !ok {
		return nil, apperrors.NewUnavailablef(
			"upstream adapter %q does not support connection probes", upstream.Name(),
		)
	}

	status, probeErr := tester.Probe(ctx)
	latencyMs := s.Clock.Now().Sub(start).Milliseconds()

	result := &ProviderTestResult{
		ProviderID: providerID,
		LatencyMs:  latencyMs,
	}
	if status > 0 {
		sc := status
		result.StatusCode = &sc
	}
	if probeErr != nil {
		msg := probeErr.Error()
		result.Error = &msg
		result.Reachable = false
		return result, nil
	}
	result.Reachable = true
	return result, nil
}

// ---------------------------------------------------------------------------
// DTO ↔ Domain helper
// ---------------------------------------------------------------------------

func toProviderResponse(p *domain.LlmProvider) *ProviderResponse {
	return &ProviderResponse{
		ID:                 p.ID,
		OrganizationID:     p.OrganizationID,
		Name:               p.Name,
		DisplayName:        p.DisplayName,
		ProviderType:       p.ProviderType,
		BaseURL:            p.BaseURL,
		Status:             p.Status,
		SupportedProtocols: p.SupportedProtocols,
		PreferredProtocol:  p.PreferredProtocol,
		RequestTimeoutMs:   p.RequestTimeoutMs,
		MaxRetries:         p.MaxRetries,
		Priority:           p.Priority,
		Weight:             p.Weight,
		RPMLimit:           p.RPMLimit,
		TPMLimit:           p.TPMLimit,
		APIKeyFingerprint:  p.APIKeyFingerprint,
		CreatedAt:          p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          p.UpdatedAt.Format(time.RFC3339),
	}
}
