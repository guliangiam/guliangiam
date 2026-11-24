package mysql

import (
	"math"
	"testing"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
)

func TestRebacPageWindow(t *testing.T) {
	cases := []struct {
		name                  string
		page, pageSize        int
		defaultSize           int
		wantLimit, wantOffset int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20, defaultSize: 20,
			wantLimit: 20, wantOffset: (platformpagination.MaxPage - 1) * 20,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt, defaultSize: 50,
			wantLimit: 500, wantOffset: 500,
		},
		{
			name: "tuple default size is preserved",
			page: 2, pageSize: 0, defaultSize: 50,
			wantLimit: 50, wantOffset: 50,
		},
		{
			name: "normal page",
			page: 3, pageSize: 25, defaultSize: 20,
			wantLimit: 25, wantOffset: 50,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limit, offset := rebacPageWindow(tc.page, tc.pageSize, tc.defaultSize)
			assert.Equal(t, tc.wantLimit, limit)
			assert.Equal(t, tc.wantOffset, offset)
			assert.GreaterOrEqual(t, offset, 0)
		})
	}
}
