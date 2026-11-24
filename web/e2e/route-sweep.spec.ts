import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect } from "@playwright/test";
import { materialize } from "./fixtures";
import { shouldRunE2E } from "./helpers";

/**
 * 全路由扫描 —— 以 `docs/design/frontend/registry.md` 的生成表为唯一页面清单。
 *
 * 每条路由在每个 viewport 项目（390/768/1024/1440/1920）下断言：
 *   1. 非 5xx、未落入错误边界；
 *   2. 无横向溢出；
 *   3. 主显示区宽度档封顶（1600 / 1200 / 960）；
 *   4. console.error 与 pageerror 为 0。
 *
 * 动态路由由 `fixtures.setup.ts` 提供真实组织/用户标识；对象不存在时由受控错误面
 * 承载并允许 4xx。会话不可用（重定向 /login）是失败，不是 SKIP。
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REGISTRY = path.resolve(HERE, "../../docs/design/frontend/registry.md");
const AUTH_STATE = path.resolve(HERE, ".auth/admin.json");

function loadRoutes(): string[] {
  const markdown = fs.readFileSync(REGISTRY, "utf8");
  const routes: string[] = [];
  for (const line of markdown.split("\n")) {
    const match = line.match(/^\|\s*\d+\s*\|\s*`([^`]+)`\s*\|/);
    if (match) routes.push(match[1]);
  }
  return [...new Set(routes)];
}

const ROUTES = loadRoutes();
const WIDTH_CAPS: Array<[string, number]> = [
  [".gl-content__inner", 1600],
  [".gl-content__inner--form", 1200],
  [".gl-content__inner--reading", 960],
];
// 主显示区容器的页面家族：控制台页面走 .gl-content__inner；独立认证页
// （登录 / 注册 / 邀请 / 忘记密码 / SAML 回调 / 设备码 / Telegram）各自有
// 文档化的独立壳层。扫描要求「至少命中一个」，避免把独立页误判为缺容器。
const MAIN_CONTAINERS = [
  ".gl-content__inner",
  // 两个宽度语义类也是文档化的页面级包装（styles.css「内容宽度三档」）。
  ".gl-content__inner--form",
  ".gl-content__inner--reading",
  ".gl-auth-background",
  ".login-content",
  ".gl-panel",
  ".gl-standalone-page",
];
const ERROR_BOUNDARY_MARKERS = ["Unexpected Application Error", "Application error", "出错了"];

if (fs.existsSync(AUTH_STATE)) {
  test.use({ storageState: AUTH_STATE });
}

test.describe.configure({ mode: "parallel" });

/**
 * 动态路由夹具：`fixtures.setup.ts` 从真实会话解析组织/用户标识并落盘，
 * 这里统一从 `fixtures.ts` 读取，避免写死 UUID 与库内对象漂移。
 */

const STATIC_ROUTES = ROUTES.filter(
  (route) => route !== "*" && !route.includes(":") && !route.includes("*"),
);
const DYNAMIC_ROUTES = ROUTES.filter((route) => route.includes(":"));

// 对象不存在的详情页由 react-router 的 RouterErrorElement 渲染
// `.gl-empty-state`（设计允许的「对象不存在/可重试错误态」），不属于控制台壳层。
// 仅在动态路由（夹具可能指向不存在的对象）下接受该兜底面。
const DYNAMIC_CONTAINERS = [...MAIN_CONTAINERS, ".gl-empty-state"];

// 协议回调面（CAS/SAML/OAuth2/provider callback）由外部协议驱动，不是控制台
// 页面，也没有同一套壳层；不在本扫描内，否则会把协议页判成布局缺陷。
const PROTOCOL_PREFIXES = ["/cas/", "/auth/callback/", "/oauth2/", "/saml/"];

interface SweepOptions {
  /** 动态路由允许对象不存在：忽略 4xx 资源错误，5xx 与其它 console 错误仍失败。 */
  allowMissingResource?: boolean;
}

async function sweepRoute(
  page: import("@playwright/test").Page,
  route: string,
  options: SweepOptions = {},
): Promise<void> {
  const consoleErrors: string[] = [];
  page.on("console", (msg) => {
    if (msg.type() !== "error") return;
    const text = msg.text();
    if (options.allowMissingResource && /status of (400|403|404|410|422)/.test(text)) return;
    consoleErrors.push(text);
  });
  page.on("pageerror", (err) => consoleErrors.push(`pageerror: ${err.message}`));

  const response = await page.goto(route, { waitUntil: "domcontentloaded" });
  const status = response?.status() ?? 0;
  expect(status, `${route} 返回 5xx`).toBeLessThan(500);

  await page.waitForLoadState("networkidle", { timeout: 5000 }).catch(() => undefined);

  const finalPath = new URL(page.url()).pathname;
  // 有 storageState 时被重定向到 /login 说明会话读取失败（常见根因：per-IP 限流
  // 429 或夹具过期）。这不能再记 SKIP，否则会把真实覆盖缺口伪装成通过。
  if (route !== "/login" && !route.startsWith("/login/") && finalPath === "/login") {
    throw new Error(
      `${route} 被重定向到 /login：会话不可用（检查 E2E 限流覆盖 RATE_LIMIT_PER_IP_REQUESTS / storageState）`,
    );
  }

  const bodyText = await page.locator("body").innerText();
  for (const marker of ERROR_BOUNDARY_MARKERS) {
    expect(bodyText, `${route} 落入错误边界（命中 ${marker}）`).not.toContain(marker);
  }

  const layout = await page.evaluate(
    ({ caps, containers, routeLabel }) => {
      const doc = document.documentElement;
      const problems: string[] = [];
      if (doc.scrollWidth > window.innerWidth + 1) {
        problems.push(`横向溢出 scrollWidth=${doc.scrollWidth} > ${window.innerWidth}`);
      }
      for (const [selector, cap] of caps) {
        // ≤1024px 时三档宽度按设计「用可用全宽并保留安全间距」（interaction.md
        // 「内容宽度三档」），此区间只断言无横向溢出，不套用桌面档封顶。
        if (window.innerWidth <= 1024) break;
        const el = document.querySelector(selector);
        if (!el) continue;
        const width = el.getBoundingClientRect().width;
        if (width > cap + 1) problems.push(`${selector} 宽度 ${Math.round(width)} > ${cap}`);
      }
      for (const container of document.querySelectorAll<HTMLElement>(".ant-table-container")) {
        const box = container.getBoundingClientRect();
        if (box.right > window.innerWidth + 1) {
          problems.push(`表格容器超出视口 right=${Math.round(box.right)}`);
        }
      }
      if (!containers.some((selector) => document.querySelector(selector))) {
        problems.push(`${routeLabel} 缺少主显示区容器（${containers.join(" / ")}）`);
      }
      return problems;
    },
    {
      caps: WIDTH_CAPS,
      containers: options.allowMissingResource ? DYNAMIC_CONTAINERS : MAIN_CONTAINERS,
      routeLabel: route,
    },
  );
  expect(layout, `${route} 布局异常：${layout.join("；")}`).toEqual([]);

  expect(consoleErrors, `${route} 控制台错误：${consoleErrors.join(" | ")}`).toEqual([]);
}

for (const route of STATIC_ROUTES) {
  test(`路由扫描: ${route}`, async ({ page }) => {
    test.skip(!shouldRunE2E, "ENABLE_E2E 未启用：路由扫描需要真实后端");
    await sweepRoute(page, route);
  });
}

for (const route of DYNAMIC_ROUTES) {
  test(`动态路由扫描: ${route}`, async ({ page }) => {
    test.skip(!shouldRunE2E, "ENABLE_E2E 未启用：路由扫描需要真实后端");
    test.skip(
      PROTOCOL_PREFIXES.some((prefix) => route.startsWith(prefix)),
      "协议回调面，不属于控制台页面族",
    );
    const path = materialize(route);
    if (path === route) test.skip(true, "路径参数无法夹具化");
    await sweepRoute(page, path, { allowMissingResource: true });
  });
}
