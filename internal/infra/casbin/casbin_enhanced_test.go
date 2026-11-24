package casbin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestField_BoundaryConditions(t *testing.T) {
	assert.Equal(t, "a", field([]string{"a"}, 0))
	assert.Equal(t, "", field([]string{"a"}, 1))
	assert.Equal(t, "", field(nil, 0))
	assert.Equal(t, "", field(nil, 1))
}

func TestNonEmpty_EdgeCases(t *testing.T) {
	result := nonEmpty()
	assert.Empty(t, result)

	result = nonEmpty("a", "", "b", "", "c")
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestAdapter_WithEnforcerID_Chaining(t *testing.T) {
	a := Adapter{enforcerID: "first"}
	b := a.WithEnforcerID("second")
	c := b.WithEnforcerID("third")

	assert.Equal(t, "first", a.enforcerID)
	assert.Equal(t, "second", b.enforcerID)
	assert.Equal(t, "third", c.enforcerID)
}

func TestAdapter_WithEnforcerID_SamePool(t *testing.T) {
	a := Adapter{enforcerID: "orig"}
	b := a.WithEnforcerID("new")
	assert.Equal(t, a.pool, b.pool)
}

func TestNewAdapter_WithEnforcerID(t *testing.T) {
	a := NewAdapter(nil)
	b := a.WithEnforcerID("enf-1")
	assert.Equal(t, "enf-1", b.enforcerID)
	assert.Empty(t, a.enforcerID)
}

func TestManager_NewManager_NilAdapter(t *testing.T) {
	mgr := NewManager(Adapter{})
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.enforcers)
	assert.NotNil(t, mgr.orgEnforcers)
	assert.Empty(t, mgr.enforcers)
	assert.Empty(t, mgr.orgEnforcers)
}

func TestManager_Invalidate_MultipleSequential(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("a", "")] = nil
	mgr.enforcers[enforcerCacheKey("b", "")] = nil
	mgr.enforcers[enforcerCacheKey("c", "")] = nil

	mgr.Invalidate("a")
	assert.Len(t, mgr.enforcers, 2)
	mgr.Invalidate("b")
	assert.Len(t, mgr.enforcers, 1)
	mgr.Invalidate("c")
	assert.Empty(t, mgr.enforcers)
}

func TestManager_InvalidateOrg_MultipleSequential(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.orgEnforcers["org-a"] = nil
	mgr.orgEnforcers["org-b"] = nil

	mgr.InvalidateOrg("org-a")
	assert.Len(t, mgr.orgEnforcers, 1)
	mgr.InvalidateOrg("org-b")
	assert.Empty(t, mgr.orgEnforcers)
}

func TestManager_InvalidateAll_WithOrgEnforcers(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("enf-1", "")] = nil
	mgr.enforcers[enforcerCacheKey("enf-2", "")] = nil

	mgr.InvalidateAll()
	assert.Empty(t, mgr.enforcers)
}

func TestManager_InvalidateAll_Empty(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.InvalidateAll()
	assert.Empty(t, mgr.enforcers)
}

func TestManager_Invalidate_DoubleInvalidate(t *testing.T) {
	mgr := NewManager(Adapter{})
	mgr.enforcers[enforcerCacheKey("enf-1", "")] = nil

	mgr.Invalidate("enf-1")
	assert.Empty(t, mgr.enforcers)

	mgr.Invalidate("enf-1")
	assert.Empty(t, mgr.enforcers)
}

func TestAdapter_WithEnforcerID_Immutable(t *testing.T) {
	original := Adapter{enforcerID: "original"}
	copy1 := original.WithEnforcerID("copy1")
	copy2 := original.WithEnforcerID("copy2")

	assert.Equal(t, "original", original.enforcerID)
	assert.Equal(t, "copy1", copy1.enforcerID)
	assert.Equal(t, "copy2", copy2.enforcerID)
}
