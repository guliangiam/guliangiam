package bootstrap

import (
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
)

func TestAllRouteDefinitionsDeclarePermissionOrPublic(t *testing.T) {
	if err := validateRouteDefinitions(allRouteDefinitions()); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRouteDefinitionsRejectsMissingPermission(t *testing.T) {
	err := validateRouteDefinitions([]route.Definition{{Method: "GET", Pattern: "/api/v1/private", Permission: ""}})
	if err == nil {
		t.Fatal("expected missing permission to fail")
	}
}
