package aigateway

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Fake repos for RouterEngine tests ──────────────────────────────────

type fakeAliasesRepo struct {
	byOrgName    map[string]*domain.LlmModelAlias // key = orgID|aliasName
	all          []*domain.LlmModelAlias
	getByNameErr error
	listAllErr   error
}

func newFakeAliasesRepo() *fakeAliasesRepo {
	return &fakeAliasesRepo{
		byOrgName: make(map[string]*domain.LlmModelAlias),
	}
}
func (f *fakeAliasesRepo) AddOrgAlias(orgID, alias string, a *domain.LlmModelAlias) {
	f.byOrgName[orgID+"|"+alias] = a
	f.all = append(f.all, a)
}
func (f *fakeAliasesRepo) GetByID(ctx context.Context, id string) (*domain.LlmModelAlias, error) {
	return nil, nil
}
func (f *fakeAliasesRepo) GetByName(ctx context.Context, orgID, alias string) (*domain.LlmModelAlias, error) {
	if f.getByNameErr != nil {
		return nil, f.getByNameErr
	}
	if a, ok := f.byOrgName[orgID+"|"+alias]; ok {
		return a, nil
	}
	return nil, nil
}
func (f *fakeAliasesRepo) List(ctx context.Context, orgID string) ([]*domain.LlmModelAlias, error) {
	return nil, nil
}
func (f *fakeAliasesRepo) ListAll(ctx context.Context) ([]*domain.LlmModelAlias, error) {
	if f.listAllErr != nil {
		return nil, f.listAllErr
	}
	return f.all, nil
}
func (f *fakeAliasesRepo) Create(ctx context.Context, a *domain.LlmModelAlias) error { return nil }
func (f *fakeAliasesRepo) Update(ctx context.Context, a *domain.LlmModelAlias) error { return nil }
func (f *fakeAliasesRepo) Delete(ctx context.Context, id string) error               { return nil }

type fakeRoutesRepo struct {
	byAliasID map[string][]*domain.LlmRoute
}

func newFakeRoutesRepo() *fakeRoutesRepo {
	return &fakeRoutesRepo{byAliasID: make(map[string][]*domain.LlmRoute)}
}
func (f *fakeRoutesRepo) Add(aliasID string, r *domain.LlmRoute) {
	f.byAliasID[aliasID] = append(f.byAliasID[aliasID], r)
}
func (f *fakeRoutesRepo) GetByID(ctx context.Context, id string) (*domain.LlmRoute, error) {
	return nil, nil
}
func (f *fakeRoutesRepo) ListByAlias(ctx context.Context, aliasID string) ([]*domain.LlmRoute, error) {
	return f.byAliasID[aliasID], nil
}
func (f *fakeRoutesRepo) List(ctx context.Context, orgID string) ([]*domain.LlmRoute, error) {
	return nil, nil
}
func (f *fakeRoutesRepo) Create(ctx context.Context, r *domain.LlmRoute) error { return nil }
func (f *fakeRoutesRepo) Update(ctx context.Context, r *domain.LlmRoute) error { return nil }
func (f *fakeRoutesRepo) Delete(ctx context.Context, id string) error          { return nil }

type fakeProvidersRepo struct {
	byID map[string]*domain.LlmProvider
}

func newFakeProvidersRepo() *fakeProvidersRepo {
	return &fakeProvidersRepo{byID: make(map[string]*domain.LlmProvider)}
}
func (f *fakeProvidersRepo) Add(p *domain.LlmProvider) { f.byID[p.ID] = p }
func (f *fakeProvidersRepo) GetByID(ctx context.Context, id string) (*domain.LlmProvider, error) {
	return f.byID[id], nil
}
func (f *fakeProvidersRepo) List(ctx context.Context, orgID string) ([]*domain.LlmProvider, error) {
	return nil, nil
}
func (f *fakeProvidersRepo) Create(ctx context.Context, p *domain.LlmProvider) error { return nil }
func (f *fakeProvidersRepo) Update(ctx context.Context, p *domain.LlmProvider) error { return nil }
func (f *fakeProvidersRepo) Delete(ctx context.Context, id string) error             { return nil }
func (f *fakeProvidersRepo) GetByName(ctx context.Context, org, name string) (*domain.LlmProvider, error) {
	return nil, nil
}

type fakeModelsRepo struct {
	byID map[string]*domain.LlmModel
}

func newFakeModelsRepo() *fakeModelsRepo {
	return &fakeModelsRepo{byID: make(map[string]*domain.LlmModel)}
}
func (f *fakeModelsRepo) Add(m *domain.LlmModel) { f.byID[m.ID] = m }
func (f *fakeModelsRepo) GetByID(ctx context.Context, id string) (*domain.LlmModel, error) {
	return f.byID[id], nil
}
func (f *fakeModelsRepo) List(ctx context.Context, orgID string) ([]*domain.LlmModel, error) {
	return nil, nil
}
func (f *fakeModelsRepo) ListByProvider(ctx context.Context, pid string) ([]*domain.LlmModel, error) {
	return nil, nil
}
func (f *fakeModelsRepo) Create(ctx context.Context, m *domain.LlmModel) error { return nil }
func (f *fakeModelsRepo) Update(ctx context.Context, m *domain.LlmModel) error { return nil }
func (f *fakeModelsRepo) Delete(ctx context.Context, id string) error          { return nil }

type fakeHealthRepo struct {
	byProviderID map[string]*domain.ProviderHealth
}

func newFakeHealthRepo() *fakeHealthRepo {
	return &fakeHealthRepo{byProviderID: make(map[string]*domain.ProviderHealth)}
}
func (f *fakeHealthRepo) SetOpen(providerID string) {
	f.byProviderID[providerID] = &domain.ProviderHealth{ProviderID: providerID, State: domain.HealthStateOpen}
}
func (f *fakeHealthRepo) SetHealthy(providerID string) {
	f.byProviderID[providerID] = &domain.ProviderHealth{ProviderID: providerID, State: domain.HealthStateHealthy}
}
func (f *fakeHealthRepo) GetByID(ctx context.Context, id string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (f *fakeHealthRepo) GetByIDForOrganization(ctx context.Context, orgID, id string) (*domain.ProviderHealth, error) {
	return nil, nil
}
func (f *fakeHealthRepo) GetByProvider(ctx context.Context, providerID string) (*domain.ProviderHealth, error) {
	return f.byProviderID[providerID], nil
}
func (f *fakeHealthRepo) ListAll(ctx context.Context) ([]*domain.ProviderHealth, error) {
	return nil, nil
}
func (f *fakeHealthRepo) ListByOrganization(ctx context.Context, orgID string) ([]*domain.ProviderHealth, error) {
	return nil, nil
}
func (f *fakeHealthRepo) Upsert(ctx context.Context, h *domain.ProviderHealth) error { return nil }
func (f *fakeHealthRepo) ListOpen(ctx context.Context) ([]*domain.ProviderHealth, error) {
	return nil, nil
}

// ── Fixture helpers ────────────────────────────────────────────────────

// buildEngine builds a DefaultRouterEngine with all repos wired up and a
// pre-populated alias → route → model → provider chain. Providers default
// to healthy.
type engineFixture struct {
	engine    *DefaultRouterEngine
	aliases   *fakeAliasesRepo
	routes    *fakeRoutesRepo
	providers *fakeProvidersRepo
	models    *fakeModelsRepo
	health    *fakeHealthRepo
}

func newEngineFixture() *engineFixture {
	f := &engineFixture{
		aliases:   newFakeAliasesRepo(),
		routes:    newFakeRoutesRepo(),
		providers: newFakeProvidersRepo(),
		models:    newFakeModelsRepo(),
		health:    newFakeHealthRepo(),
	}
	f.engine = NewDefaultRouterEngine(f.aliases, f.routes, f.providers, f.models, f.health)
	return f
}

// populate adds a single alias → route → model → provider chain so Resolve
// has something to find.
func (f *engineFixture) populate() {
	alias := &domain.LlmModelAlias{ID: "a-1", OrganizationID: "org-1", Alias: "gpt-4o", Status: "active"}
	f.aliases.AddOrgAlias("org-1", "gpt-4o", alias)

	provider := &domain.LlmProvider{ID: "p-1", OrganizationID: "org-1", ProviderType: "openai", PreferredProtocol: "openai"}
	f.providers.Add(provider)

	model := &domain.LlmModel{ID: "m-1", ProviderID: "p-1", ProviderModelName: "gpt-4o-2024"}
	f.models.Add(model)

	f.routes.Add("a-1", &domain.LlmRoute{ID: "r-1", AliasID: "a-1", ModelID: "m-1", Priority: 100, Weight: 10})
}

// ── Tests ──────────────────────────────────────────────────────────────

func TestDefaultRouterEngine_Resolve_HappyPath(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()

	res, err := fx.engine.Resolve(context.Background(), "org-1", "gpt-4o", nil)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, "p-1", res.Provider.ID)
	assert.Equal(t, "m-1", res.Model.ID)
	assert.Equal(t, "openai", res.ProtocolFace)
}

func TestDefaultRouterEngine_Resolve_UnknownAlias(t *testing.T) {
	fx := newEngineFixture()

	_, err := fx.engine.Resolve(context.Background(), "org-1", "does-not-exist", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alias")
}

func TestDefaultRouterEngine_Resolve_NoRoutes(t *testing.T) {
	fx := newEngineFixture()
	// alias exists but no routes attached
	alias := &domain.LlmModelAlias{ID: "a-2", OrganizationID: "org-1", Alias: "orphan", Status: "active"}
	fx.aliases.AddOrgAlias("org-1", "orphan", alias)

	_, err := fx.engine.Resolve(context.Background(), "org-1", "orphan", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no routes")
}

func TestDefaultRouterEngine_Resolve_AllProvidersCircuitOpen(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()
	fx.health.SetOpen("p-1")

	_, err := fx.engine.Resolve(context.Background(), "org-1", "gpt-4o", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy")
}

func TestDefaultRouterEngine_Resolve_PriorityOrdering(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()

	// Add a second provider with LOWER priority → should NOT be selected
	provider2 := &domain.LlmProvider{ID: "p-2", OrganizationID: "org-1", ProviderType: "anthropic", PreferredProtocol: "anthropic"}
	fx.providers.Add(provider2)
	model2 := &domain.LlmModel{ID: "m-2", ProviderID: "p-2", ProviderModelName: "claude-sonnet"}
	fx.models.Add(model2)
	fx.routes.Add("a-1", &domain.LlmRoute{ID: "r-2", AliasID: "a-1", ModelID: "m-2", Priority: 50, Weight: 10})

	// Run many times — due to weighted-randomness we expect >99% of selections
	// to be the high-priority r-1 (p-1). Priority is strictly higher than weight.
	var p1Wins, p2Wins int
	for i := 0; i < 500; i++ {
		res, err := fx.engine.Resolve(context.Background(), "org-1", "gpt-4o", nil)
		require.NoError(t, err)
		if res.Provider.ID == "p-1" {
			p1Wins++
		} else {
			p2Wins++
		}
	}
	assert.Greater(t, p1Wins, 490, "high-priority route must dominate selections, got p1=%d p2=%d", p1Wins, p2Wins)
}

func TestDefaultRouterEngine_Resolve_AllHealthySamePriority_WeightedSelection(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()

	provider2 := &domain.LlmProvider{ID: "p-2", OrganizationID: "org-1", ProviderType: "anthropic"}
	fx.providers.Add(provider2)
	model2 := &domain.LlmModel{ID: "m-2", ProviderID: "p-2", ProviderModelName: "claude-sonnet"}
	fx.models.Add(model2)
	fx.routes.Add("a-1", &domain.LlmRoute{ID: "r-2", AliasID: "a-1", ModelID: "m-2", Priority: 100, Weight: 30})

	// Run enough times to verify weighted selection roughly respects the 10:30 ratio (1:3)
	var p1Wins, p2Wins int
	for i := 0; i < 400; i++ {
		res, err := fx.engine.Resolve(context.Background(), "org-1", "gpt-4o", nil)
		require.NoError(t, err)
		if res.Provider.ID == "p-1" {
			p1Wins++
		} else {
			p2Wins++
		}
	}
	// p1 weight=10, p2 weight=30 → p2 should win ~3× more often
	assert.Greater(t, p2Wins, p1Wins*2, "weighted selection: p2(30) should win much more than p1(10); p1=%d p2=%d", p1Wins, p2Wins)
}

func TestDefaultRouterEngine_Resolve_ModelMissing(t *testing.T) {
	fx := newEngineFixture()

	alias := &domain.LlmModelAlias{ID: "a-3", OrganizationID: "org-1", Alias: "ghost", Status: "active"}
	fx.aliases.AddOrgAlias("org-1", "ghost", alias)
	// route points to non-existent model
	fx.routes.Add("a-3", &domain.LlmRoute{ID: "r-ghost", AliasID: "a-3", ModelID: "m-deleted", Priority: 100, Weight: 10})

	_, err := fx.engine.Resolve(context.Background(), "org-1", "ghost", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unhealthy", "missing model → candidate dropped → 0 survivors")
}

func TestDefaultRouterEngine_Resolve_CrossOrgRejected(t *testing.T) {
	fx := newEngineFixture()
	// alias 只存在于另一个组织，且挂有完整可用链路
	alias := &domain.LlmModelAlias{ID: "a-99", OrganizationID: "org-other", Alias: "crossorg", Status: "active"}
	fx.aliases.AddOrgAlias("org-other", "crossorg", alias)
	provider := &domain.LlmProvider{ID: "p-99", OrganizationID: "org-other", ProviderType: "openai"}
	fx.providers.Add(provider)
	fx.models.Add(&domain.LlmModel{ID: "m-99", ProviderID: "p-99", ProviderModelName: "gpt-4o"})
	fx.routes.Add("a-99", &domain.LlmRoute{ID: "r-99", AliasID: "a-99", ModelID: "m-99", Priority: 100, Weight: 10})

	// orgID 非空 → 绝不跨组织 fallback，即使别名在别的组织完整可用
	_, err := fx.engine.Resolve(context.Background(), "org-1", "crossorg", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found",
		"org-scoped resolve must reject aliases owned by another organization")
}

func TestDefaultRouterEngine_Resolve_EmptyOrgFallback(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()

	// orgID 为空（平台级调用）→ 允许 ListAll 扫描命中
	res, err := fx.engine.Resolve(context.Background(), "", "gpt-4o", nil)
	require.NoError(t, err)
	assert.Equal(t, "p-1", res.Provider.ID)
}

func TestDefaultRouterEngine_Resolve_LookupErrorPropagated(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()
	fx.aliases.getByNameErr = apperrors.NewUnavailable("db down")

	_, err := fx.engine.Resolve(context.Background(), "org-1", "gpt-4o", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down", "non-notfound lookup errors must propagate, not be swallowed")
}

func TestDefaultRouterEngine_Resolve_ListAllErrorPropagated(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()
	fx.aliases.listAllErr = apperrors.NewUnavailable("db down")

	_, err := fx.engine.Resolve(context.Background(), "", "gpt-4o", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down", "ListAll errors must propagate, not be swallowed")
}

func TestDefaultRouterEngine_Resolve_OpenProviderFiltered(t *testing.T) {
	fx := newEngineFixture()
	fx.populate()

	// Add a healthy alternative with same priority
	provider2 := &domain.LlmProvider{ID: "p-2", OrganizationID: "org-1", ProviderType: "openai"}
	fx.providers.Add(provider2)
	model2 := &domain.LlmModel{ID: "m-2", ProviderID: "p-2", ProviderModelName: "gpt-4o-backup"}
	fx.models.Add(model2)
	fx.routes.Add("a-1", &domain.LlmRoute{ID: "r-2", AliasID: "a-1", ModelID: "m-2", Priority: 100, Weight: 10})

	// Open the primary provider → only p-2 should be eligible
	fx.health.SetOpen("p-1")
	fx.health.SetHealthy("p-2")

	// Run 100 times — should always pick p-2 (the only eligible candidate)
	for i := 0; i < 100; i++ {
		res, err := fx.engine.Resolve(context.Background(), "org-1", "gpt-4o", nil)
		require.NoError(t, err)
		assert.Equal(t, "p-2", res.Provider.ID, "open provider p-1 must be filtered out, iteration=%d", i)
	}
}

// ── inferProtocolFace helpers ───────────────────────────────────────────

func TestInferProtocolFace(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"openai", "openai"},
		{"deepseek", "openai"},
		{"ollama", "openai"},
		{"vllm", "openai"},
		{"anthropic", "anthropic"},
		{"claude", "anthropic"},
		{"gemini", "gemini"},
		{"google_generative_ai", "gemini"},
		{"totally-unknown", "openai"}, // default
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.expected, inferProtocolFace(tc.input))
		})
	}
}
