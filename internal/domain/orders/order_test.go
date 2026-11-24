package orders

import (
	"testing"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestOrder_Validate_ValidOrder(t *testing.T) {
	order := Order{
		ID:             "order-1",
		OrganizationID: "org-1",
		Name:           "test-order",
		State:          StatePaymentPending,
		Price:          99.99,
		Currency:       "USD",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := order.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

// TestOrder_Validate_AllRegistryStates iterates the declared state set rather
// than a hand-written list: a state added to the registry is validated here
// automatically, so the test cannot drift away from the vocabulary.
func TestOrder_Validate_AllRegistryStates(t *testing.T) {
	for _, state := range States.Values() {
		order := Order{State: state, Price: 0}
		if err := order.Validate(); err != nil {
			t.Errorf("Validate() for state %q error = %v, want nil", state, err)
		}
	}
}

func TestOrder_Validate_DefaultsEmptyStateToCreated(t *testing.T) {
	order := Order{Price: 1.0}
	if err := order.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
	if order.State != StateCreated {
		t.Errorf("State = %q, want %q", order.State, StateCreated)
	}
}

func TestOrder_Validate_InvalidState(t *testing.T) {
	order := Order{State: "unknown", Price: 0}
	err := order.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for invalid state")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
	}
}

// TestOrder_Validate_RejectsPascalCase pins rule ② of the state-machine
// registry: the vocabulary is lowercase snake_case, and a legacy spelling must
// fail loudly instead of being silently persisted.
func TestOrder_Validate_RejectsPascalCase(t *testing.T) {
	for _, state := range []string{"Created", "Paid", "Pending", "PaymentPending", "Canceled"} {
		order := Order{State: state, Price: 0}
		if err := order.Validate(); err == nil {
			t.Errorf("Validate() for state %q error = nil, want rejection", state)
		}
	}
}

func TestOrder_Validate_NegativePrice(t *testing.T) {
	order := Order{State: StatePaymentPending, Price: -1.0}
	err := order.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for negative price")
	}
	appErr, ok := err.(*apperrors.Error)
	if !ok {
		t.Fatalf("error type = %T, want *apperrors.Error", err)
	}
	if appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Errorf("error code = %q, want %q", appErr.Code, apperrors.CodeUnprocessableEntity)
	}
}

func TestOrder_Validate_NegativeCouponDiscount(t *testing.T) {
	order := Order{State: StatePaymentPending, Price: 10.0, CouponDiscount: -5.0}
	err := order.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want error for negative coupon discount")
	}
}

func TestOrder_Validate_ZeroPrice(t *testing.T) {
	order := Order{State: StatePaymentPending, Price: 0}
	if err := order.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil for zero price", err)
	}
}

// TestOrder_CanTransitionTo_DeclaredEdges walks the registry's transition graph
// as authored: every declared edge must be accepted.
func TestOrder_CanTransitionTo_DeclaredEdges(t *testing.T) {
	for from, toStates := range validOrderTransitions {
		for _, to := range toStates {
			order := Order{State: from}
			if !order.CanTransitionTo(to) {
				t.Errorf("CanTransitionTo(%q -> %q) = false, want true", from, to)
			}
		}
	}
}

// TestOrder_CanTransitionTo_UndeclaredEdgesRejected asserts the complement: any
// pair outside the declared graph is refused, including the transitions the
// legacy PascalCase vocabulary allowed (`paid -> refunded`).
func TestOrder_CanTransitionTo_UndeclaredEdgesRejected(t *testing.T) {
	for _, from := range States.Values() {
		allowed := map[string]bool{}
		for _, to := range validOrderTransitions[from] {
			allowed[to] = true
		}
		for _, to := range States.Values() {
			if from == to || allowed[to] {
				continue
			}
			order := Order{State: from}
			if order.CanTransitionTo(to) {
				t.Errorf("CanTransitionTo(%q -> %q) = true, want false", from, to)
			}
		}
	}
}

func TestOrder_CanTransitionTo_RecoveryEdgeFromFailed(t *testing.T) {
	order := Order{State: StateFailed}
	if !order.CanTransitionTo(StatePaymentPending) {
		t.Error("failed -> payment_pending is the declared recovery edge and must be accepted")
	}
	if order.CanTransitionTo(StatePaid) {
		t.Error("failed -> paid skips the channel and must be refused")
	}
	if IsTerminal(StateFailed) {
		t.Error("failed is deliberately not terminal: the recovery edge requires it to stay open")
	}
}

func TestOrder_CanTransitionTo_TerminalStates(t *testing.T) {
	for _, state := range States.Values() {
		if !IsTerminal(state) {
			continue
		}
		order := Order{State: state}
		for _, to := range States.Values() {
			if order.CanTransitionTo(to) {
				t.Errorf("terminal state %q should not transition to %q", state, to)
			}
		}
	}
}

func TestOrder_CanTransitionTo_UnknownState(t *testing.T) {
	order := Order{State: "unknown"}
	for _, to := range States.Values() {
		if order.CanTransitionTo(to) {
			t.Errorf("unknown state should not transition to %q", to)
		}
	}
}

// TestOrder_RequiresPaidAt_MatchesPaidStates pins the 判定依据 contract of
// `设计报告 42.3.3`: exactly the states that presuppose a settlement instant
// require one, and the DDL constraint `ck_orders_paid_at` encodes the same set.
func TestOrder_RequiresPaidAt_MatchesPaidStates(t *testing.T) {
	want := map[string]bool{
		StatePaid: true, StateCompleted: true, StateRefunded: true, StateClosed: true,
	}
	for _, state := range States.Values() {
		if got := RequiresPaidAt(state); got != want[state] {
			t.Errorf("RequiresPaidAt(%q) = %v, want %v", state, got, want[state])
		}
	}
}

// TestOrder_IsAwaitingPayment_ExcludesFailed pins the polling verdict: a client
// keeps polling exactly while the channel may still advance the order, and
// `failed` is excluded because only an explicit retry may move it.
func TestOrder_IsAwaitingPayment_ExcludesFailed(t *testing.T) {
	awaiting := []string{StateCreated, StatePaymentPending}
	for _, state := range States.Values() {
		want := false
		for _, a := range awaiting {
			if state == a {
				want = true
			}
		}
		if got := IsAwaitingPayment(state); got != want {
			t.Errorf("IsAwaitingPayment(%q) = %v, want %v", state, got, want)
		}
	}
}

func TestOrder_PollAfter_IsServerOwned(t *testing.T) {
	if PollInterval >= SettledPollInterval {
		t.Fatalf("PollInterval %v must be shorter than SettledPollInterval %v", PollInterval, SettledPollInterval)
	}
	if got := PollAfter(StatePaymentPending); got != PollInterval {
		t.Errorf("PollAfter(payment_pending) = %v, want %v", got, PollInterval)
	}
	if got := PollAfter(StatePaid); got != SettledPollInterval {
		t.Errorf("PollAfter(paid) = %v, want %v", got, SettledPollInterval)
	}
	if got := PollAfter("unknown"); got != SettledPollInterval {
		t.Errorf("PollAfter(unknown) = %v, want %v", got, SettledPollInterval)
	}
}

func TestOrder_Validate_RejectsUnknownStateWith422(t *testing.T) {
	order := Order{ID: "order-1", State: "unknown"}
	err := order.Validate()
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity || appErr.HTTPStatus() != 422 {
		t.Fatalf("expected 422 unprocessable_entity, got %v", err)
	}
	if appErr.Meta["field"] != "state" || appErr.Meta["set"] != "ordersState" {
		t.Fatalf("expected state/ordersState metadata, got %v", appErr.Meta)
	}
}

func TestOrder_ValidateUsesRangeMetadataForNegativeAmounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		order Order
		field string
	}{
		{name: "price", order: Order{Price: -1}, field: "price"},
		{name: "coupon discount", order: Order{CouponDiscount: -1}, field: "couponDiscount"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (&tc.order).Validate()
			appErr, ok := err.(*apperrors.Error)
			if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
				t.Fatalf("expected 422 unprocessable_entity, got %v", err)
			}
			if appErr.Meta["field"] != tc.field || appErr.Meta["min"] != 0 {
				t.Fatalf("expected %s/min metadata, got %v", tc.field, appErr.Meta)
			}
		})
	}
}

func TestOrder_TransitionTo_RejectsIllegalEdgeWith422(t *testing.T) {
	order := Order{State: StatePaid}
	err := order.TransitionTo(StatePaymentPending)
	appErr, ok := err.(*apperrors.Error)
	if !ok || appErr.Code != apperrors.CodeUnprocessableEntity {
		t.Fatalf("expected 422 unprocessable_entity, got %v", err)
	}
	if appErr.Meta["field"] != "state" || appErr.Meta["value"] != StatePaid ||
		appErr.Meta["transition"] != StatePaymentPending {
		t.Fatalf("expected state/transition metadata, got %v", appErr.Meta)
	}
	allowed, ok := appErr.Meta["allowed"].([]string)
	if !ok || len(allowed) != 1 || allowed[0] != StateCompleted {
		t.Fatalf("expected allowed=[completed], got %v", appErr.Meta["allowed"])
	}
}
