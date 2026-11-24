import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { existsSync, readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

/**
 * 控制台可访问性检查（设计报告 28.6 条 6 / 28.15「可访问性」层）。
 *
 * 覆盖面以 `docs/design/frontend/registry.md`（生成物）为唯一页面清单，与
 * 路由扫描保持同一事实源；此处不维护第二份路由清单，避免与覆盖闭合门禁的
 * 判定漂移。含路径参数的动态路由不在此扫描：它们需要真实对象，由
 * `ENABLE_E2E=true` 下的端到端用例覆盖。
 *
 * 免登录可渲染的路径（登录/注册/协议面）默认扫描；需要会话的路径在未登录
 * 时会被应用重定向到登录页，此时用例判为跳过而不是失败，以免把「未提供
 * 后端环境」误报成可访问性回归。提供完整环境并设置 ENABLE_E2E=true 后，
 * 同一批用例会自动覆盖已登录页面。
 *
 * 运行方式：
 *   pnpm run e2e:a11y
 */

const ROUTER = resolve(dirname(fileURLToPath(import.meta.url)), "../src/app/router/router.tsx");
const REGISTRY = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../docs/design/frontend/registry.md",
);
const AUTH_STATE = resolve(dirname(fileURLToPath(import.meta.url)), ".auth/admin.json");
const LOGIN_PATH = "/login";

// 有登录态落盘（由 auth.setup.ts 生成）时复用会话：否则每个受保护页面都会被
// 重定向到 /login，扫描结果会变成「登录取样 120 次」。无后端环境时仍按未登录
// 路径扫描并由 redirectedToLogin 判为跳过。
if (existsSync(AUTH_STATE)) {
  test.use({ storageState: AUTH_STATE });
}

// 静态扫描不需要录像；录像依赖 ffmpeg 二进制，缺失时会让整批用例在
// 创建页面时就失败，把可访问性结论埋进基础设施错误里。
test.use({ video: "off" });

function routeModuleFiles(entry: string, seen = new Set<string>()): string[] {
  if (seen.has(entry)) return [];
  seen.add(entry);

  const source = readFileSync(entry, "utf8");
  const files = [entry];
  for (const match of source.matchAll(/import\s*{[^}]+}\s*from\s*["'](\.[^"']+)["']/g)) {
    const specifier = match[1];
    if (!specifier.includes("routes")) continue;
    const candidate = resolve(dirname(entry), `${specifier}.tsx`);
    if (existsSync(candidate)) files.push(...routeModuleFiles(candidate, seen));
  }
  return files;
}

const routeModules = routeModuleFiles(ROUTER);

function staticRoutes(): string[] {
  // 页面清单以 `docs/design/frontend/registry.md`（生成物）为唯一事实源。
  // 从 router.tsx 直接抓 `path:` 会把 AppShell 子路由的相对路径拍平成
  // `/api-keys` 这类不存在的绝对路径（真实路径是 `/ai/api-keys`），
  // 扫描到的是兜底页而不是页面本身。
  const markdown = readFileSync(REGISTRY, "utf8");
  const paths: string[] = [];
  for (const line of markdown.split("\n")) {
    const match = line.match(/^\|\s*\d+\s*\|\s*`([^`]+)`\s*\|/);
    if (match) paths.push(match[1]);
  }
  return [...new Set(paths)].filter((path) => !path.includes(":") && !path.includes("*")).sort();
}

const routes = staticRoutes();

// 第三方嵌入页：/swagger 由 Scalar API Reference 组件渲染整页（vendor 自带样式与
// 交互模型），控制台无法在不 fork 组件的前提下修正其对比度/标签。这里显式排除并
// 记录原因，避免把 vendor 债务混进控制台 a11y 结论。
const THIRD_PARTY_EMBEDS = new Set(["/swagger"]);

// 未登录时与登录页同形（跳转目标一致）的路径无法区分，故只跳过「请求了别的
// 路径却被送到登录页」的情形。
function redirectedToLogin(pathname: string, requested: string): boolean {
  return pathname !== requested && pathname.startsWith(LOGIN_PATH);
}

test("可访问性路由覆盖包含全部路由模块", () => {
  expect(routeModules.length).toBeGreaterThanOrEqual(10);
  expect(routes.length).toBeGreaterThan(100);
});

for (const route of routes) {
  test(`可访问性检查: ${route}`, async ({ page }) => {
    test.skip(THIRD_PARTY_EMBEDS.has(route), `${route} 为第三方嵌入页（vendor 自带样式）`);
    const response = await page.goto(route);
    expect(response?.status(), `${route} 应可访问（非 5xx）`).toBeLessThan(500);

    const requested = new URL(page.url()).pathname;
    test.skip(redirectedToLogin(requested, route), `${route} 需要登录会话（未提供后端环境）`);

    // 扫描前等待首屏渲染与懒加载页面挂载完成。
    await page.waitForLoadState("networkidle");

    const { violations } = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"])
      .analyze();

    const blocking = violations.filter(
      (violation) => violation.impact === "serious" || violation.impact === "critical",
    );

    const report = blocking
      .map(
        (violation) =>
          `[${violation.impact}] ${violation.id}: ${violation.help}\n` +
          violation.nodes.map((node) => `    ${node.target.join(" ")}`).join("\n"),
      )
      .join("\n");

    expect(blocking, `${route} 存在严重可访问性违规：\n${report}`).toEqual([]);
  });
}
