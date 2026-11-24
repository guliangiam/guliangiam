import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect, type Page } from "@playwright/test";
import { TEST_CREDENTIALS } from "./helpers";

/**
 * L5 回归：删除按钮的"可见性"必须在真实后端 + 真实权限下发下验证。
 *
 * 历史故障（两次）：DELETE 路由声明为 `:write`，页面守卫却挂 `:delete`，
 * 静态 diff 看起来"按钮在"，运行时 PermissionGuard 永远判 false，按钮从不渲染。
 * 本用例用管理员与只读账号分别进四页，按 DOM 可见性断言行删除/批量删除：
 *   - 管理员：真实可见（toBeVisible，不是 count > 0）
 *   - 只读：不可见（元素不渲染 → toHaveCount(0) + toBeHidden）
 *
 * 需要 ENABLE_E2E=true、真实后端、隔离库；只读账号通过
 * E2E_VIEWER_USERNAME / E2E_VIEWER_PASSWORD 注入，未设置时跳过只读用例。
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SHOT_DIR = path.resolve(HERE, "../../.trae/documents/l5-permission-visibility");

const DELETE = /Delete|删除/i;
const BULK_DELETE = /Batch Delete|批量删除/i;
const APP_ID = process.env.E2E_APPLICATION_ID ?? "01a0ea76-0c00-7000-8000-000000000002";
const ORG_SLUG = process.env.E2E_ORGANIZATION ?? "built-in";

const PAGES = [
  { name: "roles", path: "/roles" },
  { name: "groups", path: "/groups" },
  { name: "permissions", path: "/permissions" },
  { name: "organizations", path: "/organizations" },
] as const;

async function loginThroughBrowser(page: Page, username: string, password: string) {
  await page.goto("/login");
  await page.evaluate(async () => {
    await fetch("/api/v1/auth/contact", { credentials: "include" }).catch(() => undefined);
  });
  await expect
    .poll(() => page.evaluate(() => /(?:^|;\s*)csrf_token=/.test(document.cookie)), {
      timeout: 10_000,
    })
    .toBe(true);

  await page.evaluate(
    async ({ applicationId, organizationSlug, username, password }) => {
      const csrfMatch = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]*)/);
      const csrf = csrfMatch ? decodeURIComponent(csrfMatch[1]) : "";
      const res = await fetch("/api/v1/auth/login", {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
          ...(csrf ? { "X-CSRF-Token": csrf } : {}),
        },
        body: JSON.stringify({ applicationId, organizationSlug, username, password }),
      });
      if (!res.ok) {
        throw new Error(`login failed: ${res.status} ${await res.text()}`);
      }
    },
    { applicationId: APP_ID, organizationSlug: ORG_SLUG, username, password },
  );
}

async function openList(page: Page, route: string) {
  await page.goto(route);
  await expect(page.locator("tr.ant-table-row").first()).toBeVisible({ timeout: 30_000 });
}

function firstSelectableCheckbox(page: Page) {
  return page.locator('tr.ant-table-row input[type="checkbox"]:not(:disabled)').first();
}

test.describe("delete button visibility under real permissions", () => {
  test.skip(!process.env.ENABLE_E2E, "requires ENABLE_E2E=true with backend + database");

  test("admin sees row delete and bulk delete on four pages", async ({ page }) => {
    test.setTimeout(180_000);
    await page.setViewportSize({ width: 1680, height: 1000 });
    await loginThroughBrowser(
      page,
      process.env.E2E_ADMIN_USERNAME ?? TEST_CREDENTIALS.username,
      process.env.E2E_ADMIN_PASSWORD ?? TEST_CREDENTIALS.password,
    );
    fs.mkdirSync(SHOT_DIR, { recursive: true });

    for (const target of PAGES) {
      await openList(page, target.path);

      // 行删除：最后一行操作列的 danger 按钮必须真实可见（不是 DOM 里存在即可）。
      const rowDelete = page
        .locator("tr.ant-table-row")
        .last()
        .getByRole("button", { name: DELETE });
      await expect(rowDelete).toBeVisible({ timeout: 10_000 });

      // 批量删除：选中可勾选行后，批量操作栏按钮必须真实可见。
      const checkbox = firstSelectableCheckbox(page);
      await expect(checkbox).toBeVisible({ timeout: 10_000 });
      await checkbox.check();
      await expect(page.getByRole("button", { name: BULK_DELETE })).toBeVisible({
        timeout: 10_000,
      });

      await page.screenshot({
        path: path.join(SHOT_DIR, `admin-${target.name}.png`),
        fullPage: true,
      });

      await checkbox.uncheck();
      await expect(page.getByRole("button", { name: BULK_DELETE })).toBeHidden({
        timeout: 10_000,
      });
    }
  });

  test("viewer cannot see row delete or bulk delete on four pages", async ({ page }) => {
    test.skip(
      !process.env.E2E_VIEWER_USERNAME,
      "requires E2E_VIEWER_USERNAME/E2E_VIEWER_PASSWORD for the read-only account",
    );
    test.setTimeout(180_000);
    await page.setViewportSize({ width: 1680, height: 1000 });
    await loginThroughBrowser(
      page,
      process.env.E2E_VIEWER_USERNAME as string,
      process.env.E2E_VIEWER_PASSWORD ?? TEST_CREDENTIALS.password,
    );
    fs.mkdirSync(SHOT_DIR, { recursive: true });

    for (const target of PAGES) {
      await openList(page, target.path);

      // 行删除：PermissionGuard fallback=null → 元素根本不渲染。
      const rowDelete = page.locator("tr.ant-table-row").getByRole("button", { name: DELETE });
      await expect(rowDelete).toHaveCount(0);
      await expect(rowDelete.first()).toBeHidden();

      // 批量删除：勾选行后批量栏仍不得出现该按钮。
      const checkbox = firstSelectableCheckbox(page);
      if ((await checkbox.count()) > 0) {
        await checkbox.check();
        const bulkDelete = page.getByRole("button", { name: BULK_DELETE });
        await expect(bulkDelete).toHaveCount(0);
        await expect(bulkDelete.first()).toBeHidden();
      }

      await page.screenshot({
        path: path.join(SHOT_DIR, `viewer-${target.name}.png`),
        fullPage: true,
      });
    }
  });
});
