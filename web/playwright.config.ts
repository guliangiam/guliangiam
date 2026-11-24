import { defineConfig, devices } from "@playwright/test";

/**
 * Playwright E2E 测试配置
 *
 * 运行方式:
 *   npx playwright test              # 运行所有测试
 *   npx playwright test --ui         # 交互式UI模式
 *   npx playwright test e2e/auth     # 仅运行认证测试
 *   npx playwright test e2e/users    # 仅运行用户管理测试
 *   npx playwright test --headed     # 有头模式运行
 *
 * 注意: E2E测试需要完整的前后端环境才能运行：
 *   - 前端 dev server（端口以 vite.config.ts 的 server.port 为唯一口径）
 *   - 后端 API server
 *   - 数据库中有测试数据
 *
 * 设置 ENABLE_E2E=true 环境变量可启用需要后端的测试。
 * e2e/a11y.spec.ts 不需要后端：它只扫描匿名可渲染路径，需会话的页面自动跳过。
 */
const PORT = 7101;
const BASE_URL = `http://localhost:${PORT}`;

// 默认用 Playwright 自带的 chromium；CI 与本地若只有系统 Chrome（或不想再下载
// 浏览器二进制），可用 PLAYWRIGHT_CHANNEL=chrome 切换，配置只保留一处口径。
const CHANNEL = process.env.PLAYWRIGHT_CHANNEL;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  timeout: 30_000,
  expect: {
    timeout: 5_000,
  },
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : [["list"]],
  use: {
    baseURL: BASE_URL,
    channel: CHANNEL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    // 登录一次并落盘 storageState：路由扫描的 100+ 页面复用同一会话，
    // 避免每个用例都登录（慢且会撞认证面 per-IP 限流）。
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
      use: { ...devices["Desktop Chrome"], colorScheme: "light" },
    },
    // 从真实会话解析动态路由夹具（组织/用户标识），供 route-sweep 复用。
    {
      name: "fixtures",
      testMatch: /fixtures\.setup\.ts/,
      dependencies: ["setup"],
      use: { ...devices["Desktop Chrome"], colorScheme: "light" },
    },
    // 浅色与深色各扫一遍：主题跟随系统设置（设计报告 28.6 条 8），只扫一种配色
    // 会漏掉整类对比度问题（深色主题的品牌色更亮，白字必然不达标）。
    // setup 与 route-sweep 由专用项目承担，避免在桌面套件里重复跑。
    {
      name: "chromium-light",
      testIgnore: /(auth\.setup|fixtures\.setup|route-sweep|page-audit)\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], colorScheme: "light" },
    },
    {
      name: "chromium-dark",
      testIgnore: /(auth\.setup|fixtures\.setup|route-sweep|page-audit)\.spec\.ts/,
      use: { ...devices["Desktop Chrome"], colorScheme: "dark" },
    },
    // 视口矩阵：390（手机）/768（平板）/1024/1440/1920 五档，跑布局响应式
    // 与全路由扫描（route-sweep）；两条用例都以 registry 为准。
    ...[
      { name: "viewport-390", width: 390, height: 844 },
      { name: "viewport-768", width: 768, height: 1024 },
      { name: "viewport-1024", width: 1024, height: 768 },
      { name: "viewport-1440", width: 1440, height: 900 },
      { name: "viewport-1920", width: 1920, height: 1080 },
    ].map(({ name, width, height }) => ({
      name,
      testMatch: /(responsive|route-sweep|page-audit)\.spec\.ts/,
      dependencies: ["setup", "fixtures"],
      use: {
        ...devices["Desktop Chrome"],
        colorScheme: "light" as const,
        viewport: { width, height },
      },
    })),
  ],
  webServer: {
    // 与 vite.config.ts 的 server.port 一致；strictPort 防止回退到其他端口后
    // 测试静默连上别人的 dev server（历史故障）。
    command: "pnpm dev --strictPort",
    url: BASE_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
