package casbin

import (
	"testing"
)

// BenchmarkEnforceCacheGet benchmarks Casbin enforce cache lookup.
// Spec §4.1: Casbin Enforce < 5ms → cache hit must be sub-microsecond.
func BenchmarkEnforceCacheGet(b *testing.B) {
	cache := NewEnforceCache(0)
	// Pre-populate cache with entries
	cache.Set("user:admin", "users", "list", true)
	cache.Set("user:admin", "users", "create", true)
	cache.Set("user:viewer", "users", "list", true)
	cache.Set("user:viewer", "users", "create", false)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cache.Get("user:admin", "users", "list")
	}
}

// BenchmarkEnforceCacheGetMiss benchmarks Casbin cache miss (cold lookup).
func BenchmarkEnforceCacheGetMiss(b *testing.B) {
	cache := NewEnforceCache(0)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cache.Get("user:nonexistent", "resource", "action")
	}
}

// BenchmarkEnforceCacheSet benchmarks Casbin cache write.
func BenchmarkEnforceCacheSet(b *testing.B) {
	cache := NewEnforceCache(0)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cache.Set("user:admin", "users", "list", true)
	}
}

// BenchmarkEnforceCacheConcurrent benchmarks concurrent cache reads (simulating 1000 concurrent requests).
func BenchmarkEnforceCacheConcurrent(b *testing.B) {
	cache := NewEnforceCache(0)
	cache.Set("user:admin", "users", "list", true)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			cache.Get("user:admin", "users", "list")
		}
	})
}

// BenchmarkEnforceCacheKeyGeneration benchmarks the HMAC-SHA256 cache key generation.
func BenchmarkEnforceCacheKeyGeneration(b *testing.B) {
	cache := NewEnforceCache(0)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cache.cacheKey("user:admin", "users:list", "allow")
	}
}
