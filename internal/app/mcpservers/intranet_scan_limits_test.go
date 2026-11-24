package mcpservers

import (
	"context"
	"net"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncIntranetServers_RejectsOversizedScanSurface(t *testing.T) {
	tooManyCIDRs := make([]string, maxIntranetCIDRs+1)
	for i := range tooManyCIDRs {
		tooManyCIDRs[i] = "127.0.0.1/32"
	}
	tooManyPorts := make([]int, maxIntranetPorts+1)
	for i := range tooManyPorts {
		tooManyPorts[i] = i + 1
	}

	tests := []struct {
		name string
		req  SyncIntranetServersRequest
		code apperrors.Code
	}{
		{
			name: "too many cidrs",
			req:  SyncIntranetServersRequest{CIDRs: tooManyCIDRs, Ports: []int{80}},
			code: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "too many ports",
			req:  SyncIntranetServersRequest{CIDRs: []string{"127.0.0.1/32"}, Ports: tooManyPorts},
			code: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "concurrency above cap",
			req: SyncIntranetServersRequest{
				CIDRs: []string{"127.0.0.1/32"}, Ports: []int{80}, Concurrency: maxIntranetConcurrency + 1,
			},
			code: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "single cidr wider than host cap",
			req:  SyncIntranetServersRequest{CIDRs: []string{"10.0.0.0/8"}, Ports: []int{80}},
			code: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "total targets above cap",
			req:  SyncIntranetServersRequest{CIDRs: []string{"10.0.0.0/16"}, Ports: []int{80, 443}},
			code: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "port below range",
			req:  SyncIntranetServersRequest{CIDRs: []string{"127.0.0.1/32"}, Ports: []int{0}},
			code: apperrors.CodeInvalidArgument,
		},
		{
			name: "port above range",
			req:  SyncIntranetServersRequest{CIDRs: []string{"127.0.0.1/32"}, Ports: []int{65536}},
			code: apperrors.CodeInvalidArgument,
		},
		{
			name: "invalid cidr",
			req:  SyncIntranetServersRequest{CIDRs: []string{"not-a-cidr"}, Ports: []int{80}},
			code: apperrors.CodeInvalidArgument,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Service{}.SyncIntranetServers(context.Background(), tc.req)

			require.Nil(t, result)
			var appErr *apperrors.Error
			require.ErrorAs(t, err, &appErr)
			assert.Equal(t, tc.code, appErr.Code)
		})
	}
}

// TestSyncIntranetServers_AllowsSurfaceWithinCaps pins the normal path. The
// probe targets a just-closed loopback listener, so it is refused immediately
// without scanning any network.
func TestSyncIntranetServers_AllowsSurfaceWithinCaps(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())

	svc := newSvc(&mockMcpServerRepo{})

	result, err := svc.SyncIntranetServers(context.Background(), SyncIntranetServersRequest{
		OrganizationID: "org-1",
		CIDRs:          []string{"127.0.0.1/32"},
		Ports:          []int{port},
		Concurrency:    1,
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Discovered)
	assert.Empty(t, result.Errors)
}
