package errors

import (
	"testing"
)

// BenchmarkErrorCreation benchmarks structured error creation.
// Error creation is on every request path; must be sub-microsecond.
func BenchmarkErrorCreation(b *testing.B) {
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = New(CodeInvalidArgument, "invalid request parameter")
	}
}

// BenchmarkErrorWrapf benchmarks error wrapping with format.
func BenchmarkErrorWrapf(b *testing.B) {
	inner := New(CodeNotFound, "user not found")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Wrapf(CodeInternal, inner, "failed to get user %s", "admin")
	}
}

// BenchmarkErrorCodeHTTPStatus benchmarks error code to HTTP status mapping.
func BenchmarkErrorCodeHTTPStatus(b *testing.B) {
	codes := []Code{
		CodeNotFound,
		CodeInvalidArgument,
		CodeUnauthorized,
		CodeForbidden,
		CodeConflict,
		CodeTooManyRequests,
		CodeInternal,
		CodeUnavailable,
	}

	// Pre-create errors to benchmark just the HTTPStatus call
	errs := make([]*Error, len(codes))
	for i, c := range codes {
		errs[i] = New(c, "test error")
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = errs[i%len(errs)].HTTPStatus()
	}
}
