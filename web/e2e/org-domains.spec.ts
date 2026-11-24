import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { test, expect } from "@playwright/test";
import { loadFixtureIds, materialize } from "./fixtures";
import { shouldRunE2E } from "./helpers";

/**
 * 组织域名治理页（b2b）的权限回归。
 *
 * 历史缺陷：`/organizations/:organizationName/domains` 的路由要求 `b2b:read`，
 * 但后端权限集、前端管理员兜底集和 `resourceToActionKey` 通配展开都没有 b2b，
 * 导致管理员进入该页直接命中 `PermissionGuard` 的 403。
 */

const HERE = path.dirname(fileURLToPath(import.meta.url));
const AUTH_STATE = path.resolve(HERE, ".auth/admin.json");

if (fs.existsSync(AUTH_STATE)) {
  test.use({ storageState: AUTH_STATE });
}

test.describe("组织域名治理页", () => {
  test("管理员进入域名页不再渲染 403", async ({ page }) => {
    test.skip(!shouldRunE2E, "ENABLE_E2E 未启用：该用例需要真实后端与会话");

    const consoleErrors: string[] = [];
    page.on("console", (message) => {
      if (message.type() === "error") consoleErrors.push(message.text());
    });
    page.on("pageerror", (error) => consoleErrors.push(String(error)));

    const ids = loadFixtureIds();
    // 从组织详情进入时页面带 orgId；缺该参数时列表查询不触发（表单也保持 disabled）。
    const route = `${materialize("/organizations/:organizationName/domains")}?orgId=${ids.organizationId}`;
    const listResponse = page.waitForResponse(
      (response) =>
        response.url().includes("/api/v1/organizations/") &&
        response.url().includes("/domains") &&
        response.request().method() === "GET",
      { timeout: 10_000 },
    );
    const response = await page.goto(route);
    expect(response?.status() ?? 0).toBeLessThan(500);
    await page.waitForLoadState("networkidle", { timeout: 5000 }).catch(() => undefined);

    expect(new URL(page.url()).pathname, "会话不可用：被重定向到登录页").not.toBe("/login");

    // 403 结果块（PermissionDeniedResult）以 status=403 渲染，标题为 “403”。
    await expect(page.getByText("403", { exact: true })).toHaveCount(0);
    await expect(page.getByText(/B2B Domain Governance|B2B 域名治理/)).toBeVisible();
    // 认领域名的表单卡片随列表一起渲染，证明页面主体（而非只有骨架屏）加载完成。
    await expect(page.getByText(/Claim New Domain|认领域名/)).toBeVisible();
    await expect(page.locator(".ant-skeleton")).toHaveCount(0);

    // 后端权限集是 403 的根因所在：列表接口本身必须 200，而不是仅前端放行。
    const list = await listResponse;
    expect(list.status(), "域名列表接口被拒绝").toBeLessThan(400);
    expect(consoleErrors, `控制台错误：${consoleErrors.join(" | ")}`).toEqual([]);
  });
});
