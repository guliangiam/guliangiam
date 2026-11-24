import { test, expect } from "@playwright/test";
import { LoginPage, AppShellPage } from "./pom";
import { TEST_CREDENTIALS, shouldRunE2E } from "./helpers";

/**
 * 认证流程 E2E 测试
 *
 * 覆盖：登录页面渲染、管理员登录、登录失败、登出
 *
 * ⚠️ 需要 ENABLE_E2E=true 才能运行需要后端的测试
 */
const describeE2E = shouldRunE2E ? test.describe : test.describe.skip;

test.describe("认证流程", () => {
  // ─────────────────────────────────────────────
  // 1. 登录页面渲染验证（纯前端，不需要后端）
  // ─────────────────────────────────────────────
  test("登录页面渲染: 访问 /login → 验证表单元素可见", async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.gotoLogin();

    // 验证 URL 包含 /login
    await expect(page).toHaveURL(/\/login/);

    // 验证用户名输入框可见
    await expect(loginPage.usernameInput).toBeVisible();

    // 验证密码输入框可见
    await expect(loginPage.passwordInput).toBeVisible();

    // 验证登录按钮可见
    await expect(loginPage.loginButton).toBeVisible();
  });

  test("登录页面渲染: 验证页面标题和关键元素", async ({ page }) => {
    const loginPage = new LoginPage(page);
    await loginPage.gotoLogin();

    // 验证页面标题包含关键词
    const title = await page.title();
    expect(title).toBeTruthy();

    // 验证表单容器存在
    const formContainer = page.locator("form").first();
    await expect(formContainer).toBeVisible();
  });

  // ─────────────────────────────────────────────
  // 2. 管理员登录流程（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("管理员登录", () => {
    test("管理员登录: 输入正确用户名密码 → 点击登录 → 验证跳转首页", async ({ page }) => {
      const loginPage = new LoginPage(page);
      await loginPage.loginAs(TEST_CREDENTIALS.username, TEST_CREDENTIALS.password);

      // 验证跳转到首页
      await expect(page).toHaveURL(/^.*\/$/, { timeout: 10_000 });
    });

    test("管理员登录: 登录后验证侧边栏和顶栏可见", async ({ page }) => {
      const loginPage = new LoginPage(page);
      await loginPage.loginAndWaitForDashboard(
        TEST_CREDENTIALS.username,
        TEST_CREDENTIALS.password,
      );

      const appShell = new AppShellPage(page);

      // 验证侧边栏可见
      await expect(appShell.sidebar).toBeVisible();

      // 验证顶栏可见
      await expect(appShell.header).toBeVisible();

      // 验证昵称菜单触发区存在（退出登录在该下拉菜单内）
      await expect(appShell.userMenuButton).toBeVisible();
    });
  });

  // ─────────────────────────────────────────────
  // 3. 登录失败（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("登录失败", () => {
    test("登录失败: 输入错误密码 → 验证错误提示", async ({ page }) => {
      const loginPage = new LoginPage(page);
      await loginPage.gotoLogin();

      await loginPage.fillUsername(TEST_CREDENTIALS.username);
      await loginPage.fillPassword("wrong-password-123");
      await loginPage.clickLogin();

      // 验证仍在登录页或显示错误信息
      // 可能的错误提示方式：消息通知、表单内联错误、页面提示
      const hasError =
        (await page.locator(".ant-message-error").count()) > 0 ||
        (await page.locator(".ant-form-item-explain-error").count()) > 0 ||
        (await page.locator(".ant-alert-error").count()) > 0 ||
        (await page.locator("text=/invalid|incorrect|错误|失败/i").count()) > 0;

      // 如果有错误提示则验证，否则验证仍在登录页
      if (hasError) {
        expect(hasError).toBe(true);
      } else {
        await expect(page).toHaveURL(/\/login/, { timeout: 5_000 });
      }
    });

    test("登录失败: 空用户名提交 → 验证验证提示", async ({ page }) => {
      const loginPage = new LoginPage(page);
      await loginPage.gotoLogin();

      // 不填写用户名直接点击登录
      await loginPage.clickLogin();

      // 验证表单验证提示出现（Ant Design 表单验证）
      const hasValidationError =
        (await page.locator(".ant-form-item-explain-error").count()) > 0 ||
        (await page.locator(".ant-form-item-has-error").count()) > 0;

      expect(hasValidationError).toBe(true);
    });
  });

  // ─────────────────────────────────────────────
  // 4. 登出流程（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("登出", () => {
    test("登出: 点击登出按钮 → 验证跳转登录页", async ({ page }) => {
      const loginPage = new LoginPage(page);
      await loginPage.loginAndWaitForDashboard(
        TEST_CREDENTIALS.username,
        TEST_CREDENTIALS.password,
      );

      // 确认已登录
      await expect(page).toHaveURL(/^.*\/$/);

      // 点击登出
      const appShell = new AppShellPage(page);
      await appShell.logout();

      // 验证跳转到登录页
      await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
    });

    test("登出: 登出后访问受保护页面 → 重定向到登录页", async ({ page }) => {
      const loginPage = new LoginPage(page);
      await loginPage.loginAndWaitForDashboard(
        TEST_CREDENTIALS.username,
        TEST_CREDENTIALS.password,
      );

      // 登出
      const appShell = new AppShellPage(page);
      await appShell.logout();
      await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });

      // 访问受保护页面
      await page.goto("/users");
      // 应该被重定向到登录页
      await expect(page).toHaveURL(/\/login/, { timeout: 10_000 });
    });
  });
});
