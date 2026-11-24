package bootstrap

import (
	"fmt"
	"strings"

	auth "github.com/guliangiam/guliangiam/internal/transport/http/api/v1/auth"
	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

// validateRouteDefinitions enforces AC-PERM-003 / AC-PERM-004 at startup:
// every declared route must name its method/path, and every non-public route
// must declare a permission. Empty-permission routes are allowed only when the
// same method+path is on the exact public whitelist.
func validateRouteDefinitions(definitions []route.Definition) error {
	seen := make(map[string]struct{}, len(definitions))
	for _, definition := range definitions {
		if definition.Method == "" || definition.Pattern == "" {
			return fmt.Errorf("route definition has empty method or pattern: %+v", definition)
		}
		key := definition.Method + " " + definition.Pattern
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate route definition: %s", key)
		}
		seen[key] = struct{}{}
		if strings.TrimSpace(definition.Permission) == "" && !auth.IsPublicRequest(definition.Method, definition.Pattern) {
			return fmt.Errorf("route %s declares no permission and is not on the public whitelist", key)
		}
	}
	return nil
}

func validateStartupRoutes() {
	if err := validateRouteDefinitions(allRouteDefinitions()); err != nil {
		panic("startup route contract violation: " + err.Error())
	}
}
