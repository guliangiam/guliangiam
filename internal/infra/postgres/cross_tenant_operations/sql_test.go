package crosstenantops

import (
	"strings"
	"testing"
)

func TestQueryListByScopeSelectsFromCrossTenantOperations(t *testing.T) {
	if !strings.Contains(queryListByScope, "FROM guliangiam.cross_tenant_operations") {
		t.Fatal("queryListByScope must select from cross_tenant_operations")
	}
}
