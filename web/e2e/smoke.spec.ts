import { test, expect } from "@playwright/test";
import { AppShellPage } from "./pom";

/**
 * GuliangIAM 核心流程冒烟测试
 *
 * ⚠️ 这些测试需要完整的前后端环境才能运行：
 *   - 前端 dev server (port 7101)
 *   - 后端 API server (port 8000)
 *   - 数据库中有测试数据
 *
 * 因此所有测试使用 test.skip() 标记为可跳过。
 * 在完整环境中运行时，移除 skip 或设置环境变量 ENABLE_E2E=true 即可执行。
 */

const shouldRunE2E = process.env.ENABLE_E2E === "true";
const describeE2E = shouldRunE2E ? test.describe : test.describe.skip;

// ── 测试数据 ──
// 凭据必须与 helpers.ts 同源（E2E_USERNAME / E2E_PASSWORD），否则本文件会用到
// 与其它套件不同的口令，出现「只有 smoke 登录失败」的假缺陷。
const TEST_CREDENTIALS = {
  username: process.env.E2E_USERNAME ?? "admin",
  password: process.env.E2E_PASSWORD ?? "admin123",
};

describeE2E("核心流程冒烟测试", () => {
  // ─────────────────────────────────────────────
  // 1. 登录流程
  // ─────────────────────────────────────────────
  test("登录流程: 访问登录页 → 输入用户名密码 → 点击登录 → 验证跳转首页", async ({ page }) => {
    // 访问登录页
    await page.goto("/login");
    await expect(page).toHaveURL(/\/login/);

    // 验证登录表单可见
    const usernameInput = page.locator('input[id="username"], input[name="username"]').first();
    const passwordInput = page.locator('input[type="password"]').first();
    await expect(usernameInput).toBeVisible();
    await expect(passwordInput).toBeVisible();

    // 输入用户名和密码
    await usernameInput.fill(TEST_CREDENTIALS.username);
    await passwordInput.fill(TEST_CREDENTIALS.password);

    // 点击登录按钮
    const loginButton = page.getByRole("button", { name: /sign\s*in|登录/i }).first();
    await expect(loginButton).toBeVisible();
    await loginButton.click();

    // 验证跳转到首页（Dashboard）
    await expect(page).toHaveURL(/^.*\/$/, { timeout: 10_000 });
  });

  // ─────────────────────────────────────────────
  // 2. 首页渲染
  // ─────────────────────────────────────────────
  test("首页渲染: 登录后 → 验证侧边栏菜单可见 → 验证Dashboard标题", async ({ page }) => {
    // 先登录
    await loginAs(page, TEST_CREDENTIALS.username, TEST_CREDENTIALS.password);

    // 验证侧边栏菜单可见
    const sidebar = page.locator(".gl-sidebar, .ant-layout-sider").first();
    await expect(sidebar).toBeVisible();

    // 验证侧边栏中存在菜单项
    const menuItems = page.locator(".ant-menu-item, .ant-menu-submenu");
    await expect(menuItems.first()).toBeVisible();

    // 验证 Dashboard 标题
    const dashboardTitle = page
      .getByRole("heading", { level: 2 })
      .filter({ hasText: /dashboard|仪表盘/i });
    await expect(dashboardTitle).toBeVisible();
  });

  // ─────────────────────────────────────────────
  // 3. 用户管理
  // ─────────────────────────────────────────────
  test("用户管理: 点击用户管理菜单 → 验证用户列表页渲染 → 验证搜索框存在", async ({ page }) => {
    // 先登录
    await loginAs(page, TEST_CREDENTIALS.username, TEST_CREDENTIALS.password);

    // Identity 是折叠分组，先展开再点 Users
    const appShell = new AppShellPage(page);
    await appShell.expandSidebarGroup(/identity|身份/i);
    await appShell.clickSidebarMenu(/users/i);

    // 验证跳转到用户管理页
    await expect(page).toHaveURL(/\/users/);

    // 验证用户列表页渲染 — 检查 ResourceListCard 或 ant-table
    const tableOrList = page.locator(".ant-table, .ant-list").first();
    await expect(tableOrList).toBeVisible({ timeout: 10_000 });

    // 验证搜索框存在
    const searchInput = page
      .locator(
        'input[placeholder*="search" i], input[placeholder*="搜索"], .ant-input-search input',
      )
      .first();
    await expect(searchInput).toBeVisible();
  });

  // ─────────────────────────────────────────────
  // 4. 登出流程
  // ─────────────────────────────────────────────
  test("登出流程: 点击登出按钮 → 验证跳转登录页", async ({ page }) => {
    // 先登录
    await loginAs(page, TEST_CREDENTIALS.username, TEST_CREDENTIALS.password);

    // 确认已登录（在首页）
    await expect(page).toHaveURL(/^.*\/$/);

    // 登出入口在昵称下拉菜单内，不是顶栏独立按钮
    const appShell = new AppShellPage(page);
    await appShell.logout();

    // 验证跳转到登录页
    await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
  });
});

// ─────────────────────────────────────────────
// 辅助函数: 登录
// ─────────────────────────────────────────────
async function loginAs(page: import("@playwright/test").Page, username: string, password: string) {
  await page.goto("/login");
  const usernameInput = page.locator('input[id="username"], input[name="username"]').first();
  const passwordInput = page.locator('input[type="password"]').first();
  await usernameInput.fill(username);
  await passwordInput.fill(password);
  const loginButton = page.getByRole("button", { name: /sign\s*in|登录/i }).first();
  await loginButton.click();
  // 等待跳转到首页，并等外壳真正挂载（侧边栏/顶栏渲染完成）
  await expect(page).toHaveURL(/^.*\/$/, { timeout: 10_000 });
  await page.locator(".gl-header").first().waitFor({ state: "visible", timeout: 15_000 });
}
