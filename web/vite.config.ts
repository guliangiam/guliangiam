import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "VITE_");
  const apiTarget = env.VITE_API_TARGET || "http://localhost:8000";

  return {
    plugins: [react()],
    build: {
      chunkSizeWarningLimit: 3000, // Monaco editor.api (~2.7MB) and Scalar (~1.7MB) are lazy-loaded per route; warning threshold raised
      rollupOptions: {
        output: {
          manualChunks(id) {
            // ── Tier 0: React core ecosystem (loaded on every page) ──
            if (
              id.includes("node_modules/react/") ||
              id.includes("node_modules/react-dom/") ||
              id.includes("node_modules/react-router-dom/") ||
              id.includes("node_modules/react-router/")
            ) {
              return "react-vendor";
            }

            // ── Tier 1: i18n core (loaded on every page for translations) ──
            // NOTE: i18n-iso-countries is split out — it's only used by
            // CountryCodeSelect/RegionSelect which are lazy-loaded.
            if (
              id.includes("node_modules/i18next/") ||
              id.includes("node_modules/react-i18next/") ||
              id.includes("node_modules/i18next-browser-languagedetector/")
            ) {
              return "i18n-vendor";
            }

            // ── Tier 1: Data management libraries (loaded on every page) ──
            // NOTE: Zod is split into its own chunk so it can be loaded
            // on-demand by pages that need schema validation, reducing the
            // critical-path data-vendor size.
            if (
              id.includes("node_modules/@tanstack/react-query/") ||
              id.includes("node_modules/zustand/")
            ) {
              return "data-vendor";
            }

            // ── Tier 1: Zod schema validation (split from data-vendor) ──
            // Used by http.ts (critical path) and API modules (lazy pages).
            // Keeping it separate allows better caching and parallel loading.
            if (id.includes("node_modules/zod/")) {
              return "zod-vendor";
            }

            // ── Tier 1: CSS-in-JS runtime (antd v6 uses @emotion) ──
            if (id.includes("node_modules/@emotion/") || id.includes("node_modules/cssinjs/")) {
              return "emotion-vendor";
            }

            // ── Tier 1: Ant Design icons — deliberately NOT grouped ──
            // A manualChunks group for @ant-design/icons ships every icon used
            // anywhere (including lazy CRUD pages) once the first screen imports
            // a single icon. Natural splitting keeps only the icons the initial
            // route imports (V4.67: 449.5 -> 421.4 KB).

            // ── Tier 1: Ant Design components — deliberately NOT grouped ──
            // Entry modules deep-import antd components (check-app-barrel-imports.py)
            // and the static graph has no barrel imports, so rolldown can split antd
            // per component: the first screen only ships components AppShell/Login
            // actually render. Grouping all antd into one chunk (the previous
            // `antd-vendor`) forced every visitor to download Table/Form/DatePicker/
            // Upload used only by lazy CRUD pages (OPEN-PERF-01).

            // ── Tier 1: dayjs (antd's date library, shared across pages) ──
            if (id.includes("node_modules/dayjs/")) {
              return "dayjs-vendor";
            }

            // ── Tier 2: Sentry error tracking (lazy loaded via initSentry) ──
            if (id.includes("node_modules/@sentry/")) {
              return "sentry-vendor";
            }

            // ── Tier 2: Web Vitals (lazy loaded via initWebVitals) ──
            if (id.includes("node_modules/web-vitals/")) {
              return "web-vitals-vendor";
            }

            // ── Tier 2: i18n-iso-countries (lazy loaded via CountryCodeSelect) ──
            if (id.includes("node_modules/i18n-iso-countries/")) {
              return "i18n-countries-vendor";
            }

            // ── Tier 2: Charting library — deliberately NOT grouped ──
            // A manualChunks rule for recharts pulled the whole chart library into
            // the entry static graph (observed 109.8 KB gzip on the first screen).
            // Natural splitting keeps it inside the lazy DashboardPage chunk.

            // ── Tier 2: Monaco editor（与 Scalar 同理，刻意不 manualChunks）──
            // Monaco 只被 lazy 路由里的代码编辑器使用；一旦给它命名 vendor
            // chunk，rolldown 会把它提升进入口静态图（首屏 1127KB，DEF-05）。
            // 让自然分割把它留在按需拉取的动态 chunk 中。

            // ── Tier 2: Web3 libraries (lazy loaded via Web3WalletSelect) ──
            if (id.includes("node_modules/@web3-onboard/") || id.includes("node_modules/ethers/")) {
              return "web3-vendor";
            }

            // ── Tier 2: Scalar API reference ──
            // 刻意不把 `@scalar/*` 归入 manualChunks。`SwaggerPage` 已经是 lazy route，
            // 自然的代码分割会把整个 Scalar 子图放进只在 `/swagger` 才拉取的动态 chunk。
            // 一旦用 manualChunks 给这个子图命名，rolldown 会把它提升进入口的静态图：
            // 实测 `dist/index.html` 出现 `scalar-vendor-*.js` 的 modulepreload，首屏
            // gzip 从 694KB 涨到 1719KB（+1025KB，其中 897KB 是 Scalar 本体）。
            // 见 scripts/check-bundle-budget.mjs 的首屏预算门禁。

            // ── Tier 2: Phone number library (lazy loaded via CountryCodeSelect) ──
            if (id.includes("node_modules/libphonenumber-js/")) {
              return "phone-vendor";
            }

            // ── Tier 2: QR code (lazy loaded via MFA setup pages) ──
            if (id.includes("node_modules/qrcode.react/")) {
              return "qrcode-vendor";
            }

            // ── Tier 2: DOMPurify (used for HTML sanitization) ──
            if (
              id.includes("node_modules/dompurify/") ||
              id.includes("node_modules/is-dompurify/")
            ) {
              return "dom-purify";
            }

            // ── Tier 2: Virtual list (lazy loaded via large data tables) ──
            if (id.includes("node_modules/@tanstack/react-virtual/")) {
              return "virtual-vendor";
            }

            return undefined;
          },
        },
      },
    },
    resolve: {
      alias: {
        "@": path.resolve(import.meta.dirname, "./src"),
      },
    },
    server: {
      port: 7101,
      // 端口被占用时直接失败，而不是静默漂移到 7102：WebAuthn origin 白名单是
      // fail-closed 的，漂移会让注册/登录 finish 统一 422 not an allowed origin。
      strictPort: true,
      headers: {
        // Force every dev response to bypass HTTP cache — Trae preview browser
        // (and normal Chrome) serve stale .vite/deps chunks otherwise.
        "Cache-Control": "no-store, no-cache, must-revalidate",
        Pragma: "no-cache",
        Expires: "0",
      },
      proxy: {
        "/api": {
          target: apiTarget,
          changeOrigin: true,
        },
        "/openapi": {
          target: apiTarget,
          changeOrigin: true,
        },
        "/files": {
          target: apiTarget,
          changeOrigin: true,
        },
        "/.well-known": {
          target: apiTarget,
          changeOrigin: true,
        },
        "/oauth2": {
          target: apiTarget,
          changeOrigin: true,
        },
        "/cas": {
          target: apiTarget,
          changeOrigin: true,
        },
        "/scim": {
          target: apiTarget,
          changeOrigin: true,
        },
        "/ProviderHintRedirect.js": {
          target: apiTarget,
          changeOrigin: true,
        },
      },
    },
  };
});
