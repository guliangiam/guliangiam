package aigateway

import (
	"context"
	"math/rand/v2"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

// DefaultRouterEngine is the reference implementation of domain.RouterEngine.
//
// Resolution flow:
//  1. Look up the model alias by name (org-scoped exact match; cross-org
//     ListAll scan only when orgID is empty).
//  2. Fetch all routes for that alias.
//  3. Resolve route → model → provider → health; filter open circuits.
//  4. Among survivors, pick highest priority; within same priority, weighted
//     random — simple weighted round-robin.
//
// Advanced features (per-request health probing, cooldown tracking,
// region/presence-aware routing) are future work and can be swapped in
// without touching GatewayService.
type DefaultRouterEngine struct {
	AliasesRepo   domain.AliasesRepo
	RoutesRepo    domain.RoutesRepo
	ProvidersRepo domain.ProvidersRepo
	ModelsRepo    domain.ModelsRepo
	HealthRepo    domain.ProviderHealthRepo
}

func NewDefaultRouterEngine(
	aliases domain.AliasesRepo,
	routes domain.RoutesRepo,
	providers domain.ProvidersRepo,
	models domain.ModelsRepo,
	health domain.ProviderHealthRepo,
) *DefaultRouterEngine {
	return &DefaultRouterEngine{
		AliasesRepo: aliases, RoutesRepo: routes,
		ProvidersRepo: providers, ModelsRepo: models, HealthRepo: health,
	}
}

type resolvedRoute struct {
	route    *domain.LlmRoute
	provider *domain.LlmProvider
	model    *domain.LlmModel
	health   *domain.ProviderHealth
}

func (e *DefaultRouterEngine) Resolve(ctx context.Context, orgID, alias string, req *domain.CanonicalRequest) (*domain.RouteResolution, error) {
	// 1. Alias lookup — orgID 非空时只允许本组织精确匹配，绝不跨组织 fallback；
	// orgID 为空（平台级调用）才允许 ListAll 扫描。
	var a *domain.LlmModelAlias
	if orgID != "" {
		found, err := e.AliasesRepo.GetByName(ctx, orgID, alias)
		if err != nil {
			if apperrors.FromError(err).Code != apperrors.CodeNotFound {
				return nil, apperrors.WrapInternalf(err, "lookup alias %s for organization %s", alias, orgID)
			}
			return nil, apperrors.NewUnavailablef("alias %s not found", alias)
		}
		a = found
	} else {
		all, err := e.AliasesRepo.ListAll(ctx)
		if err != nil {
			return nil, apperrors.WrapInternalf(err, "list aliases for alias %s", alias)
		}
		for _, oa := range all {
			if oa.Alias == alias {
				a = oa
				break
			}
		}
	}
	if a == nil {
		return nil, apperrors.NewUnavailablef("alias %s not found", alias)
	}

	// 2. Routes
	routes, err := e.RoutesRepo.ListByAlias(ctx, a.ID)
	if err != nil || len(routes) == 0 {
		return nil, apperrors.NewUnavailablef("no routes for alias %s", alias)
	}

	// 3. 解析 chain + 过滤熔断
	var cands []resolvedRoute
	for _, r := range routes {
		model, _ := e.ModelsRepo.GetByID(ctx, r.ModelID)
		if model == nil {
			continue
		}
		provider, _ := e.ProvidersRepo.GetByID(ctx, model.ProviderID)
		if provider == nil {
			continue
		}
		health, _ := e.HealthRepo.GetByProvider(ctx, provider.ID)
		if health != nil && health.State == domain.HealthStateOpen {
			continue
		}
		cands = append(cands, resolvedRoute{
			route: r, provider: provider, model: model, health: health,
		})
	}

	if len(cands) == 0 {
		return nil, apperrors.NewUnavailablef("all routes for alias %s are unhealthy", alias)
	}

	// 4. Priority + weighted random
	choice := pickWeighted(cands)

	return &domain.RouteResolution{
		Provider:     choice.provider,
		Model:        choice.model,
		Route:        choice.route,
		Health:       choice.health,
		ProtocolFace: inferProtocolFace(choice.provider.ProviderType),
	}, nil
}

func pickWeighted(cands []resolvedRoute) resolvedRoute {
	// Highest priority
	maxP := cands[0].route.Priority
	for _, c := range cands {
		if c.route.Priority > maxP {
			maxP = c.route.Priority
		}
	}

	var top []resolvedRoute
	for _, c := range cands {
		if c.route.Priority == maxP {
			top = append(top, c)
		}
	}

	if len(top) == 1 {
		return top[0]
	}

	// Weighted random
	var totalWeight int
	for _, c := range top {
		w := c.route.Weight
		if w <= 0 {
			w = 1
		}
		totalWeight += w
	}

	if totalWeight <= 0 {
		return top[rand.IntN(len(top))] //nolint:gosec // G404: weighted routing selection, not a security primitive
	}

	r := rand.IntN(totalWeight) //nolint:gosec // G404: weighted routing selection, not a security primitive
	var cum int
	for _, c := range top {
		w := c.route.Weight
		if w <= 0 {
			w = 1
		}
		cum += w
		if r < cum {
			return c
		}
	}
	return top[len(top)-1]
}

// inferProtocolFace maps providerType → protocolFace.
//
// Unknown types default to "openai" because OpenAI-compatible is the most
// common adapter surface in the industry.
func inferProtocolFace(providerType string) string {
	switch providerType {
	case "anthropic", "claude":
		return "anthropic"
	case "gemini", "google_generative_ai":
		return "gemini"
	case "ollama", "vllm", "local":
		return "openai"
	default:
		return "openai"
	}
}
