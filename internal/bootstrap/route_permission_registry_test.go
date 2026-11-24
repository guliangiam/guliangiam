package bootstrap

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	applicationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/applications"
	groupstransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/groups"
	organizationtransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/organizations"
	providertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/providers"
	usertransport "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/users"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

type registryResource struct {
	Actions        []string `yaml:"actions"`
	CasbinResource string   `yaml:"casbin_resource"`
}

// TestDeleteRoutePermissionsAlignWithRegistry locks the contract between the
// route declarations and contracts/registries/permissions.yaml: every DELETE
// route in the aligned modules must declare the registry-registered "delete"
// action, which is the action the PEP derives from DELETE at runtime.
func TestDeleteRoutePermissionsAlignWithRegistry(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "contracts", "registries", "permissions.yaml"))
	require.NoError(t, err)

	var registry map[string]map[string]registryResource
	require.NoError(t, yaml.Unmarshal(data, &registry))

	registered := map[string]map[string]bool{}
	for _, resources := range registry {
		for key, entry := range resources {
			targets := []string{key}
			if entry.CasbinResource != "" {
				targets = append(targets, entry.CasbinResource)
			}
			for _, target := range targets {
				if registered[target] == nil {
					registered[target] = map[string]bool{}
				}
				for _, action := range entry.Actions {
					registered[target][action] = true
				}
			}
		}
	}

	suites := map[string][]route.Definition{
		"users":         usertransport.Routes(),
		"applications":  applicationtransport.Routes(),
		"organizations": organizationtransport.Routes(),
		"groups":        groupstransport.Routes(),
		"providers":     providertransport.Routes(),
	}

	for module, defs := range suites {
		for _, def := range defs {
			if def.Method != http.MethodDelete {
				continue
			}
			resource, action, ok := strings.Cut(def.Permission, ":")
			require.True(t, ok, "%s %s declares non-resource permission %q", module, def.Pattern, def.Permission)
			assert.Equal(t, "delete", action,
				"%s %s must declare the DELETE action derived by the PEP", module, def.Pattern)
			require.NotNil(t, registered[resource],
				"%s %s uses unregistered resource %q", module, def.Pattern, resource)
			assert.True(t, registered[resource][action],
				"%s %s declares %s:%s but permissions.yaml does not register that action",
				module, def.Pattern, resource, action)
		}
	}
}
