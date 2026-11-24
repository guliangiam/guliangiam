package coupons

import (
	"context"
	"errors"
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/stretchr/testify/require"
)

func assertByIDInvalidArgument(t *testing.T, err error) {
	t.Helper()
	var appErr *apperrors.Error
	require.True(t, errors.As(err, &appErr), "expected *apperrors.Error, got %v", err)
	require.Equal(t, apperrors.CodeInvalidArgument, appErr.Code)
}

// TestCouponByID_RequiresOrganization pins the non-HTTP fail-fast: without a
// trusted tenant the app layer rejects before touching any repository.
func TestCouponByID_RequiresOrganization(t *testing.T) {
	svc := Service{}
	ctx := context.Background()

	_, err := svc.GetCoupon(ctx, "  ", "coupon-1")
	assertByIDInvalidArgument(t, err)

	err = svc.PatchCoupon(ctx, "", PatchCouponCommand{ID: "coupon-1"})
	assertByIDInvalidArgument(t, err)

	err = svc.DeleteCoupon(ctx, "", "coupon-1")
	assertByIDInvalidArgument(t, err)
}
