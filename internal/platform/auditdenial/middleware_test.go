package auditdenial

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type captureRecorder struct{ events []Event }

func (c *captureRecorder) Record(_ context.Context, event Event) { c.events = append(c.events, event) }

func TestAuthorizationDenialMiddlewareRecordsOnly403(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound, http.StatusUnauthorized} {
		recorder := &captureRecorder{}
		handler := WithRecorderMiddleware(recorder)(AuthorizationDenialMiddleware(func(*http.Request) (Event, bool) {
			return Event{OrganizationID: "org-1", ActorID: "actor-1"}, true
		})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		})))
		request := httptest.NewRequest(http.MethodGet, "/api/v1/users/foreign", nil)
		handler.ServeHTTP(httptest.NewRecorder(), request)
		if status == http.StatusForbidden {
			if assert.Len(t, recorder.events, 1) {
				assert.Equal(t, "authz.deny", recorder.events[0].Action)
				assert.Equal(t, "permission_denied", recorder.events[0].Reason)
			}
		} else {
			assert.Empty(t, recorder.events, "L2 must not guess cross-tenant from %d", status)
		}
	}
}
