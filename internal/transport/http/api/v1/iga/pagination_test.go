package iga

import (
	"math"
	"testing"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
)

func TestPrivilegedSessionBounds(t *testing.T) {
	const total = 250
	cases := []struct {
		name               string
		page, pageSize     int
		wantStart, wantEnd int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			wantStart: total, wantEnd: total,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt,
			wantStart: platformpagination.MaxPageSize, wantEnd: 2 * platformpagination.MaxPageSize,
		},
		{
			name: "normal page",
			page: 3, pageSize: 25,
			wantStart: 50, wantEnd: 75,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := privilegedSessionBounds(total, tc.page, tc.pageSize)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantEnd, end)
			assert.GreaterOrEqual(t, start, 0)
			assert.GreaterOrEqual(t, end, start)
		})
	}
}
