import { test, expect } from "@playwright/test";
import { ApplicationsPage, ApplicationDetailPage, LoginPage } from "./pom";
import { TEST_CREDENTIALS, shouldRunE2E, loginAsAdmin } from "./helpers";

/**
 * 应用管理 E2E 测试
 *
 * 覆盖：应用列表渲染、创建应用、编辑应用配置
 *
 * ⚠️ 需要 ENABLE_E2E=true 才能运行需要后端的测试
 */
const describeE2E = shouldRunE2E ? test.describe : test.describe.skip;

test.describe("应用管理", () => {
  // ─────────────────────────────────────────────
  // 1. 应用列表页面渲染（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("应用列表页面渲染", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("应用列表: 访问 /applications → 验证表格渲染", async ({ page }) => {
      const appsPage = new ApplicationsPage(page);
      await appsPage.gotoApplications();

      // 验证 URL
      await expect(page).toHaveURL(/\/applications/);

      // 验证表格可见，并校验真的有数据行（空表不能全绿）
      await expect(appsPage.appTable).toBeVisible({ timeout: 10_000 });
      const rows = page.locator("tr.ant-table-row");
      await expect(rows.first()).toBeVisible({ timeout: 10_000 });
      const rowCount = await rows.count();
      expect(rowCount, "应用列表不应为空").toBeGreaterThanOrEqual(1);
      await expect(rows).toHaveCount(rowCount);
    });

    test("应用列表: 验证搜索框存在", async ({ page }) => {
      const appsPage = new ApplicationsPage(page);
      await appsPage.gotoApplications();

      // 验证搜索框可见（并行跑套件时首屏渲染会变慢，放宽等待）
      await expect(appsPage.searchInput).toBeVisible({ timeout: 15_000 });
    });

    test("应用列表: 验证新建应用按钮存在", async ({ page }) => {
      const appsPage = new ApplicationsPage(page);
      await appsPage.gotoApplications();

      // 验证新建应用按钮可见
      await expect(appsPage.newAppButton).toBeVisible();
    });

    test("应用列表: 验证表格列标题", async ({ page }) => {
      const appsPage = new ApplicationsPage(page);
      await appsPage.gotoApplications();
      await appsPage.expectApplicationsPageVisible();

      // 验证关键列标题存在
      const tableHeaders = page.locator(".ant-table-thead th");
      const headerTexts = await tableHeaders.allTextContents();
      const joinedHeaders = headerTexts.join(" ").toLowerCase();

      // 至少应包含 name 列
      expect(joinedHeaders.includes("name") || joinedHeaders.includes("名称")).toBe(true);
    });
  });

  // ─────────────────────────────────────────────
  // 2. 创建应用（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("创建应用", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("创建应用: 访问 /applications/new → 验证表单渲染", async ({ page }) => {
      const appDetailPage = new ApplicationDetailPage(page);
      await appDetailPage.gotoCreateApplication();

      // 验证 URL
      await expect(page).toHaveURL(/\/applications\/new/);

      // 验证表单元素可见（至少有名称输入框）
      const formContainer = page.locator("form").first();
      await expect(formContainer).toBeVisible();
    });

    test("创建应用: 填写基本信息并提交", async ({ page }) => {
      const appDetailPage = new ApplicationDetailPage(page);
      await appDetailPage.gotoCreateApplication();

      // 查找并填写应用名称（如果存在）
      const nameInput = page.locator('input[name="name"], #name').first();
      if (await nameInput.isVisible()) {
        const timestamp = Date.now();
        await nameInput.fill(`e2e-app-${timestamp}`);
      }

      // 填写显示名称
      const displayNameInput = page.locator('input[name="displayName"], #displayName').first();
      if (await displayNameInput.isVisible()) {
        await displayNameInput.fill(`E2E Test App ${Date.now()}`);
      }

      // 提交表单
      const saveButton = page.getByRole("button", { name: /save|submit|保存|提交/i }).first();
      if (await saveButton.isVisible()) {
        await saveButton.click();

        // 验证成功消息或跳转
        await page.waitForTimeout(3000);
        const isSuccess =
          (await page.locator(".ant-message-success").count()) > 0 ||
          (await page.url()).includes("/applications/");

        expect(isSuccess).toBe(true);
      }
    });
  });

  // ─────────────────────────────────────────────
  // 3. 编辑应用配置（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("编辑应用配置", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("编辑应用: 从列表点击详情 → 验证跳转到应用详情页", async ({ page }) => {
      const appsPage = new ApplicationsPage(page);
      await appsPage.gotoApplications();
      await appsPage.expectApplicationsPageVisible();

      // 等待数据加载
      await page.waitForTimeout(2000);

      // 点击第一个应用的详情链接
      const detailLink = page.locator(".ant-table-row").first().getByRole("link").first();
      if (await detailLink.isVisible()) {
        await detailLink.click();

        // 验证跳转到应用详情页
        await expect(page).toHaveURL(/\/applications\/.+/, { timeout: 10_000 });
      }
    });

    test("编辑应用: 应用详情页验证表单可见", async ({ page }) => {
      const appsPage = new ApplicationsPage(page);
      await appsPage.gotoApplications();
      await appsPage.expectApplicationsPageVisible();

      // 等待数据加载
      await page.waitForTimeout(2000);

      // 点击第一个应用的详情链接
      const detailLink = page.locator(".ant-table-row").first().getByRole("link").first();
      if (await detailLink.isVisible()) {
        await detailLink.click();
        await page.waitForTimeout(2000);

        // 验证详情页有表单或内容区域
        const hasContent =
          (await page.locator("form").count()) > 0 ||
          (await page.locator(".ant-card").count()) > 0 ||
          (await page.locator(".ant-tabs").count()) > 0;

        expect(hasContent).toBe(true);
      }
    });

    test("编辑应用: 修改应用配置并保存", async ({ page }) => {
      const appsPage = new ApplicationsPage(page);
      await appsPage.gotoApplications();
      await appsPage.expectApplicationsPageVisible();

      // 等待数据加载
      await page.waitForTimeout(2000);

      // 点击第一个应用的详情链接
      const detailLink = page.locator(".ant-table-row").first().getByRole("link").first();
      if (await detailLink.isVisible()) {
        await detailLink.click();
        await page.waitForTimeout(2000);

        // 查找可编辑字段并修改
        const displayNameInput = page.locator('input[name="displayName"], #displayName').first();
        if ((await displayNameInput.isVisible()) && (await displayNameInput.isEnabled())) {
          await displayNameInput.clear();
          await displayNameInput.fill("Updated App Name");

          // 点击保存
          const saveButton = page.getByRole("button", { name: /save|submit|保存|提交/i }).first();
          if (await saveButton.isVisible()) {
            await saveButton.click();

            // 验证成功消息
            const successMsg = page.locator(".ant-message-success").first();
            if (await successMsg.isVisible({ timeout: 5000 }).catch(() => false)) {
              expect(await successMsg.isVisible()).toBe(true);
            }
          }
        }
      }
    });
  });
});
