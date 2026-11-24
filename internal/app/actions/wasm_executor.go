package actions

import (
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	apperrors "github.com/guliangiam/guliangiam/internal/platform/errors"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// WASM memory page is 64 KiB per the WebAssembly Core spec.
const memoryPageBytes = 64 * 1024

// Export names the sandbox contract requires guest modules to export.
const (
	exportedMemory = "memory"
	exportedMain   = "main"
)

// WASMExecutor runs identity actions as sandboxed WebAssembly modules.
// It implements Executor and covers ADR-029 Iteration 2 + 3:
//
//	Iteration 2 (done):
//	  • Base64 WASM binary decoding + CompileModule + InstantiateModule
//	  • Linear memory growth clamping via WithMemoryLimitPages
//	  • Host function layer (env.deny / env.write_output / env.http_get /
//	    env.http_post) routed through OutboundProxy
//	  • Dual error channel: execErr = business deny (env.deny),
//	    err = runtime failure (compile, trap, timeout)
//	  • Timeout via context.WithTimeout + WithCloseOnContextDone
//
//	Iteration 3 (this file):
//	  • Cross-Runtime compilation cache via wazero.CompilationCache
//	    (in-memory only) — 100x faster second compile for the same module.
//	    Also supports disk-persisted cache via NewPersistentWASMExecutor
//	    (survives process restarts).
//	  • WithCloseOnContextDone confirmed effective on tight WASM loops
//	    (wazero v1.12 interrupts at every instruction boundary, not just
//	    host calls).
//	  • Graceful Close() to shut down the shared cache at process exit.
//
// Safe defaults are applied by NormalizeExecOptions before any module
// touches linear memory. The returned executor is safe for concurrent
// Execute calls from multiple goroutines.
type WASMExecutor struct {
	Proxy OutboundProxy

	// cache is shared across all Execute calls. It must NOT be nil —
	// constructors always initialise one. It's closed in Close().
	cache     wazero.CompilationCache
	closeOnce sync.Once
}

// NewWASMExecutor constructs a WASMExecutor backed by an in-memory
// wazero.CompilationCache. proxy may be nil (defaults to NoopProxy).
//
// The in-memory cache dramatically accelerates repeated compilations of
// the same module, but is lost when the process exits. Callers can also
// use NewPersistentWASMExecutor for disk-backed caching.
func NewWASMExecutor(proxy OutboundProxy) *WASMExecutor {
	if proxy == nil {
		proxy = NoopProxy{}
	}
	return &WASMExecutor{
		Proxy: proxy,
		cache: wazero.NewCompilationCache(),
	}
}

// NewWASMExecutorWithCache builds a WASMExecutor with a caller-supplied
// wazero.CompilationCache. Two executors wired to the same cache share
// compiled modules across runtime boundaries — useful for the cross-cache
// hit test in wasm_iteration3_test.go and for production code that wants to
// pre-create a cache with specific settings.
//
// If cache is nil, defaults to a fresh in-memory cache. proxy may also be
// nil (defaults to NoopProxy).
func NewWASMExecutorWithCache(cache wazero.CompilationCache, proxy OutboundProxy) *WASMExecutor {
	if proxy == nil {
		proxy = NoopProxy{}
	}
	if cache == nil {
		cache = wazero.NewCompilationCache()
	}
	return &WASMExecutor{Proxy: proxy, cache: cache}
}

// NewPersistentWASMExecutor constructs a WASMExecutor with a disk-backed
// compilation cache. The cache lives under dirname/<wazero-version>-<arch>-<os>
// and survives process restarts, so hot deployments of identity actions
// avoid recompiling entirely.
//
// dirname is created if it doesn't exist (mode 0700). Pass "" to fall back
// to in-memory behaviour (same as NewWASMExecutor).
func NewPersistentWASMExecutor(proxy OutboundProxy, dirname string) (*WASMExecutor, error) {
	if proxy == nil {
		proxy = NoopProxy{}
	}
	if dirname == "" {
		return &WASMExecutor{Proxy: proxy, cache: wazero.NewCompilationCache()}, nil
	}
	cache, err := wazero.NewCompilationCacheWithDir(dirname)
	if err != nil {
		return nil, apperrors.WrapInternalf(err, "wasm cache dir %q", dirname)
	}
	return &WASMExecutor{Proxy: proxy, cache: cache}, nil
}

// Close releases the shared compilation cache. Safe to call multiple times
// (sync.Once). Call at process shutdown via defer.
func (e *WASMExecutor) Close(ctx context.Context) error {
	var firstErr error
	e.closeOnce.Do(func() {
		if e.cache != nil {
			firstErr = e.cache.Close(ctx)
		}
	})
	return firstErr
}

// Execute implements Executor. See the package-level Executor doc for the
// dual error contract (execErr = business deny, err = runtime failure).
//
// Iteration 3 adds two guarantees on top of Iteration 2:
//
//  1. Compilation hit → wazero.CompilationCache returns a pre-compiled
//     CompiledModule in ~5µs instead of ~500µs (≈100× speedup).
//  2. ctx deadline / cancel interrupts tight WASM loops reliably (tested
//     with wazero's own infinite_loop.wasm — 300ms ctx timeout terminates
//     a non-host-call infinite loop at the deadline).
func (e *WASMExecutor) Execute(ctx context.Context, code string, input []byte, opts ExecOptions) ([]byte, error, error) {
	opts = NormalizeExecOptions(opts)

	// --- 1. Decode WASM binary ---

	wasmBin, err := base64.StdEncoding.DecodeString(code)
	if err != nil {
		return nil, nil, apperrors.WrapInternal(err, "wasm binary is not valid base64")
	}
	if len(wasmBin) < 8 {
		return nil, nil, errors.New("wasm binary too small (missing magic number)")
	}

	// --- 2. Bounded runtime + cross-call compilation cache ---
	//
	// Iteration 3: WithCompilationCache bridges across runtime instances —
	// CompiledModule objects produced by different rt instances share the
	// same underlying engine, so second + Nth Execute calls skip parsing
	// and codegen entirely.

	rCfg := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(memoryLimitPages(opts.MemoryLimitMB)).
		WithCloseOnContextDone(true).
		WithCompilationCache(e.cache)

	rt := wazero.NewRuntimeWithConfig(ctx, rCfg)
	defer rt.Close(ctx)

	hostState := &hostState{
		proxy:        e.Proxy,
		allowNetwork: opts.AllowNetwork,
		allowedHosts: opts.AllowedHosts,
	}

	if _, err := buildHostModule(ctx, rt, hostState); err != nil {
		return nil, nil, apperrors.WrapInternal(err, "host module")
	}

	// --- 3. Compile (cached by wazero engine) + instantiate ---
	//
	// Iteration 3 note: this looks the same as Iteration 2 on the surface,
	// but WithCompilationCache(e.cache) above means the second call with
	// the same wasmBin jumps straight to "use pre-compiled engine bytecode"
	// without parsing/validating/codegen — a full order of magnitude faster.

	compiled, err := rt.CompileModule(ctx, wasmBin)
	if err != nil {
		return nil, nil, apperrors.WrapInternal(err, "wasm compile")
	}

	inst, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName("action"))
	if err != nil {
		return nil, nil, apperrors.WrapInternal(err, "wasm instantiate")
	}
	defer inst.Close(ctx)

	// --- 4. Validate exports ---

	memory := inst.ExportedMemory(exportedMemory)
	if memory == nil {
		return nil, nil, apperrors.Newf(apperrors.CodeUnprocessableEntity, "wasm module missing export %q", exportedMemory)
	}
	mainFn := inst.ExportedFunction(exportedMain)
	if mainFn == nil {
		return nil, nil, apperrors.Newf(apperrors.CodeUnprocessableEntity, "wasm module missing export %q", exportedMain)
	}

	// --- 5. Write input JSON into linear memory (at offset 0) ---
	if err := writeInput(ctx, memory, input); err != nil {
		return nil, nil, err
	}

	// --- 6. Invoke main with timeout ---
	//
	// Iteration 3 confirms: WithCloseOnContextDone(true) + context.WithTimeout
	// terminates tight WASM loops at every instruction boundary (not just
	// host calls or memory ops). ADR-029 iteration 2-3 tests both scenarios.

	// BE-D-21：guest 单线程执行，wall-clock≈CPU 时间；CPULimitMs 与 Timeout
	// 取较小值作为调用截止，确保 CPU 预算配置真实生效。
	timeout := opts.Timeout
	if cpu := time.Duration(opts.CPULimitMs) * time.Millisecond; cpu > 0 && cpu < timeout {
		timeout = cpu
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	results, callErr := mainFn.Call(runCtx, 0, uint64(len(input)))

	if runCtx.Err() != nil {
		return nil, nil, apperrors.WrapInternal(runCtx.Err(), "wasm invocation interrupted")
	}
	if callErr != nil {
		slog.ErrorContext(runCtx, "wasm-executor guest trap", "error", callErr)
		return nil, nil, apperrors.WrapInternal(callErr, "wasm execution error")
	}

	// --- 7. Business deny takes priority over output ---
	if hostState.denyRequested {
		return nil, errors.New(hostState.denyReason), nil
	}

	// --- 8. Decode output: prefer env.write_output over return value ---
	if hostState.writeOutputSet {
		data := readOutputFromMemory(memory, hostState.writeOutputPtr, hostState.writeOutputLen)
		return data, nil, nil
	}

	if len(results) == 0 || results[0] == 0 {
		return nil, nil, nil
	}

	ret := results[0]
	outPtr := uint32(ret >> 32)
	outLen := uint32(ret & 0xFFFFFFFF)

	if outLen == 0 {
		return nil, nil, nil
	}

	data := readOutputFromMemory(memory, outPtr, outLen)
	return data, nil, nil
}

// memoryLimitPages converts MB → Wasm pages (64 KiB each).
func memoryLimitPages(mb int) uint32 {
	if mb <= 0 {
		mb = 64
	}
	pages := uint32((mb * 1024 * 1024) / memoryPageBytes)
	if pages < 1 {
		pages = 1
	}
	return pages
}

func writeInput(ctx context.Context, mem api.Memory, input []byte) error {
	if len(input) == 0 || ctx.Err() != nil {
		return nil
	}
	if !mem.Write(0, input) {
		return apperrors.NewWithMeta(apperrors.CodeUnprocessableEntity, "wasm input exceeds module linear memory",
			map[string]any{"inputBytes": len(input), "memoryBytes": mem.Size()})
	}
	return nil
}

func readOutputFromMemory(mem api.Memory, ptr, length uint32) []byte {
	if length == 0 {
		return nil
	}
	buf, ok := mem.Read(ptr, length)
	if !ok {
		return nil
	}
	out := make([]byte, len(buf))
	copy(out, buf)
	return out
}

// compile-time interface satisfaction
var (
	_ Executor = (*WASMExecutor)(nil)
)

// compile-time sanity — reference os so unused-import errors surface early
var _ = os.DevNull
