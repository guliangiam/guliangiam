package plans

import (
	"testing"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
)

func TestPlan_Validate(t *testing.T) {
	tests := []struct {
		name    string
		plan    Plan
		wantErr bool
		errCode apperrors.Code
	}{
		{
			name: "valid plan with monthly period",
			plan: Plan{
				Price:  9.99,
				Period: "monthly",
			},
			wantErr: false,
		},
		{
			name: "valid plan with no period",
			plan: Plan{
				Price:  19.99,
				Period: "",
			},
			wantErr: false,
		},
		{
			name: "valid plan with zero price",
			plan: Plan{
				Price:  0,
				Period: "monthly",
			},
			wantErr: false,
		},
		{
			name: "negative price",
			plan: Plan{
				Price:  -1,
				Period: "monthly",
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name: "invalid period",
			plan: Plan{
				Price:  10,
				Period: "biweekly",
			},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "valid daily period",
			plan:    Plan{Price: 1, Period: "daily"},
			wantErr: false,
		},
		{
			name:    "valid weekly period",
			plan:    Plan{Price: 5, Period: "weekly"},
			wantErr: false,
		},
		{
			name:    "valid quarterly period",
			plan:    Plan{Price: 30, Period: "quarterly"},
			wantErr: false,
		},
		{
			name:    "valid yearly period",
			plan:    Plan{Price: 100, Period: "yearly"},
			wantErr: false,
		},
		{
			name:    "invalid state",
			plan:    Plan{Price: 10, Period: "monthly", State: "unknown"},
			wantErr: true,
			errCode: apperrors.CodeUnprocessableEntity,
		},
		{
			name:    "valid lifetime period",
			plan:    Plan{Price: 999, Period: "lifetime"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.plan.Validate()
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

func TestPlan_ValidPeriods(t *testing.T) {
	validPeriods := []string{"daily", "weekly", "monthly", "quarterly", "yearly", "lifetime"}
	for _, period := range validPeriods {
		t.Run(period, func(t *testing.T) {
			if !Periods.Contains(period) {
				t.Errorf("period %q should be valid", period)
			}
		})
	}
}

func TestPlan_InvalidPeriods(t *testing.T) {
	// The PascalCase forms are what the console used to send; they must stay
	// rejected so the two lists cannot drift back apart.
	invalidPeriods := []string{"biweekly", "semiannual", "decade", "hourly", "P1M", "Monthly", "Yearly"}
	for _, period := range invalidPeriods {
		t.Run(period, func(t *testing.T) {
			if Periods.Contains(period) {
				t.Errorf("period %q should be invalid", period)
			}
		})
	}
}

func TestPlan_StateDefaultsToDraft(t *testing.T) {
	p := Plan{Price: 1}
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if p.State != StateDraft {
		t.Fatalf("State = %q, want %q", p.State, StateDraft)
	}
}

func TestPlan_RejectsUnknownState(t *testing.T) {
	for _, state := range []string{"Archived", "archived ", "enabled", "disabled"} {
		t.Run(state, func(t *testing.T) {
			p := Plan{Price: 1, State: state}
			if err := p.Validate(); err == nil {
				t.Fatalf("Validate() accepted state %q", state)
			}
		})
	}
}

func TestPlan_IsSellable(t *testing.T) {
	if !IsSellable(StateActive) {
		t.Fatalf("IsSellable(%q) = false, want true", StateActive)
	}
	for _, state := range []string{StateDraft, StateArchived} {
		if IsSellable(state) {
			t.Errorf("IsSellable(%q) = true, want false", state)
		}
	}
}

func TestPlan_CanTransitionTo(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want bool
	}{
		{StateDraft, StateActive, true},
		{StateDraft, StateArchived, true},
		{StateDraft, StateDraft, false},
		{StateActive, StateArchived, true},
		{StateActive, StateDraft, false},
		{StateArchived, StateActive, false},
		{"unknown", StateDraft, false},
	}
	for _, tt := range tests {
		t.Run(tt.from+"->"+tt.to, func(t *testing.T) {
			p := Plan{State: tt.from}
			if got := p.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("CanTransitionTo(%q -> %q) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

// TestPlan_TransitionsCoverRegistryStateSet keeps the transition table and the
// closed state set in lockstep.
func TestPlan_TransitionsCoverRegistryStateSet(t *testing.T) {
	for _, state := range States.Values() {
		if _, ok := validPlanTransitions[state]; !ok {
			t.Errorf("state %q has no entry in validPlanTransitions", state)
		}
	}
	for from, targets := range validPlanTransitions {
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

// TestPlan_TerminalStatesHaveNoRecoveryEdge enforces §通用规则 ②: the registry
// declares no recovery edge for `archived`.
func TestPlan_TerminalStatesHaveNoRecoveryEdge(t *testing.T) {
	if edges := validPlanTransitions[StateArchived]; len(edges) != 0 {
		t.Errorf("terminal plan state %q has outgoing edges %v", StateArchived, edges)
	}
	if !IsTerminal(StateArchived) {
		t.Fatalf("IsTerminal(%q) = false, want true", StateArchived)
	}
}
