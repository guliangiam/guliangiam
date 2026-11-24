import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useGlDraft } from "./useGlDraft";

// Mock FormInstance — antd Form 内部实现复杂，我们只需要 hook 会调用的 API
function createMockForm() {
  return {
    setFieldsValue: vi.fn<(values: Record<string, unknown>) => void>(),
    isFieldsTouched: vi.fn<() => boolean>(() => false),
    getFieldValue: vi.fn<(name: string) => unknown>(),
    getFieldsValue: vi.fn<() => Record<string, unknown>>(),
    resetFields: vi.fn<() => void>(),
    validateFields: vi.fn<() => Promise<unknown>>(),
  };
}

function readLocal(key: string): string | null {
  return localStorage.getItem(key);
}

function writeLocal(key: string, value: string) {
  localStorage.setItem(key, value);
}

function clearAllDrafts() {
  // clean all gl:draft: keys between tests
  const toRemove: string[] = [];
  for (let i = 0; i < localStorage.length; i++) {
    const k = localStorage.key(i);
    if (k?.startsWith("gl:draft:")) toRemove.push(k);
  }
  toRemove.forEach((k) => localStorage.removeItem(k));
}

// Use fake timers to control the draft debounce. Restore real timers at the
// very end (afterAll runs before test-setup.ts's afterAll, which awaits a real
// 20ms setTimeout — so we flip back to real timers here).
beforeAll(() => {
  vi.useFakeTimers();
});

afterEach(() => {
  clearAllDrafts();
  vi.clearAllMocks();
  vi.clearAllTimers();
});

afterAll(() => {
  // Drain any remaining fake-timer callbacks, then restore real timers so
  // test-setup.ts's afterAll (which awaits setTimeout(resolve, 20)) can fire.
  vi.runAllTimers();
  vi.useRealTimers();
});

describe("useGlDraft", () => {
  const BASE_OPTS = { route: "user", entityId: "u-42", debounceMs: 50 };
  const DRAFT_KEY = "gl:draft:user:u-42";

  it("reads a saved draft from localStorage and fills the form on mount", () => {
    const savedValues = { name: "Ada", email: "ada@example.com" };
    writeLocal(DRAFT_KEY, JSON.stringify({ v: 1, values: savedValues, savedAt: 1700000000000 }));

    const form = createMockForm();
    const { result } = renderHook(() => useGlDraft(form as never, BASE_OPTS));

    // Effect runs synchronously with fake timers
    expect(form.setFieldsValue).toHaveBeenCalledWith(savedValues);
    expect(result.current.hasDraft).toBe(true);
    expect(result.current.lastSavedAt).toBe(1700000000000);
    expect(result.current.dirty).toBe(false);
  });

  it("discards drafts whose version does not match the hook's version", () => {
    writeLocal(DRAFT_KEY, JSON.stringify({ v: 99, values: { name: "stale" }, savedAt: 1 }));

    const form = createMockForm();
    renderHook(() => useGlDraft(form as never, { ...BASE_OPTS, version: 1 }));

    expect(form.setFieldsValue).not.toHaveBeenCalled();
    // stale entry should have been removed
    expect(readLocal(DRAFT_KEY)).toBeNull();
  });

  it("silently discards corrupt JSON on mount", () => {
    writeLocal(DRAFT_KEY, "{not valid json!!!");

    const form = createMockForm();
    expect(() => {
      renderHook(() => useGlDraft(form as never, BASE_OPTS));
    }).not.toThrow();

    expect(form.setFieldsValue).not.toHaveBeenCalled();
    expect(readLocal(DRAFT_KEY)).toBeNull();
  });

  it("silently discards non-object envelopes on mount", () => {
    writeLocal(DRAFT_KEY, JSON.stringify("a plain string, not an object"));

    const form = createMockForm();
    expect(() => {
      renderHook(() => useGlDraft(form as never, BASE_OPTS));
    }).not.toThrow();

    expect(form.setFieldsValue).not.toHaveBeenCalled();
    expect(readLocal(DRAFT_KEY)).toBeNull();
  });

  it("writes a debounced draft to localStorage on bindValuesChange", () => {
    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, debounceMs: 50 }),
    );

    expect(result.current.dirty).toBe(false);

    act(() => {
      result.current.bindValuesChange({}, { name: "Ada", email: "ada@x.com" });
    });

    // dirty flips immediately
    expect(result.current.dirty).toBe(true);
    // but localStorage not written yet (debounce pending)
    expect(readLocal(DRAFT_KEY)).toBeNull();

    // tick past debounce
    act(() => {
      vi.advanceTimersByTime(60);
    });

    const stored = readLocal(DRAFT_KEY);
    expect(stored).not.toBeNull();
    const env = JSON.parse(stored!);
    expect(env.v).toBe(1);
    expect(env.values).toEqual({ name: "Ada", email: "ada@x.com" });
    expect(typeof env.savedAt).toBe("number");
  });

  it("debounces rapidly succeeding value changes — only the last one persists", () => {
    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, debounceMs: 50 }),
    );

    // Rapid sequence
    act(() => result.current.bindValuesChange({}, { a: 1 }));
    act(() => result.current.bindValuesChange({}, { a: 2 }));
    act(() => result.current.bindValuesChange({}, { a: 3 }));

    // 40ms in — still pending
    act(() => {
      vi.advanceTimersByTime(40);
    });
    expect(readLocal(DRAFT_KEY)).toBeNull();

    // 20ms more → 60ms since last change → writes
    act(() => {
      vi.advanceTimersByTime(20);
    });

    const env = JSON.parse(readLocal(DRAFT_KEY)!);
    expect(env.values).toEqual({ a: 3 });
  });

  it("clearDraft removes the stored draft and resets all state", () => {
    // pre-populate localStorage
    writeLocal(DRAFT_KEY, JSON.stringify({ v: 1, values: { name: "x" }, savedAt: 1 }));

    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, debounceMs: 50 }),
    );

    // produce dirty state + saved draft first
    act(() => result.current.bindValuesChange({}, { name: "y" }));
    act(() => vi.advanceTimersByTime(60));

    act(() => result.current.clearDraft());

    expect(result.current.hasDraft).toBe(false);
    expect(result.current.dirty).toBe(false);
    expect(result.current.lastSavedAt).toBeNull();
    expect(readLocal(DRAFT_KEY)).toBeNull();
  });

  it("disabled mode (enable=false) never touches localStorage but still tracks dirty", () => {
    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, enable: false, debounceMs: 50 }),
    );

    act(() => result.current.bindValuesChange({}, { name: "y" }));
    act(() => vi.advanceTimersByTime(200)); // well past debounce

    expect(result.current.dirty).toBe(true);
    // still no key written
    expect(readLocal(DRAFT_KEY)).toBeNull();
  });

  it("no entityId → draft disabled (returns null key, no persistence)", () => {
    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { route: "orphan", debounceMs: 50 }),
    );

    act(() => result.current.bindValuesChange({}, { foo: "bar" }));
    act(() => vi.advanceTimersByTime(200));

    expect(result.current.dirty).toBe(true);
    // no key at all
    const hasAny = Array.from({ length: localStorage.length }, (_, i) => localStorage.key(i)).some(
      (k) => k?.startsWith("gl:draft:"),
    );
    expect(hasAny).toBe(false);
  });

  it("wrapping the draft write in try/catch — localStorage.setItem throw is swallowed", () => {
    const originalSet = localStorage.setItem.bind(localStorage);
    localStorage.setItem = vi.fn<(key: string, value: string) => void>(() => {
      throw new Error("Quota exceeded");
    });

    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, debounceMs: 50 }),
    );

    // should not throw
    expect(() => {
      act(() => result.current.bindValuesChange({}, { name: "boom" }));
      act(() => vi.advanceTimersByTime(60));
    }).not.toThrow();

    // dirty still tracks even though write failed
    expect(result.current.dirty).toBe(true);

    localStorage.setItem = originalSet;
  });

  it("explicit draftKey overrides route+entityId derivation", () => {
    const CUSTOM_KEY = "myapp:draft:custom";
    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, {
        route: "ignored",
        entityId: "ignored-too",
        draftKey: CUSTOM_KEY,
        debounceMs: 50,
      }),
    );

    act(() => result.current.bindValuesChange({}, { x: 1 }));
    act(() => vi.advanceTimersByTime(60));

    expect(readLocal(CUSTOM_KEY)).not.toBeNull();
    expect(readLocal("gl:draft:ignored:ignored-too")).toBeNull();
  });

  it("autoRestore=false skips mount-time restoration", () => {
    writeLocal(DRAFT_KEY, JSON.stringify({ v: 1, values: { name: "skip" }, savedAt: 1 }));

    const form = createMockForm();
    renderHook(() => useGlDraft(form as never, { ...BASE_OPTS, autoRestore: false }));

    expect(form.setFieldsValue).not.toHaveBeenCalled();
  });

  it("mount with null/undefined form still returns a result (no crash)", () => {
    writeLocal(DRAFT_KEY, JSON.stringify({ v: 1, values: { name: "test" }, savedAt: 1 }));

    const { result } = renderHook(() => useGlDraft(null as never, BASE_OPTS));

    // no throw on mount-time restore path (guarded by `if (!form) return`)
    expect(result.current.hasDraft).toBe(false);
    expect(result.current.dirty).toBe(false);
  });

  it("unmount clears the pending debounce timer so a late write cannot happen", () => {
    const form = createMockForm();
    const { result, unmount } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, debounceMs: 50 }),
    );

    act(() => result.current.bindValuesChange({}, { a: 1 }));
    // unmount BEFORE debounce fires
    unmount();

    act(() => vi.advanceTimersByTime(500));

    // timer flushed on unmount → nothing persisted
    expect(readLocal(DRAFT_KEY)).toBeNull();
  });

  it("never persists credential-like fields (secret/password/*Key) by default", () => {
    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, debounceMs: 50 }),
    );

    act(() =>
      result.current.bindValuesChange(
        {},
        {
          name: "p1",
          clientSecret: "s3cret",
          password: "pw",
          sshPassword: "ssh-pw",
          apiKey: "ak",
          privateKey: "pk",
          passwordSalt: "salt",
          defaultPassword: "default-pw",
          masterPassword: "master-pw",
          masterVerificationCode: "master-code",
          nested: { displayName: "ok", appSecret: "nested-secret" },
        },
      ),
    );
    act(() => vi.advanceTimersByTime(60));

    const env = JSON.parse(readLocal(DRAFT_KEY)!);
    expect(env.values).toEqual({ name: "p1", nested: { displayName: "ok" } });
    expect(readLocal(DRAFT_KEY)).not.toContain("s3cret");
    expect(readLocal(DRAFT_KEY)).not.toContain("nested-secret");
    expect(readLocal(DRAFT_KEY)).not.toContain("master-code");
  });

  it("strips credentials when restoring a legacy draft and rewrites storage sanitized", () => {
    writeLocal(
      DRAFT_KEY,
      JSON.stringify({
        v: 1,
        values: { name: "Ada", clientSecret: "legacy-plain" },
        savedAt: 1700000000000,
      }),
    );

    const form = createMockForm();
    const { result } = renderHook(() => useGlDraft(form as never, BASE_OPTS));

    expect(form.setFieldsValue).toHaveBeenCalledWith({ name: "Ada" });
    expect(result.current.hasDraft).toBe(true);
    expect(readLocal(DRAFT_KEY)).not.toContain("legacy-plain");
    const env = JSON.parse(readLocal(DRAFT_KEY)!);
    expect(env.values).toEqual({ name: "Ada" });
    expect(env.savedAt).toBe(1700000000000);
  });

  it("explicit ignoreKeys still apply on top of the default credential filter", () => {
    const form = createMockForm();
    const { result } = renderHook(() =>
      useGlDraft(form as never, { ...BASE_OPTS, debounceMs: 50, ignoreKeys: ["revision"] }),
    );

    act(() => result.current.bindValuesChange({}, { name: "x", revision: 7, password: "pw" }));
    act(() => vi.advanceTimersByTime(60));

    const env = JSON.parse(readLocal(DRAFT_KEY)!);
    expect(env.values).toEqual({ name: "x" });
  });
});
