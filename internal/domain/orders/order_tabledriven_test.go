package orders

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestOrder_Validate_TableDriven(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		order   Order
		wantErr bool
		errCode apperrors.Code
	}{
		{
			name: "valid payment_pending order",
			order: Order{
				ID: "order-1", OrganizationID: "org-1", Name: "test",
				State: StatePaymentPending, Price: 99.99, Currency: "USD",
				CreatedAt: now, UpdatedAt: now,
			},
			wantErr: false,
		},
		{
			name:    "invalid state",
			order:   Order{State: "unknown", Price: 0},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "legacy PascalCase state",
			order:   Order{State: "Pending", Price: 0},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "negative price",
			order:   Order{State: StatePaymentPending, Price: -1.0},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "negative coupon discount",
			order:   Order{State: StatePaymentPending, Price: 10.0, CouponDiscount: -5.0},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "zero price valid",
			order:   Order{State: StatePaymentPending, Price: 0},
			wantErr: false,
		},
		{
			name: "valid paid order with product infos",
			order: Order{
				State: StatePaid, Price: 50.0,
				ProductInfos: []ProductInfo{
					{Owner: "org-1", Name: "plan-1", DisplayName: "Pro", Price: 50, Currency: "USD", Quantity: 1},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.order.Validate()
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
			}
		})
	}
}

func TestOrder_CanTransitionTo_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"created -> payment_pending", StateCreated, StatePaymentPending, true},
		{"created -> cancelled", StateCreated, StateCancelled, true},
		{"created -> paid (skips the channel)", StateCreated, StatePaid, false},
		{"created -> failed", StateCreated, StateFailed, false},
		{"payment_pending -> paid", StatePaymentPending, StatePaid, true},
		{"payment_pending -> failed", StatePaymentPending, StateFailed, true},
		{"payment_pending -> cancelled", StatePaymentPending, StateCancelled, true},
		{"payment_pending -> completed", StatePaymentPending, StateCompleted, false},
		{"payment_pending -> refunded", StatePaymentPending, StateRefunded, false},
		{"paid -> completed", StatePaid, StateCompleted, true},
		{"paid -> refunded (refund follows completion)", StatePaid, StateRefunded, false},
		{"paid -> payment_pending", StatePaid, StatePaymentPending, false},
		{"completed -> refunded", StateCompleted, StateRefunded, true},
		{"completed -> closed", StateCompleted, StateClosed, true},
		{"completed -> payment_pending", StateCompleted, StatePaymentPending, false},
		{"failed -> payment_pending (recovery edge)", StateFailed, StatePaymentPending, true},
		{"failed -> paid", StateFailed, StatePaid, false},
		{"cancelled -> payment_pending", StateCancelled, StatePaymentPending, false},
		{"cancelled -> paid", StateCancelled, StatePaid, false},
		{"refunded -> payment_pending", StateRefunded, StatePaymentPending, false},
		{"refunded -> completed", StateRefunded, StateCompleted, false},
		{"closed -> payment_pending", StateClosed, StatePaymentPending, false},
		{"unknown -> payment_pending", "unknown", StatePaymentPending, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := Order{State: tt.from}
			got := order.CanTransitionTo(tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%q -> %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestOrder_AllValidStates(t *testing.T) {
	for _, state := range States.Values() {
		t.Run(state, func(t *testing.T) {
			order := Order{State: state, Price: 0}
			if err := order.Validate(); err != nil {
				t.Errorf("Validate() for state %q error = %v, want nil", state, err)
			}
		})
	}
}

func TestProductInfo_Fields(t *testing.T) {
	pi := ProductInfo{
		Owner:       "org-1",
		Name:        "pro-plan",
		DisplayName: "Pro Plan",
		Image:       "https://img.example.com/pro.png",
		Detail:      "Professional plan",
		Price:       99.99,
		Currency:    "USD",
		IsRecharge:  false,
		Quantity:    1,
		PricingName: "monthly",
		PlanName:    "pro",
	}
	if pi.Owner != "org-1" {
		t.Errorf("Owner = %q, want %q", pi.Owner, "org-1")
	}
	if pi.Price != 99.99 {
		t.Errorf("Price = %v, want 99.99", pi.Price)
	}
	if pi.Quantity != 1 {
		t.Errorf("Quantity = %d, want 1", pi.Quantity)
	}
}
