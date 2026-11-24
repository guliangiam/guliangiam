import { test, expect } from "@playwright/test";

/**
 * 列表页批量能力回归：复选框列必须"零选中"就存在，选中后出现批量操作栏，
 * 取消后操作栏消失。只验证到"按钮可达"，不真正执行删除，避免破坏种子数据。
 *
 * 仅在 ENABLE_E2E=true 且后端与真实数据库可用时运行。
 */

const APP_ID = process.env.E2E_APPLICATION_ID ?? "01a0ea76-0c00-7000-8000-000000000002";
const ORG_SLUG = process.env.E2E_ORGANIZATION ?? "built-in";
const USERNAME = process.env.E2E_USERNAME ?? "admin";
const PASSWORD = process.env.E2E_PASSWORD ?? "admin";

const BULK_DELETE_LABEL = /Batch Delete|批量删除/i;

async function loginThroughBrowser(page: import("@playwright/test").Page) {
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
    { applicationId: APP_ID, organizationSlug: ORG_SLUG, username: USERNAME, password: PASSWORD },
  );
}

test.describe("list bulk actions", () => {
  test.skip(!process.env.ENABLE_E2E, "requires ENABLE_E2E=true with backend + database");

  test("users list exposes selection column and bulk bar", async ({ page }) => {
    await page.setViewportSize({ width: 1680, height: 1000 });
    await loginThroughBrowser(page);

    await page.goto("/users");
    const rows = page.locator("tr.ant-table-row");
    await expect(rows.first()).toBeVisible({ timeout: 30_000 });

    // 关键回归：没有任何选中项时，复选框列也必须已经渲染
    const rowCheckboxes = page.locator('tr.ant-table-row input[type="checkbox"]');
    await expect(rowCheckboxes.first()).toBeVisible({ timeout: 10_000 });
    // 受保护用户（当前登录人 / built-in admin）的复选框必须置灰。
    // 列表按创建时间排序，首行不一定是受保护用户（运行期夹具会新建账号），
    // 因此断言「存在置灰复选框」而不是「第一行置灰」。
    await expect(
      page.locator('tr.ant-table-row input[type="checkbox"]:disabled').first(),
    ).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByRole("toolbar")).toHaveCount(0);

    // 换到没有保护行的角色列表，验证选中后批量栏出现且默认删除按钮可达
    await page.goto("/roles");
    const roleRows = page.locator("tr.ant-table-row");
    await expect(roleRows.first()).toBeVisible({ timeout: 30_000 });
    const roleCheckbox = page.locator('tr.ant-table-row input[type="checkbox"]').first();
    await expect(roleCheckbox).toBeEnabled({ timeout: 10_000 });
    await roleCheckbox.check({ force: true });

    const toolbar = page.getByRole("toolbar");
    await expect(toolbar).toBeVisible({ timeout: 10_000 });
    await expect(toolbar.getByRole("button", { name: BULK_DELETE_LABEL })).toBeVisible();

    // 取消选择后批量栏消失
    await toolbar.getByRole("button", { name: /cancel|取消/i }).click();
    await expect(page.getByRole("toolbar")).toHaveCount(0, { timeout: 10_000 });

    // 应用列表此前绕过共享 deleteFn，单独确认它也已具备批量删除入口
    await page.goto("/applications");
    await expect(page.locator("tr.ant-table-row").first()).toBeVisible({ timeout: 30_000 });
    const appCheckbox = page.locator('tr.ant-table-row input[type="checkbox"]').first();
    await expect(appCheckbox).toBeEnabled({ timeout: 10_000 });
    await appCheckbox.check({ force: true });
    await expect(
      page.getByRole("toolbar").getByRole("button", { name: BULK_DELETE_LABEL }),
    ).toBeVisible();
  });
});
