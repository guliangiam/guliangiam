package uuid

import (
	"testing"
)

// BenchmarkUUIDv7Generation benchmarks UUIDv7 generation.
// UUIDv7 is used for all primary keys; must be fast on hot paths.
func BenchmarkUUIDv7Generation(b *testing.B) {
	gen := V7Generator{}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := gen.NewString()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkUUIDv7Parallel benchmarks concurrent UUIDv7 generation (simulating concurrent inserts).
func BenchmarkUUIDv7Parallel(b *testing.B) {
	gen := V7Generator{}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := gen.NewString()
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
