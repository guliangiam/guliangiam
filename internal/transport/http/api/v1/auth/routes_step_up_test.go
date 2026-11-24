package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guliangiam/guliangiam/internal/transport/http/route"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoutes_StepUp(t *testing.T) {
	definitions := Routes()
	found := make(map[string]route.Definition, len(definitions))
	for _, definition := range definitions {
		found[definition.OperationID] = definition
	}

	for _, want := range []struct {
		operationID string
		method      string
		pattern     string
		permission  string
	}{
		{"PostAuthStepUpBegin", "POST", "/api/v1/auth/step-up/begin", "auth:step_up"},
		{"PostAuthStepUpFinish", "POST", "/api/v1/auth/step-up/finish", "auth:step_up"},
	} {
		definition, ok := found[want.operationID]
		if !assert.True(t, ok, "missing route %s", want.operationID) {
			continue
		}
		assert.Equal(t, want.method, definition.Method)
		assert.Equal(t, want.pattern, definition.Pattern)
		assert.Equal(t, want.permission, definition.Permission)
		assert.Equal(t, route.SurfaceAuthentication, definition.Surface)
	}
}

// 未认证请求必须在进入 Service 之前被拒绝：只有服务端已经知道
// UserID / OrganizationID / SessionID 时才能抬升会话，缺任一项均返回 401。
func TestStepUpSessionRequiresVerifiedSession(t *testing.T) {
	cases := []struct {
		name    string
		session *SessionContext
		wantOK  bool
	}{
		{name: "no_session", session: nil, wantOK: false},
		{name: "missing_session_id", session: &SessionContext{UserID: "user-1", OrganizationID: "org-1"}, wantOK: false},
		{name: "missing_user_id", session: &SessionContext{ID: "sess-1", OrganizationID: "org-1"}, wantOK: false},
		{name: "missing_organization_id", session: &SessionContext{ID: "sess-1", UserID: "user-1"}, wantOK: false},
		{name: "complete_session", session: &SessionContext{ID: "sess-1", UserID: "user-1", OrganizationID: "org-1"}, wantOK: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/step-up/finish", nil)
			if tc.session != nil {
				request = request.WithContext(WithSessionContext(request.Context(), *tc.session))
			}
			recorder := httptest.NewRecorder()

			session, ok := stepUpSession(recorder, request)
			require.Equal(t, tc.wantOK, ok)
			if !tc.wantOK {
				require.Equal(t, http.StatusUnauthorized, recorder.Code)
				assert.Contains(t, recorder.Body.String(), "authentication required")
				return
			}
			assert.Equal(t, "sess-1", session.ID)
			assert.Empty(t, recorder.Body.String())
		})
	}
}
