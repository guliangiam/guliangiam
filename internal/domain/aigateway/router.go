package aigateway

import "context"

// RouteResolution is the outcome of the router engine. Callers inspect it to
// pick a concrete upstream, check circuit state and log the decision.
type RouteResolution struct {
	Provider     *LlmProvider
	Model        *LlmModel
	Route        *LlmRoute
	Health       *ProviderHealth
	ProtocolFace string
	FailoverFrom *string // nil on first attempt
}

// RouterEngine is the Port that selects an actual provider route given an
// alias and the incoming request. Implementations combine alias lookup, health
// filtering, priority/weight selection and failover semantics.
type RouterEngine interface {
	// Resolve selects a concrete provider route for alias within orgID.
	// orgID 为空时跨组织扫描（慢路径，仅 fallback）。
	Resolve(ctx context.Context, orgID string, alias string, req *CanonicalRequest) (*RouteResolution, error)
}
