package coupons

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestCoupon_Validate(t *testing.T) {
	tests := []struct {
		name      string
		coupon    Coupon
		wantErr   bool
		errCode   apperrors.Code
		wantField string
	}{
		{
			name: "valid percentage coupon",
			coupon: Coupon{
				Code:            "SAVE20",
				DiscountType:    "percentage",
				Discount:        20,
				MaxDiscount:     100,
				Quantity:        100,
				MaxUsagePerUser: 1,
				MinOrderAmount:  0,
			},
			wantErr: false,
		},
		{
			name: "valid fixed coupon",
			coupon: Coupon{
				Code:            "FLAT50",
				DiscountType:    "fixed",
				Discount:        50,
				MaxDiscount:     0,
				Quantity:        200,
				MaxUsagePerUser: 3,
				MinOrderAmount:  100,
			},
			wantErr: false,
		},
		{
			name: "missing code",
			coupon: Coupon{
				DiscountType: "percentage",
				Discount:     10,
			},
			wantErr: true,
			errCode: apperrors.CodeInvalidArgument,
		},
		{
			name: "invalid discountType",
			coupon: Coupon{
				Code:         "BAD",
				DiscountType: "invalid",
				Discount:     10,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "negative discount",
			coupon: Coupon{
				Code:         "NEG",
				DiscountType: "fixed",
				Discount:     -5,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "percentage over 100",
			coupon: Coupon{
				Code:         "OVER",
				DiscountType: "percentage",
				Discount:     150,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "percentage exactly 100",
			coupon: Coupon{
				Code:         "EXACT100",
				DiscountType: "percentage",
				Discount:     100,
			},
			wantErr: false,
		},
		{
			name: "negative maxDiscount",
			coupon: Coupon{
				Code:         "MD",
				DiscountType: "fixed",
				Discount:     10,
				MaxDiscount:  -1,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "negative quantity",
			coupon: Coupon{
				Code:         "QTY",
				DiscountType: "fixed",
				Discount:     10,
				Quantity:     -1,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "negative maxUsagePerUser",
			coupon: Coupon{
				Code:            "MU",
				DiscountType:    "fixed",
				Discount:        10,
				MaxUsagePerUser: -1,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "negative minOrderAmount",
			coupon: Coupon{
				Code:           "MOA",
				DiscountType:   "fixed",
				Discount:       10,
				MinOrderAmount: -1,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "valid time range",
			coupon: Coupon{
				Code:         "TIME",
				DiscountType: "fixed",
				Discount:     10,
				StartTime:    "2025-01-01T00:00:00Z",
				ExpireTime:   "2025-12-31T23:59:59Z",
			},
			wantErr: false,
		},
		{
			name: "expireTime before startTime",
			coupon: Coupon{
				Code:         "BADTIME",
				DiscountType: "fixed",
				Discount:     10,
				StartTime:    "2025-12-31T23:59:59Z",
				ExpireTime:   "2025-01-01T00:00:00Z",
			},
			wantErr:   true,
			errCode:   apperrors.CodeUnprocessableEntity,
			wantField: "expireTime",
		},
		{
			name: "expireTime same as startTime",
			coupon: Coupon{
				Code:         "SAMETIME",
				DiscountType: "fixed",
				Discount:     10,
				StartTime:    "2025-06-01T00:00:00Z",
				ExpireTime:   "2025-06-01T00:00:00Z",
			},
			wantErr:   true,
			errCode:   apperrors.CodeUnprocessableEntity,
			wantField: "expireTime",
		},
		{
			name: "invalid startTime format",
			coupon: Coupon{
				Code:         "BADFMT",
				DiscountType: "fixed",
				Discount:     10,
				StartTime:    "not-a-date",
				ExpireTime:   "2025-12-31T23:59:59Z",
			},
			wantErr:   true,
			errCode:   apperrors.CodeUnprocessableEntity,
			wantField: "startTime",
		},
		{
			name: "invalid expireTime format",
			coupon: Coupon{
				Code:         "BADFMT2",
				DiscountType: "fixed",
				Discount:     10,
				StartTime:    "2025-01-01T00:00:00Z",
				ExpireTime:   "not-a-date",
			},
			wantErr:   true,
			errCode:   apperrors.CodeUnprocessableEntity,
			wantField: "expireTime",
		},
		{
			name: "product scope without products",
			coupon: Coupon{
				Code:         "NOPROD",
				DiscountType: "fixed",
				Discount:     10,
				Scope:        "product",
				Products:     []string{},
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "product scope with products",
			coupon: Coupon{
				Code:         "HASPROD",
				DiscountType: "fixed",
				Discount:     10,
				Scope:        "product",
				Products:     []string{"plan-1"},
			},
			wantErr: false,
		},
		{
			name: "global scope without products is valid",
			coupon: Coupon{
				Code:         "GLOBAL",
				DiscountType: "fixed",
				Discount:     10,
				Scope:        "global",
			},
			wantErr: false,
		},
		{
			name: "zero values are valid",
			coupon: Coupon{
				Code:            "ZERO",
				DiscountType:    "fixed",
				Discount:        0,
				MaxDiscount:     0,
				Quantity:        0,
				MaxUsagePerUser: 0,
				MinOrderAmount:  0,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.coupon.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				appErr, ok := err.(*apperrors.Error)
				if !ok {
					t.Fatalf("error type = %T, want *apperrors.Error", err)
				}
				if appErr.Code != tt.errCode {
					t.Errorf("error code = %q, want %q", appErr.Code, tt.errCode)
				}
				if tt.wantField != "" {
					if field, _ := appErr.Meta["field"].(string); field != tt.wantField {
						t.Errorf("error meta field = %q, want %q", field, tt.wantField)
					}
				}
			}
		})
	}
}
