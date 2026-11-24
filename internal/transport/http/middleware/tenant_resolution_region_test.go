package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeRegionResolver struct {
	codes map[string]string
	err   error
}

func (r *fakeRegionResolver) ResolveBatch(_ context.Context, orgIDs []string) (map[string]string, error) {
	if r.err != nil {
		return nil, r.err
	}
	out := make(map[string]string, len(orgIDs))
	for _, id := range orgIDs {
		if code, ok := r.codes[id]; ok {
			out[id] = code
		}
	}
	return out, nil
}

func regionGuardHandler(regionResolver RegionResolver, mode RegionResidencyMode) (http.Handler, *bool) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
	return TenantResolutionMiddlewareWithRegionGuard(nil, regionResolver, mode)(next), &called
}

func TestRegionResidencyEnforceAllowsSameRegion(t *testing.T) {
	sourceOrg := uuid.NewString()
	targetOrg := uuid.NewString()
	handler, called := regionGuardHandler(&fakeRegionResolver{codes: map[string]string{sourceOrg: "cn-north", targetOrg: "cn-north"}}, RegionResidencyEnforce)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, sessionRequestWithOrg(http.MethodGet, "/api/v1/users?organizationId="+targetOrg, "", "platform_admin", sourceOrg, "acme"))
	require.True(t, *called)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestRegionResidencyEnforceRejectsCrossRegion(t *testing.T) {
	sourceOrg := uuid.NewString()
	targetOrg := uuid.NewString()
	handler, called := regionGuardHandler(&fakeRegionResolver{codes: map[string]string{sourceOrg: "cn-north", targetOrg: "eu-west"}}, RegionResidencyEnforce)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, sessionRequestWithOrg(http.MethodGet, "/api/v1/users?organizationId="+targetOrg, "", "platform_admin", sourceOrg, "acme"))
	require.False(t, *called)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestRegionResidencyObserveAllowsCrossRegion(t *testing.T) {
	sourceOrg := uuid.NewString()
	targetOrg := uuid.NewString()
	handler, called := regionGuardHandler(&fakeRegionResolver{codes: map[string]string{sourceOrg: "cn-north", targetOrg: "eu-west"}}, RegionResidencyObserve)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, sessionRequestWithOrg(http.MethodGet, "/api/v1/users?organizationId="+targetOrg, "", "platform_admin", sourceOrg, "acme"))
	require.True(t, *called)
}

func TestRegionResidencyEnforceRejectsUnboundOrganization(t *testing.T) {
	sourceOrg := uuid.NewString()
	targetOrg := uuid.NewString()
	handler, called := regionGuardHandler(&fakeRegionResolver{codes: map[string]string{sourceOrg: "cn-north"}}, RegionResidencyEnforce)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, sessionRequestWithOrg(http.MethodGet, "/api/v1/users?organizationId="+targetOrg, "", "platform_admin", sourceOrg, "acme"))
	require.False(t, *called)
	require.Equal(t, http.StatusForbidden, recorder.Code)
}
