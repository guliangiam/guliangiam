package certs

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/certs"
	"github.com/stretchr/testify/assert"
)

// trackedBody reports whether the request pipeline consumed the body, which is
// how the tenant fail-fast proves it precedes JSON decoding.
type trackedBody struct {
	reader io.Reader
	read   bool
}

func (b *trackedBody) Read(p []byte) (int, error) {
	b.read = true
	return b.reader.Read(p)
}

// An empty trusted tenant is a 400 raised before the body is read, and it must
// not reach the repository for any by-ID operation. The forged-tenant 404 path
// (valid tenant, foreign id) is covered by TestCertByID_ForeignTenantRejected.
func TestCertByID_WithoutTenantScopeRejectsBeforeBodyRead(t *testing.T) {
	cases := []struct {
		name, method, target, body string
	}{
		{"get", http.MethodGet, "/api/v1/certs/any-cert", ""},
		{"patch", http.MethodPatch, "/api/v1/certs/any-cert", `{"displayName":"hijacked","revision":1}`},
		{"delete", http.MethodDelete, "/api/v1/certs/any-cert", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &tenantByIDRepo{certs: []*domain.Cert{
				tenantCert("any-cert", forgedOrgSessionID, selfSignedPEM(t)),
			}}
			service, records := newCertScopeService(repo)

			var body io.Reader
			tracked := &trackedBody{}
			if tc.body != "" {
				tracked.reader = strings.NewReader(tc.body)
				body = tracked
			}
			req := httptest.NewRequest(tc.method, tc.target, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			recorder := serveCertRequest(t, certRouter(service), req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.False(t, tracked.read, "tenant verdict must precede body decoding")
			assert.Zero(t, repo.getCalls)
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
		})
	}
}
