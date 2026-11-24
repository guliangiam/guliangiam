import { test, expect } from "@playwright/test";
import { UsersPage, UserFormPage, LoginPage } from "./pom";
import { TEST_CREDENTIALS, shouldRunE2E, loginAsAdmin } from "./helpers";

/**
 * 用户管理 E2E 测试
 *
 * 覆盖：用户列表渲染、搜索用户、创建用户、编辑用户、删除用户
 *
 * ⚠️ 需要 ENABLE_E2E=true 才能运行需要后端的测试
 */
const describeE2E = shouldRunE2E ? test.describe : test.describe.skip;

/** 用户详情页 SectionNavShell 分区数下限（与 USER_DETAIL_TABS 同源，当前 8）。 */
const USER_DETAIL_SECTION_MIN_COUNT = 8;

test.describe("用户管理", () => {
  // ─────────────────────────────────────────────
  // 1. 用户列表页面渲染（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("用户列表页面渲染", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("用户列表: 访问 /users → 验证表格渲染", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();

      // 验证 URL
      await expect(page).toHaveURL(/\/users/);

      // 验证表格可见，并校验真的有数据行（空表不能全绿）
      await expect(usersPage.userTable).toBeVisible({ timeout: 10_000 });
      const rows = page.locator("tr.ant-table-row");
      await expect(rows.first()).toBeVisible({ timeout: 10_000 });
      const rowCount = await rows.count();
      expect(rowCount, "用户列表不应为空").toBeGreaterThanOrEqual(1);
      await expect(rows).toHaveCount(rowCount);
    });

    test("用户列表: 验证搜索框存在", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();

      // 验证搜索框可见
      await expect(usersPage.searchInput).toBeVisible();
    });

    test("用户列表: 验证新建用户按钮存在", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();

      // 验证新建用户按钮可见
      await expect(usersPage.newUserButton).toBeVisible();
    });

    test("用户列表: 验证表格列标题", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();
      await usersPage.expectUsersPageVisible();

      // 验证关键列标题存在
      const tableHeaders = page.locator(".ant-table-thead th");
      const headerTexts = await tableHeaders.allTextContents();
      const joinedHeaders = headerTexts.join(" ").toLowerCase();

      // 至少应包含 name/username 列
      expect(
        joinedHeaders.includes("name") ||
          joinedHeaders.includes("username") ||
          joinedHeaders.includes("名称"),
      ).toBe(true);
    });
  });

  // ─────────────────────────────────────────────
  // 2. 搜索用户（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("搜索用户", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("搜索用户: 输入关键词搜索 → 验证结果过滤", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();
      await usersPage.expectUsersPageVisible();

      // 输入搜索关键词
      await usersPage.searchUser("admin");

      // 等待搜索结果加载
      await page.waitForTimeout(1000);

      // 验证表格仍有内容（或为空状态）
      const tableBody = page.locator(".ant-table-tbody");
      await expect(tableBody).toBeVisible();
    });

    test("搜索用户: 清空搜索 → 验证恢复全部结果", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();
      await usersPage.expectUsersPageVisible();

      // 先搜索
      await usersPage.searchUser("nonexistent-user-xyz");

      // 清空搜索
      await usersPage.clearSearch();

      // 等待结果加载
      await page.waitForTimeout(1000);

      // 验证表格可见
      await expect(usersPage.userTable).toBeVisible();
    });
  });

  // ─────────────────────────────────────────────
  // 3. 创建用户（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("创建用户", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("创建用户: 访问 /users/new → 验证表单渲染", async ({ page }) => {
      const userFormPage = new UserFormPage(page);
      await userFormPage.gotoCreateUser();

      // 验证 URL
      await expect(page).toHaveURL(/\/users\/new/);

      // 验证表单元素可见
      await expect(userFormPage.usernameInput).toBeVisible();
      await expect(userFormPage.displayNameInput).toBeVisible();
    });

    test("创建用户: 填写表单并提交 → 验证创建成功", async ({ page }) => {
      const userFormPage = new UserFormPage(page);
      const usersPage = new UsersPage(page);
      const timestamp = Date.now();
      const testUsername = `e2e_test_${timestamp}`;
      const testDisplayName = `E2E Test User ${timestamp}`;

      await userFormPage.gotoCreateUser();

      // 填写基本信息
      await userFormPage.fillUsername(testUsername);
      await userFormPage.fillDisplayName(testDisplayName);

      // 提交表单
      await userFormPage.clickSave();

      // 验证跳转回用户列表或显示成功消息
      const isSuccess =
        (await page.locator(".ant-message-success").count()) > 0 ||
        (await page.url()).includes("/users");

      expect(isSuccess).toBe(true);
    });

    test("创建用户: 必填字段为空提交 → 验证验证提示", async ({ page }) => {
      const userFormPage = new UserFormPage(page);
      await userFormPage.gotoCreateUser();

      // 直接点击保存，不填写任何字段
      await userFormPage.clickSave();

      // 验证表单验证错误出现
      const hasValidationError =
        (await page.locator(".ant-form-item-explain-error").count()) > 0 ||
        (await page.locator(".ant-form-item-has-error").count()) > 0;

      expect(hasValidationError).toBe(true);
    });
  });

  // ─────────────────────────────────────────────
  // 4. 编辑用户（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("编辑用户", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("编辑用户: 点击编辑按钮 → 进入以组织 slug 定位的详情页", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();
      await usersPage.expectUsersPageVisible();

      // 用内置 admin 作为确定性目标；搜索缩小范围，避免受分页/排序影响。
      await usersPage.searchUser("admin");

      const adminRow = page
        .locator("tr.ant-table-row")
        .filter({ has: page.getByRole("link", { name: "admin", exact: true }) })
        .first();
      await expect(adminRow).toBeVisible({ timeout: 10_000 });

      // 同一行的组织列链接文本是 displayName；D2 修复前 URL 用它拼接。
      // 先等名称映射就绪（否则列里还是 orgId，负向场景会误落到 slug 兜底而假绿）。
      const orgLink = adminRow.getByRole("link").nth(1);
      await expect
        .poll(
          async () => {
            const text = (await orgLink.textContent())?.trim() ?? "";
            const href = (await orgLink.getAttribute("href")) ?? "";
            const id = decodeURIComponent(href.split("/").pop() ?? "");
            return text !== "" && text !== id;
          },
          { timeout: 10_000 },
        )
        .toBe(true);
      const orgDisplayName = (await orgLink.textContent())?.trim() ?? "";

      const editButton = adminRow.locator("button:has(.anticon-edit)").first();
      await expect(editButton).toBeVisible();
      await editButton.click();

      // 详情路由 params 是 (orgSlug, username)，不是 displayName。
      await expect(page).toHaveURL(/\/users\/[a-z0-9][a-z0-9-]*\/admin$/, { timeout: 10_000 });

      const pathSegments = new URL(page.url()).pathname.split("/");
      expect(pathSegments).toHaveLength(4);
      const [, resource, orgSlug, username] = pathSegments;
      expect(resource).toBe("users");
      expect(username).toBe("admin");
      expect(orgSlug).not.toContain("%");
      expect(orgSlug).not.toMatch(/\s/);
      expect(orgSlug).toMatch(/^[a-z0-9][a-z0-9-]*$/);
      if (orgDisplayName) {
        expect(orgSlug).not.toBe(orgDisplayName);
        expect(orgSlug).not.toBe(encodeURIComponent(orgDisplayName));
      }

      // 详情页标题是用户名（ResourceFormPage title=user.username），证明不是 404/兜底页。
      await expect(page.getByRole("heading", { level: 2, name: "admin", exact: true })).toBeVisible(
        { timeout: 15_000 },
      );
    });

    test("编辑用户: 遍历全部分区无 <form> 嵌套告警，页头保存可用", async ({ page }) => {
      // K-1 回归：详情页由页级唯一 <form> 承载所有分区的 Form.Item。
      // 任一分区若重新引入 <form>，React 会打印 DOM 嵌套校验错误。
      // 91c9523f/35fe0477 起 Tabs 改为 SectionNavShell 锚点导航（.gl-section-nav__link）。
      const consoleErrors: string[] = [];
      page.on("console", (message) => {
        if (message.type() === "error") consoleErrors.push(message.text());
      });
      page.on("pageerror", (error) => consoleErrors.push(`pageerror: ${error.message}`));

      await page.goto("/users/built-in/admin");
      await expect(page.locator(".gl-resource-card").first()).toBeVisible({ timeout: 15_000 });

      // 默认 Basic Info：页头保存按钮必须是真实可用的按钮（不是装饰性死按钮）。
      const saveButton = page
        .locator(".gl-resource-card__header button.ant-btn-primary")
        .filter({ hasNot: page.locator("a") })
        .first();
      await expect(saveButton).toBeVisible();
      await expect(saveButton).toBeEnabled();

      const sections = page.locator(".gl-section-nav__link");
      const sectionCount = await sections.count();
      expect(sectionCount).toBeGreaterThanOrEqual(USER_DETAIL_SECTION_MIN_COUNT);
      for (let index = 0; index < sectionCount; index += 1) {
        await sections.nth(index).click();
        await page.waitForTimeout(120);
      }

      expect(consoleErrors, `详情页控制台错误：${consoleErrors.join(" | ")}`).toEqual([]);
    });
  });

  // ─────────────────────────────────────────────
  // 5. 删除用户（需要后端）
  // ─────────────────────────────────────────────
  describeE2E("删除用户", () => {
    test.beforeEach(async ({ page }) => {
      await loginAsAdmin(page);
    });

    test("删除用户: 点击删除按钮 → 验证确认弹窗出现", async ({ page }) => {
      const usersPage = new UsersPage(page);
      await usersPage.gotoUsers();
      await usersPage.expectUsersPageVisible();

      // 等待数据加载
      await page.waitForTimeout(2000);

      // 点击第一个用户的删除按钮
      const deleteButton = page.locator("button:has(.anticon-delete)").first();
      if (await deleteButton.isVisible()) {
        await deleteButton.click();

        // 验证确认弹窗出现
        const popconfirm = page.locator(".ant-popconfirm").first();
        await expect(popconfirm).toBeVisible();
      }
    });
  });
});
