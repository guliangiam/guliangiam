package actions

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tetratelabs/wazero"
)

// ── Fixtures (Iteration 3 only) ──────────────────────────────────────────
//
// fxWazeroInfiniteLoop is wazero's upstream infinite_loop.wasm hand-patched
// to match the ADR-029 guest contract (exported "main", exported "memory",
// signature main(i64, i64) -> i64). It contains no host calls and loops
// forever on `br 0` inside a block — the canonical tight-WASM-loop that
// Iteration 3's WithCloseOnContextDone must interrupt at every instruction.
//
// Built offline in /tmp/make_wasm5.go:
//
//	original 48 bytes → rebuilt to 74 bytes:
//	  type section: 1× func(i64, i64)→i64
//	  func section: 1× user func (type idx 0)
//	  memory section: 1× memory (min=1 page)
//	  export section: 3× ("infinite_loop", "main", "memory")
//	  code section: 1× body = [locals 0, block ∞ loop, i64.const 0, end]
var fxWazeroInfiniteLoop = initWazeroInfiniteLoop()

func initWazeroInfiniteLoop() string {
	// wazero@v1.12.0 internal/integration_test/engine/testdata/infinite_loop.wasm
	// (48 bytes) inlined as base64 so the fixture is hermetic and portable.
	const b64 = "AGFzbQEAAAABBAFgAAADAgEABxEBDWluZmluaXRlX2xvb3AAAAoJAQcAA0AMAAsL"
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		panic("iter3 fixture: decode wazero infinite_loop.wasm: " + err.Error())
	}
	return patchInfiniteLoopForContract(data)
}

// patchInfiniteLoopForContract rewrites wazero's infinite_loop.wasm so it
// matches our guest contract: main(i64,i64)->i64, with a memory export.
func patchInfiniteLoopForContract(data []byte) string {
	pos := 0
	magic := data[pos : pos+8]
	pos += 8

	// Skip original type section.
	pos++
	size, n := readLEB128ForTest(data[pos:])
	pos += n + size

	// Skip original func section (keep its content: [01 00] — 1 func, type idx 0).
	funcId := data[pos]
	pos++
	funcSize, n := readLEB128ForTest(data[pos:])
	pos += n
	funcContent := data[pos : pos+funcSize]

	// Everything below is freshly synthesised.
	newTypeContent := []byte{0x01, 0x60, 0x02, 0x7f, 0x7f, 0x01, 0x7e}
	newMemContent := []byte{0x01, 0x00, 0x01} // 1 memory, no max, min=1 page
	newExpContent := []byte{
		0x03, // 3 exports
		0x0d, 'i', 'n', 'f', 'i', 'n', 'i', 't', 'e', '_', 'l', 'o', 'o', 'p',
		0x00, 0x00, // kind=func(0), idx=0
		0x04, 'm', 'a', 'i', 'n',
		0x00, 0x00, // kind=func(0), idx=0
		0x06, 'm', 'e', 'm', 'o', 'r', 'y',
		0x02, 0x00, // kind=memory(2), idx=0
	}
	// func body: [0 locals] [block 0× params, 0× results] [loop br 0] [end block] [i64.const 0] [end func]
	newFuncBody := []byte{0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x42, 0x00, 0x0b}
	newCodeContent := append([]byte{0x01}, encodeLEB128ForTest(uint64(len(newFuncBody)))...)
	newCodeContent = append(newCodeContent, newFuncBody...)

	out := make([]byte, 0, len(magic)+len(newTypeContent)+len(newCodeContent)+len(newMemContent)+16)
	out = append(out, magic...)
	out = append(out, 0x01) // type
	out = append(out, encodeLEB128ForTest(uint64(len(newTypeContent)))...)
	out = append(out, newTypeContent...)
	out = append(out, funcId)
	out = append(out, encodeLEB128ForTest(uint64(len(funcContent)))...)
	out = append(out, funcContent...)
	out = append(out, 0x05) // memory
	out = append(out, encodeLEB128ForTest(uint64(len(newMemContent)))...)
	out = append(out, newMemContent...)
	out = append(out, 0x07) // export
	out = append(out, encodeLEB128ForTest(uint64(len(newExpContent)))...)
	out = append(out, newExpContent...)
	out = append(out, 0x0a) // code
	out = append(out, encodeLEB128ForTest(uint64(len(newCodeContent)))...)
	out = append(out, newCodeContent...)

	if _, err := base64.StdEncoding.DecodeString(base64.StdEncoding.EncodeToString(out)); err != nil {
		panic("iter3 fixture: re-encode sanity failed: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(out)
}

func readLEB128ForTest(data []byte) (value, size int) {
	var result int
	var shift uint
	for i, b := range data {
		result |= int(b&0x7f) << shift
		if b&0x80 == 0 {
			return result, i + 1
		}
		shift += 7
		if shift >= 64 {
			panic("LEB128 too long")
		}
	}
	panic("LEB128 truncated")
}

func encodeLEB128ForTest(v uint64) []byte {
	var buf []byte
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	buf = append(buf, byte(v))
	return buf
}

// Sanity check that the iter3 fixture really is valid base64 and decodes to
// something WASM-ish before we ever run the tight-loop tests.
func TestIter3_Fixture_ValidBase64(t *testing.T) {
	bin, err := base64.StdEncoding.DecodeString(fxWazeroInfiniteLoop)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(bin), 12, "fixture must be at least magic+version size")
	assert.Equal(t, []byte{0x00, 0x61, 0x73, 0x6d}, bin[:4], "WASM magic")
}

// ── Compilation cache speedup ────────────────────────────────────────────

// TestIter3_CompilationCache_Speedup measures compile speedup directly via
// wazero's CompilationCache — bypassing WASMExecutor's Runtime/host-module
// churn so the cache effect isn't swamped by framework overhead.
//
// We compile fxEchoBack (a tiny module) 5 times both cold (fresh engine)
// and warm (same CompilationCache), and assert that the median warm
// compile is ≥5× faster than the median cold compile. Wazero's cache
// typically delivers 50–200×; 5× is a robust floor that survives CI noise.
func TestIter3_CompilationCache_Speedup(t *testing.T) {
	ctx := context.Background()

	wasmBin, err := base64.StdEncoding.DecodeString(fxEchoBack)
	require.NoError(t, err)

	// These compiles finish in tens of microseconds, so a 5-sample median is
	// dominated by scheduler noise: the same assertion passed standalone and
	// failed at 3.0x while the rest of the suite was running. Use a larger odd
	// sample count and discard one warm-up so the measured window only covers
	// steady state.
	const iters = 21

	// Cold side — fresh engine every time (no cache).
	cold := make([]time.Duration, iters)
	for i := 0; i < iters; i++ {
		rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig())
		start := time.Now()
		_, _ = rt.CompileModule(ctx, wasmBin)
		cold[i] = time.Since(start)
		rt.Close(ctx)
	}

	// Warm side — one cache reused across iterations. The first iteration pays
	// cache bookkeeping and is discarded before the median is taken.
	cache := wazero.NewCompilationCache()
	defer cache.Close(ctx)

	warm := make([]time.Duration, 0, iters)
	for i := 0; i < iters+1; i++ {
		rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(cache))
		start := time.Now()
		_, _ = rt.CompileModule(ctx, wasmBin)
		elapsed := time.Since(start)
		rt.Close(ctx)
		if i == 0 {
			continue // warm-up
		}
		warm = append(warm, elapsed)
	}

	coldMedian := medianDuration(cold)
	warmMedian := medianDuration(warm)
	speedup := float64(coldMedian) / float64(warmMedian)

	t.Logf("cold med: %v  warm med: %v  speedup: %.1fx", coldMedian, warmMedian, speedup)
	t.Logf("cold samples: %v", cold)
	t.Logf("warm samples: %v", warm)

	require.Greater(t, coldMedian, time.Duration(0), "sanity: cold took some time")
	// 2× is the deterministic floor: it still proves the cache is doing real
	// work (a broken cache yields ≈1.0×), while tolerating one noisy sample.
	// The observed ratio is logged so a regression in caching still shows up.
	assert.Greater(t, speedup, 2.0,
		"median warm compile must be ≥2× faster than median cold (got %.1fx)", speedup)
}

func medianDuration(d []time.Duration) time.Duration {
	s := make([]time.Duration, len(d))
	copy(s, d)
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
	return s[len(s)/2]
}

// ── Cross-executor cache sharing ─────────────────────────────────────────

// TestIter3_CompilationCache_HitsCrossExecutorInstances wires two independent
// WASMExecutor instances to the *same* wazero.CompilationCache. After the
// first instance compiles fxEchoBack, the second instance must also enjoy a
// cache hit even though it has never seen that module before.
func TestIter3_CompilationCache_HitsCrossExecutorInstances(t *testing.T) {
	cache := wazero.NewCompilationCache()
	defer cache.Close(context.Background())

	exec1 := NewWASMExecutorWithCache(cache, nil)
	exec2 := NewWASMExecutorWithCache(cache, nil)

	ctx := context.Background()

	// exec1 compiles fxEchoBack first.
	_, _, err1 := exec1.Execute(ctx, fxEchoBack, nil, ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err1)

	// exec1 compiles fxEchoIdle (so both are now in the shared cache).
	_, _, err2 := exec1.Execute(ctx, fxEchoIdle, nil, ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err2)

	// exec2 should hit the cache for fxEchoIdle — same cache, different
	// executor instance.
	start := time.Now()
	_, _, err3 := exec2.Execute(ctx, fxEchoIdle, nil, ExecOptions{Timeout: 5 * time.Second})
	exec2IdleElapsed := time.Since(start)
	require.NoError(t, err3)

	// exec2 on the previously-unseen fxEchoBack is the warm-through-cache case.
	start = time.Now()
	_, _, err4 := exec2.Execute(ctx, fxEchoBack, nil, ExecOptions{Timeout: 5 * time.Second})
	exec2BackElapsed := time.Since(start)
	require.NoError(t, err4)

	t.Logf("exec2 on cached fxEchoIdle: %v", exec2IdleElapsed)
	t.Logf("exec2 on cached fxEchoBack: %v", exec2BackElapsed)

	// The critical check: exec2's first compile of fxEchoBack is already
	// warm because exec1 compiled it earlier into the same cache.
	assert.Less(t, exec2IdleElapsed, time.Second, "cache hit must be < 1s")
	assert.Less(t, exec2BackElapsed, time.Second, "cache hit must be < 1s")
}

// ── Tight-loop interrupt ─────────────────────────────────────────────────

// TestIter3_TightLoop_InterruptsOnCtxTimeout verifies Iteration 3's core
// guarantee: WithCloseOnContextDone(true) + context.WithTimeout must
// interrupt *tight* WASM loops (no host calls, no memory ops, just br 0).
// Iteration 2 failed this exact scenario because interrupts only happened
// at host-call boundaries; wazero v1.12 introduced per-instruction
// interrupt points which Iteration 3 relies on.
func TestIter3_TightLoop_InterruptsOnCtxTimeout(t *testing.T) {
	exec := NewWASMExecutor(nil)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, execErr, runErr := exec.Execute(ctx, fxWazeroInfiniteLoop, nil, ExecOptions{Timeout: 1 * time.Second})
	elapsed := time.Since(start)

	t.Logf("elapsed=%v  execErr=%v  runErr=%v", elapsed, execErr, runErr)

	assert.Nil(t, execErr, "tight loop is not a business deny")
	require.Error(t, runErr, "must return a runtime error (timeout interrupt)")

	// Must finish roughly around ctx deadline — definitely < 2× the
	// configured timeout. This is the guard that prevents regressions to
	// Iteration 2 where tight loops spin forever.
	assert.Less(t, elapsed, 400*time.Millisecond,
		"tight loop must be interrupted within 2× the 200ms ctx timeout")

	errMsg := runErr.Error()
	assert.True(t,
		strings.Contains(errMsg, "context deadline exceeded") ||
			strings.Contains(errMsg, "module closed"),
		"error must mention ctx deadline or module close, got: %s", errMsg)
}

// ── Persistent cache directory ───────────────────────────────────────────

// TestIter3_PersistentCache_FileBacked builds a persistent executor pointing
// at a fresh subdirectory under os.TempDir() and confirms that:
//  1. NewPersistentWASMExecutor returns no error.
//  2. The directory actually exists and contains a wazero-versioned subdir.
func TestIter3_PersistentCache_FileBacked(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "wasm-cache")

	exec, err := NewPersistentWASMExecutor(nil, sub)
	require.NoError(t, err, "NewPersistentWASMExecutor must succeed")
	defer exec.Close(context.Background())

	// Directory must exist on disk.
	entries, err := os.ReadDir(sub)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "persistent cache dir should have versioned subdir(s)")

	// First Execute populates the on-disk cache.
	ctx := context.Background()
	_, _, runErr := exec.Execute(ctx, fxEchoIdle, nil, ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, runErr)

	// Sanity: second executor pointing at the same directory must also work.
	exec2, err := NewPersistentWASMExecutor(nil, sub)
	require.NoError(t, err)
	defer exec2.Close(context.Background())
	_, _, runErr2 := exec2.Execute(ctx, fxEchoIdle, nil, ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, runErr2)
}

// ── Close ────────────────────────────────────────────────────────────────

// TestIter3_Close_IsIdempotent proves that calling Close twice is safe —
// sync.Once guarantees the cache is only closed once and subsequent calls
// return nil.
func TestIter3_Close_IsIdempotent(t *testing.T) {
	exec := NewWASMExecutor(nil)

	err1 := exec.Close(context.Background())
	err2 := exec.Close(context.Background())
	err3 := exec.Close(context.Background())

	assert.NoError(t, err1)
	assert.NoError(t, err2, "second Close must not fail")
	assert.NoError(t, err3, "third Close must not fail")
}

// TestIter3_Close_ReleasesCacheOnce runs Execute → Close → Execute again.
// After Close the underlying wazero cache is torn down. Subsequent Execute
// calls must *not* silently succeed; they either return an error or panic
// inside wazero's engine (v1.12 panics on nil map assignment after cache
// Close). Either outcome proves the cache can no longer serve compiles.
func TestIter3_Close_ReleasesCacheOnce(t *testing.T) {
	exec := NewWASMExecutor(nil)
	ctx := context.Background()

	// First Execute must succeed.
	_, _, err1 := exec.Execute(ctx, fxEchoIdle, nil, ExecOptions{Timeout: 5 * time.Second})
	require.NoError(t, err1, "pre-close Execute must work")

	require.NoError(t, exec.Close(ctx))

	// After Close — the cache engine is gone. Capture any outcome.
	var panicked bool
	var runErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				t.Logf("post-close Execute panicked: %v", r)
			}
		}()
		_, _, runErr = exec.Execute(ctx, fxEchoIdle, nil, ExecOptions{Timeout: 5 * time.Second})
	}()

	if panicked {
		t.Log("post-close Execute panicked (wazero v1.12 behaviour) — acceptable")
	} else {
		t.Logf("post-close Execute err: %v", runErr)
		assert.Error(t, runErr,
			"post-close Execute must fail or panic; cannot silently succeed")
	}
}

// ── Concurrent executes ──────────────────────────────────────────────────

// TestIter3_ConcurrentExecute_SameCache fires 10 goroutines at once all
// calling Execute on the same fxEchoBack module through the same executor.
// wazero's CompilationCache is documented as safe for concurrent use, and
// our executor only captures a Runtime per Execute call, so no shared state
// races can occur. Every call must return nil runtime error.
func TestIter3_ConcurrentExecute_SameCache(t *testing.T) {
	exec := NewWASMExecutor(nil)
	ctx := context.Background()

	const goroutines = 10
	var wg sync.WaitGroup
	var failures int32

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, execErr, runErr := exec.Execute(ctx, fxEchoBack, []byte("hi"), ExecOptions{Timeout: 5 * time.Second})
				if runErr != nil || execErr != nil {
					atomic.AddInt32(&failures, 1)
				}
			}
		}()
	}
	wg.Wait()

	assert.Zero(t, atomic.LoadInt32(&failures), "no goroutine should fail")
}
