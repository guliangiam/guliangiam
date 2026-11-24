import "@testing-library/jest-dom/vitest";
import { afterAll } from "vitest";
import { loadLanguageResources } from "@/shared/i18n";

// The app now bundles only zh-CN statically and loads every other locale (en
// included) as an async chunk before first paint. Tests assert English copy, so
// preload en once per environment before any test module runs.
await loadLanguageResources("en");

// antd's form error list debounces state updates through
// `@rc-component/util`'s `useDelayState`, which schedules a `setTimeout(…, 10)`
// and (as of @rc-component/util 1.13.0) never clears it on unmount. When such a
// timer is still pending as vitest tears the jsdom environment down,
// react-dom's `resolveUpdatePriority` runs without `window` and the run fails
// with an unhandled `ReferenceError: window is not defined`, even though every
// assertion passed.
//
// Flushing one macrotask at the end of each test file lets those timers fire
// while the environment is still alive. Keep this in `afterAll` (once per file)
// rather than `afterEach` (~1200 tests) to keep the suite fast.
afterAll(async () => {
  await new Promise((resolve) => setTimeout(resolve, 20));
});

// Ensure localStorage is available (Node.js 22+ may not provide it by default)
if (typeof globalThis.localStorage === "undefined") {
  const store = new Map<string, string>();
  globalThis.localStorage = {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => store.set(key, String(value)),
    removeItem: (key: string) => store.delete(key),
    clear: () => store.clear(),
    get length() {
      return store.size;
    },
    key: (_index: number) => null,
  } as unknown as Storage;
}

globalThis.ResizeObserver = class ResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
};

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});
