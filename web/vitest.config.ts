import { configDefaults, defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
    // Unit tests live under src/ only. Playwright specs in e2e/ and Cypress
    // specs in cypress/ must NOT be collected by vitest — they use different
    // globals (page/test from @playwright/test) and would fail at collection.
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    exclude: [...configDefaults.exclude, "e2e/**", "cypress/**", "storybook-static/**"],

    // A unit test that needs longer than this is a defect worth seeing, so the
    // ceiling stays low enough to catch a hang — but it is deliberately not the
    // 5s default.
    //
    // The suite is 77 files, each a fresh jsdom environment that re-runs
    // `test-setup.ts`; the workers that do that in parallel contend for CPU with
    // each other and with whatever else is on the machine. On a loaded host the
    // transform/setup queue alone pushed plain render tests past 5s, which made
    // the suite fail ~2 tests per run at *different* files every time —
    // `AccountPage`, then `SettingsPage`, then `ApplicationPreviewPanel`. Each
    // one passed in isolation. At that point the timeout was measuring the queue,
    // not the test.
    testTimeout: 15_000,
    hookTimeout: 15_000,

    // Cap the parallelism so the transform storm cannot starve the tests it is
    // preparing. Measured on an 8-core host: 8 workers finish a green suite in
    // ~30s, 4 workers in ~45s — the trade is throughput for a result that does
    // not depend on what else the machine is doing.
    maxWorkers: 4,
  },
});
