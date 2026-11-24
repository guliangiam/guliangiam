import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect } from "@playwright/test";
import { materialize } from "./fixtures";
import { shouldRunE2E } from "./helpers";

/**
 * 逐页视觉证据：以 registry 生成表为唯一清单，逐页打开并落盘截图，
 * 供 contact sheet 复核 R2（视觉异常）与 R4（表单排布）。
 * 截图写 `reports/2026-10-30/pages/`（reports/ 已 gitignore，不入库）。
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REGISTRY = path.resolve(HERE, "../../docs/design/frontend/registry.md");
const OUT_DIR = path.resolve(HERE, "../../reports/2026-10-30/pages");
const AUTH_STATE = path.resolve(HERE, ".auth/admin.json");
if (fs.existsSync(AUTH_STATE)) test.use({ storageState: AUTH_STATE });

const PROTOCOL_PREFIXES = ["/cas/", "/auth/callback/", "/oauth2/", "/saml/"];

function loadRoutes(): string[] {
  const lines = fs.readFileSync(REGISTRY, "utf8").split("\n");
  const routes = new Set<string>();
  for (const line of lines) {
    const match = line.match(/^\|\s*\d+\s*\|\s*`([^`]+)`\s*\|/);
    if (match) routes.add(match[1].trim());
  }
  return [...routes].filter(
    (route) => route !== "*" && !PROTOCOL_PREFIXES.some((p) => route.startsWith(p)),
  );
}

const ROUTES = loadRoutes();
test.describe.configure({ mode: "parallel" });

for (const route of ROUTES) {
  test(`页面截图: ${route}`, async ({ page }, testInfo) => {
    test.skip(!shouldRunE2E, "ENABLE_E2E 未启用：逐页截图需要真实后端");
    const target = route.includes(":") ? materialize(route) : route;
    await page.goto(target, { waitUntil: "domcontentloaded" });
    await page.waitForLoadState("networkidle", { timeout: 5000 }).catch(() => undefined);
    const finalPath = new URL(page.url()).pathname;
    expect(
      finalPath,
      `${route} 被重定向到 /login（会话不可用；检查 E2E 限流覆盖与 storageState）`,
    ).not.toBe("/login");

    const overflow = await page.evaluate(() => {
      const doc = document.documentElement;
      return doc.scrollWidth > window.innerWidth + 1
        ? `横向溢出 scrollWidth=${doc.scrollWidth} > ${window.innerWidth}`
        : "";
    });
    expect(overflow, `${route} 横向溢出`).toBe("");

    fs.mkdirSync(OUT_DIR, { recursive: true });
    const file = `${testInfo.project.name}-${
      route.replace(/[^a-zA-Z0-9]+/g, "_").replace(/^_|_$/g, "") || "root"
    }.png`;
    await page.screenshot({ path: path.join(OUT_DIR, file), fullPage: false });
  });
}
