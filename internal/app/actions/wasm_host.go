package actions

import (
	"context"
	"math"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

// u32Stack safely converts a wazero uint64 stack slot to uint32, rejecting
// values that exceed i32 range (CWE-190 / gosec G115). WASM i32 operands
// are zero-extended to uint64 on the stack, so any value > math.MaxUint32
// indicates a type mismatch or maliciously crafted guest.
func u32Stack(v uint64) (uint32, bool) {
	if v > math.MaxUint32 {
		return 0, false
	}
	return uint32(v), true
}

// hostState carries per-invocation host function state. A fresh instance is
// constructed each Execute so concurrent calls don't clobber each other.
type hostState struct {
	denyRequested bool
	denyReason    string

	// writeOutputPtr/Len record the last env.write_output call.
	// If set, Execute() uses this output; if not, it falls back to decoding
	// the main() return value as (ptr << 32 | len).
	writeOutputPtr uint32
	writeOutputLen uint32
	writeOutputSet bool

	proxy        OutboundProxy
	allowNetwork bool
	allowedHosts []string
}

// buildHostModule instantiates the env namespace. All functions are in the
// "env" import namespace — matches what our fixture WASM modules import.
//
//	void env.deny(i32 reason_ptr, i32 reason_len)       — business deny
//	void env.write_output(i32 ptr, i32 len)              — guest tells host where output lives
//	i32  env.http_get(i32 url_ptr, i32 url_len, i32 resp_ptr, i32 resp_max)
//	i32  env.http_post(i32 url_ptr, i32 url_len,
//	                    i32 body_ptr, i32 body_len,
//	                    i32 resp_ptr, i32 resp_max)
func buildHostModule(ctx context.Context, rt wazero.Runtime, hs *hostState) (api.Module, error) {
	builder := rt.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(envDeny(hs)),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{}).
		Export("deny").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(envWriteOutput(hs)),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{}).
		Export("write_output").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(envHTTPGet(hs)),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		Export("http_get").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(envHTTPPost(hs)),
			[]api.ValueType{api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32, api.ValueTypeI32},
			[]api.ValueType{api.ValueTypeI32}).
		Export("http_post")

	return builder.Instantiate(ctx)
}

// ── env.deny ──────────────────────────────────────────────────────────────

func envDeny(hs *hostState) func(context.Context, api.Module, []uint64) {
	return func(_ context.Context, mod api.Module, stack []uint64) {
		if hs.denyRequested {
			return
		}
		// CWE-190: reject stack values outside i32 range.
		reasonPtr, ok := u32Stack(stack[0])
		if !ok {
			return
		}
		reasonLen, ok := u32Stack(stack[1])
		if !ok {
			return
		}
		reason := readString(mod, reasonPtr, reasonLen)
		if reason == "" {
			reason = "guest denied extension point"
		}
		hs.denyRequested = true
		hs.denyReason = reason
	}
}

// ── env.write_output ──────────────────────────────────────────────────────
// Guest tells the host where to find its JSON output. This is the primary
// output channel for MVP WASM modules (the main() return-value encoding
// (ptr<<32|len) is only needed for the simplest possible guest modules that
// don't import env.write_output).

func envWriteOutput(hs *hostState) func(context.Context, api.Module, []uint64) {
	return func(_ context.Context, _ api.Module, stack []uint64) {
		// CWE-190: reject stack values outside i32 range.
		ptr, ok1 := u32Stack(stack[0])
		ln, ok2 := u32Stack(stack[1])
		if !ok1 || !ok2 {
			return
		}
		hs.writeOutputPtr = ptr
		hs.writeOutputLen = ln
		hs.writeOutputSet = true
	}
}

// ── env.http_get ──────────────────────────────────────────────────────────

func envHTTPGet(hs *hostState) func(context.Context, api.Module, []uint64) {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		// CWE-190: reject stack values outside i32 range.
		urlPtr, ok1 := u32Stack(stack[0])
		urlLen, ok2 := u32Stack(stack[1])
		respPtr, ok3 := u32Stack(stack[2])
		respMax, ok4 := u32Stack(stack[3])
		if !ok1 || !ok2 || !ok3 || !ok4 {
			stack[0] = encodeNegOne()
			return
		}
		urlStr := readString(mod, urlPtr, urlLen)

		if !hs.allowNetwork || !hs.proxy.Allowed(urlHost(urlStr)) {
			stack[0] = encodeNegOne()
			return
		}

		data, err := hs.proxy.Get(ctx, urlStr)
		if err != nil {
			stack[0] = encodeNegOne()
			return
		}
		stack[0] = api.EncodeU32(writeLimited(mod, respPtr, data, respMax))
	}
}

// ── env.http_post ─────────────────────────────────────────────────────────

func envHTTPPost(hs *hostState) func(context.Context, api.Module, []uint64) {
	return func(ctx context.Context, mod api.Module, stack []uint64) {
		// CWE-190: reject stack values outside i32 range.
		urlPtr, ok1 := u32Stack(stack[0])
		urlLen, ok2 := u32Stack(stack[1])
		bodyPtr, ok3 := u32Stack(stack[2])
		bodyLen, ok4 := u32Stack(stack[3])
		respPtr, ok5 := u32Stack(stack[4])
		respMax, ok6 := u32Stack(stack[5])
		if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
			stack[0] = encodeNegOne()
			return
		}
		urlStr := readString(mod, urlPtr, urlLen)
		body := readBytes(mod, bodyPtr, bodyLen)

		if !hs.allowNetwork || !hs.proxy.Allowed(urlHost(urlStr)) {
			stack[0] = encodeNegOne()
			return
		}

		data, err := hs.proxy.Post(ctx, urlStr, body)
		if err != nil {
			stack[0] = encodeNegOne()
			return
		}
		stack[0] = api.EncodeU32(writeLimited(mod, respPtr, data, respMax))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func encodeNegOne() uint64 { return uint64(uint32(0xFFFFFFFF)) }

func readString(mod api.Module, ptr, length uint32) string {
	buf := readBytes(mod, ptr, length)
	if buf == nil {
		return ""
	}
	return string(buf)
}

func readBytes(mod api.Module, ptr, length uint32) []byte {
	if length == 0 {
		return nil
	}
	mem := mod.ExportedMemory(exportedMemory)
	if mem == nil {
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

func writeLimited(mod api.Module, dstPtr uint32, src []byte, dstMax uint32) uint32 {
	if dstMax == 0 || len(src) == 0 {
		return 0
	}
	// 先在 64 位空间比较并截断，随后的收窄转换不可能溢出。
	if uint64(len(src)) > uint64(dstMax) {
		src = src[:dstMax]
	}
	n := uint32(len(src)) //nolint:gosec // G115: len(src) <= dstMax <= math.MaxUint32
	mem := mod.ExportedMemory(exportedMemory)
	if mem == nil {
		return 0
	}
	if !mem.Write(dstPtr, src[:n]) {
		return 0
	}
	return n
}

// urlHost extracts host from a URL without pulling url.Parse into hot paths.
func urlHost(raw string) string {
	for i := 0; i < len(raw); i++ {
		if raw[i] == ':' && i+2 < len(raw) && raw[i+1] == '/' && raw[i+2] == '/' {
			start := i + 3
			for j := start; j < len(raw); j++ {
				c := raw[j]
				if c == '/' || c == '?' || c == '#' {
					return raw[start:j]
				}
			}
			return raw[start:]
		}
	}
	return raw
}
