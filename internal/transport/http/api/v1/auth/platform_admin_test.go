package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/guliangiam/guliangiam/internal/platform/tenantctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequirePlatformAdmin(t *testing.T) {
	cases := []struct {
		name       string
		ctx        context.Context
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "missing session is denied",
			ctx:        context.Background(),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "ordinary user is denied",
			ctx:        WithSessionContext(context.Background(), SessionContext{PrincipalType: "user"}),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "super-admin tenant scope alone is denied",
			ctx:        tenantctx.WithTenantScope(context.Background(), "", "", tenantctx.TenantFilterSuperAdmin),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "platform admin is allowed",
			ctx:        WithSessionContext(context.Background(), SessionContext{PrincipalType: "platform_admin"}),
			wantStatus: http.StatusNoContent,
			wantNext:   true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nextCalls := 0
			handler := RequirePlatformAdmin(func(w http.ResponseWriter, _ *http.Request) {
				nextCalls++
				w.WriteHeader(http.StatusNoContent)
			})

			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/global/users", nil).WithContext(tc.ctx)
			handler(recorder, req)

			require.Equal(t, tc.wantStatus, recorder.Code)
			assert.Equal(t, tc.wantNext, nextCalls > 0)
			if !tc.wantNext {
				var body struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
				assert.Equal(t, "forbidden", body.Error.Code)
			}
		})
	}
}
