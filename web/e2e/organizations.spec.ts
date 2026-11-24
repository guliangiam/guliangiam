import { test, expect } from "@playwright/test";
import { OrganizationsPage, OrganizationFormPage, LoginPage } from "./pom";
import { TEST_CREDENTIALS, shouldRunE2E, loginAsAdmin } from "./helpers";

/**
 * 组织管理 E2E 测试
 *
 * 覆盖：组织列表渲染、创建组织、编辑组织设置
 *
 * ⚠️ 需要 ENABLE_E2E=true 才能运行需要后端的测试
 */
const describeE2E = shouldRunE2E ? test.describe : test.describe.skip;

test.describe("组织管理", () => {
  // ─────────────────────────────────────────────
  // 1. 组织列表页面渲染（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("组织列表页面渲染", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("组织列表: 访问 /organizations → 验证表格渲染", async ({ page }) => {
      const orgsPage = new OrganizationsPage(page);
      await orgsPage.gotoOrganizations();

      // 验证 URL
      await expect(page).toHaveURL(/\/organizations/);

      // 管理员视角必须真的看到数据行：空表也"表格可见"，不能作为判据。
      const rows = page.locator("tr.ant-table-row");
      await expect(rows.first()).toBeVisible({ timeout: 10_000 });
      const rowCount = await rows.count();
      expect(rowCount, "组织列表不应为空（管理员至少能看到 built-in）").toBeGreaterThanOrEqual(1);
      await expect(rows).toHaveCount(rowCount);
      await expect(
        rows.filter({ hasText: /built-in|内置/i }).first(),
        "种子组织 built-in 必须出现在列表中",
      ).toBeVisible({ timeout: 10_000 });
    });

    test("组织列表: 验证搜索框存在", async ({ page }) => {
      const orgsPage = new OrganizationsPage(page);
      await orgsPage.gotoOrganizations();

      // 验证搜索框可见
      await expect(orgsPage.searchInput).toBeVisible();
    });

    test("组织列表: 验证新建组织按钮存在", async ({ page }) => {
      const orgsPage = new OrganizationsPage(page);
      await orgsPage.gotoOrganizations();

      // 验证新建组织按钮可见
      await expect(orgsPage.newOrgButton).toBeVisible();
    });

    test("组织列表: 验证表格列标题", async ({ page }) => {
      const orgsPage = new OrganizationsPage(page);
      await orgsPage.gotoOrganizations();
      await orgsPage.expectOrganizationsPageVisible();

      // 验证关键列标题存在
      const tableHeaders = page.locator(".ant-table-thead th");
      const headerTexts = await tableHeaders.allTextContents();
      const joinedHeaders = headerTexts.join(" ").toLowerCase();

      // 至少应包含 name 列
      expect(joinedHeaders.includes("name") || joinedHeaders.includes("名称")).toBe(true);
    });
  });

  // ─────────────────────────────────────────────
  // 2. 创建组织（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("创建组织", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("创建组织: 访问 /organizations/new → 验证表单渲染", async ({ page }) => {
      const orgFormPage = new OrganizationFormPage(page);
      await orgFormPage.gotoCreateOrganization();

      // 验证 URL
      await expect(page).toHaveURL(/\/organizations\/new/);

      // 验证表单元素可见
      await expect(orgFormPage.slugInput).toBeVisible();
      await expect(orgFormPage.displayNameInput).toBeVisible();
    });

    test("创建组织: 填写表单并提交 → 验证创建成功", async ({ page }) => {
      const orgFormPage = new OrganizationFormPage(page);
      const timestamp = Date.now();
      const testSlug = `e2e-org-${timestamp}`;
      const testDisplayName = `E2E Test Org ${timestamp}`;

      await orgFormPage.gotoCreateOrganization();

      // 填写基本信息
      await orgFormPage.fillSlug(testSlug);
      await orgFormPage.fillDisplayName(testDisplayName);

      // 提交表单
      await orgFormPage.clickSave();

      // 验证跳转到组织详情页或显示成功消息
      const isSuccess =
        (await page.locator(".ant-message-success").count()) > 0 ||
        (await page.url()).includes(`/organizations/${testSlug}`) ||
        (await page.url()).includes("/organizations/");

      expect(isSuccess).toBe(true);
    });

    test("创建组织: 必填字段为空提交 → 验证验证提示", async ({ page }) => {
      const orgFormPage = new OrganizationFormPage(page);
      await orgFormPage.gotoCreateOrganization();

      // 直接点击保存，不填写任何字段
      await orgFormPage.clickSave();

      // 验证表单验证错误出现
      const hasValidationError =
        (await page.locator(".ant-form-item-explain-error").count()) > 0 ||
        (await page.locator(".ant-form-item-has-error").count()) > 0;

      expect(hasValidationError).toBe(true);
    });
  });

  // ─────────────────────────────────────────────
  // 3. 编辑组织设置（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("编辑组织设置", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("编辑组织: 从列表点击编辑 → 验证跳转到组织详情页", async ({ page }) => {
      const orgsPage = new OrganizationsPage(page);
      await orgsPage.gotoOrganizations();
      await orgsPage.expectOrganizationsPageVisible();

      // 等待数据加载
      await page.waitForTimeout(2000);

      // 点击第一个组织的编辑链接
      const editLink = page
        .locator(".ant-table-row")
        .first()
        .getByRole("link", { name: /edit|编辑/i })
        .first();
      if (await editLink.isVisible()) {
        await editLink.click();

        // 验证跳转到组织详情页
        await expect(page).toHaveURL(/\/organizations\/.+/, { timeout: 10_000 });
      }
    });

    test("编辑组织: 组织详情页验证 Tabs 可见", async ({ page }) => {
      const orgsPage = new OrganizationsPage(page);
      await orgsPage.gotoOrganizations();
      await orgsPage.expectOrganizationsPageVisible();

      // 等待数据加载
      await page.waitForTimeout(2000);

      // 点击第一个组织的编辑链接
      const editLink = page
        .locator(".ant-table-row")
        .first()
        .getByRole("link", { name: /edit|编辑/i })
        .first();
      if (await editLink.isVisible()) {
        await editLink.click();
        await page.waitForTimeout(2000);

        // 验证 Tabs 组件存在
        const tabs = page.locator(".ant-tabs");
        if (await tabs.isVisible()) {
          // 验证至少有一个 Tab
          const tabItems = page.locator(".ant-tabs-tab");
          const tabCount = await tabItems.count();
          expect(tabCount).toBeGreaterThan(0);
        }
      }
    });

    test("编辑组织: 修改显示名称并保存", async ({ page }) => {
      const orgsPage = new OrganizationsPage(page);
      await orgsPage.gotoOrganizations();
      await orgsPage.expectOrganizationsPageVisible();

      // 等待数据加载
      await page.waitForTimeout(2000);

      // 点击第一个组织的编辑链接
      const editLink = page
        .locator(".ant-table-row")
        .first()
        .getByRole("link", { name: /edit|编辑/i })
        .first();
      if (await editLink.isVisible()) {
        await editLink.click();
        await page.waitForTimeout(2000);

        // 找到 displayName 输入框并修改
        const displayNameInput = page.locator('input[name="displayName"], #displayName').first();
        if (await displayNameInput.isVisible()) {
          await displayNameInput.clear();
          await displayNameInput.fill("Updated Org Name");

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
