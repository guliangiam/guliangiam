package aigateway

import (
	"context"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/aigateway"
	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	healthOwnOrg     = "org-own"
	healthForeignOrg = "org-foreign"
)

// tenantProviderRepo is a minimal ProvidersRepo for the health tenant tests.
// It implements the management-plane scoped contract so tests pin the same
// provider→organization predicate as the SQL repositories.
type tenantProviderRepo struct {
	items   map[string]*domain.LlmProvider
	lastOrg string
}

func (r *tenantProviderRepo) GetByID(_ context.Context, id string) (*domain.LlmProvider, error) {
	if p, ok := r.items[id]; ok {
		cp := *p
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("provider not found")
}
func (r *tenantProviderRepo) GetByIDForOrganization(_ context.Context, orgID, id string) (*domain.LlmProvider, error) {
	r.lastOrg = orgID
	p, ok := r.items[id]
	if !ok || p.OrganizationID != orgID {
		return nil, apperrors.NewNotFound("provider not found")
	}
	cp := *p
	return &cp, nil
}
func (r *tenantProviderRepo) List(_ context.Context, orgID string) ([]*domain.LlmProvider, error) {
	return nil, nil
}
func (r *tenantProviderRepo) Create(_ context.Context, p *domain.LlmProvider) error { return nil }
func (r *tenantProviderRepo) Update(_ context.Context, p *domain.LlmProvider) error { return nil }
func (r *tenantProviderRepo) UpdateForOrganization(_ context.Context, p *domain.LlmProvider, orgID string) error {
	cur, ok := r.items[p.ID]
	if !ok || cur.OrganizationID != orgID {
		return apperrors.NewNotFound("provider not found")
	}
	cp := *p
	r.items[p.ID] = &cp
	return nil
}
func (r *tenantProviderRepo) Delete(_ context.Context, id string) error { return nil }
func (r *tenantProviderRepo) DeleteForOrganization(_ context.Context, orgID, id string) error {
	p, ok := r.items[id]
	if !ok || p.OrganizationID != orgID {
		return apperrors.NewNotFound("provider not found")
	}
	delete(r.items, id)
	return nil
}
func (r *tenantProviderRepo) GetByName(_ context.Context, orgID, name string) (*domain.LlmProvider, error) {
	return nil, nil
}

// tenantHealthRepo records every read so tests can pin that a foreign provider
// never reaches the tenant-less health table.
type tenantHealthRepo struct {
	records       map[string]*domain.ProviderHealth
	providerOrg   map[string]string
	healthReads   int
	rawReads      int
	unscopedReads int
	listAllRead   int
	lastOrg       string
}

func (r *tenantHealthRepo) GetByID(_ context.Context, id string) (*domain.ProviderHealth, error) {
	r.healthReads++
	r.rawReads++
	if h, ok := r.records[id]; ok {
		cp := *h
		return &cp, nil
	}
	return nil, apperrors.NewNotFound("provider health not found")
}
func (r *tenantHealthRepo) GetByIDForOrganization(_ context.Context, orgID, id string) (*domain.ProviderHealth, error) {
	r.healthReads++
	r.lastOrg = orgID
	h, ok := r.records[id]
	if !ok || r.providerOrg[h.ProviderID] != orgID {
		return nil, apperrors.NewNotFound("provider health not found")
	}
	cp := *h
	return &cp, nil
}
func (r *tenantHealthRepo) GetByProvider(_ context.Context, providerID string) (*domain.ProviderHealth, error) {
	r.healthReads++
	r.unscopedReads++
	for _, h := range r.records {
		if h.ProviderID == providerID {
			cp := *h
			return &cp, nil
		}
	}
	return nil, apperrors.NewNotFound("provider health not found")
}
func (r *tenantHealthRepo) ListAll(_ context.Context) ([]*domain.ProviderHealth, error) {
	r.listAllRead++
	var out []*domain.ProviderHealth
	for _, h := range r.records {
		cp := *h
		out = append(out, &cp)
	}
	return out, nil
}
func (r *tenantHealthRepo) ListByOrganization(_ context.Context, orgID string) ([]*domain.ProviderHealth, error) {
	r.lastOrg = orgID
	var out []*domain.ProviderHealth
	for _, h := range r.records {
		if r.providerOrg[h.ProviderID] != orgID {
			continue
		}
		cp := *h
		out = append(out, &cp)
	}
	return out, nil
}
func (r *tenantHealthRepo) Upsert(_ context.Context, h *domain.ProviderHealth) error { return nil }
func (r *tenantHealthRepo) ListOpen(_ context.Context) ([]*domain.ProviderHealth, error) {
	return nil, nil
}

func newHealthTenantFixture() (*AdminService, *tenantProviderRepo, *tenantHealthRepo) {
	providers := &tenantProviderRepo{items: map[string]*domain.LlmProvider{
		"p-own":     {ID: "p-own", OrganizationID: healthOwnOrg},
		"p-foreign": {ID: "p-foreign", OrganizationID: healthForeignOrg},
	}}
	health := &tenantHealthRepo{
		records: map[string]*domain.ProviderHealth{
			"h-own":     {ID: "h-own", ProviderID: "p-own", State: domain.HealthStateHealthy},
			"h-foreign": {ID: "h-foreign", ProviderID: "p-foreign", State: domain.HealthStateOpen},
		},
		providerOrg: map[string]string{
			"p-own":     healthOwnOrg,
			"p-foreign": healthForeignOrg,
		},
	}
	return &AdminService{Providers: providers, Health: health}, providers, health
}

// TestGetHealthForOrganization_ForeignProviderUsesOnlyScopedReads pins the
// ordering: provider ownership is decided by an organization-scoped read, then
// the record-address fallback is also scoped. A foreign id never reaches the
// raw tenant-less lookups and is indistinguishable from a missing one.
func TestGetHealthForOrganization_ForeignProviderUsesOnlyScopedReads(t *testing.T) {
	t.Parallel()
	svc, providers, health := newHealthTenantFixture()

	resp, err := svc.GetHealthForOrganization(context.Background(), healthOwnOrg, "p-foreign")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	assert.Equal(t, healthOwnOrg, providers.lastOrg, "provider lookup must carry the trusted org")
	assert.Equal(t, healthOwnOrg, health.lastOrg, "record fallback must carry the trusted org")
	assert.Zero(t, health.rawReads, "the raw GetByID must not serve management reads")
	assert.Zero(t, health.unscopedReads, "GetByProvider must not run for an unowned provider")
}

// TestGetHealthForOrganization_MissingProviderIsNotFound pins that an unknown
// id answers 404 through the org-scoped record lookup.
func TestGetHealthForOrganization_MissingProviderIsNotFound(t *testing.T) {
	t.Parallel()
	svc, _, health := newHealthTenantFixture()

	resp, err := svc.GetHealthForOrganization(context.Background(), healthOwnOrg, "p-missing")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	assert.Equal(t, healthOwnOrg, health.lastOrg, "fallback lookup must carry the trusted org")
}

// TestGetHealthForOrganization_OwnProviderReturnsHealth pins the console path:
// the id is a provider id, resolved inside the caller's tenant.
func TestGetHealthForOrganization_OwnProviderReturnsHealth(t *testing.T) {
	t.Parallel()
	svc, _, _ := newHealthTenantFixture()

	resp, err := svc.GetHealthForOrganization(context.Background(), healthOwnOrg, "p-own")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "h-own", resp.ID)
	assert.Equal(t, "p-own", resp.ProviderID)
	assert.Equal(t, domain.HealthStateHealthy, resp.State)
}

// TestGetHealthForOrganization_OwnHealthRecordIDIsReadable pins the legacy
// address used by the list payload's id field: it stays readable inside the
// tenant through the provider-scoped predicate.
func TestGetHealthForOrganization_OwnHealthRecordIDIsReadable(t *testing.T) {
	t.Parallel()
	svc, _, health := newHealthTenantFixture()

	resp, err := svc.GetHealthForOrganization(context.Background(), healthOwnOrg, "h-own")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "h-own", resp.ID)
	assert.Equal(t, healthOwnOrg, health.lastOrg)
}

// TestGetHealthForOrganization_ForeignHealthRecordIDIsNotFound pins the IDOR
// regression on the record address.
func TestGetHealthForOrganization_ForeignHealthRecordIDIsNotFound(t *testing.T) {
	t.Parallel()
	svc, _, health := newHealthTenantFixture()

	resp, err := svc.GetHealthForOrganization(context.Background(), healthOwnOrg, "h-foreign")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Equal(t, apperrors.CodeNotFound, apperrors.FromError(err).Code)
	assert.Equal(t, 1, health.healthReads, "only the org-scoped record lookup may run")
}

// TestListHealth_ScopesToOrganization pins that the list no longer reads the
// tenant-less table wholesale.
func TestListHealth_ScopesToOrganization(t *testing.T) {
	t.Parallel()
	svc, _, health := newHealthTenantFixture()

	out, err := svc.ListHealth(context.Background(), healthOwnOrg)
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "h-own", out[0].ID)
	assert.Equal(t, healthOwnOrg, health.lastOrg)
	assert.Zero(t, health.listAllRead, "the unscoped ListAll must not serve the management list")
}
