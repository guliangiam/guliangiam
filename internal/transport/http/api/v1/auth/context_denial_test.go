package auth

import (
	"context"
	"net/http/httptest"
	"testing"

	platformaudit "github.com/guliangiam/guliangiam/internal/platform/auditdenial"
	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
)

type captureRecorder struct{ events []platformaudit.Event }

func (c *captureRecorder) Record(_ context.Context, event platformaudit.Event) {
	c.events = append(c.events, event)
}

func TestTrustedOrganizationIDRecordsForgedTarget(t *testing.T) {
	recorder := &captureRecorder{}
	request := httptest.NewRequest("GET", "/api/v1/users?organizationId=org-2", nil)
	ctx := platformaudit.WithRecorder(request.Context(), recorder)
	ctx = tenantctx.WithTenantScope(ctx, "org-1", "org-1", tenantctx.TenantFilterOptional)
	ctx = WithSessionContext(ctx, SessionContext{UserID: "user-1", OrganizationID: "org-1"})
	request = request.WithContext(ctx)

	org := TrustedOrganizationID(request, "org-2")
	assert.Equal(t, "org-1", org)
	if assert.Len(t, recorder.events, 1) {
		assert.Equal(t, "authz.cross_tenant_denied", recorder.events[0].Action)
		assert.Equal(t, "org-2", recorder.events[0].TargetTenant)
	}
}
