package products

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestProduct_Validate(t *testing.T) {
	tests := []struct {
		name    string
		product Product
		wantErr bool
		errCode apperrors.Code
	}{
		{
			name: "valid draft product",
			product: Product{
				Name: "basic", State: "draft", Price: 0, Quantity: 100,
			},
			wantErr: false,
		},
		{
			name: "valid active product",
			product: Product{
				Name: "pro", State: "active", Price: 99.99, Quantity: 50,
			},
			wantErr: false,
		},
		{
			name: "valid discontinued product",
			product: Product{
				Name: "legacy", State: "discontinued", Price: 0, Quantity: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			product: Product{
				Name: "bad", State: "unknown", Price: 0,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "negative price",
			product: Product{
				Name: "neg", State: "active", Price: -1,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "negative quantity",
			product: Product{
				Name: "negq", State: "active", Price: 10, Quantity: -1,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "recharge without options",
			product: Product{
				Name: "recharge", State: "active", Price: 0,
				IsRecharge: true, RechargeOptions: []float64{},
				DisableCustomRecharge: false,
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "recharge with options",
			product: Product{
				Name: "recharge", State: "active", Price: 0,
				IsRecharge: true, RechargeOptions: []float64{10, 50, 100},
				DisableCustomRecharge: false,
			},
			wantErr: false,
		},
		{
			name: "recharge with custom allowed",
			product: Product{
				Name: "recharge", State: "active", Price: 0,
				IsRecharge: true, RechargeOptions: []float64{},
				DisableCustomRecharge: true,
			},
			wantErr: false,
		},
		{
			name: "non-recharge product no options needed",
			product: Product{
				Name: "fixed", State: "active", Price: 50,
				IsRecharge: false, RechargeOptions: []float64{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.product.Validate()
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

func TestProduct_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
		want bool
	}{
		{"draft -> active", "draft", "active", true},
		{"draft -> discontinued", "draft", "discontinued", true},
		{"draft -> draft", "draft", "draft", false},
		{"active -> discontinued", "active", "discontinued", true},
		{"active -> draft", "active", "draft", false},
		{"discontinued is terminal", "discontinued", "draft", false},
		{"discontinued -> active", "discontinued", "active", false},
		{"unknown -> draft", "unknown", "draft", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := Product{State: tt.from}
			got := p.CanTransitionTo(tt.to)
			if got != tt.want {
				t.Errorf("CanTransitionTo(%q -> %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// TestProduct_TransitionsCoverRegistryStateSet keeps the transition table and the
// closed state set in lockstep: a state with no outgoing entry would silently
// reject every transition, and an edge to an unregistered state would let a
// write pass the domain check and fail at the DDL CHECK instead.
func TestProduct_TransitionsCoverRegistryStateSet(t *testing.T) {
	for _, state := range States.Values() {
		if _, ok := validProductTransitions[state]; !ok {
			t.Errorf("state %q has no entry in validProductTransitions", state)
		}
	}
	for from, targets := range validProductTransitions {
		if !States.Contains(from) {
			t.Errorf("transition source %q is not in the registry state set", from)
		}
		for _, to := range targets {
			if !States.Contains(to) {
				t.Errorf("transition %q -> %q targets an unregistered state", from, to)
			}
		}
	}
}

// TestProduct_TerminalStatesHaveNoRecoveryEdge enforces §通用规则 ②: a terminal
// state is irreversible unless the registry declares a recovery edge, and the
// registry declares none for products.
func TestProduct_TerminalStatesHaveNoRecoveryEdge(t *testing.T) {
	for state, terminal := range terminalProductStates {
		if !terminal {
			continue
		}
		if edges := validProductTransitions[state]; len(edges) != 0 {
			t.Errorf("terminal product state %q has outgoing edges %v", state, edges)
		}
	}
}

func TestProduct_IsTerminal(t *testing.T) {
	if !IsTerminal(StateDiscontinued) {
		t.Fatalf("IsTerminal(%q) = false, want true", StateDiscontinued)
	}
	for _, state := range []string{StateDraft, StateActive} {
		if IsTerminal(state) {
			t.Errorf("IsTerminal(%q) = true, want false", state)
		}
	}
}

// TestProduct_RejectsLegacyArchived ensures the withdrawn pre-registry spelling
// is not quietly accepted after the rename to `discontinued`.
func TestProduct_RejectsLegacyArchived(t *testing.T) {
	p := Product{Name: "legacy", State: "archived"}
	if err := p.Validate(); err == nil {
		t.Fatal("Validate() accepted the pre-registry state 'archived'")
	}
	p = Product{State: "Archived"}
	if IsKnownState(p.State) {
		t.Fatalf("IsKnownState(%q) = true, want false", p.State)
	}
}
