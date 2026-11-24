package tickets

import (
	"math"
	"testing"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
)

func TestTicketMessageBounds(t *testing.T) {
	const total = 250
	cases := []struct {
		name               string
		page, pageSize     int
		wantStart, wantEnd int
		wantOK             bool
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			wantOK: false,
		},
		{
			name: "hostile page size is capped",
			page: 2, pageSize: math.MaxInt,
			wantStart: platformpagination.MaxPageSize, wantEnd: 2 * platformpagination.MaxPageSize, wantOK: true,
		},
		{
			name: "normal page",
			page: 3, pageSize: 25,
			wantStart: 50, wantEnd: 75, wantOK: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, ok := ticketMessageBounds(tc.page, tc.pageSize, total)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantStart, start)
			assert.Equal(t, tc.wantEnd, end)
			assert.GreaterOrEqual(t, start, 0)
			assert.GreaterOrEqual(t, end, start)
		})
	}
}
