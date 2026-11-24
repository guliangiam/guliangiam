package coupons

import (
	"context"
	"math"
	"testing"

	domain "github.com/guliangiam/guliangiam/internal/domain/coupons"
	"github.com/guliangiam/guliangiam/internal/infra/postgres/shared"
	platformpagination "github.com/guliangiam/guliangiam/internal/platform/pagination"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCouponUsageRepository_ListByCoupon_CheckedPagination(t *testing.T) {
	const couponID = "00000000-0000-0000-0000-000000000010"
	cases := []struct {
		name           string
		page, pageSize int
		wantLimit      int
		wantOffset     int
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
			page: 3, pageSize: 20,
			wantLimit: 20, wantOffset: 40,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotLimit, gotOffset int
			db := &shared.MockDBTX{
				QueryFn: func(_ context.Context, _ string, args ...any) (pgx.Rows, error) {
					gotLimit, _ = args[1].(int)
					gotOffset, _ = args[2].(int)
					return shared.NewMockRows(nil), nil
				},
				QueryRowFn: func(_ context.Context, _ string, _ ...any) pgx.Row {
					return shared.NewMockRow([]any{int64(0)}, nil)
				},
			}

			items, total, err := NewCouponUsageRepository(db).ListByCoupon(context.Background(), couponID, tc.page, tc.pageSize)
			require.NoError(t, err)
			assert.Empty(t, items)
			assert.Equal(t, int64(0), total)
			assert.Equal(t, tc.wantLimit, gotLimit)
			assert.Equal(t, tc.wantOffset, gotOffset)
			assert.GreaterOrEqual(t, gotOffset, 0)
		})
	}
}

var _ domain.CouponUsageRepository = CouponUsageRepository{}
