package shared

import (
	"math"
	"testing"

	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/stretchr/testify/assert"
)

func TestPageOffset_ClampsHostilePageBeforeMultiplying(t *testing.T) {
	cases := []struct {
		name           string
		page, pageSize int
		want           int
	}{
		{
			name: "hostile page is capped",
			page: math.MaxInt, pageSize: 20,
			want: (platformpagination.MaxPage - 1) * 20,
		},
		{
			name: "hostile page size keeps the mysql ceiling",
			page: 2, pageSize: math.MaxInt,
			want: 500,
		},
		{
			name: "normal page",
			page: 3, pageSize: 20,
			want: 40,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, PageOffset(tc.page, tc.pageSize))
			assert.GreaterOrEqual(t, PageOffset(tc.page, tc.pageSize), 0)
		})
	}
}
