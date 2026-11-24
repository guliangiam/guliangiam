package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewTTLCache(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	assert.NotNil(t, c)
}

func TestTTLCache_SetAndGet(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	c.Set("key1", "value1")

	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value1", val)
}

func TestTTLCache_Get_Missing(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	_, ok := c.Get("nonexistent")
	assert.False(t, ok)
}

func TestTTLCache_SetWithTTL_Expired(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	c.SetWithTTL("key1", "value1", -1*time.Second)

	_, ok := c.Get("key1")
	assert.False(t, ok)
}

func TestTTLCache_Delete(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	c.Set("key1", "value1")
	c.Delete("key1")

	_, ok := c.Get("key1")
	assert.False(t, ok)
}

func TestTTLCache_Invalidate(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	c.Set("org:1:user:1", "data1")
	c.Set("org:1:user:2", "data2")
	c.Set("org:2:user:1", "data3")

	c.Invalidate("org:1:")

	_, ok1 := c.Get("org:1:user:1")
	assert.False(t, ok1)
	_, ok2 := c.Get("org:1:user:2")
	assert.False(t, ok2)
	_, ok3 := c.Get("org:2:user:1")
	assert.True(t, ok3)
}

func TestTTLCache_Clear(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	c.Set("key1", "value1")
	c.Set("key2", "value2")

	c.Clear()

	_, ok1 := c.Get("key1")
	assert.False(t, ok1)
	_, ok2 := c.Get("key2")
	assert.False(t, ok2)
}

func TestTTLCache_Overwrite(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	c.Set("key1", "value1")
	c.Set("key1", "value2")

	val, ok := c.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "value2", val)
}

func TestTTLCache_IntValue(t *testing.T) {
	c := NewTTLCache(5 * time.Minute)
	c.Set("count", 42)

	val, ok := c.Get("count")
	assert.True(t, ok)
	assert.Equal(t, 42, val)
}
