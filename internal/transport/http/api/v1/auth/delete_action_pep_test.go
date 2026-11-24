package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// seedRolePDPStub emulates the seeded RBAC roles: editor grants read/write,
// operator grants read/write/delete. It records the exact (resource, action)
// pair the PEP asks for so DELETE requests must resolve to "delete".
type seedRolePDPStub struct {
	grants       map[string]map[string]bool
	lastResource string
	lastAction   string
}

func (s *seedRolePDPStub) ResolveEnforcerUUID(context.Context, string) (string, error) {
	return "enforcer-1", nil
}

func (s *seedRolePDPStub) Enforce(_ context.Context, _, _ string, rvals ...interface{}) (bool, error) {
	if len(rvals) < 4 {
		return false, nil
	}
	subject, _ := rvals[0].(string)
	resource, _ := rvals[2].(string)
	action, _ := rvals[3].(string)
	s.lastResource, s.lastAction = resource, action
	return s.grants[subject][resource+":"+action], nil
}

func runDeletePEPForTest(pdp AuthorizationPDP, subject, path string) (int, int) {
	nextCalls := 0
	handler := testAPIAuthorizationMiddleware(pdp)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalls++
		w.WriteHeader(http.StatusNoContent)
	}))
	session := SessionContext{
		OrganizationID:   "org-uuid",
		OrganizationSlug: "acme",
		Username:         subject,
		Source:           "repository",
	}
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req = req.WithContext(WithSessionContext(req.Context(), session))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Code, nextCalls
}

func TestAPIAuthorizationMiddleware_DeleteRequiresDeleteAction(t *testing.T) {
	// Seeded role policies: editor has read/write only; operator adds delete.
	pdp := &seedRolePDPStub{grants: map[string]map[string]bool{
		"editor":   {},
		"operator": {},
	}}
	for _, resource := range []string{"users", "applications", "organizations", "groups", "providers"} {
		for _, action := range []string{"read", "write"} {
			pdp.grants["editor"][resource+":"+action] = true
			pdp.grants["operator"][resource+":"+action] = true
		}
		pdp.grants["operator"][resource+":delete"] = true
	}

	paths := map[string]string{
		"users":         "/api/v1/users/u-1",
		"applications":  "/api/v1/applications/a-1",
		"organizations": "/api/v1/organizations/o-1",
		"groups":        "/api/v1/groups/g-1",
		"providers":     "/api/v1/providers/p-1",
	}

	for _, resource := range []string{"users", "applications", "organizations", "groups", "providers"} {
		path := paths[resource]

		t.Run(resource+"/editor_denied", func(t *testing.T) {
			status, nextCalls := runDeletePEPForTest(pdp, "editor", path)
			assert.Equal(t, http.StatusForbidden, status)
			assert.Zero(t, nextCalls)
			assert.Equal(t, resource, pdp.lastResource)
			assert.Equal(t, "delete", pdp.lastAction, "DELETE must evaluate the delete action")
		})

		t.Run(resource+"/operator_allowed", func(t *testing.T) {
			status, nextCalls := runDeletePEPForTest(pdp, "operator", path)
			assert.Equal(t, http.StatusNoContent, status)
			assert.Equal(t, 1, nextCalls)
			assert.Equal(t, resource, pdp.lastResource)
			assert.Equal(t, "delete", pdp.lastAction, "DELETE must evaluate the delete action")
		})
	}
}
