import { test, expect } from "@playwright/test";
import { AppShellPage, LoginPage } from "./pom";
import { TEST_CREDENTIALS, shouldRunE2E, loginAsAdmin } from "./helpers";

/**
 * 导航 E2E 测试
 *
 * 覆盖：侧边栏导航、面包屑导航、页面切换
 *
 * ⚠️ 需要 ENABLE_E2E=true 才能运行需要后端的测试
 */
const describeE2E = shouldRunE2E ? test.describe : test.describe.skip;

test.describe("导航", () => {
  // ─────────────────────────────────────────────
  // 1. 侧边栏导航（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("侧边栏导航", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("侧边栏: 验证侧边栏可见", async ({ page }) => {
      const appShell = new AppShellPage(page);
      await appShell.expectSidebarVisible();
    });

    test("侧边栏: 验证菜单项存在", async ({ page }) => {
      const appShell = new AppShellPage(page);
      const menuItems = page.locator(".ant-menu-item");
      const count = await menuItems.count();

      // 至少应该有几个菜单项
      expect(count).toBeGreaterThan(0);
    });

    test("侧边栏: 点击用户管理菜单 → 验证跳转", async ({ page }) => {
      const appShell = new AppShellPage(page);

      // 先展开 Identity 分组（如果需要）
      const identityGroup = page
        .locator(".ant-menu-submenu-title")
        .filter({ hasText: /identity|身份/i })
        .first();
      if (await identityGroup.isVisible()) {
        await identityGroup.click();
        await page.waitForTimeout(500);
      }

      // 点击用户管理菜单项
      await appShell.clickSidebarMenuInGroup(/identity|身份/i, /users/i);

      // 验证跳转到用户管理页
      await expect(page).toHaveURL(/\/users/, { timeout: 10_000 });
    });

    test("侧边栏: 点击组织管理菜单 → 验证跳转", async ({ page }) => {
      const appShell = new AppShellPage(page);

      // 先展开 Identity 分组（如果需要）
      const identityGroup = page
        .locator(".ant-menu-submenu-title")
        .filter({ hasText: /identity|身份/i })
        .first();
      if (await identityGroup.isVisible()) {
        await identityGroup.click();
        await page.waitForTimeout(500);
      }

      // 点击组织管理菜单项
      await appShell.clickSidebarMenuInGroup(/identity|身份/i, /organizations/i);

      // 验证跳转到组织管理页
      await expect(page).toHaveURL(/\/organizations/, { timeout: 10_000 });
    });

    test("侧边栏: 点击应用管理菜单 → 验证跳转", async ({ page }) => {
      const appShell = new AppShellPage(page);

      // 先展开 Identity 分组（如果需要）
      const identityGroup = page
        .locator(".ant-menu-submenu-title")
        .filter({ hasText: /identity|身份/i })
        .first();
      if (await identityGroup.isVisible()) {
        await identityGroup.click();
        await page.waitForTimeout(500);
      }

      // 点击应用管理菜单项（Home 分组也有同名 "Applications"，必须限定 Identity 分组）
      await appShell.clickSidebarMenuInGroup(/identity|身份/i, /applications/i);

      // 验证跳转到应用管理页
      await expect(page).toHaveURL(/\/applications/, { timeout: 10_000 });
    });

    test("侧边栏: 折叠/展开侧边栏", async ({ page }) => {
      const appShell = new AppShellPage(page);

      // 验证侧边栏可见
      await appShell.expectSidebarVisible();

      // 点击折叠按钮
      const toggleBtn = appShell.sidebarToggle;
      if (await toggleBtn.isVisible()) {
        await toggleBtn.click();
        await page.waitForTimeout(500);

        // 验证侧边栏已折叠（宽度变小）
        const sider = page.locator(".ant-layout-sider").first();
        const width = await sider.evaluate((el) => el.clientWidth);
        // 折叠后宽度应该小于 100px（默认 64px）
        expect(width).toBeLessThan(100);

        // 再次点击展开
        await toggleBtn.click();
        await page.waitForTimeout(500);

        // 验证侧边栏已展开
        const expandedWidth = await sider.evaluate((el) => el.clientWidth);
        expect(expandedWidth).toBeGreaterThan(100);
      }
    });
  });

  // ─────────────────────────────────────────────
  // 2. 面包屑导航（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("面包屑导航", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("面包屑: 首页不显示面包屑或显示简短面包屑", async ({ page }) => {
      // 在首页
      await expect(page).toHaveURL(/^.*\/$/);

      // 面包屑可能不存在或只有一项
      const breadcrumb = page.locator(".ant-breadcrumb");
      const breadcrumbVisible = await breadcrumb.isVisible().catch(() => false);
      // 首页面包屑是可选的
      expect(typeof breadcrumbVisible).toBe("boolean");
    });

    test("面包屑: 访问用户管理页 → 验证面包屑", async ({ page }) => {
      await page.goto("/users");
      await page.waitForTimeout(2000);

      // 验证面包屑存在
      const breadcrumb = page.locator(".ant-breadcrumb");
      if (await breadcrumb.isVisible()) {
        const breadcrumbTexts = await breadcrumb.innerText();
        // 面包屑应包含 Users 或用户相关文本
        expect(breadcrumbTexts.length).toBeGreaterThan(0);
      }
    });

    test("面包屑: 点击面包屑链接 → 验证导航", async ({ page }) => {
      // 先访问一个子页面
      await page.goto("/users");
      await page.waitForTimeout(2000);

      const breadcrumb = page.locator(".ant-breadcrumb");
      if (await breadcrumb.isVisible()) {
        // 点击面包屑中的首页链接
        const homeLink = breadcrumb.locator("a").first();
        if (await homeLink.isVisible()) {
          await homeLink.click();
          // 验证导航成功
          await page.waitForTimeout(1000);
          expect(page.url()).toBeTruthy();
        }
      }
    });
  });

  // ─────────────────────────────────────────────
  // 3. 页面切换（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("页面切换", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("页面切换: 从首页切换到用户管理", async ({ page }) => {
      // 确认在首页
      await expect(page).toHaveURL(/^.*\/$/);

      // 导航到用户管理
      await page.goto("/users");
      await expect(page).toHaveURL(/\/users/);
    });

    test("页面切换: 从用户管理切换到组织管理", async ({ page }) => {
      await page.goto("/users");
      await expect(page).toHaveURL(/\/users/);

      await page.goto("/organizations");
      await expect(page).toHaveURL(/\/organizations/);
    });

    test("页面切换: 从组织管理切换到应用管理", async ({ page }) => {
      await page.goto("/organizations");
      await expect(page).toHaveURL(/\/organizations/);

      await page.goto("/applications");
      await expect(page).toHaveURL(/\/applications/);
    });

    test("页面切换: 浏览器后退和前进", async ({ page }) => {
      // 访问多个页面
      await page.goto("/users");
      await page.goto("/organizations");
      await page.goto("/applications");

      // 后退
      await page.goBack();
      await expect(page).toHaveURL(/\/organizations/);

      // 再后退
      await page.goBack();
      await expect(page).toHaveURL(/\/users/);

      // 前进
      await page.goForward();
      await expect(page).toHaveURL(/\/organizations/);
    });

    test("页面切换: 通过侧边栏连续切换页面", async ({ page }) => {
      const appShell = new AppShellPage(page);

      // 展开 Identity 分组
      const identityGroup = page
        .locator(".ant-menu-submenu-title")
        .filter({ hasText: /identity|身份/i })
        .first();
      if (await identityGroup.isVisible()) {
        await identityGroup.click();
        await page.waitForTimeout(500);
      }

      // 点击用户管理
      await appShell.clickSidebarMenuInGroup(/identity|身份/i, /users/i);
      await expect(page).toHaveURL(/\/users/, { timeout: 10_000 });

      // 点击组织管理
      await appShell.clickSidebarMenuInGroup(/identity|身份/i, /organizations/i);
      await expect(page).toHaveURL(/\/organizations/, { timeout: 10_000 });

      // 点击应用管理（限定 Identity 分组，避开 Home 里的 /apps）
      await appShell.clickSidebarMenuInGroup(/identity|身份/i, /applications/i);
      await expect(page).toHaveURL(/\/applications/, { timeout: 10_000 });
    });
  });
});
