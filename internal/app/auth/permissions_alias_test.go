package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aiGatewayPageKeys pins the three layers used by the 7 AI Gateway pages:
//
//	runtime resource          ← resourceFromPath("/api/v1/ai/{entity}")
//	                            (see transport auth TestResourceFromPath)
//	→ frontend action prefix  ← guard action="aigateway-{entity}:write"
//	→ frontend resource slug  ← guard resource="aigateway-{entities}"
//
// adminPermissionSet must contain the same prefix actions and resource slug,
// otherwise administrators lose the guarded buttons (the frontend
// ADMIN_FULL_PERMISSIONS mirror is maintained in web/src/shared/hooks).
var aiGatewayPageKeys = []struct {
	name             string
	runtimeResource  string
	frontendPrefix   string
	frontendResource string
}{
	{"providers", "aigateway:providers", "aigateway-provider", "aigateway-providers"},
	{"models", "aigateway:models", "aigateway-model", "aigateway-models"},
	{"aliases", "aigateway:aliases", "aigateway-alias", "aigateway-aliases"},
	{"routes", "aigateway:routes", "aigateway-route", "aigateway-routes"},
	{"api_keys", "aigateway:api_keys", "aigateway-api-key", "aigateway-api-keys"},
	{"quotas", "aigateway:quotas", "aigateway-quota", "aigateway-quotas"},
	{"prices", "aigateway:prices", "aigateway-price", "aigateway-prices"},
}

func TestAIGatewayPageKeys_ThreeLayerMapping(t *testing.T) {
	admin := adminPermissionSet()
	require.NotNil(t, admin)

	for _, page := range aiGatewayPageKeys {
		t.Run(page.name, func(t *testing.T) {
			prefix, resource, ok := frontendResourceAlias(page.runtimeResource)
			require.True(t, ok, "runtime resource must be colon-scoped")
			assert.Equal(t, page.frontendPrefix, prefix)
			assert.Equal(t, page.frontendResource, resource)

			// The exact guard keys used by the 7 pages (B5 will also use
			// read/delete for row actions) must exist in the admin fallback.
			for _, action := range []string{"read", "write", "delete"} {
				key := page.frontendPrefix + ":" + action
				require.Contains(t, admin.Actions, key,
					"adminPermissionSet missing %s", key)
			}
			require.Contains(t, admin.Resources, page.frontendResource)
		})
	}
}

func TestFrontendPolicyKeys_KeepLegacyAndAddAlias(t *testing.T) {
	tests := []struct {
		obj           string
		action        string
		legacyKey     string
		aliasKey      string
		aliasResource string
	}{
		{"aigateway:providers", "get", "aigateway:providers:get", "aigateway-provider:read", "aigateway-providers"},
		{"aigateway:providers", "post", "aigateway:providers:post", "aigateway-provider:write", "aigateway-providers"},
		{"aigateway:providers", "patch", "aigateway:providers:patch", "aigateway-provider:write", "aigateway-providers"},
		{"aigateway:providers", "delete", "aigateway:providers:delete", "aigateway-provider:delete", "aigateway-providers"},
		{"aigateway:providers", "write", "aigateway:providers:write", "aigateway-provider:write", "aigateway-providers"},
		{"aigateway:api_keys", "post", "aigateway:api_keys:post", "aigateway-api-key:write", "aigateway-api-keys"},
		// A hyphenated policy object (the canonical map key) with an HTTP verb
		// keeps the old verb key and gains the CRUD alias.
		{"aigateway-providers", "get", "aigateway-provider:get", "aigateway-provider:read", "aigateway-providers"},
		// Non-namespaced resources follow the same additive verb normalization.
		{"users", "get", "users:get", "users:read", "users"},
	}
	for _, testCase := range tests {
		t.Run(testCase.obj+"/"+testCase.action, func(t *testing.T) {
			actions, resources := frontendPolicyKeys(testCase.obj, testCase.action)
			assert.Contains(t, actions, testCase.legacyKey)
			assert.Contains(t, actions, testCase.aliasKey)
			assert.Contains(t, resources, testCase.obj)
			assert.Contains(t, resources, testCase.aliasResource)
		})
	}
}

func TestResolveFromRealCasbin_AIGatewayKeepsLegacyKeysAndEmitsAliases(t *testing.T) {
	var policies [][]string
	for _, page := range aiGatewayPageKeys {
		for _, verb := range []string{"get", "post", "patch", "delete"} {
			policies = append(policies, []string{"aigateway-admin", "built-in", page.runtimeResource, verb})
		}
	}
	svc := &Service{CasbinPolicyManager: stubCasbinPolicies{policies: policies}}

	actions, resources := svc.resolveFromRealCasbin("org-1", []string{"aigateway-admin"})

	for _, page := range aiGatewayPageKeys {
		// Legacy keys keep their exact previous shape: an already-authorized
		// role does not need to be re-granted after the alias rollout.
		for _, verb := range []string{"get", "post", "patch", "delete"} {
			legacy := page.runtimeResource + ":" + verb
			require.Contains(t, actions, legacy, "legacy key %s must survive", legacy)
		}
		for _, crud := range []string{"read", "write", "delete"} {
			alias := page.frontendPrefix + ":" + crud
			require.Contains(t, actions, alias, "alias key %s missing", alias)
		}
		require.Contains(t, resources, page.runtimeResource)
		require.Contains(t, resources, page.frontendResource)
	}
}

func TestResolveFromRealCasbin_WildcardKeepsCanonicalKeys(t *testing.T) {
	tests := []struct {
		action        string
		legacyAliases []string
	}{
		{"write", []string{"aigateway-provider:write", "aigateway-model:write"}},
		{"get", []string{"aigateway-provider:get", "aigateway-provider:read", "aigateway-model:get", "aigateway-model:read"}},
	}
	for _, testCase := range tests {
		t.Run(testCase.action, func(t *testing.T) {
			svc := &Service{CasbinPolicyManager: stubCasbinPolicies{
				policies: [][]string{{"editor", "built-in", "*", testCase.action}},
			}}
			actions, resources := svc.resolveFromRealCasbin("org-1", []string{"editor"})

			for _, key := range testCase.legacyAliases {
				require.Contains(t, actions, key)
			}
			require.Contains(t, resources, "aigateway-providers")
			require.Contains(t, resources, "aigateway-models")
			// The wildcard expansion never introduces colon-scoped resources.
			require.NotContains(t, resources, "aigateway:providers")
			require.NotContains(t, resources, "aigateway:models")
		})
	}
}

func TestCRUDActionAlias(t *testing.T) {
	tests := []struct {
		action  string
		alias   string
		changed bool
	}{
		{"get", "read", true},
		{"HEAD", "read", true},
		{"post", "write", true},
		{"put", "write", true},
		{"patch", "write", true},
		{"delete", "delete", true},
		{"read", "read", false},
		{"write", "write", false},
		{"revoke", "revoke", false},
		{"", "", false},
	}
	for _, testCase := range tests {
		t.Run(testCase.action, func(t *testing.T) {
			alias, changed := crudActionAlias(testCase.action)
			assert.Equal(t, testCase.alias, alias)
			assert.Equal(t, testCase.changed, changed)
		})
	}
}
