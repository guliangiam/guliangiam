package payments

import (
	"math"
	"testing"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
)

// reconBatchWindow is the only arithmetic on the recon-batches read path; pin
// the hostile operands here instead of standing up a database.
func TestReconBatchWindow(t *testing.T) {
	cases := []struct {
		name                  string
		page, pageSize        int
		wantLimit, wantOffset int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			wantLimit: 20, wantOffset: (platformpagination.MaxPage - 1) * 20,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt,
			wantLimit: platformpagination.MaxPageSize, wantOffset: platformpagination.MaxPageSize,
		},
		{
			name: "normal page",
			page: 3, pageSize: 25,
			wantLimit: 25, wantOffset: 50,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limit, offset := reconBatchWindow(tc.page, tc.pageSize)
			assert.Equal(t, tc.wantLimit, limit)
			assert.Equal(t, tc.wantOffset, offset)
			assert.GreaterOrEqual(t, offset, 0)
		})
	}
}
