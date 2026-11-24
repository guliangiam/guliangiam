package actions

import (
	"context"
	"time"
)

// ExecOptions controls per-action sandbox resource limits and egress policy.
// Zero values are not used — callers should always fill them via
// NormalizeExecOptions to apply safe defaults.
type ExecOptions struct {
	// Timeout caps the wall-clock duration of one action invocation.
	// Default 30s, hard cap 120s.
	Timeout time.Duration

	// MemoryLimitMB bounds WASM linear memory growth. Default 64 MiB.
	MemoryLimitMB int

	// CPULimitMs bounds total CPU time in milliseconds. Default 5000 ms.
	CPULimitMs int

	// AllowNetwork permits the WASM module to perform outbound HTTP calls.
	// ADR-029 mandates the default is false — explicit opt-in required.
	AllowNetwork bool

	// AllowedHosts is a suffix-match whitelist enforced when AllowNetwork=true.
	// Empty means "no egress allowed".
	AllowedHosts []string
}

// NormalizeExecOptions fills zero fields with ADR-029 safe defaults and
// clamps Timeout to the 120s hard cap. Returns a new value; the receiver
// is untouched.
func NormalizeExecOptions(opts ExecOptions) ExecOptions {
	out := opts
	if out.Timeout <= 0 {
		out.Timeout = 30 * time.Second
	} else if out.Timeout > 120*time.Second {
		out.Timeout = 120 * time.Second
	}
	if out.MemoryLimitMB <= 0 {
		out.MemoryLimitMB = 64
	}
	if out.CPULimitMs <= 0 {
		out.CPULimitMs = 5000
	}
	return out
}

// Executor is the sandbox runtime contract. Implementations run action code
// (WASM / JS) with bounded resources and return the raw bytes the script
// emits, plus two separate error channels:
//
//   - execErr: business-level error raised by the script itself
//     (e.g. JS `throw new Error("deny login")`). The dispatcher treats a
//     non-nil execErr as "deny the extension point".
//   - err: sandbox/runtime error (timeout, OOM, compile failure, missing
//     module, host trap). The dispatcher treats a non-nil err as a
//     technical failure and marks the execution log failed/timeout.
//
// Both errors are nil on success.
type Executor interface {
	Execute(ctx context.Context, code string, input []byte, opts ExecOptions) (output []byte, execErr error, err error)
}

// NoopExecutor is a pass-through implementation used for local development
// or when the sandbox is explicitly disabled. It always returns input
// untouched with no errors, regardless of code content.
type NoopExecutor struct{}

// Execute implements Executor. It ignores code, opts, and ctx and returns
// the input bytes verbatim.
func (NoopExecutor) Execute(_ context.Context, _ string, input []byte, _ ExecOptions) ([]byte, error, error) {
	out := make([]byte, len(input))
	copy(out, input)
	return out, nil, nil
}

// compile-time interface satisfaction check
var _ Executor = NoopExecutor{}
