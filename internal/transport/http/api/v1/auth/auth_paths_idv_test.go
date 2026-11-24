package auth

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIDVCallbackPublicWhitelistIsExact guards the callback exception against
// drifting back into a prefix wildcard. The callback is the only anonymous
// entry in /api/v1/idv/*; neighbouring verification resources are protected.
func TestIDVCallbackPublicWhitelistIsExact(t *testing.T) {
	for _, tc := range []struct {
		name string
		path string
		want bool
	}{
		{"jumio callback", "/api/v1/idv/callback/jumio", true},
		{"aliyun callback", "/api/v1/idv/callback/aliyun_idv", true},
		{"route template", "/api/v1/idv/callback/{provider}", true},

		{"trailing slash only", "/api/v1/idv/callback/", false},
		{"missing provider", "/api/v1/idv/callback", false},
		{"extra subpath", "/api/v1/idv/callback/jumio/extra", false},
		{"nested subpath", "/api/v1/idv/callback/jumio/scans/1", false},

		{"protected verifications collection", "/api/v1/idv/verifications", false},
		{"protected verifications detail", "/api/v1/idv/verifications/abc", false},
		{"protected verifications template", "/api/v1/idv/verifications/{id}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, IsTrulyPublicEndpoint(tc.path), "path=%s", tc.path)
		})
	}
}

// TestIDVCallbackPublicWhitelistRuntimeGate checks the runtime decision (method
// + path) rather than the GET-only façade: the callback is a POST, and the
// protected verification read is a GET that must still fail closed.
func TestIDVCallbackPublicWhitelistRuntimeGate(t *testing.T) {
	assert.True(t, isTrulyPublicRequest(http.MethodPost, "/api/v1/idv/callback/jumio"))
	assert.False(t, isTrulyPublicRequest(http.MethodGet, "/api/v1/idv/verifications/abc"))
	assert.False(t, isTrulyPublicRequest(http.MethodPost, "/api/v1/idv/verifications/abc"))
}
