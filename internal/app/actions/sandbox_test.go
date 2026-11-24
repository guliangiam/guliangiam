package actions

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestNormalizeExecOptions_Defaults(t *testing.T) {
	got := NormalizeExecOptions(ExecOptions{})
	if got.Timeout != 30*time.Second {
		t.Errorf("Timeout default = %v, want 30s", got.Timeout)
	}
	if got.MemoryLimitMB != 64 {
		t.Errorf("MemoryLimitMB default = %d, want 64", got.MemoryLimitMB)
	}
	if got.CPULimitMs != 5000 {
		t.Errorf("CPULimitMs default = %d, want 5000", got.CPULimitMs)
	}
	if got.AllowNetwork {
		t.Error("AllowNetwork default must be false per ADR-029")
	}
}

func TestNormalizeExecOptions_ClampTimeout(t *testing.T) {
	cases := []struct {
		name string
		in   time.Duration
		want time.Duration
	}{
		{"zero → default", 0, 30 * time.Second},
		{"negative → default", -1 * time.Second, 30 * time.Second},
		{"10s kept", 10 * time.Second, 10 * time.Second},
		{"120s kept", 120 * time.Second, 120 * time.Second},
		{"200s clamped", 200 * time.Second, 120 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeExecOptions(ExecOptions{Timeout: tc.in})
			if got.Timeout != tc.want {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tc.want)
			}
		})
	}
}

func TestNormalizeExecOptions_PreserveExplicit(t *testing.T) {
	in := ExecOptions{
		Timeout:       15 * time.Second,
		MemoryLimitMB: 128,
		CPULimitMs:    9000,
		AllowNetwork:  true,
		AllowedHosts:  []string{"api.example.com"},
	}
	got := NormalizeExecOptions(in)
	if got.Timeout != 15*time.Second {
		t.Errorf("Timeout mutated: %v", got.Timeout)
	}
	if got.MemoryLimitMB != 128 {
		t.Errorf("MemoryLimitMB mutated: %d", got.MemoryLimitMB)
	}
	if got.CPULimitMs != 9000 {
		t.Errorf("CPULimitMs mutated: %d", got.CPULimitMs)
	}
	if !got.AllowNetwork {
		t.Error("AllowNetwork mutated")
	}
	if len(got.AllowedHosts) != 1 || !strings.EqualFold(got.AllowedHosts[0], "api.example.com") {
		t.Errorf("AllowedHosts mutated: %v", got.AllowedHosts)
	}
}

func TestNoopExecutor(t *testing.T) {
	exec := NoopExecutor{}
	input := []byte(`{"user":"alice"}`)
	out, execErr, runErr := exec.Execute(context.Background(), "any code", input, ExecOptions{})
	if execErr != nil {
		t.Errorf("execErr = %v, want nil", execErr)
	}
	if runErr != nil {
		t.Errorf("runErr = %v, want nil", runErr)
	}
	if !bytes.Equal(out, input) {
		t.Errorf("output = %q, want %q", out, input)
	}
}

func TestNoopExecutor_NilInput(t *testing.T) {
	exec := NoopExecutor{}
	out, execErr, runErr := exec.Execute(context.Background(), "", nil, ExecOptions{})
	if execErr != nil || runErr != nil {
		t.Fatalf("unexpected error")
	}
	// nil input → empty non-nil slice (make with len 0)
	if len(out) != 0 {
		t.Errorf("output = %v, want empty", out)
	}
}
