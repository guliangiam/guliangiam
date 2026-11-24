package permissions

import (
	"context"
	"testing"

	appuow "github.com/guliangiam/guliangiam/internal/app/uow"
	enforcerdomain "github.com/guliangiam/guliangiam/internal/domain/enforcers"
	modeldomain "github.com/guliangiam/guliangiam/internal/domain/models"
	permissiondomain "github.com/guliangiam/guliangiam/internal/domain/permissions"
	"github.com/stretchr/testify/assert"
)

const (
	testPolicyEnforcerID        = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	testPolicyForeignEnforcerID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	testPolicyMissingEnforcerID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	testPolicyModelID           = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	testPolicyForeignModelID    = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
)

// policyScopedUOW exposes the ports used by the permission policy surfaces.
type policyScopedUOW struct {
	appuow.UnitOfWork
	permissions permissiondomain.Repository
	enforcers   enforcerdomain.Repository
	models      modeldomain.Repository
}

func (u policyScopedUOW) Permissions() permissiondomain.Repository { return u.permissions }
func (u policyScopedUOW) Enforcers() enforcerdomain.Repository     { return u.enforcers }
func (u policyScopedUOW) Models() modeldomain.Repository           { return u.models }

// policyCountingCasbin records every policy read/write so a rejected request
// can be proven to have never reached the policy store.
type policyCountingCasbin struct {
	permissiondomain.CasbinPolicyManager
	reads  int
	writes int
}

func (m *policyCountingCasbin) GetPolicies(string, string) ([][]string, error) {
	m.reads++
	return [][]string{{"p", "alice", "data1", "read"}}, nil
}

func (m *policyCountingCasbin) RemoveFilteredPolicy(string, string, int, ...string) error {
	m.writes++
	return nil
}

func (m *policyCountingCasbin) AddPolicy(string, string, []string) error {
	m.writes++
	return nil
}

func (m *policyCountingCasbin) GetAllObjects(string, string) ([]string, error) {
	m.reads++
	return []string{"data1"}, nil
}

func (m *policyCountingCasbin) GetAllActions(string, string) ([]string, error) {
	m.reads++
	return []string{"read"}, nil
}

func (m *policyCountingCasbin) GetAllRoles(string, string) ([]string, error) {
	m.reads++
	return []string{"admin"}, nil
}

type policyTestHarness struct {
	service   Service
	casbin    *policyCountingCasbin
	enforcers *scopedEnforcerRepo
	models    *scopedModelRepo
}

func newPolicyTestHarness(resources []string) *policyTestHarness {
	enforcers := &scopedEnforcerRepo{
		owners:          map[string]string{testPolicyEnforcerID: "org-1"},
		modelByEnforcer: map[string]string{testPolicyEnforcerID: testPolicyModelID},
	}
	models := &scopedModelRepo{owners: map[string]string{testPolicyModelID: "org-1"}}
	casbin := &policyCountingCasbin{}
	permissionRepo := &mockPermissionRepo{permissions: []*permissiondomain.Permission{{
		ID: "perm-1", OrganizationID: "org-1", Resources: resources,
	}}}
	return &policyTestHarness{
		service: Service{
			UOW: policyScopedUOW{
				permissions: permissionRepo,
				enforcers:   enforcers,
				models:      models,
			},
			CasbinManager: casbin,
		},
		casbin:    casbin,
		enforcers: enforcers,
		models:    models,
	}
}

type policySurface struct {
	name string
	call func(context.Context, Service, string, string) error
}

var policySurfaces = []policySurface{
	{"get-policy", func(ctx context.Context, svc Service, org, id string) error {
		_, err := svc.GetPolicies(ctx, org, id)
		return err
	}},
	{"update-policy", func(ctx context.Context, svc Service, org, id string) error {
		return svc.UpdatePolicies(ctx, org, id, []PolicyItem{
			{PType: "p", Rules: []string{"alice", "data1", "read"}},
			{PType: "p", Rules: []string{"bob", "data2", "write"}},
		})
	}},
	{"objects", func(ctx context.Context, svc Service, org, id string) error {
		_, err := svc.GetAllObjects(ctx, org, id)
		return err
	}},
	{"actions", func(ctx context.Context, svc Service, org, id string) error {
		_, err := svc.GetAllActions(ctx, org, id)
		return err
	}},
	{"roles", func(ctx context.Context, svc Service, org, id string) error {
		_, err := svc.GetAllRoles(ctx, org, id)
		return err
	}},
}

func TestPolicySurfaces_EmptyOrganizationRejected(t *testing.T) {
	svc := Service{}
	for _, surface := range policySurfaces {
		t.Run(surface.name, func(t *testing.T) {
			err := surface.call(context.Background(), svc, " ", "perm-1")
			assertInvalidArgument(t, err)
		})
	}
}

func TestPolicySurfaces_ForeignTenantEnforcerNotFound(t *testing.T) {
	for _, surface := range policySurfaces {
		t.Run(surface.name, func(t *testing.T) {
			h := newPolicyTestHarness([]string{testPolicyForeignEnforcerID})
			h.enforcers.owners[testPolicyForeignEnforcerID] = "org-2"
			h.enforcers.modelByEnforcer[testPolicyForeignEnforcerID] = testPolicyForeignModelID
			h.models.owners[testPolicyForeignModelID] = "org-2"

			err := surface.call(context.Background(), h.service, "org-1", "perm-1")
			assertNotFound(t, err)
			assert.Equal(t, []string{"org-1"}, h.enforcers.scopedOrgArgs,
				"ownership must be resolved under the trusted tenant")
			assert.Zero(t, h.enforcers.unscopedCalls, "the unscoped enforcer lookup must not be used")
			assert.Empty(t, h.models.scopedOrgArgs, "a foreign enforcer must not resolve a model")
			assert.Zero(t, h.casbin.reads, "no policy read for a foreign enforcer")
			assert.Zero(t, h.casbin.writes, "no policy write for a foreign enforcer")
		})
	}
}

func TestPolicySurfaces_MissingEnforcerNotFound(t *testing.T) {
	for _, surface := range policySurfaces {
		t.Run(surface.name, func(t *testing.T) {
			h := newPolicyTestHarness([]string{testPolicyMissingEnforcerID})

			err := surface.call(context.Background(), h.service, "org-1", "perm-1")
			assertNotFound(t, err)
			assert.Equal(t, []string{"org-1"}, h.enforcers.scopedOrgArgs)
			assert.Empty(t, h.models.scopedOrgArgs)
			assert.Zero(t, h.casbin.reads)
			assert.Zero(t, h.casbin.writes)
		})
	}
}

func TestPolicySurfaces_NonEnforcerResourceNotFound(t *testing.T) {
	// Built-in permission seeds use resources=["*"]; such a value cannot be an
	// enforcer PK (UUID), so it must fail closed without SQL or Casbin access.
	for _, surface := range policySurfaces {
		t.Run(surface.name, func(t *testing.T) {
			h := newPolicyTestHarness([]string{"*"})

			err := surface.call(context.Background(), h.service, "org-1", "perm-1")
			assertNotFound(t, err)
			assert.Empty(t, h.enforcers.scopedOrgArgs, "a non-enforcer value must not reach the repository")
			assert.Empty(t, h.models.scopedOrgArgs)
			assert.Zero(t, h.casbin.reads)
			assert.Zero(t, h.casbin.writes)
		})
	}
}

func TestPolicySurfaces_ForeignTenantModelNotFound(t *testing.T) {
	for _, surface := range []policySurface{policySurfaces[0], policySurfaces[1]} {
		t.Run(surface.name, func(t *testing.T) {
			h := newPolicyTestHarness([]string{testPolicyEnforcerID})
			h.models.owners[testPolicyModelID] = "org-2"

			err := surface.call(context.Background(), h.service, "org-1", "perm-1")
			assertNotFound(t, err)
			assert.Equal(t, []string{"org-1"}, h.models.scopedOrgArgs,
				"the model must be resolved under the trusted tenant")
			assert.Zero(t, h.casbin.reads)
			assert.Zero(t, h.casbin.writes)
		})
	}
}

func TestPolicySurfaces_OwnTenantSucceeds(t *testing.T) {
	for _, surface := range policySurfaces {
		t.Run(surface.name, func(t *testing.T) {
			h := newPolicyTestHarness([]string{testPolicyEnforcerID})

			err := surface.call(context.Background(), h.service, "org-1", "perm-1")
			assert.NoError(t, err)
			assert.Equal(t, []string{"org-1"}, h.enforcers.scopedOrgArgs)
			assert.Equal(t, []string{"org-1"}, h.models.scopedOrgArgs)
			assert.Zero(t, h.enforcers.unscopedCalls)
			assert.Zero(t, h.models.unscopedCalls)
			if surface.name == "update-policy" {
				assert.Equal(t, 3, h.casbin.writes, "one clear plus two batch rules")
				assert.Zero(t, h.casbin.reads)
			} else {
				assert.Equal(t, 1, h.casbin.reads)
				assert.Zero(t, h.casbin.writes)
			}
		})
	}
}
