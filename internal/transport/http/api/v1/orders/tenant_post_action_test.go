package orders

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// trackedBody reports whether the request pipeline consumed the body, which is
// how the empty-tenant case proves the tenant verdict precedes JSON decoding.
type trackedBody struct {
	reader io.Reader
	read   bool
}

func (b *trackedBody) Read(p []byte) (int, error) {
	b.read = true
	return b.reader.Read(p)
}

var idorOrderPostActionRoutes = []struct {
	name, path, body string
}{
	{"cancel", "/api/v1/orders/" + idorOrderID + "/cancel", ""},
	{"pay", "/api/v1/orders/" + idorOrderID + "/pay", `{"paymentRef":"pay-ref"}`},
	{"notify-paid", "/api/v1/orders/" + idorOrderID + "/notify-paid", `{"paymentId":"pay-1"}`},
}

// A POST action on a foreign order must look missing (404): no scoped row read
// with the forged org, no write, and no audit record.
func TestOrderPostActions_ForeignTenantRejected(t *testing.T) {
	for _, tt := range idorOrderPostActionRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDOROrderHarness(tenantOrder(idorOrderForeignOrg))
			recorder := serveOrder(router, idorOrderRequest(http.MethodPost,
				tt.path+"?organizationId="+idorOrderForeignOrg, idorOrderBody(tt.body)))

			assert.Equal(t, http.StatusNotFound, recorder.Code)
			assert.NotContains(t, recorder.Body.String(), "pay.example.com")
			for _, org := range repo.scopedOrgs {
				assert.Equal(t, idorOrderSessionOrg, org, "forged organizationId must be ignored")
			}
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
			require.Len(t, repo.orders, 1)
			assert.Equal(t, idorOrderForeignOrg, repo.orders[0].OrganizationID,
				"the foreign order must not be mutated")
		})
	}
}

// An empty trusted tenant is a 400 raised before the body is read.
func TestOrderPostActions_WithoutTenantScopeRejected(t *testing.T) {
	for _, tt := range idorOrderPostActionRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDOROrderHarness(tenantOrder(idorOrderSessionOrg))
			var body io.Reader
			tracked := &trackedBody{}
			if tt.body != "" {
				tracked.reader = strings.NewReader(tt.body)
				body = tracked
			}
			req := httptest.NewRequest(http.MethodPost, tt.path, body)
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			recorder := serveOrder(router, req)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.False(t, tracked.read, "tenant verdict must precede body decoding")
			assert.Zero(t, repo.scopedCalls)
			assert.Zero(t, repo.updateCalls)
			assert.Zero(t, repo.deleteCalls)
			assert.Empty(t, records.created)
		})
	}
}

// The owning tenant still reaches every action.
func TestOrderPostActions_OwnTenantSucceeds(t *testing.T) {
	for _, tt := range idorOrderPostActionRoutes {
		t.Run(tt.name, func(t *testing.T) {
			repo, records, router := newIDOROrderHarness(tenantOrder(idorOrderSessionOrg))
			recorder := serveOrder(router, idorOrderRequest(http.MethodPost, tt.path, idorOrderBody(tt.body)))

			require.Equal(t, http.StatusOK, recorder.Code)
			for _, org := range repo.scopedOrgs {
				assert.Equal(t, idorOrderSessionOrg, org)
			}
			assert.Equal(t, 1, repo.updateCalls)
			assert.Empty(t, records.created)
			assert.Equal(t, idorOrderSessionOrg, repo.orders[0].OrganizationID)
		})
	}
}
